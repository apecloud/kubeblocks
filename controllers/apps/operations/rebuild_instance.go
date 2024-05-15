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
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/generics"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	rebuildFromAnnotation  = "apps.kubeblocks.io/rebuild-from"
	rebuildTmpPVCNameLabel = "apps.kubeblocks.io/rebuild-tmp-pvc"

	waitingForInstanceReadyMessage   = "Waiting for the rebuilding instance to be ready"
	waitingForPostReadyRestorePrefix = "Waiting for postReady Restore"

	ignoreRoleCheckAnnotationKey = "kubeblocks.io/ignore-role-check"
)

type rebuildInstanceOpsHandler struct{}

type instanceHelper struct {
	comp      *appsv1alpha1.ClusterComponentSpec
	targetPod *corev1.Pod
	backup    *dpv1alpha1.Backup
	instance  appsv1alpha1.Instance
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

var _ OpsHandler = rebuildInstanceOpsHandler{}

func init() {
	rebuildInstanceBehaviour := OpsBehaviour{
		FromClusterPhases: []appsv1alpha1.ClusterPhase{appsv1alpha1.AbnormalClusterPhase, appsv1alpha1.FailedClusterPhase, appsv1alpha1.UpdatingClusterPhase},
		ToClusterPhase:    appsv1alpha1.UpdatingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        rebuildInstanceOpsHandler{},
	}
	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.RebuildInstanceType, rebuildInstanceBehaviour)
}

// ActionStartedCondition the started condition when handle the rebuild-instance request.
func (r rebuildInstanceOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewInstancesRebuildingCondition(opsRes.OpsRequest), nil
}

func (r rebuildInstanceOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	if opsRes.OpsRequest.Spec.Force {
		return nil
	}
	for _, v := range opsRes.OpsRequest.Spec.RebuildFrom {
		compStatus, ok := opsRes.Cluster.Status.Components[v.ComponentName]
		if !ok {
			continue
		}
		// check if the component has matched the `Phase` condition
		if !slices.Contains([]appsv1alpha1.ClusterComponentPhase{appsv1alpha1.FailedClusterCompPhase,
			appsv1alpha1.AbnormalClusterCompPhase, appsv1alpha1.UpdatingClusterCompPhase}, compStatus.Phase) {
			return intctrlutil.NewFatalError(fmt.Sprintf(`the phase of component "%s" can not be %s`, v.ComponentName, compStatus.Phase))
		}
		comp := opsRes.Cluster.Spec.GetComponentByName(v.ComponentName)
		synthesizedComp, err := component.BuildSynthesizedComponentWrapper(reqCtx, cli, opsRes.Cluster, comp)
		if err != nil {
			return err
		}
		for _, ins := range v.Instances {
			targetPod := &corev1.Pod{}
			if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: ins.Name, Namespace: opsRes.Cluster.Namespace}, targetPod); err != nil {
				return err
			}
			isAvailable, _ := r.instanceIsAvailable(synthesizedComp, targetPod, "")
			if isAvailable {
				return intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" is availabled, can not rebuild it`, ins.Name))
			}
		}
	}
	return nil
}

