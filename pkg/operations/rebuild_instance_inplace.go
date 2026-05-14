/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package operations

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	rebuildFromAnnotation           = "operations.kubeblocks.io/rebuild-from"
	rebuildTmpPVCNameLabel          = "operations.kubeblocks.io/rebuild-tmp-pvc"
	sourcePVReclaimPolicyAnnotation = "operations.kubeblocks.io/source-reclaim-policy"

	waitingForInstanceReadyMessage   = "Waiting for the rebuilding instance to be ready"
	waitingForPostReadyRestorePrefix = "Waiting for postReady Restore"

	ignoreRoleCheckAnnotationKey = "operations.kubeblocks.io/ignore-role-check"
)

type inplaceRebuildHelper struct {
	targetPod *corev1.Pod
	backup    *dpv1alpha1.Backup
	instance  opsv1alpha1.Instance
	actionSet *dpv1alpha1.ActionSet
	// key: source pvc name, value: the tmp pvc which using to rebuild
	pvcMap                 map[string]*corev1.PersistentVolumeClaim
	synthesizedComp        *component.SynthesizedComponent
	volumes                []corev1.Volume
	volumeMounts           []corev1.VolumeMount
	envForRestore          []corev1.EnvVar
	sourceBackupTargetName string
	rebuildPrefix          string
	index                  int
}

// rebuildPodWithNoBackup rebuilds the instance with no backup.
func (inPlaceHelper *inplaceRebuildHelper) rebuildInstanceWithNoBackup(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	progressDetail *opsv1alpha1.ProgressStatusDetail) (bool, error) {
	// 1. restore the new pvs.
	completed, err := inPlaceHelper.rebuildInstancePVByPod(reqCtx, cli, opsRes, progressDetail)
	if err != nil || !completed {
		return false, err
	}
	if progressDetail.Message != waitingForInstanceReadyMessage {
		// 2. rebuild source pvcs and recreate the instance by deleting it.
		return false, inPlaceHelper.rebuildSourcePVCsAndRecreateInstance(reqCtx, cli, opsRes.OpsRequest, progressDetail)
	}

	// 3. waiting for new instance is available.
	return instanceIsAvailable(inPlaceHelper.synthesizedComp, inPlaceHelper.targetPod, opsRes.OpsRequest.Annotations[ignoreRoleCheckAnnotationKey])
}

// rebuildPodWithBackup rebuild instance with backup.
func (inPlaceHelper *inplaceRebuildHelper) rebuildInstanceWithBackup(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	progressDetail *opsv1alpha1.ProgressStatusDetail) (bool, error) {

	getRestore := func(stage dpv1alpha1.RestoreStage) (*dpv1alpha1.Restore, string, error) {
		restoreName := fmt.Sprintf("%s-%s-%s-%s-%d", inPlaceHelper.rebuildPrefix, strings.ToLower(string(stage)),
			common.CutString(opsRes.OpsRequest.Name, 10), inPlaceHelper.synthesizedComp.Name, inPlaceHelper.index)
		restoreName = constant.ShortenKubeName(restoreName, constant.KubeNameMaxLength)
		restore := &dpv1alpha1.Restore{}
		if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: restoreName, Namespace: opsRes.Cluster.Namespace}, restore); err != nil {
			return nil, restoreName, client.IgnoreNotFound(err)
		}
		return restore, restoreName, nil
	}

	waitRestoreCompleted := func(stage dpv1alpha1.RestoreStage) (bool, error) {
		restore, restoreName, err := getRestore(stage)
		if err != nil {
			return false, nil
		}
		if restore == nil {
			// create Restore CR
			if stage == dpv1alpha1.PostReady {
				//  waiting for the pod is available and do PostReady restore.
				available, err := instanceIsAvailable(inPlaceHelper.synthesizedComp, inPlaceHelper.targetPod, opsRes.OpsRequest.Annotations[ignoreRoleCheckAnnotationKey])
				if err != nil || !available {
					return false, err
				}
				return false, inPlaceHelper.createPostReadyRestore(reqCtx, cli, opsRes.OpsRequest, restoreName)
			}
			return false, inPlaceHelper.createPrepareDataRestore(reqCtx, cli, opsRes.OpsRequest, restoreName)
		}
		if restore.Status.Phase == dpv1alpha1.RestorePhaseFailed {
			return false, intctrlutil.NewFatalError(fmt.Sprintf(`pod "%s" rebuild failed, due to the Restore "%s" is Failed`, inPlaceHelper.targetPod.Name, restoreName))
		}
		if restore.Status.Phase != dpv1alpha1.RestorePhaseCompleted {
			progressDetail.Message = fmt.Sprintf(`Waiting for %s Restore "%s" to be completed`, stage, restoreName)
			return false, nil
		}
		return true, nil
	}

	var (
		completed bool
		err       error
	)
	// 1. restore the new instance pvs.
	if inPlaceHelper.actionSet.HasPrepareDataStage() {
		completed, err = waitRestoreCompleted(dpv1alpha1.PrepareData)
	} else {
		// if no prepareData stage, restore the pv by a tmp pod.
		completed, err = inPlaceHelper.rebuildInstancePVByPod(reqCtx, cli, opsRes, progressDetail)
	}
	if err != nil || !completed {
		return false, err
	}
	if progressDetail.Message != waitingForInstanceReadyMessage &&
		!strings.HasPrefix(progressDetail.Message, waitingForPostReadyRestorePrefix) {
		// 2. rebuild source pvcs and recreate the instance by deleting it.
		return false, inPlaceHelper.rebuildSourcePVCsAndRecreateInstance(reqCtx, cli, opsRes.OpsRequest, progressDetail)
	}
	if inPlaceHelper.actionSet.HasPostReadyStage() {
		// 3. do PostReady restore
		return waitRestoreCompleted(dpv1alpha1.PostReady)
	}
	return instanceIsAvailable(inPlaceHelper.synthesizedComp, inPlaceHelper.targetPod, opsRes.OpsRequest.Annotations[ignoreRoleCheckAnnotationKey])
}

