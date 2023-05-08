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

package component

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/constant"
)

func buildPodTopologySpreadConstraints(
	cluster *appsv1alpha1.Cluster,
	clusterOrCompAffinity *appsv1alpha1.Affinity,
	component *SynthesizedComponent,
) []corev1.TopologySpreadConstraint {
	if clusterOrCompAffinity == nil {
		return nil
	}

	var topologySpreadConstraints []corev1.TopologySpreadConstraint

	var whenUnsatisfiable corev1.UnsatisfiableConstraintAction
	if clusterOrCompAffinity.PodAntiAffinity == appsv1alpha1.Required {
		whenUnsatisfiable = corev1.DoNotSchedule
	} else {
		whenUnsatisfiable = corev1.ScheduleAnyway
	}
	for _, topologyKey := range clusterOrCompAffinity.TopologyKeys {
		topologySpreadConstraints = append(topologySpreadConstraints, corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			WhenUnsatisfiable: whenUnsatisfiable,
			TopologyKey:       topologyKey,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					intctrlutil.AppInstanceLabelKey:    cluster.Name,
					intctrlutil.KBAppComponentLabelKey: component.Name,
				},
			},
		})
	}
	return topologySpreadConstraints
}

func buildPodAffinity(
	cluster *appsv1alpha1.Cluster,
	clusterOrCompAffinity *appsv1alpha1.Affinity,
	component *SynthesizedComponent,
) *corev1.Affinity {
	if clusterOrCompAffinity == nil {
		return nil
	}
	affinity := new(corev1.Affinity)
	// Build NodeAffinity
	var matchExpressions []corev1.NodeSelectorRequirement
	for key, value := range clusterOrCompAffinity.NodeLabels {
		values := strings.Split(value, ",")
		matchExpressions = append(matchExpressions, corev1.NodeSelectorRequirement{
			Key:      key,
			Operator: corev1.NodeSelectorOpIn,
			Values:   values,
		})
	}
	if len(matchExpressions) > 0 {
		nodeSelectorTerm := corev1.NodeSelectorTerm{
			MatchExpressions: matchExpressions,
		}
		affinity.NodeAffinity = &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{nodeSelectorTerm},
			},
		}
	}
	// Build PodAntiAffinity
	var podAntiAffinity *corev1.PodAntiAffinity
	var podAffinityTerms []corev1.PodAffinityTerm
	for _, topologyKey := range clusterOrCompAffinity.TopologyKeys {
		podAffinityTerms = append(podAffinityTerms, corev1.PodAffinityTerm{
			TopologyKey: topologyKey,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					intctrlutil.AppInstanceLabelKey:    cluster.Name,
					intctrlutil.KBAppComponentLabelKey: component.Name,
				},
			},
		})
	}
	if clusterOrCompAffinity.PodAntiAffinity == appsv1alpha1.Required {
		podAntiAffinity = &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: podAffinityTerms,
		}
	} else {
		var weightedPodAffinityTerms []corev1.WeightedPodAffinityTerm
		for _, podAffinityTerm := range podAffinityTerms {
			weightedPodAffinityTerms = append(weightedPodAffinityTerms, corev1.WeightedPodAffinityTerm{
				Weight:          100,
				PodAffinityTerm: podAffinityTerm,
			})
		}
		podAntiAffinity = &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: weightedPodAffinityTerms,
		}
	}
	affinity.PodAntiAffinity = podAntiAffinity
	return affinity
}

// patchBuiltInAffinity patches built-in affinity configuration
func patchBuiltInAffinity(affinity *corev1.Affinity) *corev1.Affinity {
	var matchExpressions []corev1.NodeSelectorRequirement
	matchExpressions = append(matchExpressions, corev1.NodeSelectorRequirement{
		Key:      intctrlutil.KubeBlocksDataNodeLabelKey,
		Operator: corev1.NodeSelectorOpIn,
		Values:   []string{intctrlutil.KubeBlocksDataNodeLabelValue},
	})
	preferredSchedulingTerm := corev1.PreferredSchedulingTerm{
		Preference: corev1.NodeSelectorTerm{
			MatchExpressions: matchExpressions,
		},
		Weight: 100,
	}
	if affinity != nil && affinity.NodeAffinity != nil {
		affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
			affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution, preferredSchedulingTerm)
	} else {
		if affinity == nil {
			affinity = new(corev1.Affinity)
		}
		affinity.NodeAffinity = &corev1.NodeAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{preferredSchedulingTerm},
		}
	}

	return affinity
}

// PatchBuiltInToleration patches built-in tolerations configuration
func PatchBuiltInToleration(tolerations []corev1.Toleration) []corev1.Toleration {
	tolerations = append(tolerations, corev1.Toleration{
		Key:      intctrlutil.KubeBlocksDataNodeTolerationKey,
		Operator: corev1.TolerationOpEqual,
		Value:    intctrlutil.KubeBlocksDataNodeTolerationValue,
		Effect:   corev1.TaintEffectNoSchedule,
	})
	return tolerations
}
