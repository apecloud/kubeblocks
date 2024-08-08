/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package scheduling

import (
	"encoding/json"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func BuildSchedulingPolicy(cluster *appsv1alpha1.Cluster, compSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.SchedulingPolicy, error) {
	if cluster.Spec.SchedulingPolicy != nil || (compSpec != nil && compSpec.SchedulingPolicy != nil) {
		return buildSchedulingPolicy(cluster, compSpec)
	}
	return buildSchedulingPolicy4Legacy(cluster, compSpec)
}

func BuildSchedulingPolicy4Component(clusterName, compName string, affinity *appsv1alpha1.Affinity,
	tolerations []corev1.Toleration) (*appsv1alpha1.SchedulingPolicy, error) {
	return buildSchedulingPolicy4LegacyComponent(clusterName, compName, affinity, tolerations)
}

func buildSchedulingPolicy(cluster *appsv1alpha1.Cluster, compSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.SchedulingPolicy, error) {
	schedulingPolicy := cluster.Spec.SchedulingPolicy
	if compSpec != nil && compSpec.SchedulingPolicy != nil {
		schedulingPolicy = compSpec.SchedulingPolicy
	}

	mergeGlobalAffinity := func() error {
		affinity, err := buildClusterWideAffinity()
		if err != nil {
			return err
		}
		schedulingPolicy.Affinity = mergeAffinity(schedulingPolicy.Affinity, affinity)
		return nil
	}

	mergeGlobalTolerations := func() error {
		tolerations, err := buildClusterWideTolerations()
		if err != nil {
			return err
		}
		if len(tolerations) > 0 {
			if len(schedulingPolicy.Tolerations) == 0 {
				schedulingPolicy.Tolerations = tolerations
			} else {
				schedulingPolicy.Tolerations = append(schedulingPolicy.Tolerations, tolerations...)
			}
		}
		return nil
	}

	if err := mergeGlobalAffinity(); err != nil {
		return nil, err
	}
	if err := mergeGlobalTolerations(); err != nil {
		return nil, err
	}
	return schedulingPolicy, nil
}

func buildSchedulingPolicy4Legacy(cluster *appsv1alpha1.Cluster, compSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.SchedulingPolicy, error) {
	affinity := buildAffinity4Legacy(cluster, compSpec)
	tolerations, err := buildTolerations4Legacy(cluster, compSpec)
	if err != nil {
		return nil, err
	}
	return buildSchedulingPolicy4LegacyComponent(cluster.Name, compSpec.Name, affinity, tolerations)
}

func buildSchedulingPolicy4LegacyComponent(clusterName, compName string, affinity *appsv1alpha1.Affinity,
	tolerations []corev1.Toleration) (*appsv1alpha1.SchedulingPolicy, error) {
	podAffinity, err := buildPodAffinity4Legacy(clusterName, compName, affinity)
	if err != nil {
		return nil, err
	}
	return &appsv1alpha1.SchedulingPolicy{
		Affinity:                  podAffinity,
		Tolerations:               tolerations,
		TopologySpreadConstraints: buildPodTopologySpreadConstraints4Legacy(clusterName, compName, affinity),
	}, nil
}

func buildAffinity4Legacy(cluster *appsv1alpha1.Cluster, compSpec *appsv1alpha1.ClusterComponentSpec) *appsv1alpha1.Affinity {
	var affinity *appsv1alpha1.Affinity
	if cluster.Spec.Affinity != nil {
		affinity = cluster.Spec.Affinity
	}
	if compSpec != nil && compSpec.Affinity != nil {
		affinity = compSpec.Affinity
	}
	return affinity
}

func buildTolerations4Legacy(cluster *appsv1alpha1.Cluster, compSpec *appsv1alpha1.ClusterComponentSpec) ([]corev1.Toleration, error) {
	tolerations := cluster.Spec.Tolerations
	if compSpec != nil && len(compSpec.Tolerations) != 0 {
		tolerations = compSpec.Tolerations
	}
	dpTolerations, err := buildClusterWideTolerations()
	if err != nil {
		return nil, err
	}
	return append(tolerations, dpTolerations...), nil
}

func buildPodTopologySpreadConstraints4Legacy(clusterName, compName string, compAffinity *appsv1alpha1.Affinity) []corev1.TopologySpreadConstraint {
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
					constant.KBAppComponentLabelKey: compName,
				},
			},
		})
	}
	return topologySpreadConstraints
}

func buildPodAffinity4Legacy(clusterName string, compName string, compAffinity *appsv1alpha1.Affinity) (*corev1.Affinity, error) {
	affinity := buildNewAffinity4Legacy(clusterName, compName, compAffinity)
	dpAffinity, err := buildClusterWideAffinity()
	if err != nil {
		return nil, err
	}
	return mergeAffinity(affinity, dpAffinity), nil
}

func buildNewAffinity4Legacy(clusterName, compName string, compAffinity *appsv1alpha1.Affinity) *corev1.Affinity {
	if compAffinity == nil {
		return nil
	}
	affinity := new(corev1.Affinity)
	// Build NodeAffinity
	var matchExpressions []corev1.NodeSelectorRequirement
	nodeLabelKeys := maps.Keys(compAffinity.NodeLabels)
	// NodeLabels must be ordered
	sort.Strings(nodeLabelKeys)
	for _, key := range nodeLabelKeys {
		values := strings.Split(compAffinity.NodeLabels[key], ",")
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
					constant.KBAppComponentLabelKey: compName,
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
	// if compAffinity.Tenancy == appsv1alpha1.DedicatedNode {
	//	// TODO(v1.0): workload type has been removed
	// }
	affinity.PodAntiAffinity = podAntiAffinity
	return affinity
}

// buildClusterWideAffinity builds data plane affinity from global config
func buildClusterWideAffinity() (*corev1.Affinity, error) {
	affinity := new(corev1.Affinity)
	if val := viper.GetString(constant.CfgKeyDataPlaneAffinity); val != "" {
		if err := json.Unmarshal([]byte(val), &affinity); err != nil {
			return nil, err
		}
	}
	return affinity, nil
}

// buildClusterWideTolerations builds data plane tolerations from global config
func buildClusterWideTolerations() ([]corev1.Toleration, error) {
	// build data plane tolerations from config
	var tolerations []corev1.Toleration
	if val := viper.GetString(constant.CfgKeyDataPlaneTolerations); val != "" {
		if err := json.Unmarshal([]byte(val), &tolerations); err != nil {
			return nil, err
		}
	}
	return tolerations, nil
}

// mergeAffinity merges affinity from src to dest
func mergeAffinity(dest, src *corev1.Affinity) *corev1.Affinity {
	if src == nil {
		return dest
	}

	if dest == nil {
		return src.DeepCopy()
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
	return rst
}