// rebuildInstancePVByPod rebuilds the new instance pvs by a temp pod.
func (inPlaceHelper *inplaceRebuildHelper) rebuildInstancePVByPod(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	progressDetail *opsv1alpha1.ProgressStatusDetail) (bool, error) {
	rebuildPodName := fmt.Sprintf("%s-%s-%s-%d", inPlaceHelper.rebuildPrefix, common.CutString(opsRes.OpsRequest.Name, 20), inPlaceHelper.synthesizedComp.Name, inPlaceHelper.index)
	rebuildPod := &corev1.Pod{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, cli, client.ObjectKey{Name: rebuildPodName, Namespace: opsRes.Cluster.Namespace}, rebuildPod)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, inPlaceHelper.createTmpPVCsAndPod(reqCtx, cli, opsRes.OpsRequest, rebuildPodName)
	}
	if rebuildPod.Status.Phase != corev1.PodSucceeded {
		progressDetail.Message = fmt.Sprintf(`Waiting for rebuilding pod "%s" to be completed`, rebuildPod.Name)
		return false, nil
	}
	return true, nil
}

// createPrepareDataRestore creates a Restore to rebuild new pvs with prepareData stage.
func (inPlaceHelper *inplaceRebuildHelper) createPrepareDataRestore(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRequest *opsv1alpha1.OpsRequest,
	restoreName string) error {
	getVolumeMount := func(vctName string) string {
		for i := range inPlaceHelper.volumeMounts {
			if inPlaceHelper.volumeMounts[i].Name == vctName {
				return inPlaceHelper.volumeMounts[i].MountPath
			}
		}
		return ""
	}
	var volumeClaims []dpv1alpha1.RestoreVolumeClaim
	for _, tmpPVC := range inPlaceHelper.pvcMap {
		volumeClaim := dpv1alpha1.RestoreVolumeClaim{
			ObjectMeta:      tmpPVC.ObjectMeta,
			VolumeClaimSpec: tmpPVC.Spec,
		}
		vctName := tmpPVC.Labels[constant.VolumeClaimTemplateNameLabelKey]
		if dputils.ExistTargetVolume(inPlaceHelper.backup.Status.BackupMethod.TargetVolumes, vctName) {
			volumeClaim.VolumeSource = vctName
		} else {
			volumeClaim.MountPath = getVolumeMount(vctName)
		}
		volumeClaims = append(volumeClaims, volumeClaim)
	}
	schedulePolicy := dpv1alpha1.SchedulingSpec{
		Tolerations:               inPlaceHelper.targetPod.Spec.Tolerations,
		Affinity:                  inPlaceHelper.targetPod.Spec.Affinity,
		TopologySpreadConstraints: inPlaceHelper.targetPod.Spec.TopologySpreadConstraints,
	}
	if inPlaceHelper.instance.TargetNodeName != "" {
		schedulePolicy.NodeSelector = map[string]string{
			corev1.LabelHostname: inPlaceHelper.instance.TargetNodeName,
		}
	}
	restore := &dpv1alpha1.Restore{
		ObjectMeta: inPlaceHelper.buildRestoreMetaObject(opsRequest, restoreName),
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:             inPlaceHelper.backup.Name,
				Namespace:        opsRequest.Namespace,
				SourceTargetName: inPlaceHelper.sourceBackupTargetName,
			},
			Env: inPlaceHelper.envForRestore,
			PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
				SchedulingSpec:           schedulePolicy,
				VolumeClaimRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicySerial,
				RestoreVolumeClaims:      volumeClaims,
			},
		},
	}
	_ = intctrlutil.SetControllerReference(opsRequest, restore)
	return client.IgnoreAlreadyExists(cli.Create(reqCtx.Ctx, restore))
}

