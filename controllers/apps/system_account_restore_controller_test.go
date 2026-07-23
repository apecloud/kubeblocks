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

package apps

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/systemaccount"
)

func TestSystemAccountRestoreLifecycleReleasesGoneRoot(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(request).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err != nil {
		t.Fatal(err)
	}

	current := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), current); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatalf("protocol finalizer was not released: %#v", current.Finalizers)
	}
	if current.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] !=
		string(systemaccount.RestoreRequestPhaseFailed) {
		t.Fatalf("phase = %q", current.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey])
	}
	if current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] !=
		systemaccount.RootUnavailableReason {
		t.Fatalf("reason = %q", current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
	}
}

func TestSystemAccountRestoreLifecyclePersistsDeletionFailureBeforeRelease(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	now := metav1.NewTime(time.Now())
	request.DeletionTimestamp = &now
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: request.Namespace,
			Name:      request.Annotations[systemaccount.RootClusterNameAnnotationKey],
			UID:       types.UID(request.Annotations[systemaccount.RootClusterUIDAnnotationKey]),
		},
		Spec: appsv1.ClusterSpec{
			Restore: &appsv1.ClusterRestore{
				Source: appsv1.ClusterRestoreSource{
					APIGroup:  "dataprotection.kubeblocks.io",
					Kind:      "Backup",
					Namespace: request.Namespace,
					Name:      "backup",
				},
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, request).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err != nil {
		t.Fatal(err)
	}

	current := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), current); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatalf("protocol finalizer released before failure projection")
	}
	if current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] !=
		systemaccount.RequestDeletionRequestedReason {
		t.Fatalf("reason = %q", current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
	}
}

func TestSystemAccountRestoreLifecycleClearsExactReceiptBeforeDeletionFailure(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
		string(systemaccount.RestoreRequestPhaseClaimed)
	now := metav1.NewTime(time.Now())
	request.DeletionTimestamp = &now
	cluster := newLifecycleActiveCluster(request)
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "component",
		UID:       "component-uid",
	}}
	target := newLifecycleExactTarget(t, scheme, request, component)
	originalData := target.DeepCopy().Data
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(cluster, component, request, target).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err != nil {
		t.Fatal(err)
	}

	currentRequest := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), currentRequest); err != nil {
		t.Fatal(err)
	}
	if currentRequest.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] !=
		string(systemaccount.RestoreRequestPhaseFailed) {
		t.Fatalf("phase = %q", currentRequest.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey])
	}
	if currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] !=
		systemaccount.RequestDeletionRequestedReason {
		t.Fatalf("reason = %q", currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
	}
	currentTarget := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(target), currentTarget); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(currentTarget.Data, originalData) {
		t.Fatal("deleting request changed target credential data")
	}
	for _, key := range []string{
		systemaccount.RestoreOperationDigestAnnotationKey,
		systemaccount.CredentialIntentRevisionAnnotationKey,
		systemaccount.TargetCommitRevisionAnnotationKey,
		systemaccount.RestoreRequestNameAnnotationKey,
		systemaccount.RestoreRequestUIDAnnotationKey,
	} {
		if currentTarget.Annotations[key] != "" {
			t.Fatalf("target retained protocol receipt %s=%q", key, currentTarget.Annotations[key])
		}
	}
}

func TestSystemAccountRestoreLifecycleReleasesDamagedMetadataAfterRootGone(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.OwnerReferences[0].UID = "replacement-root-uid"
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(request).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err != nil {
		t.Fatal(err)
	}

	current := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), current); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatalf("damaged metadata kept protocol finalizer after root disappeared")
	}
	if current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] !=
		systemaccount.InvalidPhaseReason {
		t.Fatalf("reason = %q", current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
	}
}

func TestSystemAccountRestoreLifecycleFailsClosedForDamagedMetadataWhileRootLive(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.OwnerReferences[0].UID = "replacement-root-uid"
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "cluster",
		UID:       "cluster-uid",
	}, Spec: appsv1.ClusterSpec{
		Restore: &appsv1.ClusterRestore{
			Source: appsv1.ClusterRestoreSource{
				APIGroup:  "dataprotection.kubeblocks.io",
				Kind:      "Backup",
				Namespace: "default",
				Name:      "backup",
			},
		},
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, request).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err == nil {
		t.Fatal("active root accepted damaged request metadata")
	}
}

