/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package backup

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	ctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
)

func deleterTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = batchv1.AddToScheme(s)
	_ = dpv1alpha1.AddToScheme(s)
	_ = vsv1.AddToScheme(s)
	return s
}

func newDeleter(cli client.Client, scheme *runtime.Scheme) *Deleter {
	return &Deleter{
		RequestCtx: ctrlutil.RequestCtx{
			Ctx: context.Background(),
			Log: logr.Discard(),
		},
		Client: cli,
		Scheme: scheme,
	}
}

// --- BuildDeleteBackupFilesJobKey ---

func TestBuildDeleteBackupFilesJobKey_Normal(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "ns1",
			UID:       types.UID("abcdefgh-1234-5678-9abc-def012345678"),
		},
	}
	key := BuildDeleteBackupFilesJobKey(backup, false)
	assert.Equal(t, "ns1", key.Namespace)
	assert.Equal(t, "abcdefgh-delete-bk-1", key.Name)
}

func TestBuildDeleteBackupFilesJobKey_PreDelete(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "ns1",
			UID:       types.UID("abcdefgh-1234-5678-9abc-def012345678"),
		},
	}
	key := BuildDeleteBackupFilesJobKey(backup, true)
	assert.Equal(t, "abcdefgh-predelete-bk-1", key.Name)
}

func TestBuildDeleteBackupFilesJobKey_Truncated(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "very-long-backup-name-that-exceeds-the-maximum-label-length-limit",
			Namespace: "ns1",
			UID:       types.UID("abcdefgh-1234"),
		},
	}
	key := BuildDeleteBackupFilesJobKey(backup, false)
	assert.LessOrEqual(t, len(key.Name), 63)
	assert.NotEqual(t, byte('-'), key.Name[len(key.Name)-1])
}

// --- buildDeleteBackupFilesScript ---

func TestBuildDeleteBackupFilesScript(t *testing.T) {
	d := &Deleter{}
	script := d.buildDeleteBackupFilesScript("/repo/ns1/prefix/bk-1")
	assert.Contains(t, script, `targetPath="/repo/ns1/prefix/bk-1"`)
	assert.Contains(t, script, "datasafed rm -r")
	assert.Contains(t, script, "rmdirs")
	assert.Contains(t, script, dptypes.DPDatasafedBinPath)
}

// --- DeleteBackupFiles ---

func TestDeleteBackupFiles_SnapshotVolumes(t *testing.T) {
	scheme := deleterTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := newDeleter(cli, scheme)

	trueVal := true
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
		Status: dpv1alpha1.BackupStatus{
			BackupMethod: &dpv1alpha1.BackupMethod{
				SnapshotVolumes: &trueVal,
			},
		},
	}

	status, err := d.DeleteBackupFiles(backup)
	require.NoError(t, err)
	assert.Equal(t, DeletionStatusSucceeded, status)
}

func TestDeleteBackupFiles_ExistingJobCompleted(t *testing.T) {
	scheme := deleterTestScheme()
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
	}
	jobKey := BuildDeleteBackupFilesJobKey(backup, false)
	existingJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobKey.Name,
			Namespace: jobKey.Namespace,
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingJob).Build()
	d := newDeleter(cli, scheme)

	status, err := d.DeleteBackupFiles(backup)
	require.NoError(t, err)
	assert.Equal(t, DeletionStatusSucceeded, status)
}

func TestDeleteBackupFiles_ExistingJobFailed(t *testing.T) {
	scheme := deleterTestScheme()
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
	}
	jobKey := BuildDeleteBackupFilesJobKey(backup, false)
	existingJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobKey.Name,
			Namespace: jobKey.Namespace,
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Message: "oops"},
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingJob).Build()
	d := newDeleter(cli, scheme)

	status, err := d.DeleteBackupFiles(backup)
	assert.Equal(t, DeletionStatusFailed, status)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed")
}

func TestDeleteBackupFiles_ExistingJobRunning(t *testing.T) {
	scheme := deleterTestScheme()
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
	}
	jobKey := BuildDeleteBackupFilesJobKey(backup, false)
	existingJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobKey.Name,
			Namespace: jobKey.Namespace,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingJob).Build()
	d := newDeleter(cli, scheme)

	status, err := d.DeleteBackupFiles(backup)
	require.NoError(t, err)
	assert.Equal(t, DeletionStatusDeleting, status)
}

func TestDeleteBackupFiles_EmptyPath(t *testing.T) {
	scheme := deleterTestScheme()

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
		Status: dpv1alpha1.BackupStatus{
			PersistentVolumeClaimName: "some-pvc",
			Path:                      "",
		},
	}

	// PVC exists so we don't skip early for missing PVC — but path is empty → skip
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "some-pvc", Namespace: "default"},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc).Build()
	d := newDeleter(cli, scheme)

	status, err := d.DeleteBackupFiles(backup)
	require.NoError(t, err)
	assert.Equal(t, DeletionStatusSucceeded, status)
}