func (r rebuildInstanceOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func (r rebuildInstanceOpsHandler) getInstanceProgressDetail(compStatus appsv1alpha1.OpsRequestComponentStatus, instance string) appsv1alpha1.ProgressStatusDetail {
	objectKey := getProgressObjectKey(constant.PodKind, instance)
	progressDetail := findStatusProgressDetail(compStatus.ProgressDetails, objectKey)
	if progressDetail != nil {
		return *progressDetail
	}
	return appsv1alpha1.ProgressStatusDetail{
		ObjectKey: objectKey,
		Status:    appsv1alpha1.ProcessingProgressStatus,
		Message:   fmt.Sprintf("Start to rebuild pod %s", instance),
	}
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for restart opsRequest.
func (r rebuildInstanceOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	var (
		oldOpsRequest   = opsRes.OpsRequest.DeepCopy()
		opsRequestPhase = opsRes.OpsRequest.Status.Phase
		expectCount     int
		completedCount  int
		failedCount     int
	)
	if opsRes.OpsRequest.Status.Components == nil {
		opsRes.OpsRequest.Status.Components = map[string]appsv1alpha1.OpsRequestComponentStatus{}
	}
	for _, v := range opsRes.OpsRequest.Spec.RebuildFrom {
		compStatus := opsRes.OpsRequest.Status.Components[v.ComponentName]
		comp := opsRes.Cluster.Spec.GetComponentByName(v.ComponentName)
		for i, instance := range v.Instances {
			expectCount += 1
			progressDetail := r.getInstanceProgressDetail(compStatus, instance.Name)
			if isCompletedProgressStatus(progressDetail.Status) {
				completedCount += 1
				if progressDetail.Status == appsv1alpha1.FailedProgressStatus {
					failedCount += 1
				}
				continue
			}
			// rebuild instance
			completed, err := r.rebuildInstance(reqCtx, cli, opsRes, comp, v.RestoreEnv, &progressDetail, instance, v.BackupName, i)
			if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
				// If a fatal error occurs, this instance rebuilds failed.
				progressDetail.SetStatusAndMessage(appsv1alpha1.FailedProgressStatus, err.Error())
				setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
				continue
			}
			if err != nil {
				return opsRequestPhase, 0, err
			}
			if completed {
				// if the pod has been rebuilt, set progressDetail phase to Succeed.
				progressDetail.SetStatusAndMessage(appsv1alpha1.SucceedProgressStatus,
					fmt.Sprintf("Rebuild pod %s successfully", instance.Name))
			}
			setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
		}
		opsRes.OpsRequest.Status.Components[v.ComponentName] = compStatus
	}
	if err := syncProgressToOpsRequest(reqCtx, cli, opsRes, oldOpsRequest, completedCount, expectCount); err != nil {
		return opsRequestPhase, 0, err
	}
	// check if the ops has been finished.
	if completedCount != expectCount {
		return opsRequestPhase, 0, nil
	}
	if failedCount == 0 {
		return appsv1alpha1.OpsSucceedPhase, 0, r.cleanupTmpResources(reqCtx, cli, opsRes)
	}
	return appsv1alpha1.OpsFailedPhase, 0, nil
}

// rebuildInstance rebuilds the instance.
func (r rebuildInstanceOpsHandler) rebuildInstance(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	comp *appsv1alpha1.ClusterComponentSpec,
	envForRestore []corev1.EnvVar,
	progressDetail *appsv1alpha1.ProgressStatusDetail,
	instance appsv1alpha1.Instance,
	backupName string,
	index int) (bool, error) {
	insHelper, err := r.prepareInstanceHelper(reqCtx, cli, opsRes, comp, envForRestore, instance, backupName, index)
	if err != nil {
		return false, err
	}
	if backupName == "" {
		return r.rebuildInstanceWithNoBackup(reqCtx, cli, opsRes, insHelper, progressDetail)
	}
	return r.rebuildInstanceWithBackup(reqCtx, cli, opsRes, insHelper, progressDetail)
}

func (r rebuildInstanceOpsHandler) prepareInstanceHelper(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	comp *appsv1alpha1.ClusterComponentSpec,
	envForRestore []corev1.EnvVar,
	instance appsv1alpha1.Instance,
	backupName string,
	index int) (*instanceHelper, error) {
	var (
		backup    *dpv1alpha1.Backup
		actionSet *dpv1alpha1.ActionSet
		err       error
	)
	if backupName != "" {
		// prepare backup infos
		backup = &dpv1alpha1.Backup{}
		if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: backupName, Namespace: opsRes.Cluster.Namespace}, backup); err != nil {
			return nil, err
		}
		if backup.Labels[dptypes.BackupTypeLabelKey] != string(dpv1alpha1.BackupTypeFull) {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`the backup "%s" is not a Full backup`, backupName))
		}
		if backup.Status.Phase != dpv1alpha1.BackupPhaseCompleted {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`the backup "%s" phase is not Completed`, backupName))
		}
		if backup.Status.BackupMethod == nil {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`the backupMethod of the backup "%s" can not be empty`, backupName))
		}
		actionSet, err = dputils.GetActionSetByName(reqCtx, cli, backup.Status.BackupMethod.ActionSetName)
		if err != nil {
			return nil, err
		}
	}
	targetPod := &corev1.Pod{}
	if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: instance.Name, Namespace: opsRes.Cluster.Namespace}, targetPod); err != nil {
		return nil, err
	}
	synthesizedComp, err := component.BuildSynthesizedComponentWrapper(reqCtx, cli, opsRes.Cluster, comp)
	if err != nil {
		return nil, err
	}
	rebuildPrefix := fmt.Sprintf("rebuild-%s", opsRes.OpsRequest.UID[:8])
	pvcMap, volumes, volumeMounts, err := r.getPVCMapAndVolumes(opsRes, synthesizedComp, targetPod, rebuildPrefix, index)
	if err != nil {
		return nil, err
	}
	return &instanceHelper{
		index:           index,
		comp:            comp,
		backup:          backup,
		instance:        instance,
		actionSet:       actionSet,
		synthesizedComp: synthesizedComp,
		pvcMap:          pvcMap,
		volumes:         volumes,
		targetPod:       targetPod,
		volumeMounts:    volumeMounts,
		rebuildPrefix:   rebuildPrefix,
		envForRestore:   envForRestore,
	}, nil
}

