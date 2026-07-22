/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package instanceset2

import (
	"reflect"
	"sort"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/revisionmap"
)

func TestBestEffortParallelUpdatePlanNormalizesMixedCaseRoles(t *testing.T) {
	roles := []workloads.ReplicaRole{
		{Name: "Follower", ParticipatesInQuorum: true, UpdatePriority: 1},
		{Name: "Leader", ParticipatesInQuorum: true, UpdatePriority: 2},
	}
	instances := []*workloads.Instance{
		newUpdatePlanTestInstance("mysql-0", "Follower", roles),
		newUpdatePlanTestInstance("mysql-1", "Follower", roles),
		newUpdatePlanTestInstance("mysql-2", "Follower", roles),
		newUpdatePlanTestInstance("mysql-3", "Follower", roles),
		newUpdatePlanTestInstance("mysql-4", "Leader", roles),
	}
	updateRevisions := make(map[string]string, len(instances))
	for _, inst := range instances {
		updateRevisions[inst.Name] = "new-revision"
	}
	encodedRevisions, err := revisionmap.Encode(updateRevisions)
	if err != nil {
		t.Fatalf("encode update revisions: %v", err)
	}
	its := workloads.InstanceSet{
		Spec: workloads.InstanceSetSpec{
			Roles:                roles,
			MemberUpdateStrategy: ptr.To(workloads.BestEffortParallelUpdateStrategy),
		},
		Status: workloads.InstanceSetStatus{UpdateRevisions: encodedRevisions},
	}

	expectedLayers := [][]string{
		{"mysql-2", "mysql-3"},
		{"mysql-0", "mysql-1"},
		{"mysql-4"},
	}
	for _, expected := range expectedLayers {
		updated, err := newUpdatePlan(its, instances).Execute()
		if err != nil {
			t.Fatalf("execute update plan: %v", err)
		}
		if got := sortedInstanceNames(updated); !reflect.DeepEqual(got, expected) {
			t.Fatalf("updated instances = %v, want %v", got, expected)
		}
		for _, updatedInst := range updated {
			for _, inst := range instances {
				if inst.Name == updatedInst.Name {
					inst.Annotations[instanceSetRevisionAnnotationKey] = "new-revision"
					break
				}
			}
		}
	}
}

func newUpdatePlanTestInstance(name, role string, roles []workloads.ReplicaRole) *workloads.Instance {
	return &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Generation:  1,
			Annotations: map[string]string{instanceSetRevisionAnnotationKey: "old-revision"},
		},
		Spec: workloads.InstanceSpec{Roles: roles},
		Status: workloads.InstanceStatus2{
			ObservedGeneration: 1,
			UpToDate:           true,
			Role:               role,
			Conditions: []metav1.Condition{
				{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
			},
		},
	}
}

func sortedInstanceNames(instances []*workloads.Instance) []string {
	names := make([]string, 0, len(instances))
	for _, inst := range instances {
		names = append(names, inst.Name)
	}
	sort.Strings(names)
	return names
}
