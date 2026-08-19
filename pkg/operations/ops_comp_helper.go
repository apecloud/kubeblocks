/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

# This file is part of KubeBlocks project

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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type ComponentOpsInterface interface {
	GetComponentName() string
}

type componentOpsHelper struct {
	componentOpsSet map[string]ComponentOpsInterface
}

func newComponentOpsHelper[T ComponentOpsInterface](compOpsList []T) componentOpsHelper {
	compOpsHelper := componentOpsHelper{
		componentOpsSet: make(map[string]ComponentOpsInterface),
	}
	for i := range compOpsList {
		compOps := compOpsList[i]
		compOpsHelper.componentOpsSet[compOps.GetComponentName()] = compOps
	}
	return compOpsHelper
}

func (c componentOpsHelper) updateClusterComponentsAndShardings(cluster *appsv1.Cluster,
	updateFunc func(compSpec *appsv1.ClusterComponentSpec, compOpsItem ComponentOpsInterface) error) error {
	updateComponentSpecs := func(compSpec *appsv1.ClusterComponentSpec, componentName string) error {
		if obj, ok := c.componentOpsSet[componentName]; ok {
			if err := updateFunc(compSpec, obj); err != nil {
				return err
			}
		}
		return nil
	}
	// 1. update the components
	for index := range cluster.Spec.ComponentSpecs {
		comSpec := &cluster.Spec.ComponentSpecs[index]
		if err := updateComponentSpecs(comSpec, comSpec.Name); err != nil {
			return err
		}
	}
	// 1. update the sharding components
	for index := range cluster.Spec.Shardings {
		sharding := &cluster.Spec.Shardings[index]
		if err := updateComponentSpecs(&sharding.Template, sharding.Name); err != nil {
			return err
		}
	}
	return nil
}

type rollingUpgradeIntent struct {
	ComponentDef   string `json:"componentDef,omitempty"`
	ServiceVersion string `json:"serviceVersion,omitempty"`
	Receipt        string `json:"receipt,omitempty"`
}

type rollingRestartIntent struct {
	RestartAt string `json:"restartAt,omitempty"`
}

