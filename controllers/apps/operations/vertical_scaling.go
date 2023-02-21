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
)

type verticalScalingHandler struct{}

var _ OpsHandler = verticalScalingHandler{}

func init() {
	verticalScalingBehaviour := OpsBehaviour{
		FromClusterPhases: []appsv1alpha1.Phase{appsv1alpha1.RunningPhase, appsv1alpha1.FailedPhase, appsv1alpha1.AbnormalPhase},
		ToClusterPhase:    appsv1alpha1.VerticalScalingPhase,
		OpsHandler:        verticalScalingHandler{},
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
func (vs verticalScalingHandler) Action(opsRes *OpsResource) error {
	verticalScalingMap := opsRes.OpsRequest.CovertVerticalScalingListToMap()
	for index, component := range opsRes.Cluster.Spec.ComponentSpecs {
		if verticalScaling, ok := verticalScalingMap[component.Name]; ok {
			component.Resources = verticalScaling.ResourceRequirements
			opsRes.Cluster.Spec.ComponentSpecs[index] = component
		}
	}
	return opsRes.Client.Update(opsRes.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for vertical scaling opsRequest.
func (vs verticalScalingHandler) ReconcileAction(opsRes *OpsResource) (appsv1alpha1.Phase, time.Duration, error) {
	return ReconcileActionWithComponentOps(opsRes, "vertical scale", handleComponentStatusProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (vs verticalScalingHandler) SaveLastConfiguration(opsRes *OpsResource) error {
	componentNameMap := opsRes.OpsRequest.GetComponentNameMap()
	lastComponentInfo := map[string]appsv1alpha1.LastComponentConfiguration{}
	for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
		if _, ok := componentNameMap[v.Name]; !ok {
			continue
		}
		lastComponentInfo[v.Name] = appsv1alpha1.LastComponentConfiguration{
			ResourceRequirements: v.Resources,
		}
	}
	patch := client.MergeFrom(opsRes.OpsRequest.DeepCopy())
	opsRes.OpsRequest.Status.LastConfiguration = appsv1alpha1.LastConfiguration{
		Components: lastComponentInfo,
	}
	return opsRes.Client.Status().Patch(opsRes.Ctx, opsRes.OpsRequest, patch)
}

// GetRealAffectedComponentMap gets the real affected component map for the operation
func (vs verticalScalingHandler) GetRealAffectedComponentMap(opsRequest *appsv1alpha1.OpsRequest) realAffectedComponentMap {
	realChangedMap := realAffectedComponentMap{}
	vsMap := opsRequest.CovertVerticalScalingListToMap()
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
