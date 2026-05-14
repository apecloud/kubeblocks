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
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

// ClusterBackupReconciler reconciles dataprotection backup cleanup during cluster deletion.
type ClusterBackupReconciler struct {
	client.Client
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;watch;delete

func (r *ClusterBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("cluster", req.NamespacedName),
		Recorder: r.Recorder,
	}

	cluster := &appsv1.Cluster{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if cluster.GetDeletionTimestamp().IsZero() {
		return r.ensureClusterFinalizer(reqCtx, cluster)
	}
	if !controllerutil.ContainsFinalizer(cluster, dptypes.DataProtectionFinalizerName) {
		return intctrlutil.Reconciled()
	}

	backups, err := r.listCandidateBackups(reqCtx.Ctx, cluster)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to list candidate backups")
	}
	if len(backups) > 0 {
		for _, backup := range backups {
			if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, backup); err != nil {
				return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to delete backup")
			}
		}
		return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "waiting for cluster backup cleanup")
	}

	return r.removeClusterFinalizer(reqCtx, cluster)
}

func (r *ClusterBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&appsv1.Cluster{}).
		Watches(&dpv1alpha1.Backup{}, handler.EnqueueRequestsFromMapFunc(r.mapBackupToCluster)).
		Complete(r)
}

func (r *ClusterBackupReconciler) mapBackupToCluster(_ context.Context, obj client.Object) []reconcile.Request {
	clusterName := obj.GetLabels()[constant.AppInstanceLabelKey]
	if clusterName == "" {
		return nil
	}
	return []reconcile.Request{{
		NamespacedName: client.ObjectKey{
			Namespace: obj.GetNamespace(),
			Name:      clusterName,
		},
	}}
}

func (r *ClusterBackupReconciler) ensureClusterFinalizer(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(cluster, dptypes.DataProtectionFinalizerName) {
		return intctrlutil.Reconciled()
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	controllerutil.AddFinalizer(cluster, dptypes.DataProtectionFinalizerName)
	if err := r.Client.Patch(reqCtx.Ctx, cluster, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to add cluster dataprotection finalizer")
	}
	return intctrlutil.Reconciled()
}

func (r *ClusterBackupReconciler) removeClusterFinalizer(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster) (ctrl.Result, error) {
	patch := client.MergeFrom(cluster.DeepCopy())
	controllerutil.RemoveFinalizer(cluster, dptypes.DataProtectionFinalizerName)
	if err := r.Client.Patch(reqCtx.Ctx, cluster, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to remove cluster dataprotection finalizer")
	}
	return intctrlutil.Reconciled()
}

func (r *ClusterBackupReconciler) listCandidateBackups(ctx context.Context, cluster *appsv1.Cluster) ([]*dpv1alpha1.Backup, error) {
	backups, err := r.listRelatedBackups(ctx, cluster)
	if err != nil {
		return nil, err
	}

	candidates := make([]*dpv1alpha1.Backup, 0, len(backups))
	for _, backup := range backups {
		if backup.Spec.DeletionPolicy == dpv1alpha1.BackupDeletionPolicyRetain {
			continue
		}
		if cluster.Spec.TerminationPolicy == appsv1.WipeOut {
			candidates = append(candidates, backup)
			continue
		}
		// TODO(r11y): This preserves the legacy apps-side behavior for non-WipeOut
		// deletion by selecting only failed, non-continuous backups. Because backup
		// deletion rewrites status.phase to Deleting, a backup chosen here may stop
		// matching before the Backup CR is fully removed. Revisit with an explicit
		// cluster-cleanup marker if we need to wait for CR disappearance strictly.
		if backup.Status.Phase != dpv1alpha1.BackupPhaseFailed {
			continue
		}
		if backup.Labels[dptypes.BackupTypeLabelKey] == string(dpv1alpha1.BackupTypeContinuous) {
			continue
		}
		candidates = append(candidates, backup)
	}
	return candidates, nil
}

func (r *ClusterBackupReconciler) listRelatedBackups(ctx context.Context, cluster *appsv1.Cluster) ([]*dpv1alpha1.Backup, error) {
	listByLabels := func(labels map[string]string) ([]dpv1alpha1.Backup, error) {
		backupList := &dpv1alpha1.BackupList{}
		if err := r.Client.List(ctx, backupList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
			return nil, err
		}
		return backupList.Items, nil
	}

	merged := make(map[client.ObjectKey]*dpv1alpha1.Backup)
	if clusterUID := string(cluster.UID); clusterUID != "" {
		backups, err := listByLabels(map[string]string{dptypes.ClusterUIDLabelKey: clusterUID})
		if err != nil {
			return nil, err
		}
		for i := range backups {
			backup := backups[i].DeepCopy()
			merged[client.ObjectKeyFromObject(backup)] = backup
		}
	}

	backups, err := listByLabels(map[string]string{constant.AppInstanceLabelKey: cluster.Name})
	if err != nil {
		return nil, err
	}
	for i := range backups {
		backup := backups[i].DeepCopy()
		backupClusterUID := backup.Labels[dptypes.ClusterUIDLabelKey]
		if backupClusterUID != "" && backupClusterUID != string(cluster.UID) {
			continue
		}
		merged[client.ObjectKeyFromObject(backup)] = backup
	}

	result := make([]*dpv1alpha1.Backup, 0, len(merged))
	for _, backup := range merged {
		result = append(result, backup)
	}
	return result, nil
}
