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

package restore

import (
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	dperrors "github.com/apecloud/kubeblocks/internal/dataprotection/errors"
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
)

func SetRestoreCondition(restore *dpv1alpha1.Restore, status metav1.ConditionStatus, conditionType, reason, message string) {
	condition := metav1.Condition{
		Type:    conditionType,
		Reason:  reason,
		Message: message,
		Status:  status,
	}
	meta.SetStatusCondition(&restore.Status.Conditions, condition)
}

// SetRestoreValidationCondition sets restore condition which type is ConditionTypeRestoreValidationPassed.
func SetRestoreValidationCondition(restore *dpv1alpha1.Restore, reason, message string) {
	status := metav1.ConditionFalse
	if reason == ReasonValidateSuccessfully {
		status = metav1.ConditionTrue
	}
	SetRestoreCondition(restore, status, ConditionTypeRestoreValidationPassed, reason, message)
}

// SetRestoreStageCondition sets restore stage condition.
func SetRestoreStageCondition(restore *dpv1alpha1.Restore, stage dpv1alpha1.RestoreStage, reason, message string) {
	status := metav1.ConditionFalse
	if reason == ReasonSucceed {
		status = metav1.ConditionTrue
	}
	conditionType := ConditionTypeRestorePreparedData
	if stage == dpv1alpha1.PostReady {
		conditionType = ConditionTypeRestorePostReady
	}
	SetRestoreCondition(restore, status, conditionType, reason, message)
}

func FindRestoreStatusAction(actions []dpv1alpha1.RestoreStatusAction, key string) *dpv1alpha1.RestoreStatusAction {
	for i := range actions {
		if actions[i].ObjectKey == key {
			return &actions[i]
		}
	}
	return nil
}

func SetRestoreStatusAction(actions *[]dpv1alpha1.RestoreStatusAction,
	statusAction dpv1alpha1.RestoreStatusAction) {
	if actions == nil {
		return
	}
	if statusAction.Message == "" {
		switch statusAction.Status {
		case dpv1alpha1.RestoreActionProcessing:
			statusAction.Message = fmt.Sprintf(`"%s" is processing`, statusAction.ObjectKey)
		case dpv1alpha1.RestoreActionCompleted:
			statusAction.Message = fmt.Sprintf(`successfully processed the "%s"`, statusAction.ObjectKey)
		case dpv1alpha1.RestoreActionFailed:
			statusAction.Message = fmt.Sprintf(`"%s" is failed, you can describe it or logs the ownered pod to get more informations`, statusAction.ObjectKey)
		}
	}
	if statusAction.Status != dpv1alpha1.RestoreActionProcessing {
		statusAction.EndTime = metav1.Now()
	}
	existingAction := FindRestoreStatusAction(*actions, statusAction.ObjectKey)
	if existingAction == nil {
		statusAction.StartTime = metav1.Now()
		*actions = append(*actions, statusAction)
		return
	}
	if existingAction.Status != statusAction.Status {
		existingAction.Status = statusAction.Status
		existingAction.EndTime = statusAction.EndTime
		existingAction.Message = statusAction.Message
	}
}

func GetRestoreActionsCountForPrepareData(config *dpv1alpha1.PrepareDataConfig) int {
	if config == nil {
		return 0
	}
	count := 1
	if config.RestoreVolumeClaimsTemplate != nil {
		count = int(config.RestoreVolumeClaimsTemplate.Replicas)
	}
	return count
}

func BuildRestoreLabels(restoreName string) map[string]string {
	return map[string]string{
		constant.AppManagedByLabelKey: constant.AppName,
		DataProtectionLabelRestoreKey: restoreName,
	}
}

func GetRestoreDuration(status dpv1alpha1.RestoreStatus) *metav1.Duration {
	if status.CompletionTimestamp == nil || status.StartTimestamp == nil {
		return nil
	}
	return &metav1.Duration{Duration: status.CompletionTimestamp.Sub(status.StartTimestamp.Time).Round(time.Second)}
}

func getTimeFormat(envs []corev1.EnvVar) string {
	for _, env := range envs {
		if env.Name == dptypes.DPTimeFormat {
			return env.Value
		}
	}
	return time.RFC3339
}

// CheckJobDone if the job is completed or failed, return true.
// if the job is failed, return an error to describe the failed message.
func CheckJobDone(job *batchv1.Job) (bool, error) {
	if job == nil {
		return false, nil
	}
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete {
			return true, nil
		} else if condition.Type == batchv1.JobFailed {
			return true, fmt.Errorf(condition.Reason + ": " + condition.Message)
		}
	}
	return false, nil
}

