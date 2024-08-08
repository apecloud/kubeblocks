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

package instanceset

import (
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type PodUpdatePolicy string

const (
	NoOpsPolicy         PodUpdatePolicy = "NoOps"
	RecreatePolicy      PodUpdatePolicy = "Recreate"
	InPlaceUpdatePolicy PodUpdatePolicy = "InPlaceUpdate"
)

func supportPodVerticalScaling() bool {
	return viper.GetBool(constant.FeatureGateInPlacePodVerticalScaling)
}

func filterInPlaceFields(src *corev1.PodTemplateSpec) *corev1.PodTemplateSpec {
	template := src.DeepCopy()
	// filter annotations
	var annotations map[string]string
	if len(template.Annotations) > 0 {
		annotations = make(map[string]string)
		// keep Restart annotation
		if restart, ok := template.Annotations[constant.RestartAnnotationKey]; ok {
			annotations[constant.RestartAnnotationKey] = restart
		}
		// keep Reconfigure annotation
		for k, v := range template.Annotations {
			if strings.HasPrefix(k, constant.UpgradeRestartAnnotationKey) {
				annotations[k] = v
			}
		}
		if len(annotations) == 0 {
			annotations = nil
		}
	}
	template.Annotations = annotations
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

func mergeInPlaceFields(src, dst *corev1.Pod) {
	mergeMap(&src.Annotations, &dst.Annotations)
	mergeMap(&src.Labels, &dst.Labels)
	dst.Spec.ActiveDeadlineSeconds = src.Spec.ActiveDeadlineSeconds
	// according to the Pod API spec, tolerations can only be appended.
	// means old tolerations must be in new toleration list.
	intctrlutil.MergeList(&src.Spec.Tolerations, &dst.Spec.Tolerations, func(item corev1.Toleration) func(corev1.Toleration) bool {
		return func(t corev1.Toleration) bool {
			return reflect.DeepEqual(item, t)
		}
	})
	for _, container := range src.Spec.InitContainers {
		for i, c := range dst.Spec.InitContainers {
			if container.Name == c.Name {
				dst.Spec.InitContainers[i].Image = container.Image
				break
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

func equalField(old, new any) bool {
	oType := reflect.TypeOf(old)
	nType := reflect.TypeOf(new)
	if oType.Kind() != nType.Kind() {
		return false
	}
	getQuantity := func(resources corev1.ResourceList, key corev1.ResourceName) (q resource.Quantity) {
		q = resource.MustParse("0")
		if len(resources) == 0 {
			return
		}
		v, ok := resources[key]
		if !ok {
			return
		}
		q = v
		return
	}
	switch o := old.(type) {
	case map[string]string:
		oldMap := o
		newMap, _ := new.(map[string]string)
		if len(newMap) > len(oldMap) {
			return false
		}
		for k, v := range newMap {
			ov, ok := oldMap[k]
			if !ok {
				return false
			}
			if ov != v {
				return false
			}
		}
		return true
	case *int64, string:
		return reflect.DeepEqual(old, new)
	case []corev1.Container:
		ocs := o
		ncs, _ := new.([]corev1.Container)
		if len(ocs) != len(ncs) {
			return false
		}
		for _, nc := range ncs {
			index := slices.IndexFunc(ocs, func(oc corev1.Container) bool {
				return nc.Name == oc.Name
			})
			if index < 0 {
				return false
			}
			if nc.Image != ocs[index].Image {
				return false
			}
		}
		return true
	case []corev1.Toleration:
		return true
	case corev1.ResourceList:
		or := o
		nr, _ := new.(corev1.ResourceList)
		oc := getQuantity(or, corev1.ResourceCPU)
		om := getQuantity(or, corev1.ResourceMemory)
		nc := getQuantity(nr, corev1.ResourceCPU)
		nm := getQuantity(nr, corev1.ResourceMemory)
		return oc.Equal(nc) && om.Equal(nm)
	}
	return false
}

func equalBasicInPlaceFields(old, new *corev1.Pod) bool {
	// Only comparing annotations and labels that are relevant to the new spec.
	// These two fields might be modified by other controllers without the InstanceSet controller knowing.
	// For instance, two new annotations have been added by Patroni.
	// There are two strategies to handle this situation: override or replace.
	// The recreation approach (recreating pod(s) when any field is updated in the pod template) used by StatefulSet/Deployment/DaemonSet
	// is a replacement policy.
	// Here, we use the override policy, which means keeping the annotations or labels that the new instance template doesn't care about during an in-place update.
	if !equalField(old.Annotations, new.Annotations) {
		return false
	}
	if !equalField(old.Labels, new.Labels) {
		return false
	}
	if !equalField(old.Spec.ActiveDeadlineSeconds, new.Spec.ActiveDeadlineSeconds) {
		return false
	}
	if !equalField(old.Spec.InitContainers, new.Spec.InitContainers) {
		return false
	}
	if !equalField(old.Spec.Containers, new.Spec.Containers) {
		return false
	}
	if !equalField(old.Spec.Tolerations, new.Spec.Tolerations) {
		return false
	}
	return true
}

func equalResourcesInPlaceFields(old, new *corev1.Pod) bool {
	if len(old.Spec.Containers) != len(new.Spec.Containers) {
		if len(old.Spec.Containers) < len(new.Spec.Containers) {
			return false
		}
		return isContainerInjected(old.Spec.Containers, new.Spec.Containers)
	}
	for _, nc := range new.Spec.Containers {
		index := slices.IndexFunc(old.Spec.Containers, func(oc corev1.Container) bool {
			return oc.Name == nc.Name
		})
		if index < 0 {
			return false
		}
		oc := old.Spec.Containers[index]
		realRequests := nc.Resources.Requests
		// 'requests' defaults to Limits if that is explicitly specified, see: https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#resources
		if realRequests == nil {
			realRequests = nc.Resources.Limits
		}
		if !equalField(oc.Resources.Requests, realRequests) {
			return false
		}
		if !equalField(oc.Resources.Limits, nc.Resources.Limits) {
			return false
		}
	}
	return true
}

func getPodUpdatePolicy(its *workloads.InstanceSet, pod *corev1.Pod) (PodUpdatePolicy, error) {
	updateRevisions, err := GetRevisions(its.Status.UpdateRevisions)
	if err != nil {
		return NoOpsPolicy, err
	}

	if getPodRevision(pod) != updateRevisions[pod.Name] {
		return RecreatePolicy, nil
	}

	itsExt, err := buildInstanceSetExt(its, nil)
	if err != nil {
		return NoOpsPolicy, err
	}
	templateList := buildInstanceTemplateExts(itsExt)
	parentName, _ := ParseParentNameAndOrdinal(pod.Name)
	templateName, _ := strings.CutPrefix(parentName, its.Name)
	if len(templateName) > 0 {
		templateName, _ = strings.CutPrefix(templateName, "-")
	}
	index := slices.IndexFunc(templateList, func(templateExt *instanceTemplateExt) bool {
		return templateName == templateExt.Name
	})
	if index < 0 {
		return NoOpsPolicy, fmt.Errorf("no corresponding template found for instance %s", pod.Name)
	}
	inst, err := buildInstanceByTemplate(pod.Name, templateList[index], its, getPodRevision(pod))
	if err != nil {
		return NoOpsPolicy, err
	}
	basicUpdate := !equalBasicInPlaceFields(pod, inst.pod)
	if viper.GetBool(FeatureGateIgnorePodVerticalScaling) {
		if basicUpdate {
			return InPlaceUpdatePolicy, nil
		}
		return NoOpsPolicy, nil
	}

	resourceUpdate := !equalResourcesInPlaceFields(pod, inst.pod)
	if resourceUpdate {
		if supportPodVerticalScaling() {
			return InPlaceUpdatePolicy, nil
		}
		return RecreatePolicy, nil
	}

	if basicUpdate {
		return InPlaceUpdatePolicy, nil
	}
	return NoOpsPolicy, nil
}

func isContainerInjected(ocs, ncs []corev1.Container) bool {
	for _, nc := range ncs {
		index := slices.IndexFunc(ocs, func(oc corev1.Container) bool {
			return nc.Name == oc.Name
		})
		if index < 0 {
			return false
		}
		if nc.Image != ocs[index].Image {
			return false
		}
	}
	return true
}

// IsPodUpdated tells whether the pod's spec is as expected in the InstanceSet.
// This function is meant to replace the old fashion `GetPodRevision(pod) == updateRevision`,
// as the pod template revision has been redefined in instanceset.
func IsPodUpdated(its *workloads.InstanceSet, pod *corev1.Pod) (bool, error) {
	policy, err := getPodUpdatePolicy(its, pod)
	return policy == NoOpsPolicy, err
}
