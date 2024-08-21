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
	"reflect"
	"slices"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlcomp "github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	createPodOperation = "Create"
	deletePodOperation = "Delete"
)

type horizontalScalingOpsHandler struct{}

var _ OpsHandler = horizontalScalingOpsHandler{}

func init() {
	hsHandler := horizontalScalingOpsHandler{}
	horizontalScalingBehaviour := OpsBehaviour{
		// if cluster is Abnormal or Failed, new opsRequest may repair it.
		FromClusterPhases: appsv1alpha1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1alpha1.UpdatingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        hsHandler,
		CancelFunc:        hsHandler.Cancel,
	}
	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.HorizontalScalingType, horizontalScalingBehaviour)
}

// ActionStartedCondition the started condition when handling the horizontal scaling request.
func (hs horizontalScalingOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewHorizontalScalingCondition(opsRes.OpsRequest), nil
}

// Action modifies Cluster.spec.components[*].replicas from the opsRequest
func (hs horizontalScalingOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	if slices.Contains([]appsv1alpha1.ClusterPhase{appsv1alpha1.StoppedClusterPhase,
		appsv1alpha1.StoppingClusterPhase}, opsRes.Cluster.Status.Phase) {
		return intctrlutil.NewFatalError("please start the cluster before scaling the cluster horizontally")
	}
	compOpsSet := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	// abort earlier running horizontal scaling opsRequest.
	if err := abortEarlierOpsRequestWithSameKind(reqCtx, cli, opsRes, []appsv1alpha1.OpsType{appsv1alpha1.HorizontalScalingType, appsv1alpha1.StartType},
		func(earlierOps *appsv1alpha1.OpsRequest) (bool, error) {
			if slices.Contains([]appsv1alpha1.OpsType{appsv1alpha1.StartType, appsv1alpha1.StopType}, earlierOps.Spec.Type) {
				return true, nil
			}
			for _, v := range earlierOps.Spec.HorizontalScalingList {
				compOps, ok := compOpsSet.componentOpsSet[v.ComponentName]
				if !ok {
					return false, nil
				}
				currHorizontalScaling := compOps.(appsv1alpha1.HorizontalScaling)
				// abort the opsRequest for overwrite replicas operation.
				if currHorizontalScaling.Replicas != nil || v.Replicas != nil {
					return true, nil
				}
				// if the earlier opsRequest is pending and not `Overwrite` operator, return false.
				if earlierOps.Status.Phase == appsv1alpha1.OpsPendingPhase {
					return false, nil
				}
				// check if the instance to be taken offline was created by another opsRequest.
				if err := hs.checkIntersectionWithEarlierOps(opsRes, earlierOps, currHorizontalScaling, v); err != nil {
					return false, err
				}
			}
			return false, nil
		}); err != nil {
		return err
	}

	if err := compOpsSet.updateClusterComponentsAndShardings(opsRes.Cluster, func(compSpec *appsv1alpha1.ClusterComponentSpec, obj ComponentOpsInterface) error {
		horizontalScaling := obj.(appsv1alpha1.HorizontalScaling)
		lastCompConfiguration := opsRes.OpsRequest.Status.LastConfiguration.Components[obj.GetComponentName()]
		if horizontalScaling.ScaleIn != nil && len(horizontalScaling.ScaleIn.OnlineInstancesToOffline) > 0 {
			// check if the instances are online.
			currPodSet, err := intctrlcomp.GenerateAllPodNamesToSet(*lastCompConfiguration.Replicas, lastCompConfiguration.Instances, lastCompConfiguration.OfflineInstances,
				opsRes.Cluster.Name, obj.GetComponentName())
			if err != nil {
				return err
			}
			for _, onlineIns := range horizontalScaling.ScaleIn.OnlineInstancesToOffline {
				if _, ok := currPodSet[onlineIns]; !ok {
					return intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" specified in onlineInstancesToOffline is not online`, onlineIns))
				}
			}
		}
		replicas, instances, offlineInstances, err := hs.getExpectedCompValues(opsRes, compSpec.DeepCopy(),
			lastCompConfiguration, horizontalScaling)
		if err != nil {
			return err
		}
		var insReplicas int32
		for _, v := range instances {
			insReplicas += v.GetReplicas()
		}
		if insReplicas > replicas {
			errMsg := fmt.Sprintf(`the total number of replicas for the instance template can not greater than the number of replicas for component "%s" after horizontally scaling`,
				horizontalScaling.ComponentName)
			return intctrlutil.NewFatalError(errMsg)
		}
		compSpec.Replicas = replicas
		compSpec.Instances = instances
		compSpec.OfflineInstances = offlineInstances
		return nil
	}); err != nil {
		return err
	}
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for horizontal scaling opsRequest.
func (hs horizontalScalingOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	handleComponentProgress := func(
		reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes *progressResource,
		compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
		lastCompConfiguration := opsRes.OpsRequest.Status.LastConfiguration.Components[pgRes.compOps.GetComponentName()]
		horizontalScaling := pgRes.compOps.(appsv1alpha1.HorizontalScaling)
		var err error
		pgRes.createdPodSet, pgRes.deletedPodSet, err = hs.getCreateAndDeletePodSet(opsRes, lastCompConfiguration, *pgRes.clusterComponent, horizontalScaling, pgRes.fullComponentName)
		if err != nil {
			return 0, 0, err
		}
		pgRes.noWaitComponentCompleted = true
		return handleComponentProgressForScalingReplicas(reqCtx, cli, opsRes, pgRes, compStatus)
	}
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	return compOpsHelper.reconcileActionWithComponentOps(reqCtx, cli, opsRes, "", handleComponentProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (hs horizontalScalingOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	getLastComponentInfo := func(compSpec appsv1alpha1.ClusterComponentSpec, comOps ComponentOpsInterface) appsv1alpha1.LastComponentConfiguration {
		lastCompConfiguration := appsv1alpha1.LastComponentConfiguration{
			Replicas:         pointer.Int32(compSpec.Replicas),
			Instances:        compSpec.Instances,
			OfflineInstances: compSpec.OfflineInstances,
		}
		return lastCompConfiguration
	}
	compOpsHelper.saveLastConfigurations(opsRes, getLastComponentInfo)
	return nil
}

// getCreateAndDeletePodSet gets the pod set that are created and deleted in this opsRequest.
func (hs horizontalScalingOpsHandler) getCreateAndDeletePodSet(opsRes *OpsResource,
	lastCompConfiguration appsv1alpha1.LastComponentConfiguration,
	currCompSpec appsv1alpha1.ClusterComponentSpec,
	horizontalScaling appsv1alpha1.HorizontalScaling,
	fullCompName string) (map[string]string, map[string]string, error) {
	clusterName := opsRes.Cluster.Name
	lastPodSet, err := intctrlcomp.GenerateAllPodNamesToSet(*lastCompConfiguration.Replicas,
		lastCompConfiguration.Instances, lastCompConfiguration.OfflineInstances, clusterName, fullCompName)
	if err != nil {
		return nil, nil, err
	}
	expectReplicas, expectInstanceTpls, expectOfflineInstances, err := hs.getExpectedCompValues(opsRes, &currCompSpec, lastCompConfiguration, horizontalScaling)
	if err != nil {
		return nil, nil, err
	}
	currPodSet, err := intctrlcomp.GenerateAllPodNamesToSet(expectReplicas, expectInstanceTpls,
		expectOfflineInstances, clusterName, fullCompName)
	if err != nil {
		return nil, nil, err
	}
	createPodSet := map[string]string{}
	deletePodSet := map[string]string{}
	for k := range currPodSet {
		if _, ok := lastPodSet[k]; !ok {
			createPodSet[k] = appsv1alpha1.GetInstanceTemplateName(clusterName, fullCompName, k)
		}
	}
	for k := range lastPodSet {
		if _, ok := currPodSet[k]; !ok {
			deletePodSet[k] = appsv1alpha1.GetInstanceTemplateName(clusterName, fullCompName, k)
		}
	}
	if opsRes.OpsRequest.Status.Phase == appsv1alpha1.OpsCancellingPhase {
		// when cancelling this opsRequest, revert the changes.
		return deletePodSet, createPodSet, nil
	}
	return createPodSet, deletePodSet, nil
}

// Cancel this function defines the cancel horizontalScaling action.
func (hs horizontalScalingOpsHandler) Cancel(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	if err := compOpsHelper.cancelComponentOps(reqCtx.Ctx, cli, opsRes, func(lastConfig *appsv1alpha1.LastComponentConfiguration, comp *appsv1alpha1.ClusterComponentSpec) {
		comp.Replicas = *lastConfig.Replicas
		comp.Instances = lastConfig.Instances
		comp.OfflineInstances = lastConfig.OfflineInstances
	}); err != nil {
		return err
	}
	// delete the running restore resource to release PVC of the pod which will be deleted after cancelling the ops.
	restoreList := &dpv1alpha1.RestoreList{}
	if err := cli.List(reqCtx.Ctx, restoreList, client.InNamespace(opsRes.OpsRequest.Namespace),
		client.MatchingLabels{constant.AppInstanceLabelKey: opsRes.Cluster.Name}); err != nil {
		return err
	}
	for i := range restoreList.Items {
		restore := &restoreList.Items[i]
		if restore.Status.Phase != dpv1alpha1.RestorePhaseRunning {
			continue
		}
		compName := restore.Labels[constant.KBAppComponentLabelKey]
		if _, ok := compOpsHelper.componentOpsSet[compName]; !ok {
			continue
		}
		workloadName := constant.GenerateWorkloadNamePattern(opsRes.Cluster.Name, compName)
		if restore.Spec.Backup.Name != constant.GenerateResourceNameWithScalingSuffix(workloadName) {
			continue
		}
		if err := intctrlutil.BackgroundDeleteObject(cli, reqCtx.Ctx, restore); err != nil {
			return err
		}
		// remove component finalizer
		patch := client.MergeFrom(restore.DeepCopy())
		controllerutil.RemoveFinalizer(restore, constant.DBComponentFinalizerName)
		if err := cli.Patch(reqCtx.Ctx, restore, patch); err != nil {
			return err
		}
	}
	return nil
}

// checkIntersectionWithEarlierOps checks if the pod deleted by the current ops is a pod created by another ops
func (hs horizontalScalingOpsHandler) checkIntersectionWithEarlierOps(opsRes *OpsResource, earlierOps *appsv1alpha1.OpsRequest,
	currOpsHScaling, earlierOpsHScaling appsv1alpha1.HorizontalScaling) error {
	getCreatedOrDeletedPodSet := func(ops *appsv1alpha1.OpsRequest, hScaling appsv1alpha1.HorizontalScaling) (map[string]string, map[string]string, error) {
		lastCompSnapshot := ops.Status.LastConfiguration.Components[earlierOpsHScaling.ComponentName]
		compSpec := getComponentSpecOrShardingTemplate(opsRes.Cluster, earlierOpsHScaling.ComponentName).DeepCopy()
		var err error
		compSpec.Replicas, compSpec.Instances, compSpec.OfflineInstances, err = hs.getExpectedCompValues(opsRes, compSpec, lastCompSnapshot, hScaling)
		if err != nil {
			return nil, nil, err
		}
		return hs.getCreateAndDeletePodSet(opsRes, lastCompSnapshot, *compSpec, hScaling, hScaling.ComponentName)
	}
	createdPodSetForEarlier, deletedPodSetForEarlier, err := getCreatedOrDeletedPodSet(earlierOps, earlierOpsHScaling)
	if err != nil {
		return err
	}
	createdPodSetForCurrent, deletedPodSetForCurrent, err := getCreatedOrDeletedPodSet(opsRes.OpsRequest, currOpsHScaling)
	if err != nil {
		return err
	}
	for deletedPod := range deletedPodSetForCurrent {
		if _, ok := createdPodSetForEarlier[deletedPod]; ok {
			return intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" cannot be deleted as it has been created by another running opsRequest "%s"`,
				deletedPod, earlierOps.Name))
		}
	}
	for createdPod := range createdPodSetForCurrent {
		if _, ok := deletedPodSetForEarlier[createdPod]; ok {
			return intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" cannot be created as it has been deleted by another running opsRequest "%s"`,
				createdPod, earlierOps.Name))
		}
	}
	return nil
}

