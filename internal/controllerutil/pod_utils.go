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
	"regexp"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metautil "k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

// statefulPodRegex is a regular expression that extracts the parent StatefulSet and ordinal from the Name of a Pod
var statefulPodRegex = regexp.MustCompile("(.*)-([0-9]+)$")

// GetParentNameAndOrdinal gets the name of pod's parent StatefulSet and pod's ordinal as extracted from its Name. If
// the Pod was not created by a StatefulSet, its parent is considered to be empty string, and its ordinal is considered
// to be -1.
func GetParentNameAndOrdinal(pod *corev1.Pod) (string, int) {
	parent := ""
	ordinal := -1
	subMatches := statefulPodRegex.FindStringSubmatch(pod.Name)
	if len(subMatches) < 3 {
		return parent, ordinal
	}
	parent = subMatches[1]
	if i, err := strconv.ParseInt(subMatches[2], 10, 32); err == nil {
		ordinal = int(i)
	}
	return parent, ordinal
}

// GetContainerByConfigTemplate function description:
// Search the container using the configmap of config from the pod
//
// Return: The first container pointer of using configs
//
//	e.g.:
//	ClusterDefinition.configTemplateRef:
//		 - Name: "mysql-8.0"
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
func GetContainerByConfigTemplate(podSpec *corev1.PodSpec, configs []appsv1alpha1.ConfigTemplate) *corev1.Container {
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
func GetPodContainerWithVolumeMount(podSpec *corev1.PodSpec, volumeName string) []*corev1.Container {
	containers := podSpec.Containers
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

type containerNameFilter func(containerName string) bool

func GetContainersByConfigmap(containers []corev1.Container, volumeName string, filters ...containerNameFilter) []string {
	containerFilter := func(c corev1.Container) bool {
		for _, f := range filters {
			if len(c.VolumeMounts) == 0 || f(c.Name) {
				return true
			}
		}
		return false
	}

	tmpList := make([]string, 0, len(containers))
	for _, c := range containers {
		if containerFilter(c) {
			continue
		}
		for _, vm := range c.VolumeMounts {
			if vm.Name == volumeName {
				tmpList = append(tmpList, c.Name)
				break
			}
		}
	}
	return tmpList
}

func getContainerWithTplList(containers []corev1.Container, configs []appsv1alpha1.ConfigTemplate) *corev1.Container {
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

func checkContainerWithVolumeMount(volumeMounts []corev1.VolumeMount, configs []appsv1alpha1.ConfigTemplate) bool {
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

func GetVolumeMountByVolume(container *corev1.Container, volumeName string) *corev1.VolumeMount {
	for _, volume := range container.VolumeMounts {
		if volume.Name == volumeName {
			return &volume
		}
	}

	return nil
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

// GetContainerID find the containerID from pod by name
func GetContainerID(pod *corev1.Pod, containerName string) string {
	const containerSep = "//"

	// container id is present in the form of <runtime>://<container-id>
	// e.g: containerID: docker://27d1586d53ef9a6af5bd983831d13b6a38128119fadcdc22894d7b2397758eb5
	for _, container := range pod.Status.ContainerStatuses {
		if container.Name == containerName {
			return strings.Split(container.ContainerID, containerSep)[1]
		}
	}
	return ""
}

func isRunning(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning && pod.DeletionTimestamp == nil
}

func IsReady(pod *corev1.Pod) bool {
	if !isRunning(pod) {
		return false
	}

	condition := GetPodCondition(&pod.Status, corev1.PodReady)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

func IsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if !isRunning(pod) {
		return false
	}

	condition := GetPodCondition(&pod.Status, corev1.PodReady)
	if condition == nil || condition.Status != corev1.ConditionTrue {
		return false
	}
	if minReadySeconds == 0 {
		return true
	}

	var (
		now                = metav1.Now()
		minDuration        = time.Duration(minReadySeconds) * time.Second
		lastTransitionTime = condition.LastTransitionTime
	)

	return !lastTransitionTime.IsZero() && lastTransitionTime.Add(minDuration).Before(now.Time)
}

func GetPodCondition(status *corev1.PodStatus, conditionType corev1.PodConditionType) *corev1.PodCondition {
	if len(status.Conditions) == 0 {
		return nil
	}

	for i, condition := range status.Conditions {
		if condition.Type == conditionType {
			return &status.Conditions[i]
		}
	}
	return nil
}

func IsMatchConfigVersion(obj client.Object, labelKey string, version string) bool {
	labels := obj.GetLabels()
	if len(labels) == 0 {
		return false
	}
	if lastVersion, ok := labels[labelKey]; ok && lastVersion == version {
		return true
	}
	return false
}

func GetIntOrPercentValue(intOrStr *metautil.IntOrString) (int, bool, error) {
	if intOrStr.Type == metautil.Int {
		return intOrStr.IntValue(), false, nil
	}

	// parse string
	s := intOrStr.StrVal
	if strings.HasSuffix(s, "%") {
		s = strings.TrimSuffix(intOrStr.StrVal, "%")
	} else {
		return 0, false, fmt.Errorf("failed to parse percentage. [%s]", intOrStr.StrVal)
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, false, fmt.Errorf("failed to atoi. [%s], error: %v", intOrStr.StrVal, err)
	}
	return v, true, nil
}

func ExistsPDBSpec(pdbSpec *policyv1.PodDisruptionBudgetSpec) bool {
	if pdbSpec == nil {
		return false
	}
	if pdbSpec.MinAvailable == nil && pdbSpec.MaxUnavailable == nil {
		return false
	}
	return true
}
