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

package rsm2

import (
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
	"github.com/rogpeppe/go-internal/semver"
	corev1 "k8s.io/api/core/v1"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/kubernetes/pkg/features"
	"reflect"

	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
)

type PodUpdatePolicy string

const (
	NoOpsPolicy         PodUpdatePolicy = "NoOps"
	RecreatePolicy      PodUpdatePolicy = "Recreate"
	InPlaceUpdatePolicy PodUpdatePolicy = "InPlaceUpdate"
)

func supportPodVerticalScaling() (bool, error) {
	kubeVersion, err := utils.GetKubeVersion()
	if err != nil {
		return false, err
	}
	if semver.Compare(kubeVersion, "1.29") >= 0 {
		return true, nil
	}

	enabled := utilfeature.DefaultFeatureGate.Enabled(features.InPlacePodVerticalScaling)
	return enabled, nil
}

func filterInPlaceFields(src *corev1.PodTemplateSpec) *corev1.PodTemplateSpec {
	template := src.DeepCopy()
	// filter annotations
	template.Annotations = nil
	// filter labels
	template.Labels = nil
	// filter spec.containers[*].images & spec.initContainers[*].images
	for i := range template.Spec.Containers {
		template.Spec.Containers[i].Image = ""
	}
	for i := range template.Spec.InitContainers {
		template.Spec.InitContainers[i].Image = ""
	}
	// filter spec.activeDeadlineSeconds
	template.Spec.ActiveDeadlineSeconds = nil
	// filter spec.tolerations
	template.Spec.Tolerations = nil
	// filter spec.containers[*].resources["cpu|memory"]
	for i := range template.Spec.Containers {
		delete(template.Spec.Containers[i].Resources.Requests, corev1.ResourceCPU)
		delete(template.Spec.Containers[i].Resources.Requests, corev1.ResourceMemory)
		delete(template.Spec.Containers[i].Resources.Limits, corev1.ResourceCPU)
		delete(template.Spec.Containers[i].Resources.Limits, corev1.ResourceMemory)
	}

	return template
}

func copyBasicInPlaceFields(src *corev1.Pod) *corev1.Pod {
	var (
		containers     []corev1.Container
		initContainers []corev1.Container
	)
	for _, container := range src.Spec.Containers {
		containers = append(containers, corev1.Container{Image: container.Image})
	}
	for _, container := range src.Spec.InitContainers {
		initContainers = append(initContainers, corev1.Container{Image: container.Image})
	}
	dst := builder.NewPodBuilder("", "").
		AddAnnotationsInMap(src.Annotations).
		AddLabelsInMap(src.Labels).
		AddTolerations(src.Spec.Tolerations...).
		SetActiveDeadlineSeconds(src.Spec.ActiveDeadlineSeconds).
		SetInitContainers(initContainers).
		SetContainers(containers).
		GetObject()
	return dst

}

func copyRequestsNLimitsFields(container *corev1.Container) (corev1.ResourceList, corev1.ResourceList) {
	requests := make(corev1.ResourceList)
	limits := make(corev1.ResourceList)
	if len(container.Resources.Requests) > 0 {
		if requestCPU, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
			requests[corev1.ResourceCPU] = requestCPU
		}
		if requestMemory, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
			requests[corev1.ResourceMemory] = requestMemory
		}
	}
	if len(container.Resources.Limits) > 0 {
		if limitCPU, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
			limits[corev1.ResourceCPU] = limitCPU
		}
		if limitMemory, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
			limits[corev1.ResourceMemory] = limitMemory
		}
	}
	return requests, limits
}

func copyResourceInPlaceFields(src *corev1.Pod) *corev1.Pod {
	var containers []corev1.Container
	for _, container := range src.Spec.Containers {
		requests, limits := copyRequestsNLimitsFields(&container)
		resources := corev1.ResourceRequirements{
			Requests: requests,
			Limits:   limits,
		}
		containers = append(containers, corev1.Container{Resources: resources})
	}
	return builder.NewPodBuilder("", "").SetContainers(containers).GetObject()
}

func mergeInPlaceFields(src, dst *corev1.Pod) {
	mergeMap(&src.Annotations, &dst.Annotations)
	mergeMap(&src.Labels, &dst.Labels)
	dst.Spec.ActiveDeadlineSeconds = src.Spec.ActiveDeadlineSeconds
	mergeList(&src.Spec.Tolerations, &dst.Spec.Tolerations, func(item corev1.Toleration) func(corev1.Toleration) bool {
		return func(t corev1.Toleration) bool {
			return false
		}
	})
	for _, container := range src.Spec.InitContainers {
		for i, c := range dst.Spec.InitContainers {
			if container.Name == c.Name {
				dst.Spec.InitContainers[i].Image = container.Image
			}
		}
	}
	mergeResources := func(src, dst *corev1.ResourceList) {
		if len(*src) == 0 {
			return
		}
		if *dst == nil {
			*dst = make(corev1.ResourceList)
		}
		for k, v := range *src {
			(*dst)[k] = v
		}
	}
	ignorePodVerticalScaling := viper.GetBool(FeatureGateIgnorePodVerticalScaling)
	for _, container := range src.Spec.Containers {
		for i, c := range dst.Spec.Containers {
			if container.Name == c.Name {
				dst.Spec.Containers[i].Image = container.Image
				if !ignorePodVerticalScaling {
					requests, limits := copyRequestsNLimitsFields(&container)
					mergeResources(&requests, &dst.Spec.Containers[i].Resources.Requests)
					mergeResources(&limits, &dst.Spec.Containers[i].Resources.Limits)
				}
				break
			}
		}
	}
}

func shouldDoPodUpdate(old, new *corev1.Pod, updateRevisions map[string]string) (PodUpdatePolicy, error) {
	if getPodRevision(old) != updateRevisions[old.Name] {
		return RecreatePolicy, nil
	}

	ignorePodVerticalScaling := viper.GetBool(FeatureGateIgnorePodVerticalScaling)
	oldInPlace := copyBasicInPlaceFields(old)
	newInPlace := copyBasicInPlaceFields(new)
	basicUpdate := reflect.DeepEqual(oldInPlace, newInPlace)
	if ignorePodVerticalScaling {
		if basicUpdate {
			return InPlaceUpdatePolicy, nil
		}
		return NoOpsPolicy, nil
	}

	oldInPlace = copyResourceInPlaceFields(old)
	newInPlace = copyResourceInPlaceFields(new)
	resourceUpdate := reflect.DeepEqual(oldInPlace, newInPlace)
	if resourceUpdate {
		supportVerticalScaling, err := supportPodVerticalScaling()
		if err != nil {
			return NoOpsPolicy, err
		}
		if supportVerticalScaling {
			return InPlaceUpdatePolicy, nil
		}
		return RecreatePolicy, nil
	}
	if basicUpdate {
		return InPlaceUpdatePolicy, nil
	}
	return NoOpsPolicy, nil
}