// getExpectedCompValues gets the expected replicas, instances, offlineInstances.
func (hs horizontalScalingOpsHandler) getExpectedCompValues(
	opsRes *OpsResource,
	compSpec *appsv1alpha1.ClusterComponentSpec,
	lastCompConfiguration appsv1alpha1.LastComponentConfiguration,
	horizontalScaling appsv1alpha1.HorizontalScaling) (int32, []appsv1alpha1.InstanceTemplate, []string, error) {
	compReplicas := compSpec.Replicas
	compInstanceTpls := compSpec.Instances
	compOfflineInstances := compSpec.OfflineInstances
	if horizontalScaling.Replicas == nil {
		compReplicas = *lastCompConfiguration.Replicas
		compInstanceTpls = slices.Clone(lastCompConfiguration.Instances)
		compOfflineInstances = lastCompConfiguration.OfflineInstances
	}
	expectOfflineInstances := hs.getCompExpectedOfflineInstances(compOfflineInstances, horizontalScaling)
	err := hs.autoSyncReplicaChanges(opsRes, horizontalScaling, compReplicas, compInstanceTpls, expectOfflineInstances)
	if err != nil {
		return 0, nil, nil, err
	}
	return hs.getCompExpectReplicas(horizontalScaling, compReplicas),
		hs.getCompExpectedInstances(compInstanceTpls, horizontalScaling),
		expectOfflineInstances, nil
}

