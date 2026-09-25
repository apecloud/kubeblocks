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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

const horizontalScalingFailedReason = "HorizontalScalingFailed"

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
	for _, target := range opsRes.OpsRequest.Spec.HorizontalScalingList {
		if !isBackupScaling(target) {
			continue
		}
		if _, err := hs.getBackupObj(reqCtx, cli, opsRes, *target.ScaleOut.FromBackup); err != nil {
			return err
		}
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
				// check if the instance to be taken offline was created by another opsRequest.
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
		compSpec.Replicas = replicas
		compSpec.Instances = instances
		compSpec.OfflineInstances = offlineInstances
		if isBackupScaling(horizontalScaling) {
			compSpec.ReplicaRestore = buildReplicaRestoreIntent(opsRes.Cluster, *horizontalScaling.ScaleOut.FromBackup)
		}
		return nil
	}); err != nil {
		return err
	}
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for horizontal scaling opsRequest.
func (hs horizontalScalingOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	opsRequest := opsRes.OpsRequest
	oldOpsRequest := opsRequest.DeepCopy()
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{}
	}
	helper := newComponentOpsHelper(opsRequest.Spec.HorizontalScalingList)
	current := helper.emptyInstanceProgress(opsRes)
	complete, failed := true, false
	var expected, completed int32
	var failureErr error
	for _, target := range opsRequest.Spec.HorizontalScalingList {
		phase, progress, err := hs.reconcileScalingTarget(reqCtx, cli, opsRes, target)
		if err != nil {
			if !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
				if patchErr := patchCurrentProgress(reqCtx, cli, opsRes, oldOpsRequest, current, completed, expected); patchErr != nil {
					return opsv1alpha1.OpsRunningPhase, 0, patchErr
				}
				return opsv1alpha1.OpsRunningPhase, 0, err
			}
			phase = opsv1alpha1.OpsFailedPhase
			status := opsRequest.Status.Components[target.ComponentName]
			status.Message = err.Error()
			opsRequest.Status.Components[target.ComponentName] = status
			if failureErr == nil {
				failureErr = err
			}
		}
		if phase == opsv1alpha1.OpsFailedPhase {
			status := opsRequest.Status.Components[target.ComponentName]
			status.Reason = horizontalScalingFailedReason
			if err == nil {
				status.Message = fmt.Sprintf("Component %s failed while reconciling horizontal scaling", target.ComponentName)
			}
			opsRequest.Status.Components[target.ComponentName] = status
		}
		expected += progress.expectedCount
		completed += progress.completedCount
		current[target.ComponentName] = progress.details
		complete = complete && phase != opsv1alpha1.OpsRunningPhase
		failed = failed || phase == opsv1alpha1.OpsFailedPhase
	}
	if err := patchCurrentProgress(reqCtx, cli, opsRes, oldOpsRequest, current, completed, expected); err != nil {
		return opsv1alpha1.OpsRunningPhase, 0, err
	}
	if !complete {
		return opsv1alpha1.OpsRunningPhase, time.Second, nil
	}
	if failed {
		return opsv1alpha1.OpsFailedPhase, 0, failureErr
	}
	return opsv1alpha1.OpsSucceedPhase, 0, nil
}

func (hs horizontalScalingOpsHandler) reconcileScalingTarget(reqCtx intctrlutil.RequestCtx, cli client.Client,
	opsRes *OpsResource, target opsv1alpha1.HorizontalScaling) (opsv1alpha1.OpsPhase, instanceProgress, error) {
	status := opsRes.OpsRequest.Status.Components[target.ComponentName]
	if status.Reason == horizontalScalingFailedReason {
		return opsv1alpha1.OpsFailedPhase, instanceProgress{}, intctrlutil.NewFatalError(status.Message)
	}
	helper := newComponentOpsHelper([]opsv1alpha1.HorizontalScaling{target})
	var progress instanceProgress
	var err error
	if target.Shards != nil {
		progress, err = hs.observeShardScaling(reqCtx, cli, opsRes, target)
	} else {
		progress, err = hs.observeReplicaScaling(reqCtx, cli, opsRes, helper, target)
	}
	if err != nil {
		return opsv1alpha1.OpsRunningPhase, progress, err
	}
	if isBackupScaling(target) && opsRes.OpsRequest.Status.Components[target.ComponentName].Message != "Restore Data Completed" {
		return opsv1alpha1.OpsRunningPhase, progress, nil
	}
	phase := helper.componentActionPhase(opsRes, appsv1.RunningComponentPhase)
	if target.Shards != nil {
		spec := opsRes.Cluster.Spec.GetShardingByName(target.ComponentName)
		if spec == nil || spec.Shards != *target.Shards {
			phase = opsv1alpha1.OpsRunningPhase
		}
	}
	if phase == opsv1alpha1.OpsSucceedPhase && (!progress.observationsComplete || progress.succeededCount != progress.expectedCount) {
		phase = opsv1alpha1.OpsRunningPhase
	}
	return phase, progress, nil
}

