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
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

func resolveConfigs(template *InstanceTemplateExt, configs, overrides []workloads.ConfigTemplate) error {
	indices := make(map[string]int, len(configs))
	counts := make(map[string]int, len(configs))
	effective := make([]workloads.ConfigTemplate, len(configs))
	for i := range configs {
		effective[i] = *configs[i].DeepCopy()
		config := &effective[i]
		if config.Volume != nil {
			index, err := configVolumeIndex(template, *config)
			if err != nil {
				return err
			}
			source := template.Spec.Volumes[index].ConfigMap.Name
			if source == "" {
				return fmt.Errorf("config %q: volume %q has no ConfigMap name", config.Name, config.Volume.VolumeName)
			}
			if config.Volume.ConfigMapName != "" && config.Volume.ConfigMapName != source {
				return fmt.Errorf("config %q: binding ConfigMap %q does not match volume %q source", config.Name, config.Volume.ConfigMapName, config.Volume.VolumeName)
			}
			config.Volume.ConfigMapName = source
		}
		counts[config.Name]++
		indices[config.Name] = i
	}
	var overriddenConfigs []string
	seen := make(map[string]bool, len(overrides))
	for i := range overrides {
		config := &overrides[i]
		if config.Name == "" {
			return fmt.Errorf("config override name must not be empty")
		}
		if seen[config.Name] {
			return fmt.Errorf("duplicate config override name %q", config.Name)
		}
		seen[config.Name] = true
		index, exists := indices[config.Name]
		if !exists {
			return fmt.Errorf("config override %q does not match an InstanceSet config", config.Name)
		}
		if counts[config.Name] > 1 {
			return fmt.Errorf("config override %q matches multiple InstanceSet configs", config.Name)
		}
		baseConfig := &effective[index]
		baseBinding := baseConfig.Volume
		if binding := config.Volume; binding != nil {
			if binding.ConfigMapName == "" {
				return fmt.Errorf("config override %q: ConfigMap name must not be empty", config.Name)
			}
			if baseBinding == nil || (binding.VolumeName != "" && binding.VolumeName != baseBinding.VolumeName) {
				return fmt.Errorf("config override %q: volume must match an explicit InstanceSet config binding", config.Name)
			}
		}
		if ptr.Deref(config.ConfigHash, "") != ptr.Deref(baseConfig.ConfigHash, "") ||
			(config.Volume != nil && config.Volume.ConfigMapName != baseBinding.ConfigMapName) {
			overriddenConfigs = append(overriddenConfigs, config.Name)
		}
		effective[index] = *config.DeepCopy()
		if effective[index].Volume == nil {
			effective[index].Volume = baseBinding
		} else {
			effective[index].Volume.VolumeName = baseBinding.VolumeName
		}
	}

	podTemplate := &template.PodTemplateSpec
	for _, config := range effective {
		if config.Volume != nil {
			podTemplate = template.PodTemplateSpec.DeepCopy()
			break
		}
	}
	destinations := make(map[string]string)
	for _, config := range effective {
		binding := config.Volume
		if binding == nil {
			continue
		}
		index, err := configVolumeIndex(template, config)
		if err != nil {
			return err
		}
		if destination, exists := destinations[binding.VolumeName]; exists && destination != binding.ConfigMapName {
			return fmt.Errorf("conflicting ConfigMap bindings for volume %q", binding.VolumeName)
		}
		destinations[binding.VolumeName] = binding.ConfigMapName
		podTemplate.Spec.Volumes[index].ConfigMap.Name = binding.ConfigMapName
	}
	template.PodTemplateSpec = *podTemplate
	template.Configs = effective
	slices.Sort(overriddenConfigs)
	template.overriddenConfigs = overriddenConfigs
	return nil
}

func configVolumeIndex(template *InstanceTemplateExt, config workloads.ConfigTemplate) (int, error) {
	binding := config.Volume
	if binding.VolumeName == "" {
		return 0, fmt.Errorf("config %q: volume name must not be empty", config.Name)
	}
	for _, claim := range template.VolumeClaimTemplates {
		if claim.Name == binding.VolumeName {
			return 0, fmt.Errorf("config %q: volume %q is replaced by a persistent volume claim", config.Name, binding.VolumeName)
		}
	}
	for i, volume := range template.Spec.Volumes {
		if volume.Name != binding.VolumeName {
			continue
		}
		if volume.ConfigMap == nil {
			return 0, fmt.Errorf("config %q: volume %q is not a ConfigMap volume", config.Name, binding.VolumeName)
		}
		return i, nil
	}
	return 0, fmt.Errorf("config %q: volume %q does not exist", config.Name, binding.VolumeName)
}

