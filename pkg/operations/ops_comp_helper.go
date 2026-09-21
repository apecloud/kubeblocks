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
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
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

func (c componentOpsHelper) getComponentOps(componentName string) (ComponentOpsInterface, bool) {
	if len(c.componentOpsSet) == 0 {
		return opsv1alpha1.ComponentOps{ComponentName: componentName}, true
	}
	compOps, ok := c.componentOpsSet[componentName]
	return compOps, ok
}

type instanceProgressResource struct {
	opsMessageKey     string
	fullComponentName string
	clusterComponent  *appsv1.ClusterComponentSpec
	compOps           ComponentOpsInterface
}

func (c componentOpsHelper) buildInstanceProgressResources(reqCtx intctrlutil.RequestCtx, cli client.Client,
	opsRes *OpsResource, opsMessageKey string) ([]instanceProgressResource, error) {
	var instanceProgressResources []instanceProgressResource
	setProgressResource := func(compSpec *appsv1.ClusterComponentSpec, compOps ComponentOpsInterface, fullComponentName string) {
		instanceProgressResources = append(instanceProgressResources, instanceProgressResource{
			opsMessageKey:     opsMessageKey,
			clusterComponent:  compSpec,
			compOps:           compOps,
			fullComponentName: fullComponentName,
		})
	}
	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		compSpec := &opsRes.Cluster.Spec.ComponentSpecs[i]
		compOps, ok := c.getComponentOps(compSpec.Name)
		if ok {
			setProgressResource(compSpec, compOps, compSpec.Name)
		}
	}
	for i := range opsRes.Cluster.Spec.Shardings {
		shardingSpec := &opsRes.Cluster.Spec.Shardings[i]
		compOps, ok := c.getComponentOps(shardingSpec.Name)
		if !ok {
			continue
		}
		components, err := sharding.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, shardingSpec.Name)
		if err != nil {
			return nil, err
		}
		for j := range components {
			setProgressResource(&shardingSpec.Template, compOps,
				components[j].Labels[constant.KBAppComponentLabelKey])
		}
	}
	return instanceProgressResources, nil
}

type instanceProgress struct {
	expectedCount        int32
	completedCount       int32
	succeededCount       int32
	observationsComplete bool
	details              []opsv1alpha1.ProgressStatusDetail
}

func (c componentOpsHelper) reconcileRunningAction(reqCtx intctrlutil.RequestCtx, cli client.Client,
	opsRes *OpsResource, opsMessageKey string) (opsv1alpha1.OpsPhase, time.Duration, error) {
	if opsRes == nil {
		return "", 0, nil
	}
	opsRequest := opsRes.OpsRequest
	oldOpsRequest := opsRequest.DeepCopy()
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{}
	}
	instanceProgressResources, err := c.buildInstanceProgressResources(reqCtx, cli, opsRes, opsMessageKey)
	if err != nil {
		return opsv1alpha1.OpsRunningPhase, 0, err
	}
	current := c.emptyInstanceProgress(opsRes)
	componentCounts := map[string]int32{}
	observationsComplete := true
	var expectedCount, completedCount, succeededCount int32
	for i := range instanceProgressResources {
		pgResource := &instanceProgressResources[i]
		componentName := pgResource.compOps.GetComponentName()
		componentCounts[componentName]++
		its := &workloads.InstanceSet{}
		key := client.ObjectKey{Namespace: opsRes.Cluster.Namespace,
			Name: constant.GenerateClusterComponentName(opsRes.Cluster.Name, pgResource.fullComponentName)}
		if err := cli.Get(reqCtx.Ctx, key, its); err != nil {
			if !apierrors.IsNotFound(err) {
				return opsv1alpha1.OpsRunningPhase, 0, err
			}
			expectedCount += pgResource.clusterComponent.Replicas
			observationsComplete = false
			continue
		}
		result := handleRunningInstanceProgress(opsRes, pgResource, its)
		expectedCount += result.expectedCount
		completedCount += result.completedCount
		succeededCount += result.succeededCount
		observationsComplete = observationsComplete && result.observationsComplete
		current[componentName] = append(current[componentName], result.details...)
	}
	for i := range opsRes.Cluster.Spec.Shardings {
		sharding := &opsRes.Cluster.Spec.Shardings[i]
		if _, selected := c.getComponentOps(sharding.Name); selected && componentCounts[sharding.Name] != sharding.Shards {
			observationsComplete = false
		}
	}
	phase := c.componentActionPhase(opsRes, appsv1.RunningComponentPhase)
	if phase == opsv1alpha1.OpsSucceedPhase &&
		(!observationsComplete || completedCount != expectedCount || succeededCount != expectedCount) {
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

func (c componentOpsHelper) emptyInstanceProgress(opsRes *OpsResource) map[string][]opsv1alpha1.ProgressStatusDetail {
	progress := make(map[string][]opsv1alpha1.ProgressStatusDetail)
	for name := range opsRes.OpsRequest.Status.Components {
		if _, ok := c.getComponentOps(name); ok {
			progress[name] = nil
		}
	}
	cluster := opsRes.Cluster
	for i := range cluster.Spec.ComponentSpecs {
		name := cluster.Spec.ComponentSpecs[i].Name
		if _, ok := c.getComponentOps(name); ok {
			progress[name] = nil
		}
	}
	for i := range cluster.Spec.Shardings {
		name := cluster.Spec.Shardings[i].Name
		if _, ok := c.getComponentOps(name); ok {
			progress[name] = nil
		}
	}
	return progress
}

func (c componentOpsHelper) componentActionPhase(opsRes *OpsResource, terminalPhase appsv1.ComponentPhase) opsv1alpha1.OpsPhase {
	if opsRes.Cluster.Generation < opsRes.OpsRequest.Status.ClusterGeneration {
		return opsv1alpha1.OpsRunningPhase
	}
	actionPhase := opsv1alpha1.OpsSucceedPhase
	matched := 0
	setProcessing := func() {
		if actionPhase != opsv1alpha1.OpsFailedPhase {
			actionPhase = opsv1alpha1.OpsRunningPhase
		}
	}
	checkTarget := func(name string, componentPhase appsv1.ComponentPhase, observedGeneration int64, upToDate bool) {
		matched++
		compStatus := opsRes.OpsRequest.Status.Components[name]
		compStatus.Phase = componentPhase
		opsRes.OpsRequest.Status.Components[name] = compStatus
		if observedGeneration != opsRes.Cluster.Generation || !upToDate {
			setProcessing()
			return
		}
		if componentPhase == appsv1.FailedComponentPhase {
			actionPhase = opsv1alpha1.OpsFailedPhase
			return
		}
		if componentPhase != terminalPhase {
			setProcessing()
		}
	}
	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		name := opsRes.Cluster.Spec.ComponentSpecs[i].Name
		if _, ok := c.getComponentOps(name); ok {
			status := opsRes.Cluster.Status.Components[name]
			checkTarget(name, status.Phase, status.ObservedGeneration, status.UpToDate)
		}
	}
	for i := range opsRes.Cluster.Spec.Shardings {
		name := opsRes.Cluster.Spec.Shardings[i].Name
		if _, ok := c.getComponentOps(name); ok {
			status := opsRes.Cluster.Status.Shardings[name]
			checkTarget(name, status.Phase, status.ObservedGeneration, status.UpToDate)
		}
	}
	if len(c.componentOpsSet) > 0 && matched != len(c.componentOpsSet) {
		setProcessing()
	}
	return actionPhase
}

