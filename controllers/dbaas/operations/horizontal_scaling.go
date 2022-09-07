/*
Copyright 2022.

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
		FromClusterPhases:      []dbaasv1alpha1.Phase{dbaasv1alpha1.RunningPhase, dbaasv1alpha1.FailedPhase},
		ToClusterPhase:         dbaasv1alpha1.UpdatingPhase,
		Action:                 HorizontalScalingAction,
		ActionStartedCondition: dbaasv1alpha1.NewHorizontalScalingCondition,
		ReconcileAction:        ReconcileActionWithComponentOps,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(dbaasv1alpha1.HorizontalScalingType, horizontalScalingBehaviour)
}

// HorizontalScalingAction Modify Cluster.spec.components[*].replicas opsRes Cluster.spec.components[*].roleGroups[*].replicas from the opsRequest
func HorizontalScalingAction(opsRes *OpsResource) error {
	componentNameMap := getComponentsNameMap(opsRes.OpsRequest)
	horizontalScaling := opsRes.OpsRequest.Spec.ComponentOps.HorizontalScaling
	for index, component := range opsRes.Cluster.Spec.Components {
		if _, ok := componentNameMap[component.Name]; !ok {
			continue
		}
		if horizontalScaling.Replicas != 0 {
			opsRes.Cluster.Spec.Components[index].Replicas = horizontalScaling.Replicas
		}
		for _, v := range horizontalScaling.RoleGroups {
			for i, r := range component.RoleGroups {
				if r.Name == v.Name {
					opsRes.Cluster.Spec.Components[index].RoleGroups[i].Replicas = v.Replicas
				}
			}
		}
	}
	return opsRes.Client.Update(opsRes.Ctx, opsRes.Cluster)
}
