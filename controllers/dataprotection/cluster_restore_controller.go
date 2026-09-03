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

	corev1 "k8s.io/api/core/v1"
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

// ClusterRestoreReconciler coordinates the Cluster-level restore lifecycle.
type ClusterRestoreReconciler struct {
	client.Client
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=restores,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch

func (r *ClusterRestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx, Req: req,
		Log:      log.FromContext(ctx).WithValues("cluster-restore", req.NamespacedName),
		Recorder: r.Recorder,
	}
	cluster := &appsv1.Cluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if cluster.DeletionTimestamp.IsZero() {
		if clusterRestoreConditionActive(cluster) {
			return r.ensureFinalizer(reqCtx, cluster)
		}
	} else if !controllerutil.ContainsFinalizer(cluster, dptypes.RestoreProtectionFinalizerName) {
		return intctrlutil.Reconciled()
	}

	hasResources, err := r.hasRestoreResources(ctx, cluster)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to inspect Cluster restore resources")
	}
	if cluster.DeletionTimestamp.IsZero() {
		if hasResources {
			return r.ensureFinalizer(reqCtx, cluster)
		}
		return r.removeFinalizer(reqCtx, cluster)
	}
	if hasResources {
		return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log,
			"waiting for restore resource owners to finish Cluster termination")
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
	// Restore=False is terminal for status aggregation, but the failed Cluster
	// still carries initial-restore intent. Keep the protection until the user
	// deletes the Cluster so remaining PVC restores never observe an absent
	// protection finalizer and stall between App and DP state machines.
	return condition == nil || condition.Status != metav1.ConditionTrue
}

func (r *ClusterRestoreReconciler) hasRestoreResources(ctx context.Context,
	cluster *appsv1.Cluster) (bool, error) {
	// VP releases target protection only after observing postReady Restore in
	// the cache. Inspect PVCs before Restores so that this handoff cannot fall
	// between the two resource scans.
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := r.Client.List(ctx, pvcs, client.InNamespace(cluster.Namespace), client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.Name,
	}); err != nil {
		return false, err
	}
	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		if pvc.Labels[dptypes.ClusterUIDLabelKey] != string(cluster.UID) {
			continue
		}
		if isClusterRestoreHelperPVC(pvc) ||
			(isClusterRestoreTargetPVC(pvc) && controllerutil.ContainsFinalizer(pvc, dptypes.DataProtectionFinalizerName)) {
			return true, nil
		}
	}
	restores := &dpv1alpha1.RestoreList{}
	if err := r.Client.List(ctx, restores, client.InNamespace(cluster.Namespace), client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.Name,
	}); err != nil {
		return false, err
	}
	for i := range restores.Items {
		restore := &restores.Items[i]
		if restore.Labels[dprestore.DataProtectionRestoreLabelKey] != restore.Name {
			continue
		}
		owned := restore.Labels[dptypes.ClusterUIDLabelKey] == string(cluster.UID)
		terminal := restore.Status.Phase == dpv1alpha1.RestorePhaseCompleted ||
			restore.Status.Phase == dpv1alpha1.RestorePhaseFailed
		if owned && (!cluster.DeletionTimestamp.IsZero() ||
			!terminal || !restore.DeletionTimestamp.IsZero()) {
			return true, nil
		}
	}

	return false, nil
}

func (r *ClusterRestoreReconciler) ensureFinalizer(reqCtx intctrlutil.RequestCtx,
	cluster *appsv1.Cluster) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(cluster, dptypes.RestoreProtectionFinalizerName) {
		return intctrlutil.Reconciled()
	}
	patch := client.MergeFromWithOptions(cluster.DeepCopy(), client.MergeFromWithOptimisticLock{})
	controllerutil.AddFinalizer(cluster, dptypes.RestoreProtectionFinalizerName)
	if err := r.Client.Patch(reqCtx.Ctx, cluster, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to add Cluster restore-protection finalizer")
	}
	return intctrlutil.Reconciled()
}

func (r *ClusterRestoreReconciler) removeFinalizer(reqCtx intctrlutil.RequestCtx,
	cluster *appsv1.Cluster) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(cluster, dptypes.RestoreProtectionFinalizerName) {
		return intctrlutil.Reconciled()
	}
	patch := client.MergeFromWithOptions(cluster.DeepCopy(), client.MergeFromWithOptimisticLock{})
	controllerutil.RemoveFinalizer(cluster, dptypes.RestoreProtectionFinalizerName)
	if err := r.Client.Patch(reqCtx.Ctx, cluster, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to remove Cluster restore-protection finalizer")
	}
	return intctrlutil.Reconciled()
}

func isClusterRestoreHelperPVC(pvc *corev1.PersistentVolumeClaim) bool {
	return pvc.Labels[dprestore.DataProtectionPopulatePVCLabelKey] != ""
}

func isClusterRestoreTargetPVC(pvc *corev1.PersistentVolumeClaim) bool {
	return pvc.Spec.DataSourceRef != nil && pvc.Spec.DataSourceRef.APIGroup != nil &&
		*pvc.Spec.DataSourceRef.APIGroup == dptypes.DataprotectionAPIGroup
}
