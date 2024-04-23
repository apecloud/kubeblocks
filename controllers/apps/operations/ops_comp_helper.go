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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type ComponentOpsInteface interface {
	GetComponentName() string
	GetShardingName() string
}

type componentOpsHelper struct {
	componentOpsSet map[string]ComponentOpsInteface
}

func newComponentOpsHelper[T ComponentOpsInteface](compOpsList []T) componentOpsHelper {
	compOpsHelper := componentOpsHelper{
		componentOpsSet: make(map[string]ComponentOpsInteface),
	}
	for i := range compOpsList {
		compOps := compOpsList[i]
		compOpsKey := getCompOpsKey(compOps.GetShardingName(), compOps.GetComponentName())
		compOpsHelper.componentOpsSet[compOpsKey] = compOps
	}
	return compOpsHelper
}

func (c componentOpsHelper) isSharding(compOps ComponentOpsInteface) bool {
	return compOps.GetShardingName() != ""
}

func (c componentOpsHelper) getOpsComponentAndShardStatus(opsRequest *appsv1alpha1.OpsRequest, comOps ComponentOpsInteface) appsv1alpha1.OpsRequestComponentStatus {
	compKey := getCompOpsKey(comOps.GetShardingName(), comOps.GetComponentName())
	return opsRequest.Status.Components[compKey]
}

func (c componentOpsHelper) setOpsComponentAndShardStatus(opsRequest *appsv1alpha1.OpsRequest,
	opsComStatus appsv1alpha1.OpsRequestComponentStatus,
	comOps ComponentOpsInteface) {
	compKey := getCompOpsKey(comOps.GetShardingName(), comOps.GetComponentName())
	opsRequest.Status.Components[compKey] = opsComStatus
}

func (c componentOpsHelper) updateClusterComponentsAndShardings(cluster *appsv1alpha1.Cluster,
	updateFunc func(compSpec *appsv1alpha1.ClusterComponentSpec, compOpsItem ComponentOpsInteface)) {
	updateComponentSpecs := func(compSpec *appsv1alpha1.ClusterComponentSpec, compOpsKey string) {
		if obj, ok := c.componentOpsSet[compOpsKey]; ok {
			updateFunc(compSpec, obj)
		}
	}
	// 1. update the components
	for index := range cluster.Spec.ComponentSpecs {
		comSpec := &cluster.Spec.ComponentSpecs[index]
		updateComponentSpecs(comSpec, comSpec.Name)
	}
	// 1. update the sharding components
	for index := range cluster.Spec.ShardingSpecs {
		shardingSpec := &cluster.Spec.ShardingSpecs[index]
		updateComponentSpecs(&shardingSpec.Template, getShardingKey(shardingSpec.Name))
	}
}

func (c componentOpsHelper) saveLastConfigurations(opsRes *OpsResource,
	buildLastCompConfiguration func(compSpec appsv1alpha1.ClusterComponentSpec, obj ComponentOpsInteface) appsv1alpha1.LastComponentConfiguration) {
	setLastCompConfiguration := func(compSpec appsv1alpha1.ClusterComponentSpec,
		lastConfiguration *appsv1alpha1.LastConfiguration,
		compOpsKey string) {
		obj, ok := c.componentOpsSet[compOpsKey]
		if !ok {
			return
		}
		lastConfiguration.Components[compOpsKey] = buildLastCompConfiguration(compSpec, obj)
	}

	// 1. record the volumeTemplate of cluster components
	lastConfiguration := &opsRes.OpsRequest.Status.LastConfiguration
	lastConfiguration.Components = map[string]appsv1alpha1.LastComponentConfiguration{}
	for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
		setLastCompConfiguration(v, lastConfiguration, v.Name)
	}
	// 2. record the volumeTemplate of sharding components
	for _, v := range opsRes.Cluster.Spec.ShardingSpecs {
		setLastCompConfiguration(v.Template, lastConfiguration, getShardingKey(v.Name))
	}
}

