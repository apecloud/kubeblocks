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

package backup

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rogpeppe/go-internal/semver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/action"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
)

func getVolumesByNames(pod *corev1.Pod, volumeNames []string) []corev1.Volume {
	var volumes []corev1.Volume
	for _, v := range pod.Spec.Volumes {
		for _, name := range volumeNames {
			if v.Name == name {
				volumes = append(volumes, v)
			}
		}
	}
	return volumes
}

func getVolumesByMounts(pod *corev1.Pod, mounts []corev1.VolumeMount) []corev1.Volume {
	var volumes []corev1.Volume
	for _, v := range pod.Spec.Volumes {
		for _, m := range mounts {
			if v.Name == m.Name {
				volumes = append(volumes, v)
			}
		}
	}
	return volumes
}

// TODO: if the result is empty, should we return the pod's volumes?
//
//	if volumes can not found in the pod spec, maybe output a warning log?
func getVolumesByVolumeInfo(pod *corev1.Pod, volumeInfo *dpv1alpha1.TargetVolumeInfo) []corev1.Volume {
	if volumeInfo == nil {
		return nil
	}
	var volumes []corev1.Volume
	if len(volumeInfo.Volumes) > 0 {
		volumes = getVolumesByNames(pod, volumeInfo.Volumes)
	} else if len(volumeInfo.VolumeMounts) > 0 {
		volumes = getVolumesByMounts(pod, volumeInfo.VolumeMounts)
	}
	return volumes
}

func getVolumeMountsByVolumeInfo(pod *corev1.Pod, info *dpv1alpha1.TargetVolumeInfo) []corev1.VolumeMount {
	if info == nil || len(info.VolumeMounts) == 0 {
		return nil
	}
	var mounts []corev1.VolumeMount
	for _, v := range pod.Spec.Volumes {
		for _, m := range info.VolumeMounts {
			if v.Name == m.Name {
				mounts = append(mounts, m)
			}
		}
	}
	return mounts
}

func getPVCsByVolumeNames(cli client.Client,
	pod *corev1.Pod,
	volumeNames []string) ([]action.PersistentVolumeClaimWrapper, error) {
	if len(volumeNames) == 0 {
		return nil, nil
	}
	var all []action.PersistentVolumeClaimWrapper
	for _, v := range pod.Spec.Volumes {
		if v.PersistentVolumeClaim == nil {
			continue
		}
		for _, name := range volumeNames {
			if v.Name != name {
				continue
			}
			// get the PVC from pod's volumes
			tmp := corev1.PersistentVolumeClaim{}
			pvcKey := client.ObjectKey{Namespace: pod.Namespace, Name: v.PersistentVolumeClaim.ClaimName}
			if err := cli.Get(context.Background(), pvcKey, &tmp); err != nil {
				return nil, err
			}

			all = append(all, action.NewPersistentVolumeClaimWrapper(*tmp.DeepCopy(), name))
		}
	}
	return all, nil
}

func excludeLabelsForWorkload() []string {
	return []string{constant.KBAppComponentLabelKey}
}

// BuildBackupWorkloadLabels builds the labels for workload which owned by backup.
func BuildBackupWorkloadLabels(backup *dpv1alpha1.Backup) map[string]string {
	labels := map[string]string{}
	excludeLabels := excludeLabelsForWorkload()
	for k, v := range backup.Labels {
		if slices.Contains(excludeLabels, k) {
			continue
		}
		labels[k] = v
	}
	labels[types.BackupNameLabelKey] = backup.Name
	return labels
}

func buildBackupJobObjMeta(backup *dpv1alpha1.Backup, prefix string) *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:      GenerateBackupJobName(backup, prefix),
		Namespace: backup.Namespace,
		Labels:    BuildBackupWorkloadLabels(backup),
	}
}

func GenerateBackupJobName(backup *dpv1alpha1.Backup, prefix string) string {
	name := fmt.Sprintf("%s-%s-%s", prefix, backup.Name, backup.UID[:8])
	// job name cannot exceed 63 characters for label name limit.
	if len(name) > 63 {
		return strings.TrimSuffix(name[:63], "-")
	}
	return name
}

