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
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
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

	if err = r.handleDatafilePolicy(reqCtx, backupPolicy); err != nil {
		return r.patchStatusFailed(reqCtx, backupPolicy, "HandleFullPolicyFailed", err)
	}

	if err = r.handleLogfilePolicy(reqCtx, backupPolicy); err != nil {
		return r.patchStatusFailed(reqCtx, backupPolicy, "HandleIncrementalPolicyFailed", err)
	}

	return r.patchStatusAvailable(reqCtx, originBackupPolicy, backupPolicy)
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dataprotectionv1alpha1.BackupPolicy{}).
		Watches(&source.Kind{Type: &dataprotectionv1alpha1.Backup{}}, r.backupDeleteHandler(),
			builder.WithPredicates(predicate.NewPredicateFuncs(filterCreatedByPolicy))).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(maxConcurDataProtectionReconKey),
		}).
		Complete(r)
}

func (r *BackupPolicyReconciler) backupDeleteHandler() *handler.Funcs {
	return &handler.Funcs{
		DeleteFunc: func(event event.DeleteEvent, limitingInterface workqueue.RateLimitingInterface) {
			backup := event.Object.(*dataprotectionv1alpha1.Backup)
			ctx := context.Background()
			backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
			if err := r.Client.Get(ctx, types.NamespacedName{Name: backup.Spec.BackupPolicyName, Namespace: backup.Namespace}, backupPolicy); err != nil {
				return
			}
			backupType := backup.Spec.BackupType
			// if not refer the backupTool, skip
			commonPolicy := backupPolicy.Spec.GetCommonPolicy(backupType)
			if commonPolicy == nil {
				return
			}
			// if not enable the schedule, skip
			schedulerPolicy := backupPolicy.Spec.GetCommonSchedulePolicy(backupType)
			if schedulerPolicy != nil && !schedulerPolicy.Enable {
				return
			}
			backupTool := &dataprotectionv1alpha1.BackupTool{}
			if err := r.Client.Get(ctx, types.NamespacedName{Name: commonPolicy.BackupToolName}, backupTool); err != nil {
				return
			}
			if backupTool.Spec.DeployKind != dataprotectionv1alpha1.DeployKindStatefulSet {
				return
			}
			_ = r.reconcileForStatefulSetKind(ctx, backupPolicy, backupType, schedulerPolicy.CronExpression)
		},
	}
}

