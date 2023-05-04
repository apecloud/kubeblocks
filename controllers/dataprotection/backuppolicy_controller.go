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
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/leaanthony/debme"
	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// BackupPolicyReconciler reconciles a BackupPolicy object
type BackupPolicyReconciler struct {
	client.Client
	Scheme   *k8sruntime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies/finalizers,verbs=update

// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs/status,verbs=get
// +kubebuilder:rbac:groups=batch,resources=cronjobs/finalizers,verbs=update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the BackupPolicy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *BackupPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// NOTES:
	// setup common request context
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("backupPolicy", req.NamespacedName),
		Recorder: r.Recorder,
	}

	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, backupPolicy); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	originBackupPolicy := backupPolicy.DeepCopy()

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, backupPolicy, dataProtectionFinalizerName, func() (*ctrl.Result, error) {
		return nil, r.deleteExternalResources(reqCtx, backupPolicy)
	})
	if res != nil {
		return *res, err
	}

	// try to remove expired or oldest backups, triggered by cronjob controller
	if err = r.removeExpiredBackups(reqCtx); err != nil {
		return r.patchStatusFailed(reqCtx, backupPolicy, "RemoveExpiredBackupsFailed", err)
	}

	if err = r.handleSnapshotPolicy(reqCtx, backupPolicy); err != nil {
		return r.patchStatusFailed(reqCtx, backupPolicy, "HandleSnapshotPolicyFailed", err)
	}

	if err = r.handleFullPolicy(reqCtx, backupPolicy); err != nil {
		return r.patchStatusFailed(reqCtx, backupPolicy, "HandleFullPolicyFailed", err)
	}

	if err = r.handleIncrementalPolicy(reqCtx, backupPolicy); err != nil {
		return r.patchStatusFailed(reqCtx, backupPolicy, "HandleIncrementalPolicyFailed", err)
	}

	return r.patchStatusAvailable(reqCtx, originBackupPolicy, backupPolicy)
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dataprotectionv1alpha1.BackupPolicy{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(maxConcurDataProtectionReconKey),
		}).
		Complete(r)
}

func (r *BackupPolicyReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, backupPolicy *dataprotectionv1alpha1.BackupPolicy) error {
	// delete cronjob resource
	cronjob := &batchv1.CronJob{}

	for _, v := range []dataprotectionv1alpha1.BackupType{dataprotectionv1alpha1.BackupTypeFull,
		dataprotectionv1alpha1.BackupTypeIncremental, dataprotectionv1alpha1.BackupTypeSnapshot} {
		key := types.NamespacedName{
			Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
			Name:      r.getCronJobName(backupPolicy.Name, backupPolicy.Namespace, v),
		}
		if err := r.Client.Get(reqCtx.Ctx, key, cronjob); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}

		// TODO: checks backupPolicy's uuid to ensure the cronjob is created by this backupPolicy
		if err := r.removeCronJobFinalizer(reqCtx, cronjob); err != nil {
			return err
		}
		if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, cronjob); err != nil {
			// failed delete k8s job, return error info.
			return err
		}
	}
	return nil
}

// patchStatusAvailable patches backup policy status phase to available.
func (r *BackupPolicyReconciler) patchStatusAvailable(reqCtx intctrlutil.RequestCtx,
	originBackupPolicy,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy) (ctrl.Result, error) {
	if !reflect.DeepEqual(originBackupPolicy.Spec, backupPolicy.Spec) {
		if err := r.Client.Update(reqCtx.Ctx, backupPolicy); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}
	// update status phase
	if backupPolicy.Status.Phase != dataprotectionv1alpha1.PolicyAvailable ||
		backupPolicy.Status.ObservedGeneration != backupPolicy.Generation {
		patch := client.MergeFrom(backupPolicy.DeepCopy())
		backupPolicy.Status.Phase = dataprotectionv1alpha1.PolicyAvailable
		backupPolicy.Status.FailureReason = ""
		if err := r.Client.Status().Patch(reqCtx.Ctx, backupPolicy, patch); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}
	return intctrlutil.Reconciled()
}

