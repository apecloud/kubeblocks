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
	"fmt"
	"strconv"
	"strings"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// byBackupStartTime sorts a list of jobs by start timestamp, using their names as a tie breaker.
type byBackupStartTime []dataprotectionv1alpha1.Backup

// Len returns the length of byBackupStartTime, for the sort.Sort
func (o byBackupStartTime) Len() int { return len(o) }

// Swap the items, for the sort.Sort
func (o byBackupStartTime) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

// Less defines how to compare items, for the sort.Sort
func (o byBackupStartTime) Less(i, j int) bool {
	if o[i].Status.StartTimestamp == nil && o[j].Status.StartTimestamp != nil {
		return false
	}
	if o[i].Status.StartTimestamp != nil && o[j].Status.StartTimestamp == nil {
		return true
	}
	if o[i].Status.StartTimestamp.Equal(o[j].Status.StartTimestamp) {
		return o[i].Name < o[j].Name
	}
	return o[i].Status.StartTimestamp.Before(o[j].Status.StartTimestamp)
}

// getBackupToolByName gets the backupTool by name.
func getBackupToolByName(reqCtx intctrlutil.RequestCtx, cli client.Client, backupName string) (*dataprotectionv1alpha1.BackupTool, error) {
	backupTool := &dataprotectionv1alpha1.BackupTool{}
	backupToolNameSpaceName := types.NamespacedName{
		Name: backupName,
	}
	if err := cli.Get(reqCtx.Ctx, backupToolNameSpaceName, backupTool); err != nil {
		reqCtx.Log.Error(err, "Unable to get backupTool for backup.", "BackupTool", backupToolNameSpaceName)
		return nil, err
	}
	return backupTool, nil
}

// getCreatedCRNameByBackupPolicy gets the CR name which is created by BackupPolicy, such as CronJob/logfile Backup.
func getCreatedCRNameByBackupPolicy(backupPolicyName, backupPolicyNamespace string, backupType dataprotectionv1alpha1.BackupType) string {
	name := fmt.Sprintf("%s-%s", backupPolicyName, backupPolicyNamespace)
	if len(name) > 30 {
		name = strings.TrimRight(name[:30], "-")
	}
	return fmt.Sprintf("%s-%s", name, string(backupType))
}

// getBackupBatchV1Job gets the v1 job which is created by backup.
func getBackupBatchV1Job(reqCtx intctrlutil.RequestCtx, cli client.Client, backup *dataprotectionv1alpha1.Backup) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	jobNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backup.Name,
	}
	if err := cli.Get(reqCtx.Ctx, jobNameSpaceName, job); err != nil {
		// not found backup, do nothing
		reqCtx.Log.Info(err.Error())
		return nil, err
	}
	return job, nil
}

func getClusterLabelKeys() []string {
	return []string{constant.AppInstanceLabelKey, constant.KBAppComponentLabelKey}
}

func buildAutoCreationAnnotations(backupPolicyName string) map[string]string {
	return map[string]string{
		dataProtectionAnnotationCreateByPolicyKey: "true",
		dataProtectionLabelBackupPolicyKey:        backupPolicyName,
	}
}

// getBackupPathPrefix gets the backup path prefix.
func getBackupPathPrefix(backup *dataprotectionv1alpha1.Backup, pathPrefix string) string {
	pathPrefix = strings.TrimRight(pathPrefix, "/")
	if strings.TrimSpace(pathPrefix) == "" || strings.HasPrefix(pathPrefix, "/") {
		return fmt.Sprintf("/%s%s/%s", backup.Namespace, pathPrefix, backup.Name)
	}
	return fmt.Sprintf("/%s/%s/%s", backup.Namespace, pathPrefix, backup.Name)
}

// buildBackupWorkloadsLabels builds the labels for workloads which owned by backup.
func buildBackupWorkloadsLabels(backup *dataprotectionv1alpha1.Backup) map[string]string {
	labels := backup.Labels
	if labels == nil {
		labels = map[string]string{}
	} else {
		for _, v := range getClusterLabelKeys() {
			delete(labels, v)
		}
	}
	labels[constant.DataProtectionLabelBackupNameKey] = backup.Name
	return labels
}

