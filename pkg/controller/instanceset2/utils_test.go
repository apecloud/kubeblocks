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

package instanceset2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

func TestComposeRolePriorityMap(t *testing.T) {
	t.Run("empty roles", func(t *testing.T) {
		m := composeRolePriorityMap(nil)
		assert.Equal(t, 0, m[""])
		assert.Len(t, m, 1)
	})

	t.Run("multiple roles", func(t *testing.T) {
		roles := []workloads.ReplicaRole{
			{Name: "Leader", UpdatePriority: 3},
			{Name: "Follower", UpdatePriority: 2},
			{Name: "Learner", UpdatePriority: 1},
		}
		m := composeRolePriorityMap(roles)
		assert.Equal(t, 0, m[""])
		assert.Equal(t, 3, m["leader"])
		assert.Equal(t, 2, m["follower"])
		assert.Equal(t, 1, m["learner"])
	})
}

func TestSortInstances(t *testing.T) {
	rolePriorityMap := map[string]int{
		"":         0,
		"learner":  1,
		"follower": 2,
		"leader":   3,
	}

	makeInst := func(name, role string) workloads.Instance {
		return workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Status:     workloads.InstanceStatus2{Role: role},
		}
	}

	t.Run("sort ascending", func(t *testing.T) {
		instances := []workloads.Instance{
			makeInst("inst-leader-0", "leader"),
			makeInst("inst-follower-0", "follower"),
			makeInst("inst-learner-0", "learner"),
			makeInst("inst-unknown-0", ""),
		}
		sortInstances(instances, rolePriorityMap, false)
		assert.Equal(t, "inst-unknown-0", instances[0].Name)
		assert.Equal(t, "inst-learner-0", instances[1].Name)
		assert.Equal(t, "inst-follower-0", instances[2].Name)
		assert.Equal(t, "inst-leader-0", instances[3].Name)
	})

	t.Run("sort descending", func(t *testing.T) {
		instances := []workloads.Instance{
			makeInst("inst-unknown-0", ""),
			makeInst("inst-leader-0", "leader"),
		}
		sortInstances(instances, rolePriorityMap, true)
		assert.Equal(t, "inst-leader-0", instances[0].Name)
		assert.Equal(t, "inst-unknown-0", instances[1].Name)
	})

	t.Run("empty list", func(t *testing.T) {
		var instances []workloads.Instance
		sortInstances(instances, rolePriorityMap, false)
		assert.Empty(t, instances)
	})
}

func TestComposeRoleMap(t *testing.T) {
	its := workloads.InstanceSet{
		Spec: workloads.InstanceSetSpec{
			Roles: []workloads.ReplicaRole{
				{Name: "Leader"},
				{Name: "Follower"},
			},
		},
	}
	m := composeRoleMap(its)
	assert.Len(t, m, 2)
	assert.Equal(t, "Leader", m["leader"].Name)
	assert.Equal(t, "Follower", m["follower"].Name)
}

func TestMergeMap(t *testing.T) {
	t.Run("empty src is noop", func(t *testing.T) {
		src := map[string]string{}
		dst := map[string]string{"k": "v"}
		mergeMap(&src, &dst)
		assert.Equal(t, map[string]string{"k": "v"}, dst)
	})

	t.Run("nil dst is initialized", func(t *testing.T) {
		src := map[string]string{"k": "v"}
		var dst map[string]string
		mergeMap(&src, &dst)
		assert.Equal(t, map[string]string{"k": "v"}, dst)
	})

	t.Run("src overwrites dst", func(t *testing.T) {
		src := map[string]string{"k": "new"}
		dst := map[string]string{"k": "old", "other": "keep"}
		mergeMap(&src, &dst)
		assert.Equal(t, "new", dst["k"])
		assert.Equal(t, "keep", dst["other"])
	})
}

func TestGetMatchLabels(t *testing.T) {
	labels := getMatchLabels("my-its")
	assert.Equal(t, "kubeblocks", labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "InstanceSet", labels[WorkloadsManagedByLabelKey])
	assert.Equal(t, "my-its", labels[WorkloadsInstanceLabelKey])
}

func TestCalculateConcurrencyReplicas(t *testing.T) {
	tests := []struct {
		name        string
		concurrency *intstr.IntOrString
		replicas    int
		want        int
		wantErr     bool
	}{
		{
			name:        "nil concurrency returns max(replicas,1)",
			concurrency: nil,
			replicas:    5,
			want:        5,
		},
		{
			name:        "nil concurrency with zero replicas returns 1",
			concurrency: nil,
			replicas:    0,
			want:        1,
		},
		{
			name:        "absolute value",
			concurrency: ptr.To(intstr.FromInt32(3)),
			replicas:    5,
			want:        3,
		},
		{
			name:        "absolute value exceeds replicas",
			concurrency: ptr.To(intstr.FromInt32(10)),
			replicas:    5,
			want:        5,
		},
		{
			name:        "percentage 50%",
			concurrency: ptr.To(intstr.FromString("50%")),
			replicas:    10,
			want:        5,
		},
		{
			name:        "percentage 100%",
			concurrency: ptr.To(intstr.FromString("100%")),
			replicas:    5,
			want:        5,
		},
		{
			name:        "percentage less than 100% with 2 replicas caps at replicas-1",
			concurrency: ptr.To(intstr.FromString("99%")),
			replicas:    2,
			want:        1,
		},
		{
			name:        "percentage rounds up ensures at least 1",
			concurrency: ptr.To(intstr.FromString("1%")),
			replicas:    3,
			want:        1,
		},
		{
			name:        "invalid percentage",
			concurrency: ptr.To(intstr.FromString("abc")),
			replicas:    5,
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calculateConcurrencyReplicas(tt.concurrency, tt.replicas)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetMemberUpdateStrategy(t *testing.T) {
	t.Run("nil strategy defaults to serial", func(t *testing.T) {
		its := &workloads.InstanceSet{}
		assert.Equal(t, workloads.SerialUpdateStrategy, getMemberUpdateStrategy(its))
	})

	t.Run("explicit parallel", func(t *testing.T) {
		parallel := workloads.ParallelUpdateStrategy
		its := &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				MemberUpdateStrategy: &parallel,
			},
		}
		assert.Equal(t, workloads.ParallelUpdateStrategy, getMemberUpdateStrategy(its))
	})

	t.Run("explicit best effort parallel", func(t *testing.T) {
		bestEffort := workloads.BestEffortParallelUpdateStrategy
		its := &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				MemberUpdateStrategy: &bestEffort,
			},
		}
		assert.Equal(t, workloads.BestEffortParallelUpdateStrategy, getMemberUpdateStrategy(its))
	})
}

func TestGetInstanceRoleName(t *testing.T) {
	inst := &workloads.Instance{
		Status: workloads.InstanceStatus2{Role: "leader"},
	}
	assert.Equal(t, "leader", getInstanceRoleName(inst))

	inst2 := &workloads.Instance{}
	assert.Equal(t, "", getInstanceRoleName(inst2))
}
