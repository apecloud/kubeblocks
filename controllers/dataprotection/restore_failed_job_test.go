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
	"testing"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestFailedRestoreCanKeepActivePrepareJob(t *testing.T) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{dpv1alpha1.AddToScheme, batchv1.AddToScheme, corev1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	restore := &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{Name: "restore", Namespace: "default", UID: "restore-uid"},
		Spec:   dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: "removed-backup", Namespace: "default"}},
		Status: dpv1alpha1.RestoreStatus{Phase: dpv1alpha1.RestorePhaseRunning}}
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "prepare", Namespace: restore.Namespace, UID: "job-uid", Finalizers: []string{dptypes.DataProtectionFinalizerName},
		OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(restore, dpv1alpha1.GroupVersion.WithKind("Restore"))}},
		Spec:   batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-target-db-0"}}}}}}},
		Status: batchv1.JobStatus{Active: 1}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(restore).WithObjects(restore, job).Build()
	reconciler := &RestoreReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(16)}
	if _, err := reconciler.handleRunningPhase(intctrlutil.RequestCtx{Ctx: context.Background(), Log: logr.Discard()}, restore); err != nil {
		t.Fatal(err)
	}
	current := &dpv1alpha1.Restore{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(restore), current); err != nil {
		t.Fatal(err)
	}
	if current.Status.Phase != dpv1alpha1.RestorePhaseFailed {
		t.Fatalf("phase=%s, want Failed", current.Status.Phase)
	}
	remaining := &batchv1.Job{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(job), remaining); err != nil {
		t.Fatal(err)
	}
	if remaining.Status.Active != 1 || !remaining.DeletionTimestamp.IsZero() {
		t.Fatal("fatal revalidation can leave the prepare Job active; cleanup cannot assume otherwise")
	}
}
