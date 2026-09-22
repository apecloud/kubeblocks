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
	"fmt"
	"reflect"
	"slices"

	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
)

const (
	KBAgentRoleLabelVolumeName = "kubeblocks-role-label"
	KBAgentRoleLabelMountPath  = "/etc/kubeblocks/pod-metadata"
	kbAgentProbeEnvName        = "KB_AGENT_PROBE"
)

// PreserveKBAgentRoleLabelReprobePodSpec retains the adopted reprobe configuration
// when the gate is disabled, without retaining unrelated workload settings.
func PreserveKBAgentRoleLabelReprobePodSpec(oldSpec, newSpec *corev1.PodSpec) error {
	if oldSpec == nil || newSpec == nil {
		return nil
	}
	oldContainer := findContainer(oldSpec.Containers, kbagent.ContainerName)
	newContainer := findContainer(newSpec.Containers, kbagent.ContainerName)
	oldVolume := findVolume(oldSpec.Volumes, roleLabelVolumeName)
	if oldVolume == nil || oldContainer == nil || newContainer == nil {
		return nil
	}
	// A user volume with the same name alone does not establish adoption.
	if oldVolume.DownwardAPI == nil || len(oldVolume.DownwardAPI.Items) != 1 {
		return nil
	}
	item := oldVolume.DownwardAPI.Items[0]
	if item.Path != podRoleLabelFileName || item.FieldRef == nil || item.FieldRef.FieldPath != fmt.Sprintf("metadata.labels['%s']", constant.RoleLabelKey) {
		return nil
	}
	mountIndex := slices.IndexFunc(oldContainer.VolumeMounts, func(m corev1.VolumeMount) bool { return reflect.DeepEqual(m, roleLabelVolumeMount) })
	if mountIndex < 0 {
		return nil
	}
	volumeIndex := slices.IndexFunc(oldSpec.Volumes, func(v corev1.Volume) bool { return v.Name == roleLabelVolumeName })
	if current := findVolume(newSpec.Volumes, roleLabelVolumeName); current == nil {
		newSpec.Volumes = slices.Insert(newSpec.Volumes, min(volumeIndex, len(newSpec.Volumes)), *oldVolume.DeepCopy())
	} else if !reflect.DeepEqual(current, oldVolume) {
		return fmt.Errorf("volume %s conflicts with adopted kbagent role label volume", roleLabelVolumeName)
	}
	mounted := false
	for _, mount := range newContainer.VolumeMounts {
		if reflect.DeepEqual(mount, roleLabelVolumeMount) {
			mounted = true
			break
		}
		if mount.MountPath == podMetadataMountPath {
			return fmt.Errorf("volumeMount path %s conflicts with adopted kbagent role label mount", podMetadataMountPath)
		}
	}
	if !mounted {
		newContainer.VolumeMounts = slices.Insert(newContainer.VolumeMounts, min(mountIndex, len(newContainer.VolumeMounts)), roleLabelVolumeMount)
	}
	// Both the server and the init worker receive the startup probe environment.
	for _, pair := range []struct {
		old, desired []corev1.Container
		name         string
	}{
		{oldSpec.Containers, newSpec.Containers, kbagent.ContainerName},
		{oldSpec.InitContainers, newSpec.InitContainers, kbagent.ContainerName4Worker},
	} {
		old, desired := findContainer(pair.old, pair.name), findContainer(pair.desired, pair.name)
		if old != nil && desired != nil {
			if err := preserveRoleProbeFields(old, desired); err != nil {
				return fmt.Errorf("preserve %s probes: %w", pair.name, err)
			}
		}
	}
	return nil
}

func preserveRoleProbeFields(oldContainer, newContainer *corev1.Container) error {
	oldEnv := findEnv(oldContainer.Env, kbAgentProbeEnvName)
	newEnv := findEnv(newContainer.Env, kbAgentProbeEnvName)
	if oldEnv == nil || newEnv == nil || oldEnv.Value == "" || newEnv.Value == "" {
		return nil
	}
	// Keep unrelated and unknown fields in the desired configuration. Reusing the
	// original bytes when otherwise equal avoids JSON formatting-only revisions.
	var oldProbes, newProbes []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(oldEnv.Value), &oldProbes); err != nil {
		return fmt.Errorf("decode existing probes: %w", err)
	}
	if err := json.Unmarshal([]byte(newEnv.Value), &newProbes); err != nil {
		return fmt.Errorf("decode desired probes: %w", err)
	}
	changed := false
	for _, desired := range newProbes {
		if string(desired["action"]) != `"roleProbe"` {
			continue
		}
		for _, old := range oldProbes {
			if string(old["action"]) != `"roleProbe"` {
				continue
			}
			for _, key := range []string{"reportOnFileChange", "reportPeriodSeconds"} {
				if value, ok := old[key]; ok {
					desired[key] = value
					changed = true
				}
			}
			break
		}
	}
	if !changed {
		return nil
	}
	data, err := json.Marshal(newProbes)
	if err != nil {
		return fmt.Errorf("encode desired probes: %w", err)
	}
	var oldValue, newValue any
	if err := json.Unmarshal([]byte(oldEnv.Value), &oldValue); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &newValue); err != nil {
		return err
	}
	if reflect.DeepEqual(oldValue, newValue) {
		newEnv.Value = oldEnv.Value
	} else {
		newEnv.Value = string(data)
	}
	return nil
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
