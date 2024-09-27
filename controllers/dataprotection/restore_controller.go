/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"fmt"
	"reflect"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dperrors "github.com/apecloud/kubeblocks/pkg/dataprotection/errors"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// RestoreReconciler reconciles a Restore object
type RestoreReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=restores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=restores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=restores/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

func (r *RestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("restore", req.NamespacedName),
		Recorder: r.Recorder,
	}

	// Get restore CR
	restore := &dpv1alpha1.Restore{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, restore); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, restore, dptypes.DataProtectionFinalizerName, func() (*ctrl.Result, error) {
		return nil, r.deleteExternalResources(reqCtx, restore)
	})
	if res != nil {
		return *res, err
	}

	switch restore.Status.Phase {
	case "":
		return r.newAction(reqCtx, restore)
	case dpv1alpha1.RestorePhaseRunning:
		return r.handleRunningPhase(reqCtx, restore)
	case dpv1alpha1.RestorePhaseCompleted:
		if err = r.deleteExternalResources(reqCtx, restore); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewNamespacedControllerManagedBy(mgr).
		For(&dpv1alpha1.Restore{}).
		Owns(&batchv1.Job{}).
		Watches(&batchv1.Job{}, handler.EnqueueRequestsFromMapFunc(r.parseRestoreJob)).
		Complete(r)
}

func (r *RestoreReconciler) parseRestoreJob(ctx context.Context, object client.Object) []reconcile.Request {
	job := object.(*batchv1.Job)
	var requests []reconcile.Request
	restoreName := job.Labels[dprestore.DataProtectionRestoreLabelKey]
	restoreNamespace := job.Labels[dprestore.DataProtectionRestoreNamespaceLabelKey]
	if restoreName != "" && restoreNamespace != "" {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: restoreNamespace,
				Name:      restoreName,
			},
		})
	}
	return requests
}

func (r *RestoreReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, restore *dpv1alpha1.Restore) error {
	labels := map[string]string{dprestore.DataProtectionRestoreLabelKey: restore.Name}

	// use map to avoid duplicate deletion of the same namespace.
	namespaces := map[string]sets.Empty{
		restore.Namespace:                          {},
		viper.GetString(constant.CfgKeyCtrlrMgrNS): {},
	}

	return deleteRelatedObjectList(reqCtx, r.Client, &batchv1.JobList{}, namespaces, labels)
}

func CheckBackupRepoForRestore(reqCtx intctrlutil.RequestCtx, cli client.Client, restore *dpv1alpha1.Restore) (string, error) {
	backupName := restore.Spec.Backup.Name
	backupNamespace := restore.Spec.Backup.Namespace
	backup := &dpv1alpha1.Backup{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: backupNamespace, Name: backupName}, backup); err != nil {
		if apierrors.IsNotFound(err) {
			err = intctrlutil.NewFatalError(err.Error())
		}
		return "", err
	}
	if backup.Status.BackupRepoName == "" {
		// The backup doesn't use backup repo.
		return "", nil
	}

	restoreNamespace := restore.Namespace
	repoName := backup.Status.BackupRepoName
	repo := &dpv1alpha1.BackupRepo{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: repoName}, repo); err != nil {
		if apierrors.IsNotFound(err) {
			return "", intctrlutil.NewFatalError(fmt.Sprintf("backup repo %s not found", repoName))
		}
		return repoName, err
	}
	if repo.Status.Phase != dpv1alpha1.BackupRepoReady {
		return repoName, dperrors.NewBackupRepoIsNotReady(repo.Name)
	}
	switch {
	case repo.AccessByMount():
		pvcName := repo.Status.BackupPVCName
		if pvcName == "" {
			return repoName, intctrlutil.NewFatalError(fmt.Sprintf("BackupPVCName is empty in BackupRepo %s", repoName))
		}
		pvc := &corev1.PersistentVolumeClaim{}
		pvcKey := client.ObjectKey{Namespace: restoreNamespace, Name: pvcName}
		if err := cli.Get(reqCtx.Ctx, pvcKey, pvc); err != nil {
			if apierrors.IsNotFound(err) {
				return repoName, intctrlutil.NewErrorf(dperrors.ErrorTypeWaitForBackupRepoPreparation,
					"backup repo %s is not ready in the namespace %s", repoName, restoreNamespace)
			}
			return repoName, err
		}
	case repo.AccessByTool():
		toolConfigSecretName := repo.Status.ToolConfigSecretName
		if toolConfigSecretName == "" {
			return repoName, intctrlutil.NewFatalError(fmt.Sprintf("ToolConfigSecretName is empty in BackupRepo %s", repoName))
		}
		secret := &corev1.Secret{}
		secretKey := client.ObjectKey{Namespace: restoreNamespace, Name: toolConfigSecretName}
		if err := cli.Get(reqCtx.Ctx, secretKey, secret); err != nil {
			if apierrors.IsNotFound(err) {
				return repoName, intctrlutil.NewErrorf(dperrors.ErrorTypeWaitForBackupRepoPreparation,
					"backup repo %s is not ready in the namespace %s", repoName, restoreNamespace)
			}
			return repoName, err
		}
	}
	return repoName, nil
}

