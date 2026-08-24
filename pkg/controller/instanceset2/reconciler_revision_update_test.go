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
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/revisionmap"
)

func TestStatusPreconditionWaitsForRevisionUpdate(t *testing.T) {
	its := revisionTestInstanceSet()
	its.Status.ObservedGeneration = its.Generation - 1
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	if result := NewStatusReconciler().PreCondition(tree); result != kubebuilderx.ConditionUnsatisfied {
		t.Fatalf("status reconciler ran before revisions were published: %#v", result)
	}
	if result := NewRevisionUpdateReconciler().PreCondition(tree); result != kubebuilderx.ConditionSatisfied {
		t.Fatalf("revision reconciler did not accept the pending generation: %#v", result)
	}
	its.Status.ObservedGeneration = its.Generation
	if result := NewStatusReconciler().PreCondition(tree); result != kubebuilderx.ConditionSatisfied {
		t.Fatalf("status reconciler did not accept the observed generation: %#v", result)
	}
}

func TestRevisionUpdatePublishesRevisionsBeforeInstanceStatus(t *testing.T) {
	its := revisionTestInstanceSet()
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	if its.Status.ObservedGeneration != its.Generation {
		t.Fatalf("ObservedGeneration was not advanced: %#v", its.Status)
	}
	if len(its.Status.InstanceStatus) != 0 {
		t.Fatalf("revision reconcile unexpectedly published InstanceStatus: %#v", its.Status.InstanceStatus)
	}
}

func TestTransientFlatOrdinalReassignmentPreservesViewAndAllowsAlignment(t *testing.T) {
	its := transientFlatReassignmentInstanceSet()
	previous := its.DeepCopy().Status.InstanceStatus
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	for ordinal, templateName := range []string{"a", "b"} {
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("demo-%d", ordinal), Namespace: its.Namespace, Generation: 1},
			Spec:       workloads.InstanceSpec{InstanceTemplateName: templateName},
			Status: workloads.InstanceStatus2{
				ObservedGeneration: 1,
				CurrentState:       workloads.InstanceCurrentStatePresent,
				UpToDate:           true,
				Conditions: []metav1.Condition{
					{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
					{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue},
				},
			},
		}
		if err := tree.Add(inst); err != nil {
			t.Fatal(err)
		}
	}

	res, err := NewStatusReconciler().Reconcile(tree)
	if err != nil || res != kubebuilderx.Continue {
		t.Fatalf("status reconcile blocked transient allocation: result=%v err=%v", res, err)
	}
	if !equality.Semantic.DeepEqual(its.Status.InstanceStatus, previous) {
		t.Fatalf("status reconcile published a partial view: %#v", its.Status)
	}

	res, err = NewRevisionUpdateReconciler().Reconcile(tree)
	if err != nil || res != kubebuilderx.Continue {
		t.Fatalf("revision reconcile blocked transient allocation: result=%v err=%v", res, err)
	}
	if its.Status.ObservedGeneration != its.Generation || !equality.Semantic.DeepEqual(its.Status.InstanceStatus, previous) {
		t.Fatalf("revision reconcile advanced an incomplete view: %#v", its.Status)
	}

	res, err = NewAlignmentReconciler().Reconcile(tree)
	if err != nil || res != kubebuilderx.Continue {
		t.Fatalf("alignment did not continue: result=%v err=%v", res, err)
	}
	scaledDown := 0
	for _, object := range tree.List(&workloads.Instance{}) {
		if ptr.Deref(object.(*workloads.Instance).Spec.ScaledDown, false) {
			scaledDown++
		}
	}
	if scaledDown != 1 {
		t.Fatalf("expected alignment to release one temporary assignment, got %d", scaledDown)
	}
}

func revisionTestInstanceSet() *workloads.InstanceSet {
	return &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 2},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
		},
	}
}

func transientFlatReassignmentInstanceSet() *workloads.InstanceSet {
	replicas, templateReplicas := int32(2), int32(1)
	templateA, templateB := "a", "b"
	return &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 2},
		Spec: workloads.InstanceSetSpec{
			Replicas:            &replicas,
			FlatInstanceOrdinal: true,
			Template:            corev1.PodTemplateSpec{},
			Instances: []workloads.InstanceTemplate{
				{Name: templateA, Replicas: &templateReplicas, Ordinals: workloads.Ordinals{Discrete: []int32{1}}},
				{Name: templateB, Replicas: &templateReplicas, Ordinals: workloads.Ordinals{Discrete: []int32{0}}},
			},
		},
		Status: workloads.InstanceSetStatus{
			ObservedGeneration: 1,
			AssignedOrdinals: map[string]workloads.Ordinals{
				templateA: {Discrete: []int32{0}},
				templateB: {Discrete: []int32{1}},
			},
			InstanceStatus: []workloads.InstanceStatus{
				{PodName: "demo-0", TemplateName: &templateA, DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent},
				{PodName: "demo-1", TemplateName: &templateB, DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent},
			},
		},
	}
}

func TestRevisionUpdateInvalidatesOnlyAffectedITS2Instances(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*workloads.InstanceSet)
	}{
		{
			name: "pod revision changes for one template",
			mutate: func(its *workloads.InstanceSet) {
				its.Spec.Instances[0].Env = []corev1.EnvVar{{Name: "REVISION_CHANGE", Value: "true"}}
			},
		},
		{
			name: "in-place resources change for one template",
			mutate: func(its *workloads.InstanceSet) {
				its.Spec.Instances[0].Resources = &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			its, tree, _ := newITS2InstanceStatusFixture(t, nil)
			oldRevisions, err := revisionmap.Decode(its.Status.UpdateRevisions)
			if err != nil {
				t.Fatal(err)
			}
			its.Generation++
			tt.mutate(its)

			if NewStatusReconciler().PreCondition(tree) != kubebuilderx.ConditionUnsatisfied {
				t.Fatal("status reconciler must remain before revision update and wait for the new generation target")
			}
			if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			newRevisions, err := revisionmap.Decode(its.Status.UpdateRevisions)
			if err != nil {
				t.Fatal(err)
			}
			if newRevisions["demo-0"] == oldRevisions["demo-0"] || newRevisions["demo-1"] != oldRevisions["demo-1"] {
				t.Fatalf("Instance spec revision change was not scoped to demo-0: old=%v new=%v", oldRevisions, newRevisions)
			}
			assertITS2UpToDate(t, its, "demo-0", false)
			assertITS2UpToDate(t, its, "demo-1", true)

			// The second pass uses the new Instance-spec revisions and keeps only the affected
			// current Instance stale until its own controller applies the desired spec.
			if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			assertITS2UpToDate(t, its, "demo-0", false)
			assertITS2UpToDate(t, its, "demo-1", true)
		})
	}
}
