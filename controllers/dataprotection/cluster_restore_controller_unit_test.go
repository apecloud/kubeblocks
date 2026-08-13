/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestClusterRestoreFinalizerLifecycle(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	cluster := newClusterWithActiveRestore()
	cluster.Finalizers = []string{dptypes.DataProtectionFinalizerName}
	reconciler := &ClusterRestoreReconciler{
		Client:   fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(cluster).WithObjects(cluster).Build(),
		Recorder: record.NewFakeRecorder(10),
	}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)
	current := &appsv1.Cluster{}
	require.NoError(t, reconciler.Client.Get(context.Background(), req.NamespacedName, current))
	require.True(t, controllerutil.ContainsFinalizer(current, dptypes.RestoreProtectionFinalizerName))
	require.True(t, controllerutil.ContainsFinalizer(current, dptypes.DataProtectionFinalizerName),
		"restore lifecycle must not take ownership of the backup finalizer")

	current.Status.Conditions = []metav1.Condition{{
		Type:   appsv1.ConditionTypeRestore,
		Status: metav1.ConditionTrue,
	}}
	require.NoError(t, reconciler.Client.Status().Update(context.Background(), current))
	execution := &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      "running-restore",
			Labels: map[string]string{
				constant.AppInstanceLabelKey:            cluster.Name,
				dprestore.DataProtectionRestoreLabelKey: "running-restore",
			},
		},
		Status: dpv1alpha1.RestoreStatus{Phase: dpv1alpha1.RestorePhaseRunning},
	}
	require.NoError(t, reconciler.Client.Create(context.Background(), execution))
	_, err = reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)
	require.NoError(t, reconciler.Client.Get(context.Background(), req.NamespacedName, current))
	require.True(t, controllerutil.ContainsFinalizer(current, dptypes.RestoreProtectionFinalizerName),
		"terminal aggregate condition must not release a still-running execution")
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(execution), execution))
	execution.Status.Phase = dpv1alpha1.RestorePhaseCompleted
	require.NoError(t, reconciler.Client.Update(context.Background(), execution))
	_, err = reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)
	require.NoError(t, reconciler.Client.Get(context.Background(), req.NamespacedName, current))
	require.False(t, controllerutil.ContainsFinalizer(current, dptypes.RestoreProtectionFinalizerName))
	require.True(t, controllerutil.ContainsFinalizer(current, dptypes.DataProtectionFinalizerName))
}

func TestDeletingClusterWaitsForRestoreThenReleasesPVCs(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	now := metav1.Now()
	cluster := newClusterWithActiveRestore()
	cluster.DeletionTimestamp = &now
	cluster.Finalizers = []string{dptypes.DataProtectionFinalizerName, dptypes.RestoreProtectionFinalizerName}
	apiGroup := dptypes.DataprotectionAPIGroup
	target := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  cluster.Namespace,
			Name:       "data-0",
			UID:        "target-uid",
			Labels:     map[string]string{constant.AppInstanceLabelKey: cluster.Name},
			Finalizers: []string{dptypes.DataProtectionFinalizerName},
		},
		Spec: corev1.PersistentVolumeClaimSpec{DataSourceRef: &corev1.TypedObjectReference{
			APIGroup: &apiGroup,
			Kind:     dptypes.BackupKind,
			Name:     "backup",
		}},
	}
	helper := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Namespace: cluster.Namespace,
		Name:      getPopulatePVCName(target.UID),
	}}
	restore := &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{
		Namespace: cluster.Namespace,
		Name:      "execution-restore",
		Labels: map[string]string{
			constant.AppInstanceLabelKey:            cluster.Name,
			dprestore.DataProtectionRestoreLabelKey: "execution-restore",
		},
		Finalizers: []string{"test.kubeblocks.io/hold"},
	}}
	reconciler := &ClusterRestoreReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, target, helper, restore).Build(),
	}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)}

	result, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)
	require.True(t, result.RequeueAfter > 0)
	currentCluster := &appsv1.Cluster{}
	require.NoError(t, reconciler.Client.Get(context.Background(), req.NamespacedName, currentCluster))
	require.True(t, controllerutil.ContainsFinalizer(currentCluster, dptypes.RestoreProtectionFinalizerName))
	currentRestore := &dpv1alpha1.Restore{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(restore), currentRestore))
	require.False(t, currentRestore.DeletionTimestamp.IsZero())

	currentRestore.Finalizers = nil
	require.NoError(t, reconciler.Client.Update(context.Background(), currentRestore))
	result, err = reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)
	require.True(t, result.RequeueAfter > 0)
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(target), target))
	require.False(t, controllerutil.ContainsFinalizer(target, dptypes.DataProtectionFinalizerName))
	require.Error(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(helper), &corev1.PersistentVolumeClaim{}))

	_, err = reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)
	require.NoError(t, reconciler.Client.Get(context.Background(), req.NamespacedName, currentCluster))
	require.False(t, controllerutil.ContainsFinalizer(currentCluster, dptypes.RestoreProtectionFinalizerName))
	require.True(t, controllerutil.ContainsFinalizer(currentCluster, dptypes.DataProtectionFinalizerName))
}

func newClusterWithActiveRestore() *appsv1.Cluster {
	return &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cluster", UID: types.UID("cluster-uid")},
		Spec: appsv1.ClusterSpec{Restore: &appsv1.ClusterRestore{Source: appsv1.ClusterRestoreSource{
			APIGroup: dptypes.DataprotectionAPIGroup,
			Kind:     dptypes.BackupKind,
			Name:     "backup",
		}}},
	}
}