func (r *RestoreReconciler) newAction(reqCtx intctrlutil.RequestCtx, restore *dpv1alpha1.Restore) (ctrl.Result, error) {
	oldRestore := restore.DeepCopy()
	patch := client.MergeFrom(oldRestore)
	// patch metaObject
	if restore.Labels == nil {
		restore.Labels = map[string]string{}
	}
	if _, ok := restore.Labels[constant.AppManagedByLabelKey]; !ok {
		restore.Labels[constant.AppManagedByLabelKey] = dptypes.AppName
	}
	waitBackupRepo := false
	repoName, err := CheckBackupRepoForRestore(reqCtx, r.Client, restore)
	switch {
	case intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal):
		dprestore.SetRestoreCheckBackupRepoCondition(restore, dprestore.ReasonCheckBackupRepoFailed, err.Error())
		restore.Status.Phase = dpv1alpha1.RestorePhaseFailed
		restore.Status.CompletionTimestamp = &metav1.Time{Time: time.Now()}
		r.Recorder.Event(restore, corev1.EventTypeWarning, dprestore.ReasonRestoreFailed, err.Error())
	case intctrlutil.IsTargetError(err, dperrors.ErrorTypeWaitForBackupRepoPreparation):
		dprestore.SetRestoreCheckBackupRepoCondition(restore, dprestore.ReasonWaitForBackupRepo, err.Error())
		waitBackupRepo = true
		restore.Labels[dataProtectionBackupRepoKey] = repoName
		restore.Labels[dataProtectionWaitRepoPreparationKey] = trueVal
	case err != nil:
		dprestore.SetRestoreCheckBackupRepoCondition(restore, ReasonUnknownError, err.Error())
		sendWarningEventForError(r.Recorder, restore, err)
		if patchErr := r.Client.Status().Patch(reqCtx.Ctx, restore, patch); patchErr != nil {
			return intctrlutil.CheckedRequeueWithError(patchErr, reqCtx.Log, "")
		}
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	default:
		dprestore.SetRestoreCheckBackupRepoCondition(restore, dprestore.ReasonCheckBackupRepoSuccessfully, "")
	}
	if !reflect.DeepEqual(restore.ObjectMeta, oldRestore.ObjectMeta) {
		if err := r.Client.Patch(reqCtx.Ctx, restore, patch); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}
	if waitBackupRepo {
		// If Restore and Backup are not in the same namespace, the backup repo may not
		// be available in the Restore's namespace. We need to wait for the BackupRepoController
		// to reconcile in such cases.
		// We don't use requeue here because after the BackupRepoController reconciles,
		// it removes relevant labels from the Restore resource, which in turn triggers
		// reconciliation for the Restore resource.
		return intctrlutil.Reconciled()
	}

	if restore.Spec.PrepareDataConfig != nil && restore.Spec.PrepareDataConfig.DataSourceRef != nil {
		restore.Status.Phase = dpv1alpha1.RestorePhaseAsDataSource
	} else {
		// check if restore CR is legal
		err := r.validateAndBuildMGR(reqCtx, dprestore.NewRestoreManager(restore, r.Recorder, r.Scheme))
		switch {
		case intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal):
			restore.Status.Phase = dpv1alpha1.RestorePhaseFailed
			restore.Status.CompletionTimestamp = &metav1.Time{Time: time.Now()}
			r.Recorder.Event(restore, corev1.EventTypeWarning, dprestore.ReasonRestoreFailed, err.Error())
		case err != nil:
			return RecorderEventAndRequeue(reqCtx, r.Recorder, restore, err)
		default:
			restore.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
			restore.Status.Phase = dpv1alpha1.RestorePhaseRunning
			r.Recorder.Event(restore, corev1.EventTypeNormal, dprestore.ReasonRestoreStarting, "start to restore")
		}
	}
	if err := r.Client.Status().Patch(reqCtx.Ctx, restore, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *RestoreReconciler) handleRunningPhase(reqCtx intctrlutil.RequestCtx, restore *dpv1alpha1.Restore) (ctrl.Result, error) {
	restoreMgr := dprestore.NewRestoreManager(restore, r.Recorder, r.Scheme)
	// validate if the restore.spec is valid and build restore manager.
	err := r.validateAndBuildMGR(reqCtx, restoreMgr)
	if err == nil {
		saName := restore.Spec.ServiceAccountName
		if saName == "" {
			// TODO: update the mcMgr param
			saName, err = EnsureWorkerServiceAccount(reqCtx, r.Client, restore.Namespace, nil)
		}
		restoreMgr.WorkerServiceAccount = saName
	}
	if err == nil {
		// handle restore actions
		err = r.HandleRestoreActions(reqCtx, restoreMgr)
	}
	if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
		// set restore phase to failed if the error is fatal.
		restoreMgr.Restore.Status.Phase = dpv1alpha1.RestorePhaseFailed
		restoreMgr.Restore.Status.CompletionTimestamp = &metav1.Time{Time: time.Now()}
		restoreMgr.Restore.Status.Duration = dprestore.GetRestoreDuration(restoreMgr.Restore.Status)
		r.Recorder.Event(restore, corev1.EventTypeWarning, dprestore.ReasonRestoreFailed, err.Error())
		err = nil
	}
	// patch restore status if changes occur
	if !reflect.DeepEqual(restoreMgr.OriginalRestore.Status, restoreMgr.Restore.Status) {
		err = r.Client.Status().Patch(reqCtx.Ctx, restoreMgr.Restore, client.MergeFrom(restoreMgr.OriginalRestore))
	}
	if err != nil {
		r.Recorder.Event(restore, corev1.EventTypeWarning, corev1.EventTypeWarning, err.Error())
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *RestoreReconciler) HandleRestoreActions(reqCtx intctrlutil.RequestCtx, restoreMgr *dprestore.RestoreManager) error {
	reqCtx.Log.V(1).Info("start to prepare data", "restore", reqCtx.Req.NamespacedName)
	// 1. handle the prepareData stage.
	isCompleted, err := r.prepareData(reqCtx, restoreMgr)
	if err != nil {
		return err
	}
	// if prepareData is not completed, return
	if !isCompleted {
		return nil
	}
	reqCtx.Log.V(1).Info("start to restore data after ready", "restore", reqCtx.Req.NamespacedName)
	// 2. handle the postReady stage.
	isCompleted, err = r.postReady(reqCtx, restoreMgr)
	if err != nil {
		return err
	}
	if isCompleted {
		restoreMgr.Restore.Status.Phase = dpv1alpha1.RestorePhaseCompleted
		restoreMgr.Restore.Status.CompletionTimestamp = &metav1.Time{Time: time.Now()}
		restoreMgr.Restore.Status.Duration = dprestore.GetRestoreDuration(restoreMgr.Restore.Status)
		r.Recorder.Event(restoreMgr.Restore, corev1.EventTypeNormal, dprestore.ReasonRestoreCompleted, "restore completed.")
	}
	return nil
}

