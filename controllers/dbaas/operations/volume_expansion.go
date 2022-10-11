/*
Copyright 2022 The KubeBlocks Authors

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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

func init() {
	volumeExpansionBehaviour := &OpsBehaviour{
		FromClusterPhases:      []dbaasv1alpha1.Phase{dbaasv1alpha1.RunningPhase, dbaasv1alpha1.FailedPhase},
		ToClusterPhase:         dbaasv1alpha1.UpdatingPhase,
		Action:                 VolumeExpansionAction,
		ActionStartedCondition: dbaasv1alpha1.NewVolumeExpandingCondition,
		ReconcileAction:        ReconcileActionWithComponentOps,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(dbaasv1alpha1.VolumeExpansionType, volumeExpansionBehaviour)
}

// VolumeExpansionAction Modify Cluster.spec.components[*].VolumeClaimTemplates[*].spec.resources
func VolumeExpansionAction(opsRes *OpsResource) error {
	var (
		componentNameMap = getAllComponentsNameMap(opsRes.OpsRequest)
		componentOps     *dbaasv1alpha1.ComponentOps
		ok               bool
	)
	for index, component := range opsRes.Cluster.Spec.Components {
		if componentOps, ok = componentNameMap[component.Name]; !ok || componentOps == nil {
			continue
		}
		for _, v := range componentOps.VolumeExpansion {
			for i, vct := range component.VolumeClaimTemplates {
				if vct.Name != v.Name {
					continue
				}
				opsRes.Cluster.Spec.Components[index].VolumeClaimTemplates[i].
					Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(v.Storage)
			}
		}

	}
	return opsRes.Client.Update(opsRes.Ctx, opsRes.Cluster)
}