// autoSyncReplicaChanges auto-sync the replicaChanges of the component and instance templates.
func (hs horizontalScalingOpsHandler) autoSyncReplicaChanges(
	opsRes *OpsResource,
	horizontalScaling appsv1alpha1.HorizontalScaling,
	compReplicas int32,
	compInstanceTpls []appsv1alpha1.InstanceTemplate,
	compExpectOfflineInstances []string) error {
	// sync the replicaChanges for component and instance template.
	getSyncedInstancesAndReplicaChanges := func(offlineOrOnlineInsCountMap map[string]int32,
		replicaChanger appsv1alpha1.ReplicaChanger,
		newInstances []appsv1alpha1.InstanceTemplate) ([]appsv1alpha1.InstanceReplicasTemplate, *int32) {
		allReplicaChanges := int32(0)
		insTplMap := map[string]sets.Empty{}
		for _, v := range replicaChanger.Instances {
			insTplMap[v.Name] = sets.Empty{}
			allReplicaChanges += v.ReplicaChanges
		}
		for k, v := range offlineOrOnlineInsCountMap {
			if k == "" {
				allReplicaChanges += v
				continue
			}
			if _, ok := insTplMap[k]; !ok {
				replicaChanger.Instances = append(replicaChanger.Instances, appsv1alpha1.InstanceReplicasTemplate{Name: k, ReplicaChanges: v})
				allReplicaChanges += v
			}
		}
		for _, v := range newInstances {
			allReplicaChanges += v.GetReplicas()
		}
		if replicaChanger.ReplicaChanges != nil {
			allReplicaChanges = *replicaChanger.ReplicaChanges
		}
		return replicaChanger.Instances, &allReplicaChanges
	}
	// auto sync the replicaChanges.
	scaleIn := horizontalScaling.ScaleIn
	if scaleIn != nil {
		offlineInsCountMap := opsRes.OpsRequest.CountOfflineOrOnlineInstances(opsRes.Cluster.Name, horizontalScaling.ComponentName, scaleIn.OnlineInstancesToOffline)
		scaleIn.Instances, scaleIn.ReplicaChanges = getSyncedInstancesAndReplicaChanges(offlineInsCountMap, scaleIn.ReplicaChanger, nil)
	}
	scaleOut := horizontalScaling.ScaleOut
	if scaleOut != nil {
		// get the pod set when removing the specified instances from offlineInstances slice
		podSet, err := intctrlcomp.GenerateAllPodNamesToSet(compReplicas, compInstanceTpls, compExpectOfflineInstances,
			opsRes.Cluster.Name, horizontalScaling.ComponentName)
		if err != nil {
			return err
		}
		onlineInsCountMap := map[string]int32{}
		for _, insName := range scaleOut.OfflineInstancesToOnline {
			if _, ok := podSet[insName]; !ok {
				//  if the specified instance will not be created, continue
				continue
			}
			insTplName := appsv1alpha1.GetInstanceTemplateName(opsRes.Cluster.Name, horizontalScaling.ComponentName, insName)
			onlineInsCountMap[insTplName]++
		}
		scaleOut.Instances, scaleOut.ReplicaChanges = getSyncedInstancesAndReplicaChanges(onlineInsCountMap, scaleOut.ReplicaChanger, scaleOut.NewInstances)
	}
	return nil
}

