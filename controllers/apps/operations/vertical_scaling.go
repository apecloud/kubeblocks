/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	verticalScalingBehaviour := OpsBehaviour{
		// if cluster is Abnormal or Failed, new opsRequest may can repair it.
		// TODO: we should add "force" flag for these opsRequest.
		FromClusterPhases:                  appsv1alpha1.GetClusterUpRunningPhases(),
		ToClusterPhase:                     appsv1alpha1.SpecReconcilingClusterPhase,
		OpsHandler:                         verticalScalingHandler{},
		ProcessingReasonInClusterCondition: ProcessingReasonVerticalScaling,
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
		if verticalScaling.Class != "" {
			component.ClassDefRef = &appsv1alpha1.ClassDefRef{Class: verticalScaling.Class}
		} else {
			component.Resources = verticalScaling.ResourceRequirements
		}
		opsRes.Cluster.Spec.ComponentSpecs[index] = component
	}
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for vertical scaling opsRequest.
func (vs verticalScalingHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	return ReconcileActionWithComponentOps(reqCtx, cli, opsRes, "vertical scale", handleComponentStatusProgress)
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
			lastConfiguration.Class = v.ClassDefRef.Class
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
		if !reflect.DeepEqual(currVs.ResourceRequirements, v.ResourceRequirements) {
			realChangedMap[k] = struct{}{}
		}
	}
	return realChangedMap
}
