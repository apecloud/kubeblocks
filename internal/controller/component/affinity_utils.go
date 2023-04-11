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
	// Add pod PodAffinityTerm for dedicated node
	if clusterOrCompAffinity.Tenancy == appsv1alpha1.DedicatedNode {
		var labelSelectorReqs []metav1.LabelSelectorRequirement
		labelSelectorReqs = append(labelSelectorReqs, metav1.LabelSelectorRequirement{
			Key:      intctrlutil.WorkloadTypeLabelKey,
			Operator: metav1.LabelSelectorOpIn,
			Values:   appsv1alpha1.WorkloadTypes,
		})
		podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
			podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution, corev1.PodAffinityTerm{
				TopologyKey: corev1.LabelHostname,
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: labelSelectorReqs,
				},
			})
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