// getCompExpectReplicas gets the expected replicas for the component.
func (hs horizontalScalingOpsHandler) getCompExpectReplicas(horizontalScaling appsv1alpha1.HorizontalScaling,
	compReplicas int32) int32 {
	if horizontalScaling.Replicas != nil {
		return *horizontalScaling.Replicas
	}
	if horizontalScaling.ScaleOut != nil && horizontalScaling.ScaleOut.ReplicaChanges != nil {
		compReplicas += *horizontalScaling.ScaleOut.ReplicaChanges
	}
	if horizontalScaling.ScaleIn != nil && horizontalScaling.ScaleIn.ReplicaChanges != nil {
		compReplicas -= *horizontalScaling.ScaleIn.ReplicaChanges
	}
	return compReplicas
}

// getCompExpectedOfflineInstances gets the expected instance templates of the component.
func (hs horizontalScalingOpsHandler) getCompExpectedInstances(
	compInstanceTpls []appsv1alpha1.InstanceTemplate,
	horizontalScaling appsv1alpha1.HorizontalScaling,
) []appsv1alpha1.InstanceTemplate {
	if horizontalScaling.Replicas != nil {
		return compInstanceTpls
	}
	compInsTplSet := map[string]int{}
	for i := range compInstanceTpls {
		compInsTplSet[compInstanceTpls[i].Name] = i
	}
	handleInstanceTplReplicaChanges := func(instances []appsv1alpha1.InstanceReplicasTemplate, isScaleIn bool) {
		for _, v := range instances {
			compInsIndex, ok := compInsTplSet[v.Name]
			if !ok {
				continue
			}
			if isScaleIn {
				compInstanceTpls[compInsIndex].Replicas = pointer.Int32(compInstanceTpls[compInsIndex].GetReplicas() - v.ReplicaChanges)
			} else {
				compInstanceTpls[compInsIndex].Replicas = pointer.Int32(compInstanceTpls[compInsIndex].GetReplicas() + v.ReplicaChanges)
			}
		}
	}
	if horizontalScaling.ScaleOut != nil {
		compInstanceTpls = append(compInstanceTpls, horizontalScaling.ScaleOut.NewInstances...)
		handleInstanceTplReplicaChanges(horizontalScaling.ScaleOut.Instances, false)
	}
	if horizontalScaling.ScaleIn != nil {
		handleInstanceTplReplicaChanges(horizontalScaling.ScaleIn.Instances, true)
	}
	return compInstanceTpls
}