// rebuildPodWithNoBackup rebuilds the instance with no backup.
func (r rebuildInstanceOpsHandler) rebuildInstanceWithNoBackup(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	insHelper *instanceHelper,
	progressDetail *appsv1alpha1.ProgressStatusDetail) (bool, error) {
	// 1. restore the new pvs.
	completed, err := r.rebuildInstancePVByPod(reqCtx, cli, opsRes, insHelper, progressDetail)
	if err != nil || !completed {
		return false, err
	}
	if progressDetail.Message != waitingForInstanceReadyMessage {
		// 2. rebuild source pvcs and recreate the instance by deleting it.
		return false, r.rebuildSourcePVCsAndRecreateInstance(reqCtx, cli, opsRes.OpsRequest, progressDetail, insHelper)
	}

	// 3. waiting for new instance is available.
	return r.instanceIsAvailable(insHelper.synthesizedComp, insHelper.targetPod, opsRes.OpsRequest.Annotations[ignoreRoleCheckAnnotationKey])
}

// rebuildPodWithBackup rebuild instance with backup.
func (r rebuildInstanceOpsHandler) rebuildInstanceWithBackup(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	insHelper *instanceHelper,
	progressDetail *appsv1alpha1.ProgressStatusDetail) (bool, error) {

	getRestore := func(stage dpv1alpha1.RestoreStage) (*dpv1alpha1.Restore, string, error) {
		restoreName := fmt.Sprintf("%s-%s-%s-%s-%d", insHelper.rebuildPrefix, strings.ToLower(string(stage)),
			common.CutString(opsRes.OpsRequest.Name, 10), insHelper.comp.Name, insHelper.index)
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
				available, err := r.instanceIsAvailable(insHelper.synthesizedComp, insHelper.targetPod, opsRes.OpsRequest.Annotations[ignoreRoleCheckAnnotationKey])
				if err != nil || !available {
					return false, err
				}
				return false, r.createPostReadyRestore(reqCtx, cli, opsRes.OpsRequest, insHelper, restoreName)
			}
			return false, r.createPrepareDataRestore(reqCtx, cli, opsRes.OpsRequest, insHelper, restoreName)
		}
		if restore.Status.Phase == dpv1alpha1.RestorePhaseFailed {
			return false, intctrlutil.NewFatalError(fmt.Sprintf(`pod "%s" rebuild failed, due to the Restore "%s" is Failed`, insHelper.targetPod.Name, restoreName))
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
	if insHelper.actionSet.HasPrepareDataStage() {
		completed, err = waitRestoreCompleted(dpv1alpha1.PrepareData)
	} else {
		// if no prepareData stage, restore the pv by a tmp pod.
		completed, err = r.rebuildInstancePVByPod(reqCtx, cli, opsRes, insHelper, progressDetail)
	}
	if err != nil || !completed {
		return false, err
	}
	if progressDetail.Message != waitingForInstanceReadyMessage &&
		!strings.HasPrefix(progressDetail.Message, waitingForPostReadyRestorePrefix) {
		// 2. rebuild source pvcs and recreate the instance by deleting it.
		return false, r.rebuildSourcePVCsAndRecreateInstance(reqCtx, cli, opsRes.OpsRequest, progressDetail, insHelper)
	}
	if insHelper.actionSet.HasPostReadyStage() {
		// 3. do PostReady restore
		return waitRestoreCompleted(dpv1alpha1.PostReady)
	}
	return r.instanceIsAvailable(insHelper.synthesizedComp, insHelper.targetPod, opsRes.OpsRequest.Annotations[ignoreRoleCheckAnnotationKey])
}

