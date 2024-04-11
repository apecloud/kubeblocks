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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type verticalScalingHandler struct{}

var _ OpsHandler = verticalScalingHandler{}

func init() {
	vsHandler := verticalScalingHandler{}
	verticalScalingBehaviour := OpsBehaviour{
		// if cluster is Abnormal or Failed, new opsRequest may can repair it.
		FromClusterPhases: appsv1alpha1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1alpha1.UpdatingClusterPhase,
		OpsHandler:        vsHandler,
		QueueByCluster:    true,
		CancelFunc:        vsHandler.Cancel,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.VerticalScalingType, verticalScalingBehaviour)
}

// ActionStartedCondition the started condition when handle the vertical scaling request.
func (vs verticalScalingHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewVerticalScalingCondition(opsRes.OpsRequest), nil
}

// Action modifies cluster component resources according to
// the definition of opsRequest with spec.componentNames and spec.componentOps.verticalScaling
func (vs verticalScalingHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	verticalScalingMap := opsRes.OpsRequest.Spec.ToVerticalScalingListToMap()
	for index, component := range opsRes.Cluster.Spec.ComponentSpecs {
		verticalScaling, ok := verticalScalingMap[component.Name]
		if !ok {
			continue
		}
		component.Resources = verticalScaling.ResourceRequirements
		opsRes.Cluster.Spec.ComponentSpecs[index] = component
	}
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for vertical scaling opsRequest.
func (vs verticalScalingHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	return reconcileActionWithComponentOps(reqCtx, cli, opsRes, "vertical scale", vs.syncOverrideByOps, handleComponentStatusProgress)
}

func (vs verticalScalingHandler) syncOverrideByOps(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	runningOpsRequests, err := getRunningOpsRequestsWithSameKind(reqCtx, cli, opsRes.Cluster, appsv1alpha1.VerticalScalingType)
	if err != nil || len(runningOpsRequests) == 0 {
		return err
	}
	// get the latest opsName which has the same resource with the component resource.
	getTheLatestOpsName := func(compName string, compResource corev1.ResourceRequirements) string {
		for _, ops := range runningOpsRequests {
			for _, v := range ops.Spec.VerticalScalingList {
				if v.ComponentName == compName && v.ResourceRequirements.String() == compResource.String() {
					return ops.Name
				}
			}
		}
		return ""
	}
	compResourceMap := map[string]corev1.ResourceRequirements{}
	for _, comp := range opsRes.Cluster.Spec.ComponentSpecs {
		compResourceMap[comp.Name] = comp.Resources
	}
	// checks if the resources applied by the current opsRequest matches the desired resources for the component.
	// if not matched, set the OverrideB info in the opsRequest.status.components.
	for _, opsComp := range opsRes.OpsRequest.Spec.VerticalScalingList {
		compResource, ok := compResourceMap[opsComp.ComponentName]
		if !ok || compResource.String() == opsComp.ResourceRequirements.String() {
			continue
		}
		// if the component resource of the cluster has changed, indicates the changes has been overwritten by other opsRequest.
		componentStatus := opsRes.OpsRequest.Status.Components[opsComp.ComponentName]
		componentStatus.OverrideBy = &appsv1alpha1.OverrideBy{
			OpsName: getTheLatestOpsName(opsComp.ComponentName, compResource),
			LastComponentConfiguration: appsv1alpha1.LastComponentConfiguration{
				ResourceRequirements: compResource,
			},
		}
		opsRes.OpsRequest.Status.Components[opsComp.ComponentName] = componentStatus
	}
	return nil
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (vs verticalScalingHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	componentNameSet := opsRes.OpsRequest.GetComponentNameSet()
	lastComponentInfo := map[string]appsv1alpha1.LastComponentConfiguration{}
	for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
		if _, ok := componentNameSet[v.Name]; !ok {
			continue
		}
		lastConfiguration := appsv1alpha1.LastComponentConfiguration{
			ResourceRequirements: v.Resources,
		}
		lastComponentInfo[v.Name] = lastConfiguration
	}
	opsRes.OpsRequest.Status.LastConfiguration.Components = lastComponentInfo
	return nil
}

// Cancel this function defines the cancel verticalScaling action.
func (vs verticalScalingHandler) Cancel(reqCxt intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	for _, v := range opsRes.OpsRequest.Status.Components {
		if v.OverrideBy != nil && v.OverrideBy.OpsName != "" {
			return intctrlutil.NewErrorf(intctrlutil.ErrorIgnoreCancel, `can not cancel the opsRequest due to another VerticalScaling opsRequest "%s" is running`, v.OverrideBy.OpsName)
		}
	}
	return cancelComponentOps(reqCxt.Ctx, cli, opsRes, func(lastConfig *appsv1alpha1.LastComponentConfiguration, comp *appsv1alpha1.ClusterComponentSpec) error {
		comp.Resources = lastConfig.ResourceRequirements
		return nil
	})
}
