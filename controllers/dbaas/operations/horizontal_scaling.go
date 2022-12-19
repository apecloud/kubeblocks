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

import dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"

func init() {
	horizontalScalingBehaviour := &OpsBehaviour{
		FromClusterPhases:      []dbaasv1alpha1.Phase{dbaasv1alpha1.RunningPhase, dbaasv1alpha1.FailedPhase, dbaasv1alpha1.AbnormalPhase},
		ToClusterPhase:         dbaasv1alpha1.UpdatingPhase,
		Action:                 HorizontalScalingAction,
		ActionStartedCondition: dbaasv1alpha1.NewHorizontalScalingCondition,
		ReconcileAction:        ReconcileActionWithComponentOps,
		GetComponentNameMap:    getHorizontalScalingComponentNameMap,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(dbaasv1alpha1.HorizontalScalingType, horizontalScalingBehaviour)
}

// HorizontalScalingAction Modify Cluster.spec.components[*].replicas from the opsRequest
func HorizontalScalingAction(opsRes *OpsResource) error {
	var (
		horizontalScalingMap = covertHorizontalScalingListToMap(opsRes.OpsRequest)
		horizontalScaling    dbaasv1alpha1.HorizontalScaling
		ok                   bool
	)

	for index, component := range opsRes.Cluster.Spec.Components {
		if horizontalScaling, ok = horizontalScalingMap[component.Name]; !ok {
			continue
		}
		if horizontalScaling.Replicas != 0 {
			r := horizontalScaling.Replicas
			opsRes.Cluster.Spec.Components[index].Replicas = &r
		}
	}
	return opsRes.Client.Update(opsRes.Ctx, opsRes.Cluster)
}

// getHorizontalScalingComponentNameMap get the component name map with horizontal scaling operation.
func getHorizontalScalingComponentNameMap(opsRequest *dbaasv1alpha1.OpsRequest) map[string]struct{} {
	componentNameMap := make(map[string]struct{})
	for _, v := range opsRequest.Spec.HorizontalScalingList {
		componentNameMap[v.ComponentName] = struct{}{}
	}
	return componentNameMap
}

// covertHorizontalScalingListToMap covert OpsRequest.spec.horizontalScaling list to map
func covertHorizontalScalingListToMap(opsRequest *dbaasv1alpha1.OpsRequest) map[string]dbaasv1alpha1.HorizontalScaling {
	verticalScalingMap := make(map[string]dbaasv1alpha1.HorizontalScaling)
	for _, v := range opsRequest.Spec.HorizontalScalingList {
		verticalScalingMap[v.ComponentName] = v
	}
	return verticalScalingMap
}
