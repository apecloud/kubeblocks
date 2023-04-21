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
		return
	}
	if *targetAnnotations == nil {
		*targetAnnotations = map[string]string{}
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
func updateComponentPhaseWithOperation(cluster *appsv1alpha1.Cluster, componentName string) {
	componentPhase := appsv1alpha1.SpecReconcilingClusterCompPhase
	if cluster.Status.Phase == appsv1alpha1.CreatingClusterPhase {
		componentPhase = appsv1alpha1.CreatingClusterCompPhase
	}
	compStatus := cluster.Status.Components[componentName]
	// synchronous component phase is consistent with cluster phase
	compStatus.Phase = componentPhase
	cluster.Status.SetComponentStatus(componentName, compStatus)
}
