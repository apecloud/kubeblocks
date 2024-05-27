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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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
	// abort earlier running vertical scaling opsRequest.
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
				// if the earlier opsRequest is pending and not `Overwrite` operator, return false.
				if earlierOps.Status.Phase == appsv1alpha1.OpsPendingPhase {
					return false, nil
				}
				if hs.existOverwriteReplicasOP(currHorizontalScaling, v) {
					return true, nil
				}
				// check if the instance that needs to be offline was created by another opsRequest
				if err := hs.checkIntersectionWithEarlierOps(opsRes, earlierOps, currHorizontalScaling, v); err != nil {
					return false, err
				}
			}
			return false, nil
		}); err != nil {
		return err
	}

	if err := compOpsSet.updateClusterComponentsAndShardings(opsRes.Cluster, func(compSpec *appsv1alpha1.ClusterComponentSpec, obj ComponentOpsInteface) error {
		horizontalScaling := obj.(appsv1alpha1.HorizontalScaling)
		lastCompConfiguration := opsRes.OpsRequest.Status.LastConfiguration.Components[obj.GetComponentName()]
		replicas, instances, offlineInstances := hs.getExpectedCompValues(opsRes.Cluster, compSpec.DeepCopy(),
			lastCompConfiguration, horizontalScaling)
		var insReplicas int32
		for _, v := range instances {
			insReplicas += intctrlutil.TemplateReplicas(v)
		}
		if insReplicas > replicas {
			errMsg := fmt.Sprintf(`the total number of replicas of the instance template can not greater than the number of replicas of component "%s" after horizontally scaling`,
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

// getExpectedCompValues gets the expected replicas, instances, offlineInstances.
func (hs horizontalScalingOpsHandler) getExpectedCompValues(
	cluster *appsv1alpha1.Cluster,
	compSpec *appsv1alpha1.ClusterComponentSpec,
	lastCompConfiguration appsv1alpha1.LastComponentConfiguration,
	horizontalScaling appsv1alpha1.HorizontalScaling) (int32, []appsv1alpha1.InstanceTemplate, []string) {
	compReplicas := compSpec.Replicas
	compInstanceTpls := compSpec.Instances
	compOfflineInstances := compSpec.OfflineInstances
	if hs.comparedWithLastConfiguration(horizontalScaling) || hs.needAutoSyncReplicas(cluster, horizontalScaling) {
		// `Add` and `Delete` operations require the use of recorded component snapshot information.
		compReplicas = *lastCompConfiguration.Replicas
		compInstanceTpls = lastCompConfiguration.Instances
		compOfflineInstances = lastCompConfiguration.OfflineInstances
	}
	expectOfflineInstances := hs.getCompExpectedOfflineInstances(compOfflineInstances, horizontalScaling)
	compReplicas, compInstanceTpls = hs.autoSyncReplicas(cluster, horizontalScaling, compReplicas,
		compInstanceTpls, compOfflineInstances, expectOfflineInstances)
	return hs.getCompExpectReplicas(horizontalScaling, compReplicas),
		hs.getCompExpectedInstances(compInstanceTpls, horizontalScaling),
		expectOfflineInstances
}

// autoSyncReplicas auto-sync the replicas of the component and instance templates.
func (hs horizontalScalingOpsHandler) autoSyncReplicas(
	cluster *appsv1alpha1.Cluster,
	horizontalScaling appsv1alpha1.HorizontalScaling,
	compReplicas int32,
	compInstanceTpls []appsv1alpha1.InstanceTemplate,
	compOfflineInstances,
	expectCompOfflineInstances []string) (int32, []appsv1alpha1.InstanceTemplate) {
	if !hs.needAutoSyncReplicas(cluster, horizontalScaling) {
		return compReplicas, compInstanceTpls
	}
	handleAutoSyncReplicas := func(podSet map[string]string, diffOfflineInstances []string, onlineInstance bool) {
		diffReplicasMap := map[string]int32{}
		for _, podName := range diffOfflineInstances {
			tplName, ok := podSet[podName]
			if !ok {
				continue
			}
			diffReplicasMap[tplName] += 1
			if onlineInstance {
				// online the instance which is removed fromm the offlineInstances slice.
				compReplicas += 1
			} else {
				compReplicas -= 1
			}
		}
		for i, insTpl := range compInstanceTpls {
			diffReplicas, ok := diffReplicasMap[insTpl.Name]
			if !ok {
				continue
			}
			if onlineInstance {
				compInstanceTpls[i].Replicas = pointer.Int32(intctrlutil.TemplateReplicas(insTpl) + diffReplicas)
			} else {
				compInstanceTpls[i].Replicas = pointer.Int32(intctrlutil.TemplateReplicas(insTpl) - diffReplicas)
			}
		}
	}

	if len(horizontalScaling.OfflineInstancesToOnline) > 0 {
		// online the specified instances by removing the instance from offlineInstances slice.
		podSet := getPodSetForComponent(compInstanceTpls, expectCompOfflineInstances,
			cluster.Name, horizontalScaling.ComponentName, compReplicas)
		handleAutoSyncReplicas(podSet, horizontalScaling.OfflineInstancesToOnline, true)
	}
	if len(horizontalScaling.OnlineInstancesToOffline) > 0 {
		// offline the specified instances by adding the instance to offlineInstances slice.
		podSet := getPodSetForComponent(compInstanceTpls, compOfflineInstances,
			cluster.Name, horizontalScaling.ComponentName, compReplicas)
		handleAutoSyncReplicas(podSet, horizontalScaling.OnlineInstancesToOffline, false)
	}

	return compReplicas, compInstanceTpls
}

// getCompExpectReplicas gets the expected replicas of the component.
func (hs horizontalScalingOpsHandler) getCompExpectReplicas(horizontalScaling appsv1alpha1.HorizontalScaling,
	compSyncedReplicas int32) int32 {
	if horizontalScaling.NoneOP() {
		return compSyncedReplicas
	}
	switch {
	case horizontalScaling.AddOP():
		return compSyncedReplicas + *horizontalScaling.ReplicasToAdd
	case horizontalScaling.DeleteOP():
		return compSyncedReplicas - *horizontalScaling.ReplicasToDelete
	default:
		return *horizontalScaling.Replicas
	}
}

// getCompExpectedOfflineInstances gets the expected instance templates of the component.
func (hs horizontalScalingOpsHandler) getCompExpectedInstances(
	compSyncedInstanceTpls []appsv1alpha1.InstanceTemplate,
	horizontalScaling appsv1alpha1.HorizontalScaling,
) []appsv1alpha1.InstanceTemplate {
	if horizontalScaling.Instances == nil {
		// delete instance template is not supported, return directly.
		return compSyncedInstanceTpls
	}

	compInsTplSet := map[string]int{}
	for i := range compSyncedInstanceTpls {
		compInsTplSet[compSyncedInstanceTpls[i].Name] = i
	}
	compSyncedInstanceTpls = append(compSyncedInstanceTpls, horizontalScaling.Instances.Add...)
	for _, v := range horizontalScaling.Instances.Change {
		compInsIndex, ok := compInsTplSet[v.Name]
		if !ok {
			continue
		}
		if v.NoneOP() {
			continue
		}
		switch {
		case v.AddOP():
			compSyncedInstanceTpls[compInsIndex].Replicas = pointer.Int32(intctrlutil.TemplateReplicas(compSyncedInstanceTpls[compInsIndex]) + *v.ReplicasToAdd)
		case v.DeleteOP():
			compSyncedInstanceTpls[compInsIndex].Replicas = pointer.Int32(intctrlutil.TemplateReplicas(compSyncedInstanceTpls[compInsIndex]) - *v.ReplicasToDelete)
		default:
			compSyncedInstanceTpls[compInsIndex].Replicas = v.Replicas
		}
	}
	return compSyncedInstanceTpls
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
	if len(horizontalScaling.OnlineInstancesToOffline) != 0 {
		compOfflineInstances = handleOfflineInstances(horizontalScaling.OnlineInstancesToOffline, compOfflineInstances, compOfflineInstances)
	}
	if len(horizontalScaling.OfflineInstancesToOnline) != 0 {
		compOfflineInstances = handleOfflineInstances(compOfflineInstances, horizontalScaling.OfflineInstancesToOnline, make([]string, 0))
	}
	return compOfflineInstances
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
		pgRes.createdPodSet, pgRes.deletedPodSet = hs.getCreateAndDeletePodSet(opsRes, lastCompConfiguration, *pgRes.clusterComponent, horizontalScaling, pgRes.fullComponentName)
		if !horizontalScaling.OverwriteOP() {
			pgRes.noWaitComponentCompleted = true
		}
		return handleComponentProgressForScalingReplicas(reqCtx, cli, opsRes, pgRes, compStatus)
	}
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	return compOpsHelper.reconcileActionWithComponentOps(reqCtx, cli, opsRes, "", handleComponentProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (hs horizontalScalingOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	getLastComponentInfo := func(compSpec appsv1alpha1.ClusterComponentSpec, comOps ComponentOpsInteface) appsv1alpha1.LastComponentConfiguration {
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
	fullCompName string) (map[string]string, map[string]string) {
	clusterName := opsRes.Cluster.Name
	lastPodSet := getPodSetForComponent(lastCompConfiguration.Instances, lastCompConfiguration.OfflineInstances, clusterName, fullCompName, *lastCompConfiguration.Replicas)
	expectReplicas, expectInstanceTpls, expectOfflineInstances := hs.getExpectedCompValues(opsRes.Cluster, &currCompSpec, lastCompConfiguration, horizontalScaling)
	currPodSet := getPodSetForComponent(expectInstanceTpls, expectOfflineInstances, clusterName, fullCompName, expectReplicas)
	createPodSet := map[string]string{}
	deletePodSet := map[string]string{}
	for k, v := range currPodSet {
		if _, ok := lastPodSet[k]; !ok {
			createPodSet[k] = v
		}
	}
	for k, v := range lastPodSet {
		if _, ok := currPodSet[k]; !ok {
			deletePodSet[k] = v
		}
	}
	if opsRes.OpsRequest.Status.Phase == appsv1alpha1.OpsCancellingPhase {
		// when cancelling this opsRequest, revert the changes.
		return deletePodSet, createPodSet
	}
	return createPodSet, deletePodSet
}

// Cancel this function defines the cancel horizontalScaling action.
func (hs horizontalScalingOpsHandler) Cancel(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.VerticalScalingList)
	return compOpsHelper.cancelComponentOps(reqCtx.Ctx, cli, opsRes, func(lastConfig *appsv1alpha1.LastComponentConfiguration, comp *appsv1alpha1.ClusterComponentSpec) {
		comp.Replicas = *lastConfig.Replicas
		comp.Instances = lastConfig.Instances
		comp.OfflineInstances = lastConfig.OfflineInstances
	})
}

// needAutoSyncReplicas When replicas, replicasToAdd, and replicasToDelete are not specified and component is not a sharding component,
// it needs to sync the component and instanceTemplate replicas automatically.
func (hs horizontalScalingOpsHandler) needAutoSyncReplicas(cluster *appsv1alpha1.Cluster, horizontalScaling appsv1alpha1.HorizontalScaling) bool {
	if len(horizontalScaling.OnlineInstancesToOffline) == 0 && len(horizontalScaling.OfflineInstancesToOnline) == 0 {
		return false
	}
	for _, shardingSpec := range cluster.Spec.ShardingSpecs {
		// component can not be a sharding component.
		if shardingSpec.Name == horizontalScaling.ComponentName {
			return false
		}
	}
	if !horizontalScaling.NoneOP() {
		return false
	}
	if horizontalScaling.Instances != nil {
		for _, v := range horizontalScaling.Instances.Change {
			if !v.NoneOP() {
				return false
			}
		}
	}
	return true
}

func (hs horizontalScalingOpsHandler) comparedWithLastConfiguration(horizontalScaling appsv1alpha1.HorizontalScaling) bool {
	// if existing replicasToAdd or replicasToDelete, compare with replicas snapshot which is saved in lastComponentConfiguration.
	if horizontalScaling.AddOP() || horizontalScaling.DeleteOP() {
		return true
	}
	if horizontalScaling.Instances != nil {
		for _, v := range horizontalScaling.Instances.Change {
			if v.AddOP() || horizontalScaling.DeleteOP() {
				return true
			}
		}
	}
	return false
}

func (hs horizontalScalingOpsHandler) existOverwriteReplicasOP(currHScaling, earlierHScaling appsv1alpha1.HorizontalScaling) bool {
	checkInstances := func(instanceOps *appsv1alpha1.InstancesOperation) bool {
		if instanceOps == nil {
			return false
		}
		if len(instanceOps.Add) > 0 {
			return true
		}
		for _, v := range instanceOps.Change {
			if v.OverwriteOP() {
				return true
			}
		}
		return false
	}
	// if existing an overwrite replicas operation, need to abort.
	if currHScaling.OverwriteOP() || earlierHScaling.OverwriteOP() {
		return true
	}
	if checkInstances(earlierHScaling.Instances) {
		return true
	}
	return checkInstances(currHScaling.Instances)
}

// checkIntersectionWithEarlierOps checks if the pod deleted by the current ops is a pod created by another ops
func (hs horizontalScalingOpsHandler) checkIntersectionWithEarlierOps(opsRes *OpsResource, earlierOps *appsv1alpha1.OpsRequest,
	currOpsHScaling, earlierOpsHScaling appsv1alpha1.HorizontalScaling) error {
	getCreatedOrDeletedPodSet := func(ops *appsv1alpha1.OpsRequest, hScaling appsv1alpha1.HorizontalScaling) (map[string]string, map[string]string) {
		lastCompSnapshot := ops.Status.LastConfiguration.Components[earlierOpsHScaling.ComponentName]
		compSpec := opsRes.Cluster.Spec.GetComponentByName(earlierOpsHScaling.ComponentName).DeepCopy()
		compSpec.Replicas, compSpec.Instances, compSpec.OfflineInstances = hs.getExpectedCompValues(opsRes.Cluster, compSpec,
			lastCompSnapshot, hScaling)
		return hs.getCreateAndDeletePodSet(opsRes, lastCompSnapshot, *compSpec, hScaling, hScaling.ComponentName)
	}
	createdPodSetForEarlier, _ := getCreatedOrDeletedPodSet(earlierOps, earlierOpsHScaling)
	_, deletedPodSetForCurrent := getCreatedOrDeletedPodSet(opsRes.OpsRequest, currOpsHScaling)
	for deletedPod := range deletedPodSetForCurrent {
		if _, ok := createdPodSetForEarlier[deletedPod]; ok {
			errMsg := fmt.Sprintf(`instance "%s" cannot be taken offline as it has been created by another running opsRequest "%s"`,
				deletedPod, earlierOps.Name)
			return intctrlutil.NewFatalError(errMsg)
		}
	}
	return nil
}