// rebuildInstancePVByPod rebuilds the new instance pvs by a temp pod.
func (r rebuildInstanceOpsHandler) rebuildInstancePVByPod(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	insHelper *instanceHelper,
	progressDetail *appsv1alpha1.ProgressStatusDetail) (bool, error) {
	rebuildPodName := fmt.Sprintf("%s-%s-%s-%d", insHelper.rebuildPrefix, common.CutString(opsRes.OpsRequest.Name, 20), insHelper.comp.Name, insHelper.index)
	rebuildPod := &corev1.Pod{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, cli, client.ObjectKey{Name: rebuildPodName, Namespace: opsRes.Cluster.Namespace}, rebuildPod)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, r.createTmpPVCsAndPod(reqCtx, cli, opsRes.OpsRequest, insHelper, rebuildPodName)
	}
	if rebuildPod.Status.Phase != corev1.PodSucceeded {
		progressDetail.Message = fmt.Sprintf(`Waiting for rebuilding pod "%s" to be completed`, rebuildPod.Name)
		return false, nil
	}
	return true, nil
}

func (r rebuildInstanceOpsHandler) getWellKnownLabels(synthesizedComp *component.SynthesizedComponent) map[string]string {
	if synthesizedComp.CompDefName != "" {
		return constant.GetKBWellKnownLabelsWithCompDef(synthesizedComp.CompDefName, synthesizedComp.ClusterName, synthesizedComp.Name)
	}
	return constant.GetKBWellKnownLabels(synthesizedComp.ClusterDefName, synthesizedComp.ClusterName, synthesizedComp.Name)
}

// getPVCMapAndVolumes gets the pvc map and the volume infos.
func (r rebuildInstanceOpsHandler) getPVCMapAndVolumes(opsRes *OpsResource,
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
		pvcLabels    = r.getWellKnownLabels(synthesizedComp)
	)
	for _, volume := range targetPod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			volumePVCMap[volume.Name] = volume.PersistentVolumeClaim.ClaimName
		}
	}
	for i, vct := range synthesizedComp.VolumeClaimTemplates {
		sourcePVCName := volumePVCMap[vct.Name]
		if sourcePVCName == "" {
			return nil, nil, nil, intctrlutil.NewFatalError("")
		}
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
		factory.BuildPersistentVolumeClaimLabels(synthesizedComp, tmpPVC, vct.Name)
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

func (r rebuildInstanceOpsHandler) buildRestoreMetaObject(opsRequest *appsv1alpha1.OpsRequest, restoreName string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      restoreName,
		Namespace: opsRequest.Namespace,
		Labels: map[string]string{
			constant.OpsRequestNameLabelKey:      opsRequest.Name,
			constant.OpsRequestNamespaceLabelKey: opsRequest.Namespace,
		},
	}
}