// validateAndBuildMGR validates the spec is valid to restore. if ok, build a manager for restoring.
func (r *RestoreReconciler) validateAndBuildMGR(reqCtx intctrlutil.RequestCtx, restoreMgr *dprestore.RestoreManager) (err error) {
	defer func() {
		if err == nil {
			dprestore.SetRestoreValidationCondition(restoreMgr.Restore, dprestore.ReasonValidateSuccessfully, "validate restore spec successfully")
		} else if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
			dprestore.SetRestoreValidationCondition(restoreMgr.Restore, dprestore.ReasonValidateFailed, err.Error())
			r.Recorder.Event(restoreMgr.Restore, corev1.EventTypeWarning, dprestore.ReasonValidateFailed, err.Error())
		}
	}()
	err = dprestore.ValidateAndInitRestoreMGR(reqCtx, r.Client, restoreMgr)
	return err
}

// prepareData handles the prepareData stage of the backups.
func (r *RestoreReconciler) prepareData(reqCtx intctrlutil.RequestCtx, restoreMgr *dprestore.RestoreManager) (bool, error) {
	if len(restoreMgr.PrepareDataBackupSets) == 0 {
		return true, nil
	}
	prepareDataConfig := restoreMgr.Restore.Spec.PrepareDataConfig
	if prepareDataConfig == nil || (prepareDataConfig.RestoreVolumeClaimsTemplate == nil && len(prepareDataConfig.RestoreVolumeClaims) == 0) {
		return true, nil
	}
	if meta.IsStatusConditionTrue(restoreMgr.Restore.Status.Conditions, dprestore.ConditionTypeRestorePreparedData) {
		return true, nil
	}
	var (
		err         error
		isCompleted bool
	)
	defer func() {
		r.handleRestoreStageError(restoreMgr.Restore, dpv1alpha1.PrepareData, err)
	}()
	// set processing prepare data condition
	dprestore.SetRestoreStageCondition(restoreMgr.Restore, dpv1alpha1.PrepareData, dprestore.ReasonProcessing, "processing prepareData stage.")
	for i, v := range restoreMgr.PrepareDataBackupSets {
		isCompleted, err = r.handleBackupActionSet(reqCtx, restoreMgr, v, dpv1alpha1.PrepareData, i)
		if err != nil {
			return false, err
		}
		// waiting for restore jobs finished.
		if !isCompleted {
			return false, nil
		}
	}
	// set prepare data successfully condition
	dprestore.SetRestoreStageCondition(restoreMgr.Restore, dpv1alpha1.PrepareData, dprestore.ReasonSucceed, "prepare data successfully")
	return true, nil
}