func (inPlaceHelper *inplaceRebuildHelper) buildRestoreMetaObject(opsRequest *opsv1alpha1.OpsRequest, restoreName string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      constant.ShortenKubeName(restoreName, constant.KubeNameMaxLength),
		Namespace: opsRequest.Namespace,
		Labels: map[string]string{
			constant.OpsRequestNameLabelKey:      opsRequest.Name,
			constant.OpsRequestNamespaceLabelKey: opsRequest.Namespace,
		},
	}
}

func (inPlaceHelper *inplaceRebuildHelper) getConnectionCredential(backup *dpv1alpha1.Backup) *dpv1alpha1.ConnectionCredential {
	if inPlaceHelper.sourceBackupTargetName == "" {
		if backup.Status.Target == nil {
			return nil
		}
		return backup.Status.Target.ConnectionCredential
	}
	for _, target := range backup.Status.Targets {
		if target.Name == inPlaceHelper.sourceBackupTargetName {
			return target.ConnectionCredential
		}
	}
	return nil
}

// createPostReadyRestore creates a Restore to restore the data with postReady stage.
func (inPlaceHelper *inplaceRebuildHelper) createPostReadyRestore(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRequest *opsv1alpha1.OpsRequest,
	restoreName string) error {
	labels := getWellKnownLabels(inPlaceHelper.synthesizedComp)
	if inPlaceHelper.targetPod.Labels[constant.KBAppPodNameLabelKey] == inPlaceHelper.targetPod.Name {
		labels[constant.KBAppPodNameLabelKey] = inPlaceHelper.targetPod.Name
	}
	podSelector := metav1.LabelSelector{
		MatchLabels: labels,
	}
	// TODO: support to rebuild instance from backup when the PodSelectionStrategy of source target is All .
	restore := &dpv1alpha1.Restore{
		ObjectMeta: inPlaceHelper.buildRestoreMetaObject(opsRequest, restoreName),
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:             inPlaceHelper.backup.Name,
				Namespace:        inPlaceHelper.backup.Namespace,
				SourceTargetName: inPlaceHelper.sourceBackupTargetName,
			},
			Env: inPlaceHelper.envForRestore,
			ReadyConfig: &dpv1alpha1.ReadyConfig{
				ExecAction: &dpv1alpha1.ExecAction{
					Target: dpv1alpha1.ExecActionTarget{PodSelector: podSelector},
				},
				JobAction: &dpv1alpha1.JobAction{
					Target: dpv1alpha1.JobActionTarget{PodSelector: dpv1alpha1.PodSelector{
						LabelSelector: &podSelector,
						Strategy:      dpv1alpha1.PodSelectionStrategyAny,
					}},
				},
				ConnectionCredential: inPlaceHelper.getConnectionCredential(inPlaceHelper.backup),
			},
		},
	}
	backupMethod := inPlaceHelper.backup.Status.BackupMethod
	if backupMethod.TargetVolumes != nil {
		restore.Spec.ReadyConfig.JobAction.Target.VolumeMounts = backupMethod.TargetVolumes.VolumeMounts
	}
	_ = intctrlutil.SetControllerReference(opsRequest, restore)
	return client.IgnoreAlreadyExists(cli.Create(reqCtx.Ctx, restore))
}

