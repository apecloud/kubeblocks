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

package dataprotection

import (
	"context"
	"errors"
	"testing"

	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dpbackup "github.com/apecloud/kubeblocks/pkg/dataprotection/backup"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	cleanupTestNamespace  = "backup-cleanup"
	cleanupTestClusterUID = "cluster-uid"
)

type transientCleanupErrorClient struct {
	client.Client
	operation string
	remaining int
}

func (c *transientCleanupErrorClient) shouldFail(operation string) bool {
	if c.operation != operation || c.remaining == 0 {
		return false
	}
	c.remaining--
	return true
}

func (c *transientCleanupErrorClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, ok := list.(*batchv1.JobList); ok && c.shouldFail("list") {
		return errors.New("transient job list error")
	}
	return c.Client.List(ctx, list, opts...)
}

func (c *transientCleanupErrorClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if _, ok := obj.(*batchv1.Job); ok && c.shouldFail("patch") {
		return errors.New("transient job patch error")
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func (c *transientCleanupErrorClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if _, ok := obj.(*batchv1.Job); ok && c.shouldFail("delete") {
		return errors.New("transient job delete error")
	}
	return c.Client.Delete(ctx, obj, opts...)
}

func newCleanupBackup() *dpv1alpha1.Backup {
	return &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "backup",
			Namespace:  cleanupTestNamespace,
			UID:        types.UID("0123456789abcdef"),
			Finalizers: []string{dptypes.DataProtectionFinalizerName},
			Labels: map[string]string{
				constant.AppManagedByLabelKey: "fixture-owner",
				dptypes.ClusterUIDLabelKey:    cleanupTestClusterUID,
			},
		},
		Spec: dpv1alpha1.BackupSpec{DeletionPolicy: dpv1alpha1.BackupDeletionPolicyDelete},
		Status: dpv1alpha1.BackupStatus{
			Phase: dpv1alpha1.BackupPhaseDeleting,
		},
	}
}

func newCleanupJob(backup *dpv1alpha1.Backup, managedBy *string, ownerUID types.UID) *batchv1.Job {
	labels := map[string]string{
		dptypes.BackupNameLabelKey: backup.Name,
		dptypes.ClusterUIDLabelKey: cleanupTestClusterUID,
	}
	if managedBy != nil {
		labels[constant.AppManagedByLabelKey] = *managedBy
	}
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "backup-job",
			Namespace:  backup.Namespace,
			UID:        types.UID("fedcba9876543210"),
			Labels:     labels,
			Finalizers: []string{dptypes.DataProtectionFinalizerName},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: dpv1alpha1.GroupVersion.String(),
				Kind:       dptypes.BackupKind,
				Name:       backup.Name,
				UID:        ownerUID,
				Controller: pointer.Bool(true),
			}},
		},
		Status: batchv1.JobStatus{
			Succeeded:  1,
			Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: "True"}},
		},
	}
}

func newCleanupReconciler(t *testing.T, objects ...client.Object) (*BackupReconciler, client.Client) {
	t.Helper()
	runtimeScheme := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(runtimeScheme))
	require.NoError(t, dpv1alpha1.AddToScheme(runtimeScheme))
	require.NoError(t, vsv1.AddToScheme(runtimeScheme))
	cli := fake.NewClientBuilder().WithScheme(runtimeScheme).WithObjects(objects...).Build()
	return &BackupReconciler{
		Client:   cli,
		Scheme:   runtimeScheme,
		Recorder: record.NewFakeRecorder(10),
	}, cli
}

func setCleanupControllerNamespace(t *testing.T) {
	t.Helper()
	setCleanupViperValue(t, constant.CfgKeyCtrlrMgrNS, cleanupTestNamespace)
}

func setCleanupViperValue(t *testing.T, key, value string) {
	t.Helper()
	old := viper.Get(key)
	viper.Set(key, value)
	t.Cleanup(func() { viper.Set(key, old) })
}