func TestSystemAccountRestoreLifecycleCommitsExactTargetAfterActiveRecheck(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
		string(systemaccount.RestoreRequestPhaseClaimed)
	cluster := newLifecycleActiveCluster(request)
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "component",
		UID:       "component-uid",
	}}
	intent, err := systemaccount.DecodeRestoreRequestV2(request)
	if err != nil {
		t.Fatal(err)
	}
	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  request.Namespace,
			Name:       "cluster-component-admin",
			UID:        "target-uid",
			Finalizers: []string{constant.DBComponentFinalizerName},
			Annotations: map[string]string{
				constant.SystemAccountProvisionedAnnotationKey:      "true",
				systemaccount.RestoreProtocolAnnotationKey:          systemaccount.RestoreProtocolV2,
				systemaccount.RestoreOperationDigestAnnotationKey:   request.Annotations[systemaccount.RestoreOperationDigestAnnotationKey],
				systemaccount.CredentialIntentRevisionAnnotationKey: request.Annotations[systemaccount.CredentialIntentRevisionAnnotationKey],
				systemaccount.RestoreRequestNameAnnotationKey:       request.Name,
				systemaccount.RestoreRequestUIDAnnotationKey:        string(request.UID),
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: intent.Credentials,
	}
	if err := controllerutil.SetControllerReference(component, target, scheme); err != nil {
		t.Fatal(err)
	}
	revision, err := systemaccount.TargetCommitRevision(target, constant.DBComponentFinalizerName)
	if err != nil {
		t.Fatal(err)
	}
	target.Annotations[systemaccount.TargetCommitRevisionAnnotationKey] = revision
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(cluster, component, request, target).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err != nil {
		t.Fatal(err)
	}

	current := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), current); err != nil {
		t.Fatal(err)
	}
	if current.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] !=
		string(systemaccount.RestoreRequestPhaseCommitted) {
		t.Fatalf("phase = %q", current.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey])
	}
	if current.Annotations[systemaccount.TargetSecretNameAnnotationKey] != target.Name ||
		current.Annotations[systemaccount.TargetSecretUIDAnnotationKey] != string(target.UID) ||
		current.Annotations[systemaccount.TargetCommitRevisionAnnotationKey] != revision {
		t.Fatalf("committed receipt is incomplete: %#v", current.Annotations)
	}
	if !slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatal("active operation commit released the protocol finalizer")
	}
}

func TestSystemAccountRestoreLifecycleDoesNotCommitTerminalTargetWithUnavailableOwner(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
		string(systemaccount.RestoreRequestPhaseClaimed)
	cluster := newLifecycleActiveCluster(request)
	cluster.Status.Conditions = []metav1.Condition{{
		Type:               appsv1.ConditionTypeRestore,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: cluster.Generation,
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, request).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err != nil {
		t.Fatal(err)
	}

	current := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), current); err != nil {
		t.Fatal(err)
	}
	if current.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] !=
		string(systemaccount.RestoreRequestPhaseFailed) {
		t.Fatalf("phase = %q", current.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey])
	}
	if current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] !=
		systemaccount.OperationTerminalReason {
		t.Fatalf("reason = %q", current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
	}
	if slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatal("terminal unavailable-owner request retained the protocol finalizer")
	}
}