func (r *RestoreReconciler) postReady(reqCtx intctrlutil.RequestCtx, restoreMgr *dprestore.RestoreManager) (bool, error) {
	readyConfig := restoreMgr.Restore.Spec.ReadyConfig
	if len(restoreMgr.PostReadyBackupSets) == 0 || readyConfig == nil {
		return true, nil
	}
	if meta.IsStatusConditionTrue(restoreMgr.Restore.Status.Conditions, dprestore.ConditionTypeRestorePostReady) {
		return true, nil
	}
	dprestore.SetRestoreStageCondition(restoreMgr.Restore, dpv1alpha1.PostReady, dprestore.ReasonProcessing, "processing postReady stage")
	var (
		err         error
		isCompleted bool
	)
	defer func() {
		r.handleRestoreStageError(restoreMgr.Restore, dpv1alpha1.PrepareData, err)
	}()
	if readyConfig.ReadinessProbe != nil && !meta.IsStatusConditionTrue(restoreMgr.Restore.Status.Conditions, dprestore.ConditionTypeReadinessProbe) {
		// TODO: check readiness probe, use a job and kubectl exec?
		_ = klog.TODO()
	}
	for _, v := range restoreMgr.PostReadyBackupSets {
		// handle postReady actions
		for i := range v.ActionSet.Spec.Restore.PostReady {
			isCompleted, err = r.handleBackupActionSet(reqCtx, restoreMgr, v, dpv1alpha1.PostReady, i)
			if err != nil {
				return false, err
			}
			// waiting for restore jobs finished.
			if !isCompleted {
				return false, nil
			}
		}
	}
	dprestore.SetRestoreStageCondition(restoreMgr.Restore, dpv1alpha1.PostReady, dprestore.ReasonSucceed, "processing postReady stage successfully")
	return true, nil
}

