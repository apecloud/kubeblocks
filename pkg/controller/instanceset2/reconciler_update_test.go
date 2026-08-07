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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/rollingupdate"
)

func TestUpdateReconcilerCommitsStableWindowBeforeUpdatingInstances(t *testing.T) {
	roles := []workloads.ReplicaRole{
		{Name: "follower", UpdatePriority: 1},
		{Name: "leader", UpdatePriority: 2},
	}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-its",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To[int32](2),
			Roles:    roles,
			InstanceUpdateStrategy: &workloads.InstanceUpdateStrategy{
				RollingUpdate: &workloads.RollingUpdate{
					Replicas:       ptr.To(intstr.FromInt32(1)),
					MaxUnavailable: ptr.To(intstr.FromInt32(1)),
				},
			},
		},
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)

	desired, names, err := buildDesiredInstancesByName(tree, its)
	if err != nil {
		t.Fatalf("build desired instances: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected two desired instances, got %v", names)
	}
	for i, name := range names {
		inst := desired[name]
		inst.Annotations[instanceSetRevisionAnnotationKey] = "old-revision"
		inst.Generation = 1
		inst.Status = workloads.InstanceStatus2{
			ObservedGeneration: 1,
			UpToDate:           true,
			Role:               roles[i].Name,
			Conditions: []metav1.Condition{
				{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
				{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue},
			},
		}
		if err := tree.Add(inst); err != nil {
			t.Fatalf("add instance %s: %v", name, err)
		}
	}

	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatalf("update desired revisions: %v", err)
	}
	rolloutID := its.Annotations[rollingupdate.RolloutIDAnnotationKey]
	if rolloutID == "" {
		t.Fatal("expected desired revision reconciliation to persist a rollout ID")
	}
	reconciler := NewUpdateReconciler()
	result, err := reconciler.Reconcile(tree)
	if err != nil {
		t.Fatalf("initialize participant window: %v", err)
	}
	if result != kubebuilderx.Commit {
		t.Fatalf("expected participant window commit gate, got %#v", result)
	}
	for _, name := range names {
		object, err := tree.Get(&workloads.Instance{ObjectMeta: metav1.ObjectMeta{Namespace: its.Namespace, Name: name}})
		if err != nil {
			t.Fatalf("get instance %s: %v", name, err)
		}
		if revision := getInstanceRevision(object.(*workloads.Instance)); revision != "old-revision" {
			t.Fatalf("instance %s updated before window commit: %s", name, revision)
		}
	}

	// A control-only spec update changes Generation and the current role order,
	// but not the desired instance revisions. The persisted participant must win.
	its.Generation++
	its.Spec.InstanceUpdateStrategy.RollingUpdate.MaxUnavailable = ptr.To(intstr.FromInt32(2))
	first, _ := tree.Get(&workloads.Instance{ObjectMeta: metav1.ObjectMeta{Namespace: its.Namespace, Name: names[0]}})
	second, _ := tree.Get(&workloads.Instance{ObjectMeta: metav1.ObjectMeta{Namespace: its.Namespace, Name: names[1]}})
	first.(*workloads.Instance).Status.Role = "leader"
	second.(*workloads.Instance).Status.Role = "follower"
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatalf("refresh desired revisions after generation change: %v", err)
	}
	if got := its.Annotations[rollingupdate.RolloutIDAnnotationKey]; got != rolloutID {
		t.Fatalf("control-only generation change reset rollout ID: got %q, want %q", got, rolloutID)
	}

	result, err = reconciler.Reconcile(tree)
	if err != nil {
		t.Fatalf("update persisted participant: %v", err)
	}
	if result != kubebuilderx.Continue {
		t.Fatalf("expected existing window not to require another commit, got %#v", result)
	}
	first, _ = tree.Get(&workloads.Instance{ObjectMeta: metav1.ObjectMeta{Namespace: its.Namespace, Name: names[0]}})
	second, _ = tree.Get(&workloads.Instance{ObjectMeta: metav1.ObjectMeta{Namespace: its.Namespace, Name: names[1]}})
	if revision := getInstanceRevision(first.(*workloads.Instance)); revision == "old-revision" {
		t.Fatalf("expected persisted participant %s to be updated", names[0])
	}
	if revision := getInstanceRevision(second.(*workloads.Instance)); revision != "old-revision" {
		t.Fatalf("newly prioritized instance %s escaped the persisted window", names[1])
	}
}

func TestUpdateReconcilerResetsWindowWhenLeavingRollingUpdate(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-its",
			Namespace: "default",
			Annotations: map[string]string{
				rollingupdate.WindowAnnotationKey: `{"rolloutID":"old","replicas":1,"participants":["test-its-0"]}`,
			},
		},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To[int32](2),
			InstanceUpdateStrategy: &workloads.InstanceUpdateStrategy{
				Type: kbappsv1.OnDeleteStrategyType,
			},
		},
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	desired, names, err := buildDesiredInstancesByName(tree, its)
	if err != nil {
		t.Fatalf("build desired instances: %v", err)
	}
	for _, name := range names {
		if err := tree.Add(desired[name]); err != nil {
			t.Fatalf("add instance %s: %v", name, err)
		}
	}

	reconciler := NewUpdateReconciler()
	result, err := reconciler.Reconcile(tree)
	if err != nil {
		t.Fatalf("reset window for OnDelete: %v", err)
	}
	if result != kubebuilderx.Commit {
		t.Fatalf("expected window reset commit, got %#v", result)
	}
	if _, ok := its.Annotations[rollingupdate.WindowAnnotationKey]; !ok {
		t.Fatal("expected OnDelete to persist the ended window state")
	}

	result, err = reconciler.Reconcile(tree)
	if err != nil {
		t.Fatalf("reconcile OnDelete without window: %v", err)
	}
	if result != kubebuilderx.Continue {
		t.Fatalf("expected OnDelete without window to continue, got %#v", result)
	}

	its.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{
		RollingUpdate: &workloads.RollingUpdate{
			Replicas: ptr.To(intstr.FromInt32(1)),
		},
	}
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatalf("update desired revisions: %v", err)
	}
	result, err = reconciler.Reconcile(tree)
	if err != nil {
		t.Fatalf("start a fresh rolling-update window: %v", err)
	}
	if result != kubebuilderx.Commit {
		t.Fatalf("expected fresh window commit, got %#v", result)
	}
	if _, ok := its.Annotations[rollingupdate.WindowAnnotationKey]; !ok {
		t.Fatal("expected RollingUpdate to create a fresh window")
	}
}
