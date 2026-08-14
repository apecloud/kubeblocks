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
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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
				dptypes.ClusterUIDLabelKey:              string(cluster.UID),
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
			Namespace: cluster.Namespace,
			Name:      "data-0",
			UID:       "target-uid",
			Labels: map[string]string{
				constant.AppInstanceLabelKey: cluster.Name,
				dptypes.ClusterUIDLabelKey:   string(cluster.UID),
			},
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
		Labels: map[string]string{
			dptypes.ClusterUIDLabelKey:                       string(cluster.UID),
			dprestore.DataProtectionPopulatePVCLabelKey:      getPopulatePVCName(target.UID),
			dprestore.DataProtectionRestoreNamespaceLabelKey: target.Namespace,
		},
		Finalizers: []string{"test.kubeblocks.io/hold"},
	}}
	restore := &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{
		Namespace: cluster.Namespace,
		Name:      "execution-restore",
		Labels: map[string]string{
			constant.AppInstanceLabelKey:            cluster.Name,
			dprestore.DataProtectionRestoreLabelKey: "execution-restore",
			dptypes.ClusterUIDLabelKey:              string(cluster.UID),
		},
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
			Name:       target.Name,
			UID:        target.UID,
		}},
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
	require.True(t, controllerutil.ContainsFinalizer(target, dptypes.DataProtectionFinalizerName),
		"target must remain protected until the helper is actually gone")
	currentHelper := &corev1.PersistentVolumeClaim{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(helper), currentHelper))
	require.False(t, currentHelper.DeletionTimestamp.IsZero())

	currentHelper.Finalizers = nil
	require.NoError(t, reconciler.Client.Update(context.Background(), currentHelper))
	result, err = reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)
	require.True(t, result.RequeueAfter > 0)
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(target), target))
	require.False(t, controllerutil.ContainsFinalizer(target, dptypes.DataProtectionFinalizerName))

	_, err = reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)
	require.NoError(t, reconciler.Client.Get(context.Background(), req.NamespacedName, currentCluster))
	require.False(t, controllerutil.ContainsFinalizer(currentCluster, dptypes.RestoreProtectionFinalizerName))
	require.True(t, controllerutil.ContainsFinalizer(currentCluster, dptypes.DataProtectionFinalizerName))
}

func TestFailedAndDeletingRestoreKeepClusterProtected(t *testing.T) {
	scheme := newClusterRestoreTestScheme(t)
	for _, phase := range []dpv1alpha1.RestorePhase{
		dpv1alpha1.RestorePhaseFailed,
		dpv1alpha1.RestorePhaseRunning,
	} {
		t.Run(string(phase), func(t *testing.T) {
			cluster := newClusterWithActiveRestore()
			cluster.Status.Conditions = []metav1.Condition{{Type: appsv1.ConditionTypeRestore, Status: metav1.ConditionFalse}}
			cluster.Finalizers = []string{dptypes.RestoreProtectionFinalizerName}
			restore := newClusterExecutionRestore(cluster, phase)
			if phase == dpv1alpha1.RestorePhaseRunning {
				now := metav1.Now()
				restore.DeletionTimestamp = &now
				restore.Finalizers = []string{"test.kubeblocks.io/hold"}
			}
			reconciler := &ClusterRestoreReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).
				WithStatusSubresource(cluster).WithObjects(cluster, restore).Build()}

			_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
			require.NoError(t, err)
			current := &appsv1.Cluster{}
			require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(cluster), current))
			require.True(t, controllerutil.ContainsFinalizer(current, dptypes.RestoreProtectionFinalizerName))
		})
	}
}

func TestDeletingClusterIgnoresResourcesFromPreviousClusterUID(t *testing.T) {
	scheme := newClusterRestoreTestScheme(t)
	now := metav1.Now()
	cluster := newClusterWithActiveRestore()
	cluster.DeletionTimestamp = &now
	cluster.Finalizers = []string{dptypes.DataProtectionFinalizerName, dptypes.RestoreProtectionFinalizerName}
	stale := newClusterExecutionRestore(cluster, dpv1alpha1.RestorePhaseRunning)
	stale.Labels[dptypes.ClusterUIDLabelKey] = "previous-cluster-uid"
	reconciler := &ClusterRestoreReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, stale).Build()}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
	require.NoError(t, err)
	current := &appsv1.Cluster{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(cluster), current))
	require.False(t, controllerutil.ContainsFinalizer(current, dptypes.RestoreProtectionFinalizerName))
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(stale), &dpv1alpha1.Restore{}))
}