func TestSystemAccountRestoreLifecycleRequiresExactAppsAuthorityGroup(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	controller := true
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "cluster",
		UID:       "cluster-uid",
	}}
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "component",
		UID:       "component-uid",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: appsv1.GroupVersion.String(),
			Kind:       appsv1.ClusterKind,
			Name:       cluster.Name,
			UID:        cluster.UID,
			Controller: &controller,
		}},
	}}
	workload := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "workload",
		UID:       "workload-uid",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "apps.kubeblocks.io/v1alpha1",
			Kind:       appsv1.ComponentKind,
			Name:       component.Name,
			UID:        component.UID,
			Controller: &controller,
		}},
	}}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "data",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: workloads.GroupVersion.String(),
			Kind:       workloads.InstanceSetKind,
			Name:       workload.Name,
			UID:        workload.UID,
			Controller: &controller,
		}},
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(cluster, component, workload, pvc).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, Scheme: scheme}

	live, err := reconciler.targetOwnerLive(context.Background(), systemaccount.ObjectIdentity{
		APIVersion: "apps.kubeblocks.io/v1alpha1",
		Kind:       appsv1.ComponentKind,
		Namespace:  component.Namespace,
		Name:       component.Name,
		UID:        component.UID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if live {
		t.Fatal("target owner with a non-exact API version was accepted")
	}
	if reconciler.legacyPVCHasAuthority(context.Background(), pvc, systemaccount.ObjectIdentity{
		APIVersion: appsv1.GroupVersion.String(),
		Kind:       appsv1.ClusterKind,
		Namespace:  cluster.Namespace,
		Name:       cluster.Name,
		UID:        cluster.UID,
	}) {
		t.Fatal("legacy workload chain with a non-exact Component API version was accepted")
	}

	workload.OwnerReferences[0].APIVersion = appsv1.GroupVersion.String()
	if err := cli.Update(context.Background(), workload); err != nil {
		t.Fatal(err)
	}
	component.OwnerReferences[0].APIVersion = "apps.kubeblocks.io/v1alpha1"
	if err := cli.Update(context.Background(), component); err != nil {
		t.Fatal(err)
	}
	if reconciler.legacyPVCHasAuthority(context.Background(), pvc, systemaccount.ObjectIdentity{
		APIVersion: appsv1.GroupVersion.String(),
		Kind:       appsv1.ClusterKind,
		Namespace:  cluster.Namespace,
		Name:       cluster.Name,
		UID:        cluster.UID,
	}) {
		t.Fatal("legacy workload chain with a non-exact Cluster API version was accepted")
	}
}

func newSystemAccountLifecycleScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func newLifecycleRestoreRequest(t *testing.T) *corev1.Secret {
	t.Helper()
	root := systemaccount.ObjectIdentity{
		APIVersion: appsv1.GroupVersion.String(),
		Kind:       appsv1.ClusterKind,
		Namespace:  "default",
		Name:       "cluster",
		UID:        "cluster-uid",
	}
	request, err := systemaccount.BuildRestoreRequest(systemaccount.CredentialIntent{
		Operation: systemaccount.RestoreOperationIdentity{
			Protocol: systemaccount.RestoreProtocolV2,
			Profile:  systemaccount.RestoreProfileInitialCluster,
			Root:     root,
			Source: systemaccount.SourceIdentity{
				APIGroup:  "dataprotection.kubeblocks.io",
				Kind:      "Backup",
				Namespace: "default",
				Name:      "backup",
			},
		},
		Target: systemaccount.LogicalTargetIdentity{
			Protocol:  systemaccount.LogicalTargetProtocolV1,
			Namespace: "default",
			Root:      root,
			Owner: systemaccount.ObjectIdentity{
				APIVersion: appsv1.GroupVersion.String(),
				Kind:       appsv1.ComponentKind,
				Namespace:  "default",
				Name:       "component",
				UID:        "component-uid",
			},
			Scope:   systemaccount.SystemAccountScopeComponent,
			Account: "admin",
		},
		ResolvedSource: systemaccount.ObjectIdentity{
			APIVersion: "dataprotection.kubeblocks.io/v1alpha1",
			Kind:       "Backup",
			Namespace:  "default",
			Name:       "backup",
			UID:        "backup-uid",
		},
		Credentials: map[string][]byte{
			constant.AccountNameForSecret:   []byte("admin"),
			constant.AccountPasswdForSecret: []byte("password"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	request.UID = "request-uid"
	request.ResourceVersion = "1"
	return request
}

func newLifecycleActiveCluster(request *corev1.Secret) *appsv1.Cluster {
	return &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: request.Namespace,
			Name:      request.Annotations[systemaccount.RootClusterNameAnnotationKey],
			UID:       types.UID(request.Annotations[systemaccount.RootClusterUIDAnnotationKey]),
		},
		Spec: appsv1.ClusterSpec{
			Restore: &appsv1.ClusterRestore{
				Source: appsv1.ClusterRestoreSource{
					APIGroup:  "dataprotection.kubeblocks.io",
					Kind:      "Backup",
					Namespace: request.Namespace,
					Name:      "backup",
				},
			},
		},
	}
}

func newLifecycleExactTarget(
	t *testing.T,
	scheme *runtime.Scheme,
	request *corev1.Secret,
	component *appsv1.Component,
) *corev1.Secret {
	t.Helper()
	intent, err := systemaccount.DecodeRestoreRequestV2(request)
	if err != nil {
		t.Fatal(err)
	}
	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  request.Namespace,
			Name:       "cluster-component-admin",
			UID:        "target-uid",
			Finalizers: []string{constant.DBComponentFinalizerName},
			Annotations: map[string]string{
				constant.SystemAccountProvisionedAnnotationKey:      "true",
				systemaccount.RestoreProtocolAnnotationKey:          systemaccount.RestoreProtocolV2,
				systemaccount.RestoreOperationDigestAnnotationKey:   request.Annotations[systemaccount.RestoreOperationDigestAnnotationKey],
				systemaccount.CredentialIntentRevisionAnnotationKey: request.Annotations[systemaccount.CredentialIntentRevisionAnnotationKey],
				systemaccount.RestoreRequestNameAnnotationKey:       request.Name,
				systemaccount.RestoreRequestUIDAnnotationKey:        string(request.UID),
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: intent.Credentials,
	}
	if err := controllerutil.SetControllerReference(component, target, scheme); err != nil {
		t.Fatal(err)
	}
	revision, err := systemaccount.TargetCommitRevision(target, constant.DBComponentFinalizerName)
	if err != nil {
		t.Fatal(err)
	}
	target.Annotations[systemaccount.TargetCommitRevisionAnnotationKey] = revision
	return target
}