func TestDeleteBackupFiles_PathWithoutBackupName(t *testing.T) {
	scheme := deleterTestScheme()
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "some-pvc", Namespace: "default"},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc).Build()
	d := newDeleter(cli, scheme)

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
		Status: dpv1alpha1.BackupStatus{
			PersistentVolumeClaimName: "some-pvc",
			Path:                      "/some/other/path",
		},
	}

	status, err := d.DeleteBackupFiles(backup)
	require.NoError(t, err)
	assert.Equal(t, DeletionStatusSucceeded, status)
}

func TestDeleteBackupFiles_NoPVC_NoRepo(t *testing.T) {
	scheme := deleterTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := newDeleter(cli, scheme)

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
		Status: dpv1alpha1.BackupStatus{
			PersistentVolumeClaimName: "",
		},
	}

	status, err := d.DeleteBackupFiles(backup)
	require.NoError(t, err)
	assert.Equal(t, DeletionStatusSucceeded, status)
}

func TestDeleteBackupFiles_PVCNotFound(t *testing.T) {
	scheme := deleterTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := newDeleter(cli, scheme)

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
		Status: dpv1alpha1.BackupStatus{
			PersistentVolumeClaimName: "missing-pvc",
		},
	}

	status, err := d.DeleteBackupFiles(backup)
	require.NoError(t, err)
	assert.Equal(t, DeletionStatusSucceeded, status)
}

func TestDeleteBackupFiles_BackupRepoNotFound(t *testing.T) {
	scheme := deleterTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := newDeleter(cli, scheme)

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
		Status: dpv1alpha1.BackupStatus{
			BackupRepoName: "missing-repo",
		},
	}

	status, err := d.DeleteBackupFiles(backup)
	require.NoError(t, err)
	assert.Equal(t, DeletionStatusSucceeded, status)
}

// --- DeleteVolumeSnapshots ---

func TestDeleteVolumeSnapshots_NoSnapshots(t *testing.T) {
	scheme := deleterTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := newDeleter(cli, scheme)

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "bk-1", Namespace: "default"},
	}

	err := d.DeleteVolumeSnapshots(backup)
	assert.NoError(t, err)
}

func TestDeleteVolumeSnapshots_DeletesMatchingSnapshots(t *testing.T) {
	scheme := deleterTestScheme()
	snap := &vsv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "snap-1",
			Namespace: "default",
			Labels: map[string]string{
				dptypes.BackupNameLabelKey: "bk-1",
			},
			Finalizers: []string{dptypes.DataProtectionFinalizerName},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(snap).Build()
	d := newDeleter(cli, scheme)

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "bk-1", Namespace: "default"},
	}

	err := d.DeleteVolumeSnapshots(backup)
	require.NoError(t, err)

	// verify snapshot was deleted
	list := &vsv1.VolumeSnapshotList{}
	err = cli.List(context.Background(), list, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Empty(t, list.Items)
}

func TestDeleteVolumeSnapshots_WithFinalizerAndNoDeletionTimestamp(t *testing.T) {
	scheme := deleterTestScheme()
	snap := &vsv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "snap-1",
			Namespace: "default",
			Labels: map[string]string{
				dptypes.BackupNameLabelKey: "bk-1",
			},
			Finalizers: []string{dptypes.DataProtectionFinalizerName, "other-finalizer"},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(snap).Build()
	d := newDeleter(cli, scheme)

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "bk-1", Namespace: "default"},
	}

	err := d.DeleteVolumeSnapshots(backup)
	require.NoError(t, err)
}

// --- getPreDeleteAction ---

func TestGetPreDeleteAction_NilMethod(t *testing.T) {
	d := &Deleter{}
	action, err := d.getPreDeleteAction(nil)
	require.NoError(t, err)
	assert.Nil(t, action)
}

func TestGetPreDeleteAction_EmptyActionSetName(t *testing.T) {
	d := &Deleter{}
	method := &dpv1alpha1.BackupMethod{ActionSetName: ""}
	action, err := d.getPreDeleteAction(method)
	require.NoError(t, err)
	assert.Nil(t, action)
}

// --- createDeleteBackupFilesJob ---

func TestCreateDeleteBackupFilesJob(t *testing.T) {
	scheme := deleterTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := newDeleter(cli, scheme)

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
		Status: dpv1alpha1.BackupStatus{
			Path:                      "/repo/ns1/prefix/bk-1",
			PersistentVolumeClaimName: "repo-pvc",
		},
	}

	jobKey := BuildDeleteBackupFilesJobKey(backup, false)
	err := d.createDeleteBackupFilesJob(jobKey, backup, nil, "repo-pvc")
	require.NoError(t, err)

	// verify job was created
	job := &batchv1.Job{}
	err = cli.Get(context.Background(), client.ObjectKey{Namespace: jobKey.Namespace, Name: jobKey.Name}, job)
	require.NoError(t, err)
	assert.Equal(t, jobKey.Name, job.Name)
	require.Len(t, job.Spec.Template.Spec.Containers, 1)
	assert.Equal(t, deleteContainerName, job.Spec.Template.Spec.Containers[0].Name)
}

