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
	"slices"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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
	if err := hs.checkFlatOrdinalSupport(reqCtx, cli, opsRes, false); err != nil {
		return err
	}
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
				// if the earlier opsRequest is pending return false.
				if earlierOps.Status.Phase == opsv1alpha1.OpsPendingPhase {
					return false, nil
				}
				if v.Shards != nil && currHorizontalScaling.Shards != nil {
					return true, nil
				}
				// Ordinary scaling can supersede an earlier request without predicting
				// which instance names that unfinished request would allocate.
				if !hscaleFromBackup(currHorizontalScaling) {
					return true, nil
				}
				// Backup recovery still needs the existing planned-name safety check.
				if err := hs.checkIntersectionWithEarlierOps(opsRes, earlierOps, currHorizontalScaling, v); err != nil {
					return false, err
				}
			}
			return false, nil
		}); err != nil {
		return err
	}

	// update shard count
	for i := range opsRes.Cluster.Spec.Shardings {
		sharding := &opsRes.Cluster.Spec.Shardings[i]
		if compOps, ok := compOpsSet.componentOpsSet[sharding.Name]; ok {
			horizontalScaling := compOps.(opsv1alpha1.HorizontalScaling)
			if horizontalScaling.Shards != nil {
				sharding.Shards = *horizontalScaling.Shards
			}
		}
	}

	if err := compOpsSet.updateClusterComponentsAndShardings(opsRes.Cluster, func(compSpec *appsv1.ClusterComponentSpec, obj ComponentOpsInterface) error {
		horizontalScaling := obj.(opsv1alpha1.HorizontalScaling)
		if horizontalScaling.Shards != nil {
			return nil
		}
		replicas, instances, offlineInstances, err := hs.prepareReplicaScaling(opsRes, horizontalScaling)
		if err != nil {
			return err
		}
		if horizontalScaling.ScaleOut != nil && horizontalScaling.ScaleOut.FromBackup != nil {
			// The backup path submits this configuration from reconcileBackupScaling
			// only after all persistent volumes have been restored.
			return nil
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
		horizontalScaling := pgRes.compOps.(opsv1alpha1.HorizontalScaling)
		pgRes.noWaitComponentCompleted = true
		if horizontalScaling.Shards != nil {
			// horizontal scaling for shard count.
			return handleComponentProgressForScalingShards(reqCtx, cli, opsRes, pgRes, compStatus)
		}
		if horizontalScaling.ScaleOut != nil && horizontalScaling.ScaleOut.FromBackup != nil {
			return hs.reconcileBackupScaling(reqCtx, cli, opsRes, pgRes, compStatus)
		}
		return hs.reconcileAllocatedScaling(reqCtx, cli, opsRes, pgRes, compStatus)
	}
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	return compOpsHelper.reconcileActionWithComponentOps(reqCtx, cli, opsRes, "", handleComponentProgress)
}

// prepareReplicaScaling validates and computes the target configuration for both
// ordinary and backup requests. Each path decides when to submit it.
func (hs horizontalScalingOpsHandler) prepareReplicaScaling(opsRes *OpsResource,
	horizontalScaling opsv1alpha1.HorizontalScaling) (int32, []appsv1.InstanceTemplate, []string, error) {
	lastCompConfiguration := opsRes.OpsRequest.Status.LastConfiguration.Components[horizontalScaling.ComponentName]
	if err := hs.validateHorizontalScaling(opsRes, lastCompConfiguration, horizontalScaling); err != nil {
		return 0, nil, nil, err
	}
	replicas, instances, offlineInstances, err := hs.getExpectedCompValues(opsRes,
		lastCompConfiguration, horizontalScaling)
	if err != nil {
		return 0, nil, nil, err
	}
	var insReplicas int32
	for _, v := range instances {
		insReplicas += v.GetReplicas()
	}
	if insReplicas > replicas {
		errMsg := fmt.Sprintf(`the total number of replicas for the instance template cannot be greater than the number of replicas for component "%s" after horizontally scaling`,
			horizontalScaling.ComponentName)
		return 0, nil, nil, intctrlutil.NewFatalError(errMsg)
	}
	return replicas, instances, offlineInstances, nil
}

// setReplicaScalingParticipants selects the backup path's planned instances,
// independently of when restored volumes allow the target to be submitted.
func (hs horizontalScalingOpsHandler) setReplicaScalingParticipants(opsRes *OpsResource,
	pgRes *progressResource) error {
	horizontalScaling := pgRes.compOps.(opsv1alpha1.HorizontalScaling)
	lastCompConfiguration := opsRes.OpsRequest.Status.LastConfiguration.Components[horizontalScaling.ComponentName]
	var err error
	pgRes.createdPodSet, pgRes.deletedPodSet, err = hs.getReplicaScalingChanges(opsRes, lastCompConfiguration,
		horizontalScaling, pgRes.fullComponentName)
	if err != nil {
		return err
	}
	if opsRes.OpsRequest.Status.Phase == opsv1alpha1.OpsCancellingPhase {
		pgRes.createdPodSet, pgRes.deletedPodSet = pgRes.deletedPodSet, pgRes.createdPodSet
	}
	return nil
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (hs horizontalScalingOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	if err := hs.checkFlatOrdinalSupport(reqCtx, cli, opsRes, false); err != nil {
		return err
	}
	shardsMap := make(map[string]int32, len(opsRes.Cluster.Spec.Shardings))
	for _, v := range opsRes.Cluster.Spec.Shardings {
		shardsMap[v.Name] = v.Shards
	}
	getLastComponentInfo := func(compSpec appsv1.ClusterComponentSpec, comOps ComponentOpsInterface) opsv1alpha1.LastComponentConfiguration {
		horizontalScaling := comOps.(opsv1alpha1.HorizontalScaling)
		if horizontalScaling.Shards != nil {
			var lastCompConfiguration opsv1alpha1.LastComponentConfiguration
			if shards, ok := shardsMap[comOps.GetComponentName()]; ok {
				lastCompConfiguration.Shards = pointer.Int32(shards)
			}
			return lastCompConfiguration
		}
		return opsv1alpha1.LastComponentConfiguration{
			Replicas:         pointer.Int32(compSpec.Replicas),
			Instances:        compSpec.Instances,
			OfflineInstances: compSpec.OfflineInstances,
		}
	}
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	compOpsHelper.saveLastConfigurations(opsRes, getLastComponentInfo)
	return hs.saveSourceAssignments(reqCtx, cli, opsRes)
}

// getReplicaScalingChanges compares the saved and requested instance names in
// the forward direction. Callers choose how to use the changes when cancelling.
func (hs horizontalScalingOpsHandler) getReplicaScalingChanges(opsRes *OpsResource,
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	horizontalScaling opsv1alpha1.HorizontalScaling,
	fullCompName string) (map[string]string, map[string]string, error) {
	clusterName := opsRes.Cluster.Name
	runtime, err := opsRes.GetRuntime(horizontalScaling.ComponentName)
	if err != nil {
		return nil, nil, err
	}
	lastPodSet, err := runtime.GenerateInstanceNameSet(clusterName, fullCompName,
		*lastCompConfiguration.Replicas, lastCompConfiguration.Instances, lastCompConfiguration.OfflineInstances)
	if err != nil {
		return nil, nil, err
	}
	expectReplicas, expectInstanceTpls, expectOfflineInstances, err := hs.getExpectedCompValues(opsRes, lastCompConfiguration, horizontalScaling)
	if err != nil {
		return nil, nil, err
	}
	currPodSet, err := runtime.GenerateInstanceNameSet(clusterName, fullCompName,
		expectReplicas, expectInstanceTpls, expectOfflineInstances)
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
	return createPodSet, deletePodSet, nil
}

// Cancel this function defines the cancel horizontalScaling action.
func (hs horizontalScalingOpsHandler) Cancel(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	if err := hs.checkFlatOrdinalSupport(reqCtx, cli, opsRes, true); err != nil {
		return err
	}
	for _, v := range opsRes.OpsRequest.Spec.HorizontalScalingList {
		if v.Shards != nil {
			// This operation requires intervention by operations personnel.
			return intctrlutil.NewErrorf(intctrlutil.ErrorIgnoreCancel, "does not support cancellation of shard count changes during horizontal scaling.")
		}
	}
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	return compOpsHelper.cancelComponentOps(reqCtx.Ctx, cli, opsRes, func(lastConfig *opsv1alpha1.LastComponentConfiguration, comp *appsv1.ClusterComponentSpec) {
		comp.Replicas = *lastConfig.Replicas
		comp.Instances = lastConfig.Instances
		comp.OfflineInstances = lastConfig.OfflineInstances
	})
}

// checkIntersectionWithEarlierOps checks if the pod deleted by the current ops is a pod created by another ops
func (hs horizontalScalingOpsHandler) checkIntersectionWithEarlierOps(opsRes *OpsResource, earlierOps *opsv1alpha1.OpsRequest,
	currOpsHScaling, earlierOpsHScaling opsv1alpha1.HorizontalScaling) error {
	getCreatedOrDeletedPodSet := func(ops *opsv1alpha1.OpsRequest, hScaling opsv1alpha1.HorizontalScaling) (map[string]string, map[string]string, error) {
		lastCompSnapshot := ops.Status.LastConfiguration.Components[earlierOpsHScaling.ComponentName]
		// Preserve target validation before instance-set generation, including its
		// error order relative to the earlier and current requests.
		if _, _, _, err := hs.getExpectedCompValues(opsRes, lastCompSnapshot, hScaling); err != nil {
			return nil, nil, err
		}
		created, deleted, err := hs.getReplicaScalingChanges(opsRes, lastCompSnapshot, hScaling, hScaling.ComponentName)
		if err != nil {
			return nil, nil, err
		}
		// Both comparisons use the current request's phase, as in the original
		// intersection check, even when calculating changes for an earlier Ops.
		if opsRes.OpsRequest.Status.Phase == opsv1alpha1.OpsCancellingPhase {
			created, deleted = deleted, created
		}
		return created, deleted, nil
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
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	horizontalScaling opsv1alpha1.HorizontalScaling) (int32, []appsv1.InstanceTemplate, []string, error) {
	if !hscaleFromBackup(horizontalScaling) {
		replicas, templates, offline := hs.getAllocatedCompValues(lastCompConfiguration, horizontalScaling)
		return replicas, templates, offline, nil
	}
	compReplicas := *lastCompConfiguration.Replicas
	compInstanceTpls := slices.Clone(lastCompConfiguration.Instances)
	compOfflineInstances := lastCompConfiguration.OfflineInstances
	filteredHorizontal := horizontalScaling.DeepCopy()
	// Obtain the existing names before filtering, even for requests without
	// explicit online/offline lists, preserving the original lookup/error order.
	runtime, err := opsRes.GetRuntime(horizontalScaling.ComponentName)
	if err != nil {
		return 0, nil, nil, err
	}
	podSet, err := runtime.GenerateInstanceNameSet(opsRes.Cluster.Name, horizontalScaling.ComponentName,
		compReplicas, compInstanceTpls, compOfflineInstances)
	if err != nil {
		return 0, nil, nil, err
	}
	filterHorizontalScalingSpec(podSet, compOfflineInstances, filteredHorizontal)
	expectOfflineInstances := hs.getCompExpectedOfflineInstances(compOfflineInstances, *filteredHorizontal)
	err = hs.autoSyncReplicaChanges(opsRes, *filteredHorizontal, compReplicas, compInstanceTpls, expectOfflineInstances)
	if err != nil {
		return 0, nil, nil, err
	}
	return hs.getCompExpectReplicas(*filteredHorizontal, compReplicas),
		hs.getCompExpectedInstances(compInstanceTpls, *filteredHorizontal),
		expectOfflineInstances, nil
}

// only offlined instances could be taken online.
// and only onlined instances could be taken offline.
func filterHorizontalScalingSpec(
	podSet map[string]string,
	compOfflineInstances []string,
	horizontalScaling *opsv1alpha1.HorizontalScaling) {
	offlineInstances := sets.New(compOfflineInstances...)
	if horizontalScaling.ScaleIn != nil && len(horizontalScaling.ScaleIn.OnlineInstancesToOffline) > 0 {
		onlinedInstanceFromOps := sets.Set[string]{}
		for _, insName := range horizontalScaling.ScaleIn.OnlineInstancesToOffline {
			if _, ok := podSet[insName]; ok {
				onlinedInstanceFromOps.Insert(insName)
			}
		}
		horizontalScaling.ScaleIn.OnlineInstancesToOffline = sets.List(onlinedInstanceFromOps)
	}
	if horizontalScaling.ScaleOut != nil && len(horizontalScaling.ScaleOut.OfflineInstancesToOnline) > 0 {
		offlinedInstanceFromOps := sets.Set[string]{}
		for _, insName := range horizontalScaling.ScaleOut.OfflineInstancesToOnline {
			if _, ok := offlineInstances[insName]; ok {
				offlinedInstanceFromOps.Insert(insName)
			}
		}
		horizontalScaling.ScaleOut.OfflineInstancesToOnline = sets.List(offlinedInstanceFromOps)
	}
}

// autoSyncReplicaChanges auto-sync the replicaChanges of the component and instance templates.
func (hs horizontalScalingOpsHandler) autoSyncReplicaChanges(
	opsRes *OpsResource,
	horizontalScaling opsv1alpha1.HorizontalScaling,
	compReplicas int32,
	compInstanceTpls []appsv1.InstanceTemplate,
	compExpectOfflineInstances []string) error {
	// auto sync the replicaChanges.
	scaleIn := horizontalScaling.ScaleIn
	if scaleIn != nil {
		offlineInsCountMap := opsRes.OpsRequest.CountOfflineOrOnlineInstances(opsRes.Cluster.Name, horizontalScaling.ComponentName, scaleIn.OnlineInstancesToOffline)
		scaleIn.Instances, scaleIn.ReplicaChanges = syncReplicaChangesFromCounts(offlineInsCountMap, scaleIn.ReplicaChanger, nil)
	}
	scaleOut := horizontalScaling.ScaleOut
	if scaleOut != nil {
		onlineInsCountMap, err := hs.getPlannedOnlineInstanceCounts(opsRes, horizontalScaling, compReplicas, compInstanceTpls, compExpectOfflineInstances)
		if err != nil {
			return err
		}
		scaleOut.Instances, scaleOut.ReplicaChanges = syncReplicaChangesFromCounts(onlineInsCountMap, scaleOut.ReplicaChanger, scaleOut.NewInstances)
	}
	return nil
}

// syncReplicaChangesFromCounts applies per-template instance counts and explicit
// replica changes without consulting runtime objects or instance naming rules.
func syncReplicaChangesFromCounts(offlineOrOnlineInsCountMap map[string]int32,
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
			// auto sync the replicaChanges for the instance template if the replicaChanges is not specified.
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

func (hs horizontalScalingOpsHandler) getPlannedOnlineInstanceCounts(
	opsRes *OpsResource,
	horizontalScaling opsv1alpha1.HorizontalScaling,
	compReplicas int32,
	compInstanceTpls []appsv1.InstanceTemplate,
	compExpectOfflineInstances []string) (map[string]int32, error) {
	if horizontalScaling.ScaleOut.ReplicaChanges != nil || len(horizontalScaling.ScaleOut.OfflineInstancesToOnline) == 0 {
		return nil, nil
	}
	compInstanceTplsClone := slices.Clone(compInstanceTpls)
	// 1. Automatically synchronize replicaChanges based on the specified OfflineInstancesToOnline
	offlineInsMap := groupInstancesToOnline(opsRes.Cluster.Name, horizontalScaling.ComponentName, horizontalScaling.ScaleOut)
	for _, insNames := range offlineInsMap {
		compReplicas += int32(len(insNames))
	}
	for i := range compInstanceTplsClone {
		tplName := compInstanceTplsClone[i].Name
		if insNames, ok := offlineInsMap[tplName]; ok {
			compInstanceTplsClone[i].Replicas = pointer.Int32(compInstanceTplsClone[i].GetReplicas() + int32(len(insNames)))
		}
	}
	// 2. obtain the updated Pod set after synchronization replicas.
	runtime, err := opsRes.GetRuntime(horizontalScaling.ComponentName)
	if err != nil {
		return nil, err
	}
	podSet, err := runtime.GenerateInstanceNameSet(opsRes.Cluster.Name, horizontalScaling.ComponentName,
		compReplicas, compInstanceTplsClone, compExpectOfflineInstances)
	if err != nil {
		return nil, err
	}
	// 3. count the number of online instances for each instance template.
	return countPlannedOnlineInstances(offlineInsMap, podSet), nil
}

// groupInstancesToOnline resolves template membership using the current naming
// rules. Preserve sorted order within each template for prefix counting below.
func groupInstancesToOnline(clusterName, componentName string, scaleOut *opsv1alpha1.ScaleOut) map[string][]string {
	slices.Sort(scaleOut.OfflineInstancesToOnline)
	offlineInsMap := map[string][]string{}
	instanceTplChangesMap := map[string]int32{}
	for _, tplChange := range scaleOut.ReplicaChanger.Instances {
		instanceTplChangesMap[tplChange.Name] = tplChange.ReplicaChanges
	}
	for _, insName := range scaleOut.OfflineInstancesToOnline {
		insTplName := appsv1.GetInstanceTemplateName(clusterName, componentName, insName)
		if _, ok := instanceTplChangesMap[insTplName]; ok {
			// Explicit template replica changes take precedence over inferred counts.
			continue
		}
		offlineInsMap[insTplName] = append(offlineInsMap[insTplName], insName)
	}
	return offlineInsMap
}

func countPlannedOnlineInstances(offlineInsMap map[string][]string, podSet map[string]string) map[string]int32 {
	onlineInsCountMap := map[string]int32{}
	for insTplName, insNames := range offlineInsMap {
		for _, insName := range insNames {
			// Count the leading requested names present in the plan, stopping at
			// the first missing name in each template. This does not check readiness.
			if _, ok := podSet[insName]; !ok {
				break
			}
			onlineInsCountMap[insTplName]++
		}
	}
	return onlineInsCountMap
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

// getCompExpectedInstances gets the expected instance templates of the component.
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

// validateHorizontalScaling validates the horizontal scaling if they are already offlined or onlined.
// if ignoreHscaleValidateAnnoKey is 'true', it would ignore and skip this validation.
func (hs horizontalScalingOpsHandler) validateHorizontalScaling(
	opsRes *OpsResource,
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	horizontalScaling opsv1alpha1.HorizontalScaling,
) error {
	if opsRes.OpsRequest.Annotations[constant.IgnoreHscaleValidateAnnoKey] == "true" {
		return nil
	}
	if horizontalScaling.ScaleIn != nil {
		var err error
		if hscaleFromBackup(horizontalScaling) {
			err = hs.validateOnlineInstancesToOffline(lastCompConfiguration,
				horizontalScaling.ScaleIn.OnlineInstancesToOffline, opsRes, horizontalScaling.ComponentName)
		} else {
			err = validateSourceOfflineNames(lastCompConfiguration.SourceInstanceAssignments, horizontalScaling.ScaleIn.OnlineInstancesToOffline)
		}
		if err != nil {
			return err
		}
	}
	if horizontalScaling.ScaleOut != nil {
		if err := hs.validateOfflineInstancesToOnline(lastCompConfiguration,
			horizontalScaling.ScaleOut.OfflineInstancesToOnline, horizontalScaling.ComponentName); err != nil {
			return err
		}
	}
	return nil
}

func (hs horizontalScalingOpsHandler) validateOnlineInstancesToOffline(
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	onlineInstancesToOffline []string,
	opsRes *OpsResource,
	componentName string) error {
	if len(onlineInstancesToOffline) == 0 {
		return nil
	}
	toOfflineSet := sets.New(onlineInstancesToOffline...)
	if len(toOfflineSet) < len(onlineInstancesToOffline) {
		return intctrlutil.NewFatalError("instances specified in onlineInstancesToOffline has duplicates")
	}
	runtime, err := opsRes.GetRuntime(componentName)
	if err != nil {
		return err
	}
	currPodSet, err := runtime.GenerateInstanceNameSet(opsRes.Cluster.Name, componentName,
		*lastCompConfiguration.Replicas, lastCompConfiguration.Instances, lastCompConfiguration.OfflineInstances)
	if err != nil {
		return err
	}
	for _, onlineIns := range onlineInstancesToOffline {
		if _, ok := currPodSet[onlineIns]; !ok {
			return intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" specified in onlineInstancesToOffline is not online`, onlineIns))
		}
	}
	return nil
}

func (hs horizontalScalingOpsHandler) validateOfflineInstancesToOnline(
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	offlineInstancesToOnline []string,
	componentName string) error {
	if len(offlineInstancesToOnline) == 0 {
		return nil
	}
	toOnlineSet := sets.New(offlineInstancesToOnline...)
	if len(toOnlineSet) < len(offlineInstancesToOnline) {
		return intctrlutil.NewFatalError("instances specified in offlineInstancesToOnline has duplicates")
	}
	offlineInstanceSet := sets.New(lastCompConfiguration.OfflineInstances...)
	for _, offlineIns := range offlineInstancesToOnline {
		if _, ok := offlineInstanceSet[offlineIns]; !ok {
			return intctrlutil.NewFatalError(fmt.Sprintf(`cannot find the offline instance "%s" in component "%s" for scaleOut operation`, offlineIns, componentName))
		}
	}
	return nil
}

func hscaleFromBackup(request opsv1alpha1.HorizontalScaling) bool {
	return request.ScaleOut != nil && request.ScaleOut.FromBackup != nil
}

// Check the whole request before any component is mutated. A rejected cancel must
// leave all components running the original operation, not roll back a subset.
func (hs horizontalScalingOpsHandler) checkFlatOrdinalSupport(reqCtx intctrlutil.RequestCtx, cli client.Client,
	opsRes *OpsResource, cancelling bool) error {
	for _, request := range opsRes.OpsRequest.Spec.HorizontalScalingList {
		if request.Shards != nil || !cancelling && !hscaleFromBackup(request) {
			continue
		}
		specs, err := hscaleComponentSpecs(reqCtx, cli, opsRes, request.ComponentName, nil)
		if err != nil {
			return err
		}
		for _, spec := range specs {
			if !spec.FlatInstanceOrdinal {
				continue
			}
			if cancelling {
				return intctrlutil.NewErrorf(intctrlutil.ErrorIgnoreCancel,
					"cancellation of started horizontal scaling on flat-ordinal component %q is unsupported", request.ComponentName)
			}
			return intctrlutil.NewFatalError(fmt.Sprintf("horizontal scaling from backup is unsupported for flat-ordinal component %q", request.ComponentName))
		}
	}
	return nil
}

// Resolve only existing components. Instance names always come from InstanceStatus.
// A shard's explicit overrides take precedence over the common template, both
// before and after an operation changes that common template.
func hscaleComponentSpecs(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource,
	componentName string, common *appsv1.ClusterComponentSpec) (map[string]*appsv1.ClusterComponentSpec, error) {
	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		spec := &opsRes.Cluster.Spec.ComponentSpecs[i]
		if spec.Name == componentName {
			if common != nil {
				spec = common
			}
			return map[string]*appsv1.ClusterComponentSpec{componentName: spec.DeepCopy()}, nil
		}
	}
	for _, group := range opsRes.Cluster.Spec.Shardings {
		if group.Name != componentName {
			continue
		}
		components, err := sharding.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, componentName)
		if err != nil {
			return nil, err
		}
		result := map[string]*appsv1.ClusterComponentSpec{}
		for _, component := range components {
			spec := group.Template.DeepCopy()
			if common != nil {
				spec = common.DeepCopy()
			}
			for _, template := range group.ShardTemplates {
				if template.Name != component.Labels[constant.KBAppShardTemplateLabelKey] {
					continue
				}
				if template.Replicas != nil {
					spec.Replicas = *template.Replicas
				}
				if template.Instances != nil {
					spec.Instances = template.Instances
				}
				if template.Ordinals != nil {
					spec.Ordinals = *template.Ordinals
				}
				if template.FlatInstanceOrdinal != nil {
					spec.FlatInstanceOrdinal = *template.FlatInstanceOrdinal
				}
			}
			result[component.Labels[constant.KBAppComponentLabelKey]] = spec
		}
		if len(result) != int(group.Shards) {
			return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting, "waiting for all components of sharding %q", componentName)
		}
		return result, nil
	}
	return nil, fmt.Errorf("component or sharding %q not found", componentName)
}

func (hs horizontalScalingOpsHandler) saveSourceAssignments(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	for _, request := range opsRes.OpsRequest.Spec.HorizontalScalingList {
		if request.Shards != nil || hscaleFromBackup(request) {
			continue
		}
		runtime, err := opsRes.GetRuntime(request.ComponentName)
		if err != nil {
			return err
		}
		specs, err := hscaleComponentSpecs(reqCtx, cli, opsRes, request.ComponentName, nil)
		if err != nil {
			return err
		}
		last := opsRes.OpsRequest.Status.LastConfiguration.Components[request.ComponentName]
		requestedOnline := sets.New[string]()
		if request.ScaleOut != nil {
			requestedOnline.Insert(request.ScaleOut.OfflineInstancesToOnline...)
		}
		// Ignore validation can filter names that were not explicitly offline.
		requestedOnline = requestedOnline.Intersection(sets.New(last.OfflineInstances...))
		foundOnline := sets.New[string]()
		for name, spec := range specs {
			workload, err := runtime.GetWorkload(opsRes.Cluster.Namespace, opsRes.Cluster.Name, name)
			if err != nil {
				return err
			}
			active, complete, err := activeAssignmentsForTarget(workload, spec)
			if err != nil {
				return err
			}
			if !complete || assignmentsIncludeOffline(active, spec.OfflineInstances) {
				return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting, "waiting for source instance allocation of component %q", name)
			}
			workloadName := constant.GenerateClusterComponentName(opsRes.Cluster.Name, name)
			for _, status := range workload.GetInstanceStatuses() {
				state := status.EffectiveDesiredState()
				if state != workloads.InstanceDesiredStateActive && !(state == workloads.InstanceDesiredStateOffline && requestedOnline.Has(status.PodName)) {
					continue
				}
				if status.TemplateName == nil {
					return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting, "waiting for template of instance %q", status.PodName)
				}
				last.SourceInstanceAssignments = append(last.SourceInstanceAssignments, opsv1alpha1.InstanceTemplateAssignment{
					WorkloadName: workloadName, PodName: status.PodName, TemplateName: *status.TemplateName, DesiredState: state,
				})
				if state == workloads.InstanceDesiredStateOffline {
					foundOnline.Insert(status.PodName)
				}
			}
		}
		if !foundOnline.IsSuperset(requestedOnline) {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting, "waiting for requested offline instance assignments")
		}
		slices.SortFunc(last.SourceInstanceAssignments, func(a, b opsv1alpha1.InstanceTemplateAssignment) int { return strings.Compare(a.PodName, b.PodName) })
		opsRes.OpsRequest.Status.LastConfiguration.Components[request.ComponentName] = last
	}
	return nil
}