// createTmpPVCsAndPod creates the tmp pvcs and pod.
func (inPlaceHelper *inplaceRebuildHelper) createTmpPVCsAndPod(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRequest *opsv1alpha1.OpsRequest,
	tmpPodName string) error {
	for _, v := range inPlaceHelper.pvcMap {
		_ = intctrlutil.SetControllerReference(opsRequest, v)
		if err := cli.Create(reqCtx.Ctx, v); client.IgnoreAlreadyExists(err) != nil {
			return err
		}
	}
	container := &corev1.Container{
		Name:            "rebuild",
		Command:         []string{"sh", "-c", "echo 'rebuild done.'"},
		ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy)),
		Image:           viper.GetString(constant.KBToolsImage),
		VolumeMounts:    inPlaceHelper.volumeMounts,
	}
	intctrlutil.InjectZeroResourcesLimitsIfEmpty(container)
	rebuildPodBuilder := builder.NewPodBuilder(inPlaceHelper.targetPod.Namespace, tmpPodName).AddTolerations(inPlaceHelper.targetPod.Spec.Tolerations...).
		AddContainer(*container).
		AddVolumes(inPlaceHelper.volumes...).
		SetRestartPolicy(corev1.RestartPolicyNever).
		AddLabels(constant.OpsRequestNameLabelKey, opsRequest.Name).
		AddLabels(constant.OpsRequestNamespaceLabelKey, opsRequest.Namespace).
		SetTopologySpreadConstraints(inPlaceHelper.targetPod.Spec.TopologySpreadConstraints).
		SetAffinity(inPlaceHelper.targetPod.Spec.Affinity).
		SetImagePullSecrets(intctrlutil.BuildImagePullSecrets())
	if inPlaceHelper.instance.TargetNodeName != "" {
		rebuildPodBuilder.SetNodeSelector(map[string]string{
			corev1.LabelHostname: inPlaceHelper.instance.TargetNodeName,
		})
	}
	rebuildPod := rebuildPodBuilder.GetObject()
	_ = intctrlutil.SetControllerReference(opsRequest, rebuildPod)
	return client.IgnoreAlreadyExists(cli.Create(reqCtx.Ctx, rebuildPod))
}

// rebuildSourcePVCsAndRecreateInstance rebuilds the source pvcs and recreate the instance by deleting it.
func (inPlaceHelper *inplaceRebuildHelper) rebuildSourcePVCsAndRecreateInstance(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRequest *opsv1alpha1.OpsRequest,
	progressDetail *opsv1alpha1.ProgressStatusDetail) error {
	itsName := constant.GenerateWorkloadNamePattern(inPlaceHelper.synthesizedComp.ClusterName, inPlaceHelper.synthesizedComp.Name)

	needDeleteTargetPod := false
	waitingForSourcePVC := false
	for sourcePVCName, builtTmpPVC := range inPlaceHelper.pvcMap {
		tmpPVC, err := inPlaceHelper.getLiveTmpPVCOrBuilt(reqCtx, cli, builtTmpPVC)
		if err != nil {
			return err
		}
		pv, err := inPlaceHelper.getRestoredPV(reqCtx, cli, tmpPVC)
		if err != nil {
			return err
		}
		sourcePVC, err := inPlaceHelper.getSourcePVC(reqCtx, cli, sourcePVCName, tmpPVC.Namespace)
		if err != nil {
			if apierrors.IsNotFound(err) {
				waitingForSourcePVC = true
				continue
			}
			return err
		}
		if _, ok := pv.Annotations[rebuildFromAnnotation]; !ok {
			if pv.Labels[rebuildTmpPVCNameLabel] != tmpPVC.Name {
				if err = inPlaceHelper.retainAndAnnotatePV(reqCtx, cli, opsRequest.Name, pv, tmpPVC, sourcePVC); err != nil {
					return err
				}
			}
		} else if pv.Annotations[rebuildFromAnnotation] != opsRequest.Name {
			return intctrlutil.NewFatalError(fmt.Sprintf(`the pv "%s" is rebuilt by another OpsRequest "%s"`, pv.Name, pv.Annotations[rebuildFromAnnotation]))
		}
		if err = inPlaceHelper.preBindPVToSourcePVC(reqCtx, cli, pv, tmpPVC, sourcePVCName, sourcePVC); err != nil {
			return err
		}
		if sourcePVC.DeletionTimestamp != nil {
			waitingForSourcePVC = true
			if err := inPlaceHelper.removePVCFinalizer(reqCtx, cli, sourcePVC); err != nil {
				return err
			}
			continue
		}
		if sourcePVC.Spec.VolumeName == pv.Name {
			if err := inPlaceHelper.cleanupTmpPVC(reqCtx, cli, tmpPVC); err != nil {
				return err
			}
			latestPV := &corev1.PersistentVolume{}
			if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: pv.Name}, latestPV); err != nil {
				return err
			}
			if err := inPlaceHelper.revertReclaimPolicy(reqCtx, cli, latestPV); err != nil {
				return err
			}
			continue
		}

		waitingForSourcePVC = true
		if sourcePVC.Spec.VolumeName == "" {
			if err := inPlaceHelper.setSourcePVCVolumeNameForRebuild(reqCtx, cli, sourcePVC, pv.Name); err != nil {
				return err
			}
		} else {
			if err := inPlaceHelper.failIfSourcePVCBoundToOtherActiveRebuildPV(reqCtx, cli, opsRequest, sourcePVC, pv); err != nil {
				return err
			}
			needDeleteTargetPod = true
			if err := inPlaceHelper.deleteSourcePVCForRebuild(reqCtx, cli, sourcePVC); err != nil {
				return err
			}
		}
	}
	if needDeleteTargetPod {
		if err := inPlaceHelper.setInstanceNodeSelectorForRebuild(reqCtx, cli, itsName); err != nil {
			return err
		}
		if err := inPlaceHelper.deleteTargetPodForRebuild(reqCtx, cli, opsRequest); err != nil {
			return err
		}
	}
	if waitingForSourcePVC {
		progressDetail.Message = "Waiting for source PVCs to bind restored PVs"
		return nil
	}

	progressDetail.Message = waitingForInstanceReadyMessage
	return nil
}

