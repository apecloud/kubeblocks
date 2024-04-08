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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlcomp "github.com/apecloud/kubeblocks/pkg/controller/component"
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
	var (
		horizontalScalingMap = opsRes.OpsRequest.Spec.ToHorizontalScalingListToMap()
		horizontalScaling    appsv1alpha1.HorizontalScaling
		ok                   bool
	)
	for index, component := range opsRes.Cluster.Spec.ComponentSpecs {
		if horizontalScaling, ok = horizontalScalingMap[component.Name]; !ok {
			continue
		}

		instances := buildInstances(opsRes.Cluster.Spec.ComponentSpecs[index], horizontalScaling)
		opsRes.Cluster.Spec.ComponentSpecs[index].Instances = instances
		if horizontalScaling.OfflineInstances != nil {
			opsRes.Cluster.Spec.ComponentSpecs[index].OfflineInstances = horizontalScaling.OfflineInstances
		}
		opsRes.Cluster.Spec.ComponentSpecs[index].Replicas = horizontalScaling.Replicas
	}
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

func buildInstances(componentSpec appsv1alpha1.ClusterComponentSpec, horizontalScaling appsv1alpha1.HorizontalScaling) []appsv1alpha1.InstanceTemplate {
	if componentSpec.Instances == nil && horizontalScaling.Instances == nil {
		return nil
	}
	templates := componentSpec.Instances
	for _, instance := range horizontalScaling.Instances {
		found := false
		for i := range templates {
			if templates[i].Name != instance.Name {
				continue
			}
			templates[i].Replicas = instance.Replicas
			found = true
		}
		if !found {
			templates = append(templates, instance)
		}
	}
	return templates
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for horizontal scaling opsRequest.
func (hs horizontalScalingOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	handleComponentProgress := func(
		reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes progressResource,
		compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
		return handleComponentProgressForScalingReplicas(reqCtx, cli, opsRes, pgRes, compStatus, hs.getExpectReplicas)
	}
	return reconcileActionWithComponentOps(reqCtx, cli, opsRes, "", syncOverrideByOpsForScaleReplicas, handleComponentProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (hs horizontalScalingOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	opsRequest := opsRes.OpsRequest
	lastComponentInfo := map[string]appsv1alpha1.LastComponentConfiguration{}
	componentNameMap := opsRequest.Spec.ToHorizontalScalingListToMap()
	for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
		hsInfo, ok := componentNameMap[v.Name]
		if !ok {
			continue
		}
		copyReplicas := v.Replicas
		var (
			copyInstances        *[]appsv1alpha1.InstanceTemplate
			copyOfflineInstances *[]string
		)
		if len(v.Instances) > 0 {
			var instances []appsv1alpha1.InstanceTemplate
			instances = append(instances, v.Instances...)
			copyInstances = &instances
		}
		if len(v.OfflineInstances) > 0 {
			var offlineInstances []string
			offlineInstances = append(offlineInstances, v.OfflineInstances...)
			copyOfflineInstances = &offlineInstances
		}
		lastCompConfiguration := appsv1alpha1.LastComponentConfiguration{
			Replicas:         &copyReplicas,
			Instances:        copyInstances,
			OfflineInstances: copyOfflineInstances,
		}
		if hsInfo.Replicas < copyReplicas {
			podNames, err := getCompPodNamesBeforeScaleDownReplicas(reqCtx, cli, *opsRes.Cluster, v.Name)
			if err != nil {
				return err
			}
			lastCompConfiguration.TargetResources = map[appsv1alpha1.ComponentResourceKey][]string{
				appsv1alpha1.PodsCompResourceKey: podNames,
			}
		}
		lastComponentInfo[v.Name] = lastCompConfiguration
	}
	opsRequest.Status.LastConfiguration.Components = lastComponentInfo
	return nil
}

func (hs horizontalScalingOpsHandler) getExpectReplicas(opsRequest *appsv1alpha1.OpsRequest, componentName string) *int32 {
	compStatus := opsRequest.Status.Components[componentName]
	if compStatus.OverrideBy != nil {
		return compStatus.OverrideBy.Replicas
	}
	for _, v := range opsRequest.Spec.HorizontalScalingList {
		if v.ComponentName == componentName {
			return &v.Replicas
		}
	}
	return nil
}

// getCompPodNamesBeforeScaleDownReplicas gets the component pod names before scale down replicas.
func getCompPodNamesBeforeScaleDownReplicas(reqCtx intctrlutil.RequestCtx,
	cli client.Client, cluster appsv1alpha1.Cluster, compName string) ([]string, error) {
	podNames := make([]string, 0)
	podList, err := intctrlcomp.GetComponentPodList(reqCtx.Ctx, cli, cluster, compName)
	if err != nil {
		return podNames, err
	}
	for _, v := range podList.Items {
		podNames = append(podNames, v.Name)
	}
	return podNames, nil
}

// Cancel this function defines the cancel horizontalScaling action.
func (hs horizontalScalingOpsHandler) Cancel(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	for _, v := range opsRes.OpsRequest.Status.Components {
		if v.OverrideBy != nil && v.OverrideBy.OpsName != "" {
			return intctrlutil.NewErrorf(intctrlutil.ErrorIgnoreCancel, `can not cancel the opsRequest due to another opsRequest "%s" is running`, v.OverrideBy.OpsName)
		}
	}
	return cancelComponentOps(reqCtx.Ctx, cli, opsRes, func(lastConfig *appsv1alpha1.LastComponentConfiguration, comp *appsv1alpha1.ClusterComponentSpec) error {
		if lastConfig.Replicas == nil {
			return nil
		}
		podNames, err := getCompPodNamesBeforeScaleDownReplicas(reqCtx, cli, *opsRes.Cluster, comp.Name)
		if err != nil {
			return err
		}
		if lastConfig.TargetResources == nil {
			lastConfig.TargetResources = map[appsv1alpha1.ComponentResourceKey][]string{}
		}
		lastConfig.TargetResources[appsv1alpha1.PodsCompResourceKey] = podNames
		comp.Replicas = *lastConfig.Replicas
		if lastConfig.Instances != nil {
			comp.Instances = *lastConfig.Instances
		}
		if lastConfig.OfflineInstances != nil {
			comp.OfflineInstances = *lastConfig.OfflineInstances
		}
		return nil
	})
}
