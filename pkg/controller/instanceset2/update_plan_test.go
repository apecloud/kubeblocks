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

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func makeTestInstance(name string, itsGen int64, instGen int64, observedGen int64, upToDate bool, role string) *workloads.Instance {
	return &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				constant.KubeBlocksGenerationKey: "1",
			},
			Generation: instGen,
			Labels: map[string]string{
				constant.RoleLabelKey: role,
			},
		},
		Status: workloads.InstanceStatus2{
			ObservedGeneration: observedGen,
			UpToDate:           upToDate,
			Role:               role,
			Ready:              true,
		},
	}
}

func TestUpdatePlan_SerialExecute(t *testing.T) {
	its := workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Generation: 2},
	}

	// inst-0 is updated (gen matches, upToDate, ready)
	inst0 := makeTestInstance("inst-0", 2, 1, 1, true, "")
	inst0.Annotations[constant.KubeBlocksGenerationKey] = "2"

	// inst-1 is NOT updated (gen doesn't match)
	inst1 := makeTestInstance("inst-1", 2, 1, 1, true, "")
	inst1.Annotations[constant.KubeBlocksGenerationKey] = "1"

	plan := newUpdatePlan(its, []*workloads.Instance{inst0, inst1}, isInstanceUpdated)
	toUpdate, err := plan.Execute()
	require.NoError(t, err)

	// Serial: first non-updated instance should be returned
	require.Len(t, toUpdate, 1)
	assert.Equal(t, "inst-1", toUpdate[0].Name)
}

func TestUpdatePlan_ParallelExecute(t *testing.T) {
	parallel := workloads.ParallelUpdateStrategy
	its := workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Generation: 2},
		Spec: workloads.InstanceSetSpec{
			MemberUpdateStrategy: &parallel,
		},
	}

	inst0 := makeTestInstance("inst-0", 2, 1, 1, true, "")
	inst0.Annotations[constant.KubeBlocksGenerationKey] = "1"

	inst1 := makeTestInstance("inst-1", 2, 1, 1, true, "")
	inst1.Annotations[constant.KubeBlocksGenerationKey] = "1"

	plan := newUpdatePlan(its, []*workloads.Instance{inst0, inst1}, isInstanceUpdated)
	toUpdate, err := plan.Execute()
	require.NoError(t, err)

	// Parallel: first encounter triggers stop, so only 1 returned per walk step
	require.NotEmpty(t, toUpdate)
}

func TestUpdatePlan_AllUpdated(t *testing.T) {
	its := workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
	}

	inst0 := makeTestInstance("inst-0", 1, 1, 1, true, "")
	inst1 := makeTestInstance("inst-1", 1, 1, 1, true, "")

	plan := newUpdatePlan(its, []*workloads.Instance{inst0, inst1}, isInstanceUpdated)
	toUpdate, err := plan.Execute()
	require.NoError(t, err)
	assert.Empty(t, toUpdate)
}

func TestUpdatePlan_EmptyInstances(t *testing.T) {
	its := workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
	}
	plan := newUpdatePlan(its, nil, isInstanceUpdated)
	toUpdate, err := plan.Execute()
	require.NoError(t, err)
	assert.Empty(t, toUpdate)
}

func TestUpdatePlan_BestEffortParallel(t *testing.T) {
	bestEffort := workloads.BestEffortParallelUpdateStrategy
	its := workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Generation: 2},
		Spec: workloads.InstanceSetSpec{
			MemberUpdateStrategy: &bestEffort,
			Roles: []workloads.ReplicaRole{
				{Name: "leader", UpdatePriority: 3, ParticipatesInQuorum: true},
				{Name: "follower", UpdatePriority: 2, ParticipatesInQuorum: true},
				{Name: "learner", UpdatePriority: 1, ParticipatesInQuorum: false},
			},
		},
	}

	// All need update (gen=1, its.gen=2)
	learner := makeTestInstance("inst-learner-0", 2, 1, 1, true, "learner")
	learner.Annotations[constant.KubeBlocksGenerationKey] = "1"

	follower0 := makeTestInstance("inst-follower-0", 2, 1, 1, true, "follower")
	follower0.Annotations[constant.KubeBlocksGenerationKey] = "1"

	follower1 := makeTestInstance("inst-follower-1", 2, 1, 1, true, "follower")
	follower1.Annotations[constant.KubeBlocksGenerationKey] = "1"

	leader := makeTestInstance("inst-leader-0", 2, 1, 1, true, "leader")
	leader.Annotations[constant.KubeBlocksGenerationKey] = "1"

	plan := newUpdatePlan(its, []*workloads.Instance{learner, follower0, follower1, leader}, isInstanceUpdated)
	toUpdate, err := plan.Execute()
	require.NoError(t, err)
	// BestEffortParallel: learner (non-quorum) should be updated first
	require.NotEmpty(t, toUpdate)
	assert.Equal(t, "learner", toUpdate[0].Status.Role)
}

func TestPlanWalkFunc_TerminatingInstanceWaits(t *testing.T) {
	now := metav1.Now()
	its := workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
	}
	inst := makeTestInstance("inst-0", 1, 1, 1, true, "")
	inst.DeletionTimestamp = &now

	plan := newUpdatePlan(its, []*workloads.Instance{inst}, isInstanceUpdated)
	toUpdate, err := plan.Execute()
	require.NoError(t, err)
	assert.Empty(t, toUpdate)
}
