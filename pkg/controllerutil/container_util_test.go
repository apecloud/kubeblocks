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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestGetContainerByName(t *testing.T) {
	containerName1 := "test1"
	containerName2 := "test2"
	containers := []corev1.Container{
		{Name: containerName1},
		{Name: containerName2},
	}
	i, container := GetContainerByName(containers, containerName1)
	if i != 0 || container.Name != containerName1 {
		t.Error("expected to return 0 and the corresponding index container!")
	}
	i, container = GetContainerByName(containers, "test3")
	if i != -1 || container != nil {
		t.Error("expected to return 0 and the corresponding index container!")
	}
}

func TestInjectZeroResourceLimits(t *testing.T) {
	container := &corev1.Container{}
	InjectZeroResourcesLimitsIfEmpty(container)
	if container.Resources.Limits.Cpu().String() != "0" {
		t.Fatalf("expected zero cpu limit, got %s", container.Resources.Limits.Cpu().String())
	}
	if container.Resources.Limits.Memory().String() != "0" {
		t.Fatalf("expected zero memory limit, got %s", container.Resources.Limits.Memory().String())
	}

	container = &corev1.Container{Resources: corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}}
	InjectZeroResourceLimitIfEmpty(container, corev1.ResourceCPU)
	if _, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
		t.Fatalf("expected cpu limit to remain unset when request exists")
	}
}

func TestInjectZeroResourceLimitsByFeatureFlags(t *testing.T) {
	defer func() {
		viper.Set(constant.CfgKeyDataProtectionZeroResourceForUnset, false)
		viper.Set(constant.CfgKeyOperationZeroResourceForUnset, false)
	}()

	container := &corev1.Container{}
	InjectZeroResourcesLimitsForDataProtection(container)
	if len(container.Resources.Limits) != 0 {
		t.Fatalf("expected dataprotection injection to be disabled by default")
	}
	viper.Set(constant.CfgKeyDataProtectionZeroResourceForUnset, true)
	InjectZeroResourcesLimitsForDataProtection(container)
	if len(container.Resources.Limits) != 2 {
		t.Fatalf("expected dataprotection injection to set zero cpu and memory")
	}

	container = &corev1.Container{}
	viper.Set(constant.CfgKeyOperationZeroResourceForUnset, true)
	InjectZeroResourcesLimitsForOps(container)
	if len(container.Resources.Limits) != 2 {
		t.Fatalf("expected ops injection to set zero cpu and memory")
	}
}
