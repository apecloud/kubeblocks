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

package operations

import (
	"context"
	"reflect"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func ownerTestObjects() (*appsv1.Cluster, *opsv1alpha1.OpsRequest) {
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "original", Generation: 1},
		Spec:       appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "db", ComponentDef: "database", Replicas: 1}}},
		Status:     appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase, Components: map[string]appsv1.ClusterComponentStatus{"db": {Phase: appsv1.RunningComponentPhase}}},
	}
	ops := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "stop-demo", Namespace: "default", UID: "request", Finalizers: []string{constant.OpsRequestFinalizerName},
			Labels:          map[string]string{constant.AppInstanceLabelKey: cluster.Name, constant.OpsRequestTypeLabelKey: string(opsv1alpha1.StopType)},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: appsv1.GroupVersion.String(), Kind: appsv1.ClusterKind, Name: cluster.Name, UID: cluster.UID}}},
		Spec: opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.StopType},
	}
	return cluster, ops
}

func ownerTestReconciler(t *testing.T, objects ...client.Object) *OpsRequestReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{batchv1.AddToScheme, corev1.AddToScheme, appsv1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	return &OpsRequestReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&appsv1.Cluster{}, &opsv1alpha1.OpsRequest{}).WithObjects(objects...).Build(),
		Scheme: scheme, Recorder: record.NewFakeRecorder(100)}
}

func reconcileOwnerTest(t *testing.T, r *OpsRequestReconciler, ops *opsv1alpha1.OpsRequest) {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(ops)}); err != nil {
		t.Fatal(err)
	}
}

func TestOpsRequestEstablishesOwnerWithExistingLabels(t *testing.T) {
	cluster, ops := ownerTestObjects()
	ops.OwnerReferences = nil
	r := ownerTestReconciler(t, cluster, ops)
	reconcileOwnerTest(t, r, ops)
	actual := &opsv1alpha1.OpsRequest{}
	if err := r.Get(context.Background(), client.ObjectKeyFromObject(ops), actual); err != nil {
		t.Fatal(err)
	}
	if len(actual.OwnerReferences) != 1 || actual.OwnerReferences[0].UID != cluster.UID {
		t.Fatalf("pre-labelled request did not acquire Cluster owner: %v", actual.OwnerReferences)
	}
	reconcileOwnerTest(t, r, ops)
	if err := r.Get(context.Background(), client.ObjectKeyFromObject(ops), actual); err != nil {
		t.Fatal(err)
	}
	if actual.Status.Phase != opsv1alpha1.OpsPendingPhase {
		t.Fatalf("owned request did not advance: %s", actual.Status.Phase)
	}
}

