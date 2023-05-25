/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// CronJobReconciler reconciles a cronjob object
type CronJobReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs/status,verbs=get
// +kubebuilder:rbac:groups=batch,resources=cronjobs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *CronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		cronJob      = &batchv1.CronJob{}
		backupPolicy = &dataprotectionv1alpha1.BackupPolicy{}
		err          error
	)

	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("cronJob", req.NamespacedName),
	}

	if err = r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, cronJob); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	backupPolicyKey := types.NamespacedName{
		Namespace: cronJob.Annotations["kubeblocks.io/backup-namespace"],
		Name:      cronJob.Labels[dataProtectionLabelBackupPolicyKey],
	}
	if err = r.Client.Get(reqCtx.Ctx, backupPolicyKey, backupPolicy); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	patch := client.MergeFrom(backupPolicy.DeepCopy())
	if cronJob.Status.LastScheduleTime != nil {
		backupPolicy.Status.LastScheduleTime = cronJob.Status.LastScheduleTime
		backupPolicy.Status.LastSuccessfulTime = cronJob.Status.LastSuccessfulTime
		if err := r.Client.Status().Patch(ctx, backupPolicy, patch); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *CronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.CronJob{}).
		Owns(&batchv1.Job{}).
		WithEventFilter(predicate.NewPredicateFuncs(intctrlutil.ManagedByKubeBlocksFilterPredicate)).
		Complete(r)
}
