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
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

const (
	scalingOutPodPrefixMsg    = "Scaling out a new pod"
	reasonCompReplicasChanged = "ComponentReplicasChanged"
)

type rebuildInstanceWrapper struct {
	replicas int32
	insNames []string
}

type rebuildInstanceOpsHandler struct{}

var _ OpsHandler = rebuildInstanceOpsHandler{}

func init() {
	rebuildInstanceBehaviour := OpsBehaviour{
		FromClusterPhases: []appsv1.ClusterPhase{appsv1.AbnormalClusterPhase, appsv1.FailedClusterPhase, appsv1.UpdatingClusterPhase},
		ToClusterPhase:    appsv1.UpdatingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        rebuildInstanceOpsHandler{},
	}
	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.RebuildInstanceType, rebuildInstanceBehaviour)
}

// ActionStartedCondition the started condition when handle the rebuild-instance request.
func (r rebuildInstanceOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewInstancesRebuildingCondition(opsRes.OpsRequest), nil
}

func (r rebuildInstanceOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	for _, v := range opsRes.OpsRequest.Spec.RebuildFrom {
		compStatus, ok := opsRes.Cluster.Status.Components[v.ComponentName]
		if !ok {
			continue
		}
		// check if the component has matched the `Phase` condition
		if !opsRes.OpsRequest.Spec.Force && !slices.Contains([]appsv1.ClusterComponentPhase{appsv1.FailedClusterCompPhase,
			appsv1.UpdatingClusterCompPhase}, compStatus.Phase) {
			return intctrlutil.NewFatalError(fmt.Sprintf(`the phase of component "%s" can not be %s`, v.ComponentName, compStatus.Phase))
		}
		var (
			synthesizedComp *component.SynthesizedComponent
			err             error
			instanceNames   []string
		)
		for _, ins := range v.Instances {
			targetPod := &corev1.Pod{}
			if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: ins.Name, Namespace: opsRes.Cluster.Namespace}, targetPod); err != nil {
				return err
			}
			synthesizedComp, err = r.buildSynthesizedComponent(reqCtx.Ctx, cli, opsRes.Cluster, targetPod.Labels[constant.KBAppComponentLabelKey])
			if err != nil {
				return err
			}
			isAvailable, _ := instanceIsAvailable(synthesizedComp, targetPod, "")
			if !opsRes.OpsRequest.Spec.Force && isAvailable {
				return intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" is availabled, can not rebuild it`, ins.Name))
			}
			instanceNames = append(instanceNames, ins.Name)
		}
		if len(v.Instances) > 0 && !v.InPlace {
			if synthesizedComp.Name != v.ComponentName {
				return intctrlutil.NewFatalError("sharding cluster only supports to rebuild instance in place")
			}
			// validate when rebuilding instance with horizontal scaling
			if err = r.validateRebuildInstanceWithHScale(reqCtx, cli, opsRes, synthesizedComp, instanceNames); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r rebuildInstanceOpsHandler) validateRebuildInstanceWithHScale(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	synthesizedComp *component.SynthesizedComponent,
	instanceNames []string) error {
	// rebuild instance by horizontal scaling
	pods, err := component.ListOwnedPods(reqCtx.Ctx, cli, opsRes.Cluster.Namespace, opsRes.Cluster.Name, synthesizedComp.Name)
	if err != nil {
		return err
	}
	for _, v := range pods {
		if slices.Contains(instanceNames, v.Name) {
			continue
		}
		available, _ := instanceIsAvailable(synthesizedComp, v, "")
		if available {
			return nil
		}
	}
	return intctrlutil.NewFatalError("Due to insufficient available instances, cannot create a new pod for rebuilding instance. " +
		"may you can rebuild instances in place with backup by set 'inPlace' to 'true'.")
}

func (r rebuildInstanceOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.RebuildFrom)
	getLastComponentInfo := func(compSpec appsv1.ClusterComponentSpec, comOps ComponentOpsInterface) opsv1alpha1.LastComponentConfiguration {
		lastCompConfiguration := opsv1alpha1.LastComponentConfiguration{
			Replicas:         pointer.Int32(compSpec.Replicas),
			Instances:        compSpec.Instances,
			OfflineInstances: compSpec.OfflineInstances,
		}
		return lastCompConfiguration
	}
	compOpsHelper.saveLastConfigurations(opsRes, getLastComponentInfo)
	return nil
}

