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
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestClusterRestoreControllerAddsProtectionForRestoreIntent(t *testing.T) {
	scheme := clusterRestoreTestScheme(t)
	cluster := activeRestoreCluster()
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
				t.Fatal("initial restore protection must not depend on resource listing")
				return nil
			},
		}).Build()
	reconciler := &ClusterRestoreReconciler{Client: cli}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
	require.NoError(t, err)
	current := &appsv1.Cluster{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(cluster), current))
	require.Contains(t, current.Finalizers, dptypes.RestoreProtectionFinalizerName)
}

func TestClusterRestoreControllerSkipsDeletionWithoutProtection(t *testing.T) {
	scheme := clusterRestoreTestScheme(t)
	cluster := activeRestoreCluster()
	now := metav1.Now()
	cluster.DeletionTimestamp = &now
	cluster.Finalizers = []string{"example.io/app-owner"}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
				t.Fatal("deletion without DP protection must not inspect restore resources")
				return nil
			},
		}).Build()
	reconciler := &ClusterRestoreReconciler{Client: cli}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
	require.NoError(t, err)
	require.Zero(t, result)
	current := &appsv1.Cluster{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(cluster), current))
	require.Equal(t, cluster.Finalizers, current.Finalizers)
}

func TestClusterRestoreControllerWaitsWithoutMutatingOwnerResources(t *testing.T) {
	scheme := clusterRestoreTestScheme(t)
	cluster := activeRestoreCluster()
	now := metav1.Now()
	cluster.DeletionTimestamp = &now
	cluster.Finalizers = []string{dptypes.RestoreProtectionFinalizerName}
	target := clusterRestoreTarget(cluster, "target-uid")
	target.Finalizers = []string{dptypes.DataProtectionFinalizerName}
	helper := clusterRestoreHelper(cluster, target)
	restore := clusterExecutionRestore(cluster, target)
	restore.Finalizers = []string{"dataprotection.kubeblocks.io/restore-finalizer"}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, target, helper, restore).Build()
	reconciler := &ClusterRestoreReconciler{Client: cli}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
	require.NoError(t, err)
	require.NotZero(t, result.RequeueAfter)
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(restore), &dpv1alpha1.Restore{}))
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(helper), &corev1.PersistentVolumeClaim{}))
	currentTarget := &corev1.PersistentVolumeClaim{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(target), currentTarget))
	require.Contains(t, currentTarget.Finalizers, dptypes.DataProtectionFinalizerName)
}

func TestClusterRestoreControllerReleasesFinalizerAfterOwnersFinish(t *testing.T) {
	scheme := clusterRestoreTestScheme(t)
	cluster := activeRestoreCluster()
	now := metav1.Now()
	cluster.DeletionTimestamp = &now
	cluster.Finalizers = []string{dptypes.RestoreProtectionFinalizerName, "example.io/keep"}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
	reconciler := &ClusterRestoreReconciler{Client: cli}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
	require.NoError(t, err)
	current := &appsv1.Cluster{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(cluster), current))
	require.NotContains(t, current.Finalizers, dptypes.RestoreProtectionFinalizerName)
	require.Contains(t, current.Finalizers, "example.io/keep")
}

func TestClusterRestoreControllerReleasesProtectionAfterSuccessfulRestore(t *testing.T) {
	scheme := clusterRestoreTestScheme(t)
	cluster := activeRestoreCluster()
	cluster.Finalizers = []string{dptypes.RestoreProtectionFinalizerName, "example.io/keep"}
	cluster.Status.Conditions = []metav1.Condition{{
		Type: appsv1.ConditionTypeRestore, Status: metav1.ConditionTrue,
	}}
	target := clusterRestoreTarget(cluster, "target-uid")
	restore := clusterExecutionRestore(cluster, target)
	restore.Status.Phase = dpv1alpha1.RestorePhaseCompleted
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, target, restore).Build()
	reconciler := &ClusterRestoreReconciler{Client: cli}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
	require.NoError(t, err)
	current := &appsv1.Cluster{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(cluster), current))
	require.NotContains(t, current.Finalizers, dptypes.RestoreProtectionFinalizerName)
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(restore), &dpv1alpha1.Restore{}),
		"ClusterRestoreReconciler must leave terminal Restore objects for their resource owner to manage")
}

