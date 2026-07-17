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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clocktesting "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestReconcileDeleteJobSchedulingDiagnostic(t *testing.T) {
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	fakeClock := clocktesting.NewFakeClock(now)
	originalClock := wallClock
	wallClock = fakeClock
	defer func() { wallClock = originalClock }()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "ns", UID: types.UID("backup-uid")},
		Status: dpv1alpha1.BackupStatus{
			Phase:         dpv1alpha1.BackupPhaseDeleting,
			FailureReason: "original backup failure",
		},
	}
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: "delete-backup", Namespace: "ns", UID: types.UID("job-uid"),
	}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "delete-backup-pod",
			Namespace: "ns",
			UID:       types.UID("pod-uid"),
			Labels:    map[string]string{"job-name": job.Name},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: batchv1.SchemeGroupVersion.String(), Kind: "Job", Name: job.Name, UID: job.UID,
			}},
		},
		Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{
			Type:               corev1.PodScheduled,
			Status:             corev1.ConditionFalse,
			Reason:             corev1.PodReasonUnschedulable,
			Message:            "0/3 nodes are available: node selector conflicts with volume node affinity",
			LastTransitionTime: metav1.NewTime(now.Add(-6 * time.Minute)),
		}}},
	}

	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&dpv1alpha1.Backup{}, &corev1.Pod{}).
		WithObjects(backup, job, pod).
		Build()
	recorder := record.NewFakeRecorder(10)
	reconciler := &BackupReconciler{Client: cli, Scheme: scheme, Recorder: recorder}
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}

	result, err := reconciler.reconcileDeleteJobSchedulingDiagnostic(reqCtx, backup.DeepCopy(), job.DeepCopy())
	require.NoError(t, err)
	assert.Equal(t, deleteJobDiagnosticRefreshInterval, result.RequeueAfter)

	got := &dpv1alpha1.Backup{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(backup), got))
	require.NotNil(t, got.Status.DeletionDiagnostic)
	assert.Equal(t, job.Name, got.Status.DeletionDiagnostic.ActiveJobName)
	assert.Equal(t, string(job.UID), got.Status.DeletionDiagnostic.ActiveJobUID)
	assert.Equal(t, string(pod.UID), got.Status.DeletionDiagnostic.PodUID)
	assert.Equal(t, deletionSchedulingConflictReason, got.Status.DeletionDiagnostic.Reason)
	assert.Equal(t, "original backup failure", got.Status.FailureReason)

	select {
	case event := <-recorder.Events:
		assert.Contains(t, event, deletionSchedulingConflictReason)
	case <-time.After(time.Second):
		t.Fatal("expected the first scheduling conflict event")
	}

	// A fresh reconciler must use the persisted diagnostic as the event dedupe source.
	reconciler = &BackupReconciler{Client: cli, Scheme: scheme, Recorder: recorder}
	result, err = reconciler.reconcileDeleteJobSchedulingDiagnostic(reqCtx, got.DeepCopy(), job.DeepCopy())
	require.NoError(t, err)
	assert.Equal(t, deleteJobDiagnosticRefreshInterval, result.RequeueAfter)
	select {
	case event := <-recorder.Events:
		t.Fatalf("unexpected duplicate event: %s", event)
	default:
	}

	// A replacement Job/Pod identity replaces the persisted diagnostic and
	// emits one event for the new active worker.
	replacementJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: "delete-backup-replacement", Namespace: "ns", UID: types.UID("replacement-job-uid"),
	}}
	replacementPod := pod.DeepCopy()
	replacementPod.Name = "delete-backup-replacement-pod"
	replacementPod.UID = types.UID("replacement-pod-uid")
	replacementPod.ResourceVersion = ""
	replacementPod.Labels["job-name"] = replacementJob.Name
	replacementPod.OwnerReferences[0].Name = replacementJob.Name
	replacementPod.OwnerReferences[0].UID = replacementJob.UID
	require.NoError(t, cli.Create(context.Background(), replacementJob))
	require.NoError(t, cli.Create(context.Background(), replacementPod))
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(backup), got))

	result, err = reconciler.reconcileDeleteJobSchedulingDiagnostic(reqCtx, got.DeepCopy(), replacementJob.DeepCopy())
	require.NoError(t, err)
	assert.Equal(t, deleteJobDiagnosticRefreshInterval, result.RequeueAfter)
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(backup), got))
	require.NotNil(t, got.Status.DeletionDiagnostic)
	assert.Equal(t, string(replacementJob.UID), got.Status.DeletionDiagnostic.ActiveJobUID)
	assert.Equal(t, string(replacementPod.UID), got.Status.DeletionDiagnostic.PodUID)
	select {
	case event := <-recorder.Events:
		assert.Contains(t, event, replacementJob.Name)
	case <-time.After(time.Second):
		t.Fatal("expected an event for the replacement worker identity")
	}

	// A scheduled pod clears only the deletion diagnostic.
	currentPod := &corev1.Pod{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(replacementPod), currentPod))
	currentPod.Status.Conditions = []corev1.PodCondition{{
		Type: corev1.PodScheduled, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(now),
	}}
	require.NoError(t, cli.Status().Update(context.Background(), currentPod))
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(backup), got))

	result, err = reconciler.reconcileDeleteJobSchedulingDiagnostic(reqCtx, got.DeepCopy(), replacementJob.DeepCopy())
	require.NoError(t, err)
	assert.Equal(t, deleteJobDiagnosticRefreshInterval, result.RequeueAfter)
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(backup), got))
	assert.Nil(t, got.Status.DeletionDiagnostic)
	assert.Equal(t, "original backup failure", got.Status.FailureReason)
}