func (r *RestoreReconciler) handleBackupActionSet(reqCtx intctrlutil.RequestCtx,
	restoreMgr *dprestore.RestoreManager,
	backupSet dprestore.BackupActionSet,
	stage dpv1alpha1.RestoreStage,
	step int) (bool, error) {
	target := utils.GetBackupStatusTarget(backupSet.Backup, restoreMgr.Restore.Spec.Backup.SourceTargetName)
	if target == nil {
		return false, intctrlutil.NewFatalError("can not found any source targe in backup " + backupSet.Backup.Name)
	}
	handleFailed := func(restore *dpv1alpha1.Restore, backupName string) error {
		errorMsg := fmt.Sprintf(`restore failed for backup "%s", more information can be found in status.actions.%s`, backupName, stage)
		dprestore.SetRestoreStageCondition(restore, stage, dprestore.ReasonFailed, errorMsg)
		return intctrlutil.NewFatalError(errorMsg)
	}

	checkIsCompleted := func(allActionsFinished, existFailedAction bool) (bool, error) {
		if !allActionsFinished {
			return false, nil
		}
		if existFailedAction {
			return true, handleFailed(restoreMgr.Restore, backupSet.Backup.Name)
		}
		return true, nil
	}

	actionName := fmt.Sprintf("%s-%d", stage, step)
	// 1. check if the restore actions are completed from status.actions firstly.
	allActionsFinished, existFailedAction := restoreMgr.AnalysisRestoreActionsWithBackup(stage, backupSet.Backup.Name, actionName)
	isCompleted, err := checkIsCompleted(allActionsFinished, existFailedAction)
	if isCompleted || err != nil {
		return isCompleted, err
	}

	var jobs []*batchv1.Job
	switch stage {
	case dpv1alpha1.PrepareData:
		if backupSet.UseVolumeSnapshot {
			if err = restoreMgr.RestorePVCFromSnapshot(reqCtx, r.Client, backupSet, target); err != nil {
				return false, nil
			}
		}
		jobs, err = restoreMgr.BuildPrepareDataJobs(reqCtx, r.Client, backupSet, target, actionName)
	case dpv1alpha1.PostReady:
		// 2. build jobs for postReady action
		jobs, err = restoreMgr.BuildPostReadyActionJobs(reqCtx, r.Client, backupSet, target, step)
	}
	if err != nil {
		return false, err
	}
	if len(jobs) == 0 {
		return true, nil
	}
	// 3. create jobs
	jobs, err = restoreMgr.CreateJobsIfNotExist(reqCtx, r.Client, restoreMgr.Restore, jobs)
	if err != nil {
		return false, err
	}

	// 4. check if jobs are finished.
	allActionsFinished, existFailedAction = restoreMgr.CheckJobsDone(stage, actionName, backupSet, jobs)
	if stage == dpv1alpha1.PrepareData {
		// recalculation whether all actions have been completed.
		restoreMgr.Recalculation(backupSet.Backup.Name, actionName, &allActionsFinished, &existFailedAction)
	}
	return checkIsCompleted(allActionsFinished, existFailedAction)
}

func (r *RestoreReconciler) handleRestoreStageError(restore *dpv1alpha1.Restore, stage dpv1alpha1.RestoreStage, err error) {
	if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
		condition := meta.FindStatusCondition(restore.Status.Conditions, dprestore.ConditionTypeRestorePreparedData)
		if condition != nil && condition.Reason != dprestore.ReasonFailed {
			dprestore.SetRestoreStageCondition(restore, stage, dprestore.ReasonFailed, err.Error())
		}
	}
}
