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
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(dbaasv1alpha1.HorizontalScalingType, horizontalScalingBehaviour)
}

// HorizontalScalingAction Modify Cluster.spec.components[*].replicas from the opsRequest
func HorizontalScalingAction(opsRes *OpsResource) error {
	var (
		componentNameMap = getAllComponentsNameMap(opsRes.OpsRequest)
		componentOps     *dbaasv1alpha1.ComponentOps
		ok               bool
	)

	for index, component := range opsRes.Cluster.Spec.Components {
		if componentOps, ok = componentNameMap[component.Name]; !ok || componentOps == nil {
			continue
		}
		if componentOps.HorizontalScaling.Replicas != 0 {
			opsRes.Cluster.Spec.Components[index].Replicas = componentOps.HorizontalScaling.Replicas
		}
	}
	return opsRes.Client.Update(opsRes.Ctx, opsRes.Cluster)
}