func TestClusterRestoreControllerKeepsProtectionAfterRestoreFailure(t *testing.T) {
	scheme := clusterRestoreTestScheme(t)
	cluster := activeRestoreCluster()
	cluster.Finalizers = []string{dptypes.RestoreProtectionFinalizerName}
	cluster.Status.Conditions = []metav1.Condition{{
		Type: appsv1.ConditionTypeRestore, Status: metav1.ConditionFalse,
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
	reconciler := &ClusterRestoreReconciler{Client: cli}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
	require.NoError(t, err)
	current := &appsv1.Cluster{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(cluster), current))
	require.Contains(t, current.Finalizers, dptypes.RestoreProtectionFinalizerName,
		"failed restore intent must remain protected until Cluster deletion")
}

func TestClusterRestoreControllerWaitsForTerminalRestoreDuringDeletion(t *testing.T) {
	scheme := clusterRestoreTestScheme(t)
	cluster := activeRestoreCluster()
	now := metav1.Now()
	cluster.DeletionTimestamp = &now
	cluster.Finalizers = []string{dptypes.RestoreProtectionFinalizerName}
	target := clusterRestoreTarget(cluster, "target-uid")
	restore := clusterExecutionRestore(cluster, target)
	restore.Status.Phase = dpv1alpha1.RestorePhaseFailed
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, target, restore).Build()
	reconciler := &ClusterRestoreReconciler{Client: cli}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
	require.NoError(t, err)
	require.NotZero(t, result.RequeueAfter)
	current := &appsv1.Cluster{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(cluster), current))
	require.Contains(t, current.Finalizers, dptypes.RestoreProtectionFinalizerName)
}

func TestClusterRestoreControllerIgnoresResourcesWithoutExactClusterUID(t *testing.T) {
	scheme := clusterRestoreTestScheme(t)
	cluster := activeRestoreCluster()
	now := metav1.Now()
	cluster.DeletionTimestamp = &now
	cluster.Finalizers = []string{dptypes.RestoreProtectionFinalizerName, "example.io/keep"}
	target := clusterRestoreTarget(cluster, "target-uid")
	target.Finalizers = []string{dptypes.DataProtectionFinalizerName}
	delete(target.Labels, dptypes.ClusterUIDLabelKey)
	helper := clusterRestoreHelper(cluster, target)
	delete(helper.Labels, dptypes.ClusterUIDLabelKey)
	restore := clusterExecutionRestore(cluster, target)
	delete(restore.Labels, dptypes.ClusterUIDLabelKey)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, target, helper, restore).Build()
	reconciler := &ClusterRestoreReconciler{Client: cli}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
	require.NoError(t, err)
	current := &appsv1.Cluster{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(cluster), current))
	require.NotContains(t, current.Finalizers, dptypes.RestoreProtectionFinalizerName)
	require.Contains(t, current.Finalizers, "example.io/keep")
}

func clusterRestoreTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	return scheme
}

func activeRestoreCluster() *appsv1.Cluster {
	return &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cluster", UID: "cluster-uid"},
		Spec:       appsv1.ClusterSpec{Restore: &appsv1.ClusterRestore{}},
	}
}

func clusterRestoreTarget(cluster *appsv1.Cluster, uid string) *corev1.PersistentVolumeClaim {
	apiGroup := dptypes.DataprotectionAPIGroup
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace, Name: "data-0", UID: types.UID(uid),
			Labels: map[string]string{
				constant.AppInstanceLabelKey: cluster.Name,
				dptypes.ClusterUIDLabelKey:   string(cluster.UID),
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{DataSourceRef: &corev1.TypedObjectReference{
			APIGroup: &apiGroup, Kind: dptypes.BackupKind, Name: "backup",
		}},
	}
}

func clusterRestoreHelper(cluster *appsv1.Cluster, target *corev1.PersistentVolumeClaim) *corev1.PersistentVolumeClaim {
	name := getPopulatePVCName(target.UID)
	return &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Namespace: cluster.Namespace, Name: name,
		Labels: map[string]string{
			constant.AppInstanceLabelKey:                cluster.Name,
			dptypes.ClusterUIDLabelKey:                  string(cluster.UID),
			dprestore.DataProtectionPopulatePVCLabelKey: name,
		},
	}}
}

func clusterExecutionRestore(cluster *appsv1.Cluster, target *corev1.PersistentVolumeClaim) *dpv1alpha1.Restore {
	name := getPopulatePVCName(target.UID)
	return &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{
		Namespace: cluster.Namespace, Name: name,
		Labels: map[string]string{
			constant.AppInstanceLabelKey:            cluster.Name,
			dptypes.ClusterUIDLabelKey:              string(cluster.UID),
			dprestore.DataProtectionRestoreLabelKey: name,
		},
	}}
}