func TestAPIReaderResidualPreventsClusterFinalizerRelease(t *testing.T) {
	scheme := newClusterRestoreTestScheme(t)
	now := metav1.Now()
	cluster := newClusterWithActiveRestore()
	cluster.DeletionTimestamp = &now
	cluster.Finalizers = []string{dptypes.RestoreProtectionFinalizerName}
	clientWithoutResidual := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
	residual := newClusterExecutionRestore(cluster, dpv1alpha1.RestorePhaseFailed)
	residual.DeletionTimestamp = &now
	residual.Finalizers = []string{"test.kubeblocks.io/hold"}
	apiReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster.DeepCopy(), residual).Build()
	reconciler := &ClusterRestoreReconciler{Client: clientWithoutResidual, APIReader: apiReader}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
	require.NoError(t, err)
	require.True(t, result.RequeueAfter > 0)
	current := &appsv1.Cluster{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(cluster), current))
	require.True(t, controllerutil.ContainsFinalizer(current, dptypes.RestoreProtectionFinalizerName))
}

func TestDeletingClusterDoesNotDeleteLabelOnlyRestore(t *testing.T) {
	scheme := newClusterRestoreTestScheme(t)
	now := metav1.Now()
	cluster := newClusterWithActiveRestore()
	cluster.DeletionTimestamp = &now
	cluster.Finalizers = []string{dptypes.RestoreProtectionFinalizerName}
	forged := newClusterExecutionRestore(cluster, dpv1alpha1.RestorePhaseRunning)
	forged.OwnerReferences = nil
	reconciler := &ClusterRestoreReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, forged).Build(),
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
	require.Error(t, err)
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(forged), &dpv1alpha1.Restore{}))
	current := &appsv1.Cluster{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(cluster), current))
	require.True(t, controllerutil.ContainsFinalizer(current, dptypes.RestoreProtectionFinalizerName))
}

func TestLegacyRestoreResourcesAreAdoptedAfterClusterIsProtected(t *testing.T) {
	scheme := newClusterRestoreTestScheme(t)
	cluster := newClusterWithActiveRestore()
	component, its, target := newLegacyRestoreTarget(cluster)
	helper := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Namespace: target.Namespace,
		Name:      getPopulatePVCName(target.UID),
		Labels: map[string]string{
			constant.AppInstanceLabelKey:                cluster.Name,
			dprestore.DataProtectionPopulatePVCLabelKey: getPopulatePVCName(target.UID),
		},
	}}
	restore := &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{
		Namespace: target.Namespace,
		Name:      getPopulatePVCName(target.UID),
		Labels: map[string]string{
			constant.AppInstanceLabelKey:            cluster.Name,
			dprestore.DataProtectionRestoreLabelKey: getPopulatePVCName(target.UID),
		},
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: corev1.SchemeGroupVersion.String(), Kind: "PersistentVolumeClaim",
			Name: target.Name, UID: target.UID,
		}},
	}}
	reconciler := &ClusterRestoreReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(cluster, component, its, target, helper, restore).Build()}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)
	currentCluster := &appsv1.Cluster{}
	require.NoError(t, reconciler.Client.Get(context.Background(), req.NamespacedName, currentCluster))
	require.Contains(t, currentCluster.Finalizers, dptypes.RestoreProtectionFinalizerName)
	currentTarget := &corev1.PersistentVolumeClaim{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(target), currentTarget))
	require.Empty(t, currentTarget.Labels[dptypes.ClusterUIDLabelKey],
		"legacy resources must not be mutated before Cluster protection is established")

	result, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)
	require.True(t, result.RequeueAfter > 0)
	for _, object := range []client.Object{currentTarget, &corev1.PersistentVolumeClaim{}, &dpv1alpha1.Restore{}} {
		switch object := object.(type) {
		case *corev1.PersistentVolumeClaim:
			key := client.ObjectKeyFromObject(target)
			if object != currentTarget {
				key = client.ObjectKeyFromObject(helper)
			}
			require.NoError(t, reconciler.Client.Get(context.Background(), key, object))
		case *dpv1alpha1.Restore:
			require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(restore), object))
		}
		require.Equal(t, string(cluster.UID), object.GetLabels()[dptypes.ClusterUIDLabelKey])
	}
	require.Equal(t, string(cluster.UID), currentTarget.Annotations[constant.KBAppClusterUIDKey])
}