func validateSourceOfflineNames(source []opsv1alpha1.InstanceTemplateAssignment, names []string) error {
	if len(sets.New(names...)) != len(names) {
		return intctrlutil.NewFatalError("instances specified in onlineInstancesToOffline has duplicates")
	}
	active := sourceAssignments(source, "", workloads.InstanceDesiredStateActive)
	for _, name := range names {
		if _, ok := active[name]; !ok {
			return intctrlutil.NewFatalError(fmt.Sprintf("instance %q specified in onlineInstancesToOffline is not online", name))
		}
	}
	return nil
}

func (hs horizontalScalingOpsHandler) getAllocatedCompValues(last opsv1alpha1.LastComponentConfiguration,
	request opsv1alpha1.HorizontalScaling) (int32, []appsv1.InstanceTemplate, []string) {
	filtered := request.DeepCopy()
	active := sourceAssignments(last.SourceInstanceAssignments, "", workloads.InstanceDesiredStateActive)
	filterHorizontalScalingSpec(active, last.OfflineInstances, filtered)
	count := func(names []string, state workloads.InstanceDesiredState) map[string]int32 {
		assignments := sourceAssignments(last.SourceInstanceAssignments, "", state)
		counts := map[string]int32{}
		for _, name := range names {
			if template, ok := assignments[name]; ok {
				counts[template]++
			}
		}
		return counts
	}
	if filtered.ScaleIn != nil {
		scale := filtered.ScaleIn
		scale.Instances, scale.ReplicaChanges = syncReplicaChangesFromCounts(count(scale.OnlineInstancesToOffline, workloads.InstanceDesiredStateActive), scale.ReplicaChanger, nil)
	}
	if filtered.ScaleOut != nil {
		scale := filtered.ScaleOut
		scale.Instances, scale.ReplicaChanges = syncReplicaChangesFromCounts(count(scale.OfflineInstancesToOnline, workloads.InstanceDesiredStateOffline), scale.ReplicaChanger, scale.NewInstances)
	}
	return hs.getCompExpectReplicas(*filtered, *last.Replicas), hs.getCompExpectedInstances(slices.Clone(last.Instances), *filtered), hs.getCompExpectedOfflineInstances(slices.Clone(last.OfflineInstances), *filtered)
}

