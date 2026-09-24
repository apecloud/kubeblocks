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

package workloads

import (
	"context"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestInstanceSetReplicaRestoreMismatchCommitsFailedStatus(t *testing.T) {
	scheme := replicaRestoreTestScheme(t)
	group := "dataprotection.kubeblocks.io"
	intent := &appsv1.ReplicaRestoreProjection{
		StartOrdinal: 1,
		EndOrdinal:   2,
		Fingerprint:  "accepted",
		SourceRef: corev1.TypedObjectReference{
			APIGroup: &group,
			Kind:     "Backup",
			Name:     "backup",
		},
		Annotations: map[string]string{constant.RestoreSourceNameAnnotationKey: "backup"},
	}
	its := &workloadsv1.InstanceSet{
		TypeMeta: metav1.TypeMeta{APIVersion: workloadsv1.GroupVersion.String(), Kind: workloadsv1.InstanceSetKind},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "mysql",
			Namespace:  "default",
			UID:        types.UID("its-uid"),
			Generation: 1,
			Labels:     map[string]string{constant.KBAppComponentLabelKey: "mysql"},
		},
		Spec: workloadsv1.InstanceSetSpec{
			Replicas:       ptr.To[int32](2),
			ReplicaRestore: intent,
			Selector:       &metav1.LabelSelector{MatchLabels: map[string]string{"app": "mysql"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "mysql"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db"}}},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}}},
		},
		Status: workloadsv1.InstanceSetStatus{ObservedGeneration: 1},
	}
	labels := map[string]string{
		constant.AppManagedByLabelKey:        constant.AppName,
		"workloads.kubeblocks.io/managed-by": workloadsv1.InstanceSetKind,
		"workloads.kubeblocks.io/instance":   its.Name,
	}
	predecessor := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mysql-0",
			Namespace:   its.Namespace,
			Labels:      labels,
			Annotations: map[string]string{"keep": "pod"},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "existing"}}},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
			}},
		},
	}
	targetName := "mysql-1"
	pvcName := intctrlutil.ComposePVCName(its.Spec.VolumeClaimTemplates[0], its.Name, targetName)
	pvc := ordinaryInstanceSetPVC(its, pvcName, targetName)
	pvc.Annotations = map[string]string{"keep": "pvc"}

	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&workloadsv1.InstanceSet{}).
		WithObjects(its, predecessor, pvc).
		Build()
	r := &InstanceSetReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
	reconcileTwice(t, r, types.NamespacedName{Namespace: its.Namespace, Name: its.Name})

	got := &workloadsv1.InstanceSet{}
	getObject(t, cli, types.NamespacedName{Namespace: its.Namespace, Name: its.Name}, got)
	if got.Status.ReplicaRestore == nil || got.Status.ReplicaRestore.Phase != appsv1.ReplicaRestoreFailed ||
		!strings.Contains(got.Status.ReplicaRestore.Message, pvcName) {
		t.Fatalf("expected persisted Failed replica restore status for %s, got %#v", pvcName, got.Status.ReplicaRestore)
	}
	assertObjectUnchanged(t, cli, predecessor)
	assertObjectUnchanged(t, cli, pvc)
	assertObjectMissing(t, cli, &corev1.Pod{}, types.NamespacedName{Namespace: its.Namespace, Name: targetName})
}