func TestReconcileDeleteJobSchedulingDiagnosticBeforeWindow(t *testing.T) {
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	fakeClock := clocktesting.NewFakeClock(now)
	originalClock := wallClock
	wallClock = fakeClock
	defer func() { wallClock = originalClock }()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "ns", UID: "backup-uid"},
		Status: dpv1alpha1.BackupStatus{
			FailureReason: "original backup failure",
			DeletionDiagnostic: &dpv1alpha1.BackupDeletionDiagnostic{
				ActiveJobName: "old-job", ActiveJobUID: "old-job-uid", PodUID: "old-pod-uid",
				Reason:             deletionSchedulingConflictReason,
				FirstObservedTime:  metav1.NewTime(now.Add(-time.Hour)),
				LastTransitionTime: metav1.NewTime(now.Add(-time.Hour)),
			},
		},
	}
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "delete-backup", Namespace: "ns", UID: "job-uid"}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod", Namespace: "ns", UID: "pod-uid", Labels: map[string]string{"job-name": job.Name},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: batchv1.SchemeGroupVersion.String(), Kind: "Job", Name: job.Name, UID: job.UID}},
		},
		Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{
			Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: corev1.PodReasonUnschedulable,
			LastTransitionTime: metav1.NewTime(now.Add(-2 * time.Minute)),
		}}},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&dpv1alpha1.Backup{}).
		WithObjects(backup, job, pod).Build()
	reconciler := &BackupReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(1)}

	result, err := reconciler.reconcileDeleteJobSchedulingDiagnostic(
		intctrlutil.RequestCtx{Ctx: context.Background()}, backup.DeepCopy(), job.DeepCopy())
	require.NoError(t, err)
	assert.Equal(t, 3*time.Minute, result.RequeueAfter)
	got := &dpv1alpha1.Backup{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(backup), got))
	assert.Nil(t, got.Status.DeletionDiagnostic)
	assert.Equal(t, "original backup failure", got.Status.FailureReason)
}

func TestTruncateDeletionDiagnosticMessage(t *testing.T) {
	message := strings.Repeat("\u00e9", deletionDiagnosticMessageLimit+1)
	truncated := truncateDeletionDiagnosticMessage(message)
	assert.Len(t, []rune(truncated), deletionDiagnosticMessageLimit)
	assert.Equal(t, strings.Repeat("\u00e9", deletionDiagnosticMessageLimit), truncated)
}