func TestCreateDeleteBackupFilesJob_WithBackupRepo(t *testing.T) {
	scheme := deleterTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := newDeleter(cli, scheme)

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
		Status: dpv1alpha1.BackupStatus{
			Path: "/repo/ns1/prefix/bk-1",
		},
	}

	repo := &dpv1alpha1.BackupRepo{
		Status: dpv1alpha1.BackupRepoStatus{
			BackupPVCName: "repo-pvc",
		},
	}

	jobKey := BuildDeleteBackupFilesJobKey(backup, false)
	err := d.createDeleteBackupFilesJob(jobKey, backup, repo, "")
	require.NoError(t, err)

	job := &batchv1.Job{}
	err = cli.Get(context.Background(), client.ObjectKey{Namespace: jobKey.Namespace, Name: jobKey.Name}, job)
	require.NoError(t, err)
}

// --- DeletionStatus constants ---

func TestDeletionStatusValues(t *testing.T) {
	assert.Equal(t, DeletionStatus("Deleting"), DeletionStatusDeleting)
	assert.Equal(t, DeletionStatus("Failed"), DeletionStatusFailed)
	assert.Equal(t, DeletionStatus("Succeeded"), DeletionStatusSucceeded)
	assert.Equal(t, DeletionStatus("Unknown"), DeletionStatusUnknown)
}

// --- DeleteBackupFiles full path with valid backup file path ---

func TestDeleteBackupFiles_CreatesJob(t *testing.T) {
	scheme := deleterTestScheme()
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data-pvc", Namespace: "default"},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc).Build()
	d := newDeleter(cli, scheme)

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
		Status: dpv1alpha1.BackupStatus{
			PersistentVolumeClaimName: "data-pvc",
			Path:                      "/repo/ns1/prefix/bk-1",
		},
	}

	status, err := d.DeleteBackupFiles(backup)
	require.NoError(t, err)
	assert.Equal(t, DeletionStatusDeleting, status)

	// verify job was created
	jobKey := BuildDeleteBackupFilesJobKey(backup, false)
	job := &batchv1.Job{}
	err = cli.Get(context.Background(), client.ObjectKey{Namespace: jobKey.Namespace, Name: jobKey.Name}, job)
	require.NoError(t, err)
}

func TestDeleteBackupFiles_NoLeadingSlash(t *testing.T) {
	scheme := deleterTestScheme()
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data-pvc", Namespace: "default"},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc).Build()
	d := newDeleter(cli, scheme)

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
		Status: dpv1alpha1.BackupStatus{
			PersistentVolumeClaimName: "data-pvc",
			Path:                      "repo/ns1/prefix/bk-1", // no leading slash
		},
	}

	status, err := d.DeleteBackupFiles(backup)
	require.NoError(t, err)
	assert.Equal(t, DeletionStatusDeleting, status)
}

func TestDeleteBackupFiles_WithBackupRepo(t *testing.T) {
	scheme := deleterTestScheme()
	repo := &dpv1alpha1.BackupRepo{
		ObjectMeta: metav1.ObjectMeta{Name: "my-repo"},
		Status: dpv1alpha1.BackupRepoStatus{
			BackupPVCName: "repo-pvc",
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(repo).Build()
	d := newDeleter(cli, scheme)

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
		Status: dpv1alpha1.BackupStatus{
			BackupRepoName: "my-repo",
			Path:            "/repo/ns1/prefix/bk-1",
		},
	}

	status, err := d.DeleteBackupFiles(backup)
	require.NoError(t, err)
	assert.Equal(t, DeletionStatusDeleting, status)
}

// --- SnapshotVolumes helper edge cases ---

func TestDeleteBackupFiles_SnapshotVolumesSetToFalse(t *testing.T) {
	scheme := deleterTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := newDeleter(cli, scheme)

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
		Status: dpv1alpha1.BackupStatus{
			BackupMethod: &dpv1alpha1.BackupMethod{
				SnapshotVolumes: boolptr.False(),
			},
			PersistentVolumeClaimName: "",
		},
	}

	// SnapshotVolumes=false, no PVC, no repo → should skip
	status, err := d.DeleteBackupFiles(backup)
	require.NoError(t, err)
	assert.Equal(t, DeletionStatusSucceeded, status)
}

// --- getPreDeleteAction with valid ActionSet ---