func addTolerations(podSpec *corev1.PodSpec) (err error) {
	if cmTolerations := viper.GetString(constant.CfgKeyCtrlrMgrTolerations); cmTolerations != "" {
		if err = json.Unmarshal([]byte(cmTolerations), &podSpec.Tolerations); err != nil {
			return err
		}
	}
	if cmAffinity := viper.GetString(constant.CfgKeyCtrlrMgrAffinity); cmAffinity != "" {
		if err = json.Unmarshal([]byte(cmAffinity), &podSpec.Affinity); err != nil {
			return err
		}
	}
	if cmNodeSelector := viper.GetString(constant.CfgKeyCtrlrMgrNodeSelector); cmNodeSelector != "" {
		if err = json.Unmarshal([]byte(cmNodeSelector), &podSpec.NodeSelector); err != nil {
			return err
		}
	}
	return nil
}

// getIntervalSecondsForLogfile gets the interval seconds for logfile schedule cronExpression.
// currently, only the fields of minutes and hours are taken and contain expressions such as '*/'.
// If there is no such field, the default return is 60s.
func getIntervalSecondsForLogfile(backupType dataprotectionv1alpha1.BackupType, cronExpression string) string {
	if backupType != dataprotectionv1alpha1.BackupTypeLogFile {
		return ""
	}
	// move time zone field
	if strings.HasPrefix(cronExpression, "TZ=") || strings.HasPrefix(cronExpression, "CRON_TZ=") {
		i := strings.Index(cronExpression, " ")
		cronExpression = strings.TrimSpace(cronExpression[i:])
	}
	var interval = "60"
	// skip the macro syntax
	if strings.HasPrefix(cronExpression, "@") {
		return interval + "s"
	}
	fields := strings.Fields(cronExpression)
loop:
	for i, v := range fields {
		switch i {
		case 0:
			if strings.HasPrefix(v, "*/") {
				m, _ := strconv.Atoi(strings.ReplaceAll(v, "*/", ""))
				interval = strconv.Itoa(m * 60)
				break loop
			}
		case 1:
			if strings.HasPrefix(v, "*/") {
				m, _ := strconv.Atoi(strings.ReplaceAll(v, "*/", ""))
				interval = strconv.Itoa(m * 60 * 60)
				break loop
			}
		default:
			break loop
		}
	}
	return interval + "s"
}

// filterCreatedByPolicy filters the workloads which are create by backupPolicy.
func filterCreatedByPolicy(object client.Object) bool {
	labels := object.GetLabels()
	_, containsPolicyNameLabel := labels[dataProtectionLabelBackupPolicyKey]
	return labels[dataProtectionLabelAutoBackupKey] == "true" && containsPolicyNameLabel
}

// sendWarningEventForError sends warning event for backup controller error
func sendWarningEventForError(recorder record.EventRecorder, backup *dataprotectionv1alpha1.Backup, err error) {
	controllerErr := intctrlutil.UnwrapControllerError(err)
	if controllerErr != nil {
		recorder.Eventf(backup, corev1.EventTypeWarning, string(controllerErr.Type), err.Error())
	} else {
		recorder.Eventf(backup, corev1.EventTypeWarning, "FailedCreatedBackup",
			"Creating backup failed, error: %s", err.Error())
	}
}

var configVolumeSnapshotError = []string{
	"Failed to set default snapshot class with error",
	"Failed to get snapshot class with error",
}

func isVolumeSnapshotConfigError(snap *snapshotv1.VolumeSnapshot) bool {
	if snap.Status == nil || snap.Status.Error == nil || snap.Status.Error.Message == nil {
		return false
	}
	for _, errMsg := range configVolumeSnapshotError {
		if strings.Contains(*snap.Status.Error.Message, errMsg) {
			return true
		}
	}
	return false
}

func generateJSON(path string, value string) string {
	segments := strings.Split(path, ".")
	jsonString := value
	for i := len(segments) - 1; i >= 0; i-- {
		jsonString = fmt.Sprintf(`{\"%s\":%s}`, segments[i], jsonString)
	}
	return jsonString
}

// cropJobName job name cannot exceed 64 characters.
func cropJobName(jobName string) string {
	if len(jobName) > 63 {
		return jobName[:64]
	}
	return jobName
}

func buildBackupInfoENV(pathPrefix string) string {
	return backupPathBase + pathPrefix + "/backup.info"
}

func generateUniqueNameWithBackupPolicy(backupPolicy *dataprotectionv1alpha1.BackupPolicy) string {
	uniqueName := backupPolicy.Name
	if len(backupPolicy.OwnerReferences) > 0 {
		uniqueName = fmt.Sprintf("%s-%s", backupPolicy.OwnerReferences[0].UID[:8], backupPolicy.OwnerReferences[0].Name)
	}
	return uniqueName
}

func generateUniqueJobName(backup *dataprotectionv1alpha1.Backup, prefix string) string {
	return cropJobName(fmt.Sprintf("%s-%s-%s", prefix, backup.UID[:8], backup.Name))
}