// patchStatusFailed patches backup policy status phase to failed.
func (r *BackupPolicyReconciler) patchStatusFailed(reqCtx intctrlutil.RequestCtx,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy,
	reason string,
	err error) (ctrl.Result, error) {
	backupPolicyDeepCopy := backupPolicy.DeepCopy()
	backupPolicy.Status.Phase = dataprotectionv1alpha1.PolicyFailed
	backupPolicy.Status.FailureReason = err.Error()
	if !reflect.DeepEqual(backupPolicy.Status, backupPolicyDeepCopy.Status) {
		if patchErr := r.Client.Status().Patch(reqCtx.Ctx, backupPolicy, client.MergeFrom(backupPolicyDeepCopy)); patchErr != nil {
			return intctrlutil.RequeueWithError(patchErr, reqCtx.Log, "")
		}
	}
	r.Recorder.Event(backupPolicy, corev1.EventTypeWarning, reason, err.Error())
	return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
}

func (r *BackupPolicyReconciler) removeExpiredBackups(reqCtx intctrlutil.RequestCtx) error {
	backups := dataprotectionv1alpha1.BackupList{}
	if err := r.Client.List(reqCtx.Ctx, &backups,
		client.InNamespace(reqCtx.Req.Namespace)); err != nil {
		return err
	}
	now := metav1.Now()
	for _, item := range backups.Items {
		// ignore retained backup.
		if item.GetLabels()[constant.BackupProtectionLabelKey] == constant.BackupRetain {
			continue
		}
		if item.Status.Expiration != nil && item.Status.Expiration.Before(&now) {
			if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, &item); err != nil {
				// failed delete backups, return error info.
				return err
			}
		}
	}
	return nil
}

// removeOldestBackups removes old backups according to backupsHistoryLimit policy.
func (r *BackupPolicyReconciler) removeOldestBackups(reqCtx intctrlutil.RequestCtx,
	backupPolicyName string,
	backupType dataprotectionv1alpha1.BackupType,
	backupsHistoryLimit int32) error {
	if backupsHistoryLimit == 0 {
		return nil
	}
	matchLabels := map[string]string{
		dataProtectionLabelBackupPolicyKey: backupPolicyName,
		dataProtectionLabelBackupTypeKey:   string(backupType),
		dataProtectionLabelAutoBackupKey:   "true",
	}
	backups := dataprotectionv1alpha1.BackupList{}
	if err := r.Client.List(reqCtx.Ctx, &backups,
		client.InNamespace(reqCtx.Req.Namespace),
		client.MatchingLabels(matchLabels)); err != nil {
		return err
	}
	// filter final state backups only
	backupItems := []dataprotectionv1alpha1.Backup{}
	for _, item := range backups.Items {
		if item.Status.Phase == dataprotectionv1alpha1.BackupCompleted ||
			item.Status.Phase == dataprotectionv1alpha1.BackupFailed {
			backupItems = append(backupItems, item)
		}
	}
	numToDelete := len(backupItems) - int(backupsHistoryLimit)
	if numToDelete <= 0 {
		return nil
	}
	sort.Sort(byBackupStartTime(backupItems))
	for i := 0; i < numToDelete; i++ {
		if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, &backupItems[i]); err != nil {
			// failed delete backups, return error info.
			return err
		}
	}
	return nil
}

func (r *BackupPolicyReconciler) getCronJobName(backupPolicyName, backupPolicyNamespace string, backupType dataprotectionv1alpha1.BackupType) string {
	name := fmt.Sprintf("%s-%s", backupPolicyName, backupPolicyNamespace)
	if len(name) > 30 {
		name = name[:30]
	}
	return fmt.Sprintf("%s-%s", name, string(backupType))
}

