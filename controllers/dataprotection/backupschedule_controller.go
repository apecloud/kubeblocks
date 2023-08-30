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
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/leaanthony/debme"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	dpbackup "github.com/apecloud/kubeblocks/internal/dataprotection/backup"
	dperrors "github.com/apecloud/kubeblocks/internal/dataprotection/errors"
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/internal/dataprotection/utils"
	"github.com/apecloud/kubeblocks/internal/dataprotection/utils/boolptr"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
)

// BackupScheduleReconciler reconciles a BackupPolicy object
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

	origin := backupSchedule.DeepCopy()

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, backupSchedule, dptypes.DataProtectionFinalizerName, func() (*ctrl.Result, error) {
		return nil, r.deleteExternalResources(reqCtx, backupSchedule)
	})
	if res != nil {
		return *res, err
	}

	// try to remove expired or oldest backups, triggered by cronjob controller
	if err = r.removeExpiredBackups(reqCtx); err != nil {
		return r.patchStatusFailed(reqCtx, backupSchedule, "RemoveExpiredBackupsFailed", err)
	}

	if err = r.handleSchedule(reqCtx, backupSchedule); err != nil {
		return r.patchStatusFailed(reqCtx, backupSchedule, "HandleBackupScheduleFailed", err)
	}

	return r.patchStatusAvailable(reqCtx, origin, backupSchedule)
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dpv1alpha1.BackupSchedule{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(maxConcurDataProtectionReconKey),
		}).
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
			dataProtectionLabelBackupScheduleKey: backupSchedule.Name,
			constant.AppManagedByLabelKey:        constant.AppName,
		},
	); err != nil {
		return err
	}
	for _, cronjob := range cronJobList.Items {
		if err := r.removeCronJobFinalizer(reqCtx, &cronjob); err != nil {
			return err
		}
		if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, &cronjob); err != nil {
			// failed delete k8s job, return error info.
			return err
		}
	}
	// notice running backup to completed
	backup := &dpv1alpha1.Backup{}
	for _, s := range backupSchedule.Spec.Schedules {
		backupKey := client.ObjectKey{
			Namespace: backupSchedule.Namespace,
			Name:      generateCRNameByBackupSchedule(backupSchedule, s.BackupMethod),
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
	if backupSchedule.Status.Phase != dpv1alpha1.BackupScheduleAvailable ||
		backupSchedule.Status.ObservedGeneration != backupSchedule.Generation {
		patch := client.MergeFrom(backupSchedule.DeepCopy())
		backupSchedule.Status.ObservedGeneration = backupSchedule.Generation
		backupSchedule.Status.Phase = dpv1alpha1.BackupScheduleAvailable
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
	backupSchedule.Status.Phase = dpv1alpha1.BackupScheduleFailed
	backupSchedule.Status.FailureReason = err.Error()
	if !reflect.DeepEqual(backupSchedule.Status, backupScheduleDeepCopy.Status) {
		if patchErr := r.Client.Status().Patch(reqCtx.Ctx, backupSchedule, client.MergeFrom(backupScheduleDeepCopy)); patchErr != nil {
			return intctrlutil.RequeueWithError(patchErr, reqCtx.Log, "")
		}
	}
	r.Recorder.Event(backupSchedule, corev1.EventTypeWarning, reason, err.Error())
	return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
}

func (r *BackupScheduleReconciler) removeExpiredBackups(reqCtx intctrlutil.RequestCtx) error {
	backups := dpv1alpha1.BackupList{}
	if err := r.Client.List(reqCtx.Ctx, &backups,
		client.InNamespace(reqCtx.Req.Namespace)); err != nil {
		return err
	}
	now := metav1.Now()
	for _, item := range backups.Items {
		// ignore retained backup.
		if strings.EqualFold(item.GetLabels()[constant.BackupProtectionLabelKey], constant.BackupRetain) {
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
func (r *BackupScheduleReconciler) removeOldestBackups(reqCtx intctrlutil.RequestCtx,
	backupPolicyName string,
	backupType dpv1alpha1.BackupType,
	backupsHistoryLimit int32) error {
	if backupsHistoryLimit == 0 {
		return nil
	}
	matchLabels := map[string]string{
		dataProtectionLabelBackupPolicyKey: backupPolicyName,
		dataProtectionLabelBackupTypeKey:   string(backupType),
		dataProtectionLabelAutoBackupKey:   "true",
	}
	backups := dpv1alpha1.BackupList{}
	if err := r.Client.List(reqCtx.Ctx, &backups,
		client.InNamespace(reqCtx.Req.Namespace),
		client.MatchingLabels(matchLabels)); err != nil {
		return err
	}
	// filter final state backups only
	var backupItems []dpv1alpha1.Backup
	for _, item := range backups.Items {
		if item.Status.Phase == dpv1alpha1.BackupPhaseCompleted ||
			item.Status.Phase == dpv1alpha1.BackupPhaseFailed {
			backupItems = append(backupItems, item)
		}
	}
	numToDelete := len(backupItems) - int(backupsHistoryLimit)
	if numToDelete <= 0 {
		return nil
	}
	sort.Sort(dpbackup.ByBackupStartTime(backupItems))
	for i := 0; i < numToDelete; i++ {
		if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, &backupItems[i]); err != nil {
			// failed delete backups, return error info.
			return err
		}
	}
	return nil
}

// buildCronJob builds cronjob from backup policy.
func (r *BackupScheduleReconciler) buildCronJob(
	backupSchedule *dpv1alpha1.BackupSchedule,
	schedulePolicy *dpv1alpha1.SchedulePolicy,
	backupPolicy *dpv1alpha1.BackupPolicy,
	cronJobName string) (*batchv1.CronJob, error) {
	tplFile := "cronjob.cue"
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplFile))
	if err != nil {
		return nil, err
	}
	tolerationPodSpec := corev1.PodSpec{}
	if err = dputils.AddTolerations(&tolerationPodSpec); err != nil {
		return nil, err
	}

	ttl := metav1.Duration{Duration: schedulePolicy.RetentionPeriod.ToDuration()}
	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	if cronJobName == "" {
		cronJobName = generateCRNameByBackupSchedule(backupSchedule, schedulePolicy.BackupMethod)
	}
	target := backupPolicy.Spec.Target
	saName := func() string {
		if target.ServiceAccountName != nil {
			return *target.ServiceAccountName
		}
		return ""
	}
	options := backupScheduleOptions{
		Name:             cronJobName,
		BackupPolicyName: backupPolicy.Name,
		Namespace:        backupPolicy.Namespace,
		Cluster:          target.PodSelector.MatchLabels[constant.AppInstanceLabelKey],
		Schedule:         schedulePolicy.CronExpression,
		TTL:              ttl,
		BackupMethod:     schedulePolicy.BackupMethod,
		ServiceAccount:   saName(),
		MgrNamespace:     backupSchedule.Namespace,
		Image:            viper.GetString(constant.KBToolsImage),
		Tolerations:      &tolerationPodSpec,
	}
	optionsByte, err := json.Marshal(options)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("options", optionsByte); err != nil {
		return nil, err
	}
	cuePath := "cronjob"
	cronjobByte, err := cueValue.Lookup(cuePath)
	if err != nil {
		return nil, err
	}

	cronjob := &batchv1.CronJob{}
	if err = json.Unmarshal(cronjobByte, cronjob); err != nil {
		return nil, err
	}

	controllerutil.AddFinalizer(cronjob, dptypes.DataProtectionFinalizerName)
	// set labels
	for k, v := range backupPolicy.Labels {
		if cronjob.Labels == nil {
			cronjob.SetLabels(map[string]string{})
		}
		cronjob.Labels[k] = v
	}
	cronjob.Labels[dataProtectionLabelBackupScheduleKey] = backupPolicy.Name
	cronjob.Labels[dataProtectionLabelBackupTypeKey] = string("")
	return cronjob, nil
}

func (r *BackupScheduleReconciler) removeCronJobFinalizer(reqCtx intctrlutil.RequestCtx, cronjob *batchv1.CronJob) error {
	patch := client.MergeFrom(cronjob.DeepCopy())
	controllerutil.RemoveFinalizer(cronjob, dptypes.DataProtectionFinalizerName)
	return r.Patch(reqCtx.Ctx, cronjob, patch)
}

// reconcileCronJob will create/delete/patch cronjob according to cronExpression and policy changes.
func (r *BackupScheduleReconciler) reconcileCronJob(reqCtx intctrlutil.RequestCtx,
	backupSchedule *dpv1alpha1.BackupSchedule,
	schedulePolicy *dpv1alpha1.SchedulePolicy,
	backupPolicy *dpv1alpha1.BackupPolicy) error {
	// get cronjob from labels
	cronJob := &batchv1.CronJob{}
	cronJobList := &batchv1.CronJobList{}
	if err := r.Client.List(reqCtx.Ctx, cronJobList,
		client.InNamespace(backupSchedule.Namespace),
		client.MatchingLabels{
			dataProtectionLabelBackupScheduleKey: backupSchedule.Name,
			dataProtectionLabelBackupMethodKey:   schedulePolicy.BackupMethod,
			constant.AppManagedByLabelKey:        constant.AppName,
		},
	); err != nil {
		return err
	} else if len(cronJobList.Items) > 0 {
		cronJob = &cronJobList.Items[0]
	}

	// schedule is disabled, delete cronjob if exists
	if schedulePolicy == nil || !boolptr.IsSetToTrue(schedulePolicy.Enabled) {
		if len(cronJob.Name) != 0 {
			// delete the old cronjob.
			if err := r.removeCronJobFinalizer(reqCtx, cronJob); err != nil {
				return err
			}
			return r.Client.Delete(reqCtx.Ctx, cronJob)
		}
		// if no cron expression, return
		return nil
	}

	/*cronjobProto, err := r.buildCronJob(backupSchedule, basePolicy.Target, schedulePolicy.CronExpression, backType, cronJob.Name)
	if err != nil {
		return err
	}

	if backupSchedule.Spec.Schedule.StartingDeadlineMinutes != nil {
		startingDeadlineSeconds := *backupSchedule.Spec.Schedule.StartingDeadlineMinutes * 60
		cronjobProto.Spec.StartingDeadlineSeconds = &startingDeadlineSeconds
	}
	if len(cronJob.Name) == 0 {
		// if no cronjob, create it.
		return r.Client.Create(reqCtx.Ctx, cronjobProto)
	}
	// sync the cronjob with the current backup policy configuration.
	patch := client.MergeFrom(cronJob.DeepCopy())
	cronJob.Spec.StartingDeadlineSeconds = cronjobProto.Spec.StartingDeadlineSeconds
	cronJob.Spec.JobTemplate.Spec.BackoffLimit = &basePolicy.OnFailAttempted
	cronJob.Spec.JobTemplate.Spec.Template = cronjobProto.Spec.JobTemplate.Spec.Template
	cronJob.Spec.Schedule = schedulePolicy.CronExpression
	return r.Client.Patch(reqCtx.Ctx, cronJob, patch)*/
	return nil
}

// handlePolicy handles backup schedule.
func (r *BackupScheduleReconciler) handleSchedule(
	reqCtx intctrlutil.RequestCtx,
	backupSchedule *dpv1alpha1.BackupSchedule) error {
	backupPolicy, err := getBackupPolicyByName(reqCtx, r.Client, backupSchedule.Spec.BackupPolicyName)
	if err != nil {
		return err
	}

	// handle all schedules
	for _, s := range backupSchedule.Spec.Schedules {
		if err = r.handleOneSchedule(reqCtx, backupSchedule, &s, backupPolicy); err != nil {
			return err
		}
	}
	return nil
}

func (r *BackupScheduleReconciler) handleOneSchedule(
	reqCtx intctrlutil.RequestCtx,
	backupSchedule *dpv1alpha1.BackupSchedule,
	schedulePolicy *dpv1alpha1.SchedulePolicy,
	backupPolicy *dpv1alpha1.BackupPolicy) error {
	// TODO(ldm): better to remove this dependency in the future
	if err := r.reconfigure(reqCtx, backupSchedule, schedulePolicy, backupPolicy); err != nil {
		return err
	}

	// create/delete/patch cronjob workload
	if err := r.reconcileCronJob(reqCtx, backupSchedule, schedulePolicy, backupPolicy); err != nil {
		return err
	}

	// return r.removeOldestBackups(reqCtx, backupSchedule.Name, backType, basePolicy.BackupsHistoryLimit)
	return nil
}

/*// handleSnapshotPolicy handles snapshot policy.
func (r *BackupScheduleReconciler) handleSnapshotPolicy(
	reqCtx intctrlutil.RequestCtx,
	backupPolicy *dpv1alpha1.BackupPolicy) error {
	if backupPolicy.Spec.Snapshot == nil {
		// TODO delete cronjob if exists
		return nil
	}
	return r.handlePolicy(reqCtx, backupPolicy, backupPolicy.Spec.Snapshot.BasePolicy,
		backupPolicy.Spec.Schedule.Snapshot, dpv1alpha1.BackupTypeSnapshot)
}

// handleDatafilePolicy handles datafile policy.
func (r *BackupScheduleReconciler) handleDatafilePolicy(
	reqCtx intctrlutil.RequestCtx,
	backupPolicy *dpv1alpha1.BackupPolicy) error {
	if backupPolicy.Spec.Datafile == nil {
		// TODO delete cronjob if exists
		return nil
	}
	r.setGlobalPersistentVolumeClaim(backupPolicy.Spec.Datafile)
	return r.handlePolicy(reqCtx, backupPolicy, backupPolicy.Spec.Datafile.BasePolicy,
		backupPolicy.Spec.Schedule.Datafile, dpv1alpha1.BackupTypeDataFile)
}

// setGlobalPersistentVolumeClaim sets global config of pvc to common policy.
func (r *BackupScheduleReconciler) setGlobalPersistentVolumeClaim(backupPolicy *dpv1alpha1.CommonBackupPolicy) {
	pvcCfg := backupPolicy.PersistentVolumeClaim
	globalPVCName := viper.GetString(constant.CfgKeyBackupPVCName)
	if (pvcCfg.Name == nil || len(*pvcCfg.Name) == 0) && globalPVCName != "" {
		backupPolicy.PersistentVolumeClaim.Name = &globalPVCName
	}

	globalInitCapacity := viper.GetString(constant.CfgKeyBackupPVCInitCapacity)
	if pvcCfg.InitCapacity.IsZero() && globalInitCapacity != "" {
		backupPolicy.PersistentVolumeClaim.InitCapacity = resource.MustParse(globalInitCapacity)
	}
}*/

type backupReconfigureRef struct {
	Name    string         `json:"name"`
	Key     string         `json:"key"`
	Enable  parameterPairs `json:"enable,omitempty"`
	Disable parameterPairs `json:"disable,omitempty"`
}

type parameterPairs map[string][]appsv1alpha1.ParameterPair

func (r *BackupScheduleReconciler) reconfigure(reqCtx intctrlutil.RequestCtx,
	backupSchedule *dpv1alpha1.BackupSchedule,
	schedulePolicy *dpv1alpha1.SchedulePolicy,
	backupPolicy *dpv1alpha1.BackupPolicy) error {
	reconfigRef := backupSchedule.Annotations[dptypes.ReconfigureRefAnnotationKey]
	if reconfigRef == "" {
		return nil
	}
	configRef := backupReconfigureRef{}
	if err := json.Unmarshal([]byte(reconfigRef), &configRef); err != nil {
		return err
	}

	enable := boolptr.IsSetToTrue(schedulePolicy.Enabled)
	if backupSchedule.Annotations[constant.LastAppliedConfigAnnotationKey] == "" && !enable {
		// disable in the first policy created, no need reconfigure because default configs had been set.
		return nil
	}
	configParameters := configRef.Disable
	if enable {
		configParameters = configRef.Enable
	}
	if configParameters == nil {
		return nil
	}
	parameters := configParameters[schedulePolicy.BackupMethod]
	if len(parameters) == 0 {
		// skip reconfigure if not found parameters.
		return nil
	}
	updateParameterPairsBytes, _ := json.Marshal(parameters)
	updateParameterPairs := string(updateParameterPairsBytes)
	if updateParameterPairs == backupSchedule.Annotations[constant.LastAppliedConfigAnnotationKey] {
		// reconcile the config job if finished
		return r.reconcileReconfigure(reqCtx, backupSchedule)
	}

	targetPodSelector := backupPolicy.Spec.Target.PodSelector
	ops := appsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: backupSchedule.Name + "-",
			Namespace:    backupSchedule.Namespace,
			Labels: map[string]string{
				dataProtectionLabelBackupScheduleKey: backupSchedule.Name,
			},
		},
		Spec: appsv1alpha1.OpsRequestSpec{
			Type:       appsv1alpha1.ReconfiguringType,
			ClusterRef: targetPodSelector.MatchLabels[constant.AppInstanceLabelKey],
			Reconfigure: &appsv1alpha1.Reconfigure{
				ComponentOps: appsv1alpha1.ComponentOps{
					ComponentName: targetPodSelector.MatchLabels[constant.KBAppComponentLabelKey],
				},
				Configurations: []appsv1alpha1.Configuration{
					{
						Name: configRef.Name,
						Keys: []appsv1alpha1.ParameterConfig{
							{
								Key:        configRef.Key,
								Parameters: parameters,
							},
						},
					},
				},
			},
		},
	}
	if err := r.Client.Create(reqCtx.Ctx, &ops); err != nil {
		return err
	}
	r.Recorder.Eventf(backupSchedule, corev1.EventTypeNormal, "Reconfiguring", "update config %s", updateParameterPairs)
	patch := client.MergeFrom(backupSchedule.DeepCopy())
	if backupSchedule.Annotations == nil {
		backupSchedule.Annotations = map[string]string{}
	}
	backupSchedule.Annotations[constant.LastAppliedConfigAnnotationKey] = updateParameterPairs
	if err := r.Client.Patch(reqCtx.Ctx, backupSchedule, patch); err != nil {
		return err
	}
	return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRequeue, "requeue to waiting for ops %s finished.", ops.Name)
}

func (r *BackupScheduleReconciler) reconcileReconfigure(
	reqCtx intctrlutil.RequestCtx,
	backupSchedule *dpv1alpha1.BackupSchedule) error {
	opsList := appsv1alpha1.OpsRequestList{}
	if err := r.Client.List(reqCtx.Ctx, &opsList,
		client.InNamespace(backupSchedule.Namespace),
		client.MatchingLabels{dataProtectionLabelBackupScheduleKey: backupSchedule.Name}); err != nil {
		return err
	}
	if len(opsList.Items) > 0 {
		sort.Slice(opsList.Items, func(i, j int) bool {
			return opsList.Items[j].CreationTimestamp.Before(&opsList.Items[i].CreationTimestamp)
		})
		latestOps := opsList.Items[0]
		if latestOps.Status.Phase == appsv1alpha1.OpsFailedPhase {
			return intctrlutil.NewErrorf(dperrors.ErrorTypeReconfigureFailed, "ops failed %s", latestOps.Name)
		} else if latestOps.Status.Phase != appsv1alpha1.OpsSucceedPhase {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRequeue, "requeue to waiting for ops %s finished.", latestOps.Name)
		}
	}
	return nil
}
