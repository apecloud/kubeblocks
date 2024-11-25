/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"fmt"
	"reflect"
	"slices"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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
	return cli.Update(ctx, opsRes.Cluster)
}

func (c componentOpsHelper) existFailure(ops *opsv1alpha1.OpsRequest, componentName string) bool {
	for _, v := range ops.Status.Components[componentName].ProgressDetails {
		if v.Status == opsv1alpha1.FailedProgressStatus {
			return true
		}
	}
	return false
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
	opsMessageKey string) ([]progressResource, error) {
	var progressResources []progressResource
	setProgressResource := func(compSpec *appsv1.ClusterComponentSpec, compOps ComponentOpsInterface,
		fullComponentName string, shards *int32) error {
		var componentDefinition *appsv1.ComponentDefinition
		if compSpec.ComponentDef != "" {
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
		sharding := opsRes.Cluster.Spec.Shardings[i]
		compOps, ok := c.getComponentOps(sharding.Name)
		if !ok {
			continue
		}
		if c.isHScaleShards(opsRes.OpsRequest, compOps) {
			if err := setProgressResource(&sharding.Template, compOps, "", &sharding.Shards); err != nil {
				return nil, err
			}
			continue
		}
		// handle the progress of the components of the sharding.
		shardingComps, err := intctrlutil.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, sharding.Name)
		if err != nil {
			return nil, err
		}
		for j := range shardingComps {
			if err = setProgressResource(&sharding.Template, compOps,
				shardingComps[j].Labels[constant.KBAppComponentLabelKey], &sharding.Shards); err != nil {
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
	)
	if opsRes.Cluster.Spec.ClusterDef != "" {
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
	progressResources, err := c.buildProgressResources(reqCtx, cli, opsRes, clusterDef, opsMessageKey)
	if err != nil {
		return opsRequestPhase, 0, err
	}
	opsIsCompleted := true
	existFailure := false
	for i := range progressResources {
		pgResource := progressResources[i]
		opsCompStatus := opsRequest.Status.Components[pgResource.compOps.GetComponentName()]
		expectCount, completedCount, err := handleStatusProgress(reqCtx, cli, opsRes, &pgResource, &opsCompStatus)
		if err != nil {
			return opsRequestPhase, 0, err
		}
		expectProgressCount += expectCount
		completedProgressCount += completedCount
		if c.existFailure(opsRes.OpsRequest, pgResource.compOps.GetComponentName()) {
			existFailure = true
		}
		var componentPhase appsv1.ComponentPhase
		if pgResource.shards == nil {
			componentPhase = opsRes.Cluster.Status.Components[pgResource.compOps.GetComponentName()].Phase
		} else {
			componentPhase = opsRes.Cluster.Status.Shardings[pgResource.compOps.GetComponentName()].Phase
		}
		// conditions whether ops is running:
		//  1. completedProgressCount is not equal to expectProgressCount.
		//  2. the component phase is not a terminal phase or no completed progress if the ops
		//  needs to wait for the component phase to reach a terminal state.
		if expectCount != completedCount {
			opsIsCompleted = false
		} else if !pgResource.noWaitComponentCompleted &&
			(!slices.Contains(componentTerminalPhases(), componentPhase) || completedCount == 0) {
			opsIsCompleted = false
		}
		opsCompStatus.Phase = componentPhase
		opsRequest.Status.Components[pgResource.compOps.GetComponentName()] = opsCompStatus
	}
	// TODO: wait for sharding cluster to completed for next opsRequest.
	opsRequest.Status.Progress = fmt.Sprintf("%d/%d", completedProgressCount, expectProgressCount)
	if !reflect.DeepEqual(opsRequest.Status, oldOpsRequest.Status) {
		if err = cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return opsRequestPhase, 0, err
		}
	}
	if !opsIsCompleted {
		return opsRequestPhase, 0, nil
	}
	if existFailure {
		return opsv1alpha1.OpsFailedPhase, 0, nil
	}
	return opsv1alpha1.OpsSucceedPhase, 0, nil
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