// buildCronJob builds cronjob from backup policy.
func (r *BackupPolicyReconciler) buildCronJob(
	backupPolicy *dataprotectionv1alpha1.BackupPolicy,
	target dataprotectionv1alpha1.TargetCluster,
	cronExpression string,
	backType dataprotectionv1alpha1.BackupType) (*batchv1.CronJob, error) {
	tplFile := "cronjob.cue"
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplFile))
	if err != nil {
		return nil, err
	}
	var ttl metav1.Duration
	if backupPolicy.Spec.TTL != nil {
		ttl = metav1.Duration{Duration: dataprotectionv1alpha1.ToDuration(backupPolicy.Spec.TTL)}
	}
	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	options := backupPolicyOptions{
		Name:             r.getCronJobName(backupPolicy.Name, backupPolicy.Namespace, backType),
		BackupPolicyName: backupPolicy.Name,
		Namespace:        backupPolicy.Namespace,
		Cluster:          target.LabelsSelector.MatchLabels[constant.AppInstanceLabelKey],
		Schedule:         cronExpression,
		TTL:              ttl,
		BackupType:       string(backType),
		ServiceAccount:   viper.GetString("KUBEBLOCKS_SERVICEACCOUNT_NAME"),
		MgrNamespace:     viper.GetString(constant.CfgKeyCtrlrMgrNS),
		Image:            viper.GetString(constant.KBToolsImage),
	}
	backupPolicyOptionsByte, err := json.Marshal(options)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("options", backupPolicyOptionsByte); err != nil {
		return nil, err
	}
	cuePath := "cronjob"
	if backType == dataprotectionv1alpha1.BackupTypeIncremental {
		cuePath = "cronjob_incremental"
	}
	cronjobByte, err := cueValue.Lookup(cuePath)
	if err != nil {
		return nil, err
	}

	cronjob := &batchv1.CronJob{}
	if err = json.Unmarshal(cronjobByte, cronjob); err != nil {
		return nil, err
	}

	controllerutil.AddFinalizer(cronjob, dataProtectionFinalizerName)

	// set labels
	for k, v := range backupPolicy.Labels {
		if cronjob.Labels == nil {
			cronjob.SetLabels(map[string]string{})
		}
		cronjob.Labels[k] = v
	}
	cronjob.Labels[dataProtectionLabelBackupPolicyKey] = backupPolicy.Name
	cronjob.Labels[dataProtectionLabelBackupTypeKey] = string(backType)
	return cronjob, nil
}

func (r *BackupPolicyReconciler) removeCronJobFinalizer(reqCtx intctrlutil.RequestCtx, cronjob *batchv1.CronJob) error {
	patch := client.MergeFrom(cronjob.DeepCopy())
	controllerutil.RemoveFinalizer(cronjob, dataProtectionFinalizerName)
	return r.Patch(reqCtx.Ctx, cronjob, patch)
}

// reconcileCronJob will create/delete/patch cronjob according to cronExpression and policy changes.
func (r *BackupPolicyReconciler) reconcileCronJob(reqCtx intctrlutil.RequestCtx,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy,
	basePolicy dataprotectionv1alpha1.BasePolicy,
	cronExpression string,
	backType dataprotectionv1alpha1.BackupType) error {
	cronjobProto, err := r.buildCronJob(backupPolicy, basePolicy.Target, cronExpression, backType)
	if err != nil {
		return err
	}
	cronJob := &batchv1.CronJob{}
	if err = r.Client.Get(reqCtx.Ctx, client.ObjectKey{Name: cronjobProto.Name,
		Namespace: cronjobProto.Namespace}, cronJob); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if len(cronExpression) == 0 {
		if len(cronJob.Name) != 0 {
			// delete the old cronjob.
			if err = r.removeCronJobFinalizer(reqCtx, cronJob); err != nil {
				return err
			}
			return r.Client.Delete(reqCtx.Ctx, cronJob)
		}
		// if no cron expression, return
		return nil
	}

	if len(cronJob.Name) == 0 {
		// if no cronjob, create it.
		return r.Client.Create(reqCtx.Ctx, cronjobProto)
	}
	// sync the cronjob with the current backup policy configuration.
	patch := client.MergeFrom(cronJob.DeepCopy())
	cronJob.Spec.JobTemplate.Spec.BackoffLimit = &basePolicy.OnFailAttempted
	cronJob.Spec.JobTemplate.Spec.Template = cronjobProto.Spec.JobTemplate.Spec.Template
	cronJob.Spec.Schedule = cronExpression
	return r.Client.Patch(reqCtx.Ctx, cronJob, patch)
}

