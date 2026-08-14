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
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
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

	if cluster.DeletionTimestamp.IsZero() || controllerutil.ContainsFinalizer(cluster, dptypes.RestoreProtectionFinalizerName) {
		legacyFound, adopted, err := r.adoptLegacyClusterRestoreResources(reqCtx, cluster)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to adopt legacy cluster restore resources")
		}
		if legacyFound && cluster.DeletionTimestamp.IsZero() &&
			!controllerutil.ContainsFinalizer(cluster, dptypes.RestoreProtectionFinalizerName) {
			return r.ensureFinalizer(reqCtx, cluster)
		}
		if adopted {
			return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "waiting for legacy restore ownership labels to become visible")
		}
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
			if pvc.UID == owner.UID && isClusterRestoreTargetPVC(pvc) {
				if pvc.Labels[dptypes.ClusterUIDLabelKey] == string(cluster.UID) {
					return true, nil
				}
				if pvc.Labels[dptypes.ClusterUIDLabelKey] == "" &&
					validateLegacyRestoreTargetPVC(ctx, r.directReader(), pvc, cluster) == nil {
					return true, nil
				}
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

func (r *ClusterRestoreReconciler) adoptLegacyClusterRestoreResources(reqCtx intctrlutil.RequestCtx,
	cluster *appsv1.Cluster) (bool, bool, error) {
	targetList := &corev1.PersistentVolumeClaimList{}
	if err := r.directReader().List(reqCtx.Ctx, targetList, client.InNamespace(cluster.Namespace), client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.Name,
	}); err != nil {
		return false, false, err
	}
	legacyTargets := make([]*corev1.PersistentVolumeClaim, 0)
	verifiedTargetsByHelper := map[string]*corev1.PersistentVolumeClaim{}
	for i := range targetList.Items {
		pvc := &targetList.Items[i]
		if !isClusterRestoreTargetPVC(pvc) {
			continue
		}
		clusterUID := pvc.Labels[dptypes.ClusterUIDLabelKey]
		if clusterUID == string(cluster.UID) {
			verifiedTargetsByHelper[getPopulatePVCName(pvc.UID)] = pvc
			continue
		}
		if clusterUID != "" || validateLegacyRestoreTargetPVC(reqCtx.Ctx, r.directReader(), pvc, cluster) != nil {
			continue
		}
		verifiedTargetsByHelper[getPopulatePVCName(pvc.UID)] = pvc
		legacyTargets = append(legacyTargets, pvc)
	}
	legacyHelpers := make([]*corev1.PersistentVolumeClaim, 0)
	unverifiedLegacyFound := false
	for i := range targetList.Items {
		helper := &targetList.Items[i]
		if !isClusterRestoreHelperPVC(helper) || helper.Labels[dptypes.ClusterUIDLabelKey] != "" {
			continue
		}
		if helper.Labels[dprestore.DataProtectionPopulatePVCLabelKey] != helper.Name ||
			verifiedTargetsByHelper[helper.Name] == nil {
			unverifiedLegacyFound = true
			continue
		}
		legacyHelpers = append(legacyHelpers, helper)
	}

	restoreList := &dpv1alpha1.RestoreList{}
	if err := r.directReader().List(reqCtx.Ctx, restoreList, client.InNamespace(cluster.Namespace), client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.Name,
	}); err != nil {
		return false, false, err
	}
	legacyRestores := make([]*dpv1alpha1.Restore, 0)
	for i := range restoreList.Items {
		restore := &restoreList.Items[i]
		if restore.Labels[dptypes.ClusterUIDLabelKey] != "" ||
			restore.Labels[dprestore.DataProtectionRestoreLabelKey] == "" {
			continue
		}
		owned, err := r.restoreOwnedByCluster(reqCtx.Ctx, restore, cluster)
		if err != nil {
			return false, false, err
		}
		if owned {
			legacyRestores = append(legacyRestores, restore)
		} else {
			unverifiedLegacyFound = true
		}
	}

	legacyFound := len(legacyTargets) > 0 || len(legacyHelpers) > 0 ||
		len(legacyRestores) > 0 || unverifiedLegacyFound
	if !legacyFound || !controllerutil.ContainsFinalizer(cluster, dptypes.RestoreProtectionFinalizerName) {
		return legacyFound, false, nil
	}
	changed := false
	for _, pvc := range legacyTargets {
		if err := r.patchLegacyClusterUID(reqCtx.Ctx, pvc, cluster); err != nil {
			return true, changed, err
		}
		changed = true
	}
	for _, helper := range legacyHelpers {
		if err := r.patchLegacyClusterUID(reqCtx.Ctx, helper, cluster); err != nil {
			return true, changed, err
		}
		changed = true
	}
	for _, restore := range legacyRestores {
		if err := r.patchLegacyClusterUID(reqCtx.Ctx, restore, cluster); err != nil {
			return true, changed, err
		}
		changed = true
	}
	if unverifiedLegacyFound {
		return true, changed, intctrlutil.NewRequeueError(reconcileInterval,
			fmt.Sprintf("legacy restore resources for Cluster %s/%s can not be safely attributed to Cluster UID %s",
				cluster.Namespace, cluster.Name, cluster.UID))
	}
	return true, changed, nil
}

