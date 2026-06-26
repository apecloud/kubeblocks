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

package instance

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func buildStatusTestInstance() *workloads.Instance {
	inst := builder.NewInstanceBuilder("default", "mysql-0").
		SetUID(types.UID("uid-status-test")).
		SetPodTemplate(corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "mysql",
					Image: "mysql:8.0",
				}},
			},
		}).
		SetSelectorMatchLabels(map[string]string{"app": "mysql"}).
		SetInstanceSetName("mysql").
		GetObject()
	inst.Generation = 1
	return inst
}

func buildReadyPod(inst *workloads.Instance) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      inst.Name,
			Namespace: inst.Namespace,
			Labels: map[string]string{
				constant.KBAppInstanceNameLabelKey: inst.Name,
				constant.KBAppPodNameLabelKey:      inst.Name,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "mysql",
				Image: "mysql:8.0",
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
			},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "mysql",
				Image: "mysql:8.0",
				Ready: true,
			}},
		},
	}
}

func TestNewStatusReconciler(t *testing.T) {
	r := NewStatusReconciler()
	if r == nil {
		t.Fatal("NewStatusReconciler() returned nil")
	}
	if _, ok := r.(*statusReconciler); !ok {
		t.Fatalf("expected *statusReconciler, got %T", r)
	}
}

func TestStatusReconcilerPreCondition(t *testing.T) {
	r := &statusReconciler{}

	// nil root
	tree := kubebuilderx.NewObjectTree()
	if result := r.PreCondition(tree); result.Satisfied {
		t.Fatal("expected unsatisfied for nil root")
	}

	// deleting root
	inst := buildStatusTestInstance()
	inst.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	tree = kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if result := r.PreCondition(tree); result.Satisfied {
		t.Fatal("expected unsatisfied for deleting root")
	}

	// updating root (Generation != ObservedGeneration)
	inst = buildStatusTestInstance()
	inst.Status.ObservedGeneration = 0
	tree = kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if result := r.PreCondition(tree); result.Satisfied {
		t.Fatal("expected unsatisfied for updating root")
	}

	// status updating (Generation == ObservedGeneration, not deleting)
	inst = buildStatusTestInstance()
	inst.Status.ObservedGeneration = 1
	tree = kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if result := r.PreCondition(tree); !result.Satisfied {
		t.Fatal("expected satisfied for status-updating root")
	}
}

func TestBuildReadyCondition(t *testing.T) {
	r := &statusReconciler{}
	inst := buildStatusTestInstance()

	cond := r.buildReadyCondition(inst, true, "")
	if cond.Status != metav1.ConditionTrue {
		t.Fatalf("expected ConditionTrue for ready, got %s", cond.Status)
	}
	if cond.Reason != workloads.ReasonReady {
		t.Fatalf("expected ReasonReady, got %s", cond.Reason)
	}

	cond = r.buildReadyCondition(inst, false, "pod-not-ready")
	if cond.Status != metav1.ConditionFalse {
		t.Fatalf("expected ConditionFalse for not ready, got %s", cond.Status)
	}
	if cond.Reason != workloads.ReasonNotReady {
		t.Fatalf("expected ReasonNotReady, got %s", cond.Reason)
	}
	if cond.Message != "pod-not-ready" {
		t.Fatalf("expected message 'pod-not-ready', got %s", cond.Message)
	}
}

func TestBuildAvailableCondition(t *testing.T) {
	r := &statusReconciler{}
	inst := buildStatusTestInstance()

	cond := r.buildAvailableCondition(inst, true, "")
	if cond.Status != metav1.ConditionTrue {
		t.Fatalf("expected ConditionTrue for available, got %s", cond.Status)
	}
	if cond.Reason != workloads.ReasonAvailable {
		t.Fatalf("expected ReasonAvailable, got %s", cond.Reason)
	}

	cond = r.buildAvailableCondition(inst, false, "pod-not-available")
	if cond.Status != metav1.ConditionFalse {
		t.Fatalf("expected ConditionFalse for not available, got %s", cond.Status)
	}
	if cond.Reason != workloads.ReasonNotAvailable {
		t.Fatalf("expected ReasonNotAvailable, got %s", cond.Reason)
	}
	if cond.Message != "pod-not-available" {
		t.Fatalf("expected message 'pod-not-available', got %s", cond.Message)
	}
}

func TestBuildFailureCondition(t *testing.T) {
	r := &statusReconciler{}
	inst := buildStatusTestInstance()

	// terminating pod -> nil
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &metav1.Time{Time: time.Now()}},
	}
	if cond := r.buildFailureCondition(inst, pod); cond != nil {
		t.Fatalf("expected nil for terminating pod, got %v", cond)
	}

	// healthy pod -> nil
	pod = buildReadyPod(inst)
	if cond := r.buildFailureCondition(inst, pod); cond != nil {
		t.Fatalf("expected nil for healthy pod, got %v", cond)
	}

	// PodFailed phase
	pod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "failed-pod"},
		Status:     corev1.PodStatus{Phase: corev1.PodFailed},
	}
	cond := r.buildFailureCondition(inst, pod)
	if cond == nil {
		t.Fatal("expected failure condition for PodFailed")
	}
	if cond.Status != metav1.ConditionTrue {
		t.Fatalf("expected ConditionTrue, got %s", cond.Status)
	}
	if cond.Message != "failed-pod" {
		t.Fatalf("expected message 'failed-pod', got %s", cond.Message)
	}
}

