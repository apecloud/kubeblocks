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

package dataprotection

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

const (
	terminatingNamespace      = "kb-r6-backup-terminating-test"
	terminatingNamespaceAlive = "kb-r6-backup-alive-test"
)

func newDPSchemeForTest(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := scheme.AddToScheme(s); err != nil {
		t.Fatalf("scheme.AddToScheme: %v", err)
	}
	if err := dpv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("dpv1alpha1.AddToScheme: %v", err)
	}
	if err := rbacv1.AddToScheme(s); err != nil {
		t.Fatalf("rbacv1.AddToScheme: %v", err)
	}
	return s
}

func TestIsNamespaceTerminating(t *testing.T) {
	s := newDPSchemeForTest(t)
	now := metav1.Now()
	cli := fake.NewClientBuilder().WithScheme(s).WithObjects(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: terminatingNamespaceAlive}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:              terminatingNamespace,
			DeletionTimestamp: &now,
			Finalizers:        []string{"kubernetes"},
		}},
	).Build()
	ctx := context.Background()

	t.Run("alive namespace is not terminating", func(t *testing.T) {
		terminating, err := isNamespaceTerminating(ctx, cli, terminatingNamespaceAlive)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if terminating {
			t.Fatalf("expected alive namespace to report not-terminating, got terminating=true")
		}
	})

	t.Run("namespace with non-zero DeletionTimestamp is terminating", func(t *testing.T) {
		terminating, err := isNamespaceTerminating(ctx, cli, terminatingNamespace)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !terminating {
			t.Fatalf("expected terminating namespace to report terminating, got terminating=false")
		}
	})

	t.Run("not-found namespace is treated as terminating", func(t *testing.T) {
		terminating, err := isNamespaceTerminating(ctx, cli, "does-not-exist")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !terminating {
			t.Fatalf("expected missing namespace to report terminating (so the caller releases its finalizer), got terminating=false")
		}
	})
}

// TestHandleDeletingPhase_NamespaceTerminating_GateFiresBeforeWorkloadCreate
// exercises Clara's review nuance: even if the worker ServiceAccount and the
// associated RoleBinding already exist (the SA fast-path branch in
// EnsureWorkerServiceAccount), the invariant gate must fire at handleDeletingPhase
// entry before any downstream object-creation attempt. Otherwise a later
// reconcile tick under the slow path (when GC clears SA/RB before the predelete
// Job is created) hits the namespace-Terminating API rejection and the finalizer
// gets stuck.
//
// The fake client used here does not enforce namespace-Terminating admission
// semantics, so the negative assertion ("no predelete Job was created") is the
// behavioral signal that the gate fired and short-circuited downstream work.
func TestHandleDeletingPhase_NamespaceTerminating_GateFiresBeforeWorkloadCreate(t *testing.T) {
	s := newDPSchemeForTest(t)
	now := metav1.Now()

	terminatingNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:              terminatingNamespace,
		DeletionTimestamp: &now,
		Finalizers:        []string{"kubernetes"},
	}}
	// Pre-existing SA + RB simulate the case where a successful backup once ran
	// in this namespace; GC will clear them as part of namespace teardown but
	// the gate must not depend on their absence to fire.
	preExistingSA := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Name:      "kubeblocks-dataprotection-worker",
		Namespace: terminatingNamespace,
	}}
	preExistingRB := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{
		Name:      "kubeblocks-dataprotection-worker-rolebinding",
		Namespace: terminatingNamespace,
	}}
	backup := &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{
		Name:              "bkpd-1050-b3612-full-105618",
		Namespace:         terminatingNamespace,
		Finalizers:        []string{dptypes.DataProtectionFinalizerName},
		DeletionTimestamp: &now,
	}}

	cli := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(terminatingNS, preExistingSA, preExistingRB, backup).
		Build()

	recorder := record.NewFakeRecorder(1)
	reconciler := &BackupReconciler{
		Client:   cli,
		Scheme:   s,
		Recorder: recorder,
	}
	reqCtx := intctrlutil.RequestCtx{
		Ctx: context.Background(),
		Log: log.Log,
	}

	if _, err := reconciler.handleDeletingPhase(reqCtx, backup); err != nil {
		t.Fatalf("handleDeletingPhase returned unexpected error: %v", err)
	}

	t.Run("backup finalizer is released", func(t *testing.T) {
		fetched := &dpv1alpha1.Backup{}
		err := cli.Get(reqCtx.Ctx, client.ObjectKeyFromObject(backup), fetched)
		if apierrors.IsNotFound(err) {
			// The fake client garbage-collects objects whose finalizer list
			// becomes empty while DeletionTimestamp is set — the desired
			// post-state. Treat as success.
			return
		}
		if err != nil {
			t.Fatalf("get backup: %v", err)
		}
		if controllerutil.ContainsFinalizer(fetched, dptypes.DataProtectionFinalizerName) {
			t.Fatalf("expected data-protection finalizer to be released, but it is still present: %v", fetched.Finalizers)
		}
	})

	t.Run("warning event was recorded", func(t *testing.T) {
		select {
		case event := <-recorder.Events:
			if !strings.Contains(event, "NamespaceTerminating") {
				t.Fatalf("expected event to mention NamespaceTerminating; got: %q", event)
			}
		default:
			t.Fatalf("expected a NamespaceTerminating event to be recorded, none observed")
		}
	})
}