func GenerateBackupStatefulSetName(backup *dpv1alpha1.Backup, targetName, prefix string) string {
	name := backup.Name
	// for cluster mode with multiple targets, the statefulSet name should include the target name.
	if targetName != "" {
		name = fmt.Sprintf("%s-%s-%s", prefix, targetName, backup.Name)
	}
	// statefulSet name cannot exceed 52 characters for label name limit as the statefulset controller will
	// add a 10-length suffix to the name to construct the label "controller-revision-hash": "<statefulset_name>-<hash>"
	return strings.TrimSuffix(name[:min(len(name), 52)], "-")
}

func generateBaseCRNameByBackupSchedule(uniqueNameWithBackupSchedule, backupScheduleNS, method string) string {
	name := fmt.Sprintf("%s-%s", uniqueNameWithBackupSchedule, backupScheduleNS)
	if len(name) > 30 {
		name = strings.TrimSuffix(name[:30], "-")
	}
	return fmt.Sprintf("%s-%s", name, method)
}

// GenerateCRNameByBackupSchedule generate a CR name which is created by BackupSchedule, such as CronJob Backup.
func GenerateCRNameByBackupSchedule(backupSchedule *dpv1alpha1.BackupSchedule, method string) string {
	uid := backupSchedule.UID[:8]
	if len(backupSchedule.OwnerReferences) > 0 {
		uid = backupSchedule.OwnerReferences[0].UID[:8]
	}
	uniqueNameWithBackupSchedule := fmt.Sprintf("%s-%s", uid, backupSchedule.Name)
	return generateBaseCRNameByBackupSchedule(uniqueNameWithBackupSchedule, backupSchedule.Namespace, method)
}

// GenerateLegacyCRNameByBackupSchedule generate a legacy CR name which is created by BackupSchedule, such as CronJob Backup.
func GenerateLegacyCRNameByBackupSchedule(backupSchedule *dpv1alpha1.BackupSchedule, method string) string {
	uniqueNameWithBackupSchedule := fmt.Sprintf("%s-%s", backupSchedule.UID[:8], backupSchedule.Name)
	return generateBaseCRNameByBackupSchedule(uniqueNameWithBackupSchedule, backupSchedule.Namespace, method)
}

// BuildBaseBackupPath builds the path to storage backup data in backup repository.
func BuildBaseBackupPath(backup *dpv1alpha1.Backup, repoPathPrefix, pathPrefix string) string {
	backupRootPath := BuildBackupRootPath(backup, repoPathPrefix, pathPrefix)
	// pattern: ${repoPathPrefix}/${namespace}/${pathPrefix}/${backupName}
	return filepath.Join("/", backupRootPath, backup.Name)
}

// BuildBackupRootPath builds the root path to storage backup data in backup repository.
func BuildBackupRootPath(backup *dpv1alpha1.Backup, repoPathPrefix, pathPrefix string) string {
	repoPathPrefix = strings.Trim(repoPathPrefix, "/")
	pathPrefix = strings.Trim(pathPrefix, "/")
	// pattern: ${repoPathPrefix}/${namespace}/${pathPrefix}
	return filepath.Join("/", repoPathPrefix, backup.Namespace, pathPrefix)
}

// BuildBackupPathByTarget builds the backup by target.name and podSelectionStrategy.
func BuildBackupPathByTarget(backup *dpv1alpha1.Backup,
	target *dpv1alpha1.BackupTarget,
	repoPathPrefix,
	pathPrefix,
	targetPodName string) string {
	baseBackupPath := BuildBaseBackupPath(backup, repoPathPrefix, pathPrefix)
	targetRelativePath := BuildTargetRelativePath(target, targetPodName)
	return filepath.Join("/", baseBackupPath, targetRelativePath)
}

