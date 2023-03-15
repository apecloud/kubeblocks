/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllerutil

import (
	"fmt"
	"sort"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type createVolumeFn func(volumeName string) corev1.Volume
type updateVolumeFn func(*corev1.Volume) error

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

func CreateOrUpdatePodVolumes(podSpec *corev1.PodSpec, volumes map[string]appsv1alpha1.ComponentTemplateSpec) error {
	var (
		err        error
		podVolumes = podSpec.Volumes
	)
	// sort the volumes
	volumeKeys := maps.Keys(volumes)
	sort.Strings(volumeKeys)
	// Update PodTemplate Volumes
	for _, cmName := range volumeKeys {
		templateSpec := volumes[cmName]
		if podVolumes, err = CreateOrUpdateVolume(podVolumes, templateSpec.VolumeName, func(volumeName string) corev1.Volume {
			return corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
						DefaultMode:          templateSpec.DefaultMode,
					},
				},
			}
		}, func(volume *corev1.Volume) error {
			configMap := volume.ConfigMap
			if configMap == nil {
				return fmt.Errorf("mount volume[%s] type require ConfigMap: [%+v]", volume.Name, volume)
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