func TestOpsRequestDoesNotRetargetRecreatedCluster(t *testing.T) {
	for _, tc := range []struct {
		name       string
		phase      opsv1alpha1.OpsPhase
		cancel     bool
		deleting   bool
		apiVersion string
	}{
		{name: "action", phase: opsv1alpha1.OpsCreatingPhase},
		{name: "result", phase: opsv1alpha1.OpsRunningPhase},
		{name: "cancel", phase: opsv1alpha1.OpsRunningPhase, cancel: true},
		{name: "cancelling", phase: opsv1alpha1.OpsCancellingPhase, cancel: true},
		{name: "completed", phase: opsv1alpha1.OpsSucceedPhase},
		{name: "deleting", phase: opsv1alpha1.OpsRunningPhase, deleting: true},
		{name: "older-owner-api", phase: opsv1alpha1.OpsCreatingPhase, apiVersion: "apps.kubeblocks.io/v1alpha1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cluster, ops := ownerTestObjects()
			ops.Status.Phase = tc.phase
			if tc.apiVersion != "" {
				ops.OwnerReferences[0].APIVersion = tc.apiVersion
			}
			if tc.cancel {
				ops.Spec.Type = opsv1alpha1.VerticalScalingType
				ops.Spec.Cancel = true
				ops.Labels[constant.OpsRequestTypeLabelKey] = string(ops.Spec.Type)
			}
			r := ownerTestReconciler(t, cluster, ops)
			if err := r.Delete(ctx, cluster); err != nil {
				t.Fatal(err)
			}
			replacement := cluster.DeepCopy()
			replacement.UID = "replacement"
			replacement.ResourceVersion = ""
			replacement.Annotations = map[string]string{constant.OpsRequestAnnotationKey: `[{"name":"new-request"},{"name":"stop-demo"}]`}
			if err := r.Create(ctx, replacement); err != nil {
				t.Fatal(err)
			}
			before := replacement.DeepCopy()
			if tc.deleting {
				if err := r.Delete(ctx, ops); err != nil {
					t.Fatal(err)
				}
			}
			reconcileOwnerTest(t, r, ops)
			actual := &opsv1alpha1.OpsRequest{}
			if !tc.deleting {
				if err := r.Get(ctx, client.ObjectKeyFromObject(ops), actual); err != nil {
					t.Fatal(err)
				}
				if actual.DeletionTimestamp.IsZero() {
					t.Fatal("request outlived its owner without entering deletion")
				}
				if actual.Status.Phase != tc.phase || actual.OwnerReferences[0].UID != cluster.UID {
					t.Fatal("request was rebound or given a new outcome")
				}
				reconcileOwnerTest(t, r, ops)
			}
			if err := r.Get(ctx, client.ObjectKeyFromObject(ops), actual); !apierrors.IsNotFound(err) {
				t.Fatalf("old request was not finalized: %v", err)
			}
			after := &appsv1.Cluster{}
			if err := r.Get(ctx, client.ObjectKeyFromObject(replacement), after); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("old request changed replacement Cluster: before=%#v after=%#v", before, after)
			}
		})
	}
}

func TestOpsRequestSameOwnerStillExecutesStop(t *testing.T) {
	cluster, ops := ownerTestObjects()
	ops.Status.Phase = opsv1alpha1.OpsCreatingPhase
	r := ownerTestReconciler(t, cluster, ops)
	reconcileOwnerTest(t, r, ops)
	actual := &appsv1.Cluster{}
	if err := r.Get(context.Background(), client.ObjectKeyFromObject(cluster), actual); err != nil {
		t.Fatal(err)
	}
	if !ptr.Deref(actual.Spec.ComponentSpecs[0].Stop, false) {
		t.Fatal("Stop did not update its original Cluster")
	}
}

func TestOpsRequestRunningDeletionStillWaitsForCompletion(t *testing.T) {
	cluster, ops := ownerTestObjects()
	cluster.Spec.ComponentSpecs[0].Stop = ptr.To(true)
	cluster.Annotations = map[string]string{constant.OpsRequestAnnotationKey: `[{"name":"stop-demo"}]`}
	ops.Status.Phase = opsv1alpha1.OpsRunningPhase
	r := ownerTestReconciler(t, cluster, ops)
	if err := r.Delete(context.Background(), ops); err != nil {
		t.Fatal(err)
	}
	reconcileOwnerTest(t, r, ops)
	actual := &opsv1alpha1.OpsRequest{}
	if err := r.Get(context.Background(), client.ObjectKeyFromObject(ops), actual); err != nil {
		t.Fatal(err)
	}
	if len(actual.Finalizers) == 0 || actual.Status.Phase != opsv1alpha1.OpsRunningPhase {
		t.Fatalf("Running deletion stopped waiting: %#v", actual)
	}
	after := &appsv1.Cluster{}
	if err := r.Get(context.Background(), client.ObjectKeyFromObject(cluster), after); err != nil {
		t.Fatal(err)
	}
	if after.Annotations[constant.OpsRequestAnnotationKey] != cluster.Annotations[constant.OpsRequestAnnotationKey] {
		t.Fatal("Running deletion removed its queue entry")
	}
}
