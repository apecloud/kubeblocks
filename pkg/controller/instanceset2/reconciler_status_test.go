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
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestSyncInstanceConfigStatus(t *testing.T) {
	instanceStatus := []workloads.InstanceStatus{
		{PodName: "test-its-0"},
		{PodName: "test-its-1"},
	}
	instances := []*workloads.Instance{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "test-its-0"},
			Status: workloads.InstanceStatus2{
				Configs: []workloads.InstanceConfigStatus{
					{Name: "log", ConfigHash: ptr.To("hash-0")},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "test-its-1"},
			Status: workloads.InstanceStatus2{
				Configs: []workloads.InstanceConfigStatus{
					{Name: "log", ConfigHash: ptr.To("hash-1")},
					{Name: "server", ConfigHash: ptr.To("hash-2")},
				},
			},
		},
	}

	syncInstanceConfigStatus(instanceStatus, instances)

	expected := []workloads.InstanceStatus{
		{
			PodName: "test-its-0",
			Configs: []workloads.InstanceConfigStatus{
				{Name: "log", ConfigHash: ptr.To("hash-0")},
			},
		},
		{
			PodName: "test-its-1",
			Configs: []workloads.InstanceConfigStatus{
				{Name: "log", ConfigHash: ptr.To("hash-1")},
				{Name: "server", ConfigHash: ptr.To("hash-2")},
			},
		},
	}
	if !reflect.DeepEqual(expected, instanceStatus) {
		t.Fatalf("unexpected instance status: %#v", instanceStatus)
	}
}

func TestSyncInstanceConfigStatusKeepsEmptyWhenInstanceHasNotReported(t *testing.T) {
	instanceStatus := []workloads.InstanceStatus{
		{PodName: "test-its-0"},
	}
	instances := []*workloads.Instance{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "test-its-0"},
		},
	}

	syncInstanceConfigStatus(instanceStatus, instances)

	if instanceStatus[0].Configs != nil {
		t.Fatalf("expected empty configs, got %#v", instanceStatus[0].Configs)
	}
}

func TestIsInstanceUpdated(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 3,
		},
	}

	tests := []struct {
		name string
		inst *workloads.Instance
		want bool
	}{
		{
			name: "false when parent generation has not propagated",
			inst: &workloads.Instance{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 2,
					Annotations: map[string]string{
						constant.KubeBlocksGenerationKey: "2",
					},
				},
				Status: workloads.InstanceStatus2{
					ObservedGeneration: 2,
					UpToDate:           true,
				},
			},
			want: false,
		},
		{
			name: "false when instance spec is latest but pod status is not up to date",
			inst: &workloads.Instance{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 3,
					Annotations: map[string]string{
						constant.KubeBlocksGenerationKey: "3",
					},
				},
				Status: workloads.InstanceStatus2{
					ObservedGeneration: 3,
					UpToDate:           false,
				},
			},
			want: false,
		},
		{
			name: "true only when latest parent generation has converged",
			inst: &workloads.Instance{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 3,
					Annotations: map[string]string{
						constant.KubeBlocksGenerationKey: "3",
					},
				},
				Status: workloads.InstanceStatus2{
					ObservedGeneration: 3,
					UpToDate:           true,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInstanceUpdated(its, tt.inst); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
