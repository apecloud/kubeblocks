/*
Copyright 2022 The Kubeblocks Authors

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
	verticalScalingBehaviour := &OpsBehaviour{
		FromClusterPhases:      []dbaasv1alpha1.Phase{dbaasv1alpha1.RunningPhase, dbaasv1alpha1.FailedPhase},
		ToClusterPhase:         dbaasv1alpha1.UpdatingPhase,
		Action:                 VerticalScalingAction,
		ActionStartedCondition: dbaasv1alpha1.NewVerticalScalingCondition,
		ReconcileAction:        ReconcileActionWithComponentOps,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(dbaasv1alpha1.VerticalScalingType, verticalScalingBehaviour)
}

// VerticalScalingAction Modify cluster component resources according to
// the definition of opsRequest with spec.componentNames and spec.componentOps.verticalScaling
func VerticalScalingAction(opsRes *OpsResource) error {
	componentNameMap := getAllComponentsNameMap(opsRes.OpsRequest)
	for index, component := range opsRes.Cluster.Spec.Components {
		if componentOps, ok := componentNameMap[component.Name]; ok && componentOps != nil {
			component.Resources = *componentOps.VerticalScaling
			opsRes.Cluster.Spec.Components[index] = component
		}
	}
	return opsRes.Client.Update(opsRes.Ctx, opsRes.Cluster)
}
