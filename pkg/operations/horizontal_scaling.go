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
	"sort"
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

func scaleOutFromBackup(horizontalScaling opsv1alpha1.HorizontalScaling) bool {
	return horizontalScaling.ScaleOut != nil && horizontalScaling.ScaleOut.FromBackup != nil
}

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
	// Reject unsupported requests before aborting earlier operations or updating
	// any component in a multi-component request.
	for _, scaling := range opsRes.OpsRequest.Spec.HorizontalScalingList {
		comp := getComponentSpecOrShardingTemplate(opsRes.Cluster, scaling.ComponentName)
		if scaling.Shards == nil && comp != nil && comp.FlatInstanceOrdinal && scaleOutFromBackup(scaling) {
			return intctrlutil.NewFatalError(fmt.Sprintf(
				"horizontal scale-out from backup is not supported for flat-ordinal component %q because the future instance names are not allocated yet", scaling.ComponentName))
		}
	}
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
				if _, ok := compOpsSet.componentOpsSet[v.ComponentName]; ok {
					return true, nil
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
		if scaleOutFromBackup(horizontalScaling) {
			return hs.prepareBackupScaling(opsRes, horizontalScaling)
		}
		replicas, instances, offlineInstances, err := hs.prepareReplicaScaling(opsRes, horizontalScaling)
		if err != nil {
			return err
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
	if opsRes.OpsRequest.Status.Phase == opsv1alpha1.OpsCancellingPhase {
		if err := hs.validateCancellation(opsRes); err != nil {
			return opsRes.OpsRequest.Status.Phase, 0, err
		}
	}
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
		if scaleOutFromBackup(horizontalScaling) {
			return hs.reconcileBackupScaling(reqCtx, cli, opsRes, pgRes, compStatus)
		}
		complete, err := hs.setReplicaScalingParticipants(opsRes, pgRes)
		if err != nil {
			return 0, 0, err
		}
		if !complete {
			return 1, 0, nil
		}
		return handleComponentProgressForScalingReplicas(reqCtx, cli, opsRes, pgRes, compStatus)
	}
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	return compOpsHelper.reconcileActionWithComponentOps(reqCtx, cli, opsRes, "", handleComponentProgress)
}