// getCompExpectedOfflineInstances gets the expected offlineInstances of the component.
func (hs horizontalScalingOpsHandler) getCompExpectedOfflineInstances(
	compOfflineInstances []string,
	horizontalScaling appsv1alpha1.HorizontalScaling,
) []string {
	handleOfflineInstances := func(baseInstanceNames, comparedInstanceNames, newOfflineInstances []string) []string {
		instanceNameSet := sets.New(comparedInstanceNames...)
		for _, instanceName := range baseInstanceNames {
			if _, ok := instanceNameSet[instanceName]; !ok {
				newOfflineInstances = append(newOfflineInstances, instanceName)
			}
		}
		return newOfflineInstances
	}
	if horizontalScaling.ScaleIn != nil && len(horizontalScaling.ScaleIn.OnlineInstancesToOffline) > 0 {
		compOfflineInstances = handleOfflineInstances(horizontalScaling.ScaleIn.OnlineInstancesToOffline, compOfflineInstances, compOfflineInstances)
	}
	if horizontalScaling.ScaleOut != nil && len(horizontalScaling.ScaleOut.OfflineInstancesToOnline) > 0 {
		compOfflineInstances = handleOfflineInstances(compOfflineInstances, horizontalScaling.ScaleOut.OfflineInstancesToOnline, make([]string, 0))
	}
	return compOfflineInstances
}

