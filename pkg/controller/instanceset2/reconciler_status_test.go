/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
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

func TestStatusReconcilerPopulatesCurrentRevisionsFromInstances(t *testing.T) {
	replicas := int32(2)
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "default",
			Name:       "test-its",
			Generation: 3,
		},
		Spec: workloads.InstanceSetSpec{
			Replicas: &replicas,
		},
	}
	inst0 := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test-its-0",
			Annotations: map[string]string{
				constant.KubeBlocksGenerationKey: "1",
			},
		},
	}
	inst1 := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test-its-1",
			Annotations: map[string]string{
				constant.KubeBlocksGenerationKey: "3",
			},
		},
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	if err := tree.Add(inst0, inst1); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	result, err := NewStatusReconciler().Reconcile(tree)
	if err != nil {
		t.Fatalf("reconcile status: %v", err)
	}
	if result != kubebuilderx.Continue {
		t.Fatalf("unexpected result: %v", result)
	}
	expected := map[string]string{
		"test-its-0": "1",
		"test-its-1": "3",
	}
	if !reflect.DeepEqual(expected, its.Status.CurrentRevisions) {
		t.Fatalf("unexpected current revisions: %#v", its.Status.CurrentRevisions)
	}
}

func TestStatusReconcilerCountsInstanceAPIScaleOutReplicasUpdated(t *testing.T) {
	replicas := int32(3)
	its := newInstanceAPIStatusTestInstanceSet(replicas)
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	desiredInstances, err := buildDesiredInstances(tree, its)
	if err != nil {
		t.Fatalf("build desired instances: %v", err)
	}
	instances := []*workloads.Instance{
		readyInstanceForStatusTest(desiredInstances["test-its-0"], 1, "1"),
		readyInstanceForStatusTest(desiredInstances["test-its-1"], 1, "1"),
		readyInstanceForStatusTest(desiredInstances["test-its-2"], 2, "2"),
	}
	if err := tree.Add(instances[0], instances[1], instances[2]); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	result, err := NewStatusReconciler().Reconcile(tree)
	if err != nil {
		t.Fatalf("reconcile status: %v", err)
	}
	if result != kubebuilderx.Continue {
		t.Fatalf("unexpected result: %v", result)
	}
	if its.Status.UpdatedReplicas != replicas {
		t.Fatalf("expected updated replicas %d, got %d", replicas, its.Status.UpdatedReplicas)
	}
	if its.Status.CurrentReplicas != replicas {
		t.Fatalf("expected current replicas %d, got %d", replicas, its.Status.CurrentReplicas)
	}
	expected := map[string]string{
		"test-its-0": "1",
		"test-its-1": "1",
		"test-its-2": "2",
	}
	if !reflect.DeepEqual(expected, its.Status.CurrentRevisions) {
		t.Fatalf("unexpected current revisions: %#v", its.Status.CurrentRevisions)
	}
}

func TestRevisionUpdateReconcilerCountsInstanceAPIScaleOutReplicasUpdated(t *testing.T) {
	replicas := int32(3)
	its := newInstanceAPIStatusTestInstanceSet(replicas)
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	desiredInstances, err := buildDesiredInstances(tree, its)
	if err != nil {
		t.Fatalf("build desired instances: %v", err)
	}
	if err := tree.Add(
		readyInstanceForStatusTest(desiredInstances["test-its-0"], 1, "1"),
		readyInstanceForStatusTest(desiredInstances["test-its-1"], 1, "1"),
		readyInstanceForStatusTest(desiredInstances["test-its-2"], 2, "2"),
	); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	result, err := NewRevisionUpdateReconciler().Reconcile(tree)
	if err != nil {
		t.Fatalf("reconcile revision update: %v", err)
	}
	if result != kubebuilderx.Continue {
		t.Fatalf("unexpected result: %v", result)
	}
	if its.Status.UpdatedReplicas != replicas {
		t.Fatalf("expected updated replicas %d, got %d", replicas, its.Status.UpdatedReplicas)
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

func newInstanceAPIStatusTestInstanceSet(replicas int32) *workloads.InstanceSet {
	return &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "default",
			Name:       "test-its",
			Generation: 2,
		},
		Spec: workloads.InstanceSetSpec{
			Replicas:          &replicas,
			EnableInstanceAPI: ptr.To(true),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "main", Image: "nginx:latest"},
					},
				},
			},
		},
	}
}

func readyInstanceForStatusTest(inst *workloads.Instance, generation int64, parentGeneration string) *workloads.Instance {
	inst = inst.DeepCopy()
	inst.Generation = generation
	inst.Annotations[constant.KubeBlocksGenerationKey] = parentGeneration
	inst.Status = workloads.InstanceStatus2{
		ObservedGeneration: generation,
		UpToDate:           true,
		Conditions: []metav1.Condition{
			{
				Type:   string(workloads.InstanceReady),
				Status: metav1.ConditionTrue,
			},
			{
				Type:   string(workloads.InstanceAvailable),
				Status: metav1.ConditionTrue,
			},
		},
	}
	return inst
}