func (hs horizontalScalingOpsHandler) observeReplicaScaling(reqCtx intctrlutil.RequestCtx, cli client.Client,
	opsRes *OpsResource, helper componentOpsHelper, target opsv1alpha1.HorizontalScaling) (instanceProgress, error) {
	progress := instanceProgress{observationsComplete: true}
	resources, err := helper.buildInstanceProgressResources(reqCtx, cli, opsRes, "horizontal scale")
	if err != nil {
		return progress, err
	}
	for i := range resources {
		resource := &resources[i]
		if isBackupScaling(target) {
			status := opsRes.OpsRequest.Status.Components[target.ComponentName]
			if status.Message != replicaRestoreCompletedMessage {
				restoreProgress, err := hs.observeReplicaRestore(reqCtx, cli, opsRes, resource, target, &status)
				if err != nil {
					return progress, err
				}
				opsRes.OpsRequest.Status.Components[target.ComponentName] = status
				if status.Message != replicaRestoreCompletedMessage {
					return restoreProgress, nil
				}
				progress = restoreProgress
			}
		}
		its := &workloads.InstanceSet{}
		key := client.ObjectKey{Namespace: opsRes.Cluster.Namespace,
			Name: constant.GenerateClusterComponentName(opsRes.Cluster.Name, resource.fullComponentName)}
		if err := cli.Get(reqCtx.Ctx, key, its); err != nil {
			if !apierrors.IsNotFound(err) {
				return progress, err
			}
			progress.expectedCount += resource.clusterComponent.Replicas
			progress.observationsComplete = false
			continue
		}
		observed := handleReplicaScalingProgress(opsRes, resource, its)
		progress.expectedCount += observed.expectedCount
		progress.completedCount += observed.completedCount
		progress.succeededCount += observed.succeededCount
		progress.observationsComplete = progress.observationsComplete && observed.observationsComplete
		progress.details = append(progress.details, observed.details...)
	}
	expectedComponents := int32(1)
	if spec := opsRes.Cluster.Spec.GetShardingByName(target.ComponentName); spec != nil {
		expectedComponents = spec.Shards
	}
	progress.observationsComplete = progress.observationsComplete && int32(len(resources)) == expectedComponents
	return progress, nil
}