func compareWithBackupStopTime(backupI, backupJ dpv1alpha1.Backup) bool {
	endTimeI := backupI.GetEndTime()
	endTimeJ := backupJ.GetEndTime()
	if endTimeI.IsZero() {
		return false
	}
	if endTimeJ.IsZero() {
		return true
	}
	if endTimeI.Equal(endTimeJ) {
		return backupI.Name < backupJ.Name
	}
	return endTimeI.Before(endTimeJ)
}

func buildJobKeyForActionStatus(jobName string) string {
	return fmt.Sprintf("%s/%s", constant.JobKind, jobName)
}

func getMountPathWithSourceVolume(backup *dpv1alpha1.Backup, volumeSource string) string {
	backupMethod := backup.Status.BackupMethod
	if backupMethod != nil && backupMethod.TargetVolumes != nil {
		for _, v := range backupMethod.TargetVolumes.VolumeMounts {
			if v.Name == volumeSource {
				return v.MountPath
			}
		}
	}
	return ""
}

func restoreJobHasCompleted(statusActions []dpv1alpha1.RestoreStatusAction, jobName string) bool {
	jobKey := buildJobKeyForActionStatus(jobName)
	for i := range statusActions {
		if statusActions[i].ObjectKey == jobKey && statusActions[i].Status == dpv1alpha1.RestoreActionCompleted {
			return true
		}
	}
	return false
}

func deleteRestoreJob(reqCtx intctrlutil.RequestCtx, cli client.Client, jobKey string, namespace string) error {
	jobName := strings.ReplaceAll(jobKey, fmt.Sprintf("%s/", constant.JobKind), "")
	job := &batchv1.Job{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: jobName, Namespace: namespace}, job); err != nil {
		return client.IgnoreNotFound(err)
	}
	if controllerutil.ContainsFinalizer(job, dptypes.DataProtectionFinalizerName) {
		patch := client.MergeFrom(job.DeepCopy())
		controllerutil.RemoveFinalizer(job, dptypes.DataProtectionFinalizerName)
		if err := cli.Patch(reqCtx.Ctx, job, patch); err != nil {
			return err
		}
	}
	return intctrlutil.BackgroundDeleteObject(cli, reqCtx.Ctx, job)
}

// ValidateAndInitRestoreMGR validate if the restore CR is valid and init the restore manager.
func ValidateAndInitRestoreMGR(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	recorder record.EventRecorder,
	restoreMgr *RestoreManager) error {

	// get backupActionSet based on the specified backup name.
	backupName := restoreMgr.Restore.Spec.Backup.Name
	backupSet, err := restoreMgr.GetBackupActionSetByNamespaced(reqCtx, cli, backupName, restoreMgr.Restore.Spec.Backup.Namespace)
	if err != nil {
		return err
	}

	// TODO: check if there is permission for cross namespace recovery.

	// check if the backup is completed exclude continuous backup.
	var backupType dpv1alpha1.BackupType
	if backupSet.ActionSet != nil {
		backupType = backupSet.ActionSet.Spec.BackupType
	} else if backupSet.UseVolumeSnapshot {
		backupType = dpv1alpha1.BackupTypeFull
	}
	if backupType != dpv1alpha1.BackupTypeContinuous && backupSet.Backup.Status.Phase != dpv1alpha1.BackupPhaseCompleted {
		err = intctrlutil.NewFatalError(fmt.Sprintf(`phase of backup "%s" is not completed`, backupName))
		return err
	}

	// build backupActionSets of prepareData and postReady stage based on the specified backup's type.
	switch backupType {
	case dpv1alpha1.BackupTypeFull:
		restoreMgr.SetBackupSets(*backupSet)
	case dpv1alpha1.BackupTypeIncremental:
		err = restoreMgr.BuildIncrementalBackupActionSets(reqCtx, cli, *backupSet)
	case dpv1alpha1.BackupTypeDifferential:
		err = restoreMgr.BuildDifferentialBackupActionSets(reqCtx, cli, *backupSet)
	case dpv1alpha1.BackupTypeContinuous:
		err = intctrlutil.NewErrorf(dperrors.ErrorTypeWaitForExternalHandler, "wait for external handler to do handle the Point-In-Time recovery.")
		recorder.Event(restoreMgr.Restore, corev1.EventTypeWarning, string(dperrors.ErrorTypeWaitForExternalHandler), err.Error())
	default:
		err = intctrlutil.NewFatalError(fmt.Sprintf("backup type of %s is empty", backupName))
	}
	return err
}