func (hs horizontalScalingOpsHandler) reconcileAllocatedScaling(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource,
	pgRes *progressResource, compStatus *opsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
	request := pgRes.compOps.(opsv1alpha1.HorizontalScaling)
	if opsRes.OpsRequest.Status.Phase == opsv1alpha1.OpsCancellingPhase {
		// Only non-flat cancellation reaches here. The deterministic plan preserves
		// the original rollback behavior even before forward progress was recorded.
		if err := hs.setCancellationParticipants(reqCtx, cli, opsRes, pgRes); err != nil {
			return 0, 0, err
		}
		return handleComponentProgressForScalingReplicas(reqCtx, cli, opsRes, pgRes, compStatus)
	}
	last := opsRes.OpsRequest.Status.LastConfiguration.Components[request.ComponentName]
	replicas, templates, offline := hs.getAllocatedCompValues(last, request)
	targetSpec := pgRes.clusterComponent.DeepCopy()
	targetSpec.Replicas, targetSpec.Instances, targetSpec.OfflineInstances = replicas, templates, offline
	specs, err := hscaleComponentSpecs(reqCtx, cli, opsRes, request.ComponentName, targetSpec)
	if err != nil {
		return 0, 0, err
	}
	targetSpec = specs[pgRes.fullComponentName]
	if targetSpec == nil {
		return 1, 0, nil
	}
	runtime, err := opsRes.GetRuntime(request.ComponentName)
	if err != nil {
		return 0, 0, err
	}
	workload, err := runtime.GetWorkload(opsRes.Cluster.Namespace, opsRes.Cluster.Name, pgRes.fullComponentName)
	if err != nil {
		return 0, 0, err
	}
	target, complete, err := activeAssignmentsForTarget(workload, targetSpec)
	if err != nil {
		return 0, 0, err
	}
	if !complete || assignmentsIncludeOffline(target, offline) {
		return 1, 0, nil
	}
	workloadName := constant.GenerateClusterComponentName(opsRes.Cluster.Name, pgRes.fullComponentName)
	source := sourceAssignments(last.SourceInstanceAssignments, workloadName, workloads.InstanceDesiredStateActive)
	for _, assignment := range last.SourceInstanceAssignments {
		if assignment.WorkloadName == workloadName && assignment.DesiredState == workloads.InstanceDesiredStateOffline {
			if template, ok := target[assignment.PodName]; !ok || template != assignment.TemplateName {
				return 1, 0, nil
			}
		}
	}
	pgRes.createdPodSet, pgRes.deletedPodSet, pgRes.updatedPodSet = diffInstanceAssignments(source, target)
	expected, completed, err := handleComponentProgressForScalingReplicas(reqCtx, cli, opsRes, pgRes, compStatus)
	if err != nil || len(pgRes.updatedPodSet) == 0 {
		return expected, completed, err
	}
	// A template reassignment retains the instance name; it is an update, not
	// a deletion and a creation of that same name. Applied and healthy are separate.
	statuses := map[string]workloads.InstanceStatus{}
	for _, status := range workload.GetInstanceStatuses() {
		statuses[status.PodName] = status
	}
	pgRes.opsMessageKey = "Update"
	updateExpected, updateCompleted, err := handleComponentStatusProgress(reqCtx, cli, opsRes, pgRes, compStatus,
		func(_ *opsv1alpha1.OpsRequest, instance Instance, _ *progressResource) bool {
			status := statuses[instance.GetName()]
			return status.CurrentState == workloads.InstanceCurrentStatePresent && status.UpToDate
		})
	return expected + updateExpected, completed + updateCompleted, err
}