func (r rebuildInstanceOpsHandler) getInstanceProgressDetail(compStatus opsv1alpha1.OpsRequestComponentStatus, instance string) opsv1alpha1.ProgressStatusDetail {
	objectKey := getProgressObjectKey(constant.PodKind, instance)
	progressDetail := findStatusProgressDetail(compStatus.ProgressDetails, objectKey)
	if progressDetail != nil {
		return *progressDetail
	}
	return opsv1alpha1.ProgressStatusDetail{
		ObjectKey: objectKey,
		Status:    opsv1alpha1.ProcessingProgressStatus,
		Message:   fmt.Sprintf("Start to rebuild pod %s", instance),
	}
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for restart opsRequest.
func (r rebuildInstanceOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	var (
		oldOpsRequest   = opsRes.OpsRequest.DeepCopy()
		oldCluster      = opsRes.Cluster.DeepCopy()
		opsRequestPhase = opsRes.OpsRequest.Status.Phase
		expectCount     int
		completedCount  int
		failedCount     int
		err             error
	)
	if opsRes.OpsRequest.Status.Components == nil {
		opsRes.OpsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{}
	}
	for _, v := range opsRes.OpsRequest.Spec.RebuildFrom {
		compStatus := opsRes.OpsRequest.Status.Components[v.ComponentName]
		var (
			subCompletedCount int
			subFailedCount    int
		)
		if v.InPlace {
			// rebuild instances in place.
			if subCompletedCount, subFailedCount, err = r.rebuildInstancesInPlace(reqCtx, cli, opsRes, v, &compStatus); err != nil {
				return opsRequestPhase, 0, err
			}
		} else {
			// rebuild instances with horizontal scaling
			if subCompletedCount, subFailedCount, err = r.rebuildInstancesWithHScaling(reqCtx, cli, opsRes, v, &compStatus); err != nil {
				if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
					return opsv1alpha1.OpsFailedPhase, 0, err
				}
				return opsRequestPhase, 0, err
			}
		}
		expectCount += len(v.Instances)
		completedCount += subCompletedCount
		failedCount += subFailedCount
		opsRes.OpsRequest.Status.Components[v.ComponentName] = compStatus
	}
	if !reflect.DeepEqual(oldCluster.Spec, opsRes.Cluster.Spec) {
		if err = cli.Update(reqCtx.Ctx, opsRes.Cluster); err != nil {
			return opsRequestPhase, 0, err
		}
	}
	if err = syncProgressToOpsRequest(reqCtx, cli, opsRes, oldOpsRequest, completedCount, expectCount); err != nil {
		return opsRequestPhase, 0, err
	}
	// check if the ops has been finished.
	if completedCount != expectCount {
		return opsRequestPhase, 0, nil
	}
	if failedCount == 0 {
		return opsv1alpha1.OpsSucceedPhase, 0, r.cleanupTmpResources(reqCtx, cli, opsRes)
	}
	return opsv1alpha1.OpsFailedPhase, 0, nil
}

func (r rebuildInstanceOpsHandler) rebuildInstancesInPlace(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	rebuildInstance opsv1alpha1.RebuildInstance,
	compStatus *opsv1alpha1.OpsRequestComponentStatus) (int, int, error) {
	// rebuild instances in place.
	var (
		completedCount int
		failedCount    int
	)
	for i, instance := range rebuildInstance.Instances {
		progressDetail := r.getInstanceProgressDetail(*compStatus, instance.Name)
		if isCompletedProgressStatus(progressDetail.Status) {
			completedCount += 1
			if progressDetail.Status == opsv1alpha1.FailedProgressStatus {
				failedCount += 1
			}
			continue
		}
		// rebuild instance
		completed, err := r.rebuildInstanceInPlace(reqCtx, cli, opsRes, &progressDetail, rebuildInstance, instance, i)
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
			// If a fatal error occurs, this instance rebuilds failed.
			progressDetail.SetStatusAndMessage(opsv1alpha1.FailedProgressStatus, err.Error())
			setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
			continue
		}
		if err != nil {
			return 0, 0, err
		}
		if completed {
			// if the pod has been rebuilt, set progressDetail phase to Succeed.
			progressDetail.SetStatusAndMessage(opsv1alpha1.SucceedProgressStatus,
				fmt.Sprintf("Rebuild pod %s successfully", instance.Name))
		}
		setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
	}
	return completedCount, failedCount, nil
}

// rebuildInstance rebuilds the instance.
func (r rebuildInstanceOpsHandler) rebuildInstanceInPlace(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	progressDetail *opsv1alpha1.ProgressStatusDetail,
	rebuildFrom opsv1alpha1.RebuildInstance,
	instance opsv1alpha1.Instance,
	index int) (bool, error) {
	inPlaceHelper, err := r.prepareInplaceRebuildHelper(reqCtx, cli, opsRes, rebuildFrom.RestoreEnv,
		instance, rebuildFrom.BackupName, index)
	if err != nil {
		return false, err
	}

	if rebuildFrom.BackupName == "" {
		return inPlaceHelper.rebuildInstanceWithNoBackup(reqCtx, cli, opsRes, progressDetail)
	}
	return inPlaceHelper.rebuildInstanceWithBackup(reqCtx, cli, opsRes, progressDetail)
}