type rollingInstanceResources struct {
	Name      string                       `json:"name"`
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

type rollingVerticalScalingIntent struct {
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	Instances []rollingInstanceResources   `json:"instances,omitempty"`
}

// rollingTargetSpecHash fingerprints only the part of the Cluster target owned
// by this operation. A later, compatible operation may advance Cluster
// generation or change unrelated fields without invalidating the rolling
// operation. Changes that overwrite its submitted intent still produce a
// different hash and are treated as superseding it.
func rollingTargetSpecHash(target *appsv1.ClusterComponentSpec, compOps ComponentOpsInterface) (string, error) {
	var intent any
	switch op := compOps.(type) {
	case opsv1alpha1.UpgradeComponent:
		intent = rollingUpgradeIntent{
			ComponentDef:   target.ComponentDef,
			ServiceVersion: target.ServiceVersion,
			Receipt:        target.Annotations[constant.UpgradeIntentAnnotationKey],
		}
	case opsv1alpha1.VerticalScaling:
		vsIntent := rollingVerticalScalingIntent{}
		if len(op.Requests) > 0 || len(op.Limits) > 0 {
			resources := target.Resources.DeepCopy()
			vsIntent.Resources = resources
		}
		instanceNames := make(map[string]struct{}, len(op.Instances))
		for _, instance := range op.Instances {
			instanceNames[instance.Name] = struct{}{}
		}
		for i := range target.Instances {
			instance := &target.Instances[i]
			if _, ok := instanceNames[instance.Name]; !ok {
				continue
			}
			vsIntent.Instances = append(vsIntent.Instances, rollingInstanceResources{
				Name:      instance.Name,
				Resources: instance.Resources.DeepCopy(),
			})
		}
		slices.SortFunc(vsIntent.Instances, func(a, b rollingInstanceResources) int {
			return strings.Compare(a.Name, b.Name)
		})
		intent = vsIntent
	case opsv1alpha1.ComponentOps:
		intent = rollingRestartIntent{RestartAt: target.Annotations[constant.RestartAnnotationKey]}
	default:
		// Some non-rolling cancellation paths share recordRollingTargetSpecs.
		// Preserve their existing behavior until they provide a narrower intent.
		intent = target
	}
	data, err := json.Marshal(intent)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func findRollingTargetSpec(cluster *appsv1.Cluster, targetName string) (*appsv1.ClusterComponentSpec, bool) {
	for i := range cluster.Spec.ComponentSpecs {
		if cluster.Spec.ComponentSpecs[i].Name == targetName {
			return &cluster.Spec.ComponentSpecs[i], true
		}
	}
	for i := range cluster.Spec.Shardings {
		if cluster.Spec.Shardings[i].Name == targetName {
			return &cluster.Spec.Shardings[i].Template, true
		}
	}
	return nil, false
}

func (c componentOpsHelper) recordRollingTargetSpecs(opsRes *OpsResource) error {
	if opsRes.OpsRequest.Status.Components == nil {
		opsRes.OpsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{}
	}
	for targetName, compOps := range c.componentOpsSet {
		target, found := findRollingTargetSpec(opsRes.Cluster, targetName)
		if !found {
			return fmt.Errorf("rolling operation target %q not found in Cluster spec", targetName)
		}
		hash, err := rollingTargetSpecHash(target, compOps)
		if err != nil {
			return fmt.Errorf("hash rolling operation target %q: %w", targetName, err)
		}
		status := opsRes.OpsRequest.Status.Components[targetName]
		status.TargetSpecHash = hash
		opsRes.OpsRequest.Status.Components[targetName] = status
	}
	return nil
}

func (c componentOpsHelper) saveLastConfigurations(opsRes *OpsResource,
	buildLastCompConfiguration func(compSpec appsv1.ClusterComponentSpec, obj ComponentOpsInterface) opsv1alpha1.LastComponentConfiguration) {
	setLastCompConfiguration := func(compSpec appsv1.ClusterComponentSpec,
		lastConfiguration *opsv1alpha1.LastConfiguration,
		componentName string) {
		obj, ok := c.componentOpsSet[componentName]
		if !ok {
			return
		}
		lastConfiguration.Components[componentName] = buildLastCompConfiguration(compSpec, obj)
	}

	// 1. record the volumeTemplate of cluster components
	lastConfiguration := &opsRes.OpsRequest.Status.LastConfiguration
	lastConfiguration.Components = map[string]opsv1alpha1.LastComponentConfiguration{}
	for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
		setLastCompConfiguration(v, lastConfiguration, v.Name)
	}
	// 2. record the volumeTemplate of sharding components
	for _, v := range opsRes.Cluster.Spec.Shardings {
		setLastCompConfiguration(v.Template, lastConfiguration, v.Name)
	}
}

// cancelComponentOps the common function to cancel th opsRequest which updates the component attributes.
func (c componentOpsHelper) cancelComponentOps(ctx context.Context,
	cli client.Client,
	opsRes *OpsResource,
	updateCompSpec func(lastConfig *opsv1alpha1.LastComponentConfiguration, comp *appsv1.ClusterComponentSpec)) error {
	rollBackCompSpec := func(compSpec *appsv1.ClusterComponentSpec,
		lastCompInfos map[string]opsv1alpha1.LastComponentConfiguration,
		componentName string) {
		lastConfig, ok := lastCompInfos[componentName]
		if !ok {
			return
		}
		updateCompSpec(&lastConfig, compSpec)
		lastCompInfos[componentName] = lastConfig
	}

	// 1. rollback the clusterComponentSpecs
	lastCompInfos := opsRes.OpsRequest.Status.LastConfiguration.Components
	for index := range opsRes.Cluster.Spec.ComponentSpecs {
		compSpec := &opsRes.Cluster.Spec.ComponentSpecs[index]
		rollBackCompSpec(compSpec, lastCompInfos, compSpec.Name)
	}
	// 2. rollback the shardings
	for index := range opsRes.Cluster.Spec.Shardings {
		sharding := &opsRes.Cluster.Spec.Shardings[index]
		rollBackCompSpec(&sharding.Template, lastCompInfos, sharding.Name)
	}
	if err := cli.Update(ctx, opsRes.Cluster); err != nil {
		return err
	}
	return c.recordRollingTargetSpecs(opsRes)
}

func componentStatusFailureCount(compStatus opsv1alpha1.OpsRequestComponentStatus) int32 {
	var count int32
	for _, v := range compStatus.ProgressDetails {
		if v.Status == opsv1alpha1.FailedProgressStatus {
			count++
		}
	}
	return count
}

func (c componentOpsHelper) getComponentOps(componentName string) (ComponentOpsInterface, bool) {
	if len(c.componentOpsSet) == 0 {
		return opsv1alpha1.ComponentOps{ComponentName: componentName}, true
	}
	compOps, ok := c.componentOpsSet[componentName]
	return compOps, ok
}

func (c componentOpsHelper) isHScaleShards(opsRequest *opsv1alpha1.OpsRequest, compOps ComponentOpsInterface) bool {
	if opsRequest.Spec.Type != opsv1alpha1.HorizontalScalingType {
		return false
	}
	return compOps.(opsv1alpha1.HorizontalScaling).Shards != nil
}

func (c componentOpsHelper) buildProgressResources(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	clusterDef *appsv1.ClusterDefinition,
	opsMessageKey string,
	loadComponentDefinition bool) ([]progressResource, error) {
	var progressResources []progressResource
	setProgressResource := func(compSpec *appsv1.ClusterComponentSpec, compOps ComponentOpsInterface,
		fullComponentName string, shards *int32) error {
		var componentDefinition *appsv1.ComponentDefinition
		if loadComponentDefinition && compSpec.ComponentDef != "" {
			componentDefinition = &appsv1.ComponentDefinition{}
			if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: compSpec.ComponentDef}, componentDefinition); err != nil {
				return err
			}
		}
		progressResources = append(progressResources, progressResource{
			opsMessageKey:     opsMessageKey,
			clusterComponent:  compSpec,
			clusterDef:        clusterDef,
			componentDef:      componentDefinition,
			compOps:           compOps,
			fullComponentName: fullComponentName,
			shards:            shards,
		})
		return nil
	}
	// 1. handle the component status
	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		compSpec := &opsRes.Cluster.Spec.ComponentSpecs[i]
		compOps, ok := c.getComponentOps(compSpec.Name)
		if !ok {
			continue
		}
		if err := setProgressResource(compSpec, compOps, compSpec.Name, nil); err != nil {
			return nil, err
		}
	}

	// 2. handle the sharding status.
	for i := range opsRes.Cluster.Spec.Shardings {
		spec := opsRes.Cluster.Spec.Shardings[i]
		compOps, ok := c.getComponentOps(spec.Name)
		if !ok {
			continue
		}
		if c.isHScaleShards(opsRes.OpsRequest, compOps) {
			if err := setProgressResource(&spec.Template, compOps, "", &spec.Shards); err != nil {
				return nil, err
			}
			continue
		}
		// handle the progress of the components of the sharding.
		shardingComps, err := sharding.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, spec.Name)
		if err != nil {
			return nil, err
		}
		for j := range shardingComps {
			if err = setProgressResource(&spec.Template, compOps,
				shardingComps[j].Labels[constant.KBAppComponentLabelKey], &spec.Shards); err != nil {
				return nil, err
			}
		}
	}
	return progressResources, nil
}