// cancelComponentOps the common function to cancel th opsRequest which updates the component attributes.
func (c componentOpsHelper) cancelComponentOps(ctx context.Context,
	cli client.Client,
	opsRes *OpsResource,
	updateCompSpec func(lastConfig *appsv1alpha1.LastComponentConfiguration, comp *appsv1alpha1.ClusterComponentSpec)) error {
	rollBackCompSpec := func(compSpec *appsv1alpha1.ClusterComponentSpec,
		lastCompInfos map[string]appsv1alpha1.LastComponentConfiguration,
		compOpsKey string) {
		lastConfig, ok := lastCompInfos[compOpsKey]
		if !ok {
			return
		}
		updateCompSpec(&lastConfig, compSpec)
		lastCompInfos[compOpsKey] = lastConfig
	}

	// 1. rollback the clusterComponentSpecs
	lastCompInfos := opsRes.OpsRequest.Status.LastConfiguration.Components
	for index := range opsRes.Cluster.Spec.ComponentSpecs {
		compSpec := &opsRes.Cluster.Spec.ComponentSpecs[index]
		rollBackCompSpec(compSpec, lastCompInfos, compSpec.Name)
	}
	// 2. rollback the shardingSpecs
	for index := range opsRes.Cluster.Spec.ShardingSpecs {
		shardingSpec := &opsRes.Cluster.Spec.ShardingSpecs[index]
		rollBackCompSpec(&shardingSpec.Template, lastCompInfos, getShardingKey(shardingSpec.Name))
	}
	return cli.Update(ctx, opsRes.Cluster)
}

