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

	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func BuildSchedulingPolicy(cluster *appsv1.Cluster, compSpec *appsv1.ClusterComponentSpec) (*appsv1.SchedulingPolicy, error) {
	if cluster.Spec.SchedulingPolicy != nil || (compSpec != nil && compSpec.SchedulingPolicy != nil) {
		return buildSchedulingPolicy(cluster, compSpec)
	}
	return nil, nil
}

func buildSchedulingPolicy(cluster *appsv1.Cluster, compSpec *appsv1.ClusterComponentSpec) (*appsv1.SchedulingPolicy, error) {
	schedulingPolicy := cluster.Spec.SchedulingPolicy.DeepCopy()
	if compSpec != nil && compSpec.SchedulingPolicy != nil {
		schedulingPolicy = compSpec.SchedulingPolicy.DeepCopy()
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