// reconcileActionWithComponentOps will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the common function to reconcile opsRequest status when the opsRequest will affect the lifecycle of the components.
func (c componentOpsHelper) reconcileActionWithComponentOps(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	opsMessageKey string,
	handleStatusProgress handleStatusProgressWithComponent,
) (opsv1alpha1.OpsPhase, time.Duration, error) {
	return c.reconcileActionWithComponentOpsPolicy(reqCtx, cli, opsRes, opsMessageKey, handleStatusProgress, false)
}

// reconcileRollingActionWithComponentOps fences rolling operations with the exact
// Cluster target generation and intent. Full-component operations use Cluster
// phase as their terminal result; partial operations use participating instances.
func (c componentOpsHelper) reconcileRollingActionWithComponentOps(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	opsMessageKey string,
	handleStatusProgress handleStatusProgressWithComponent,
) (opsv1alpha1.OpsPhase, time.Duration, error) {
	return c.reconcileActionWithComponentOpsPolicy(reqCtx, cli, opsRes, opsMessageKey, handleStatusProgress, true)
}

func (c componentOpsHelper) reconcileActionWithComponentOpsPolicy(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	opsMessageKey string,
	handleStatusProgress handleStatusProgressWithComponent,
	clusterStatusAuthoritative bool,
) (opsv1alpha1.OpsPhase, time.Duration, error) {
	if opsRes == nil {
		return "", 0, nil
	}
	var (
		opsRequestPhase        = opsv1alpha1.OpsRunningPhase
		opsRequest             = opsRes.OpsRequest
		expectProgressCount    int32
		completedProgressCount int32
		err                    error
		clusterDef             *appsv1.ClusterDefinition
		rollingProgress        = map[string]rollingTargetProgressState{}
	)
	if !clusterStatusAuthoritative && opsRes.Cluster.Spec.ClusterDef != "" {
		if clusterDef, err = getClusterDefByName(reqCtx.Ctx, cli, opsRes.Cluster.Spec.ClusterDef); err != nil {
			return opsRequestPhase, 0, err
		}
	}
	// if no specified components, we should check the all components phase of cluster.
	oldOpsRequest := opsRequest.DeepCopy()
	patch := client.MergeFrom(oldOpsRequest)
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{}
	}
	progressResources, err := c.buildProgressResources(reqCtx, cli, opsRes, clusterDef, opsMessageKey, !clusterStatusAuthoritative)
	if err != nil {
		return opsRequestPhase, 0, err
	}
	opsIsCompleted := true
	existFailure := false
	for i := range progressResources {
		pgResource := progressResources[i]
		var componentPhase appsv1.ComponentPhase
		if pgResource.shards == nil {
			status := opsRes.Cluster.Status.Components[pgResource.compOps.GetComponentName()]
			componentPhase = status.Phase
		} else {
			status := opsRes.Cluster.Status.Shardings[pgResource.compOps.GetComponentName()]
			componentPhase = status.Phase
		}
		opsCompStatus := opsRequest.Status.Components[pgResource.compOps.GetComponentName()]
		expectCount, completedCount, err := handleStatusProgress(reqCtx, cli, opsRes, &pgResource, &opsCompStatus)
		if err != nil {
			return opsRequestPhase, 0, err
		}
		expectProgressCount += expectCount
		completedProgressCount += completedCount
		if clusterStatusAuthoritative {
			targetName := pgResource.compOps.GetComponentName()
			state := rollingProgress[targetName]
			if state.resources == 0 {
				state.completed = true
			}
			state.resources++
			state.completed = state.completed && pgResource.rollingProgressCompleted
			state.failed = state.failed || pgResource.rollingProgressFailed
			state.partial = state.partial || pgResource.partialRollingTarget
			rollingProgress[targetName] = state
		} else {
			componentFailureCount := componentStatusFailureCount(opsCompStatus)
			if componentFailureCount > 0 {
				existFailure = true
			}
			// conditions whether ops is running:
			//  1. completedProgressCount is not equal to expectProgressCount.
			//  2. the component phase is not a terminal phase or no completed progress if the ops
			//  needs to wait for the component phase to reach a terminal state.
			switch {
			case expectCount != completedCount:
				opsIsCompleted = false
			case !pgResource.noWaitComponentCompleted &&
				(!slices.Contains(componentTerminalPhases(), componentPhase) || noAnyProgressCompleted(pgResource.clusterComponent.Replicas, completedCount)):
				opsIsCompleted = false
			}
		}
		opsCompStatus.Phase = componentPhase
		opsRequest.Status.Components[pgResource.compOps.GetComponentName()] = opsCompStatus
	}
	if clusterStatusAuthoritative {
		opsIsCompleted, existFailure = c.rollingTargetsState(opsRes, rollingProgress)
	}
	if clusterStatusAuthoritative && opsIsCompleted {
		completedProgressCount = expectProgressCount
	}
	opsRequest.Status.Progress = fmt.Sprintf("%d/%d", completedProgressCount, expectProgressCount)
	if !reflect.DeepEqual(opsRequest.Status, oldOpsRequest.Status) {
		if err = cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return opsRequestPhase, 0, err
		}
	}
	if existFailure {
		return opsv1alpha1.OpsFailedPhase, 0, nil
	}
	if !opsIsCompleted {
		return opsRequestPhase, 0, nil
	}
	return opsv1alpha1.OpsSucceedPhase, 0, nil
}