func (inPlaceHelper *inplaceRebuildHelper) deleteTargetPodForRebuild(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRequest *opsv1alpha1.OpsRequest) error {
	var options []client.DeleteOption
	if opsRequest.Spec.Force {
		options = append(options, client.GracePeriodSeconds(0))
	}
	return client.IgnoreNotFound(intctrlutil.BackgroundDeleteObject(cli, reqCtx.Ctx, inPlaceHelper.targetPod, options...))
}

func (inPlaceHelper *inplaceRebuildHelper) setSourcePVCVolumeNameForRebuild(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	sourcePVC *corev1.PersistentVolumeClaim,
	pvName string) error {
	patch := client.MergeFrom(sourcePVC.DeepCopy())
	// Bind the replacement PVC that InstanceSet just created to the restored PV.
	// If it has already bound to another PV, the caller deletes it and waits for
	// a fresh PVC instead of changing an immutable bound claim.
	sourcePVC.Spec.VolumeName = pvName
	return cli.Patch(reqCtx.Ctx, sourcePVC, patch)
}

func (inPlaceHelper *inplaceRebuildHelper) setInstanceNodeSelectorForRebuild(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	itsName string) error {
	if inPlaceHelper.instance.TargetNodeName == "" {
		return nil
	}
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		its := &workloads.InstanceSet{}
		if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: itsName, Namespace: inPlaceHelper.synthesizedComp.Namespace}, its); err != nil {
			return err
		}
		patch := client.MergeFrom(its.DeepCopy())
		// If the rebuilt instance targets a node, ask InstanceSet to apply that node selector when it recreates the pod.
		if err := instanceset.MergeNodeSelectorOnceAnnotation(its, map[string]string{inPlaceHelper.targetPod.Name: inPlaceHelper.instance.TargetNodeName}); err != nil {
			return err
		}
		return cli.Patch(reqCtx.Ctx, its, patch)
	}); err != nil {
		return err
	}
	return nil
}

func (inPlaceHelper *inplaceRebuildHelper) getLiveTmpPVCOrBuilt(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	builtTmpPVC *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	tmpPVC := &corev1.PersistentVolumeClaim{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: builtTmpPVC.Name, Namespace: builtTmpPVC.Namespace}, tmpPVC); err != nil {
		if apierrors.IsNotFound(err) {
			return builtTmpPVC, nil
		}
		return nil, err
	}
	return tmpPVC, nil
}

func (inPlaceHelper *inplaceRebuildHelper) getSourcePVC(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	name string,
	namespace string) (*corev1.PersistentVolumeClaim, error) {
	sourcePVC := &corev1.PersistentVolumeClaim{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: name, Namespace: namespace}, sourcePVC); err != nil {
		return nil, err
	}
	return sourcePVC, nil
}

