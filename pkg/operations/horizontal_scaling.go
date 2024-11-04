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
	"time"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	intctrlcomp "github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type horizontalScalingOpsHandler struct{}

var _ OpsHandler = horizontalScalingOpsHandler{}

func init() {
	hsHandler := horizontalScalingOpsHandler{}
	horizontalScalingBehaviour := OpsBehaviour{
		// if cluster is Abnormal or Failed, new opsRequest may repair it.
		FromClusterPhases: appsv1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1.UpdatingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        hsHandler,
		CancelFunc:        hsHandler.Cancel,
	}
	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.HorizontalScalingType, horizontalScalingBehaviour)
}

// ActionStartedCondition the started condition when handling the horizontal scaling request.
func (hs horizontalScalingOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewHorizontalScalingCondition(opsRes.OpsRequest), nil
}

// Action modifies Cluster.spec.components[*].replicas from the opsRequest
func (hs horizontalScalingOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	if slices.Contains([]appsv1.ClusterPhase{appsv1.StoppedClusterPhase,
		appsv1.StoppingClusterPhase}, opsRes.Cluster.Status.Phase) {
		return intctrlutil.NewFatalError("please start the cluster before scaling the cluster horizontally")
	}
	compOpsSet := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	// abort earlier running horizontal scaling opsRequest.
	if err := abortEarlierOpsRequestWithSameKind(reqCtx, cli, opsRes, []opsv1alpha1.OpsType{opsv1alpha1.HorizontalScalingType, opsv1alpha1.StartType},
		func(earlierOps *opsv1alpha1.OpsRequest) (bool, error) {
			if slices.Contains([]opsv1alpha1.OpsType{opsv1alpha1.StartType, opsv1alpha1.StopType}, earlierOps.Spec.Type) {
				return true, nil
			}
			for _, v := range earlierOps.Spec.HorizontalScalingList {
				compOps, ok := compOpsSet.componentOpsSet[v.ComponentName]
				if !ok {
					return false, nil
				}
				currHorizontalScaling := compOps.(opsv1alpha1.HorizontalScaling)
				// if the earlier opsRequest is pending and not `Overwrite` operator, return false.
				if earlierOps.Status.Phase == opsv1alpha1.OpsPendingPhase {
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

	if err := compOpsSet.updateClusterComponentsAndShardings(opsRes.Cluster, func(compSpec *appsv1.ClusterComponentSpec, obj ComponentOpsInterface) error {
		horizontalScaling := obj.(opsv1alpha1.HorizontalScaling)
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
func (hs horizontalScalingOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	handleComponentProgress := func(
		reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes *progressResource,
		compStatus *opsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
		lastCompConfiguration := opsRes.OpsRequest.Status.LastConfiguration.Components[pgRes.compOps.GetComponentName()]
		horizontalScaling := pgRes.compOps.(opsv1alpha1.HorizontalScaling)
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

// getCreateAndDeletePodSet gets the pod set that are created and deleted in this opsRequest.
func (hs horizontalScalingOpsHandler) getCreateAndDeletePodSet(opsRes *OpsResource,
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	currCompSpec appsv1.ClusterComponentSpec,
	horizontalScaling opsv1alpha1.HorizontalScaling,
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
			createPodSet[k] = appsv1.GetInstanceTemplateName(clusterName, fullCompName, k)
		}
	}
	for k := range lastPodSet {
		if _, ok := currPodSet[k]; !ok {
			deletePodSet[k] = appsv1.GetInstanceTemplateName(clusterName, fullCompName, k)
		}
	}
	if opsRes.OpsRequest.Status.Phase == opsv1alpha1.OpsCancellingPhase {
		// when cancelling this opsRequest, revert the changes.
		return deletePodSet, createPodSet, nil
	}
	return createPodSet, deletePodSet, nil
}

// Cancel this function defines the cancel horizontalScaling action.
func (hs horizontalScalingOpsHandler) Cancel(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	if err := compOpsHelper.cancelComponentOps(reqCtx.Ctx, cli, opsRes, func(lastConfig *opsv1alpha1.LastComponentConfiguration, comp *appsv1.ClusterComponentSpec) {
		comp.Replicas = *lastConfig.Replicas
		comp.Instances = lastConfig.Instances
		comp.OfflineInstances = lastConfig.OfflineInstances
	}); err != nil {
		return err
	}

	// TODO: impl

	return nil
}

// checkIntersectionWithEarlierOps checks if the pod deleted by the current ops is a pod created by another ops
func (hs horizontalScalingOpsHandler) checkIntersectionWithEarlierOps(opsRes *OpsResource, earlierOps *opsv1alpha1.OpsRequest,
	currOpsHScaling, earlierOpsHScaling opsv1alpha1.HorizontalScaling) error {
	getCreatedOrDeletedPodSet := func(ops *opsv1alpha1.OpsRequest, hScaling opsv1alpha1.HorizontalScaling) (map[string]string, map[string]string, error) {
		lastCompSnapshot := ops.Status.LastConfiguration.Components[earlierOpsHScaling.ComponentName]
		compSpec := getComponentSpecOrShardingTemplate(opsRes.Cluster, earlierOpsHScaling.ComponentName).DeepCopy()
		var err error
		compSpec.Replicas, compSpec.Instances, compSpec.OfflineInstances, err = hs.getExpectedCompValues(opsRes, compSpec, lastCompSnapshot, hScaling)
		if err != nil {
			return nil, nil, err
		}
		return hs.getCreateAndDeletePodSet(opsRes, lastCompSnapshot, *compSpec, hScaling, hScaling.ComponentName)
	}
	createdPodSetForEarlier, _, err := getCreatedOrDeletedPodSet(earlierOps, earlierOpsHScaling)
	if err != nil {
		return err
	}
	_, deletedPodSetForCurrent, err := getCreatedOrDeletedPodSet(opsRes.OpsRequest, currOpsHScaling)
	if err != nil {
		return err
	}
	for deletedPod := range deletedPodSetForCurrent {
		if _, ok := createdPodSetForEarlier[deletedPod]; ok {
			errMsg := fmt.Sprintf(`instance "%s" cannot be taken offline as it has been created by another running opsRequest "%s"`,
				deletedPod, earlierOps.Name)
			return intctrlutil.NewFatalError(errMsg)
		}
	}
	return nil
}

// getExpectedCompValues gets the expected replicas, instances, offlineInstances.
func (hs horizontalScalingOpsHandler) getExpectedCompValues(
	opsRes *OpsResource,
	compSpec *appsv1.ClusterComponentSpec,
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	horizontalScaling opsv1alpha1.HorizontalScaling) (int32, []appsv1.InstanceTemplate, []string, error) {
	compReplicas := *lastCompConfiguration.Replicas
	compInstanceTpls := slices.Clone(lastCompConfiguration.Instances)
	compOfflineInstances := lastCompConfiguration.OfflineInstances
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
	horizontalScaling opsv1alpha1.HorizontalScaling,
	compReplicas int32,
	compInstanceTpls []appsv1.InstanceTemplate,
	compExpectOfflineInstances []string) error {
	// sync the replicaChanges for component and instance template.
	getSyncedInstancesAndReplicaChanges := func(offlineOrOnlineInsCountMap map[string]int32,
		replicaChanger opsv1alpha1.ReplicaChanger,
		newInstances []appsv1.InstanceTemplate) ([]opsv1alpha1.InstanceReplicasTemplate, *int32) {
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
				replicaChanger.Instances = append(replicaChanger.Instances, opsv1alpha1.InstanceReplicasTemplate{Name: k, ReplicaChanges: v})
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
			insTplName := appsv1.GetInstanceTemplateName(opsRes.Cluster.Name, horizontalScaling.ComponentName, insName)
			onlineInsCountMap[insTplName]++
		}
		scaleOut.Instances, scaleOut.ReplicaChanges = getSyncedInstancesAndReplicaChanges(onlineInsCountMap, scaleOut.ReplicaChanger, scaleOut.NewInstances)
	}
	return nil
}

// getCompExpectReplicas gets the expected replicas for the component.
func (hs horizontalScalingOpsHandler) getCompExpectReplicas(horizontalScaling opsv1alpha1.HorizontalScaling,
	compReplicas int32) int32 {
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
	compInstanceTpls []appsv1.InstanceTemplate,
	horizontalScaling opsv1alpha1.HorizontalScaling,
) []appsv1.InstanceTemplate {
	compInsTplSet := map[string]int{}
	for i := range compInstanceTpls {
		compInsTplSet[compInstanceTpls[i].Name] = i
	}
	handleInstanceTplReplicaChanges := func(instances []opsv1alpha1.InstanceReplicasTemplate, isScaleIn bool) {
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
	horizontalScaling opsv1alpha1.HorizontalScaling,
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
