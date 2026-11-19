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
	"fmt"

	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

// ResolveConfigs resolves config overrides and their volume bindings together.
// Callers opt in to binding expansion after building the instance Pod template.
func ResolveConfigs(template *InstanceTemplateExt, configs, overrides []workloads.ConfigTemplate) error {
	indices := make(map[string]int, len(configs))
	counts := make(map[string]int, len(configs))
	effective := make([]workloads.ConfigTemplate, len(configs))
	for i := range configs {
		config := &configs[i]
		if config.Volume != nil {
			index, err := configVolumeIndex(template, *config)
			if err != nil {
				return err
			}
			if template.Spec.Volumes[index].ConfigMap.Name != config.Volume.ConfigMapName {
				return fmt.Errorf("config %q: binding ConfigMap %q does not match volume %q source", config.Name, config.Volume.ConfigMapName, config.Volume.VolumeName)
			}
		}
		counts[config.Name]++
		indices[config.Name] = i
		effective[i] = *config.DeepCopy()
	}
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
		baseBinding := configs[index].Volume
		if config.Volume != nil && (baseBinding == nil || config.Volume.VolumeName != baseBinding.VolumeName) {
			return fmt.Errorf("config override %q: volume must match an explicit InstanceSet config binding", config.Name)
		}
		effective[index] = *config.DeepCopy()
		if effective[index].Volume == nil && baseBinding != nil {
			effective[index].Volume = baseBinding.DeepCopy()
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
	return nil
}

func configVolumeIndex(template *InstanceTemplateExt, config workloads.ConfigTemplate) (int, error) {
	binding := config.Volume
	if binding.VolumeName == "" || binding.ConfigMapName == "" {
		return 0, fmt.Errorf("config %q: volume and ConfigMap names must not be empty", config.Name)
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

// BuildInstanceTemplatesWithConfigs maps desired instance names to effective Pod templates and configs.
func BuildInstanceTemplatesWithConfigs(its *workloads.InstanceSet, tree *kubebuilderx.ObjectTree) (map[string]*InstanceTemplateExt, error) {
	itsExt, err := BuildInstanceSetExt(its, tree)
	if err != nil {
		return nil, err
	}
	if len(itsExt.InstanceTemplates) == 0 {
		template := &InstanceTemplateExt{
			PodTemplateSpec:      its.Spec.Template,
			VolumeClaimTemplates: its.Spec.VolumeClaimTemplates,
		}
		if err := ResolveConfigs(template, its.Spec.Configs, nil); err != nil {
			return nil, err
		}
	}
	opts := &PodNameBuilderOpts{}
	if tree != nil {
		opts.EventLogger = tree.EventRecorder
	}
	nameBuilder, err := NewPodNameBuilder(itsExt, opts)
	if err != nil {
		return nil, err
	}
	templates, err := nameBuilder.BuildInstanceName2TemplateMap()
	if err != nil {
		return nil, err
	}
	resolved := make(map[string]*InstanceTemplateExt, len(itsExt.InstanceTemplates))
	for _, template := range templates {
		resolved[template.Name] = template
	}
	for name, source := range itsExt.InstanceTemplates {
		template := resolved[name]
		if template == nil {
			// Validate bindings for zero-replica templates as well.
			template = buildInstanceTemplateExt(source, its)
		}
		if err := ResolveConfigs(template, its.Spec.Configs, source.Configs); err != nil {
			return nil, fmt.Errorf("instance template %q: %w", name, err)
		}
	}
	return templates, nil
}

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