func (inPlaceHelper *inplaceRebuildHelper) getRestoredPV(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	tmpPVC *corev1.PersistentVolumeClaim) (*corev1.PersistentVolume, error) {
	pv := &corev1.PersistentVolume{}
	volumeName := tmpPVC.Spec.VolumeName
	if volumeName != "" {
		if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: volumeName}, pv); err != nil {
			// We'll get called again later when the PV exists
			return nil, err
		}
	} else {
		pvList := &corev1.PersistentVolumeList{}
		if err := cli.List(reqCtx.Ctx, pvList, client.MatchingLabels{rebuildTmpPVCNameLabel: tmpPVC.Name}, client.Limit(1)); err != nil {
			return nil, err
		}
		if len(pvList.Items) == 0 {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`can not found the pv by the pvc "%s"`, tmpPVC.Name))
		}
		pv = &pvList.Items[0]
	}
	return pv, nil
}

func (inPlaceHelper *inplaceRebuildHelper) retainAndAnnotatePV(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRequestName string,
	pv *corev1.PersistentVolume,
	tmpPVC *corev1.PersistentVolumeClaim,
	sourcePvc *corev1.PersistentVolumeClaim) error {
	patchPV := client.MergeFrom(pv.DeepCopy())
	// Examine the claimRef for the PV and see if it's bound to the correct PVC
	claimRef := pv.Spec.ClaimRef
	if claimRef != nil && (claimRef.Name != tmpPVC.Name || claimRef.Namespace != tmpPVC.Namespace) {
		return intctrlutil.NewFatalError(fmt.Sprintf(`the pv "%s" is not bound by the pvc "%s"`, pv.Name, tmpPVC.Name))
	}
	// 1. retain and label the pv
	pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
	if pv.Labels == nil {
		pv.Labels = map[string]string{}
	}
	// record the tmp pvc name to the pv labels. used for idempotent reentry if occurs error.
	pv.Labels[rebuildTmpPVCNameLabel] = tmpPVC.Name

	// annotate the pv with ['rebuildFromAnnotation', 'sourcePVReclaimPolicyAnnotation'].
	if pv.Annotations == nil {
		pv.Annotations = map[string]string{}
	}
	pv.Annotations[rebuildFromAnnotation] = opsRequestName
	if pv.Annotations[sourcePVReclaimPolicyAnnotation] == "" {
		// obtain the reclaim policy from the source pv.
		sourcePV := &corev1.PersistentVolume{}
		if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: sourcePvc.Spec.VolumeName}, sourcePV); err != nil {
			return err
		}
		pv.Annotations[sourcePVReclaimPolicyAnnotation] = string(sourcePV.Spec.PersistentVolumeReclaimPolicy)
	}
	return cli.Patch(reqCtx.Ctx, pv, patchPV)
}

func (inPlaceHelper *inplaceRebuildHelper) preBindPVToSourcePVC(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	pv *corev1.PersistentVolume,
	tmpPVC *corev1.PersistentVolumeClaim,
	sourcePVCName string,
	sourcePVC *corev1.PersistentVolumeClaim) error {
	uid := types.UID("")
	if sourcePVC.Spec.VolumeName == "" || sourcePVC.Spec.VolumeName == pv.Name {
		uid = sourcePVC.UID
	}
	claimRef := pv.Spec.ClaimRef
	if claimRef != nil &&
		claimRef.Name == sourcePVCName &&
		claimRef.Namespace == tmpPVC.Namespace &&
		claimRef.UID == uid &&
		claimRef.APIVersion == "v1" &&
		claimRef.Kind == "PersistentVolumeClaim" {
		return nil
	}
	if claimRef != nil &&
		(claimRef.Name != tmpPVC.Name || claimRef.Namespace != tmpPVC.Namespace) &&
		(claimRef.Name != sourcePVCName || claimRef.Namespace != tmpPVC.Namespace) {
		return intctrlutil.NewFatalError(fmt.Sprintf(`the pv "%s" is not owned by the rebuild pvc "%s"`, pv.Name, tmpPVC.Name))
	}
	patchPV := client.MergeFrom(pv.DeepCopy())
	// Reserve the restored PV for the current source PVC object. If InstanceSet
	// recreates that PVC later, this check will run again and update the UID.
	pv.Spec.ClaimRef = &corev1.ObjectReference{
		APIVersion: "v1",
		Kind:       "PersistentVolumeClaim",
		Namespace:  tmpPVC.Namespace,
		Name:       sourcePVCName,
		UID:        uid,
	}
	return cli.Patch(reqCtx.Ctx, pv, patchPV)
}

