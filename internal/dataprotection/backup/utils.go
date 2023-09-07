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
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
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

func backupRepoVolumeName(pvcName string) string {
	return fmt.Sprintf("backup-%s", pvcName)
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