func (r rebuildInstanceOpsHandler) rebuildInstancesWithHScaling(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	rebuildInstance opsv1alpha1.RebuildInstance,
	compStatus *opsv1alpha1.OpsRequestComponentStatus) (int, int, error) {
	var (
		completedCount int
		failedCount    int
		err            error
	)
	if len(compStatus.ProgressDetails) == 0 {
		// 1. scale out the required instances
		err := r.scaleOutRequiredInstances(reqCtx, cli, opsRes, rebuildInstance, compStatus)
		return 0, 0, err
	}
	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		compSpec := &opsRes.Cluster.Spec.ComponentSpecs[i]
		if compSpec.Name != rebuildInstance.ComponentName {
			continue
		}
		// 2. check if the new pods are available.
		var instancesNeedToOffline []string
		if completedCount, failedCount, instancesNeedToOffline, err = r.checkProgressForScalingOutPods(reqCtx,
			cli, opsRes, rebuildInstance, compSpec, compStatus); err != nil {
			return 0, 0, err
		}

		if len(instancesNeedToOffline) > 0 {
			// 3. offline the instances that require rebuilding when the new pod successfully scales out.
			r.offlineSpecifiedInstances(compSpec, opsRes.Cluster.Name, instancesNeedToOffline)
		}
		break
	}
	return completedCount, failedCount, nil
}

func (r rebuildInstanceOpsHandler) scaleOutRequiredInstances(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	rebuildInstance opsv1alpha1.RebuildInstance,
	compStatus *opsv1alpha1.OpsRequestComponentStatus) error {
	// 1. sort the instances
	slices.SortFunc(rebuildInstance.Instances, func(a, b opsv1alpha1.Instance) bool {
		return a.Name < b.Name
	})

	// 2. assemble the corresponding replicas and instances based on the template
	rebuildInsWrapper := r.getRebuildInstanceWrapper(opsRes, rebuildInstance)

	compName := rebuildInstance.ComponentName
	lastCompConfiguration := opsRes.OpsRequest.Status.LastConfiguration.Components[compName]

	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		compSpec := &opsRes.Cluster.Spec.ComponentSpecs[i]
		if compSpec.Name != compName {
			continue
		}
		if *lastCompConfiguration.Replicas != compSpec.Replicas {
			// means the componentSpec has been updated, ignore it.
			opsRes.Recorder.Eventf(opsRes.OpsRequest, corev1.EventTypeWarning, reasonCompReplicasChanged, "then replicas of the component %s has been changed", compName)
			continue
		}
		return r.scaleOutCompReplicasAndSyncProgress(reqCtx, cli, opsRes, compSpec, rebuildInstance, compStatus, rebuildInsWrapper)
	}
	return nil
}

// getRebuildInstanceWrapper assembles the corresponding replicas and instances based on the template
func (r rebuildInstanceOpsHandler) getRebuildInstanceWrapper(opsRes *OpsResource, rebuildInstance opsv1alpha1.RebuildInstance) map[string]*rebuildInstanceWrapper {
	rebuildInsWrapper := map[string]*rebuildInstanceWrapper{}
	for _, ins := range rebuildInstance.Instances {
		insTplName := appsv1.GetInstanceTemplateName(opsRes.Cluster.Name, rebuildInstance.ComponentName, ins.Name)
		if _, ok := rebuildInsWrapper[insTplName]; !ok {
			rebuildInsWrapper[insTplName] = &rebuildInstanceWrapper{replicas: 1, insNames: []string{ins.Name}}
		} else {
			rebuildInsWrapper[insTplName].replicas += 1
			rebuildInsWrapper[insTplName].insNames = append(rebuildInsWrapper[insTplName].insNames, ins.Name)
		}
	}
	return rebuildInsWrapper
}

