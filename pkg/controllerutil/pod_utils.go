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

package controllerutil

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metautil "k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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

// GetContainerByConfigSpec searches for container using the configmap of config from the pod
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
func GetContainerByConfigSpec(podSpec *corev1.PodSpec, configs []appsv1alpha1.ComponentConfigSpec) *corev1.Container {
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

// GetPodContainerWithVolumeMount searches for containers mounting the volume
func GetPodContainerWithVolumeMount(podSpec *corev1.PodSpec, volumeName string) []*corev1.Container {
	containers := podSpec.Containers
	if len(containers) == 0 || volumeName == "" {
		return nil
	}
	return getContainerWithVolumeMount(containers, volumeName)
}

// GetVolumeMountName finds the volume with mount name
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

func GetContainersByConfigmap(containers []corev1.Container, volumeName string, cmName string, filters ...containerNameFilter) []string {
	containerFilter := func(c corev1.Container) bool {
		for _, f := range filters {
			if (len(c.VolumeMounts) == 0 && len(c.EnvFrom) == 0) ||
				f(c.Name) {
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
				goto breakHere
			}
		}
		if cmName == "" {
			continue
		}
		for _, source := range c.EnvFrom {
			if source.ConfigMapRef != nil && source.ConfigMapRef.Name == cmName {
				tmpList = append(tmpList, c.Name)
				break
			}
		}
	breakHere:
	}
	return tmpList
}

func getContainerWithTplList(containers []corev1.Container, configs []appsv1alpha1.ComponentConfigSpec) *corev1.Container {
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

func checkContainerWithVolumeMount(volumeMounts []corev1.VolumeMount, configs []appsv1alpha1.ComponentConfigSpec) bool {
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

// GetCoreNum gets content of Resources.Limits.cpu
func GetCoreNum(container corev1.Container) int64 {
	limits := container.Resources.Limits
	if val, ok := (limits)[corev1.ResourceCPU]; ok {
		return val.Value()
	}
	return 0
}

// GetMemorySize gets content of Resources.Limits.memory
func GetMemorySize(container corev1.Container) int64 {
	limits := container.Resources.Limits
	if val, ok := (limits)[corev1.ResourceMemory]; ok {
		return val.Value()
	}
	return 0
}

// GetRequestMemorySize gets content of Resources.Limits.memory
func GetRequestMemorySize(container corev1.Container) int64 {
	requests := container.Resources.Requests
	if val, ok := (requests)[corev1.ResourceMemory]; ok {
		return val.Value()
	}
	return 0
}

// GetStorageSizeFromPersistentVolume gets content of Resources.Requests.storage
func GetStorageSizeFromPersistentVolume(pvc corev1.PersistentVolumeClaimTemplate) int64 {
	requests := pvc.Spec.Resources.Requests
	if val, ok := (requests)[corev1.ResourceStorage]; ok {
		return val.Value()
	}
	return -1
}

// PodIsReady checks if pod is ready
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

// GetContainerID gets the containerID from pod by name
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
		return 0, false, fmt.Errorf("failed to atoi [%s], error: %v", intOrStr.StrVal, err)
	}
	return v, true, nil
}

// GetPortByPortName gets the Port from pod by name
func GetPortByPortName(containers []corev1.Container, portName string) (int32, error) {
	for _, container := range containers {
		for _, port := range container.Ports {
			if port.Name == portName {
				return port.ContainerPort, nil
			}
		}
	}
	return 0, fmt.Errorf("port %s not found", portName)
}

func GetLorryGRPCPort(pod *corev1.Pod) (int32, error) {
	return GetLorryGRPCPortFromContainers(pod.Spec.Containers)
}

func GetLorryGRPCPortFromContainers(containers []corev1.Container) (int32, error) {
	return GetPortByPortName(containers, constant.LorryGRPCPortName)
}

func GetLorryHTTPPort(pod *corev1.Pod) (int32, error) {
	return GetLorryHTTPPortFromContainers(pod.Spec.Containers)
}

func GetLorryHTTPPortFromContainers(containers []corev1.Container) (int32, error) {
	return GetPortByPortName(containers, constant.LorryHTTPPortName)
}

// GetLorryContainerName gets the lorry container from pod
func GetLorryContainerName(pod *corev1.Pod) (string, error) {
	container := GetLorryContainer(pod.Spec.Containers)
	if container != nil {
		return container.Name, nil
	}
	return "", fmt.Errorf("lorry container not found")
}

func GetLorryContainer(containers []corev1.Container) *corev1.Container {
	var container *corev1.Container
	for i := range containers {
		container = &containers[i]
		for _, port := range container.Ports {
			if port.Name == constant.LorryHTTPPortName {
				return container
			}
		}
	}
	return nil
}

// PodIsReadyWithLabel checks if pod is ready for ConsensusSet/ReplicationSet component,
// it will be available when the pod is ready and labeled with role.
func PodIsReadyWithLabel(pod corev1.Pod) bool {
	if _, ok := pod.Labels[constant.RoleLabelKey]; !ok {
		return false
	}

	return PodIsReady(&pod)
}

// PodIsControlledByLatestRevision checks if the pod is controlled by latest controller revision.
func PodIsControlledByLatestRevision(pod *corev1.Pod, sts *appsv1.StatefulSet) bool {
	return GetPodRevision(pod) == sts.Status.UpdateRevision && sts.Status.ObservedGeneration == sts.Generation
}

// GetPodRevision gets the revision of Pod by inspecting the StatefulSetRevisionLabel. If pod has no revision empty
// string is returned.
func GetPodRevision(pod *corev1.Pod) string {
	if pod.Labels == nil {
		return ""
	}
	return pod.Labels[appsv1.StatefulSetRevisionLabel]
}

// ByPodName sorts a list of jobs by pod name
type ByPodName []corev1.Pod

// Len returns the length of byPodName for sort.Sort
func (c ByPodName) Len() int {
	return len(c)
}

// Swap swaps the items for sort.Sort
func (c ByPodName) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

// Less defines compare method for sort.Sort
func (c ByPodName) Less(i, j int) bool {
	return c[i].Name < c[j].Name
}

// BuildPodHostDNS builds the host dns of pod.
// ref: https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/
func BuildPodHostDNS(pod *corev1.Pod) string {
	if pod == nil {
		return ""
	}
	// build pod dns string
	// ref: https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/
	if pod.Spec.Subdomain != "" {
		hostDNS := []string{pod.Name}
		if pod.Spec.Hostname != "" {
			hostDNS[0] = pod.Spec.Hostname
		}
		hostDNS = append(hostDNS, pod.Spec.Subdomain)
		return strings.Join(hostDNS, ".")
	}
	return pod.Status.PodIP
}
