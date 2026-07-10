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

package v1alpha1

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestLifecycleActionExecutionCRDContract(t *testing.T) {
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("start envtest: %v", err)
	}
	t.Cleanup(func() {
		if err := testEnv.Stop(); err != nil {
			t.Errorf("stop envtest: %v", err)
		}
	})

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	const namespace = "lae-api"
	if err := k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}); err != nil {
		t.Fatal(err)
	}

	execution := validLifecycleActionExecution(t)
	execution.Namespace = namespace
	execution.Spec.SourceRef.Namespace = namespace
	execution.Spec.WorkloadRef.Namespace = namespace
	execution.Spec.Target.Pod.Namespace = namespace
	recomputeExecutionIdentity(t, execution)
	if err := k8sClient.Create(ctx, execution); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	stored := &LifecycleActionExecution{}
	key := client.ObjectKeyFromObject(execution)
	if err := k8sClient.Get(ctx, key, stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.Phase != LifecycleActionExecutionPhaseUnobserved {
		t.Fatalf("API default did not persist Unobserved: got %q", stored.Status.Phase)
	}

	stored.Spec.ActionName = "other"
	if err := k8sClient.Update(ctx, stored); err == nil {
		t.Fatalf("API server accepted immutable spec mutation")
	}
	if err := k8sClient.Get(ctx, key, stored); err != nil {
		t.Fatal(err)
	}

	stored.Status = status(LifecycleActionExecutionPhasePending)
	if err := k8sClient.Status().Update(ctx, stored); err != nil {
		t.Fatalf("Unobserved -> Pending: %v", err)
	}
	if err := k8sClient.Get(ctx, key, stored); err != nil {
		t.Fatal(err)
	}
	stored.Status.Reason = LifecycleActionReasonActionRejected
	if err := k8sClient.Status().Update(ctx, stored); err == nil {
		t.Fatalf("API server accepted same-phase status mutation")
	}

	if err := k8sClient.Get(ctx, key, stored); err != nil {
		t.Fatal(err)
	}
	start := metav1.NewTime(time.Unix(100, 0).UTC())
	finish := metav1.NewTime(time.Unix(200, 0).UTC())
	stored.Status = LifecycleActionExecutionStatus{
		Phase: LifecycleActionExecutionPhaseCancelled, Reason: LifecycleActionReasonCancelledBySource,
		StartTime: &start, FinishedTime: &finish,
	}
	if err := k8sClient.Status().Update(ctx, stored); err == nil {
		t.Fatalf("API server accepted Pending -> Cancelled with invented startTime")
	}
	if err := k8sClient.Get(ctx, key, stored); err != nil {
		t.Fatal(err)
	}
	stored.Status = failedStatus(&start, finish, LifecycleActionFailureClassPermanent, LifecycleActionReasonActionRejected, nil)
	if err := k8sClient.Status().Update(ctx, stored); err == nil {
		t.Fatalf("API server accepted Pending -> Failed with invented startTime")
	}
	if err := k8sClient.Get(ctx, key, stored); err != nil {
		t.Fatal(err)
	}
	stored.Status = runningStatus(start)
	if err := k8sClient.Status().Update(ctx, stored); err != nil {
		t.Fatalf("Pending -> Running: %v", err)
	}
	if err := k8sClient.Get(ctx, key, stored); err != nil {
		t.Fatal(err)
	}
	stored.Status = failedStatus(&start, finish, LifecycleActionFailureClassRetryable, LifecycleActionReasonTargetUnavailable, nil)
	if err := k8sClient.Status().Update(ctx, stored); err == nil {
		t.Fatalf("API server accepted post-dispatch TargetUnavailable")
	}
	if err := k8sClient.Get(ctx, key, stored); err != nil {
		t.Fatal(err)
	}
	stored.Status = LifecycleActionExecutionStatus{
		Phase: LifecycleActionExecutionPhaseCancelled, Reason: LifecycleActionReasonCancelledBySource,
		StartTime: &start, FinishedTime: &finish,
	}
	if err := k8sClient.Status().Update(ctx, stored); err == nil {
		t.Fatalf("API server accepted Running -> Cancelled")
	}

	if err := k8sClient.Get(ctx, key, stored); err != nil {
		t.Fatal(err)
	}
	stored.Status = failedStatus(nil, finish, LifecycleActionFailureClassUnknown, LifecycleActionReasonConfirmationLost, nil)
	if err := k8sClient.Status().Update(ctx, stored); err == nil {
		t.Fatalf("API server accepted Running -> Failed without startTime")
	}
	if err := k8sClient.Get(ctx, key, stored); err != nil {
		t.Fatal(err)
	}
	changedStart := metav1.NewTime(time.Unix(101, 0).UTC())
	stored.Status = failedStatus(&changedStart, finish, LifecycleActionFailureClassUnknown, LifecycleActionReasonConfirmationLost, nil)
	if err := k8sClient.Status().Update(ctx, stored); err == nil {
		t.Fatalf("API server accepted Running -> Failed with changed startTime")
	}
	if err := k8sClient.Get(ctx, key, stored); err != nil {
		t.Fatal(err)
	}
	stored.Status = succeededStatus(changedStart, finish)
	if err := k8sClient.Status().Update(ctx, stored); err == nil {
		t.Fatalf("API server accepted Running -> Succeeded with changed startTime")
	}

	if err := k8sClient.Get(ctx, key, stored); err != nil {
		t.Fatal(err)
	}
	stored.Status = succeededStatus(start, finish)
	if err := k8sClient.Status().Update(ctx, stored); err != nil {
		t.Fatalf("Running -> Succeeded: %v", err)
	}
	if err := k8sClient.Get(ctx, key, stored); err != nil {
		t.Fatal(err)
	}
	stored.Status.FinishedTime = ptrTime(metav1.NewTime(time.Unix(201, 0).UTC()))
	if err := k8sClient.Status().Update(ctx, stored); err == nil {
		t.Fatalf("API server accepted terminal status mutation")
	}
}

func recomputeExecutionIdentity(t *testing.T, execution *LifecycleActionExecution) {
	t.Helper()
	key, _, err := ComputeLifecycleActionInvocationKey(execution.Spec)
	if err != nil {
		t.Fatal(err)
	}
	execution.Spec.InvocationKey = key
	name, _, err := ComputeLifecycleActionExecutionName(execution.Spec)
	if err != nil {
		t.Fatal(err)
	}
	execution.Name = name
}

func ptrTime(value metav1.Time) *metav1.Time {
	return &value
}