// BuildTargetRelativePath builds the relative path by target.name and podSelectionStrategy.
func BuildTargetRelativePath(target *dpv1alpha1.BackupTarget, targetPodName string) string {
	path := ""
	if target.Name != "" {
		path = filepath.Join(path, target.Name)
	}
	if target.PodSelector.Strategy == dpv1alpha1.PodSelectionStrategyAll {
		path = filepath.Join(path, targetPodName)
	}
	// return ${DP_TARGET_RELATIVE_PATH}
	return path
}

// BuildKopiaRepoPath builds the path of kopia repository.
func BuildKopiaRepoPath(backup *dpv1alpha1.Backup, repoPathPrefix, pathPrefix string) string {
	repoPathPrefix = strings.Trim(repoPathPrefix, "/")
	pathPrefix = strings.TrimRight(pathPrefix, "/")
	return filepath.Join("/", repoPathPrefix, backup.Namespace, pathPrefix, types.KopiaRepoFolderName)
}

func GetSchedulePolicyByMethod(backupSchedule *dpv1alpha1.BackupSchedule, method string) *dpv1alpha1.SchedulePolicy {
	for _, s := range backupSchedule.Spec.Schedules {
		if s.BackupMethod == method {
			return &s
		}
	}
	return nil
}

func SetExpirationByCreationTime(backup *dpv1alpha1.Backup) error {
	// if expiration is already set, do not update it.
	if backup.Status.Expiration != nil {
		return nil
	}

	duration, err := backup.Spec.RetentionPeriod.ToDuration()
	if err != nil {
		return fmt.Errorf("failed to parse retention period %s, %v", backup.Spec.RetentionPeriod, err)
	}

	// if duration is zero, the backup will be kept forever.
	// Do not set expiration time for it.
	if duration.Seconds() == 0 {
		return nil
	}

	var expiration *metav1.Time
	if backup.Status.StartTimestamp != nil {
		expiration = &metav1.Time{
			Time: backup.Status.StartTimestamp.Add(duration),
		}
	} else {
		expiration = &metav1.Time{
			Time: backup.CreationTimestamp.Add(duration),
		}
	}
	backup.Status.Expiration = expiration
	return nil
}

// BuildCronJobSchedule build cron job schedule info based on kubernetes version.
// For kubernetes version >= 1.25, the timeZone field is supported, return timezone.
// Ref https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/#time-zones
//
// For kubernetes version < 1.25 and >= 1.22, the timeZone field is not supported.
// Therefore, we need to set the CRON_TZ environment variable.
// Ref https://github.com/kubernetes/kubernetes/issues/47202#issuecomment-901294870
//
// For kubernetes version < 1.22, the CRON_TZ environment variable is not supported.
// The kube-controller-manager interprets schedules relative to its local time zone.
func BuildCronJobSchedule(cronExpression string) (*string, string) {
	timeZone := "UTC"
	ver, err := dputils.GetKubeVersion()
	if err != nil {
		return nil, cronExpression
	}
	if semver.Compare(ver, "v1.25") >= 0 {
		return &timeZone, cronExpression
	}
	if semver.Compare(ver, "v1.22") < 0 {
		return nil, cronExpression
	}
	return nil, fmt.Sprintf("CRON_TZ=%s %s", timeZone, cronExpression)
}

// StopStatefulSetsWhenFailed stops the sts to un-bound the pvcs.
func StopStatefulSetsWhenFailed(ctx context.Context, cli client.Client, backup *dpv1alpha1.Backup, targetName string) error {
	if backup.Status.Phase != dpv1alpha1.BackupPhaseFailed {
		return nil
	}
	sts := &appsv1.StatefulSet{}
	stsName := GenerateBackupStatefulSetName(backup, targetName, BackupDataJobNamePrefix)
	if err := cli.Get(ctx, client.ObjectKey{Name: stsName, Namespace: backup.Namespace}, sts); client.IgnoreNotFound(err) != nil {
		return nil
	}
	sts.Spec.Replicas = pointer.Int32(0)
	return cli.Update(ctx, sts)
}
