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

package lifecycle

import (
	"strings"

	"golang.org/x/exp/maps"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

// mergeAnnotations keeps the original annotations.
// if annotations exist and are replaced, the Deployment/StatefulSet will be updated.
func mergeAnnotations(originalAnnotations map[string]string, targetAnnotations *map[string]string) {
	if targetAnnotations == nil {
		targetAnnotations = &map[string]string{}
	}
	for k, v := range originalAnnotations {
		// if the annotation not exist in targetAnnotations, copy it from original.
		if _, ok := (*targetAnnotations)[k]; !ok {
			(*targetAnnotations)[k] = v
		}
	}
}

// mergeServiceAnnotations keeps the original annotations except prometheus scrape annotations.
// if annotations exist and are replaced, the Service will be updated.
func mergeServiceAnnotations(originalAnnotations, targetAnnotations map[string]string) map[string]string {
	if len(originalAnnotations) == 0 {
		return targetAnnotations
	}
	tmpAnnotations := make(map[string]string, len(originalAnnotations)+len(targetAnnotations))
	for k, v := range originalAnnotations {
		if !strings.HasPrefix(k, "prometheus.io") {
			tmpAnnotations[k] = v
		}
	}
	maps.Copy(tmpAnnotations, targetAnnotations)
	return tmpAnnotations
}

// updateComponentPhaseWithOperation if workload of component changes, should update the component phase.
// REVIEW: this function need provide return value to determine mutation or not
// Deprecated:
func updateComponentPhaseWithOperation(cluster *appsv1alpha1.Cluster, componentName string) {
	if len(componentName) == 0 {
		return
	}
	componentPhase := appsv1alpha1.SpecReconcilingClusterCompPhase
	if cluster.Status.Phase == appsv1alpha1.StartingClusterPhase {
		componentPhase = appsv1alpha1.StartingClusterCompPhase
	}
	compStatus := cluster.Status.Components[componentName]
	// synchronous component phase is consistent with cluster phase
	compStatus.Phase = componentPhase
	cluster.Status.SetComponentStatus(componentName, compStatus)
}
