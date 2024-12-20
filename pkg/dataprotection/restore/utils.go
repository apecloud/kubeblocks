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

package restore

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
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
func SetRestoreCheckBackupRepoCondition(restore *dpv1alpha1.Restore, reason, message string) {
	status := metav1.ConditionFalse
	if reason == ReasonCheckBackupRepoSuccessfully {
		status = metav1.ConditionTrue
	}
	SetRestoreCondition(restore, status, ConditionTypeRestoreCheckBackupRepo, reason, message)
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
			statusAction.Message = fmt.Sprintf(`"%s" is failed, you can describe it or logs the ownered pod to get more information`, statusAction.ObjectKey)
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
		if !strings.HasPrefix(existingAction.Message, dptypes.LogCollectorOutput) {
			existingAction.Message = statusAction.Message
		}
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
		constant.AppManagedByLabelKey: dptypes.AppName,
		DataProtectionRestoreLabelKey: restoreName,
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

func transformTimeWithZone(targetTime *metav1.Time, timeZone string) (*metav1.Time, error) {
	if timeZone == "" {
		return targetTime, nil
	}
	formatErr := fmt.Errorf("incorrect format: only support such as +07:00")
	if len(timeZone) != 6 {
		return targetTime, formatErr
	}
	strs := strings.Split(timeZone, ":")
	if len(strs) != 2 {
		return targetTime, formatErr
	}
	hour, err := strconv.Atoi(strs[0])
	if err != nil {
		return targetTime, formatErr
	}
	minute, err := strconv.Atoi(strs[1])
	if err != nil {
		return targetTime, formatErr
	}
	offset := hour * 60 * 60
	if hour < 0 {
		offset += -1 * minute * 60
	} else {
		offset += minute * 60
	}
	zone := time.FixedZone("UTC", offset)
	return &metav1.Time{Time: targetTime.In(zone)}, nil
}

func BuildJobKeyForActionStatus(jobName string) string {
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
	jobKey := BuildJobKeyForActionStatus(jobName)
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
	restoreMgr *RestoreManager) error {

	// get backupActionSet based on the specified backup name.
	backupName := restoreMgr.Restore.Spec.Backup.Name
	backupSet, err := restoreMgr.GetBackupActionSetByNamespaced(reqCtx, cli, backupName, restoreMgr.Restore.Spec.Backup.Namespace)
	if err != nil {
		return err
	}

	// validate restore parameters
	if backupSet.ActionSet != nil {
		if err := utils.ValidateParameters(backupSet.ActionSet, restoreMgr.Restore.Spec.Parameters, false); err != nil {
			return fmt.Errorf("fails to validate parameters with actionset %s: %v", backupSet.ActionSet.Name, err)
		}
	}

	// TODO: check if there is permission for cross namespace recovery.

	// check if the backup is completed exclude continuous backup.
	backupType := utils.GetBackupType(backupSet.ActionSet, &backupSet.UseVolumeSnapshot)
	if backupType != dpv1alpha1.BackupTypeContinuous && backupSet.Backup.Status.Phase != dpv1alpha1.BackupPhaseCompleted {
		err = intctrlutil.NewFatalError(fmt.Sprintf(`phase of backup "%s" is not completed`, backupName))
		return err
	}

	// build backupActionSets of prepareData and postReady stage based on the specified backup's type.
	switch backupType {
	case dpv1alpha1.BackupTypeFull, dpv1alpha1.BackupTypeSelective:
		restoreMgr.SetBackupSets(*backupSet)
	case dpv1alpha1.BackupTypeIncremental:
		err = restoreMgr.BuildIncrementalBackupActionSet(reqCtx, cli, *backupSet)
	case dpv1alpha1.BackupTypeDifferential:
		err = restoreMgr.BuildDifferentialBackupActionSets(reqCtx, cli, *backupSet)
	case dpv1alpha1.BackupTypeContinuous:
		err = restoreMgr.BuildContinuousRestoreManager(reqCtx, cli, *backupSet)
	default:
		err = intctrlutil.NewFatalError(fmt.Sprintf("backup type of %s is empty", backupName))
	}
	return err
}

func cutJobName(jobName string) string {
	l := len(jobName)
	if l > 63 {
		return fmt.Sprintf("%s-%s", jobName[:57], jobName[l-5:l])
	}
	return jobName
}

func FormatRestoreTimeAndValidate(restoreTimeStr string, continuousBackup *dpv1alpha1.Backup) (string, error) {
	if restoreTimeStr == "" {
		return restoreTimeStr, nil
	}
	layout := "Jan 02,2006 15:04:05 UTC-0700"
	restoreTime, err := time.Parse(layout, restoreTimeStr)
	if err != nil {
		// retry to parse time with RFC3339 format.
		var errRFC error
		restoreTime, errRFC = time.Parse(time.RFC3339, restoreTimeStr)
		if errRFC != nil {
			// if retry failure, report the error
			return restoreTimeStr, err
		}
	}
	restoreTimeStr = restoreTime.UTC().Format(time.RFC3339)
	// TODO: check with Recoverable time

	if continuousBackup.Status.TimeRange == nil || continuousBackup.Status.TimeRange.Start.IsZero() || continuousBackup.Status.TimeRange.End.IsZero() {
		return restoreTimeStr, fmt.Errorf("invalid timeRange of the backup")
	}
	if !isTimeInRange(restoreTime, continuousBackup.Status.TimeRange.Start.Time, continuousBackup.Status.TimeRange.End.Time) {
		return restoreTimeStr, fmt.Errorf("restore-to-time is out of time range, you can view the recoverable time: \n"+
			"\tkbcli cluster describe %s -n %s", continuousBackup.Labels[constant.AppInstanceLabelKey], continuousBackup.Namespace)
	}
	return restoreTimeStr, nil
}

func isTimeInRange(t time.Time, start time.Time, end time.Time) bool {
	return !t.Before(start) && !t.After(end)
}

func GetRestoreFromBackupAnnotation(
	backup *dpv1alpha1.Backup,
	volumeRestorePolicy string,
	restoreTime string,
	env []corev1.EnvVar,
	doReadyRestoreAfterClusterRunning bool,
	parameters []dpv1alpha1.ParameterPair,
) (string, error) {
	componentName := component.GetComponentNameFromObj(backup)
	if len(componentName) == 0 {
		return "", intctrlutil.NewFatalError("unable to obtain the name of the component to be recovered, please ensure that Backup.status.componentName exists")
	}
	restoreInfoMap := map[string]string{}
	restoreInfoMap[constant.BackupNameKeyForRestore] = backup.Name
	restoreInfoMap[constant.BackupNamespaceKeyForRestore] = backup.Namespace
	restoreInfoMap[constant.VolumeRestorePolicyKeyForRestore] = volumeRestorePolicy
	restoreInfoMap[constant.DoReadyRestoreAfterClusterRunning] = strconv.FormatBool(doReadyRestoreAfterClusterRunning)
	if restoreTime != "" {
		restoreInfoMap[constant.RestoreTimeKeyForRestore] = restoreTime
	}
	if env != nil {
		bytes, err := json.Marshal(env)
		if err != nil {
			return "", err
		}
		restoreInfoMap[constant.EnvForRestore] = string(bytes)
	}
	if len(parameters) > 0 {
		bytes, err := json.Marshal(parameters)
		if err != nil {
			return "", err
		}
		restoreInfoMap[constant.ParametersForRestore] = string(bytes)
	}
	connectionPassword := backup.Annotations[dptypes.ConnectionPasswordAnnotationKey]
	if connectionPassword != "" {
		restoreInfoMap[constant.ConnectionPassword] = connectionPassword
	}
	encryptedSystemAccountsString := backup.Annotations[constant.EncryptedSystemAccountsAnnotationKey]
	if encryptedSystemAccountsString != "" {
		encryptedSystemAccountsMap := map[string]map[string]string{}
		_ = json.Unmarshal([]byte(encryptedSystemAccountsString), &encryptedSystemAccountsMap)
		// only set systemAccounts owned by this component
		if encryptedSystemAccountsMap[componentName] != nil {
			encryptedComponentSystemAccountsBytes, _ := json.Marshal(encryptedSystemAccountsMap[componentName])
			restoreInfoMap[constant.EncryptedSystemAccounts] = string(encryptedComponentSystemAccountsBytes)
		}
	}
	restoreForClusterMap := map[string]map[string]string{}
	restoreForClusterMap[componentName] = restoreInfoMap
	bytes, err := json.Marshal(restoreForClusterMap)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// GetSourcePodNameFromTarget gets the source pod name from backup status target according to 'RequiredPolicyForAllPodSelection'.
func GetSourcePodNameFromTarget(target *dpv1alpha1.BackupStatusTarget,
	requiredPolicy *dpv1alpha1.RequiredPolicyForAllPodSelection,
	index int) (string, error) {
	if target.PodSelector.Strategy == dpv1alpha1.PodSelectionStrategyAny {
		return "", nil
	}
	if requiredPolicy == nil {
		return "", intctrlutil.NewFatalError("requiredPolicyForAllPodSelection can not be empty when the pod selection strategy of the source target is All")
	}
	if requiredPolicy.DataRestorePolicy == dpv1alpha1.OneToManyRestorePolicy {
		if requiredPolicy.SourceOfOneToMany == nil || requiredPolicy.SourceOfOneToMany.TargetPodName == "" {
			return "", intctrlutil.NewFatalError("the source target pod can not be empty when restore policy is OneToMany")
		}
		return requiredPolicy.SourceOfOneToMany.TargetPodName, nil
	}
	if index >= len(target.SelectedTargetPods) {
		return "", nil
	}
	// get the source target pod according to index for 'OneToOne' policy
	return target.SelectedTargetPods[index], nil
}

// GetVolumeSnapshotsBySourcePod gets the volume snapshots of the backup and group by source target pod.
func GetVolumeSnapshotsBySourcePod(backup *dpv1alpha1.Backup, target *dpv1alpha1.BackupStatusTarget, sourcePodName string) map[string]string {
	actions := backup.Status.Actions
	for i := range actions {
		if len(actions[i].VolumeSnapshots) == 0 {
			continue
		}
		if len(target.SelectedTargetPods) > 0 && !slices.Contains(target.SelectedTargetPods, actions[i].TargetPodName) {
			continue
		}
		if sourcePodName != "" && sourcePodName != actions[i].TargetPodName {
			continue
		}
		snapshotGroup := map[string]string{}
		for _, v := range actions[i].VolumeSnapshots {
			snapshotGroup[v.VolumeName] = v.Name
		}
		return snapshotGroup
	}
	return nil
}

// ValidateParentBackupSet validates the parent backup and child backup.
func ValidateParentBackupSet(parentBackupSet *BackupActionSet, backupSet *BackupActionSet) error {
	parentBackup := parentBackupSet.Backup
	backup := backupSet.Backup
	if parentBackup == nil || backup == nil {
		return intctrlutil.NewFatalError("parent backup or child backup is nil")
	}
	// validate parent backup policy
	if parentBackup.Spec.BackupPolicyName != backup.Spec.BackupPolicyName {
		return intctrlutil.NewFatalError(
			fmt.Sprintf(`parent backup policy: "%s" is defferent with child backup policy: "%s"`,
				parentBackup.Spec.BackupPolicyName, backup.Spec.BackupPolicyName))
	}
	// validate parent backup method
	if parentBackupSet.ActionSet != nil && parentBackupSet.ActionSet.Spec.BackupType == dpv1alpha1.BackupTypeIncremental {
		if parentBackup.Spec.BackupMethod != backup.Spec.BackupMethod {
			return intctrlutil.NewFatalError(
				fmt.Sprintf(`the parent incremental backup method "%s" is not the same with the child backup method "%s"`,
					parentBackup.Spec.BackupMethod, backup.Spec.BackupMethod))
		}
	} else if parentBackupSet.ActionSet != nil && parentBackupSet.ActionSet.Spec.BackupType == dpv1alpha1.BackupTypeFull {
		if parentBackup.Spec.BackupMethod != backup.Status.BackupMethod.CompatibleMethod {
			return intctrlutil.NewFatalError(
				fmt.Sprintf(`the parent full backup method "%s" is not compatible with the child backup method "%s"`,
					parentBackup.Spec.BackupMethod, backup.Spec.BackupMethod))
		}
	} else {
		return intctrlutil.NewFatalError(fmt.Sprintf(`the parent backup "%s" is not incremental or full backup`,
			parentBackup.Name))
	}
	// validate parent backup end time
	if !utils.CompareWithBackupStopTime(*parentBackup, *backup) {
		return intctrlutil.NewFatalError(fmt.Sprintf(`the parent backup "%s" is not before the child backup "%s"`,
			parentBackup.Name, backup.Name))
	}
	return nil
}

// BackupFilePathEnv returns the envs for backup root path and target relative path.
func BackupFilePathEnv(filePath, targetName, targetPodName string) []corev1.EnvVar {
	envs := []corev1.EnvVar{}
	if len(filePath) == 0 {
		return envs
	}
	targetRelativePath := ""
	if targetName != "" {
		targetRelativePath = filepath.Join("/", targetRelativePath, targetName)
	}
	if targetPodName != "" {
		targetRelativePath = filepath.Join("/", targetRelativePath, targetPodName)
	}
	envs = append(envs, []corev1.EnvVar{
		{
			Name:  dptypes.DPTargetRelativePath,
			Value: targetRelativePath,
		},
		{
			Name:  dptypes.DPBackupRootPath,
			Value: filepath.Join("/", filePath, "../"),
		},
	}...)
	return envs
}
