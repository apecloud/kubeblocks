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

package component

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

// ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
// Resource requests and limits of Pod could be following types:
// spec.containers[].resources.limits.cpu
// spec.containers[].resources.limits.memory
// spec.containers[].resources.limits.hugepages-<size>
// spec.containers[].resources.requests.cpu
// spec.containers[].resources.requests.memory
// spec.containers[].resources.requests.hugepages-<size>
//
// extractComponentResourceValue extracts the value of a resource in an already known component.
// This function is a simplified version of resource.ExtractContainerResourceValue. Please refer to kubectl/pkg/util/resource/resource.go for more details.
func extractComponentResourceValue(fs *appsv1alpha1.ComponentResourceFieldRef, comp *appsv1alpha1.ClusterComponentSpec) (string, error) {
	divisor := resource.Quantity{}
	if divisor.Cmp(fs.Divisor) == 0 {
		divisor = resource.MustParse("1")
	} else {
		divisor = fs.Divisor
	}

	switch fs.Resource {
	case "limits.cpu":
		return convertResourceCPUToString(comp.Resources.Limits.Cpu(), divisor)
	case "limits.memory":
		return convertResourceMemoryToString(comp.Resources.Limits.Memory(), divisor)
	case "requests.cpu":
		return convertResourceCPUToString(comp.Resources.Requests.Cpu(), divisor)
	case "requests.memory":
		return convertResourceMemoryToString(comp.Resources.Requests.Memory(), divisor)
	}
	// handle extended standard resources with dynamic names
	// example: requests.hugepages-<pageSize> or limits.hugepages-<pageSize>
	if strings.HasPrefix(fs.Resource, "requests.") {
		resourceName := corev1.ResourceName(strings.TrimPrefix(fs.Resource, "requests."))
		if isHugePageResourceName(resourceName) {
			return convertResourceHugePagesToString(comp.Resources.Requests.Name(resourceName, resource.BinarySI), divisor)
		}
	}
	if strings.HasPrefix(fs.Resource, "limits.") {
		resourceName := corev1.ResourceName(strings.TrimPrefix(fs.Resource, "limits."))
		if isHugePageResourceName(resourceName) {
			return convertResourceHugePagesToString(comp.Resources.Limits.Name(resourceName, resource.BinarySI), divisor)
		}
	}
	return "", fmt.Errorf("unsupported container resource : %v", fs.Resource)
}

// convertResourceCPUToString converts cpu value to the format of divisor and returns
// ceiling of the value.
func convertResourceCPUToString(cpu *resource.Quantity, divisor resource.Quantity) (string, error) {
	c := int64(math.Ceil(float64(cpu.MilliValue()) / float64(divisor.MilliValue())))
	return strconv.FormatInt(c, 10), nil
}

// convertResourceMemoryToString converts memory value to the format of divisor and returns
// ceiling of the value.
func convertResourceMemoryToString(memory *resource.Quantity, divisor resource.Quantity) (string, error) {
	m := int64(math.Ceil(float64(memory.Value()) / float64(divisor.Value())))
	return strconv.FormatInt(m, 10), nil
}

// convertResourceHugePagesToString converts hugepages value to the format of divisor and returns
// ceiling of the value.
func convertResourceHugePagesToString(hugePages *resource.Quantity, divisor resource.Quantity) (string, error) {
	m := int64(math.Ceil(float64(hugePages.Value()) / float64(divisor.Value())))
	return strconv.FormatInt(m, 10), nil
}

// IsHugePageResourceName returns true if the resource name has the huge page
// resource prefix.
func isHugePageResourceName(name corev1.ResourceName) bool {
	return strings.HasPrefix(string(name), corev1.ResourceHugePagesPrefix)
}
