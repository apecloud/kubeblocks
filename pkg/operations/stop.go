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
	"slices"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type StopOpsHandler struct{}

var _ OpsHandler = StopOpsHandler{}

func init() {
	stopBehaviour := OpsBehaviour{
		FromClusterPhases: append(appsv1.GetClusterUpRunningPhases(), appsv1.UpdatingClusterPhase),
		ToClusterPhase:    appsv1.StoppingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        StopOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.StopType, stopBehaviour)
}

// ActionStartedCondition the started condition when handling the stop request.
func (stop StopOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewStopCondition(opsRes.OpsRequest), nil
}

// Action sets stop on the requested component specs and sharding templates.
func (stop StopOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	var (
		cluster  = opsRes.Cluster
		stopList = opsRes.OpsRequest.Spec.StopList
	)

	// if the cluster is already stopping or stopped, return
	if slices.Contains([]appsv1.ClusterPhase{appsv1.StoppedClusterPhase,
		appsv1.StoppingClusterPhase}, opsRes.Cluster.Status.Phase) {
		return nil
	}
	compOpsHelper := newComponentOpsHelper(stopList)
	// abort earlier running opsRequests.
	if err := abortEarlierOpsRequestWithSameKind(reqCtx, cli, opsRes, []opsv1alpha1.OpsType{opsv1alpha1.HorizontalScalingType,
		opsv1alpha1.StartType, opsv1alpha1.RestartType, opsv1alpha1.VerticalScalingType},
		func(earlierOps *opsv1alpha1.OpsRequest) (bool, error) {
			if len(stopList) == 0 {
				// stop all components
				return true, nil
			}
			switch earlierOps.Spec.Type {
			case opsv1alpha1.RestartType:
				return hasIntersectionCompOpsList(compOpsHelper.componentOpsSet, earlierOps.Spec.RestartList), nil
			case opsv1alpha1.VerticalScalingType:
				return hasIntersectionCompOpsList(compOpsHelper.componentOpsSet, earlierOps.Spec.VerticalScalingList), nil
			case opsv1alpha1.HorizontalScalingType:
				return hasIntersectionCompOpsList(compOpsHelper.componentOpsSet, earlierOps.Spec.HorizontalScalingList), nil
			case opsv1alpha1.StartType:
				return len(earlierOps.Spec.StartList) == 0 || hasIntersectionCompOpsList(compOpsHelper.componentOpsSet, earlierOps.Spec.StartList), nil
			}
			return false, nil
		}); err != nil {
		return err
	}

	stopComp := func(compSpec *appsv1.ClusterComponentSpec, clusterCompName string) {
		if len(stopList) > 0 {
			if _, ok := compOpsHelper.componentOpsSet[clusterCompName]; !ok {
				return
			}
		}
		compSpec.Stop = ptr.To(true)
	}

	for i, v := range cluster.Spec.ComponentSpecs {
		stopComp(&cluster.Spec.ComponentSpecs[i], v.Name)
	}
	for i, v := range cluster.Spec.Shardings {
		stopComp(&cluster.Spec.Shardings[i].Template, v.Name)
	}
	return cli.Update(reqCtx.Ctx, cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for stop opsRequest.
func (stop StopOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	if rollingActionGenerationPending(opsRes) {
		return opsv1alpha1.OpsRunningPhase, 0, nil
	}
	if !stop.targetsStopped(opsRes) {
		return opsv1alpha1.OpsAbortedPhase, 0, nil
	}
	return stop.reconcile(reqCtx, cli, opsRes)
}

// SaveLastConfiguration is empty because Stop reconciles the current component target.
func (stop StopOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func stopParticipantStatuses(its *workloads.InstanceSet) []workloads.InstanceStatus {
	preexistingOffline := make(map[string]struct{}, len(its.Spec.OfflineInstances))
	for _, name := range its.Spec.OfflineInstances {
		preexistingOffline[name] = struct{}{}
	}
	stopping := ptr.Deref(its.Spec.Stop, false)
	participants := make([]workloads.InstanceStatus, 0, len(its.Status.InstanceStatus))
	for i := range its.Status.InstanceStatus {
		status := its.Status.InstanceStatus[i]
		switch status.EffectiveDesiredState() {
		case workloads.InstanceDesiredStateActive:
			participants = append(participants, status)
		case workloads.InstanceDesiredStateOffline:
			if stopping {
				if _, alreadyOffline := preexistingOffline[status.PodName]; !alreadyOffline {
					participants = append(participants, status)
				}
			}
		}
	}
	return participants
}

func (stop StopOpsHandler) reconcile(reqCtx intctrlutil.RequestCtx, cli client.Client,
	opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	opsRequest := opsRes.OpsRequest
	oldOpsRequest := opsRequest.DeepCopy()
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{}
	}
	helper := newComponentOpsHelper(opsRequest.Spec.StopList)
	resources, err := helper.buildInstanceProgressResources(reqCtx, cli, opsRes, "stop")
	if err != nil {
		return opsv1alpha1.OpsRunningPhase, 0, err
	}
	current := helper.emptyInstanceProgress(opsRes)
	var expectedCount, completedCount int32
	observationsComplete := true
	componentCounts := map[string]int32{}
	for _, resource := range resources {
		name := resource.compOps.GetComponentName()
		componentCounts[name]++
		if _, ok := current[name]; !ok {
			current[name] = nil
		}
		its := &workloads.InstanceSet{}
		key := client.ObjectKey{Namespace: opsRes.Cluster.Namespace,
			Name: constant.GenerateClusterComponentName(opsRes.Cluster.Name, resource.fullComponentName)}
		if err := cli.Get(reqCtx.Ctx, key, its); err != nil {
			if !apierrors.IsNotFound(err) {
				return opsv1alpha1.OpsRunningPhase, 0, err
			}
			expectedCount += resource.clusterComponent.Replicas
			observationsComplete = false
			continue
		}
		participants := stopParticipantStatuses(its)
		replicas := ptr.Deref(its.Spec.Replicas, 1)
		expectedCount += max(replicas, int32(len(participants)))
		if !ptr.Deref(its.Spec.Stop, false) || int32(len(participants)) != replicas {
			observationsComplete = false
		}
		for _, instance := range participants {
			objectKey := getProgressObjectKey(constant.PodKind, instance.PodName)
			detail := opsv1alpha1.ProgressStatusDetail{ObjectKey: objectKey, Group: resource.fullComponentName}
			if instance.EffectiveCurrentState() == workloads.InstanceCurrentStateAbsent {
				detail.SetStatusAndMessage(opsv1alpha1.SucceedProgressStatus,
					getProgressSucceedMessage("stop", objectKey, resource.fullComponentName))
				completedCount++
			} else {
				detail.SetStatusAndMessage(opsv1alpha1.ProcessingProgressStatus,
					getProgressProcessingMessage("stop", objectKey, resource.fullComponentName))
			}
			current[name] = append(current[name], detail)
		}
	}
	for _, sharding := range opsRes.Cluster.Spec.Shardings {
		if _, selected := helper.getComponentOps(sharding.Name); selected && componentCounts[sharding.Name] != sharding.Shards {
			observationsComplete = false
		}
	}
	phase := helper.componentActionPhase(opsRes, appsv1.StoppedComponentPhase)
	// Apps and workload observations can arrive separately. Keep reconciling until
	// the observed progress agrees with success; never fill missing progress from it.
	if phase == opsv1alpha1.OpsSucceedPhase && (!observationsComplete || completedCount != expectedCount) {
		phase = opsv1alpha1.OpsRunningPhase
	}
	if err := patchCurrentProgress(reqCtx, cli, opsRes, oldOpsRequest, current, completedCount, expectedCount); err != nil {
		return opsv1alpha1.OpsRunningPhase, 0, err
	}
	if phase == opsv1alpha1.OpsRunningPhase {
		return phase, time.Second, nil
	}
	return phase, 0, nil
}

func (stop StopOpsHandler) targetsStopped(opsRes *OpsResource) bool {
	if opsRes == nil || opsRes.Cluster == nil || opsRes.OpsRequest == nil {
		return false
	}
	stopList := opsRes.OpsRequest.Spec.StopList
	if len(stopList) > 0 {
		for i := range stopList {
			compSpec := getComponentSpecOrShardingTemplate(opsRes.Cluster, stopList[i].ComponentName)
			if compSpec == nil || !ptr.Deref(compSpec.Stop, false) {
				return false
			}
		}
		return true
	}
	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		if !ptr.Deref(opsRes.Cluster.Spec.ComponentSpecs[i].Stop, false) {
			return false
		}
	}
	for i := range opsRes.Cluster.Spec.Shardings {
		if !ptr.Deref(opsRes.Cluster.Spec.Shardings[i].Template.Stop, false) {
			return false
		}
	}
	return true
}
