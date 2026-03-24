/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package reconfigure

import (
	"fmt"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

const (
	legacyConfigManagerContainerName = "config-manager"
	legacyConfigManagerPortName      = "config-manager"
	legacyConfigManagerDefaultPort   = 9901
	legacyConfigManagerGRPCService   = "proto.Reconfigure"
	legacyConfigManagerGRPCMethod    = "OnlineUpgradeParams"
)

// ValidateLegacyConfigManagerRuntime checks whether an existing workload still carries
// the legacy config-manager runtime required by ParametersDefinition.reloadAction.
func ValidateLegacyConfigManagerRuntime(its *workloads.InstanceSet) error {
	if HasLegacyConfigManagerRuntime(its) {
		return nil
	}
	if its == nil {
		return fmt.Errorf("legacy reloadAction requires an existing instanceSet with config-manager injected")
	}
	for _, container := range its.Spec.Template.Spec.Containers {
		if container.Name != legacyConfigManagerContainerName {
			continue
		}
		if len(container.Ports) == 0 {
			return fmt.Errorf("legacy config-manager container has no reachable port")
		}
		for _, port := range container.Ports {
			if port.Name == legacyConfigManagerPortName || len(container.Ports) == 1 {
				return nil
			}
		}
		return fmt.Errorf("legacy config-manager container does not expose a compatible gRPC port")
	}
	return fmt.Errorf("legacy reloadAction is only supported for existing instances that still have config-manager injected")
}

func HasLegacyConfigManagerRuntime(its *workloads.InstanceSet) bool {
	if its == nil {
		return false
	}
	for _, container := range its.Spec.Template.Spec.Containers {
		if container.Name != legacyConfigManagerContainerName {
			continue
		}
		for _, port := range container.Ports {
			if port.Name == legacyConfigManagerPortName || len(container.Ports) == 1 {
				return true
			}
		}
	}
	return false
}

func resolveLegacyConfigManagerPort(its *workloads.InstanceSet) int {
	if its == nil {
		return legacyConfigManagerDefaultPort
	}
	for _, container := range its.Spec.Template.Spec.Containers {
		if container.Name != legacyConfigManagerContainerName {
			continue
		}
		for _, port := range container.Ports {
			if port.Name == legacyConfigManagerPortName || len(container.Ports) == 1 {
				return int(port.ContainerPort)
			}
		}
	}
	return legacyConfigManagerDefaultPort
}
