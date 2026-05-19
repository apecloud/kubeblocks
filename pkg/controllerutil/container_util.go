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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func GetContainerByName(containers []corev1.Container, name string) (int, *corev1.Container) {
	for i, container := range containers {
		if container.Name == name {
			return i, &containers[i]
		}
	}
	return -1, nil
}

func InjectZeroResourcesLimitsIfNeed(c *corev1.Container) {
	InjectZeroResourceLimitIfNeed(c, corev1.ResourceCPU)
	InjectZeroResourceLimitIfNeed(c, corev1.ResourceMemory)
}

func InjectZeroResourceLimitIfNeed(c *corev1.Container, name corev1.ResourceName) {
	if !viper.GetBool(constant.CfgKeyEnableZeroResourceForUnset) {
		return
	}
	if _, ok := c.Resources.Requests[name]; ok {
		return
	}
	if _, ok := c.Resources.Limits[name]; ok {
		return
	}
	if c.Resources.Limits == nil {
		c.Resources.Limits = corev1.ResourceList{}
	}
	c.Resources.Limits[name] = resource.MustParse("0")
}
