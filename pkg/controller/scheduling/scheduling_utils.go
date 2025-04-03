/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"k8s.io/apimachinery/pkg/api/equality"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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
		schedulingPolicy.Affinity = MergeAffinity(schedulingPolicy.Affinity, affinity)
		return nil
	}

	mergeGlobalTolerations := func() error {
		tolerations, err := buildClusterWideTolerations()
		if err != nil {
			return err
		}
		intctrlutil.MergeList(&schedulingPolicy.Tolerations, &tolerations, makeCmp[corev1.Toleration]())
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

func makeCmp[E any]() func(E) func(E) bool {
	return func(a E) func(E) bool {
		return func(b E) bool {
			return equality.Semantic.DeepEqual(a, b)
		}
	}
}

// MergeAffinity merges src to dst, return value is deepcopied
// Items in src will overwrite items in dst, if possible.
func MergeAffinity(src, dst *corev1.Affinity) *corev1.Affinity {
	if src == nil {
		return dst.DeepCopy()
	}

	if dst == nil {
		return src.DeepCopy()
	}

	rtn := dst.DeepCopy()

	// Merge PodAffinity
	if src.PodAffinity != nil {
		if rtn.PodAffinity == nil {
			rtn.PodAffinity = &corev1.PodAffinity{}
		}
		intctrlutil.MergeList(&src.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution, &rtn.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution, makeCmp[corev1.PodAffinityTerm]())
		intctrlutil.MergeList(&src.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution, &rtn.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution, makeCmp[corev1.WeightedPodAffinityTerm]())
	}

	// Merge PodAntiAffinity
	if src.PodAntiAffinity != nil {
		if rtn.PodAntiAffinity == nil {
			rtn.PodAntiAffinity = &corev1.PodAntiAffinity{}
		}
		intctrlutil.MergeList(&src.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution, &rtn.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution, makeCmp[corev1.PodAffinityTerm]())
		intctrlutil.MergeList(&src.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution, &rtn.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution, makeCmp[corev1.WeightedPodAffinityTerm]())
	}

	// Merge NodeAffinity
	if src.NodeAffinity != nil {
		if rtn.NodeAffinity == nil {
			rtn.NodeAffinity = &corev1.NodeAffinity{}
		}
		if src.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			if rtn.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
				rtn.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
			}
			// FIXME: NodeSelectorTerms are ORed, this can be a problem
			intctrlutil.MergeList(&src.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, &rtn.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, makeCmp[corev1.NodeSelectorTerm]())
		}
		intctrlutil.MergeList(&src.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution, &rtn.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution, makeCmp[corev1.PreferredSchedulingTerm]())
	}

	return rtn
}