// setReplicaScalingParticipants reads the current allocation and selects the
// instances whose progress this request tracks. False means allocation is not ready.
func (hs horizontalScalingOpsHandler) setReplicaScalingParticipants(opsRes *OpsResource,
	pgRes *progressResource) (bool, error) {
	horizontalScaling := pgRes.compOps.(opsv1alpha1.HorizontalScaling)
	lastCompConfiguration := opsRes.OpsRequest.Status.LastConfiguration.Components[horizontalScaling.ComponentName]
	clusterComponentSpec := pgRes.clusterComponent.DeepCopy()
	// Compare against this request's target, not an unrelated later edit to
	// the live Cluster spec.
	if lastCompConfiguration.Replicas == nil {
		return false, fmt.Errorf("missing source replicas for component %q", pgRes.fullComponentName)
	}
	var err error
	clusterComponentSpec.Replicas, clusterComponentSpec.Instances, clusterComponentSpec.OfflineInstances, err =
		hs.getExpectedCompValues(lastCompConfiguration, horizontalScaling)
	if err != nil {
		return false, err
	}
	workloadName := constant.GenerateClusterComponentName(opsRes.Cluster.Name, pgRes.fullComponentName)
	source := sourceAssignmentsForWorkload(lastCompConfiguration, workloadName)
	sourceComponent := clusterComponentSpec.DeepCopy()
	sourceComponent.Replicas = *lastCompConfiguration.Replicas
	sourceComponent.Instances = lastCompConfiguration.Instances
	sourceComponent.OfflineInstances = lastCompConfiguration.OfflineInstances
	shardingSpec := opsRes.Cluster.Spec.GetShardingByName(horizontalScaling.ComponentName)
	applyHScaleShardOverrides(sourceComponent, shardingSpec, pgRes.shardTemplateName)
	applyHScaleShardOverrides(clusterComponentSpec, shardingSpec, pgRes.shardTemplateName)
	if !assignmentsMatchComponent(source, sourceComponent) {
		return false, fmt.Errorf("source instance assignments for InstanceSet %q are incomplete", workloadName)
	}
	runtime, err := opsRes.GetRuntime(pgRes.compOps.GetComponentName())
	if err != nil {
		return false, err
	}
	if opsRes.OpsRequest.Status.Phase == opsv1alpha1.OpsCancellingPhase {
		pgRes.createdPodSet, pgRes.deletedPodSet, err = hs.nonFlatRollbackInstanceSets(runtime,
			opsRes.Cluster.Name, pgRes.fullComponentName, source, clusterComponentSpec)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	workload, err := runtime.GetWorkload(opsRes.Cluster.Namespace, opsRes.Cluster.Name, pgRes.fullComponentName)
	if err != nil {
		return false, err
	}
	target, complete, err := activeAssignmentsForTarget(workload, clusterComponentSpec)
	if err != nil {
		return false, err
	}
	if !complete {
		return false, nil
	}
	created, deleted, updated := diffAssignments(source, target)
	effective := filterSourceHorizontalScalingSpec(lastCompConfiguration, horizontalScaling.DeepCopy())
	if !horizontalDiffMatchesOperation(*effective, created, deleted) {
		return false, nil
	}
	pgRes.createdPodSet, pgRes.deletedPodSet = created, deleted
	pgRes.updatedPodSet = updated
	return true, nil
}

// prepareReplicaScaling validates ordinary scaling using the captured source allocation.
func (hs horizontalScalingOpsHandler) prepareReplicaScaling(opsRes *OpsResource,
	horizontalScaling opsv1alpha1.HorizontalScaling) (int32, []appsv1.InstanceTemplate, []string, error) {
	lastCompConfiguration := opsRes.OpsRequest.Status.LastConfiguration.Components[horizontalScaling.ComponentName]
	if err := hs.validateHorizontalScaling(opsRes, lastCompConfiguration, horizontalScaling); err != nil {
		return 0, nil, nil, err
	}
	replicas, instances, offlineInstances, err := hs.getExpectedCompValues(lastCompConfiguration, horizontalScaling)
	if err != nil {
		return 0, nil, nil, err
	}
	if err := validateScalingTarget(horizontalScaling.ComponentName, replicas, instances); err != nil {
		return 0, nil, nil, err
	}
	return replicas, instances, offlineInstances, nil
}

// validateScalingTarget is shared by ordinary and backup requests.
func validateScalingTarget(componentName string, replicas int32, instances []appsv1.InstanceTemplate) error {
	var insReplicas int32
	for _, instance := range instances {
		insReplicas += instance.GetReplicas()
	}
	if insReplicas > replicas {
		return intctrlutil.NewFatalError(fmt.Sprintf(
			"the total number of replicas for the instance template cannot be greater than the number of replicas for component %q after horizontally scaling", componentName))
	}
	return nil
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (hs horizontalScalingOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
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
	requiredOffline := map[string]sets.Set[string]{}
	for _, horizontalScaling := range opsRes.OpsRequest.Spec.HorizontalScalingList {
		if horizontalScaling.Shards == nil && !scaleOutFromBackup(horizontalScaling) && horizontalScaling.ScaleOut != nil {
			last := opsRes.OpsRequest.Status.LastConfiguration.Components[horizontalScaling.ComponentName]
			requiredOffline[horizontalScaling.ComponentName] = sets.New(horizontalScaling.ScaleOut.OfflineInstancesToOnline...).Intersection(sets.New(last.OfflineInstances...))
		}
	}
	return captureHScaleSourceAssignments(reqCtx, cli, opsRes, compOpsHelper, requiredOffline)
}

// Cancel this function defines the cancel horizontalScaling action.
func (hs horizontalScalingOpsHandler) Cancel(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	// Validate the whole request before restoring any component configuration.
	if err := hs.validateCancellation(opsRes); err != nil {
		return err
	}
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	return compOpsHelper.cancelComponentOps(reqCtx.Ctx, cli, opsRes, func(lastConfig *opsv1alpha1.LastComponentConfiguration, comp *appsv1.ClusterComponentSpec) {
		comp.Replicas = *lastConfig.Replicas
		comp.Instances = lastConfig.Instances
		comp.OfflineInstances = lastConfig.OfflineInstances
	})
}

func (hs horizontalScalingOpsHandler) validateCancellation(opsRes *OpsResource) error {
	for _, v := range opsRes.OpsRequest.Spec.HorizontalScalingList {
		if v.Shards != nil {
			// This operation requires intervention by operations personnel.
			return intctrlutil.NewErrorf(intctrlutil.ErrorIgnoreCancel, "does not support cancellation of shard count changes during horizontal scaling.")
		}
		comp := opsRes.Cluster.Spec.GetComponentByName(v.ComponentName)
		flat := comp != nil && comp.FlatInstanceOrdinal
		if sharding := opsRes.Cluster.Spec.GetShardingByName(v.ComponentName); sharding != nil {
			// Shard templates may override the common template's ordinal mode.
			remaining := sharding.Shards
			for _, template := range sharding.ShardTemplates {
				count := pointer.Int32Deref(template.Shards, 0)
				remaining -= count
				if count > 0 && pointer.BoolDeref(template.FlatInstanceOrdinal, sharding.Template.FlatInstanceOrdinal) {
					flat = true
				}
			}
			flat = flat || remaining > 0 && sharding.Template.FlatInstanceOrdinal
		}
		if flat {
			return intctrlutil.NewErrorf(intctrlutil.ErrorIgnoreCancel,
				"cancellation of horizontal scaling is not supported for flat-ordinal component/sharding %q; the operation will continue", v.ComponentName)
		}
	}
	return nil
}

// getExpectedCompValues gets the expected replicas, instances and offline instances from
// the InstanceSet-owned assignments captured before the operation.
func (hs horizontalScalingOpsHandler) getExpectedCompValues(
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	horizontalScaling opsv1alpha1.HorizontalScaling) (int32, []appsv1.InstanceTemplate, []string, error) {
	compReplicas := *lastCompConfiguration.Replicas
	compInstanceTpls := slices.Clone(lastCompConfiguration.Instances)
	compOfflineInstances := lastCompConfiguration.OfflineInstances
	filteredHorizontal := filterSourceHorizontalScalingSpec(lastCompConfiguration, horizontalScaling.DeepCopy())
	expectOfflineInstances := hs.getCompExpectedOfflineInstances(compOfflineInstances, *filteredHorizontal)
	if err := hs.autoSyncReplicaChanges(lastCompConfiguration, *filteredHorizontal); err != nil {
		return 0, nil, nil, err
	}
	return hs.getCompExpectReplicas(*filteredHorizontal, compReplicas),
		hs.getCompExpectedInstances(compInstanceTpls, *filteredHorizontal),
		expectOfflineInstances, nil
}

// filterSourceHorizontalScalingSpec keeps only identities that are in the source assignment state
// required by the requested transition.
func filterSourceHorizontalScalingSpec(
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	horizontalScaling *opsv1alpha1.HorizontalScaling) *opsv1alpha1.HorizontalScaling {
	podSet := map[string]string{}
	for _, assignment := range lastCompConfiguration.SourceInstanceAssignments {
		if assignment.DesiredState == workloads.InstanceDesiredStateActive {
			podSet[assignment.PodName] = assignment.TemplateName
		}
	}
	filterHorizontalScalingSpec(podSet, lastCompConfiguration.OfflineInstances, horizontalScaling)
	return horizontalScaling
}

// filterHorizontalScalingSpec filters transitions using explicit identity inputs.
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

func syncReplicaChanges(horizontalScaling opsv1alpha1.HorizontalScaling,
	offlineInsCountMap, onlineInsCountMap map[string]int32) {
	if scaleIn := horizontalScaling.ScaleIn; scaleIn != nil {
		scaleIn.Instances, scaleIn.ReplicaChanges = syncReplicaChangesFromCounts(
			offlineInsCountMap, scaleIn.ReplicaChanger, nil)
	}
	if scaleOut := horizontalScaling.ScaleOut; scaleOut != nil {
		scaleOut.Instances, scaleOut.ReplicaChanges = syncReplicaChangesFromCounts(
			onlineInsCountMap, scaleOut.ReplicaChanger, scaleOut.NewInstances)
	}
}

// syncReplicaChangesFromCounts computes replica changes without naming or runtime dependencies.
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

func (hs horizontalScalingOpsHandler) autoSyncReplicaChanges(
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	horizontalScaling opsv1alpha1.HorizontalScaling) error {
	templateByActiveName := map[string]string{}
	templateByOfflineName := map[string]string{}
	for _, assignment := range lastCompConfiguration.SourceInstanceAssignments {
		switch assignment.DesiredState {
		case workloads.InstanceDesiredStateActive:
			templateByActiveName[assignment.PodName] = assignment.TemplateName
		case workloads.InstanceDesiredStateOffline:
			templateByOfflineName[assignment.PodName] = assignment.TemplateName
		}
	}
	offlineInsCountMap := map[string]int32{}
	if scaleIn := horizontalScaling.ScaleIn; scaleIn != nil {
		for _, name := range scaleIn.OnlineInstancesToOffline {
			templateName, ok := templateByActiveName[name]
			if !ok {
				return intctrlutil.NewFatalError(fmt.Sprintf("cannot determine the template of active instance %q", name))
			}
			offlineInsCountMap[templateName]++
		}
	}
	onlineInsCountMap, err := getToOnlineInsCountMap(horizontalScaling, templateByOfflineName)
	if err != nil {
		return err
	}
	syncReplicaChanges(horizontalScaling, offlineInsCountMap, onlineInsCountMap)
	return nil
}

func getToOnlineInsCountMap(horizontalScaling opsv1alpha1.HorizontalScaling,
	templateByOfflineName map[string]string) (map[string]int32, error) {
	if horizontalScaling.ScaleOut == nil || horizontalScaling.ScaleOut.ReplicaChanges != nil ||
		len(horizontalScaling.ScaleOut.OfflineInstancesToOnline) == 0 {
		return nil, nil
	}
	onlineInsCountMap := map[string]int32{}
	instanceTplChangesMap := map[string]int32{}
	for _, tplChange := range horizontalScaling.ScaleOut.ReplicaChanger.Instances {
		instanceTplChangesMap[tplChange.Name] = tplChange.ReplicaChanges
	}
	for _, insName := range horizontalScaling.ScaleOut.OfflineInstancesToOnline {
		insTplName, found := templateByOfflineName[insName]
		if !found {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf("cannot determine the template of offline instance %q", insName))
		}
		if _, ok := instanceTplChangesMap[insTplName]; ok {
			continue
		}
		onlineInsCountMap[insTplName]++
	}
	return onlineInsCountMap, nil
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

// validateHorizontalScaling validates the horizontal scaling if they are already offlined or onlined.
// if ignoreHscaleValidateAnnoKey is 'true', it would ignore and skip this validation.
func (hs horizontalScalingOpsHandler) validateHorizontalScaling(
	opsRes *OpsResource,
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	horizontalScaling opsv1alpha1.HorizontalScaling,
) error {
	activeAssignments := map[string]string{}
	for _, assignment := range lastCompConfiguration.SourceInstanceAssignments {
		if assignment.DesiredState == workloads.InstanceDesiredStateActive {
			activeAssignments[assignment.PodName] = assignment.TemplateName
		}
	}
	return hs.validateHorizontalScalingWithActiveAssignments(opsRes, lastCompConfiguration, horizontalScaling,
		activeAssignments)
}

func (hs horizontalScalingOpsHandler) validateHorizontalScalingWithActiveAssignments(
	opsRes *OpsResource,
	lastCompConfiguration opsv1alpha1.LastComponentConfiguration,
	horizontalScaling opsv1alpha1.HorizontalScaling,
	activeAssignments map[string]string,
) error {
	if opsRes.OpsRequest.Annotations[constant.IgnoreHscaleValidateAnnoKey] == "true" {
		return nil
	}
	if horizontalScaling.ScaleIn != nil {
		if err := hs.validateOnlineInstancesToOffline(horizontalScaling.ScaleIn.OnlineInstancesToOffline,
			activeAssignments); err != nil {
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
	onlineInstancesToOffline []string,
	activeAssignments map[string]string) error {
	if len(onlineInstancesToOffline) == 0 {
		return nil
	}
	toOfflineSet := sets.New(onlineInstancesToOffline...)
	if len(toOfflineSet) < len(onlineInstancesToOffline) {
		return intctrlutil.NewFatalError("instances specified in onlineInstancesToOffline has duplicates")
	}
	for _, onlineIns := range onlineInstancesToOffline {
		if _, ok := activeAssignments[onlineIns]; !ok {
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

func sourceAssignmentsForWorkload(last opsv1alpha1.LastComponentConfiguration, workloadName string) map[string]string {
	result := map[string]string{}
	for _, assignment := range last.SourceInstanceAssignments {
		if assignment.WorkloadName == workloadName && assignment.DesiredState == workloads.InstanceDesiredStateActive {
			result[assignment.PodName] = assignment.TemplateName
		}
	}
	return result
}

func horizontalDiffMatchesOperation(horizontalScaling opsv1alpha1.HorizontalScaling,
	created, deleted map[string]string) bool {
	if horizontalScaling.ScaleOut == nil && len(created) > 0 {
		return false
	}
	if horizontalScaling.ScaleIn == nil && len(deleted) > 0 {
		return false
	}
	if horizontalScaling.ScaleIn != nil {
		for _, name := range horizontalScaling.ScaleIn.OnlineInstancesToOffline {
			if _, ok := deleted[name]; !ok {
				return false
			}
		}
	}
	if horizontalScaling.ScaleOut != nil {
		for _, name := range horizontalScaling.ScaleOut.OfflineInstancesToOnline {
			if _, ok := created[name]; !ok {
				return false
			}
		}
	}
	return true
}

// nonFlatRollbackInstanceSets preserves the existing cancellation contract:
// reverse the operation's planned creations/deletions, including when no
// forward progress was observed. Name planning is valid here only because
// non-flat names are determined by configuration, not allocation history.
func (hs horizontalScalingOpsHandler) nonFlatRollbackInstanceSets(runtime OpsRuntime,
	clusterName, componentName string, source map[string]string,
	target *appsv1.ClusterComponentSpec) (map[string]string, map[string]string, error) {
	templates := slices.Clone(target.Instances)
	// HScale does not change the component's default ordinals. Include the
	// default template explicitly so cancellation planning respects them too.
	defaultReplicas := target.Replicas
	for _, template := range templates {
		defaultReplicas -= template.GetReplicas()
	}
	if defaultReplicas > 0 {
		templates = append(templates, appsv1.InstanceTemplate{Replicas: &defaultReplicas, Ordinals: target.Ordinals})
	}
	forward, err := runtime.GenerateInstanceNameSet(clusterName, componentName, target.Replicas, templates, target.OfflineInstances)
	if err != nil {
		return nil, nil, err
	}
	// Non-flat template names are part of the instance name, so changing a
	// template cannot reassign an existing name.
	created, deleted, _ := diffAssignments(source, forward)
	return deleted, created, nil
}

func captureHScaleSourceAssignments(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource,
	compOps componentOpsHelper, requiredOffline map[string]sets.Set[string]) error {
	remainingOffline := map[string]sets.Set[string]{}
	for componentName, names := range requiredOffline {
		remainingOffline[componentName] = names.Clone()
	}
	capture := func(logicalComponentName, fullComponentName string, op ComponentOpsInterface,
		component *appsv1.ClusterComponentSpec) error {
		if component == nil || op.(opsv1alpha1.HorizontalScaling).Shards != nil || scaleOutFromBackup(op.(opsv1alpha1.HorizontalScaling)) {
			return nil
		}
		runtime, err := opsRes.GetRuntime(logicalComponentName)
		if err != nil {
			return err
		}
		workload, err := runtime.GetWorkload(opsRes.Cluster.Namespace, opsRes.Cluster.Name, fullComponentName)
		if err != nil {
			return err
		}
		active, err := activeInstanceTemplates(workload.GetInstanceStatuses())
		if err != nil {
			return err
		}
		if !assignmentsMatchComponent(active, component) {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting,
				"waiting for InstanceSet %q to publish its active instance allocation", constant.GenerateClusterComponentName(opsRes.Cluster.Name, fullComponentName))
		}
		workloadName := constant.GenerateClusterComponentName(opsRes.Cluster.Name, fullComponentName)
		last := opsRes.OpsRequest.Status.LastConfiguration.Components[logicalComponentName]
		kept := last.SourceInstanceAssignments[:0]
		for _, assignment := range last.SourceInstanceAssignments {
			if assignment.WorkloadName != workloadName {
				kept = append(kept, assignment)
			}
		}
		last.SourceInstanceAssignments = kept
		for podName, templateName := range active {
			last.SourceInstanceAssignments = append(last.SourceInstanceAssignments, opsv1alpha1.InstanceTemplateAssignment{
				WorkloadName: workloadName, PodName: podName, TemplateName: templateName,
				DesiredState: workloads.InstanceDesiredStateActive,
			})
		}
		if names := remainingOffline[logicalComponentName]; names.Len() > 0 {
			for _, status := range workload.GetInstanceStatuses() {
				name := status.PodName
				if !names.Has(name) {
					continue
				}
				if status.EffectiveDesiredState() != workloads.InstanceDesiredStateOffline || status.TemplateName == nil {
					return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting,
						"waiting for InstanceSet %q to publish offline instance %q and its template", workloadName, name)
				}
				last.SourceInstanceAssignments = append(last.SourceInstanceAssignments, opsv1alpha1.InstanceTemplateAssignment{
					WorkloadName: workloadName, PodName: name, TemplateName: *status.TemplateName,
					DesiredState: workloads.InstanceDesiredStateOffline,
				})
				names.Delete(name)
			}
		}
		sort.Slice(last.SourceInstanceAssignments, func(i, j int) bool {
			a, b := last.SourceInstanceAssignments[i], last.SourceInstanceAssignments[j]
			if a.WorkloadName != b.WorkloadName {
				return a.WorkloadName < b.WorkloadName
			}
			return a.PodName < b.PodName
		})
		opsRes.OpsRequest.Status.LastConfiguration.Components[logicalComponentName] = last
		return nil
	}
	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		component := &opsRes.Cluster.Spec.ComponentSpecs[i]
		if op, ok := compOps.getComponentOps(component.Name); ok {
			if err := capture(component.Name, component.Name, op, component); err != nil {
				return err
			}
		}
	}
	for i := range opsRes.Cluster.Spec.Shardings {
		shardingSpec := &opsRes.Cluster.Spec.Shardings[i]
		op, ok := compOps.getComponentOps(shardingSpec.Name)
		if !ok {
			continue
		}
		components, err := sharding.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, shardingSpec.Name)
		if err != nil {
			return err
		}
		for _, component := range components {
			fullName := component.Labels[constant.KBAppComponentLabelKey]
			spec := shardingSpec.Template.DeepCopy()
			applyHScaleShardOverrides(spec, shardingSpec, component.Labels[constant.KBAppShardTemplateLabelKey])
			if err := capture(shardingSpec.Name, fullName, op, spec); err != nil {
				return err
			}
		}
	}
	for componentName, names := range remainingOffline {
		if names.Len() > 0 {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting,
				"waiting for InstanceSet to publish offline instances %v of component %q and their templates",
				sets.List(names), componentName)
		}
	}
	return nil
}

// HScale updates the common sharding template. Explicit per-shard settings
// continue to override it, both before scaling and in the operation's target.
// Only allocation-related fields are needed here; this is not a full spec merge.
func applyHScaleShardOverrides(spec *appsv1.ClusterComponentSpec, sharding *appsv1.ClusterSharding, templateName string) {
	if sharding == nil {
		return
	}
	for _, template := range sharding.ShardTemplates {
		if template.Name != templateName {
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
		return
	}
}