func TestUnverifiedLegacyHelperBlocksClusterFinalizerRelease(t *testing.T) {
	scheme := newClusterRestoreTestScheme(t)
	cluster := newClusterWithActiveRestore()
	now := metav1.Now()
	cluster.DeletionTimestamp = &now
	cluster.Finalizers = []string{dptypes.RestoreProtectionFinalizerName}
	helper := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Namespace: cluster.Namespace,
		Name:      "kb-populate-orphaned-target-uid",
		Labels: map[string]string{
			constant.AppInstanceLabelKey:                cluster.Name,
			dprestore.DataProtectionPopulatePVCLabelKey: "kb-populate-orphaned-target-uid",
		},
	}}
	reconciler := &ClusterRestoreReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(cluster, helper).Build()}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
	require.Error(t, err)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	require.Zero(t, result)
	currentCluster := &appsv1.Cluster{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(cluster), currentCluster))
	require.Contains(t, currentCluster.Finalizers, dptypes.RestoreProtectionFinalizerName)
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(helper), &corev1.PersistentVolumeClaim{}))
}

func newClusterRestoreTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	require.NoError(t, workloads.AddToScheme(scheme))
	return scheme
}

func newLegacyRestoreTarget(cluster *appsv1.Cluster) (*appsv1.Component, *workloads.InstanceSet, *corev1.PersistentVolumeClaim) {
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: cluster.Namespace,
		Name:      constant.GenerateClusterComponentName(cluster.Name, "mysql"),
		UID:       "component-uid",
		Labels:    map[string]string{constant.AppInstanceLabelKey: cluster.Name},
		Annotations: map[string]string{
			constant.KBAppClusterUIDKey: string(cluster.UID),
		},
	}}
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{
		Namespace: cluster.Namespace,
		Name:      component.Name,
		UID:       "instanceset-uid",
		Labels:    map[string]string{constant.AppInstanceLabelKey: cluster.Name},
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: appsv1.GroupVersion.String(), Kind: appsv1.ComponentKind,
			Name: component.Name, UID: component.UID, Controller: ptr.To(true),
		}},
	}}
	apiGroup := dptypes.DataprotectionAPIGroup
	target := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      "data-0",
			UID:       "target-uid",
			Labels:    map[string]string{constant.AppInstanceLabelKey: cluster.Name},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: workloads.GroupVersion.String(), Kind: workloads.InstanceSetKind,
				Name: its.Name, UID: its.UID, Controller: ptr.To(true),
			}},
		},
		Spec: corev1.PersistentVolumeClaimSpec{DataSourceRef: &corev1.TypedObjectReference{
			APIGroup: &apiGroup, Kind: dptypes.BackupKind, Name: "backup",
		}},
	}
	return component, its, target
}

func newClusterExecutionRestore(cluster *appsv1.Cluster, phase dpv1alpha1.RestorePhase) *dpv1alpha1.Restore {
	return &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      "execution-restore",
			Labels: map[string]string{
				constant.AppInstanceLabelKey:            cluster.Name,
				dprestore.DataProtectionRestoreLabelKey: "execution-restore",
				dptypes.ClusterUIDLabelKey:              string(cluster.UID),
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: appsv1.GroupVersion.String(),
				Kind:       appsv1.ClusterKind,
				Name:       cluster.Name,
				UID:        cluster.UID,
			}},
		},
		Status: dpv1alpha1.RestoreStatus{Phase: phase},
	}
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
