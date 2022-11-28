/*
Copyright ApeCloud Inc.

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
	corev1 "k8s.io/api/core/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

// GetContainerUsingConfig function description:
// Search the container using the configmap of config from the pod
//
// Return: The first container pointer of using configs
//
//	e.g.:
//	ClusterDefinition.configTemplateRef:
//		 - Name: "mysql-8.0-config"
//		   VolumeName: "mysql_config"
//
//
//	PodTemplate.containers[*].volumeMounts:
//		 - mountPath: /data/config
//		   name: mysql_config
//		 - mountPath: /data
//		   name: data
//		 - mountPath: /log
//		   name: log
func GetContainerUsingConfig(podSpec *corev1.PodSpec, configs []dbaasv1alpha1.ConfigTemplate) *corev1.Container {
	// volumes := podSpec.Volumes
	containers := podSpec.Containers
	initContainers := podSpec.InitContainers
	if container := getContainerWithTplList(containers, configs); container != nil {
		return container
	}
	if container := getContainerWithTplList(initContainers, configs); container != nil {
		return container
	}
	return nil
}

// GetPodContainerWithVolumeMount function description:
// Search which containers mounting the volume
//
// Case: When the configmap update, we restart all containers who using configmap
//
// Return: all containers mount volumeName
func GetPodContainerWithVolumeMount(pod *corev1.Pod, volumeName string) []*corev1.Container {
	containers := pod.Spec.Containers
	if len(containers) == 0 {
		return nil
	}
	return getContainerWithVolumeMount(containers, volumeName)
}

// GetVolumeMountName function description:
// Find the volume of pod using name of cm
//
// Case: When the configmap object of configuration is modified by user, we need to query whose volumeName
//
// Return: The volume pointer of pod
func GetVolumeMountName(volumes []corev1.Volume, resourceName string) *corev1.Volume {
	for i := range volumes {
		if volumes[i].ConfigMap != nil && volumes[i].ConfigMap.Name == resourceName {
			return &volumes[i]
		}
		if volumes[i].Projected == nil {
			continue
		}
		for j := range volumes[i].Projected.Sources {
			if volumes[i].Projected.Sources[j].ConfigMap != nil && volumes[i].Projected.Sources[j].ConfigMap.Name == resourceName {
				return &volumes[i]
			}
		}
	}
	return nil
}

func getContainerWithTplList(containers []corev1.Container, configs []dbaasv1alpha1.ConfigTemplate) *corev1.Container {
	if len(containers) == 0 {
		return nil
	}
	for i, c := range containers {
		volumeMounts := c.VolumeMounts
		if len(volumeMounts) > 0 && checkContainerWithVolumeMount(volumeMounts, configs) {
			return &containers[i]
		}
	}
	return nil
}

func checkContainerWithVolumeMount(volumeMounts []corev1.VolumeMount, configs []dbaasv1alpha1.ConfigTemplate) bool {
	volumes := make(map[string]int)
	for _, c := range configs {
		for j, vm := range volumeMounts {
			if vm.Name == c.VolumeName {
				volumes[vm.Name] = j
				break
			}
		}
	}
	return len(configs) == len(volumes)
}

func getContainerWithVolumeMount(containers []corev1.Container, volumeName string) []*corev1.Container {
	mountContainers := make([]*corev1.Container, 0, len(containers))
	for i, c := range containers {
		volumeMounts := c.VolumeMounts
		for _, vm := range volumeMounts {
			if vm.Name == volumeName {
				mountContainers = append(mountContainers, &containers[i])
				break
			}
		}
	}
	return mountContainers
}

// GetCoreNum function description:
// if not Resource field return 0 else Resources.Limits.cpu
func GetCoreNum(container corev1.Container) int64 {
	limits := container.Resources.Limits
	if val, ok := (limits)[corev1.ResourceCPU]; ok {
		return val.Value()
	}
	return 0
}

// GetMemorySize function description:
// if not Resource field, return 0 else Resources.Limits.memory
func GetMemorySize(container corev1.Container) int64 {
	limits := container.Resources.Limits
	if val, ok := (limits)[corev1.ResourceMemory]; ok {
		return val.Value()
	}
	return 0
}

// PodIsReady check the pod is ready
func PodIsReady(pod *corev1.Pod) bool {
	if pod.Status.Conditions == nil {
		return false
	}

	if pod.DeletionTimestamp != nil {
		return false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
