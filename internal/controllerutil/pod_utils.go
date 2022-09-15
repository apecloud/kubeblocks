/*
Copyright 2022.

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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

func GetContainerUsingConfig(sts *appsv1.StatefulSet, configs []dbaasv1alpha1.ConfigTemplate) *corev1.Container {
	// volumes := sts.Spec.Template.Spec.Volumes
	containers := sts.Spec.Template.Spec.Containers
	initContainers := sts.Spec.Template.Spec.InitContainers

	if container := GetContainerWithTplList(containers, configs); container != nil {
		return container
	}

	if container := GetContainerWithTplList(initContainers, configs); container != nil {
		return container
	}

	return nil
}

func GetContainerWithTplList(containers []corev1.Container, configs []dbaasv1alpha1.ConfigTemplate) *corev1.Container {
	if len(containers) == 0 {
		return nil
	}

	for i := range containers {
		volumeMounts := containers[i].VolumeMounts
		if len(volumeMounts) > 0 && checkContainerWithVolumeMount(volumeMounts, configs) {
			return &containers[i]
		}
	}

	return nil
}

func GetContainerWithVolumeMount(containers []corev1.Container, volumeMountName string) *corev1.Container {
	for i := range containers {
		volumeMounts := containers[i].VolumeMounts
		for j := range volumeMounts {
			if volumeMounts[j].Name == volumeMountName {
				return &containers[i]
			}
		}
	}

	return nil
}

func checkContainerWithVolumeMount(volumeMounts []corev1.VolumeMount, configs []dbaasv1alpha1.ConfigTemplate) bool {
	volumes := make(map[string]int)
	for i := range configs {
		for j := range volumeMounts {
			if volumeMounts[j].Name == configs[i].VolumeName {
				volumes[volumeMounts[j].Name] = j
				break
			}
		}
	}

	return len(configs) == len(volumes)
}

func GetVolumeMountName(volumes []corev1.Volume, resourceName string) *corev1.Volume {
	for i := range volumes {
		if volumes[i].ConfigMap != nil && volumes[i].ConfigMap.Name == resourceName {
			return &volumes[i]
		}

		if volumes[i].Projected != nil {
			for j := range volumes[i].Projected.Sources {
				if volumes[i].Projected.Sources[j].ConfigMap != nil && volumes[i].Projected.Sources[j].ConfigMap.Name == resourceName {
					return &volumes[i]
				}
			}
		}
	}

	return nil
}

func GetCoreNum(limits corev1.ResourceList) int {
	// TODO cal cpu
	return -1
}

func GetMemorySize(limits corev1.ResourceList) int {
	// TODO cal MemorySize
	return -1
}