// reconcileActionWithComponentOps will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the common function to reconcile opsRequest status when the opsRequest will affect the lifecycle of the components.
func (c componentOpsHelper) reconcileActionWithComponentOps(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	opsMessageKey string,
	syncOverrideBy syncOverrideByOps,
	handleStatusProgress handleStatusProgressWithComponent,
) (appsv1alpha1.OpsPhase, time.Duration, error) {
	if opsRes == nil {
		return "", 0, nil
	}
	var (
		opsRequestPhase        = appsv1alpha1.OpsRunningPhase
		opsRequest             = opsRes.OpsRequest
		isFailed               bool
		expectProgressCount    int32
		completedProgressCount int32
		requeueTimeAfterFailed time.Duration
		err                    error
		clusterDef             *appsv1alpha1.ClusterDefinition
	)
	if opsRes.Cluster.Spec.ClusterDefRef != "" {
		if clusterDef, err = getClusterDefByName(reqCtx.Ctx, cli, opsRes.Cluster.Spec.ClusterDefRef); err != nil {
			return opsRequestPhase, 0, err
		}
	}
	// if no specified components, we should check the all components phase of cluster.
	oldOpsRequest := opsRequest.DeepCopy()
	patch := client.MergeFrom(oldOpsRequest)
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = map[string]appsv1alpha1.OpsRequestComponentStatus{}
	}
	if syncOverrideBy != nil {
		if err = syncOverrideBy(reqCtx, cli, opsRes); err != nil {
			return "", 0, nil
		}
	}
	var progressResources []progressResource
	setProgressResource := func(compSpec *appsv1alpha1.ClusterComponentSpec, compOps ComponentOpsInteface, fullComponentName string) error {
		var componentDefinition *appsv1alpha1.ComponentDefinition
		if compSpec.ComponentDef != "" {
			componentDefinition = &appsv1alpha1.ComponentDefinition{}
			if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: compSpec.ComponentDef}, componentDefinition); err != nil {
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
		})
		return nil
	}
	getCompOps := func(shardingName, componentName string) (ComponentOpsInteface, bool) {
		if len(c.componentOpsSet) == 0 {
			return appsv1alpha1.ComponentOps{ComponentName: componentName, ShardingName: shardingName}, true
		}
		compOps, ok := c.componentOpsSet[getCompOpsKey(shardingName, componentName)]
		return compOps, ok
	}
	// 1. handle the component status
	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		compSpec := &opsRes.Cluster.Spec.ComponentSpecs[i]
		compOps, ok := getCompOps("", compSpec.Name)
		if !ok {
			continue
		}
		if err = setProgressResource(compSpec, compOps, compSpec.Name); err != nil {
			return opsRequestPhase, 0, err
		}
	}

	// 2. handle the sharding status.
	for i := range opsRes.Cluster.Spec.ShardingSpecs {
		shardingSpec := opsRes.Cluster.Spec.ShardingSpecs[i]
		compOps, ok := getCompOps(shardingSpec.Name, "")
		if !ok {
			continue
		}
		// handle the progress of the components of the sharding.
		shardingComps, err := intctrlutil.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, &shardingSpec)
		if err != nil {
			return opsRequestPhase, 0, err
		}
		for j := range shardingComps {
			if err = setProgressResource(&shardingSpec.Template, compOps,
				shardingComps[j].Labels[constant.KBAppComponentLabelKey]); err != nil {
				return opsRequestPhase, 0, err
			}
		}
	}
	var waitCompleted bool
	for _, pgResource := range progressResources {
		opsCompStatus := c.getOpsComponentAndShardStatus(opsRequest, pgResource.compOps)
		expectCount, completedCount, err := handleStatusProgress(reqCtx, cli, opsRes, pgResource, &opsCompStatus)
		if err != nil {
			if intctrlutil.IsTargetError(err, intctrlutil.ErrorWaitCacheRefresh) {
				return opsRequestPhase, time.Second, nil
			}
			return opsRequestPhase, 0, err
		}
		expectProgressCount += expectCount
		completedProgressCount += completedCount
		if !c.isSharding(pgResource.compOps) {
			lastFailedTime := opsCompStatus.LastFailedTime
			componentPhase := opsRes.Cluster.Status.Components[pgResource.compOps.GetComponentName()].Phase
			if isFailedOrAbnormal(componentPhase) {
				isFailed = true
				if lastFailedTime.IsZero() {
					lastFailedTime = metav1.Now()
				}
				if time.Now().Before(lastFailedTime.Add(componentFailedTimeout)) {
					requeueTimeAfterFailed = componentFailedTimeout - time.Since(lastFailedTime.Time)
				}
			} else if !lastFailedTime.IsZero() {
				// reset lastFailedTime if component is not failed
				lastFailedTime = metav1.Time{}
			}
			if opsCompStatus.Phase != componentPhase {
				opsCompStatus.Phase = componentPhase
				opsCompStatus.LastFailedTime = lastFailedTime
			}
			// wait the component to complete
			if !pgResource.noWaitComponentCompleted && !isComponentCompleted(componentPhase) {
				waitCompleted = true
			}
		}
		c.setOpsComponentAndShardStatus(opsRequest, opsCompStatus, pgResource.compOps)
	}
	// TODO: wait for sharding cluster to completed for next opsRequest.
	opsRequest.Status.Progress = fmt.Sprintf("%d/%d", completedProgressCount, expectProgressCount)
	if !reflect.DeepEqual(opsRequest.Status, oldOpsRequest.Status) {
		if err = cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return opsRequestPhase, 0, err
		}
	}
	if waitCompleted || completedProgressCount != expectProgressCount {
		return opsRequestPhase, 0, nil
	}
	if isFailed {
		if requeueTimeAfterFailed != 0 {
			// component failure may be temporary, waiting for component failure timeout.
			return opsRequestPhase, requeueTimeAfterFailed, nil
		}
		return appsv1alpha1.OpsFailedPhase, 0, nil
	}
	return appsv1alpha1.OpsSucceedPhase, 0, nil
}

func getCompOpsKey(shardingName, componentName string) string {
	if shardingName != "" {
		return getShardingKey(shardingName)
	}
	return componentName
}

func getShardingKey(shardingName string) string {
	return fmt.Sprintf("sharding/%s", shardingName)
}
