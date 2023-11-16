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
	"reflect"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dpbackup "github.com/apecloud/kubeblocks/pkg/dataprotection/backup"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
)

// BackupScheduleReconciler reconciles a BackupSchedule object
type BackupScheduleReconciler struct {
	client.Client
	Scheme   *k8sruntime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backupschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backupschedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backupschedules/finalizers,verbs=update

// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs/status,verbs=get
// +kubebuilder:rbac:groups=batch,resources=cronjobs/finalizers,verbs=update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the backupschedule closer to the desired state.
func (r *BackupScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("backupSchedule", req.NamespacedName),
		Recorder: r.Recorder,
	}

	backupSchedule := &dpv1alpha1.BackupSchedule{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, backupSchedule); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	original := backupSchedule.DeepCopy()

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, backupSchedule, dptypes.DataProtectionFinalizerName, func() (*ctrl.Result, error) {
		return nil, r.deleteExternalResources(reqCtx, backupSchedule)
	})
	if res != nil {
		return *res, err
	}

	if err = r.handleSchedule(reqCtx, backupSchedule); err != nil {
		return r.patchStatusFailed(reqCtx, backupSchedule, "HandleBackupScheduleFailed", err)
	}

	return r.patchStatusAvailable(reqCtx, original, backupSchedule)
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dpv1alpha1.BackupSchedule{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}

func (r *BackupScheduleReconciler) deleteExternalResources(
	reqCtx intctrlutil.RequestCtx,
	backupSchedule *dpv1alpha1.BackupSchedule) error {
	// delete cronjob resource
	cronJobList := &batchv1.CronJobList{}
	if err := r.Client.List(reqCtx.Ctx, cronJobList,
		client.InNamespace(backupSchedule.Namespace),
		client.MatchingLabels{
			dptypes.BackupScheduleLabelKey: backupSchedule.Name,
		},
	); err != nil {
		return err
	}
	for _, cronjob := range cronJobList.Items {
		if err := dputils.RemoveDataProtectionFinalizer(reqCtx.Ctx, r.Client, &cronjob); err != nil {
			return err
		}
		if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, &cronjob); err != nil {
			// failed delete k8s job, return error info.
			return err
		}
	}
	// notice running backup to completed
	// TODO(ldm): is it necessary to notice running backup to completed?
	backup := &dpv1alpha1.Backup{}
	for _, s := range backupSchedule.Spec.Schedules {
		backupKey := client.ObjectKey{
			Namespace: backupSchedule.Namespace,
			Name:      dpbackup.GenerateCRNameByBackupSchedule(backupSchedule, s.BackupMethod),
		}
		if err := r.Client.Get(reqCtx.Ctx, backupKey, backup); err != nil {
			if client.IgnoreNotFound(err) == nil {
				continue
			}
			return err
		}
		patch := client.MergeFrom(backup.DeepCopy())
		backup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
		backup.Status.CompletionTimestamp = &metav1.Time{Time: time.Now().UTC()}
		if err := r.Client.Status().Patch(reqCtx.Ctx, backup, patch); err != nil {
			return err
		}
	}
	return nil
}

// patchStatusAvailable patches backup policy status phase to available.
func (r *BackupScheduleReconciler) patchStatusAvailable(reqCtx intctrlutil.RequestCtx,
	origin, backupSchedule *dpv1alpha1.BackupSchedule) (ctrl.Result, error) {
	if !reflect.DeepEqual(origin.Spec, backupSchedule.Spec) {
		if err := r.Client.Update(reqCtx.Ctx, backupSchedule); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}
	// update status phase
	if backupSchedule.Status.Phase != dpv1alpha1.BackupSchedulePhaseAvailable ||
		backupSchedule.Status.ObservedGeneration != backupSchedule.Generation {
		patch := client.MergeFrom(backupSchedule.DeepCopy())
		backupSchedule.Status.ObservedGeneration = backupSchedule.Generation
		backupSchedule.Status.Phase = dpv1alpha1.BackupSchedulePhaseAvailable
		backupSchedule.Status.FailureReason = ""
		if err := r.Client.Status().Patch(reqCtx.Ctx, backupSchedule, patch); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}
	return intctrlutil.Reconciled()
}

// patchStatusFailed patches backup policy status phase to failed.
func (r *BackupScheduleReconciler) patchStatusFailed(reqCtx intctrlutil.RequestCtx,
	backupSchedule *dpv1alpha1.BackupSchedule,
	reason string,
	err error) (ctrl.Result, error) {
	if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeRequeue) {
		return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
	}
	backupScheduleDeepCopy := backupSchedule.DeepCopy()
	backupSchedule.Status.Phase = dpv1alpha1.BackupSchedulePhaseFailed
	backupSchedule.Status.FailureReason = err.Error()
	if !reflect.DeepEqual(backupSchedule.Status, backupScheduleDeepCopy.Status) {
		if patchErr := r.Client.Status().Patch(reqCtx.Ctx, backupSchedule, client.MergeFrom(backupScheduleDeepCopy)); patchErr != nil {
			return intctrlutil.RequeueWithError(patchErr, reqCtx.Log, "")
		}
	}
	r.Recorder.Event(backupSchedule, corev1.EventTypeWarning, reason, err.Error())
	return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
}

// handleSchedule handles backup schedules for different backup method.
func (r *BackupScheduleReconciler) handleSchedule(
	reqCtx intctrlutil.RequestCtx,
	backupSchedule *dpv1alpha1.BackupSchedule) error {
	backupPolicy, err := dputils.GetBackupPolicyByName(reqCtx, r.Client, backupSchedule.Spec.BackupPolicyName)
	if err != nil {
		return err
	}
	if err = r.patchScheduleMetadata(reqCtx, backupSchedule); err != nil {
		return err
	}
	scheduler := dpbackup.Scheduler{
		RequestCtx:     reqCtx,
		BackupSchedule: backupSchedule,
		BackupPolicy:   backupPolicy,
		Client:         r.Client,
		Scheme:         r.Scheme,
	}
	return scheduler.Schedule()
}

func (r *BackupScheduleReconciler) patchScheduleMetadata(
	reqCtx intctrlutil.RequestCtx,
	backupSchedule *dpv1alpha1.BackupSchedule) error {
	if backupSchedule.Labels[dptypes.BackupPolicyLabelKey] == backupSchedule.Spec.BackupPolicyName {
		return nil
	}
	patch := client.MergeFrom(backupSchedule.DeepCopy())
	if backupSchedule.Labels == nil {
		backupSchedule.Labels = map[string]string{}
	}
	backupSchedule.Labels[dptypes.BackupPolicyLabelKey] = backupSchedule.Spec.BackupPolicyName
	return r.Client.Patch(reqCtx.Ctx, backupSchedule, patch)
}
