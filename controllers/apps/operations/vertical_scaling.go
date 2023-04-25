/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type verticalScalingHandler struct{}

var _ OpsHandler = verticalScalingHandler{}

func init() {
	vsHandler := verticalScalingHandler{}
	verticalScalingBehaviour := OpsBehaviour{
		// if cluster is Abnormal or Failed, new opsRequest may can repair it.
		// TODO: we should add "force" flag for these opsRequest.
		FromClusterPhases:                  appsv1alpha1.GetClusterUpRunningPhases(),
		ToClusterPhase:                     appsv1alpha1.SpecReconcilingClusterPhase,
		OpsHandler:                         vsHandler,
		ProcessingReasonInClusterCondition: ProcessingReasonVerticalScaling,
		CancelFunc:                         vsHandler.Cancel,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.VerticalScalingType, verticalScalingBehaviour)
}

// ActionStartedCondition the started condition when handle the vertical scaling request.
func (vs verticalScalingHandler) ActionStartedCondition(opsRequest *appsv1alpha1.OpsRequest) *metav1.Condition {
	return appsv1alpha1.NewVerticalScalingCondition(opsRequest)
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
		// TODO: support specify class object name in the Class field
		if verticalScaling.ClassDefRef != nil {
			component.ClassDefRef = verticalScaling.ClassDefRef
		} else {
			// clear old class ref
			component.ClassDefRef = &appsv1alpha1.ClassDefRef{}
			component.Resources = verticalScaling.ResourceRequirements
		}
		opsRes.Cluster.Spec.ComponentSpecs[index] = component
	}
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for vertical scaling opsRequest.
func (vs verticalScalingHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	return reconcileActionWithComponentOps(reqCtx, cli, opsRes, "vertical scale", handleComponentStatusProgress)
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
		if v.ClassDefRef != nil {
			lastConfiguration.ClassDefRef = v.ClassDefRef
		}
		lastComponentInfo[v.Name] = lastConfiguration
	}
	opsRes.OpsRequest.Status.LastConfiguration.Components = lastComponentInfo
	return nil
}

// GetRealAffectedComponentMap gets the real affected component map for the operation
func (vs verticalScalingHandler) GetRealAffectedComponentMap(opsRequest *appsv1alpha1.OpsRequest) realAffectedComponentMap {
	realChangedMap := realAffectedComponentMap{}
	vsMap := opsRequest.Spec.ToVerticalScalingListToMap()
	for k, v := range opsRequest.Status.LastConfiguration.Components {
		currVs, ok := vsMap[k]
		if !ok {
			continue
		}
		if !reflect.DeepEqual(currVs.ResourceRequirements, v.ResourceRequirements) ||
			!reflect.DeepEqual(currVs.ClassDefRef, v.ClassDefRef) {
			realChangedMap[k] = struct{}{}
		}
	}
	return realChangedMap
}

// Cancel this function defines the cancel verticalScaling action.
func (vs verticalScalingHandler) Cancel(reqCxt intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return cancelComponentOps(reqCxt.Ctx, cli, opsRes, func(lastConfig *appsv1alpha1.LastComponentConfiguration, comp *appsv1alpha1.ClusterComponentSpec) error {
		comp.Resources = lastConfig.ResourceRequirements
		if lastConfig.ClassDefRef != nil {
			comp.ClassDefRef = lastConfig.ClassDefRef
		}
		return nil
	})
}
