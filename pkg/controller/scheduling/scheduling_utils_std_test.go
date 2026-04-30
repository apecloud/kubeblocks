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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func TestApplySchedulingPolicyToPodSpec_NilPolicy(t *testing.T) {
	podSpec := &corev1.PodSpec{
		NodeSelector: map[string]string{"existing": "value"},
	}
	ApplySchedulingPolicyToPodSpec(podSpec, nil)
	assert.Equal(t, "value", podSpec.NodeSelector["existing"])
}

func TestApplySchedulingPolicyToPodSpec_AllFields(t *testing.T) {
	podSpec := &corev1.PodSpec{}
	policy := &appsv1.SchedulingPolicy{
		SchedulerName: "custom-scheduler",
		NodeSelector:  map[string]string{"zone": "us-east-1"},
		NodeName:      "node-1",
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: "zone", Operator: corev1.NodeSelectorOpIn, Values: []string{"us-east-1"}},
						}},
					},
				},
			},
		},
		Tolerations: []corev1.Toleration{
			{Key: "key1", Operator: corev1.TolerationOpEqual, Value: "value1", Effect: corev1.TaintEffectNoSchedule},
		},
		TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
			{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}}},
		},
	}

	ApplySchedulingPolicyToPodSpec(podSpec, policy)

	assert.Equal(t, "custom-scheduler", podSpec.SchedulerName)
	assert.Equal(t, "us-east-1", podSpec.NodeSelector["zone"])
	assert.Equal(t, "node-1", podSpec.NodeName)
	assert.NotNil(t, podSpec.Affinity)
	assert.Len(t, podSpec.Tolerations, 1)
	assert.Len(t, podSpec.TopologySpreadConstraints, 1)
}

func TestBuildSchedulingPolicy_BothNil(t *testing.T) {
	cluster := &appsv1.Cluster{}
	result := BuildSchedulingPolicy(cluster, nil)
	assert.Nil(t, result)
}

func TestBuildSchedulingPolicy_CompSpecNil(t *testing.T) {
	cluster := &appsv1.Cluster{
		Spec: appsv1.ClusterSpec{
			SchedulingPolicy: &appsv1.SchedulingPolicy{
				NodeName: "cluster-node",
			},
		},
	}
	result := BuildSchedulingPolicy(cluster, nil)
	assert.NotNil(t, result)
	assert.Equal(t, "cluster-node", result.NodeName)
}

func TestBuildSchedulingPolicy_CompSpecWins(t *testing.T) {
	cluster := &appsv1.Cluster{
		Spec: appsv1.ClusterSpec{
			SchedulingPolicy: &appsv1.SchedulingPolicy{
				NodeName: "cluster-node",
			},
		},
	}
	compSpec := &appsv1.ClusterComponentSpec{
		SchedulingPolicy: &appsv1.SchedulingPolicy{
			NodeName: "comp-node",
		},
	}
	result := BuildSchedulingPolicy(cluster, compSpec)
	assert.NotNil(t, result)
	assert.Equal(t, "comp-node", result.NodeName)
}

func TestBuildSchedulingPolicy_CompSpecNilPolicyFallsToCluster(t *testing.T) {
	cluster := &appsv1.Cluster{
		Spec: appsv1.ClusterSpec{
			SchedulingPolicy: &appsv1.SchedulingPolicy{
				SchedulerName: "my-scheduler",
			},
		},
	}
	compSpec := &appsv1.ClusterComponentSpec{} // no scheduling policy
	result := BuildSchedulingPolicy(cluster, compSpec)
	assert.NotNil(t, result)
	assert.Equal(t, "my-scheduler", result.SchedulerName)
}

func TestBuildSchedulingPolicy_BothNilPolicies(t *testing.T) {
	cluster := &appsv1.Cluster{}
	compSpec := &appsv1.ClusterComponentSpec{}
	result := BuildSchedulingPolicy(cluster, compSpec)
	assert.Nil(t, result)
}
