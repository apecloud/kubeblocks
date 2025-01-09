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

package controllerutil

import (
	"fmt"
	"slices"
	"sort"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type createVolumeFn func(volumeName string) corev1.Volume
type updateVolumeFn func(*corev1.Volume) error

var (
	scriptsDefaultMode int32 = 0555
	configsDefaultMode int32 = 0444
)

func findVolumeWithVolumeName(volumes []corev1.Volume, volumeName string) int {
	for index, itr := range volumes {
		if itr.Name == volumeName {
			return index
		}
	}
	return -1
}

func CreateOrUpdateVolume(volumes []corev1.Volume, volumeName string, createFn createVolumeFn, updateFn updateVolumeFn) ([]corev1.Volume, error) {
	// for update volume
	if existIndex := findVolumeWithVolumeName(volumes, volumeName); existIndex >= 0 {
		if updateFn == nil {
			return volumes, nil
		}
		if err := updateFn(&volumes[existIndex]); err != nil {
			return volumes, err
		}
		return volumes, nil
	}

	// for create volume
	return append(volumes, createFn(volumeName)), nil
}

func CreateOrUpdatePodVolumes(podSpec *corev1.PodSpec, volumes map[string]appsv1.ComponentTemplateSpec, configSet []string) error {
	var (
		err        error
		podVolumes = podSpec.Volumes
		volumeKeys = maps.Keys(volumes)
	)

	// sort the volumes
	sort.Strings(volumeKeys)
	// Update PodTemplate Volumes
	for _, cmName := range volumeKeys {
		templateSpec := volumes[cmName]
		if templateSpec.VolumeName == "" {
			continue
		}
		if podVolumes, err = CreateOrUpdateVolume(podVolumes, templateSpec.VolumeName, func(volumeName string) corev1.Volume {
			return corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
						// TODO: remove ComponentTemplateSpec.DefaultMode
						DefaultMode: BuildVolumeMode(configSet, templateSpec),
					},
				},
			}
		}, func(volume *corev1.Volume) error {
			configMap := volume.ConfigMap
			if configMap == nil {
				return fmt.Errorf("mount volume[%s] requires a ConfigMap: [%+v]", volume.Name, volume)
			}
			configMap.Name = cmName
			return nil
		}); err != nil {
			return err
		}
	}
	podSpec.Volumes = podVolumes
	return nil
}

func BuildVolumeMode(configs []string, configSpec appsv1.ComponentTemplateSpec) *int32 {
	// If the defaultMode is not set, permissions are automatically set based on the template type.
	if !viper.GetBool(constant.FeatureGateIgnoreConfigTemplateDefaultMode) && configSpec.DefaultMode != nil {
		return configSpec.DefaultMode
	}
	if slices.Contains(configs, configSpec.Name) {
		return &configsDefaultMode
	}
	return &scriptsDefaultMode
}

func ToCoreV1PVCs(vcts []appsv1.ClusterComponentVolumeClaimTemplate) []corev1.PersistentVolumeClaim {
	storageClassName := func(spec appsv1.PersistentVolumeClaimSpec, defaultStorageClass string) *string {
		if spec.StorageClassName != nil && *spec.StorageClassName != "" {
			return spec.StorageClassName
		}
		if defaultStorageClass != "" {
			return &defaultStorageClass
		}
		return nil
	}
	var pvcs []corev1.PersistentVolumeClaim
	for _, v := range vcts {
		pvcs = append(pvcs, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: v.Name,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      v.Spec.AccessModes,
				Resources:        v.Spec.Resources,
				StorageClassName: storageClassName(v.Spec, viper.GetString(constant.CfgKeyDefaultStorageClass)),
				VolumeMode:       v.Spec.VolumeMode,
			},
		})
	}
	return pvcs
}
