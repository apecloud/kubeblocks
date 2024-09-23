/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	rebuildFromAnnotation  = "apps.kubeblocks.io/rebuild-from"
	rebuildTmpPVCNameLabel = "apps.kubeblocks.io/rebuild-tmp-pvc"

	waitingForInstanceReadyMessage   = "Waiting for the rebuilding instance to be ready"
	waitingForPostReadyRestorePrefix = "Waiting for postReady Restore"

	ignoreRoleCheckAnnotationKey = "kubeblocks.io/ignore-role-check"
)

type inplaceRebuildHelper struct {
	targetPod *corev1.Pod
	backup    *dpv1alpha1.Backup
	instance  opsv1alpha1.Instance
	actionSet *dpv1alpha1.ActionSet
	// key: source pvc name, value: the tmp pvc which using to rebuild
	pvcMap          map[string]*corev1.PersistentVolumeClaim
	synthesizedComp *component.SynthesizedComponent
	volumes         []corev1.Volume
	volumeMounts    []corev1.VolumeMount
	envForRestore   []corev1.EnvVar
	rebuildPrefix   string
	index           int
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
				Name:      inPlaceHelper.backup.Name,
				Namespace: opsRequest.Namespace,
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
		Name:      restoreName,
		Namespace: opsRequest.Namespace,
		Labels: map[string]string{
			constant.OpsRequestNameLabelKey:      opsRequest.Name,
			constant.OpsRequestNamespaceLabelKey: opsRequest.Namespace,
		},
	}
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
				Name:      inPlaceHelper.backup.Name,
				Namespace: inPlaceHelper.backup.Namespace,
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
	for sourcePVCName, v := range inPlaceHelper.pvcMap {
		tmpPVC := &corev1.PersistentVolumeClaim{}
		_ = cli.Get(reqCtx.Ctx, types.NamespacedName{Name: v.Name, Namespace: v.Namespace}, tmpPVC)
		if tmpPVC.UID == "" {
			// if the tmp pvc not exists in k8s, replace with v.
			tmpPVC = v
		}
		// 1. get the restored pv
		pv, err := inPlaceHelper.getRestoredPV(reqCtx, cli, tmpPVC)
		if err != nil {
			return err
		}
		if _, ok := pv.Annotations[rebuildFromAnnotation]; !ok {
			if pv.Labels[rebuildTmpPVCNameLabel] != tmpPVC.Name {
				// 2. retain and label the pv with 'rebuildTmpPVCNameLabel'
				if err = inPlaceHelper.retainAndLabelPV(reqCtx, cli, pv, tmpPVC); err != nil {
					return err
				}
			}
			// 3. cleanup the tmp pvc firstly.
			if err = inPlaceHelper.cleanupTmpPVC(reqCtx, cli, tmpPVC); err != nil {
				return err
			}
			// 4. release the pv and annotate the pv with 'rebuildFromAnnotation'.
			if err = inPlaceHelper.releasePV(reqCtx, cli, pv, opsRequest.Name); err != nil {
				return err
			}
		}
		// set volumeName to tmp pvc, it will be used when recreating the source pvc.
		tmpPVC.Spec.VolumeName = pv.Name
		// 5. recreate the source pvc.
		if err = inPlaceHelper.recreateSourcePVC(reqCtx, cli, tmpPVC, sourcePVCName, opsRequest.Name); err != nil {
			return err
		}
	}
	// update progress message and recreate the target instance by deleting it.
	progressDetail.Message = waitingForInstanceReadyMessage
	var options []client.DeleteOption
	if opsRequest.Spec.Force {
		options = append(options, client.GracePeriodSeconds(0))
	}

	if inPlaceHelper.instance.TargetNodeName != "" {
		// under the circumstance of using cloud disks, need to set node selector again to make sure pod
		// goes to the specified node
		its := &workloads.InstanceSet{}
		itsName := constant.GenerateWorkloadNamePattern(inPlaceHelper.synthesizedComp.ClusterName, inPlaceHelper.synthesizedComp.Name)
		if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: itsName, Namespace: inPlaceHelper.synthesizedComp.Namespace}, its); err != nil {
			return err
		}
		if err := instanceset.MergeNodeSelectorOnceAnnotation(its, map[string]string{inPlaceHelper.targetPod.Name: inPlaceHelper.instance.TargetNodeName}); err != nil {
			return err
		}
		if err := cli.Update(reqCtx.Ctx, its); err != nil {
			return err
		}
	}

	return intctrlutil.BackgroundDeleteObject(cli, reqCtx.Ctx, inPlaceHelper.targetPod, options...)
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