// HandleAbortForHScale handling the aborted horizontal scaling.
func HandleAbortForHScale(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	var (
		needUpdateCluster bool
		err               error
		ops               = opsRes.OpsRequest
		opsDeepCopy       = ops.DeepCopy()
	)
	if ops.Annotations[constant.RetryAbortedHScaleAnnoKey] == "true" {
		if needUpdateCluster, err = handleClusterCompSpecForAbortedHScale(opsRes, appsv1alpha1.AbortedProgressStatus,
			appsv1alpha1.PendingProgressStatus, true); needUpdateCluster {
			ops.Status.Phase = appsv1alpha1.OpsRunningPhase
			// enqueue the opsRequest
			opsRequestSlice, err := opsutil.GetOpsRequestSliceFromCluster(opsRes.Cluster)
			if err != nil {
				return err
			}
			if opsRequestSlice == nil {
				opsRequestSlice = make([]appsv1alpha1.OpsRecorder, 0)
			}
			if index, _ := GetOpsRecorderFromSlice(opsRequestSlice, opsRes.OpsRequest.Name); index == -1 {
				opsRequestSlice = append(opsRequestSlice, appsv1alpha1.OpsRecorder{
					Name: opsRes.OpsRequest.Name,
					Type: opsRes.OpsRequest.Spec.Type,
				})
				opsutil.SetOpsRequestToCluster(opsRes.Cluster, opsRequestSlice)
			}
		}
		reqCtx.Log.Info(fmt.Sprintf("retry to run the aborted horizontal scaling opsRequest, update cluster's replicas: %t", needUpdateCluster))
	} else {
		needUpdateCluster, err = handleClusterCompSpecForAbortedHScale(opsRes, appsv1alpha1.PendingProgressStatus,
			appsv1alpha1.AbortedProgressStatus, false)
		reqCtx.Log.Info(fmt.Sprintf("abort the horizontal scaling opsRequest, update cluster's replicas: %t", needUpdateCluster))
	}
	if err != nil {
		return err
	}
	if needUpdateCluster {
		if err = cli.Update(reqCtx.Ctx, opsRes.Cluster); err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(opsDeepCopy.Status, ops.Status) {
		if err = cli.Status().Patch(reqCtx.Ctx, ops, client.MergeFrom(opsDeepCopy)); err != nil {
			return err
		}
	}
	if ops.Annotations[constant.RetryAbortedHScaleAnnoKey] == "true" {
		ops.Annotations[constant.RetryAbortedHScaleAnnoKey] = "done"
	}
	if !reflect.DeepEqual(opsDeepCopy.Annotations, ops.Annotations) {
		return cli.Patch(reqCtx.Ctx, ops, client.MergeFrom(opsDeepCopy))
	}
	return nil
}

func handleClusterCompSpecForAbortedHScale(opsRes *OpsResource, expectProgressStatus, toProgressStatus appsv1alpha1.ProgressStatus, reverse bool) (bool, error) {
	needUpdateCluster := false
	// update component and instance template replicas
	updateCompReplicas := func(compSpec *appsv1alpha1.ClusterComponentSpec, tplName string, operation func(replicas int32) int32) {
		compSpec.Replicas = operation(compSpec.Replicas)
		if tplName != "" {
			for k := range compSpec.Instances {
				instanceTPL := &compSpec.Instances[k]
				if instanceTPL.Name == tplName {
					instanceTPL.Replicas = pointer.Int32(operation(instanceTPL.GetReplicas()))
				}
			}
		}
	}
	// offline the required instances
	offlineInstance := func(compSpec *appsv1alpha1.ClusterComponentSpec, tplName, podName string) {
		compSpec.OfflineInstances = append(compSpec.OfflineInstances, podName)
		updateCompReplicas(compSpec, tplName, func(replicas int32) int32 {
			return replicas - int32(1)
		})
	}
	isCreateOperation := func(progressGroup, compName string) bool {
		return (progressGroup == fmt.Sprintf("%s/%s", compName, createPodOperation) && !reverse) ||
			(progressGroup == fmt.Sprintf("%s/%s", compName, deletePodOperation) && reverse)
	}
	needReCreatedPodMap := map[string]*appsv1alpha1.ProgressStatusDetail{}
	clusterCompMap := map[string]*appsv1alpha1.ClusterComponentSpec{}
	for i, comp := range opsRes.Cluster.Spec.ComponentSpecs {
		clusterCompMap[comp.Name] = &opsRes.Cluster.Spec.ComponentSpecs[i]
	}
	for compName := range opsRes.OpsRequest.Status.Components {
		compSpec, ok := clusterCompMap[compName]
		if !ok {
			continue
		}
		compStatus := opsRes.OpsRequest.Status.Components[compName]
		for j := range compStatus.ProgressDetails {
			pd := &compStatus.ProgressDetails[j]
			if pd.Status != expectProgressStatus {
				continue
			}
			currExpectPodSet, err := intctrlcomp.GenerateAllPodNamesToSet(compSpec.Replicas, compSpec.Instances,
				compSpec.OfflineInstances, opsRes.Cluster.Name, compName)
			if err != nil {
				return false, err
			}
			podName := strings.Replace(pd.ObjectKey, constant.PodKind+"/", "", 1)
			tplName := appsv1alpha1.GetInstanceTemplateName(opsRes.Cluster.Name, compSpec.Name, podName)
			if isCreateOperation(pd.Group, compName) {
				if _, ok = currExpectPodSet[podName]; ok {
					// offline the pods that have not been created yet during scaling out.
					offlineInstance(compSpec, tplName, podName)
					needUpdateCluster = true
					pd.Status = toProgressStatus
				}
				continue
			}
			// if the pods that have not been deleted during scaling in, only add the replicas. and need to check
			// the pod if it can be re-created.
			if _, ok = currExpectPodSet[podName]; !ok {
				needReCreatedPodMap[podName] = pd
				updateCompReplicas(compSpec, tplName, func(replicas int32) int32 {
					return replicas + int32(1)
				})
			}
		}
		if len(needReCreatedPodMap) > 0 {
			var currOfflineInstances []string
			for _, podName := range compSpec.OfflineInstances {
				if _, ok = needReCreatedPodMap[podName]; !ok {
					currOfflineInstances = append(currOfflineInstances, podName)
				}
			}
			compSpec.OfflineInstances = currOfflineInstances
			expectPodSetAfterAborted, err := intctrlcomp.GenerateAllPodNamesToSet(compSpec.Replicas, compSpec.Instances,
				compSpec.OfflineInstances, opsRes.Cluster.Name, compName)
			if err != nil {
				return false, err
			}
			for podName := range needReCreatedPodMap {
				// check if the pod can be re-created, if not, replicas-1.
				if tplName, ok := expectPodSetAfterAborted[podName]; !ok {
					offlineInstance(compSpec, tplName, podName)
				} else {
					needUpdateCluster = true
					needReCreatedPodMap[podName].Status = toProgressStatus
				}
			}
		}
	}
	return needUpdateCluster, nil
}