func TestGetPreDeleteAction_WithActionSet(t *testing.T) {
	scheme := deleterTestScheme()
	actionSet := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "my-actionset"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeFull,
			Backup: &dpv1alpha1.BackupActionSpec{
				PreDeleteBackup: &dpv1alpha1.BaseJobActionSpec{
					Image:   "cleanup:latest",
					Command: []string{"sh", "-c", "cleanup"},
				},
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actionSet).Build()
	d := newDeleter(cli, scheme)

	method := &dpv1alpha1.BackupMethod{ActionSetName: "my-actionset"}
	action, err := d.getPreDeleteAction(method)
	require.NoError(t, err)
	require.NotNil(t, action)
	assert.Equal(t, "cleanup:latest", action.Image)
	assert.Equal(t, []string{"sh", "-c", "cleanup"}, action.Command)
}

func TestGetPreDeleteAction_NoPreDelete(t *testing.T) {
	scheme := deleterTestScheme()
	actionSet := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "my-actionset"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeFull,
			Backup: &dpv1alpha1.BackupActionSpec{
				PreDeleteBackup: nil,
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actionSet).Build()
	d := newDeleter(cli, scheme)

	method := &dpv1alpha1.BackupMethod{ActionSetName: "my-actionset"}
	action, err := d.getPreDeleteAction(method)
	require.NoError(t, err)
	assert.Nil(t, action)
}

// --- doPreDeleteAction ---

func TestDoPreDeleteAction_CreatesJob(t *testing.T) {
	scheme := deleterTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := newDeleter(cli, scheme)

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
		Status: dpv1alpha1.BackupStatus{
			Path: "/repo/ns1/prefix/bk-1",
		},
	}

	preDeleteAction := &dpv1alpha1.BaseJobActionSpec{
		Image:   "cleanup:latest",
		Command: []string{"sh", "-c", "cleanup"},
	}

	job, err := d.doPreDeleteAction(backup, nil, preDeleteAction, "some-pvc", "/repo/ns1/prefix/bk-1")
	require.NoError(t, err)
	require.NotNil(t, job)

	// verify job was created
	preJobKey := BuildDeleteBackupFilesJobKey(backup, true)
	created := &batchv1.Job{}
	err = cli.Get(context.Background(), client.ObjectKey{Name: preJobKey.Name, Namespace: preJobKey.Namespace}, created)
	require.NoError(t, err)
	assert.Equal(t, deleteContainerName, created.Spec.Template.Spec.Containers[0].Name)
}

func TestDoPreDeleteAction_ExistingJobReturnsIt(t *testing.T) {
	scheme := deleterTestScheme()
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
	}
	preJobKey := BuildDeleteBackupFilesJobKey(backup, true)
	existingJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      preJobKey.Name,
			Namespace: preJobKey.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers:    []corev1.Container{{Name: "c", Image: "img"}},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingJob).Build()
	d := newDeleter(cli, scheme)

	preDeleteAction := &dpv1alpha1.BaseJobActionSpec{
		Image:   "cleanup:latest",
		Command: []string{"sh", "-c", "cleanup"},
	}

	job, err := d.doPreDeleteAction(backup, nil, preDeleteAction, "some-pvc", "/path")
	require.NoError(t, err)
	assert.Equal(t, preJobKey.Name, job.Name)
}

func TestDoPreDeleteAction_WithActionSetEnv(t *testing.T) {
	scheme := deleterTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := newDeleter(cli, scheme)
	d.actionSet = &dpv1alpha1.ActionSet{
		Spec: dpv1alpha1.ActionSetSpec{
			Env: []corev1.EnvVar{{Name: "AS_VAR", Value: "as_val"}},
		},
	}

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234"),
		},
		Status: dpv1alpha1.BackupStatus{
			BackupMethod: &dpv1alpha1.BackupMethod{
				Env: []corev1.EnvVar{{Name: "BM_VAR", Value: "bm_val"}},
			},
		},
	}

	preDeleteAction := &dpv1alpha1.BaseJobActionSpec{
		Image:   "cleanup:latest",
		Command: []string{"cleanup"},
	}

	job, err := d.doPreDeleteAction(backup, nil, preDeleteAction, "pvc", "/path/bk-1")
	require.NoError(t, err)
	require.NotNil(t, job)

	preJobKey := BuildDeleteBackupFilesJobKey(backup, true)
	created := &batchv1.Job{}
	err = cli.Get(context.Background(), client.ObjectKey{Name: preJobKey.Name, Namespace: preJobKey.Namespace}, created)
	require.NoError(t, err)
	envNames := make([]string, 0)
	for _, e := range created.Spec.Template.Spec.Containers[0].Env {
		envNames = append(envNames, e.Name)
	}
	assert.Contains(t, envNames, "AS_VAR")
	assert.Contains(t, envNames, "BM_VAR")
}