// createPrepareDataRestore creates a Restore to rebuild new pvs with prepareData stage.
func (r rebuildInstanceOpsHandler) createPrepareDataRestore(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRequest *appsv1alpha1.OpsRequest,
	insHelper *instanceHelper,
	restoreName string) error {
	getVolumeMount := func(vctName string) string {
		for i := range insHelper.volumeMounts {
			if insHelper.volumeMounts[i].Name == vctName {
				return insHelper.volumeMounts[i].MountPath
			}
		}
		return ""
	}
	var volumeClaims []dpv1alpha1.RestoreVolumeClaim
	for _, tmpPVC := range insHelper.pvcMap {
		volumeClaim := dpv1alpha1.RestoreVolumeClaim{
			ObjectMeta:      tmpPVC.ObjectMeta,
			VolumeClaimSpec: tmpPVC.Spec,
		}
		vctName := tmpPVC.Labels[constant.VolumeClaimTemplateNameLabelKey]
		if dputils.ExistTargetVolume(insHelper.backup.Status.BackupMethod.TargetVolumes, vctName) {
			volumeClaim.VolumeSource = vctName
		} else {
			volumeClaim.MountPath = getVolumeMount(vctName)
		}
		volumeClaims = append(volumeClaims, volumeClaim)
	}
	schedulePolicy := dpv1alpha1.SchedulingSpec{
		Tolerations:               insHelper.targetPod.Spec.Tolerations,
		Affinity:                  insHelper.targetPod.Spec.Affinity,
		TopologySpreadConstraints: insHelper.targetPod.Spec.TopologySpreadConstraints,
	}
	if insHelper.instance.TargetNodeName != "" {
		schedulePolicy.NodeSelector = map[string]string{
			corev1.LabelHostname: insHelper.instance.TargetNodeName,
		}
	}
	restore := &dpv1alpha1.Restore{
		ObjectMeta: r.buildRestoreMetaObject(opsRequest, restoreName),
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:      insHelper.backup.Name,
				Namespace: opsRequest.Namespace,
			},
			Env: insHelper.envForRestore,
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

// createPostReadyRestore creates a Restore to restore the data with postReady stage.
func (r rebuildInstanceOpsHandler) createPostReadyRestore(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRequest *appsv1alpha1.OpsRequest,
	insHelper *instanceHelper,
	restoreName string) error {
	labels := r.getWellKnownLabels(insHelper.synthesizedComp)
	if insHelper.targetPod.Labels[constant.KBAppPodNameLabelKey] == insHelper.targetPod.Name {
		labels[constant.KBAppPodNameLabelKey] = insHelper.targetPod.Name
	}
	podSelector := metav1.LabelSelector{
		MatchLabels: labels,
	}
	// TODO: support to rebuild instance from backup when the PodSelectionStrategy of source target is All .
	restore := &dpv1alpha1.Restore{
		ObjectMeta: r.buildRestoreMetaObject(opsRequest, restoreName),
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:      insHelper.backup.Name,
				Namespace: insHelper.backup.Namespace,
			},
			Env: insHelper.envForRestore,
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
	backupMethod := insHelper.backup.Status.BackupMethod
	if backupMethod.TargetVolumes != nil {
		restore.Spec.ReadyConfig.JobAction.Target.VolumeMounts = backupMethod.TargetVolumes.VolumeMounts
	}
	_ = intctrlutil.SetControllerReference(opsRequest, restore)
	return client.IgnoreAlreadyExists(cli.Create(reqCtx.Ctx, restore))
}

// createTmpPVCsAndPod creates the tmp pvcs and pod.
func (r rebuildInstanceOpsHandler) createTmpPVCsAndPod(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRequest *appsv1alpha1.OpsRequest,
	insHelper *instanceHelper,
	tmpPodName string) error {
	for _, v := range insHelper.pvcMap {
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
		VolumeMounts:    insHelper.volumeMounts,
	}
	intctrlutil.InjectZeroResourcesLimitsIfEmpty(container)
	rebuildPodBuilder := builder.NewPodBuilder(insHelper.targetPod.Namespace, tmpPodName).AddTolerations(insHelper.targetPod.Spec.Tolerations...).
		AddContainer(*container).
		AddVolumes(insHelper.volumes...).
		SetRestartPolicy(corev1.RestartPolicyNever).
		AddLabels(constant.OpsRequestNameLabelKey, opsRequest.Name).
		AddLabels(constant.OpsRequestNamespaceLabelKey, opsRequest.Namespace).
		SetTopologySpreadConstraints(insHelper.targetPod.Spec.TopologySpreadConstraints).
		SetAffinity(insHelper.targetPod.Spec.Affinity)
	if insHelper.instance.TargetNodeName != "" {
		rebuildPodBuilder.SetNodeSelector(map[string]string{
			corev1.LabelHostname: insHelper.instance.TargetNodeName,
		})
	}
	rebuildPod := rebuildPodBuilder.GetObject()
	_ = intctrlutil.SetControllerReference(opsRequest, rebuildPod)
	return client.IgnoreAlreadyExists(cli.Create(reqCtx.Ctx, rebuildPod))
}

// rebuildSourcePVCsAndRecreateInstance rebuilds the source pvcs and recreate the instance by deleting it.
func (r rebuildInstanceOpsHandler) rebuildSourcePVCsAndRecreateInstance(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRequest *appsv1alpha1.OpsRequest,
	progressDetail *appsv1alpha1.ProgressStatusDetail,
	insHelper *instanceHelper) error {
	for sourcePVCName, v := range insHelper.pvcMap {
		tmpPVC := &corev1.PersistentVolumeClaim{}
		_ = cli.Get(reqCtx.Ctx, types.NamespacedName{Name: v.Name, Namespace: v.Namespace}, tmpPVC)
		if tmpPVC.UID == "" {
			// if the tmp pvc not exists in k8s, replace with v.
			tmpPVC = v
		}
		// 1. get the restored pv
		pv, err := r.getRestoredPV(reqCtx, cli, tmpPVC)
		if err != nil {
			return err
		}
		if _, ok := pv.Annotations[rebuildFromAnnotation]; !ok {
			if pv.Labels[rebuildTmpPVCNameLabel] != tmpPVC.Name {
				// 2. retain and label the pv with 'rebuildTmpPVCNameLabel'
				if err = r.retainAndLabelPV(reqCtx, cli, pv, tmpPVC); err != nil {
					return err
				}
			}
			// 3. cleanup the tmp pvc firstly.
			if err = r.cleanupTmpPVC(reqCtx, cli, tmpPVC); err != nil {
				return err
			}
			// 4. release the pv and annotate the pv with 'rebuildFromAnnotation'.
			if err = r.releasePV(reqCtx, cli, pv, opsRequest.Name); err != nil {
				return err
			}
		}
		// set volumeName to tmp pvc, it will be used when recreating the source pvc.
		tmpPVC.Spec.VolumeName = pv.Name
		// 5. recreate the source pbc.
		if err = r.recreateSourcePVC(reqCtx, cli, tmpPVC, sourcePVCName, opsRequest.Name); err != nil {
			return err
		}
	}
	// update progress message and recreate the target instance by deleting it.
	progressDetail.Message = waitingForInstanceReadyMessage
	return intctrlutil.BackgroundDeleteObject(cli, reqCtx.Ctx, insHelper.targetPod)
}

func (r rebuildInstanceOpsHandler) getRestoredPV(reqCtx intctrlutil.RequestCtx,
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

func (r rebuildInstanceOpsHandler) retainAndLabelPV(reqCtx intctrlutil.RequestCtx,
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

func (r rebuildInstanceOpsHandler) cleanupTmpPVC(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	tmpPVC *corev1.PersistentVolumeClaim) error {
	// if the tmp pvc exists, delete it.
	if err := intctrlutil.BackgroundDeleteObject(cli, reqCtx.Ctx, tmpPVC); err != nil {
		return err
	}
	return r.removePVCFinalizer(reqCtx, cli, tmpPVC)
}

func (r rebuildInstanceOpsHandler) recreateSourcePVC(reqCtx intctrlutil.RequestCtx,
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
	if err := r.removePVCFinalizer(reqCtx, cli, sourcePvc); err != nil {
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
func (r rebuildInstanceOpsHandler) releasePV(reqCtx intctrlutil.RequestCtx,
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

func (r rebuildInstanceOpsHandler) removePVCFinalizer(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	pvc *corev1.PersistentVolumeClaim) error {
	patch := client.MergeFrom(pvc.DeepCopy())
	pvc.Finalizers = nil
	return client.IgnoreNotFound(cli.Patch(reqCtx.Ctx, pvc, patch))
}

// instanceIsAvailable checks if the instance is available.
func (r rebuildInstanceOpsHandler) instanceIsAvailable(
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

// cleanupTmpResources clean up the temporary resources generated during the process of rebuilding the instance.
func (r rebuildInstanceOpsHandler) cleanupTmpResources(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource) error {
	matchLabels := client.MatchingLabels{
		constant.OpsRequestNameLabelKey:      opsRes.OpsRequest.Name,
		constant.OpsRequestNamespaceLabelKey: opsRes.OpsRequest.Namespace,
	}
	// TODO: need to delete the restore CR?
	// Pods are limited in k8s, so we need to release them if they are not needed.
	return intctrlutil.DeleteOwnedResources(reqCtx.Ctx, cli, opsRes.OpsRequest, matchLabels, generics.PodSignature)
}