func (r rebuildInstanceOpsHandler) scaleOutCompReplicasAndSyncProgress(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	compSpec *appsv1.ClusterComponentSpec,
	rebuildInstance opsv1alpha1.RebuildInstance,
	compStatus *opsv1alpha1.OpsRequestComponentStatus,
	rebuildInsWrapper map[string]*rebuildInstanceWrapper) error {
	scaleOutInsMap := map[string]string{}
	setScaleOutInsMap := func(workloadName, templateName string,
		replicas int32, offlineInstances []string, wrapper *rebuildInstanceWrapper) {
		insNames, _ := instanceset.GenerateInstanceNamesFromTemplate(workloadName, "", replicas, offlineInstances, nil)
		for i, insName := range wrapper.insNames {
			scaleOutInsMap[insName] = insNames[int(replicas-wrapper.replicas)+i]
		}
	}
	// update component spec to scale out required instances.
	workloadName := constant.GenerateWorkloadNamePattern(opsRes.Cluster.Name, compSpec.Name)
	var allTemplateReplicas int32
	for j := range compSpec.Instances {
		insTpl := &compSpec.Instances[j]
		if wrapper, ok := rebuildInsWrapper[insTpl.Name]; ok {
			insTpl.Replicas = pointer.Int32(insTpl.GetReplicas() + wrapper.replicas)
			setScaleOutInsMap(workloadName, insTpl.Name, *insTpl.Replicas, compSpec.OfflineInstances, wrapper)
		}
		allTemplateReplicas += insTpl.GetReplicas()
	}
	compSpec.Replicas += int32(len(rebuildInstance.Instances))
	if wrapper, ok := rebuildInsWrapper[""]; ok {
		setScaleOutInsMap(workloadName, "", compSpec.Replicas-allTemplateReplicas, compSpec.OfflineInstances, wrapper)
	}

	its := &workloads.InstanceSet{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: workloadName, Namespace: opsRes.OpsRequest.Namespace}, its); err != nil {
		return err
	}
	itsUpdated := false
	for _, ins := range rebuildInstance.Instances {
		// set progress details
		scaleOutInsName := scaleOutInsMap[ins.Name]
		setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails,
			opsv1alpha1.ProgressStatusDetail{
				ObjectKey: getProgressObjectKey(constant.PodKind, ins.Name),
				Status:    opsv1alpha1.ProcessingProgressStatus,
				Message:   r.buildScalingOutPodMessage(scaleOutInsName, "Processing"),
			})

		// specify node to scale out
		if ins.TargetNodeName != "" {
			if err := instanceset.MergeNodeSelectorOnceAnnotation(its, map[string]string{scaleOutInsName: ins.TargetNodeName}); err != nil {
				return err
			}
			itsUpdated = true
		}
	}

	if itsUpdated {
		if err := cli.Update(reqCtx.Ctx, its); err != nil {
			return err
		}
	}
	return nil
}

// checkProgressForScalingOutPods checks if the new pods are available.
func (r rebuildInstanceOpsHandler) checkProgressForScalingOutPods(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	rebuildInstance opsv1alpha1.RebuildInstance,
	compSpec *appsv1.ClusterComponentSpec,
	compStatus *opsv1alpha1.OpsRequestComponentStatus) (int, int, []string, error) {
	var (
		instancesNeedToOffline []string
		failedCount            int
		completedCount         int
	)
	synthesizedComp, err := r.buildSynthesizedComponent(reqCtx.Ctx, cli, opsRes.Cluster, rebuildInstance.ComponentName)
	if err != nil {
		return 0, 0, nil, err
	}
	currPodSet, _ := component.GenerateAllPodNamesToSet(compSpec.Replicas, compSpec.Instances, compSpec.OfflineInstances,
		opsRes.Cluster.Name, compSpec.Name)
	for _, instance := range rebuildInstance.Instances {
		progressDetail := r.getInstanceProgressDetail(*compStatus, instance.Name)
		scalingOutPodName := r.getScalingOutPodNameFromMessage(progressDetail.Message)
		if _, ok := currPodSet[scalingOutPodName]; !ok {
			return 0, 0, nil, intctrlutil.NewFatalError(fmt.Sprintf(`the replicas of the component "%s" has been modifeied by another operation`, compSpec.Name))
		}
		pod := &corev1.Pod{}
		if exist, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, cli,
			client.ObjectKey{Name: scalingOutPodName, Namespace: opsRes.Cluster.Namespace}, pod); err != nil {
			return 0, 0, nil, err
		} else if !exist {
			reqCtx.Log.Info(fmt.Sprintf("waiting to create the pod %s", scalingOutPodName))
			continue
		}
		isAvailable, err := instanceIsAvailable(synthesizedComp, pod, opsRes.OpsRequest.Annotations[ignoreRoleCheckAnnotationKey])
		if err != nil {
			// set progress status to failed when new pod is failed
			failedCount += 1
			completedCount += 1
			progressDetail.SetStatusAndMessage(opsv1alpha1.FailedProgressStatus,
				r.buildScalingOutPodMessage(scalingOutPodName, string(opsv1alpha1.UnavailablePhase)))
			setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
			continue
		}
		if !isAvailable {
			// wait for the pod to be available
			continue
		}
		if slices.Contains(compSpec.OfflineInstances, instance.Name) {
			pod = &corev1.Pod{}
			exist, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, cli,
				client.ObjectKey{Name: instance.Name, Namespace: opsRes.Cluster.Namespace}, pod)
			if err != nil {
				return 0, 0, nil, err
			}
			if !exist {
				// f the pod that needs to be rebuilt is not found, and the new pod is available,
				// it indicates that the rebuild process has been completed.
				completedCount += 1
				progressDetail.SetStatusAndMessage(opsv1alpha1.SucceedProgressStatus,
					r.buildScalingOutPodMessage(scalingOutPodName, string(opsv1alpha1.AvailablePhase)))
			} else {
				progressDetail.SetStatusAndMessage(opsv1alpha1.ProcessingProgressStatus,
					r.buildScalingOutPodMessage(scalingOutPodName, string(opsv1alpha1.AvailablePhase)))
				if !pod.DeletionTimestamp.IsZero() && opsRes.OpsRequest.Force() {
					// delete the pod forcibly
					_ = intctrlutil.BackgroundDeleteObject(cli, reqCtx.Ctx, pod, client.GracePeriodSeconds(0))
				}
			}
			setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
		} else {
			instancesNeedToOffline = append(instancesNeedToOffline, instance.Name)
		}
	}
	return completedCount, failedCount, instancesNeedToOffline, nil
}