func TestInstanceReplicaRestoreMismatchCommitsFailedCondition(t *testing.T) {
	scheme := replicaRestoreTestScheme(t)
	group := "dataprotection.kubeblocks.io"
	intent := &appsv1.ReplicaRestoreProjection{
		StartOrdinal: 0,
		EndOrdinal:   1,
		Fingerprint:  "accepted",
		SourceRef: corev1.TypedObjectReference{
			APIGroup: &group,
			Kind:     "Backup",
			Name:     "backup",
		},
		Annotations: map[string]string{constant.RestoreSourceNameAnnotationKey: "backup"},
	}
	inst := &workloadsv1.Instance{
		TypeMeta: metav1.TypeMeta{APIVersion: workloadsv1.GroupVersion.String(), Kind: "Instance"},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "mysql-0",
			Namespace:  "default",
			UID:        types.UID("instance-uid"),
			Generation: 1,
		},
		Spec: workloadsv1.InstanceSpec{
			InstanceSetName: "mysql",
			ReplicaRestore:  intent,
			Selector:        &metav1.LabelSelector{MatchLabels: map[string]string{"app": "mysql"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "mysql"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db"}}},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{ObjectMeta: metav1.ObjectMeta{Name: "data"}}},
		},
		Status: workloadsv1.InstanceStatus2{ObservedGeneration: 1},
	}
	pvcName := intctrlutil.ComposePVCName(
		corev1.PersistentVolumeClaim{ObjectMeta: inst.Spec.VolumeClaimTemplates[0].ObjectMeta},
		inst.Spec.InstanceSetName,
		inst.Name,
	)
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: inst.Namespace,
			Labels: map[string]string{
				constant.AppManagedByLabelKey:            constant.AppName,
				constant.KBAppInstanceNameLabelKey:       inst.Name,
				constant.KBAppPodNameLabelKey:            inst.Name,
				constant.VolumeClaimTemplateNameLabelKey: "data",
			},
			Annotations: map[string]string{"keep": "pvc"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: workloadsv1.GroupVersion.String(),
				Kind:       "Instance",
				Name:       inst.Name,
				UID:        inst.UID,
				Controller: ptr.To(true),
			}},
		},
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&workloadsv1.Instance{}).
		WithObjects(inst, pvc).
		Build()
	r := &InstanceReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
	reconcileTwice(t, r, types.NamespacedName{Namespace: inst.Namespace, Name: inst.Name})

	got := &workloadsv1.Instance{}
	getObject(t, cli, types.NamespacedName{Namespace: inst.Namespace, Name: inst.Name}, got)
	cond := meta.FindStatusCondition(got.Status.Conditions, string(workloadsv1.InstanceReplicaRestore))
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != workloadsv1.ReasonRestoreFailed ||
		!strings.Contains(cond.Message, pvcName) {
		t.Fatalf("expected persisted Failed replica restore condition for %s, got %#v", pvcName, cond)
	}
	assertObjectUnchanged(t, cli, pvc)
	assertObjectMissing(t, cli, &corev1.Pod{}, types.NamespacedName{Namespace: inst.Namespace, Name: inst.Name})
}

type replicaRestoreReconciler interface {
	Reconcile(context.Context, ctrl.Request) (ctrl.Result, error)
}

func reconcileTwice(t *testing.T, reconciler replicaRestoreReconciler, key types.NamespacedName) {
	t.Helper()
	req := ctrl.Request{NamespacedName: key}
	if _, err := reconciler.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("expected PVC mismatch to commit status without returning an error, got %v", err)
	}
	if !result.Requeue || result.RequeueAfter <= 0 {
		t.Fatalf("expected PVC mismatch to retry after committing status, got %#v", result)
	}
}

func replicaRestoreTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := workloadsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := rbacv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	model.AddScheme(corev1.AddToScheme)
	model.AddScheme(workloadsv1.AddToScheme)
	return scheme
}

func ordinaryInstanceSetPVC(its *workloadsv1.InstanceSet, name, instanceName string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: its.Namespace,
			Labels: map[string]string{
				constant.AppManagedByLabelKey:            constant.AppName,
				"workloads.kubeblocks.io/managed-by":     workloadsv1.InstanceSetKind,
				"workloads.kubeblocks.io/instance":       its.Name,
				constant.KBAppPodNameLabelKey:            instanceName,
				constant.VolumeClaimTemplateNameLabelKey: "data",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: workloadsv1.GroupVersion.String(),
				Kind:       workloadsv1.InstanceSetKind,
				Name:       its.Name,
				UID:        its.UID,
				Controller: ptr.To(true),
			}},
		},
	}
}

func assertObjectUnchanged(t *testing.T, cli client.Client, want client.Object) {
	t.Helper()
	got := want.DeepCopyObject().(client.Object)
	getObject(t, cli, client.ObjectKeyFromObject(want), got)
	if !reflect.DeepEqual(got.GetLabels(), want.GetLabels()) || !reflect.DeepEqual(got.GetAnnotations(), want.GetAnnotations()) {
		t.Fatalf("object %T %s metadata changed: got labels/annotations %#v/%#v, want %#v/%#v",
			want, want.GetName(), got.GetLabels(), got.GetAnnotations(), want.GetLabels(), want.GetAnnotations())
	}
	switch want := want.(type) {
	case *corev1.Pod:
		if got := got.(*corev1.Pod); !reflect.DeepEqual(got.Spec, want.Spec) {
			t.Fatalf("Pod %s spec changed: got %#v, want %#v", want.Name, got.Spec, want.Spec)
		}
	case *corev1.PersistentVolumeClaim:
		if got := got.(*corev1.PersistentVolumeClaim); !reflect.DeepEqual(got.Spec, want.Spec) {
			t.Fatalf("PVC %s spec changed: got %#v, want %#v", want.Name, got.Spec, want.Spec)
		}
	}
}

func assertObjectMissing(t *testing.T, cli client.Client, obj client.Object, key types.NamespacedName) {
	t.Helper()
	if err := cli.Get(context.Background(), key, obj); !errors.IsNotFound(err) {
		t.Fatalf("expected %T %s to be absent, got %v", obj, key, err)
	}
}

func getObject(t *testing.T, cli client.Client, key types.NamespacedName, obj client.Object) {
	t.Helper()
	if err := cli.Get(context.Background(), key, obj); err != nil {
		t.Fatal(err)
	}
}