const configReloadScopeAnnotation = "workloads.kubeblocks.io/config-reload-scope"

// Scoped configurations can differ anywhere in the InstanceSet. Overrides
// track only the differences applied by this Pod, which can outlive the
// InstanceTemplate override until the Pod has reverted or been retired.
type configReloadScope struct {
	Scoped      []string `json:"scoped,omitempty"`
	Overrides   []string `json:"overrides,omitempty"`
	InstanceSet string   `json:"instanceSet,omitempty"`
}

func setConfigReloadScope(templates map[string]*InstanceTemplateExt, names sets.Set[string], instanceSet string) {
	scoped := sets.List(names)
	for _, template := range templates {
		if len(scoped) == 0 {
			delete(template.Annotations, configReloadScopeAnnotation)
			continue
		}
		scope := configReloadScope{Scoped: scoped, Overrides: template.overriddenConfigs}
		if len(scope.Overrides) != 0 {
			scope.InstanceSet = instanceSet
		}
		data, _ := json.Marshal(scope)
		if template.Annotations == nil {
			template.Annotations = make(map[string]string)
		}
		template.Annotations[configReloadScopeAnnotation] = string(data)
	}
}

func podConfigReloadScope(pod *corev1.Pod) configReloadScope {
	var scope configReloadScope
	if value := pod.Annotations[configReloadScopeAnnotation]; value != "" {
		if err := json.Unmarshal([]byte(value), &scope); err != nil {
			return configReloadScope{}
		}
	}
	return scope
}

// ConfigReloadScope returns the configuration names marked for Pod-local reload.
func ConfigReloadScope(pod *corev1.Pod) sets.Set[string] {
	return sets.New(podConfigReloadScope(pod).Scoped...)
}

// AppliedConfigReloadOverrides returns the names of overrides recorded on Pods
// of the InstanceSet in the current observation.
func AppliedConfigReloadOverrides(instanceSet string, pods ...*corev1.Pod) sets.Set[string] {
	overrides := sets.New[string]()
	for _, pod := range pods {
		scope := podConfigReloadScope(pod)
		if scope.InstanceSet == instanceSet {
			overrides.Insert(scope.Overrides...)
		}
	}
	return overrides
}

// ConfigReloadTarget pins a scoped reload to the triggering Pod and skips it
// if the configured volume is unmounted. Other reloads keep their selector.
func ConfigReloadTarget(pod *corev1.Pod, config workloads.ConfigTemplate, scoped bool) (targetPod string, eligible bool) {
	if scoped {
		return pod.Name, config.Volume == nil || hasConfigVolumeMount(pod, config.Volume.VolumeName)
	}
	return "", true
}

func hasConfigVolumeMount(pod *corev1.Pod, name string) bool {
	for _, containers := range [][]corev1.Container{pod.Spec.Containers, pod.Spec.InitContainers} {
		for _, container := range containers {
			if slices.ContainsFunc(container.VolumeMounts, func(mount corev1.VolumeMount) bool {
				return mount.Name == name
			}) {
				return true
			}
		}
	}
	return false
}

// AnnotationsWithoutConfigReloadScope excludes the bookkeeping annotation
// from the Pod update policy.
func AnnotationsWithoutConfigReloadScope(pod *corev1.Pod) map[string]string {
	if _, ok := pod.Annotations[configReloadScopeAnnotation]; !ok {
		return pod.Annotations
	}
	annotations := maps.Clone(pod.Annotations)
	delete(annotations, configReloadScopeAnnotation)
	return annotations
}

// SyncConfigReloadScope copies the desired annotation to a Pod.
func SyncConfigReloadScope(pod, desiredPod *corev1.Pod) {
	scope := desiredPod.Annotations[configReloadScopeAnnotation]
	if scope == "" {
		delete(pod.Annotations, configReloadScopeAnnotation)
		return
	}
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	pod.Annotations[configReloadScopeAnnotation] = scope
}

// ConfigReloadScopePatch records a scope-only change without applying the Pod
// replacement policy to otherwise unchanged configuration.
func ConfigReloadScopePatch(pod, desiredPod *corev1.Pod) *corev1.Pod {
	if pod.Annotations[configReloadScopeAnnotation] == desiredPod.Annotations[configReloadScopeAnnotation] {
		return nil
	}
	patch := pod.DeepCopy()
	SyncConfigReloadScope(patch, desiredPod)
	return patch
}
