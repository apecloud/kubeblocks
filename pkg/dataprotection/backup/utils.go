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

package backup

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/action"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/types"
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
	labels := backup.Labels
	if labels == nil {
		labels = map[string]string{}
	} else {
		for _, v := range excludeLabelsForWorkload() {
			delete(labels, v)
		}
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
		return name[:63]
	}
	return name
}

// GenerateCRNameByBackupSchedule generate a CR name which is created by BackupSchedule, such as CronJob Backup.
func GenerateCRNameByBackupSchedule(backupSchedule *dpv1alpha1.BackupSchedule, method string) string {
	name := fmt.Sprintf("%s-%s", generateUniqueNameWithBackupSchedule(backupSchedule), backupSchedule.Namespace)
	if len(name) > 30 {
		name = strings.TrimRight(name[:30], "-")
	}
	return fmt.Sprintf("%s-%s", name, method)
}

func generateUniqueNameWithBackupSchedule(backupSchedule *dpv1alpha1.BackupSchedule) string {
	uniqueName := backupSchedule.Name
	if len(backupSchedule.OwnerReferences) > 0 {
		uniqueName = fmt.Sprintf("%s-%s", backupSchedule.OwnerReferences[0].UID[:8], backupSchedule.OwnerReferences[0].Name)
	}
	return uniqueName
}

// BuildBackupPath builds the path to storage backup data in backup repository.
func BuildBackupPath(backup *dpv1alpha1.Backup, pathPrefix string) string {
	pathPrefix = strings.TrimRight(pathPrefix, "/")
	if strings.TrimSpace(pathPrefix) == "" || strings.HasPrefix(pathPrefix, "/") {
		return fmt.Sprintf("/%s%s/%s", backup.Namespace, pathPrefix, backup.Name)
	}
	return fmt.Sprintf("/%s/%s/%s", backup.Namespace, pathPrefix, backup.Name)
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
