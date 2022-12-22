/*
Copyright ApeCloud Inc.

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

	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

type verticalScalingHandler struct{}

func init() {
	vs := verticalScalingHandler{}
	verticalScalingBehaviour := OpsBehaviour{
		FromClusterPhases: []dbaasv1alpha1.Phase{dbaasv1alpha1.RunningPhase, dbaasv1alpha1.FailedPhase, dbaasv1alpha1.AbnormalPhase},
		ToClusterPhase:    dbaasv1alpha1.UpdatingPhase,
		OpsHandler:        vs,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(dbaasv1alpha1.VerticalScalingType, verticalScalingBehaviour)
}

// ActionStartedCondition the started condition when handle the vertical scaling request.
func (vs verticalScalingHandler) ActionStartedCondition(opsRequest *dbaasv1alpha1.OpsRequest) *metav1.Condition {
	return dbaasv1alpha1.NewHorizontalScalingCondition(opsRequest)
}

// Action Modify cluster component resources according to
// the definition of opsRequest with spec.componentNames and spec.componentOps.verticalScaling
func (vs verticalScalingHandler) Action(opsRes *OpsResource) error {
	verticalScalingMap := opsRes.OpsRequest.CovertVerticalScalingListToMap()
	for index, component := range opsRes.Cluster.Spec.Components {
		if verticalScaling, ok := verticalScalingMap[component.Name]; ok {
			component.Resources = verticalScaling.ResourceRequirements
			opsRes.Cluster.Spec.Components[index] = component
		}
	}
	return opsRes.Client.Update(opsRes.Ctx, opsRes.Cluster)
}

// ReconcileAction it will be performed when action is done and loop util OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for vertical scaling opsRequest.
func (vs verticalScalingHandler) ReconcileAction(opsRes *OpsResource) (dbaasv1alpha1.Phase, time.Duration, error) {
	return ReconcileActionWithComponentOps(opsRes, "vertical scale", handleComponentStatusProgress)
}

// SaveLastConfiguration record last configuration to the OpsRequest.status.lastConfiguration
func (vs verticalScalingHandler) SaveLastConfiguration(opsRes *OpsResource) error {
	componentNameMap := opsRes.OpsRequest.GetComponentNameMap()
	lastComponentInfo := map[string]dbaasv1alpha1.LastComponentConfiguration{}
	for _, v := range opsRes.Cluster.Spec.Components {
		if _, ok := componentNameMap[v.Name]; !ok {
			continue
		}
		lastComponentInfo[v.Name] = dbaasv1alpha1.LastComponentConfiguration{
			ResourceRequirements: v.Resources,
		}
	}
	patch := client.MergeFrom(opsRes.OpsRequest.DeepCopy())
	opsRes.OpsRequest.Status.LastConfiguration = dbaasv1alpha1.LastConfiguration{
		Components: lastComponentInfo,
	}
	return opsRes.Client.Status().Patch(opsRes.Ctx, opsRes.OpsRequest, patch)
}

// GetRealAffectedComponentMap get the real affected component map for the operation
func (vs verticalScalingHandler) GetRealAffectedComponentMap(opsRequest *dbaasv1alpha1.OpsRequest) realAffectedComponentMap {
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