func (r *BackupPolicyReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, backupPolicy *dataprotectionv1alpha1.BackupPolicy) error {
	// delete cronjob resource
	cronJobList := &batchv1.CronJobList{}
	if err := r.Client.List(reqCtx.Ctx, cronJobList,
		client.InNamespace(viper.GetString(constant.CfgKeyCtrlrMgrNS)),
		client.MatchingLabels{
			dataProtectionLabelBackupPolicyKey: backupPolicy.Name,
			constant.AppManagedByLabelKey:      constant.AppName,
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
	backup := &dataprotectionv1alpha1.Backup{}
	for _, v := range []dataprotectionv1alpha1.BackupType{dataprotectionv1alpha1.BackupTypeDataFile,
		dataprotectionv1alpha1.BackupTypeLogFile, dataprotectionv1alpha1.BackupTypeSnapshot} {
		if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: backupPolicy.Namespace,
			Name: getCreatedCRNameByBackupPolicy(backupPolicy, v),
		}, backup); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		patch := client.MergeFrom(backup.DeepCopy())
		backup.Status.Phase = dataprotectionv1alpha1.BackupCompleted
		backup.Status.CompletionTimestamp = &metav1.Time{Time: time.Now().UTC()}
		if err := r.Client.Status().Patch(reqCtx.Ctx, backup, patch); err != nil {
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
		backupPolicy.Status.ObservedGeneration = backupPolicy.Generation
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
	if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeRequeue) {
		return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
	}
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

// reconcileForStatefulSetKind reconciles the backup which is controlled by backupPolicy.
func (r *BackupPolicyReconciler) reconcileForStatefulSetKind(
	ctx context.Context,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy,
	backType dataprotectionv1alpha1.BackupType,
	cronExpression string) error {
	backupName := getCreatedCRNameByBackupPolicy(backupPolicy, backType)
	backup := &dataprotectionv1alpha1.Backup{}
	exists, err := intctrlutil.CheckResourceExists(ctx, r.Client, types.NamespacedName{Name: backupName, Namespace: backupPolicy.Namespace}, backup)
	if err != nil {
		return err
	}
	patch := client.MergeFrom(backup.DeepCopy())
	backup.Name = backupName
	backup.Namespace = backupPolicy.Namespace
	if backup.Labels == nil {
		backup.Labels = map[string]string{}
	}
	backup.Labels[constant.AppManagedByLabelKey] = constant.AppName
	backup.Labels[dataProtectionLabelBackupPolicyKey] = backupPolicy.Name
	backup.Labels[dataProtectionLabelBackupTypeKey] = string(backType)
	backup.Labels[dataProtectionLabelAutoBackupKey] = trueVal
	if !exists {
		if cronExpression == "" {
			return nil
		}
		backup.Spec.BackupType = backType
		backup.Spec.BackupPolicyName = backupPolicy.Name
		return intctrlutil.IgnoreIsAlreadyExists(r.Client.Create(ctx, backup))
	}

	// notice to reconcile backup CR
	if cronExpression != "" && slices.Contains([]dataprotectionv1alpha1.BackupPhase{
		dataprotectionv1alpha1.BackupCompleted, dataprotectionv1alpha1.BackupFailed},
		backup.Status.Phase) {
		// if schedule is enabled and backup already is completed, update phase to running
		backup.Status.Phase = dataprotectionv1alpha1.BackupRunning
		backup.Status.FailureReason = ""
		return r.Client.Status().Patch(ctx, backup, patch)
	}
	if backup.Annotations == nil {
		backup.Annotations = map[string]string{}
	}
	backup.Annotations[constant.ReconcileAnnotationKey] = time.Now().Format(time.RFC3339Nano)
	return r.Client.Patch(ctx, backup, patch)
}

// buildCronJob builds cronjob from backup policy.
func (r *BackupPolicyReconciler) buildCronJob(
	backupPolicy *dataprotectionv1alpha1.BackupPolicy,
	target dataprotectionv1alpha1.TargetCluster,
	cronExpression string,
	backType dataprotectionv1alpha1.BackupType,
	cronJobName string) (*batchv1.CronJob, error) {
	tplFile := "cronjob.cue"
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplFile))
	if err != nil {
		return nil, err
	}
	tolerationPodSpec := corev1.PodSpec{}
	if err = addTolerations(&tolerationPodSpec); err != nil {
		return nil, err
	}
	var ttl metav1.Duration
	if backupPolicy.Spec.Retention != nil && backupPolicy.Spec.Retention.TTL != nil {
		ttl = metav1.Duration{Duration: dataprotectionv1alpha1.ToDuration(backupPolicy.Spec.Retention.TTL)}
	}
	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	if cronJobName == "" {
		cronJobName = getCreatedCRNameByBackupPolicy(backupPolicy, backType)
	}
	options := backupPolicyOptions{
		Name:             cronJobName,
		BackupPolicyName: backupPolicy.Name,
		Namespace:        backupPolicy.Namespace,
		Cluster:          target.LabelsSelector.MatchLabels[constant.AppInstanceLabelKey],
		Schedule:         cronExpression,
		TTL:              ttl,
		BackupType:       string(backType),
		ServiceAccount:   viper.GetString("KUBEBLOCKS_SERVICEACCOUNT_NAME"),
		MgrNamespace:     viper.GetString(constant.CfgKeyCtrlrMgrNS),
		Image:            viper.GetString(constant.KBToolsImage),
		Tolerations:      &tolerationPodSpec,
	}
	backupPolicyOptionsByte, err := json.Marshal(options)
	if err != nil {
		return nil, err
	}
	if err = cueValue.Fill("options", backupPolicyOptionsByte); err != nil {
		return nil, err
	}
	cuePath := "cronjob"
	if backType == dataprotectionv1alpha1.BackupTypeLogFile {
		cuePath = "cronjob_logfile"
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
	schedulePolicy *dataprotectionv1alpha1.SchedulePolicy,
	backType dataprotectionv1alpha1.BackupType) error {
	// get cronjob from labels
	cronJob := &batchv1.CronJob{}
	cronJobList := &batchv1.CronJobList{}
	if err := r.Client.List(reqCtx.Ctx, cronJobList,
		client.InNamespace(viper.GetString(constant.CfgKeyCtrlrMgrNS)),
		client.MatchingLabels{
			dataProtectionLabelBackupPolicyKey: backupPolicy.Name,
			dataProtectionLabelBackupTypeKey:   string(backType),
			constant.AppManagedByLabelKey:      constant.AppName,
		},
	); err != nil {
		return err
	} else if len(cronJobList.Items) > 0 {
		cronJob = &cronJobList.Items[0]
	}
	if schedulePolicy == nil || !schedulePolicy.Enable {
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
	cronjobProto, err := r.buildCronJob(backupPolicy, basePolicy.Target, schedulePolicy.CronExpression, backType, cronJob.Name)
	if err != nil {
		return err
	}

	if backupPolicy.Spec.Schedule.StartingDeadlineMinutes != nil {
		startingDeadlineSeconds := *backupPolicy.Spec.Schedule.StartingDeadlineMinutes * 60
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
	return r.Client.Patch(reqCtx.Ctx, cronJob, patch)
}

// handlePolicy handles backup policy.
func (r *BackupPolicyReconciler) handlePolicy(reqCtx intctrlutil.RequestCtx,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy,
	basePolicy dataprotectionv1alpha1.BasePolicy,
	schedulePolicy *dataprotectionv1alpha1.SchedulePolicy,
	backType dataprotectionv1alpha1.BackupType) error {

	if err := r.reconfigure(reqCtx, backupPolicy, basePolicy, backType); err != nil {
		return err
	}
	// create/delete/patch cronjob workload
	if err := r.reconcileCronJob(reqCtx, backupPolicy, basePolicy, schedulePolicy, backType); err != nil {
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
	return r.handlePolicy(reqCtx, backupPolicy, backupPolicy.Spec.Snapshot.BasePolicy,
		backupPolicy.Spec.Schedule.Snapshot, dataprotectionv1alpha1.BackupTypeSnapshot)
}

// handleDatafilePolicy handles datafile policy.
func (r *BackupPolicyReconciler) handleDatafilePolicy(
	reqCtx intctrlutil.RequestCtx,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy) error {
	if backupPolicy.Spec.Datafile == nil {
		// TODO delete cronjob if exists
		return nil
	}
	r.setGlobalPersistentVolumeClaim(backupPolicy.Spec.Datafile)
	return r.handlePolicy(reqCtx, backupPolicy, backupPolicy.Spec.Datafile.BasePolicy,
		backupPolicy.Spec.Schedule.Datafile, dataprotectionv1alpha1.BackupTypeDataFile)
}

// handleLogFilePolicy handles logfile policy.
func (r *BackupPolicyReconciler) handleLogfilePolicy(
	reqCtx intctrlutil.RequestCtx,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy) error {
	logfile := backupPolicy.Spec.Logfile
	if logfile == nil {
		return nil
	}
	backupTool, err := getBackupToolByName(reqCtx, r.Client, logfile.BackupToolName)
	if err != nil {
		return err
	}
	r.setGlobalPersistentVolumeClaim(logfile)
	schedule := backupPolicy.Spec.Schedule.Logfile
	if backupTool.Spec.DeployKind == dataprotectionv1alpha1.DeployKindStatefulSet {
		var cronExpression string
		if schedule != nil && schedule.Enable {
			cronExpression = schedule.CronExpression
		}
		return r.reconcileForStatefulSetKind(reqCtx.Ctx, backupPolicy, dataprotectionv1alpha1.BackupTypeLogFile, cronExpression)
	}
	return r.handlePolicy(reqCtx, backupPolicy, logfile.BasePolicy, schedule, dataprotectionv1alpha1.BackupTypeLogFile)
}

// setGlobalPersistentVolumeClaim sets global config of pvc to common policy.
func (r *BackupPolicyReconciler) setGlobalPersistentVolumeClaim(backupPolicy *dataprotectionv1alpha1.CommonBackupPolicy) {
	pvcCfg := backupPolicy.PersistentVolumeClaim
	globalPVCName := viper.GetString(constant.CfgKeyBackupPVCName)
	if (pvcCfg.Name == nil || len(*pvcCfg.Name) == 0) && globalPVCName != "" {
		backupPolicy.PersistentVolumeClaim.Name = &globalPVCName
	}

	globalInitCapacity := viper.GetString(constant.CfgKeyBackupPVCInitCapacity)
	if pvcCfg.InitCapacity.IsZero() && globalInitCapacity != "" {
		backupPolicy.PersistentVolumeClaim.InitCapacity = resource.MustParse(globalInitCapacity)
	}
}

type backupReconfigureRef struct {
	Name    string         `json:"name"`
	Key     string         `json:"key"`
	Enable  parameterPairs `json:"enable,omitempty"`
	Disable parameterPairs `json:"disable,omitempty"`
}

type parameterPairs map[string][]appsv1alpha1.ParameterPair

func (r *BackupPolicyReconciler) reconfigure(reqCtx intctrlutil.RequestCtx,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy,
	basePolicy dataprotectionv1alpha1.BasePolicy,
	backType dataprotectionv1alpha1.BackupType) error {

	reconfigRef := backupPolicy.Annotations[constant.ReconfigureRefAnnotationKey]
	if reconfigRef == "" {
		return nil
	}
	configRef := backupReconfigureRef{}
	if err := json.Unmarshal([]byte(reconfigRef), &configRef); err != nil {
		return err
	}

	enable := false
	commonSchedule := backupPolicy.Spec.GetCommonSchedulePolicy(backType)
	if commonSchedule != nil {
		enable = commonSchedule.Enable
	}
	if backupPolicy.Annotations[constant.LastAppliedConfigAnnotationKey] == "" && !enable {
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
	parameters := configParameters[string(backType)]
	if len(parameters) == 0 {
		// skip reconfigure if not found parameters.
		return nil
	}
	updateParameterPairsBytes, _ := json.Marshal(parameters)
	updateParameterPairs := string(updateParameterPairsBytes)
	if updateParameterPairs == backupPolicy.Annotations[constant.LastAppliedConfigAnnotationKey] {
		// reconcile the config job if finished
		return r.reconcileReconfigure(reqCtx, backupPolicy)
	}

	ops := appsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: backupPolicy.Name + "-",
			Namespace:    backupPolicy.Namespace,
			Labels: map[string]string{
				dataProtectionLabelBackupPolicyKey: backupPolicy.Name,
			},
		},
		Spec: appsv1alpha1.OpsRequestSpec{
			Type:       appsv1alpha1.ReconfiguringType,
			ClusterRef: basePolicy.Target.LabelsSelector.MatchLabels[constant.AppInstanceLabelKey],
			Reconfigure: &appsv1alpha1.Reconfigure{
				ComponentOps: appsv1alpha1.ComponentOps{
					ComponentName: basePolicy.Target.LabelsSelector.MatchLabels[constant.KBAppComponentLabelKey],
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

	r.Recorder.Eventf(backupPolicy, corev1.EventTypeNormal, "Reconfiguring", "update config %s", updateParameterPairs)
	patch := client.MergeFrom(backupPolicy.DeepCopy())
	if backupPolicy.Annotations == nil {
		backupPolicy.Annotations = map[string]string{}
	}
	backupPolicy.Annotations[constant.LastAppliedConfigAnnotationKey] = updateParameterPairs
	if err := r.Client.Patch(reqCtx.Ctx, backupPolicy, patch); err != nil {
		return err
	}
	return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRequeue, "requeue to waiting for ops %s finished.", ops.Name)
}

func (r *BackupPolicyReconciler) reconcileReconfigure(reqCtx intctrlutil.RequestCtx,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy) error {

	opsList := appsv1alpha1.OpsRequestList{}
	if err := r.Client.List(reqCtx.Ctx, &opsList,
		client.InNamespace(backupPolicy.Namespace),
		client.MatchingLabels{dataProtectionLabelBackupPolicyKey: backupPolicy.Name}); err != nil {
		return err
	}
	if len(opsList.Items) > 0 {
		sort.Slice(opsList.Items, func(i, j int) bool {
			return opsList.Items[j].CreationTimestamp.Before(&opsList.Items[i].CreationTimestamp)
		})
		latestOps := opsList.Items[0]
		if latestOps.Status.Phase == appsv1alpha1.OpsFailedPhase {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeReconfigureFailed, "ops failed %s", latestOps.Name)
		} else if latestOps.Status.Phase != appsv1alpha1.OpsSucceedPhase {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRequeue, "requeue to waiting for ops %s finished.", latestOps.Name)
		}
	}
	return nil
}