func (hs horizontalScalingOpsHandler) observeShardScaling(reqCtx intctrlutil.RequestCtx, cli client.Client,
	opsRes *OpsResource, target opsv1alpha1.HorizontalScaling) (instanceProgress, error) {
	previousShards := int32(0)
	if previous := opsRes.OpsRequest.Status.LastConfiguration.Components[target.ComponentName].Shards; previous != nil {
		previousShards = *previous
	}
	targetShards := *target.Shards
	delta := targetShards - previousShards
	if delta < 0 {
		delta = -delta
	}
	progress := instanceProgress{expectedCount: delta}
	children, err := sharding.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, target.ComponentName)
	if err != nil {
		return progress, err
	}
	progress.observationsComplete = int32(len(children)) == targetShards
	runningChildren, terminalChildren := int32(0), int32(0)
	for _, child := range children {
		detail := opsv1alpha1.ProgressStatusDetail{ObjectKey: getProgressObjectKey(appsv1.ComponentKind, child.Name),
			Status: opsv1alpha1.ProcessingProgressStatus}
		observed := child.Status.ObservedGeneration == child.Generation
		if !child.DeletionTimestamp.IsZero() {
			progress.observationsComplete = false
		} else {
			switch {
			case !observed:
				progress.observationsComplete = false
			case child.Status.Phase == appsv1.RunningComponentPhase:
				detail.Status = opsv1alpha1.SucceedProgressStatus
				runningChildren++
				terminalChildren++
			case child.Status.Phase == appsv1.FailedComponentPhase:
				detail.Status = opsv1alpha1.FailedProgressStatus
				terminalChildren++
			}
		}
		detail.Message = fmt.Sprintf("%s shard %s", detail.Status, child.Name)
		progress.details = append(progress.details, detail)
	}
	if targetShards >= previousShards {
		progress.completedCount = min(delta, max(0, terminalChildren-previousShards))
		progress.succeededCount = min(delta, max(0, runningChildren-previousShards))
	} else {
		progress.completedCount = min(delta, max(0, previousShards-int32(len(children))))
		progress.succeededCount = progress.completedCount
	}
	return progress, nil
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
			Replicas:          pointer.Int32(compSpec.Replicas),
			InstanceTemplates: compSpec.Instances,
			OfflineInstances:  compSpec.OfflineInstances,
		}
	}
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	compOpsHelper.saveLastConfigurations(opsRes, getLastComponentInfo)
	for _, target := range opsRes.OpsRequest.Spec.HorizontalScalingList {
		if isBackupScaling(target) || !hasExplicitScalingInstances(target) {
			continue
		}
		last := opsRes.OpsRequest.Status.LastConfiguration.Components[target.ComponentName]
		its := &workloads.InstanceSet{}
		key := client.ObjectKey{Namespace: opsRes.Cluster.Namespace,
			Name: constant.GenerateClusterComponentName(opsRes.Cluster.Name, target.ComponentName)}
		if err := cli.Get(reqCtx.Ctx, key, its); err != nil {
			return err
		}
		active, err := activeInstanceTemplates(its.Status.InstanceStatus)
		if err != nil {
			return err
		}
		previous := &appsv1.ClusterComponentSpec{Replicas: *last.Replicas, Instances: last.InstanceTemplates}
		if !assignmentsMatchComponent(active, previous) {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting, "waiting for instance assignments of component %q", target.ComponentName)
		}
		assignments := map[string]string{}
		if target.ScaleIn != nil {
			for _, name := range target.ScaleIn.OnlineInstancesToOffline {
				if template, ok := active[name]; ok {
					assignments[name] = template
				}
			}
		}
		if target.ScaleOut != nil {
			for _, name := range target.ScaleOut.OfflineInstancesToOnline {
				if !slices.Contains(last.OfflineInstances, name) {
					continue
				}
				status := its.FindInstanceStatus(name)
				if status == nil || status.EffectiveDesiredState() != workloads.InstanceDesiredStateOffline || status.TemplateName == nil {
					return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting, "waiting for the template assignment of offline instance %q", name)
				}
				assignments[name] = *status.TemplateName
			}
		}
		last.Instances = []opsv1alpha1.LastInstanceConfiguration{}
		for _, name := range sets.List(sets.KeySet(assignments)) {
			last.Instances = append(last.Instances, opsv1alpha1.LastInstanceConfiguration{Name: name, TemplateName: assignments[name]})
		}
		if err := hs.validateHorizontalScaling(opsRes, last, target); err != nil {
			return err
		}
		opsRes.OpsRequest.Status.LastConfiguration.Components[target.ComponentName] = last
	}
	return nil
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
		*lastCompConfiguration.Replicas, lastCompConfiguration.InstanceTemplates, lastCompConfiguration.OfflineInstances)
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
	for _, v := range opsRes.OpsRequest.Spec.HorizontalScalingList {
		if v.Shards != nil {
			// This operation requires intervention by operations personnel.
			return intctrlutil.NewErrorf(intctrlutil.ErrorIgnoreCancel, "does not support cancellation of shard count changes during horizontal scaling.")
		}
	}
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	if err := compOpsHelper.cancelComponentOps(reqCtx.Ctx, cli, opsRes, func(lastConfig *opsv1alpha1.LastComponentConfiguration, comp *appsv1.ClusterComponentSpec) {
		comp.Replicas = *lastConfig.Replicas
		comp.Instances = lastConfig.InstanceTemplates
		comp.OfflineInstances = lastConfig.OfflineInstances
		comp.ReplicaRestore = nil
	}); err != nil {
		return err
	}
	for name, status := range opsRes.OpsRequest.Status.Components {
		if status.Reason == horizontalScalingFailedReason {
			status.Reason, status.Message = "", ""
			opsRes.OpsRequest.Status.Components[name] = status
		}
	}
	return nil
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
	if isBackupScaling(horizontalScaling) {
		return hs.getBackupExpectedCompValues(opsRes, lastCompConfiguration, horizontalScaling)
	}
	compReplicas := *lastCompConfiguration.Replicas
	compInstanceTpls := slices.Clone(lastCompConfiguration.InstanceTemplates)
	compOfflineInstances := lastCompConfiguration.OfflineInstances
	filteredHorizontal := horizontalScaling.DeepCopy()
	instances := lastCompConfiguration.Instances
	if hasExplicitScalingInstances(horizontalScaling) && instances == nil &&
		(compReplicas > 0 || len(compOfflineInstances) > 0) &&
		opsRes.OpsRequest.Annotations[constant.IgnoreHscaleValidateAnnoKey] != "true" {
		return 0, nil, nil, fmt.Errorf("missing pre-operation instance assignments for component %q", horizontalScaling.ComponentName)
	}
	online := make(map[string]string, len(instances))
	for _, instance := range instances {
		if !slices.Contains(compOfflineInstances, instance.Name) {
			online[instance.Name] = instance.TemplateName
		}
	}
	filterHorizontalScalingSpec(online, compOfflineInstances, filteredHorizontal)
	expectOfflineInstances := hs.getCompExpectedOfflineInstances(compOfflineInstances, *filteredHorizontal)
	hs.autoSyncReplicaChanges(*filteredHorizontal, lastCompConfiguration)
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

