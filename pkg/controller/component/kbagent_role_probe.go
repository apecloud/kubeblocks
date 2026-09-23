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

package component

import (
	"encoding/json"
	"reflect"

	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/kbagent"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	roleLabelVolumeName  = "kubeblocks-role-label"
	podMetadataMountPath = "/etc/kubeblocks/pod-metadata"
	kbAgentProbeEnvName  = "KB_AGENT_PROBE"
)

// PreserveKBAgentRoleLabelRecoveryPodSpec keeps #10201 fields on an existing
// workload when the feature gate is turned off. New workloads still omit the
// fields, but disabling the gate cannot create a second rollout by removing
// fields from a workload that already adopted the feature.
func PreserveKBAgentRoleLabelRecoveryPodSpec(oldSpec, newSpec *corev1.PodSpec) {
	if oldSpec == nil || newSpec == nil {
		return
	}
	oldVolume := findVolume(oldSpec.Volumes, roleLabelVolumeName)
	oldContainer := findContainer(oldSpec.Containers, kbagent.ContainerName)
	newContainer := findContainer(newSpec.Containers, kbagent.ContainerName)
	if oldVolume == nil || oldContainer == nil || newContainer == nil {
		return
	}
	if findVolume(newSpec.Volumes, roleLabelVolumeName) == nil {
		newSpec.Volumes = append(newSpec.Volumes, *oldVolume.DeepCopy())
	}
	for _, mount := range oldContainer.VolumeMounts {
		if mount.Name != roleLabelVolumeName || mount.MountPath != podMetadataMountPath {
			continue
		}
		found := false
		for _, current := range newContainer.VolumeMounts {
			if reflect.DeepEqual(current, mount) {
				found = true
				break
			}
		}
		if !found {
			newContainer.VolumeMounts = append(newContainer.VolumeMounts, mount)
		}
	}
	preserveRoleProbeFields(oldContainer, newContainer)
}

func preserveRoleProbeFields(oldContainer, newContainer *corev1.Container) {
	oldEnv := findEnv(oldContainer.Env, kbAgentProbeEnvName)
	newEnv := findEnv(newContainer.Env, kbAgentProbeEnvName)
	if oldEnv == nil || newEnv == nil || oldEnv.Value == "" || newEnv.Value == "" {
		return
	}
	var oldProbes, newProbes []proto.Probe
	if err := json.Unmarshal([]byte(oldEnv.Value), &oldProbes); err != nil {
		newEnv.Value = oldEnv.Value
		return
	}
	if err := json.Unmarshal([]byte(newEnv.Value), &newProbes); err != nil {
		newEnv.Value = oldEnv.Value
		return
	}
	for i := range newProbes {
		if newProbes[i].Action != "roleProbe" {
			continue
		}
		for _, oldProbe := range oldProbes {
			if oldProbe.Action == "roleProbe" && len(oldProbe.ReportOnFileChange) > 0 {
				newProbes[i].ReportOnFileChange = append([]string(nil), oldProbe.ReportOnFileChange...)
				newProbes[i].ReportPeriodSeconds = oldProbe.ReportPeriodSeconds
				break
			}
		}
	}
	data, err := json.Marshal(newProbes)
	if err != nil {
		newEnv.Value = oldEnv.Value
		return
	}
	newEnv.Value = string(data)
}

func findVolume(volumes []corev1.Volume, name string) *corev1.Volume {
	for i := range volumes {
		if volumes[i].Name == name {
			return &volumes[i]
		}
	}
	return nil
}

func findContainer(containers []corev1.Container, name string) *corev1.Container {
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i]
		}
	}
	return nil
}

func findEnv(env []corev1.EnvVar, name string) *corev1.EnvVar {
	for i := range env {
		if env[i].Name == name {
			return &env[i]
		}
	}
	return nil
}
