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

package instancetemplate

import (
	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

// ChangedConfigMapVolumes identifies desired ConfigMap volume sources that differ from the existing Pod.
func ChangedConfigMapVolumes(old, desired *corev1.Pod) map[string]bool {
	previous := make(map[string]string)
	for _, volume := range old.Spec.Volumes {
		if volume.ConfigMap != nil {
			previous[volume.Name] = volume.ConfigMap.Name
		}
	}
	changed := make(map[string]bool)
	for _, volume := range desired.Spec.Volumes {
		if volume.ConfigMap != nil {
			if source, exists := previous[volume.Name]; !exists || source != volume.ConfigMap.Name {
				changed[volume.Name] = true
			}
		}
	}
	return changed
}

// ConfigsForMountedVolumes excludes actions whose explicitly associated volume needs replacement.
// Unbound legacy configs retain their existing reload behavior.
func ConfigsForMountedVolumes(configs []workloads.ConfigTemplate, old, desired *corev1.Pod) []workloads.ConfigTemplate {
	changed := ChangedConfigMapVolumes(old, desired)
	if len(changed) == 0 {
		return configs
	}
	mounted := make([]workloads.ConfigTemplate, 0, len(configs))
	for _, config := range configs {
		if config.Volume != nil && changed[config.Volume.VolumeName] {
			continue
		}
		mounted = append(mounted, config)
	}
	return mounted
}