// handlePolicy the common function to handle backup policy.
func (r *BackupPolicyReconciler) handlePolicy(reqCtx intctrlutil.RequestCtx,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy,
	basePolicy dataprotectionv1alpha1.BasePolicy,
	cronExpression string,
	backType dataprotectionv1alpha1.BackupType) error {
	// create/delete/patch cronjob workload
	if err := r.reconcileCronJob(reqCtx, backupPolicy, basePolicy,
		cronExpression, backType); err != nil {
		return err
	}
	return r.removeOldestBackups(reqCtx, backupPolicy.Name, backType, basePolicy.BackupsHistoryLimit)
}

// handleSnapshotPolicy handles snapshot policy.
func (r *BackupPolicyReconciler) handleSnapshotPolicy(
	reqCtx intctrlutil.RequestCtx,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy) error {
	if backupPolicy.Spec.Snapshot == nil {
		// TODO delete cronjob if exists
		return nil
	}
	var cronExpression string
	schedule := backupPolicy.Spec.Schedule.BaseBackup
	if schedule != nil && schedule.Enable && schedule.Type == dataprotectionv1alpha1.BaseBackupTypeSnapshot {
		cronExpression = schedule.CronExpression
	}
	return r.handlePolicy(reqCtx, backupPolicy, backupPolicy.Spec.Snapshot.BasePolicy,
		cronExpression, dataprotectionv1alpha1.BackupTypeSnapshot)
}

// handleFullPolicy handles full policy.
func (r *BackupPolicyReconciler) handleFullPolicy(
	reqCtx intctrlutil.RequestCtx,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy) error {
	if backupPolicy.Spec.Full == nil {
		// TODO delete cronjob if exists
		return nil
	}
	var cronExpression string
	schedule := backupPolicy.Spec.Schedule.BaseBackup
	if schedule != nil && schedule.Enable && schedule.Type == dataprotectionv1alpha1.BaseBackupTypeFull {
		cronExpression = schedule.CronExpression
	}
	r.setGlobalPersistentVolumeClaim(backupPolicy.Spec.Full)
	return r.handlePolicy(reqCtx, backupPolicy, backupPolicy.Spec.Full.BasePolicy,
		cronExpression, dataprotectionv1alpha1.BackupTypeFull)
}

// handleIncrementalPolicy handles incremental policy.
func (r *BackupPolicyReconciler) handleIncrementalPolicy(
	reqCtx intctrlutil.RequestCtx,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy) error {
	if backupPolicy.Spec.Incremental == nil {
		return nil
	}
	var cronExpression string
	schedule := backupPolicy.Spec.Schedule.Incremental
	if schedule != nil && schedule.Enable {
		cronExpression = schedule.CronExpression
	}
	r.setGlobalPersistentVolumeClaim(backupPolicy.Spec.Incremental)
	return r.handlePolicy(reqCtx, backupPolicy, backupPolicy.Spec.Incremental.BasePolicy,
		cronExpression, dataprotectionv1alpha1.BackupTypeIncremental)
}

// setGlobalPersistentVolumeClaim sets global config of pvc to common policy.
func (r *BackupPolicyReconciler) setGlobalPersistentVolumeClaim(backupPolicy *dataprotectionv1alpha1.CommonBackupPolicy) {
	pvcCfg := backupPolicy.PersistentVolumeClaim
	globalPVCName := viper.GetString(constant.CfgKeyBackupPVCName)
	if len(pvcCfg.Name) == 0 && globalPVCName != "" {
		backupPolicy.PersistentVolumeClaim.Name = globalPVCName
	}

	globalInitCapacity := viper.GetString(constant.CfgKeyBackupPVCInitCapacity)
	if pvcCfg.InitCapacity.IsZero() && globalInitCapacity != "" {
		backupPolicy.PersistentVolumeClaim.InitCapacity = resource.MustParse(globalInitCapacity)
	}
}
