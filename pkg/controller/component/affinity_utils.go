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
	"encoding/json"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// BuildAffinity builds affinities for components from cluster and comp spec.
func BuildAffinity(cluster *appsv1alpha1.Cluster, compSpec *appsv1alpha1.ClusterComponentSpec) *appsv1alpha1.Affinity {
	affinityTopoKey := func(policyType appsv1alpha1.AvailabilityPolicyType) string {
		switch policyType {
		case appsv1alpha1.AvailabilityPolicyZone:
			return "topology.kubernetes.io/zone"
		case appsv1alpha1.AvailabilityPolicyNode:
			return "kubernetes.io/hostname"
		}
		return ""
	}
	var affinity *appsv1alpha1.Affinity
	if len(cluster.Spec.Tenancy) > 0 || len(cluster.Spec.AvailabilityPolicy) > 0 {
		affinity = &appsv1alpha1.Affinity{
			PodAntiAffinity: appsv1alpha1.Preferred,
			TopologyKeys:    []string{affinityTopoKey(cluster.Spec.AvailabilityPolicy)},
			Tenancy:         cluster.Spec.Tenancy,
		}
	}
	if cluster.Spec.Affinity != nil {
		affinity = cluster.Spec.Affinity
	}
	if compSpec != nil && compSpec.Affinity != nil {
		affinity = compSpec.Affinity
	}
	return affinity
}

// BuildTolerations builds tolerations for components from cluster and comp spec.
func BuildTolerations(cluster *appsv1alpha1.Cluster, compSpec *appsv1alpha1.ClusterComponentSpec) ([]corev1.Toleration, error) {
	tolerations := cluster.Spec.Tolerations
	if compSpec != nil && len(compSpec.Tolerations) != 0 {
		tolerations = compSpec.Tolerations
	}
	// build data plane tolerations from config
	var dpTolerations []corev1.Toleration
	if val := viper.GetString(constant.CfgKeyDataPlaneTolerations); val != "" {
		if err := json.Unmarshal([]byte(val), &dpTolerations); err != nil {
			return nil, err
		}
	}
	return append(tolerations, dpTolerations...), nil
}

func BuildPodTopologySpreadConstraints(clusterName, compName string, compAffinity *appsv1alpha1.Affinity) []corev1.TopologySpreadConstraint {
	if compAffinity == nil {
		return nil
	}

	var topologySpreadConstraints []corev1.TopologySpreadConstraint

	var whenUnsatisfiable corev1.UnsatisfiableConstraintAction
	if compAffinity.PodAntiAffinity == appsv1alpha1.Required {
		whenUnsatisfiable = corev1.DoNotSchedule
	} else {
		whenUnsatisfiable = corev1.ScheduleAnyway
	}
	for _, topologyKey := range compAffinity.TopologyKeys {
		topologySpreadConstraints = append(topologySpreadConstraints, corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			WhenUnsatisfiable: whenUnsatisfiable,
			TopologyKey:       topologyKey,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: FullName(clusterName, compName),
				},
			},
		})
	}
	return topologySpreadConstraints
}

func BuildPodAffinity(clusterName string, compName string, compAffinity *appsv1alpha1.Affinity) (*corev1.Affinity, error) {
	affinity := buildNewAffinity(clusterName, compName, compAffinity)

	// read data plane affinity from config and merge it
	dpAffinity := new(corev1.Affinity)
	if val := viper.GetString(constant.CfgKeyDataPlaneAffinity); val != "" {
		if err := json.Unmarshal([]byte(val), &dpAffinity); err != nil {
			return nil, err
		}
	}
	return mergeAffinity(affinity, dpAffinity)
}

func buildNewAffinity(clusterName, compName string, compAffinity *appsv1alpha1.Affinity) *corev1.Affinity {
	if compAffinity == nil {
		return nil
	}
	affinity := new(corev1.Affinity)
	// Build NodeAffinity
	var matchExpressions []corev1.NodeSelectorRequirement
	for key, value := range compAffinity.NodeLabels {
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
	for _, topologyKey := range compAffinity.TopologyKeys {
		podAffinityTerms = append(podAffinityTerms, corev1.PodAffinityTerm{
			TopologyKey: topologyKey,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: FullName(clusterName, compName),
				},
			},
		})
	}
	if compAffinity.PodAntiAffinity == appsv1alpha1.Required {
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
	if compAffinity.Tenancy == appsv1alpha1.DedicatedNode {
		var labelSelectorReqs []metav1.LabelSelectorRequirement
		labelSelectorReqs = append(labelSelectorReqs, metav1.LabelSelectorRequirement{
			Key:      constant.WorkloadTypeLabelKey,
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

// mergeAffinity merges affinity from src to dest
func mergeAffinity(dest, src *corev1.Affinity) (*corev1.Affinity, error) {
	if src == nil {
		return dest, nil
	}

	if dest == nil {
		return src.DeepCopy(), nil
	}

	rst := dest.DeepCopy()
	skipPodAffinity := src.PodAffinity == nil
	skipPodAntiAffinity := src.PodAntiAffinity == nil
	skipNodeAffinity := src.NodeAffinity == nil

	if rst.PodAffinity == nil && !skipPodAffinity {
		rst.PodAffinity = src.PodAffinity
		skipPodAffinity = true
	}
	if rst.PodAntiAffinity == nil && !skipPodAntiAffinity {
		rst.PodAntiAffinity = src.PodAntiAffinity
		skipPodAntiAffinity = true
	}
	if rst.NodeAffinity == nil && !skipNodeAffinity {
		rst.NodeAffinity = src.NodeAffinity
		skipNodeAffinity = true
	}

	// if not skip, both are not nil
	if !skipPodAffinity {
		rst.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
			rst.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			src.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution...)

		rst.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
			rst.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
			src.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution...)
	}
	if !skipPodAntiAffinity {
		rst.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
			rst.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			src.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution...)

		rst.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
			rst.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
			src.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution...)
	}
	if !skipNodeAffinity {
		rst.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
			rst.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			src.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution...)

		skip := src.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil
		if rst.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil && !skip {
			rst.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = src.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
			skip = true
		}
		if !skip {
			rst.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(
				rst.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
				src.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms...)
		}
	}
	return rst, nil
}