func (inPlaceHelper *inplaceRebuildHelper) retainAndLabelPV(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	pv *corev1.PersistentVolume,
	tmpPVC *corev1.PersistentVolumeClaim) error {
	patchPV := client.MergeFrom(pv.DeepCopy())
	// Examine the claimRef for the PV and see if it's bound to the correct PVC
	claimRef := pv.Spec.ClaimRef
	if claimRef != nil && claimRef.Name != tmpPVC.Name || claimRef.Namespace != tmpPVC.Namespace {
		return intctrlutil.NewFatalError(fmt.Sprintf(`the pv "%s" is not bound by the pvc "%s"`, pv.Name, tmpPVC.Name))
	}
	// 1. retain and label the pv
	pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
	if pv.Labels == nil {
		pv.Labels = map[string]string{}
	}
	// record the tmp pvc name to the pv labels. used for idempotent reentry if occurs error.
	pv.Labels[rebuildTmpPVCNameLabel] = tmpPVC.Name
	return cli.Patch(reqCtx.Ctx, pv, patchPV)
}

func (inPlaceHelper *inplaceRebuildHelper) cleanupTmpPVC(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	tmpPVC *corev1.PersistentVolumeClaim) error {
	// if the tmp pvc exists, delete it.
	if err := intctrlutil.BackgroundDeleteObject(cli, reqCtx.Ctx, tmpPVC); err != nil {
		return err
	}
	return inPlaceHelper.removePVCFinalizer(reqCtx, cli, tmpPVC)
}

func (inPlaceHelper *inplaceRebuildHelper) recreateSourcePVC(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	tmpPVC *corev1.PersistentVolumeClaim,
	sourcePVCName,
	opsRequestName string) error {
	sourcePvc := &corev1.PersistentVolumeClaim{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: sourcePVCName, Namespace: tmpPVC.Namespace}, sourcePvc); err != nil {
		// if not exists, wait for the pvc to recreate by external controller.
		return err
	}
	// if the pvc is rebuilt by current opsRequest, return.
	if sourcePvc.Annotations[rebuildFromAnnotation] == opsRequestName {
		return nil
	}
	// 1. retain labels of the source pvc.
	intctrlutil.MergeMetadataMapInplace(sourcePvc.Labels, &tmpPVC.Labels)
	// 2. delete the old pvc
	if err := intctrlutil.BackgroundDeleteObject(cli, reqCtx.Ctx, sourcePvc); err != nil {
		return err
	}
	if err := inPlaceHelper.removePVCFinalizer(reqCtx, cli, sourcePvc); err != nil {
		return err
	}

	// 3. recreate the pvc with restored PV.
	newPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sourcePVCName,
			Namespace: tmpPVC.Namespace,
			Labels:    tmpPVC.Labels,
			Annotations: map[string]string{
				rebuildFromAnnotation: opsRequestName,
			},
		},
		Spec: tmpPVC.Spec,
	}
	return cli.Create(reqCtx.Ctx, newPVC)
}

// releasePV releases the persistentVolume by resetting `claimRef`.
func (inPlaceHelper *inplaceRebuildHelper) releasePV(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	pv *corev1.PersistentVolume,
	opsRequestName string) error {
	patchPV := client.MergeFrom(pv.DeepCopy())
	pv.Spec.ClaimRef = nil
	if pv.Annotations == nil {
		pv.Annotations = map[string]string{}
	}
	pv.Annotations[rebuildFromAnnotation] = opsRequestName
	return cli.Patch(reqCtx.Ctx, pv, patchPV)
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
	templateName, _, err := component.GetTemplateNameAndOrdinal(workloadName, targetPod.Name)
	if err != nil {
		return nil, nil, nil, err
	}
	// TODO: create pvc by the volumeClaimTemplates of instance template if it is necessary.
	for i, vct := range synthesizedComp.VolumeClaimTemplates {
		sourcePVCName := volumePVCMap[vct.Name]
		if sourcePVCName == "" {
			return nil, nil, nil, intctrlutil.NewFatalError("")
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
		factory.BuildPersistentVolumeClaimLabels(synthesizedComp, tmpPVC, vct.Name, templateName)
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