func syncCurrentProgressDetails(opsRes *OpsResource, current map[string][]opsv1alpha1.ProgressStatusDetail) {
	for componentName, details := range current {
		slices.SortFunc(details, func(a, b opsv1alpha1.ProgressStatusDetail) int {
			return strings.Compare(a.ObjectKey, b.ObjectKey)
		})
		compStatus := opsRes.OpsRequest.Status.Components[componentName]
		for i := range details {
			detail := &details[i]
			existing := findStatusProgressDetail(compStatus.ProgressDetails, detail.ObjectKey)
			if existing != nil {
				detail.StartTime = existing.StartTime
				if existing.Status == detail.Status {
					detail.EndTime = existing.EndTime
				}
			}
			updateProgressDetailTime(detail)
			if existing == nil || existing.Status != detail.Status || existing.Message != detail.Message {
				sendProgressDetailEvent(opsRes.Recorder, opsRes.OpsRequest, *detail)
			}
		}
		compStatus.ProgressDetails = details
		opsRes.OpsRequest.Status.Components[componentName] = compStatus
	}
}

func patchCurrentProgress(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource,
	oldOpsRequest *opsv1alpha1.OpsRequest, current map[string][]opsv1alpha1.ProgressStatusDetail,
	completedCount, expectedCount int32) error {
	syncCurrentProgressDetails(opsRes, current)
	opsRes.OpsRequest.Status.Progress = fmt.Sprintf("%d/%d", completedCount, expectedCount)
	if reflect.DeepEqual(opsRes.OpsRequest.Status, oldOpsRequest.Status) {
		return nil
	}
	return cli.Status().Patch(reqCtx.Ctx, opsRes.OpsRequest, client.MergeFrom(oldOpsRequest))
}

func rollingActionGenerationPending(opsRes *OpsResource) bool {
	return opsRes != nil && opsRes.Cluster != nil && opsRes.OpsRequest != nil &&
		opsRes.Cluster.Generation < opsRes.OpsRequest.Status.ClusterGeneration
}

func hasIntersectionCompOpsList[T ComponentOpsInterface, S ComponentOpsInterface](currCompOpsMap map[string]T, list []S) bool {
	for _, comp := range list {
		if _, ok := currCompOpsMap[comp.GetComponentName()]; ok {
			return true
		}
	}
	return false
}
