/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"encoding/json"

	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type ClusterDefaultResources struct {
	Zero     bool                `json:"zero,omitempty"`
	Requests corev1.ResourceList `json:"requests,omitempty"`
	Limits   corev1.ResourceList `json:"limits,omitempty"`
}

func ParseClusterDefaultResources(value string) (ClusterDefaultResources, error) {
	resources := ClusterDefaultResources{}
	if value == "" {
		return resources, nil
	}
	if err := json.Unmarshal([]byte(value), &resources); err != nil {
		return ClusterDefaultResources{}, err
	}
	return resources, nil
}

func GetClusterDefaultResources() (ClusterDefaultResources, error) {
	return ParseClusterDefaultResources(viper.GetString(constant.CfgKeyClusterDefaultResources))
}

func SetClusterDefaultResourcesFromConfig(container *corev1.Container) error {
	resources, err := GetClusterDefaultResources()
	if err != nil {
		return err
	}
	SetClusterDefaultResources(container, resources)
	return nil
}

func SetClusterDefaultResourcesForPodSpecFromConfig(podSpec *corev1.PodSpec) error {
	resources, err := GetClusterDefaultResources()
	if err != nil {
		return err
	}
	SetClusterDefaultResourcesForPodSpec(podSpec, resources)
	return nil
}

func SetClusterDefaultResources(container *corev1.Container, resources ClusterDefaultResources) {
	for _, name := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
		if hasClusterDefaultResource(resources, name) {
			completeClusterDefaultResource(container, resources, name)
			continue
		}
		if resources.Zero {
			InjectZeroResourceLimitIfEmpty(container, name)
		}
	}
}

func SetClusterDefaultResourcesForPodSpec(podSpec *corev1.PodSpec, resources ClusterDefaultResources) {
	for i := range podSpec.Containers {
		SetClusterDefaultResources(&podSpec.Containers[i], resources)
	}
	for i := range podSpec.InitContainers {
		SetClusterDefaultResources(&podSpec.InitContainers[i], resources)
	}
}

func hasClusterDefaultResource(resources ClusterDefaultResources, name corev1.ResourceName) bool {
	_, hasRequest := resources.Requests[name]
	_, hasLimit := resources.Limits[name]
	return hasRequest || hasLimit
}

func completeClusterDefaultResource(container *corev1.Container, resources ClusterDefaultResources, name corev1.ResourceName) {
	if container.Resources.Requests == nil {
		container.Resources.Requests = corev1.ResourceList{}
	}
	if container.Resources.Limits == nil {
		container.Resources.Limits = corev1.ResourceList{}
	}

	request, hasRequest := container.Resources.Requests[name]
	limit, hasLimit := container.Resources.Limits[name]
	if hasRequest && hasLimit {
		return
	}
	if hasRequest {
		container.Resources.Limits[name] = request
		return
	}
	if hasLimit {
		container.Resources.Requests[name] = limit
		return
	}

	request, hasRequest = resources.Requests[name]
	limit, hasLimit = resources.Limits[name]
	if hasRequest && !hasLimit {
		limit = request
	} else if !hasRequest && hasLimit {
		request = limit
	}
	container.Resources.Requests[name] = request
	container.Resources.Limits[name] = limit
}
