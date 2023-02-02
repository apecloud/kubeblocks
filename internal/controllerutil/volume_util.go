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

import corev1 "k8s.io/api/core/v1"

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

func CheckAndUpdateVolume(volumes []corev1.Volume, volumeName string, createFn createVolumeFn, updateFn updateVolumeFn) ([]corev1.Volume, error) {
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
