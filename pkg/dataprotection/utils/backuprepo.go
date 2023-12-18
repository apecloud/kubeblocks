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

package utils

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	datasafedImageEnv        = "DATASAFED_IMAGE"
	defaultDatasafedImage    = "apecloud/datasafed:latest"
	datasafedBinMountPath    = "/bin/datasafed"
	datasafedConfigMountPath = "/etc/datasafed"
)

func InjectDatasafed(podSpec *corev1.PodSpec, repo *dpv1alpha1.BackupRepo, repoVolumeMountPath string, backupPath string) {
	if repo.AccessByMount() {
		InjectDatasafedWithPVC(podSpec, repo.Status.BackupPVCName, repoVolumeMountPath, backupPath)
	} else if repo.AccessByTool() {
		InjectDatasafedWithConfig(podSpec, repo.Status.ToolConfigSecretName, backupPath)
	}
}

func InjectDatasafedWithPVC(podSpec *corev1.PodSpec, pvcName string, mountPath string, backupPath string) {
	volumeName := "dp-backup-data"
	volume := corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}
	volumeMount := corev1.VolumeMount{
		Name:      volumeName,
		MountPath: mountPath,
	}
	envs := []corev1.EnvVar{
		{
			// force datasafed to use local backend with the path
			Name:  dptypes.DPDatasafedLocalBackendPath,
			Value: mountPath,
		},
	}
	injectElements(podSpec, toSlice(volume), toSlice(volumeMount), envs)
	injectDatasafedInstaller(podSpec)
}

func InjectDatasafedWithConfig(podSpec *corev1.PodSpec, configSecretName string, backupPath string) {
	volumeName := "dp-datasafed-config"
	volume := corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: configSecretName,
			},
		},
	}
	volumeMount := corev1.VolumeMount{
		Name:      volumeName,
		ReadOnly:  true,
		MountPath: datasafedConfigMountPath,
	}
	injectElements(podSpec, toSlice(volume), toSlice(volumeMount), nil)
	injectDatasafedInstaller(podSpec)
}

func injectDatasafedInstaller(podSpec *corev1.PodSpec) {
	sharedVolumeName := "dp-datasafed-bin"
	sharedVolume := corev1.Volume{
		Name: sharedVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	sharedVolumeMount := corev1.VolumeMount{
		Name:      sharedVolumeName,
		MountPath: datasafedBinMountPath,
	}
	env := corev1.EnvVar{
		Name:  dptypes.DPDatasafedBinPath,
		Value: datasafedBinMountPath,
	}

	// copy the datasafed binary from the image to the shared volume
	datasafedImage := viper.GetString(datasafedImageEnv)
	if datasafedImage == "" {
		datasafedImage = defaultDatasafedImage
	}
	initContainer := corev1.Container{
		Name:            "dp-copy-datasafed",
		Image:           datasafedImage,
		ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy)),
		Command:         []string{"/bin/sh", "-c", fmt.Sprintf("/scripts/install-datasafed.sh %s", datasafedBinMountPath)},
		VolumeMounts:    []corev1.VolumeMount{sharedVolumeMount},
	}
	intctrlutil.InjectZeroResourcesLimitsIfEmpty(&initContainer)
	podSpec.InitContainers = append(podSpec.InitContainers, initContainer)
	injectElements(podSpec, toSlice(sharedVolume), toSlice(sharedVolumeMount), toSlice(env))
}

func injectElements(podSpec *corev1.PodSpec, volumes []corev1.Volume, volumeMounts []corev1.VolumeMount, envs []corev1.EnvVar) {
	podSpec.Volumes = append(podSpec.Volumes, volumes...)
	for i := range podSpec.Containers {
		container := &podSpec.Containers[i]
		container.VolumeMounts = append(container.VolumeMounts, volumeMounts...)
		container.Env = append(container.Env, envs...)
	}
}

func toSlice[T any](s ...T) []T {
	return s
}