type rollingTargetProgressState struct {
	resources int
	completed bool
	failed    bool
	partial   bool
}

func (c componentOpsHelper) rollingTargetsState(
	opsRes *OpsResource,
	progress map[string]rollingTargetProgressState) (completed, failed bool) {
	completed = true
	for targetName := range c.componentOpsSet {
		var (
			phase              appsv1.ComponentPhase
			observedGeneration int64
			upToDate           bool
			found              bool
		)
		for i := range opsRes.Cluster.Spec.ComponentSpecs {
			if opsRes.Cluster.Spec.ComponentSpecs[i].Name != targetName {
				continue
			}
			status := opsRes.Cluster.Status.Components[targetName]
			phase, observedGeneration, upToDate = status.Phase, status.ObservedGeneration, status.UpToDate
			found = true
			break
		}
		if !found {
			for i := range opsRes.Cluster.Spec.Shardings {
				if opsRes.Cluster.Spec.Shardings[i].Name != targetName {
					continue
				}
				status := opsRes.Cluster.Status.Shardings[targetName]
				phase, observedGeneration, upToDate = status.Phase, status.ObservedGeneration, status.UpToDate
				found = true
				break
			}
		}

		opsCompStatus := opsRes.OpsRequest.Status.Components[targetName]
		opsCompStatus.Phase = phase
		opsRes.OpsRequest.Status.Components[targetName] = opsCompStatus

		if opsRes.Cluster.Generation < opsRes.OpsRequest.Status.ClusterGeneration {
			completed = false
			continue
		}

		targetSpec, targetFound := findRollingTargetSpec(opsRes.Cluster, targetName)
		if !targetFound {
			opsCompStatus.Reason = "ClusterSpecSuperseded"
			opsCompStatus.Message = "Cluster target submitted by this operation no longer exists"
			opsRes.OpsRequest.Status.Components[targetName] = opsCompStatus
			failed = true
			completed = false
			continue
		}
		currentSpecHash, err := rollingTargetSpecHash(targetSpec, c.componentOpsSet[targetName])
		if err != nil {
			opsCompStatus.Reason = "ClusterSpecSuperseded"
			opsCompStatus.Message = "Cluster target spec can no longer be matched to the intent submitted by this operation"
			opsRes.OpsRequest.Status.Components[targetName] = opsCompStatus
			failed = true
			completed = false
			continue
		}
		if opsCompStatus.TargetSpecHash == "" {
			// Running rolling operations created by an older controller do not
			// carry TargetSpecHash. Controller upgrades are expected to happen
			// without running operations, but adopting the current operation-owned
			// target is a safe best-effort fallback that avoids waiting forever.
			opsCompStatus.TargetSpecHash = currentSpecHash
			opsRes.OpsRequest.Status.Components[targetName] = opsCompStatus
		} else if currentSpecHash != opsCompStatus.TargetSpecHash {
			opsCompStatus.Reason = "ClusterSpecSuperseded"
			opsCompStatus.Message = "Cluster target spec no longer matches the intent submitted by this operation"
			opsRes.OpsRequest.Status.Components[targetName] = opsCompStatus
			failed = true
			completed = false
			continue
		}

		statusIsCurrent := found && rollingTargetStatusIsCurrent(
			opsRes.Cluster.Generation, opsRes.OpsRequest.Status.ClusterGeneration, observedGeneration, upToDate)
		if !statusIsCurrent {
			completed = false
			continue
		}

		progressState, hasProgress := progress[targetName]
		switch {
		case hasProgress && progressState.partial && !slices.Contains(componentTerminalPhases(), phase):
			// InstanceStatus is an eventually consistent observation. In the first
			// reconciliation after a new InstanceSet generation is observed, it may
			// still contain the previous per-instance values. A terminal aggregate
			// phase is therefore required as the stability barrier, while the
			// participating instances remain authoritative for the result.
			completed = false
		case hasProgress && progressState.partial && progressState.failed:
			failed = true
			completed = false
		case hasProgress && progressState.partial && progressState.completed:
			// Aggregate Component health can include instances outside a partial operation.
		case hasProgress && progressState.partial:
			completed = false
		case phase == appsv1.FailedComponentPhase:
			failed = true
			completed = false
		case phase == appsv1.RunningComponentPhase || phase == appsv1.StoppedComponentPhase:
		default:
			completed = false
		}
	}
	return completed, failed
}

func rollingTargetStatusIsCurrent(clusterGeneration, opsClusterGeneration, observedGeneration int64, upToDate bool) bool {
	return clusterGeneration >= opsClusterGeneration &&
		observedGeneration == clusterGeneration && upToDate
}

func noAnyProgressCompleted(replicas, completedCount int32) bool {
	return replicas > 0 && completedCount == 0
}

func hasIntersectionCompOpsList[T ComponentOpsInterface, S ComponentOpsInterface](currCompOpsMap map[string]T, list []S) bool {
	for _, comp := range list {
		if _, ok := currCompOpsMap[comp.GetComponentName()]; ok {
			return true
		}
	}
	return false
}

func componentTerminalPhases() []appsv1.ComponentPhase {
	return []appsv1.ComponentPhase{
		appsv1.RunningComponentPhase,
		appsv1.StoppedComponentPhase,
		appsv1.FailedComponentPhase,
	}
}