func (r *ClusterRestoreReconciler) patchLegacyClusterUID(ctx context.Context,
	obj client.Object,
	cluster *appsv1.Cluster) error {
	patch := client.MergeFromWithOptions(obj.DeepCopyObject().(client.Object), client.MergeFromWithOptimisticLock{})
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[dptypes.ClusterUIDLabelKey] = string(cluster.UID)
	labels[constant.AppInstanceLabelKey] = cluster.Name
	obj.SetLabels(labels)
	if target, ok := obj.(*corev1.PersistentVolumeClaim); ok && isClusterRestoreTargetPVC(target) {
		annotations := target.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[constant.KBAppClusterUIDKey] = string(cluster.UID)
		target.SetAnnotations(annotations)
	}
	return r.Client.Patch(ctx, obj, patch)
}

func validateLegacyRestoreTargetPVC(ctx context.Context,
	reader client.Reader,
	pvc *corev1.PersistentVolumeClaim,
	cluster *appsv1.Cluster) error {
	if pvc.Labels[constant.AppInstanceLabelKey] != cluster.Name {
		return fmt.Errorf("target PVC %s/%s does not identify Cluster %s", pvc.Namespace, pvc.Name, cluster.Name)
	}
	owner := metav1.GetControllerOf(pvc)
	if owner == nil || owner.APIVersion != workloads.GroupVersion.String() {
		return fmt.Errorf("target PVC %s/%s has no supported workload controller owner", pvc.Namespace, pvc.Name)
	}
	var its *workloads.InstanceSet
	switch owner.Kind {
	case workloads.InstanceSetKind:
		its = &workloads.InstanceSet{}
		if err := reader.Get(ctx, client.ObjectKey{Namespace: pvc.Namespace, Name: owner.Name}, its); err != nil {
			return err
		}
		if owner.UID != its.UID {
			return fmt.Errorf("target PVC %s/%s owner UID does not match InstanceSet %s/%s", pvc.Namespace, pvc.Name, its.Namespace, its.Name)
		}
	case "Instance":
		instance := &workloads.Instance{}
		if err := reader.Get(ctx, client.ObjectKey{Namespace: pvc.Namespace, Name: owner.Name}, instance); err != nil {
			return err
		}
		if owner.UID != instance.UID {
			return fmt.Errorf("target PVC %s/%s owner UID does not match Instance %s/%s", pvc.Namespace, pvc.Name, instance.Namespace, instance.Name)
		}
		itsOwner := metav1.GetControllerOf(instance)
		if itsOwner == nil || itsOwner.APIVersion != workloads.GroupVersion.String() || itsOwner.Kind != workloads.InstanceSetKind {
			return fmt.Errorf("Instance %s/%s has no InstanceSet controller owner", instance.Namespace, instance.Name)
		}
		its = &workloads.InstanceSet{}
		if err := reader.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: itsOwner.Name}, its); err != nil {
			return err
		}
		if itsOwner.UID != its.UID {
			return fmt.Errorf("Instance %s/%s owner UID does not match InstanceSet %s/%s", instance.Namespace, instance.Name, its.Namespace, its.Name)
		}
	default:
		return fmt.Errorf("target PVC %s/%s has unsupported workload owner kind %s", pvc.Namespace, pvc.Name, owner.Kind)
	}
	componentOwner := metav1.GetControllerOf(its)
	if componentOwner == nil || componentOwner.APIVersion != appsv1.GroupVersion.String() || componentOwner.Kind != appsv1.ComponentKind {
		return fmt.Errorf("InstanceSet %s/%s has no Component controller owner", its.Namespace, its.Name)
	}
	component := &appsv1.Component{}
	if err := reader.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: componentOwner.Name}, component); err != nil {
		return err
	}
	if componentOwner.UID != component.UID || component.Annotations[constant.KBAppClusterUIDKey] != string(cluster.UID) ||
		component.Labels[constant.AppInstanceLabelKey] != cluster.Name {
		return fmt.Errorf("InstanceSet %s/%s is not owned by current Cluster %s/%s UID %s",
			its.Namespace, its.Name, cluster.Namespace, cluster.Name, cluster.UID)
	}
	return nil
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
