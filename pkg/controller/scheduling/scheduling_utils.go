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
	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// ApplySchedulingPolicyToPodSpec overrides podSpec with schedulingPolicy, if schedulingPolicy is not nil
func ApplySchedulingPolicyToPodSpec(podSpec *corev1.PodSpec, schedulingPolicy *appsv1.SchedulingPolicy) {
	if schedulingPolicy != nil {
		podSpec.SchedulerName = schedulingPolicy.SchedulerName
		podSpec.NodeSelector = schedulingPolicy.NodeSelector
		podSpec.NodeName = schedulingPolicy.NodeName
		podSpec.Affinity = schedulingPolicy.Affinity
		podSpec.Tolerations = schedulingPolicy.Tolerations
		podSpec.TopologySpreadConstraints = schedulingPolicy.TopologySpreadConstraints
	}
}

func BuildSchedulingPolicy(cluster *appsv1.Cluster, compSpec *appsv1.ClusterComponentSpec) *appsv1.SchedulingPolicy {
	if cluster.Spec.SchedulingPolicy == nil && (compSpec == nil || compSpec.SchedulingPolicy == nil) {
		return nil
	}
	if compSpec != nil && compSpec.SchedulingPolicy != nil {
		return compSpec.SchedulingPolicy.DeepCopy()
	}
	return cluster.Spec.SchedulingPolicy.DeepCopy()
}