// offlineSpecifiedInstances to take the specific instances offline.
func (r rebuildInstanceOpsHandler) offlineSpecifiedInstances(compSpec *appsv1.ClusterComponentSpec, clusterName string, instancesNeedToOffline []string) {
	for _, insName := range instancesNeedToOffline {
		compSpec.OfflineInstances = append(compSpec.OfflineInstances, insName)
		templateName := appsv1.GetInstanceTemplateName(clusterName, compSpec.Name, insName)
		if templateName == constant.EmptyInsTemplateName {
			continue
		}
		for j := range compSpec.Instances {
			instanceTpl := &compSpec.Instances[j]
			if instanceTpl.Name == templateName {
				instanceTpl.Replicas = pointer.Int32(instanceTpl.GetReplicas() - 1)
			}
		}
	}
	compSpec.Replicas -= int32(len(instancesNeedToOffline))
}

func (r rebuildInstanceOpsHandler) buildScalingOutPodMessage(scaleOutPodName string, status string) string {
	return fmt.Sprintf("%s: %s, status: %s", scalingOutPodPrefixMsg, scaleOutPodName, status)
}

func (r rebuildInstanceOpsHandler) getScalingOutPodNameFromMessage(progressMsg string) string {
	if !strings.HasPrefix(progressMsg, scalingOutPodPrefixMsg) {
		return ""
	}
	strArr := strings.Split(progressMsg, ",")
	return strings.Replace(strArr[0], scalingOutPodPrefixMsg+": ", "", 1)
}

func (r rebuildInstanceOpsHandler) buildSynthesizedComponent(ctx context.Context,
	cli client.Client,
	cluster *appsv1.Cluster,
	componentName string) (*component.SynthesizedComponent, error) {
	comp, compDef, err := component.GetCompNCompDefByName(ctx, cli, cluster.Namespace, constant.GenerateClusterComponentName(cluster.Name, componentName))
	if err != nil {
		return nil, err
	}
	return component.BuildSynthesizedComponent(ctx, cli, compDef, comp, cluster)
}

func (r rebuildInstanceOpsHandler) prepareInplaceRebuildHelper(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	envForRestore []corev1.EnvVar,
	instance opsv1alpha1.Instance,
	backupName string,
	index int) (*inplaceRebuildHelper, error) {
	var (
		backup          *dpv1alpha1.Backup
		actionSet       *dpv1alpha1.ActionSet
		synthesizedComp *component.SynthesizedComponent
		err             error
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
	synthesizedComp, err = r.buildSynthesizedComponent(reqCtx.Ctx, cli, opsRes.Cluster, targetPod.Labels[constant.KBAppComponentLabelKey])
	if err != nil {
		return nil, err
	}
	rebuildPrefix := fmt.Sprintf("rebuild-%s", opsRes.OpsRequest.UID[:8])
	pvcMap, volumes, volumeMounts, err := getPVCMapAndVolumes(opsRes, synthesizedComp, targetPod, rebuildPrefix, index)
	if err != nil {
		return nil, err
	}
	return &inplaceRebuildHelper{
		index:           index,
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