func (inPlaceHelper *inplaceRebuildHelper) failIfSourcePVCBoundToOtherActiveRebuildPV(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRequest *opsv1alpha1.OpsRequest,
	sourcePVC *corev1.PersistentVolumeClaim,
	restoredPV *corev1.PersistentVolume) error {
	if sourcePVC.Spec.VolumeName == "" || sourcePVC.Spec.VolumeName == restoredPV.Name {
		return nil
	}
	sourcePV := &corev1.PersistentVolume{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: sourcePVC.Spec.VolumeName}, sourcePV); err != nil {
		return err
	}
	if sourcePV.Annotations == nil {
		return nil
	}
	ownerName := sourcePV.Annotations[rebuildFromAnnotation]
	if ownerName == "" || ownerName == opsRequest.Name {
		return nil
	}
	claimRef := sourcePV.Spec.ClaimRef
	if claimRef == nil || claimRef.Name != sourcePVC.Name || claimRef.Namespace != sourcePVC.Namespace {
		return nil
	}
	owner := &opsv1alpha1.OpsRequest{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: ownerName, Namespace: opsRequest.Namespace}, owner); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if owner.IsComplete() {
		return nil
	}
	return intctrlutil.NewFatalError(fmt.Sprintf(`the source pvc "%s" is already bound to pv "%s" restored by OpsRequest "%s"`,
		sourcePVC.Name, sourcePV.Name, ownerName))
}

func (inPlaceHelper *inplaceRebuildHelper) cleanupTmpPVC(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	tmpPVC *corev1.PersistentVolumeClaim) error {
	if tmpPVC.UID == "" || tmpPVC.DeletionTimestamp != nil {
		return nil
	}
	// if the tmp pvc exists, delete it.
	if err := intctrlutil.BackgroundDeleteObject(cli, reqCtx.Ctx, tmpPVC); err != nil {
		return err
	}
	return inPlaceHelper.removePVCFinalizer(reqCtx, cli, tmpPVC)
}

func (inPlaceHelper *inplaceRebuildHelper) deleteSourcePVCForRebuild(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	sourcePVC *corev1.PersistentVolumeClaim) error {
	if sourcePVC.DeletionTimestamp != nil {
		return nil
	}
	// Remove the old source PVC name. InstanceSet will recreate it from the normal volumeClaimTemplate.
	if err := intctrlutil.BackgroundDeleteObject(cli, reqCtx.Ctx, sourcePVC); err != nil {
		return err
	}
	return inPlaceHelper.removePVCFinalizer(reqCtx, cli, sourcePVC)
}

func (inPlaceHelper *inplaceRebuildHelper) revertReclaimPolicy(reqCtx intctrlutil.RequestCtx,
	cli client.Client, pv *corev1.PersistentVolume) error {
	reclaimPolicy := pv.Annotations[sourcePVReclaimPolicyAnnotation]
	patch := client.MergeFrom(pv.DeepCopy())
	if reclaimPolicy != "" && string(pv.Spec.PersistentVolumeReclaimPolicy) != reclaimPolicy {
		if pv.Status.Phase != corev1.VolumeBound {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting, `wait for the PV "%s" to be bound`, pv.GetName())
		}
		pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimPolicy(reclaimPolicy)
	}
	// Preserve the rebuild identity metadata (rebuildFromAnnotation,
	// sourcePVReclaimPolicyAnnotation, rebuildTmpPVCNameLabel) on the PV
	// so that a later re-entry on this OpsRequest can still resolve the
	// restored PV via the tmp PVC label. Clearing them eagerly here
	// caused getRestoredPV to return a Fatal error when the tmp PVC was
	// cleaned up between reconciles.
	return cli.Patch(reqCtx.Ctx, pv, patch)
}

func (inPlaceHelper *inplaceRebuildHelper) removePVCFinalizer(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	pvc *corev1.PersistentVolumeClaim) error {
	patch := client.MergeFrom(pvc.DeepCopy())
	pvc.Finalizers = nil
	return client.IgnoreNotFound(cli.Patch(reqCtx.Ctx, pvc, patch))
}

