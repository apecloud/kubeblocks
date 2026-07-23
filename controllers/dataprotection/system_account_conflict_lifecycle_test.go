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

package dataprotection

import (
	"context"
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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/systemaccount"
)

func TestConflictReceiptLifecycleReleasesGoneRoot(t *testing.T) {
	scheme := newConflictLifecycleScheme(t)
	receipt := newConflictLifecycleReceipt(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(receipt).Build()
	reconciler := &SystemAccountConflictReceiptLifecycleReconciler{Client: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(receipt),
	}); err != nil {
		t.Fatal(err)
	}

	current := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(receipt), current); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatalf("protocol finalizer was not released: %#v", current.Finalizers)
	}
}

func TestConflictReceiptLifecycleRetainsDeletingReceiptWhileRootLive(t *testing.T) {
	scheme := newConflictLifecycleScheme(t)
	receipt := newConflictLifecycleReceipt(t)
	now := metav1.NewTime(time.Now())
	receipt.DeletionTimestamp = &now
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "cluster",
		UID:       "cluster-uid",
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, receipt).Build()
	reconciler := &SystemAccountConflictReceiptLifecycleReconciler{Client: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(receipt),
	}); err != nil {
		t.Fatal(err)
	}

	current := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(receipt), current); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatal("live-root conflict receipt lost its protocol finalizer")
	}
}

func TestConflictReceiptLifecycleReleasesDamagedMetadataAfterRootGone(t *testing.T) {
	scheme := newConflictLifecycleScheme(t)
	receipt := newConflictLifecycleReceipt(t)
	receipt.OwnerReferences[0].UID = "replacement-root-uid"
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(receipt).Build()
	reconciler := &SystemAccountConflictReceiptLifecycleReconciler{Client: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(receipt),
	}); err != nil {
		t.Fatal(err)
	}

	current := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(receipt), current); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatalf("damaged metadata kept protocol finalizer after root disappeared")
	}
}

func TestConflictReceiptLifecycleFailsClosedForDamagedMetadataWhileRootLive(t *testing.T) {
	scheme := newConflictLifecycleScheme(t)
	receipt := newConflictLifecycleReceipt(t)
	receipt.OwnerReferences[0].UID = "replacement-root-uid"
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "cluster",
		UID:       "cluster-uid",
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, receipt).Build()
	reconciler := &SystemAccountConflictReceiptLifecycleReconciler{Client: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(receipt),
	}); err == nil {
		t.Fatal("active root accepted damaged receipt metadata")
	}
}

func newConflictLifecycleScheme(t *testing.T) *runtime.Scheme {
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

func newConflictLifecycleReceipt(t *testing.T) *corev1.Secret {
	t.Helper()
	root := systemaccount.ObjectIdentity{
		APIVersion: appsv1.GroupVersion.String(),
		Kind:       appsv1.ClusterKind,
		Namespace:  "default",
		Name:       "cluster",
		UID:        "cluster-uid",
	}
	target := systemaccount.LogicalTargetIdentity{
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
	}
	winner := conflictLifecycleIntent(root, target, "winner", "winner-backup-uid")
	loser := conflictLifecycleIntent(root, target, "loser", "loser-backup-uid")
	request, err := systemaccount.BuildRestoreRequest(winner)
	if err != nil {
		t.Fatal(err)
	}
	request.UID = "request-uid"
	request.ResourceVersion = "7"
	receipt, err := systemaccount.BuildConflictReceipt(loser, request)
	if err != nil {
		t.Fatal(err)
	}
	receipt.UID = "receipt-uid"
	receipt.ResourceVersion = "8"
	return receipt
}

func conflictLifecycleIntent(
	root systemaccount.ObjectIdentity,
	target systemaccount.LogicalTargetIdentity,
	backupName string,
	backupUID types.UID,
) systemaccount.CredentialIntent {
	return systemaccount.CredentialIntent{
		Operation: systemaccount.RestoreOperationIdentity{
			Protocol: systemaccount.RestoreProtocolV2,
			Profile:  systemaccount.RestoreProfileLegacyPVCGroup,
			Root:     root,
			Source: systemaccount.SourceIdentity{
				APIGroup:  "dataprotection.kubeblocks.io",
				Kind:      "Backup",
				Namespace: "default",
				Name:      backupName,
			},
		},
		Target: target,
		ResolvedSource: systemaccount.ObjectIdentity{
			APIVersion: "dataprotection.kubeblocks.io/v1alpha1",
			Kind:       "Backup",
			Namespace:  "default",
			Name:       backupName,
			UID:        backupUID,
		},
		Credentials: map[string][]byte{
			constant.AccountNameForSecret:   []byte("admin"),
			constant.AccountPasswdForSecret: []byte("password"),
		},
	}
}