func TestBackupExternalResourceCleanupSupportsLegacyWorkloadLabels(t *testing.T) {
	setCleanupControllerNamespace(t)
	tests := []struct {
		name      string
		managedBy *string
	}{
		{name: "wrong inherited managed-by", managedBy: pointer.String("fixture-owner")},
		{name: "missing managed-by"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backup := newCleanupBackup()
			job := newCleanupJob(backup, tt.managedBy, backup.UID)
			reconciler, cli := newCleanupReconciler(t, backup, job)

			require.NoError(t, reconciler.deleteExternalResources(newCleanupRequestContext(), backup))

			err := cli.Get(context.Background(), client.ObjectKeyFromObject(job), &batchv1.Job{})
			require.True(t, apierrors.IsNotFound(err), "owned backup job should be deleted")
		})
	}
}

func TestBackupExternalResourceCleanupLeavesNonOwnerJobUntouched(t *testing.T) {
	setCleanupControllerNamespace(t)
	backup := newCleanupBackup()
	job := newCleanupJob(backup, pointer.String(dptypes.AppName), types.UID("another-backup"))
	reconciler, cli := newCleanupReconciler(t, backup, job)

	require.NoError(t, reconciler.deleteExternalResources(newCleanupRequestContext(), backup))

	fetched := &batchv1.Job{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(job), fetched))
	require.Equal(t, []string{dptypes.DataProtectionFinalizerName}, fetched.Finalizers)
}

func TestBackupExternalResourceCleanupIsIdempotentWhenJobIsGone(t *testing.T) {
	setCleanupControllerNamespace(t)
	backup := newCleanupBackup()
	reconciler, _ := newCleanupReconciler(t, backup)

	require.NoError(t, reconciler.deleteExternalResources(newCleanupRequestContext(), backup))
	require.NoError(t, reconciler.deleteExternalResources(newCleanupRequestContext(), backup))
}

func TestBackupExternalResourceCleanupRetriesTransientErrors(t *testing.T) {
	setCleanupControllerNamespace(t)
	for _, operation := range []string{"list", "patch", "delete"} {
		t.Run(operation, func(t *testing.T) {
			backup := newCleanupBackup()
			job := newCleanupJob(backup, pointer.String(dptypes.AppName), backup.UID)
			reconciler, baseClient := newCleanupReconciler(t, backup, job)
			failingClient := &transientCleanupErrorClient{
				Client:    baseClient,
				operation: operation,
				remaining: 1,
			}
			reconciler.Client = failingClient

			require.Error(t, reconciler.deleteExternalResources(newCleanupRequestContext(), backup))
			require.NoError(t, reconciler.deleteExternalResources(newCleanupRequestContext(), backup))

			err := baseClient.Get(context.Background(), client.ObjectKeyFromObject(job), &batchv1.Job{})
			require.True(t, apierrors.IsNotFound(err), "retry should delete the owned backup job")
		})
	}
}

func TestBackupDeletingReconcileRemovesFinalizerAndJob(t *testing.T) {
	setCleanupControllerNamespace(t)
	setCleanupViperValue(t, dptypes.CfgKeyWorkerServiceAccountName, "worker")
	setCleanupViperValue(t, dptypes.CfgKeyWorkerClusterRoleName, "worker-role")
	backup := newCleanupBackup()
	deletionTimestamp := metav1.Now()
	backup.DeletionTimestamp = &deletionTimestamp
	jobLabels := dpbackup.BuildBackupWorkloadLabels(backup)
	job := newCleanupJob(backup, pointer.String(jobLabels[constant.AppManagedByLabelKey]), backup.UID)
	reconciler, cli := newCleanupReconciler(t, backup, job)

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(backup)})
	require.NoError(t, err)

	err = cli.Get(context.Background(), client.ObjectKeyFromObject(backup), &dpv1alpha1.Backup{})
	require.True(t, apierrors.IsNotFound(err), "backup should disappear after its finalizer is removed")
	err = cli.Get(context.Background(), client.ObjectKeyFromObject(job), &batchv1.Job{})
	require.True(t, apierrors.IsNotFound(err), "normal deleting reconciliation should delete the owned job")
}

func newCleanupRequestContext() intctrlutil.RequestCtx {
	return intctrlutil.RequestCtx{Ctx: context.Background()}
}
