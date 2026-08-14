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
	APIReader client.Reader
	Recorder  record.EventRecorder
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
		if !active {
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
	pending, err = r.deleteClusterRestoreHelpers(reqCtx, cluster)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to delete cluster restore helper PVCs")
	}
	if pending {
		return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "waiting for cluster restore helper PVCs to disappear")
	}
	pending, err = r.releaseClusterRestorePVCs(reqCtx, cluster)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to release cluster restore PVCs")
	}
	if pending {
		return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "waiting for cluster restore PVC cleanup")
	}
	// All destructive cleanup checks above use the uncached API reader. Perform
	// one final pass before the irreversible Cluster finalizer removal so a
	// transient informer gap can never be interpreted as cleanup completion.
	pending, err = r.hasClusterRestoreCleanupResources(reqCtx.Ctx, cluster)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to confirm cluster restore cleanup")
	}
	if pending {
		return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "waiting for API server to confirm cluster restore cleanup")
	}
	return r.removeFinalizer(reqCtx, cluster)
}

func (r *ClusterRestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.APIReader == nil {
		r.APIReader = mgr.GetAPIReader()
	}
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
	restores, err := r.listClusterRestores(reqCtx.Ctx, cluster)
	if err != nil {
		return false, err
	}
	for _, restore := range restores {
		if restore.Labels[dprestore.DataProtectionRestoreLabelKey] == "" {
			continue
		}
		// Failed and deleting Restores still own retained restore resources. Only a
		// completed, non-deleting execution is inactive.
		if !restore.DeletionTimestamp.IsZero() || restore.Status.Phase != dpv1alpha1.RestorePhaseCompleted {
			return true, nil
		}
	}
	pvcs, err := r.listClusterPVCs(reqCtx.Ctx, cluster)
	if err != nil {
		return false, err
	}
	for _, pvc := range pvcs {
		if isClusterRestoreHelperPVC(pvc) ||
			(isClusterRestoreTargetPVC(pvc) && controllerutil.ContainsFinalizer(pvc, dptypes.DataProtectionFinalizerName)) {
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
	restores, err := r.listClusterRestores(reqCtx.Ctx, cluster)
	if err != nil {
		return false, err
	}
	pending := false
	for _, restore := range restores {
		if restore.Labels[dprestore.DataProtectionRestoreLabelKey] == "" {
			continue
		}
		owned, err := r.restoreOwnedByCluster(reqCtx.Ctx, restore, cluster)
		if err != nil {
			return false, err
		}
		if !owned {
			return false, fmt.Errorf(
				"refusing to delete Restore %s/%s: labels identify Cluster %s/%s UID %s but no producer ownerReference verifies that ownership",
				restore.Namespace, restore.Name, cluster.Namespace, cluster.Name, cluster.UID)
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

// restoreOwnedByCluster verifies the producer-written ownership chain before a
// label-selected Restore is deleted. Labels route reconciliation, but are not
// sufficient authority for destructive cleanup.
func (r *ClusterRestoreReconciler) restoreOwnedByCluster(ctx context.Context,
	restore *dpv1alpha1.Restore,
	cluster *appsv1.Cluster) (bool, error) {
	for _, owner := range restore.OwnerReferences {
		if owner.APIVersion == appsv1.GroupVersion.String() && owner.Kind == appsv1.ClusterKind &&
			owner.Name == cluster.Name && owner.UID == cluster.UID {
			return true, nil
		}
		switch owner.Kind {
		case "PersistentVolumeClaim":
			if owner.APIVersion != corev1.SchemeGroupVersion.String() {
				continue
			}
			pvc := &corev1.PersistentVolumeClaim{}
			key := client.ObjectKey{Namespace: restore.Namespace, Name: owner.Name}
			if err := r.directReader().Get(ctx, key, pvc); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return false, err
			}
			if pvc.UID == owner.UID && pvc.Labels[dptypes.ClusterUIDLabelKey] == string(cluster.UID) &&
				isClusterRestoreTargetPVC(pvc) {
				return true, nil
			}
		case "Component":
			if owner.APIVersion != appsv1.GroupVersion.String() {
				continue
			}
			component := &appsv1.Component{}
			key := client.ObjectKey{Namespace: restore.Namespace, Name: owner.Name}
			if err := r.directReader().Get(ctx, key, component); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return false, err
			}
			if component.UID == owner.UID && component.Annotations[constant.KBAppClusterUIDKey] == string(cluster.UID) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (r *ClusterRestoreReconciler) deleteClusterRestoreHelpers(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster) (bool, error) {
	pvcs, err := r.listClusterPVCs(reqCtx.Ctx, cluster)
	if err != nil {
		return false, err
	}
	pending := false
	for _, pvc := range pvcs {
		if !isClusterRestoreHelperPVC(pvc) {
			continue
		}
		pending = true
		if pvc.DeletionTimestamp.IsZero() {
			if err := r.Client.Delete(reqCtx.Ctx, pvc); err != nil && !apierrors.IsNotFound(err) {
				return false, fmt.Errorf("delete helper PVC %s/%s: %w", pvc.Namespace, pvc.Name, err)
			}
		}
	}
	return pending, nil
}

func (r *ClusterRestoreReconciler) releaseClusterRestorePVCs(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster) (bool, error) {
	pvcs, err := r.listClusterPVCs(reqCtx.Ctx, cluster)
	if err != nil {
		return false, err
	}
	pending := false
	for _, pvc := range pvcs {
		if !isClusterRestoreTargetPVC(pvc) {
			continue
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

func (r *ClusterRestoreReconciler) directReader() client.Reader {
	if r.APIReader != nil {
		return r.APIReader
	}
	return r.Client
}

func (r *ClusterRestoreReconciler) listClusterRestores(ctx context.Context, cluster *appsv1.Cluster) ([]*dpv1alpha1.Restore, error) {
	list := &dpv1alpha1.RestoreList{}
	if err := r.directReader().List(ctx, list, client.InNamespace(cluster.Namespace), client.MatchingLabels{
		dptypes.ClusterUIDLabelKey: string(cluster.UID),
	}); err != nil {
		return nil, err
	}
	result := make([]*dpv1alpha1.Restore, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, &list.Items[i])
	}
	return result, nil
}

func (r *ClusterRestoreReconciler) listClusterPVCs(ctx context.Context, cluster *appsv1.Cluster) ([]*corev1.PersistentVolumeClaim, error) {
	list := &corev1.PersistentVolumeClaimList{}
	if err := r.directReader().List(ctx, list, client.InNamespace(cluster.Namespace), client.MatchingLabels{
		dptypes.ClusterUIDLabelKey: string(cluster.UID),
	}); err != nil {
		return nil, err
	}
	result := make([]*corev1.PersistentVolumeClaim, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, &list.Items[i])
	}
	return result, nil
}

func (r *ClusterRestoreReconciler) hasClusterRestoreCleanupResources(ctx context.Context, cluster *appsv1.Cluster) (bool, error) {
	restores, err := r.listClusterRestores(ctx, cluster)
	if err != nil {
		return false, err
	}
	for _, restore := range restores {
		if restore.Labels[dprestore.DataProtectionRestoreLabelKey] != "" {
			return true, nil
		}
	}
	pvcs, err := r.listClusterPVCs(ctx, cluster)
	if err != nil {
		return false, err
	}
	for _, pvc := range pvcs {
		if isClusterRestoreHelperPVC(pvc) ||
			(isClusterRestoreTargetPVC(pvc) && controllerutil.ContainsFinalizer(pvc, dptypes.DataProtectionFinalizerName)) {
			return true, nil
		}
	}
	return false, nil
}

func isClusterRestoreHelperPVC(pvc *corev1.PersistentVolumeClaim) bool {
	return pvc.Labels[dprestore.DataProtectionPopulatePVCLabelKey] != ""
}

func isClusterRestoreTargetPVC(pvc *corev1.PersistentVolumeClaim) bool {
	return pvc.Spec.DataSourceRef != nil && pvc.Spec.DataSourceRef.APIGroup != nil &&
		*pvc.Spec.DataSourceRef.APIGroup == dptypes.DataprotectionAPIGroup
}