func (hs horizontalScalingOpsHandler) setCancellationParticipants(reqCtx intctrlutil.RequestCtx, cli client.Client,
	opsRes *OpsResource, pgRes *progressResource) error {
	request := pgRes.compOps.(opsv1alpha1.HorizontalScaling)
	last := opsRes.OpsRequest.Status.LastConfiguration.Components[request.ComponentName]
	sourceSpec := pgRes.clusterComponent.DeepCopy()
	sourceSpec.Replicas, sourceSpec.Instances, sourceSpec.OfflineInstances = *last.Replicas, last.Instances, last.OfflineInstances
	targetSpec := sourceSpec.DeepCopy()
	targetSpec.Replicas, targetSpec.Instances, targetSpec.OfflineInstances = hs.getAllocatedCompValues(last, request)
	runtime, err := opsRes.GetRuntime(request.ComponentName)
	if err != nil {
		return err
	}
	plan := func(common *appsv1.ClusterComponentSpec) (map[string]string, error) {
		specs, err := hscaleComponentSpecs(reqCtx, cli, opsRes, request.ComponentName, common)
		if err != nil {
			return nil, err
		}
		spec := specs[pgRes.fullComponentName]
		if spec == nil {
			return nil, fmt.Errorf("component %q no longer exists", pgRes.fullComponentName)
		}
		result := map[string]string{}
		templates := slices.Clone(spec.Instances)
		defaultReplicas := spec.Replicas
		for _, template := range templates {
			defaultReplicas -= template.GetReplicas()
		}
		templates = append(templates, appsv1.InstanceTemplate{Replicas: &defaultReplicas, Ordinals: spec.Ordinals})
		for _, template := range templates {
			names, err := runtime.GenerateTemplateInstanceNames(opsRes.Cluster.Name, pgRes.fullComponentName,
				template.Name, template.GetReplicas(), spec.OfflineInstances, template.Ordinals)
			if err != nil {
				return nil, err
			}
			for _, name := range names {
				result[name] = template.Name
			}
		}
		return result, nil
	}
	source, err := plan(sourceSpec)
	if err != nil {
		return err
	}
	target, err := plan(targetSpec)
	if err != nil {
		return err
	}
	pgRes.createdPodSet, pgRes.deletedPodSet, _ = diffInstanceAssignments(target, source)
	return nil
}
