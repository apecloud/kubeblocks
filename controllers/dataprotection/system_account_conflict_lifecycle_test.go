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

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/systemaccount"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestConflictReceiptLifecycleReleasesGoneRoot(t *testing.T) {
	scheme := newConflictLifecycleScheme(t)
	receipt := newConflictLifecycleReceipt(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(receipt).Build()
	reconciler := &SystemAccountConflictReceiptLifecycleReconciler{
		Client: cli, APIReader: cli, Scheme: scheme,
	}

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
	reconciler := &SystemAccountConflictReceiptLifecycleReconciler{
		Client: cli, APIReader: cli, Scheme: scheme,
	}

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

func TestSameOperationShardingParticipantsReuseWinnerRequest(t *testing.T) {
	root := systemaccount.ObjectIdentity{
		APIVersion: appsv1.GroupVersion.String(),
		Kind:       appsv1.ClusterKind,
		Namespace:  "default",
		Name:       "cluster",
		UID:        "cluster-uid",
	}
	target := systemaccount.LogicalTargetIdentity{
		Protocol:     systemaccount.LogicalTargetProtocolV1,
		Namespace:    root.Namespace,
		Root:         root,
		Owner:        root,
		Scope:        systemaccount.SystemAccountScopeSharding,
		ShardingName: "shard",
		Account:      "admin",
	}
	first := conflictLifecycleIntent(root, target, "backup", "backup-uid")
	first.AuthorityWitness = &systemaccount.ObjectIdentity{
		APIVersion: appsv1.GroupVersion.String(),
		Kind:       appsv1.ComponentKind,
		Namespace:  "default",
		Name:       "cluster-shard-0",
		UID:        "shard-0-uid",
	}
	second := first
	secondWitness := *first.AuthorityWitness
	secondWitness.Name = "cluster-shard-1"
	secondWitness.UID = "shard-1-uid"
	second.AuthorityWitness = &secondWitness
	firstRequest, err := systemaccount.BuildRestoreRequest(first)
	if err != nil {
		t.Fatal(err)
	}
	secondRequest, err := systemaccount.BuildRestoreRequest(second)
	if err != nil {
		t.Fatal(err)
	}

	if firstRequest.Name != secondRequest.Name {
		t.Fatalf("same operation participants split request slot: %s != %s",
			firstRequest.Name, secondRequest.Name)
	}
	if err := validateSystemAccountRestoreRequest(firstRequest, secondRequest); err != nil {
		t.Fatalf("same operation participants conflicted: %v", err)
	}
}

func TestConflictReceiptLifecycleReleasesDamagedMetadataAfterRootGone(t *testing.T) {
	scheme := newConflictLifecycleScheme(t)
	receipt := newConflictLifecycleReceipt(t)
	receipt.OwnerReferences[0].UID = "replacement-root-uid"
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(receipt).Build()
	reconciler := &SystemAccountConflictReceiptLifecycleReconciler{
		Client: cli, APIReader: cli, Scheme: scheme,
	}

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

func TestConflictReceiptLifecycleRecoversDamagedProtocolMirror(t *testing.T) {
	for _, protocol := range []string{"", "foreign"} {
		t.Run(protocol, func(t *testing.T) {
			scheme := newConflictLifecycleScheme(t)
			receipt := newConflictLifecycleReceipt(t)
			if protocol == "" {
				delete(receipt.Annotations, systemaccount.RestoreProtocolAnnotationKey)
			} else {
				receipt.Annotations[systemaccount.RestoreProtocolAnnotationKey] = protocol
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(receipt).Build()
			reconciler := &SystemAccountConflictReceiptLifecycleReconciler{
				Client: cli, APIReader: cli, Scheme: scheme,
			}

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
				t.Fatalf("protocol %q kept finalizer after root disappeared", protocol)
			}
		})
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
	reconciler := &SystemAccountConflictReceiptLifecycleReconciler{
		Client: cli, APIReader: cli, Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(receipt),
	}); err == nil {
		t.Fatal("active root accepted damaged receipt metadata")
	}
}

func TestConflictReceiptLifecycleDamagedProtocolMirrorFailsClosedWhileRootLive(t *testing.T) {
	for _, protocol := range []string{"", "foreign"} {
		t.Run(protocol, func(t *testing.T) {
			scheme := newConflictLifecycleScheme(t)
			receipt := newConflictLifecycleReceipt(t)
			if protocol == "" {
				delete(receipt.Annotations, systemaccount.RestoreProtocolAnnotationKey)
			} else {
				receipt.Annotations[systemaccount.RestoreProtocolAnnotationKey] = protocol
			}
			cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "cluster",
				UID:       "cluster-uid",
			}}
			cli := fake.NewClientBuilder().WithScheme(scheme).
				WithObjects(cluster, receipt).Build()
			reconciler := &SystemAccountConflictReceiptLifecycleReconciler{
				Client: cli, APIReader: cli, Scheme: scheme,
			}

			if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(receipt),
			}); err == nil {
				t.Fatalf("active root accepted protocol mirror %q", protocol)
			}
			current := &corev1.Secret{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(receipt), current); err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
				t.Fatalf("active root released finalizer for protocol mirror %q", protocol)
			}
		})
	}
}

func TestConflictReceiptLifecycleProjectsVerifiedParticipantsForTerminatingRoot(t *testing.T) {
	scheme := newConflictLifecycleScheme(t)
	pvc := &corev1.PersistentVolumeClaim{}
	backup, cluster, component, instanceSet :=
		prepareSystemAccountAuthorityFixture(NewWithT(t), scheme, pvc)
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(pvc, backup, cluster, component, instanceSet).Build()
	volumePopulator := &VolumePopulatorReconciler{Client: cli, APIReader: cli, Scheme: scheme}
	authority, err := volumePopulator.resolveSystemAccountRestoreAuthority(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, cluster.Name)
	if err != nil {
		t.Fatal(err)
	}
	reconciler := &SystemAccountConflictReceiptLifecycleReconciler{
		Client: cli, APIReader: cli, Scheme: scheme,
	}

	live, err := reconciler.participantsForOperation(context.Background(), authority.operation)
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 1 || live[0].UID != pvc.UID {
		t.Fatalf("live participants = %#v, want exact PVC UID %s", live, pvc.UID)
	}

	now := metav1.Now()
	terminatingCluster := cluster.DeepCopy()
	terminatingCluster.ResourceVersion = ""
	terminatingCluster.DeletionTimestamp = &now
	terminatingCluster.Finalizers = []string{"test.kubeblocks.io/hold"}
	terminatingClient := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(pvc.DeepCopy(), backup.DeepCopy(), terminatingCluster,
			component.DeepCopy(), instanceSet.DeepCopy()).Build()
	reconciler.Client = terminatingClient
	reconciler.APIReader = terminatingClient

	terminating, err := reconciler.participantsForOperation(context.Background(), authority.operation)
	if err != nil {
		t.Fatal(err)
	}
	if len(terminating) != 1 || terminating[0].UID != pvc.UID {
		t.Fatalf("terminating-root participants = %#v, want exact PVC UID %s", terminating, pvc.UID)
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
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := workloads.AddToScheme(scheme); err != nil {
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
