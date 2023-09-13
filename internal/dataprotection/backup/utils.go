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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/dataprotection/types"
)

// GetBackupPolicy returns the BackupPolicy with the given namespace and name.
func GetBackupPolicy(ctx context.Context, cli client.Client, namespace, name string) (*dpv1alpha1.BackupPolicy, error) {
	backupPolicy := &dpv1alpha1.BackupPolicy{}
	if err := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, backupPolicy); err != nil {
		return nil, err
	}
	return backupPolicy, nil
}

func GetActionSet(ctx context.Context, cli client.Client, namespace, name string) (*dpv1alpha1.ActionSet, error) {
	actionSet := &dpv1alpha1.ActionSet{}
	if err := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, actionSet); err != nil {
		return nil, err
	}
	return actionSet, nil
}

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

func getPVCsByVolumeNames(cli client.Client,
	pod *corev1.Pod,
	volumeNames []string) ([]corev1.PersistentVolumeClaim, error) {
	var all []corev1.PersistentVolumeClaim
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
			all = append(all, *tmp.DeepCopy())
		}
	}
	return all, nil
}

func backupRepoVolumeName(pvcName string) string {
	return fmt.Sprintf("dp-backup-%s", pvcName)
}

func buildBackupRepoVolume(pvcName string) corev1.Volume {
	return corev1.Volume{
		Name: backupRepoVolumeName(pvcName),
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}
}

func buildBackupRepoVolumeMount(pvcName string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      backupRepoVolumeName(pvcName),
		MountPath: types.BackupPathBase,
	}
}

func generateBackupJobName(backup *dpv1alpha1.Backup, prefix string) string {
	name := fmt.Sprintf("%s-%s-%s", prefix, backup.Name, backup.UID[:8])
	// job name cannot exceed 63 characters for label name limit.
	if len(name) > 63 {
		return name[:63]
	}
	return name
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
	labels[types.DataProtectionLabelBackupNameKey] = backup.Name
	return labels
}

func buildBackupJobObjMeta(backup *dpv1alpha1.Backup, prefix string) *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Name:      generateBackupJobName(backup, prefix),
		Namespace: backup.Namespace,
		Labels:    BuildBackupWorkloadLabels(backup),
	}
}

func buildBackupInfoENV(backupDestinationPath string) string {
	return types.BackupPathBase + backupDestinationPath + "/backup.info"
}

func injectBackupVolumeMount(
	pvcName string,
	podSpec *corev1.PodSpec,
	container *corev1.Container) {
	// TODO(ldm): mount multi remote backup volumes
	remoteVolumeName := fmt.Sprintf("backup-%s", pvcName)
	remoteVolume := corev1.Volume{
		Name: remoteVolumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}
	remoteVolumeMount := corev1.VolumeMount{
		Name:      remoteVolumeName,
		MountPath: types.BackupPathBase,
	}
	podSpec.Volumes = append(podSpec.Volumes, remoteVolume)
	container.VolumeMounts = append(container.VolumeMounts, remoteVolumeMount)
}