// getPVCMapAndVolumes gets the pvc map and the volume infos.
func getPVCMapAndVolumes(opsRes *OpsResource,
	synthesizedComp *component.SynthesizedComponent,
	targetPod *corev1.Pod,
	rebuildPrefix string,
	index int) (map[string]*corev1.PersistentVolumeClaim, []corev1.Volume, []corev1.VolumeMount, error) {
	var (
		volumes      []corev1.Volume
		volumeMounts []corev1.VolumeMount
		// key: source pvc name, value: tmp pvc
		pvcMap       = map[string]*corev1.PersistentVolumeClaim{}
		volumePVCMap = map[string]string{}
	)
	for _, volume := range targetPod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			volumePVCMap[volume.Name] = volume.PersistentVolumeClaim.ClaimName
		}
	}
	// backup's ready, then start to check restore
	workloadName := constant.GenerateWorkloadNamePattern(opsRes.Cluster.Name, synthesizedComp.Name)
	templateName, _, err := getTemplateNameAndOrdinal(workloadName, targetPod.Name)
	if err != nil {
		return nil, nil, nil, err
	}
	// TODO: create pvc by the volumeClaimTemplates of instance template if it is necessary.
	for i, vct := range synthesizedComp.VolumeClaimTemplates {
		sourcePVCName := volumePVCMap[vct.Name]
		if sourcePVCName == "" {
			sourcePVCName = intctrlutil.ComposePVCName(corev1.PersistentVolumeClaim{
				ObjectMeta: vct.ObjectMeta,
				Spec:       vct.Spec,
			}, workloadName, targetPod.Name)
		}
		pvcLabels := getWellKnownLabels(synthesizedComp)
		tmpPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-%d", rebuildPrefix, common.CutString(synthesizedComp.Name+"-"+vct.Name, 30), index),
				Namespace: targetPod.Namespace,
				Labels:    pvcLabels,
				Annotations: map[string]string{
					rebuildFromAnnotation: opsRes.OpsRequest.Name,
				},
			},
			Spec: vct.Spec,
		}
		plan.BuildPersistentVolumeClaimLabels(synthesizedComp, tmpPVC, vct.Name, templateName)
		pvcMap[sourcePVCName] = tmpPVC
		// build volumes and volumeMount
		volumes = append(volumes, corev1.Volume{
			Name: vct.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: tmpPVC.Name,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      vct.Name,
			MountPath: fmt.Sprintf("/kb-tmp/%d", i),
		})
	}
	return pvcMap, volumes, volumeMounts, nil
}

func getWellKnownLabels(synthesizedComp *component.SynthesizedComponent) map[string]string {
	return constant.GetCompLabels(synthesizedComp.ClusterName, synthesizedComp.Name)
}

// instanceIsAvailable checks if the instance is available.
func instanceIsAvailable(
	synthesizedComp *component.SynthesizedComponent,
	targetPod *corev1.Pod,
	ignoreRoleCheckAnnotation string) (bool, error) {
	if !targetPod.DeletionTimestamp.IsZero() {
		return false, nil
	}
	isFailed, isTimeout, _ := intctrlutil.IsPodFailedAndTimedOut(targetPod)
	if isFailed && isTimeout {
		return false, intctrlutil.NewFatalError(fmt.Sprintf(`the new instance "%s" is failed, please check it`, targetPod.Name))
	}
	if !podutils.IsPodAvailable(targetPod, synthesizedComp.MinReadySeconds, metav1.Now()) {
		return false, nil
	}
	// If roleProbe is not defined, return true.
	if len(synthesizedComp.Roles) == 0 || ignoreRoleCheckAnnotation == "true" {
		return true, nil
	}
	// check if the role detection is successfully.
	if _, ok := targetPod.Labels[constant.RoleLabelKey]; ok {
		return true, nil
	}
	return false, nil
}

func getTemplateNameAndOrdinal(workloadName, podName string) (string, int32, error) {
	podSuffix := strings.Replace(podName, workloadName+"-", "", 1)
	lastDashIndex := strings.LastIndex(podSuffix, "-")
	if lastDashIndex == len(podSuffix)-1 {
		return "", 0, fmt.Errorf("no pod ordinal found after the last dash")
	}
	templateName := ""
	indexStr := ""
	if lastDashIndex == -1 {
		indexStr = podSuffix
	} else {
		templateName = podSuffix[0:lastDashIndex]
		indexStr = podSuffix[lastDashIndex+1:]
	}
	index, err := strconv.ParseInt(indexStr, 10, 32)
	if err != nil {
		return "", 0, fmt.Errorf("failed to obtain pod ordinal")
	}
	return templateName, int32(index), nil
}
