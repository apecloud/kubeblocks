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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

// ClusterRestoreReconciler protects a Cluster while its initial restore owns
// running resources and synchronizes those resources when deletion starts.
type ClusterRestoreReconciler struct {
	client.Client
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=restores,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;delete;patch;update
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/finalizers,verbs=update;patch

func (r *ClusterRestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("cluster-restore", req.NamespacedName),
		Recorder: r.Recorder,
	}
	cluster := &appsv1.Cluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if cluster.DeletionTimestamp.IsZero() {
		active := clusterRestoreConditionActive(cluster)
		if !active && cluster.Spec.Restore != nil {
			var err error
			active, err = r.hasActiveClusterRestores(reqCtx, cluster)
			if err != nil {
				return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to inspect cluster restore executions")
			}
		}
		if active {
			return r.ensureFinalizer(reqCtx, cluster)
		}
		return r.removeFinalizer(reqCtx, cluster)
	}
	if !controllerutil.ContainsFinalizer(cluster, dptypes.RestoreProtectionFinalizerName) {
		return intctrlutil.Reconciled()
	}

	pending, err := r.deleteClusterRestores(reqCtx, cluster)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to stop cluster restore executions")
	}
	if pending {
		return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "waiting for cluster restore executions to stop")
	}
	pending, err = r.releaseClusterRestorePVCs(reqCtx, cluster)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to release cluster restore PVCs")
	}
	if pending {
		return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "waiting for cluster restore PVC cleanup")
	}
	return r.removeFinalizer(reqCtx, cluster)
}

func (r *ClusterRestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		Named("cluster_restore").
		For(&appsv1.Cluster{}).
		Watches(&dpv1alpha1.Restore{}, handler.EnqueueRequestsFromMapFunc(r.mapObjectToCluster)).
		Watches(&corev1.PersistentVolumeClaim{}, handler.EnqueueRequestsFromMapFunc(r.mapObjectToCluster)).
		Complete(r)
}

func (r *ClusterRestoreReconciler) mapObjectToCluster(_ context.Context, obj client.Object) []reconcile.Request {
	clusterName := obj.GetLabels()[constant.AppInstanceLabelKey]
	if clusterName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Namespace: obj.GetNamespace(), Name: clusterName}}}
}

func clusterRestoreConditionActive(cluster *appsv1.Cluster) bool {
	if cluster.Spec.Restore == nil {
		return false
	}
	condition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeRestore)
	return condition == nil || condition.Status == metav1.ConditionUnknown
}

func (r *ClusterRestoreReconciler) hasActiveClusterRestores(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster) (bool, error) {
	list := &dpv1alpha1.RestoreList{}
	if err := r.Client.List(reqCtx.Ctx, list, client.InNamespace(cluster.Namespace),
		client.MatchingLabels{constant.AppInstanceLabelKey: cluster.Name}); err != nil {
		return false, err
	}
	for i := range list.Items {
		restore := &list.Items[i]
		if restore.Labels[dprestore.DataProtectionRestoreLabelKey] == "" || !restore.DeletionTimestamp.IsZero() {
			continue
		}
		if restore.Status.Phase != dpv1alpha1.RestorePhaseCompleted && restore.Status.Phase != dpv1alpha1.RestorePhaseFailed {
			return true, nil
		}
	}
	return false, nil
}

func (r *ClusterRestoreReconciler) ensureFinalizer(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(cluster, dptypes.RestoreProtectionFinalizerName) {
		return intctrlutil.Reconciled()
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	controllerutil.AddFinalizer(cluster, dptypes.RestoreProtectionFinalizerName)
	if err := r.Client.Patch(reqCtx.Ctx, cluster, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to add cluster restore finalizer")
	}
	return intctrlutil.Reconciled()
}

func (r *ClusterRestoreReconciler) removeFinalizer(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(cluster, dptypes.RestoreProtectionFinalizerName) {
		return intctrlutil.Reconciled()
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	controllerutil.RemoveFinalizer(cluster, dptypes.RestoreProtectionFinalizerName)
	if err := r.Client.Patch(reqCtx.Ctx, cluster, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to remove cluster restore finalizer")
	}
	return intctrlutil.Reconciled()
}

func (r *ClusterRestoreReconciler) deleteClusterRestores(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster) (bool, error) {
	list := &dpv1alpha1.RestoreList{}
	if err := r.Client.List(reqCtx.Ctx, list, client.InNamespace(cluster.Namespace),
		client.MatchingLabels{constant.AppInstanceLabelKey: cluster.Name}); err != nil {
		return false, err
	}
	pending := false
	for i := range list.Items {
		restore := &list.Items[i]
		if restore.Labels[dprestore.DataProtectionRestoreLabelKey] == "" {
			continue
		}
		pending = true
		if restore.DeletionTimestamp.IsZero() {
			if err := r.Client.Delete(reqCtx.Ctx, restore); err != nil && !apierrors.IsNotFound(err) {
				return false, fmt.Errorf("delete Restore %s/%s: %w", restore.Namespace, restore.Name, err)
			}
		}
	}
	return pending, nil
}

func (r *ClusterRestoreReconciler) releaseClusterRestorePVCs(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster) (bool, error) {
	list := &corev1.PersistentVolumeClaimList{}
	if err := r.Client.List(reqCtx.Ctx, list, client.InNamespace(cluster.Namespace),
		client.MatchingLabels{constant.AppInstanceLabelKey: cluster.Name}); err != nil {
		return false, err
	}
	pending := false
	for i := range list.Items {
		pvc := &list.Items[i]
		if pvc.Spec.DataSourceRef == nil || pvc.Spec.DataSourceRef.APIGroup == nil ||
			*pvc.Spec.DataSourceRef.APIGroup != dptypes.DataprotectionAPIGroup {
			continue
		}
		helper := &corev1.PersistentVolumeClaim{}
		helperKey := client.ObjectKey{Namespace: pvc.Namespace, Name: getPopulatePVCName(pvc.UID)}
		if err := r.Client.Get(reqCtx.Ctx, helperKey, helper); err == nil {
			pending = true
			if helper.DeletionTimestamp.IsZero() {
				if err := r.Client.Delete(reqCtx.Ctx, helper); err != nil && !apierrors.IsNotFound(err) {
					return false, fmt.Errorf("delete helper PVC %s/%s: %w", helperKey.Namespace, helperKey.Name, err)
				}
			}
		} else if !apierrors.IsNotFound(err) {
			return false, err
		}
		if controllerutil.ContainsFinalizer(pvc, dptypes.DataProtectionFinalizerName) {
			pending = true
			patch := client.MergeFrom(pvc.DeepCopy())
			controllerutil.RemoveFinalizer(pvc, dptypes.DataProtectionFinalizerName)
			if err := r.Client.Patch(reqCtx.Ctx, pvc, patch); err != nil && !apierrors.IsNotFound(err) {
				return false, fmt.Errorf("release target PVC %s/%s: %w", pvc.Namespace, pvc.Name, err)
			}
		}
	}
	return pending, nil
}