func hasExplicitScalingInstances(target opsv1alpha1.HorizontalScaling) bool {
	return target.ScaleIn != nil && len(target.ScaleIn.OnlineInstancesToOffline) > 0 ||
		target.ScaleOut != nil && len(target.ScaleOut.OfflineInstancesToOnline) > 0
}

// autoSyncReplicaChanges uses the owner's template assignments for explicitly named instances.
func (hs horizontalScalingOpsHandler) autoSyncReplicaChanges(
	horizontalScaling opsv1alpha1.HorizontalScaling, last opsv1alpha1.LastComponentConfiguration) {
	counts := func(names []string) map[string]int32 {
		result := map[string]int32{}
		for _, name := range names {
			templateName := ""
			if instance := last.FindInstance(name); instance != nil {
				templateName = instance.TemplateName
			}
			result[templateName]++
		}
		return result
	}
	if scaleIn := horizontalScaling.ScaleIn; scaleIn != nil {
		scaleIn.Instances, scaleIn.ReplicaChanges = syncReplicaChangesFromCounts(counts(scaleIn.OnlineInstancesToOffline), scaleIn.ReplicaChanger, nil)
	}
	if scaleOut := horizontalScaling.ScaleOut; scaleOut != nil {
		var onlineCounts map[string]int32
		if scaleOut.ReplicaChanges == nil {
			onlineCounts = counts(scaleOut.OfflineInstancesToOnline)
		}
		scaleOut.Instances, scaleOut.ReplicaChanges = syncReplicaChangesFromCounts(onlineCounts, scaleOut.ReplicaChanger, scaleOut.NewInstances)
	}
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
		if err := hs.validateOnlineInstancesToOffline(lastCompConfiguration,
			horizontalScaling.ScaleIn.OnlineInstancesToOffline, opsRes, horizontalScaling.ComponentName); err != nil {
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
	instances := lastCompConfiguration.Instances
	if instances == nil && *lastCompConfiguration.Replicas > 0 {
		return fmt.Errorf("missing pre-operation instance assignments for component %q", componentName)
	}
	for _, onlineIns := range onlineInstancesToOffline {
		if lastCompConfiguration.FindInstance(onlineIns) == nil || slices.Contains(lastCompConfiguration.OfflineInstances, onlineIns) {
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