func TestObservedRoleOfPod(t *testing.T) {
	r := &statusReconciler{}

	// no roles defined
	inst := buildStatusTestInstance()
	pod := buildReadyPod(inst)
	if role := r.observedRoleOfPod(inst, pod); role != "" {
		t.Fatalf("expected empty role for no roles, got %s", role)
	}

	// with roles, pod ready, has role label
	inst = buildStatusTestInstance()
	inst.Spec.Roles = []workloads.ReplicaRole{{Name: "leader"}, {Name: "follower"}}
	pod = buildReadyPod(inst)
	pod.Labels[constant.RoleLabelKey] = "leader"
	if role := r.observedRoleOfPod(inst, pod); role != "leader" {
		t.Fatalf("expected role 'leader', got %s", role)
	}

	// with roles, pod not ready
	pod = buildReadyPod(inst)
	pod.Status.Conditions = nil
	pod.Labels[constant.RoleLabelKey] = "leader"
	if role := r.observedRoleOfPod(inst, pod); role != "" {
		t.Fatalf("expected empty role for not-ready pod, got %s", role)
	}
}

func TestHasRunningVolumeExpansion(t *testing.T) {
	r := &statusReconciler{}

	inst := buildStatusTestInstance()
	inst.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaimTemplate{{
		ObjectMeta: metav1.ObjectMeta{Name: "data"},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("2Gi"),
				},
			},
		},
	}}

	// no PVCs in tree -> false
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	if r.hasRunningVolumeExpansion(tree, inst) {
		t.Fatal("expected false with no PVCs")
	}

	// PVC with completed expansion (capacity >= request)
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "data-mysql-0",
			Labels: map[string]string{constant.KBAppPodNameLabelKey: "mysql-0"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("2Gi"),
			},
		},
	}
	tree = kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if err := tree.Add(pvc); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}
	if r.hasRunningVolumeExpansion(tree, inst) {
		t.Fatal("expected false with completed expansion")
	}

	// PVC with running expansion (capacity < request)
	pvc.Status.Capacity = corev1.ResourceList{
		corev1.ResourceStorage: resource.MustParse("1Gi"),
	}
	pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	tree = kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if err := tree.Add(pvc); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}
	if !r.hasRunningVolumeExpansion(tree, inst) {
		t.Fatal("expected true with running expansion")
	}
}

func TestObservedConfigsOfPod(t *testing.T) {
	r := &statusReconciler{}

	// no configs
	pod := &corev1.Pod{}
	configs, err := r.observedConfigsOfPod(pod)
	if err != nil {
		t.Fatalf("observedConfigsOfPod() error = %v", err)
	}
	if configs != nil {
		t.Fatalf("expected nil for no configs, got %#v", configs)
	}

	// with configs
	pod = buildReadyPod(buildStatusTestInstance())
	if err := configsToPod([]workloads.ConfigTemplate{
		{Name: "conf1", ConfigHash: ptr.To("hash1")},
	}, pod); err != nil {
		t.Fatalf("configsToPod() error = %v", err)
	}
	configs, err = r.observedConfigsOfPod(pod)
	if err != nil {
		t.Fatalf("observedConfigsOfPod() error = %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].Name != "conf1" || configs[0].ConfigHash == nil || *configs[0].ConfigHash != "hash1" {
		t.Fatalf("unexpected config: %#v", configs[0])
	}
}

func TestStatusReconcileNoPod(t *testing.T) {
	r := &statusReconciler{}
	inst := buildStatusTestInstance()
	inst.Status.ObservedGeneration = 1

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()

	result, err := r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.Next != "Continue" {
		t.Fatalf("expected Continue, got %s", result.Next)
	}
}

func TestStatusReconcileWithReadyPod(t *testing.T) {
	r := &statusReconciler{}
	inst := buildStatusTestInstance()
	inst.Status.ObservedGeneration = 1
	inst.Status.UpdateRevision = "test-rev"

	pod := buildReadyPod(inst)
	pod.Labels["controller.kubernetes.io/revisionhash"] = "test-rev"

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()
	if err := tree.Add(pod); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	_, err := r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if !inst.Status.Ready {
		t.Fatal("expected Ready to be true")
	}
	if !inst.Status.Available {
		t.Fatal("expected Available to be true")
	}
}

func TestStatusReconcileWithPendingPod(t *testing.T) {
	r := &statusReconciler{}
	inst := buildStatusTestInstance()
	inst.Status.ObservedGeneration = 1

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      inst.Name,
			Namespace: inst.Namespace,
			Labels: map[string]string{
				constant.KBAppInstanceNameLabelKey: inst.Name,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodPending},
	}

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()
	if err := tree.Add(pod); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	_, err := r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if inst.Status.Ready {
		t.Fatal("expected Ready to be false for pending pod")
	}
}

func TestStatusReconcileWithMinReadySeconds(t *testing.T) {
	r := &statusReconciler{}
	inst := buildStatusTestInstance()
	inst.Status.ObservedGeneration = 1
	inst.Status.UpdateRevision = "test-rev"
	inst.Spec.MinReadySeconds = 5

	pod := buildReadyPod(inst)
	pod.Labels["controller.kubernetes.io/revisionhash"] = "test-rev"
	pod.Status.Conditions = append(pod.Status.Conditions,
		corev1.PodCondition{Type: corev1.PodInitialized, Status: corev1.ConditionTrue},
	)

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	tree.Context = context.Background()
	tree.Logger = testLogger()
	if err := tree.Add(pod); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	result, err := r.Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RetryAfter == 0 {
		t.Fatal("expected RetryAfter for pod not yet available with MinReadySeconds")
	}
}

func testLogger() logr.Logger {
	return logr.Discard()
}

var _ = model.GetScheme
