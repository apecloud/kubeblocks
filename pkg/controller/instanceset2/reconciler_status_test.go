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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/revisionmap"
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
	newInstance := func(generationAnnotation string, upToDate bool) *workloads.Instance {
		return &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-its-0",
				Generation: 2,
				Annotations: map[string]string{
					constant.KubeBlocksGenerationKey: generationAnnotation,
				},
			},
			Spec: workloads.InstanceSpec{
				MinReadySeconds: 1,
			},
			Status: workloads.InstanceStatus2{
				ObservedGeneration: 2,
				UpToDate:           upToDate,
			},
		}
	}
	latestInst := newInstance("2", true)
	updateRevisions, err := revisionmap.Encode(map[string]string{
		latestInst.Name: buildInstanceRevision(latestInst),
	})
	if err != nil {
		t.Fatalf("build revisions: %v", err)
	}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 3,
		},
		Status: workloads.InstanceSetStatus{
			UpdateRevisions: updateRevisions,
		},
	}

	tests := []struct {
		name string
		inst *workloads.Instance
		want bool
	}{
		{
			name: "true when instance revision matches even if parent generation changed",
			inst: newInstance("1", true),
			want: true,
		},
		{
			name: "false when instance spec is latest but pod status is not up to date",
			inst: newInstance("3", false),
			want: false,
		},
		{
			name: "false when instance intent revision differs",
			inst: func() *workloads.Instance {
				inst := newInstance("3", true)
				inst.Spec.MinReadySeconds = 2
				return inst
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInstanceUpdated(its, tt.inst, nil); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestBuildInstanceRevisionIgnoresParentGenerationAnnotation(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-its-0",
			Annotations: map[string]string{
				constant.KubeBlocksGenerationKey: "1",
			},
		},
		Spec: workloads.InstanceSpec{
			MinReadySeconds: 1,
		},
	}
	revision := buildInstanceRevision(inst)

	inst.Annotations[constant.KubeBlocksGenerationKey] = "2"
	if got := buildInstanceRevision(inst); got != revision {
		t.Fatalf("expected generation annotation to be ignored, got %s want %s", got, revision)
	}

	inst.Spec.MinReadySeconds = 2
	if got := buildInstanceRevision(inst); got == revision {
		t.Fatalf("expected spec change to alter revision")
	}
}

func TestBuildInstanceRevisionUsesDesiredMetadataKeys(t *testing.T) {
	desired := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-its-0",
			Labels: map[string]string{
				constant.KBAppInstanceTemplateLabelKey: "tpl",
				"managed-label":                        "desired",
			},
			Annotations: map[string]string{
				constant.KubeBlocksGenerationKey: "3",
				"managed-annotation":             "desired",
			},
		},
		Spec: workloads.InstanceSpec{
			MinReadySeconds: 1,
		},
	}
	revision := buildInstanceRevision(desired)

	actual := desired.DeepCopy()
	actual.Annotations[constant.KubeBlocksGenerationKey] = "1"
	actual.Annotations["external-annotation"] = "ignored"
	actual.Labels["external-label"] = "ignored"
	if got := buildCurrentInstanceRevision(actual, desired); got != revision {
		t.Fatalf("expected unmanaged metadata to be ignored, got %s want %s", got, revision)
	}

	actual = desired.DeepCopy()
	actual.Annotations["managed-annotation"] = "changed"
	if got := buildCurrentInstanceRevision(actual, desired); got == revision {
		t.Fatalf("expected managed annotation change to alter revision")
	}

	actual = desired.DeepCopy()
	actual.Labels["managed-label"] = "changed"
	if got := buildCurrentInstanceRevision(actual, desired); got == revision {
		t.Fatalf("expected managed label change to alter revision")
	}
}

func TestStatusReconcilerKeepsScaleOutInstancesUpdatedAfterParentGenerationBump(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-its",
			Namespace:  "default",
			Generation: 3,
		},
		Spec: workloads.InstanceSetSpec{
			Replicas:            ptr.To[int32](1),
			FlatInstanceOrdinal: true,
			Instances: []workloads.InstanceTemplate{
				{
					Name:     "tpl",
					Replicas: ptr.To[int32](1),
					Labels: map[string]string{
						"managed-label": "desired",
					},
					Annotations: map[string]string{
						"managed-annotation": "desired",
					},
				},
			},
		},
		Status: workloads.InstanceSetStatus{
			ObservedGeneration: 3,
		},
	}

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	desiredInstances, _, err := buildDesiredInstancesByName(tree, its)
	if err != nil {
		t.Fatalf("build desired instances: %v", err)
	}
	desired := desiredInstances["test-its-0"]
	if desired == nil {
		t.Fatalf("expected desired instance test-its-0, got %#v", desiredInstances)
	}
	desiredRevision := buildInstanceRevision(desired)
	updateRevisions, err := revisionmap.Encode(map[string]string{
		desired.Name: desiredRevision,
	})
	if err != nil {
		t.Fatalf("build revisions: %v", err)
	}
	its.Status.UpdateRevision = "update-revision"
	its.Status.UpdateRevisions = updateRevisions

	inst := desired.DeepCopy()
	inst.Annotations[constant.KubeBlocksGenerationKey] = "1"
	inst.Annotations["external-annotation"] = "ignored"
	inst.Labels["external-label"] = "ignored"
	inst.Generation = 2
	inst.Status = workloads.InstanceStatus2{
		ObservedGeneration: 2,
		UpToDate:           true,
		Conditions: []metav1.Condition{
			{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
			{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue},
		},
	}

	if err := tree.Add(inst); err != nil {
		t.Fatalf("add instance: %v", err)
	}
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatalf("reconcile status: %v", err)
	}

	got := tree.GetRoot().(*workloads.InstanceSet)
	currentRevisions, err := revisionmap.Decode(got.Status.CurrentRevisions)
	if err != nil {
		t.Fatalf("get current revisions: %v", err)
	}
	if currentRevisions[inst.Name] != buildCurrentInstanceRevision(inst, desired) {
		t.Fatalf("unexpected current revision: %#v", currentRevisions)
	}
	if currentRevisions[inst.Name] != desiredRevision {
		t.Fatalf("expected current revision to match desired update revision, got %s want %s", currentRevisions[inst.Name], desiredRevision)
	}
	if got.Status.UpdatedReplicas != 1 {
		t.Fatalf("expected updated replicas to stay at 1, got %d", got.Status.UpdatedReplicas)
	}
	if len(got.Status.TemplatesStatus) != 1 ||
		got.Status.TemplatesStatus[0].Name != "tpl" ||
		got.Status.TemplatesStatus[0].Replicas != 1 ||
		got.Status.TemplatesStatus[0].UpdatedReplicas != 1 ||
		got.Status.TemplatesStatus[0].CurrentReplicas != 1 {
		t.Fatalf("unexpected template status: %#v", got.Status.TemplatesStatus)
	}
	if got.Status.CurrentRevision != got.Status.UpdateRevision {
		t.Fatalf("expected current revision to advance to update revision")
	}
}
