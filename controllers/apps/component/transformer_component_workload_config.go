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
	"maps"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func dependOnInstanceConfigMaps(cli model.GraphClient, dag *graph.DAG, its *workloads.InstanceSet) {
	referenced := boundConfigMapNames(its)
	for name := range referenced {
		cli.DependOn(dag, its, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: its.Namespace, Name: name}})
	}
}

// Start can apply a config override within an existing InstanceTemplate without
// advancing the shared Pod template or changing which instances are allocated.
func publishStartingInstanceConfigs(target, desired *workloads.InstanceSet) {
	next := make(map[string]*workloads.InstanceTemplate, len(desired.Spec.Instances))
	for i := range desired.Spec.Instances {
		instance := &desired.Spec.Instances[i]
		if instance.GetReplicas() > 0 {
			next[instance.Name] = instance
		}
	}
	configMaps := sets.New[string]()
	for i := range target.Spec.Instances {
		current := &target.Spec.Instances[i]
		future := next[current.Name]
		if future == nil || current.GetReplicas() == 0 {
			continue
		}
		if syncStartingConfigOverrides(target, desired, current, future, configMaps) {
			syncStartingConfigInputs(target, desired, current, future)
		}
	}
	for _, ref := range desired.Spec.InstanceAssistantObjects {
		if ref.Kind == "ConfigMap" && ref.Namespace == target.Namespace && configMaps.Has(ref.Name) &&
			!slices.Contains(target.Spec.InstanceAssistantObjects, ref) {
			target.Spec.InstanceAssistantObjects = append(target.Spec.InstanceAssistantObjects, ref)
		}
	}
}

func syncStartingConfigOverrides(target, desired *workloads.InstanceSet,
	current, next *workloads.InstanceTemplate, configMaps sets.Set[string]) bool {
	names := sets.New[string]()
	for _, configs := range [][]workloads.ConfigTemplate{current.Configs, next.Configs} {
		for _, config := range configs {
			if config.Volume != nil {
				names.Insert(config.Name)
			}
		}
	}
	changed := false
	for _, name := range sets.List(names) {
		previous, future := configByName(current.Configs, name), configByName(next.Configs, name)
		if previous != nil && future != nil && ptr.Equal(previous.ConfigHash, future.ConfigHash) &&
			ptr.Equal(previous.Volume, future.Volume) {
			continue
		}

		base, desiredBase := configByName(target.Spec.Configs, name), configByName(desired.Spec.Configs, name)
		if base == nil || desiredBase == nil || desiredBase.Volume == nil {
			continue
		}
		// InstanceTemplates can select a ConfigMap for an existing volume.
		// The shared Pod template keeps its mounts until the normal update.
		binding := base.Volume
		if binding == nil {
			binding = desiredBase.Volume
		}
		mounted := configMapForVolume(&target.Spec.Template.Spec, binding.VolumeName)
		if mounted == "" {
			continue
		}
		source := configMapForVolume(&desired.Spec.Template.Spec, desiredBase.Volume.VolumeName)
		effective := desiredBase
		if future != nil {
			effective = future
			if future.Volume != nil {
				source = future.Volume.ConfigMapName
			}
		}
		if source == "" {
			continue
		}

		base.Volume = binding
		configs := slices.DeleteFunc(current.Configs, func(config workloads.ConfigTemplate) bool {
			return config.Name == name
		})
		// Returning to the shared ConfigMap can still require an override while
		// the InstanceSet's shared configuration entry is pending.
		if future != nil || !ptr.Equal(base.ConfigHash, effective.ConfigHash) || source != mounted {
			config := effective.DeepCopy()
			config.Volume = nil
			if source != mounted {
				config.Volume = &workloads.ConfigVolumeBinding{ConfigMapName: source}
			}
			configs = append(configs, *config)
		}
		current.Configs = configs
		configMaps.Insert(source)
		changed = true
	}
	if changed {
		slices.SortFunc(current.Configs, func(a, b workloads.ConfigTemplate) int {
			return strings.Compare(a.Name, b.Name)
		})
	}
	return changed
}

func configByName(configs []workloads.ConfigTemplate, name string) *workloads.ConfigTemplate {
	for i := range configs {
		if configs[i].Name == name {
			return &configs[i]
		}
	}
	return nil
}

func configMapForVolume(pod *corev1.PodSpec, volumeName string) string {
	for _, volume := range pod.Volumes {
		if volume.Name == volumeName && volume.ConfigMap != nil {
			return volume.ConfigMap.Name
		}
	}
	return ""
}

func syncStartingConfigInputs(target, desired *workloads.InstanceSet, current, next *workloads.InstanceTemplate) {
	effective := func(its *workloads.InstanceSet, tpl *workloads.InstanceTemplate) *instancetemplate.InstanceTemplateExt {
		return instancetemplate.BuildInstanceTemplateExt(&instancetemplate.InstanceSetExt{
			InstanceSet: its, InstanceTemplates: map[string]*workloads.InstanceTemplate{tpl.Name: tpl},
		})[0]
	}
	oldRuntime, newRuntime := effective(target, current), effective(desired, next)
	if len(newRuntime.Spec.Containers) > 0 {
		current.Resources = newRuntime.Spec.Containers[0].Resources.DeepCopy()
	}
	current.Images = maps.Clone(next.Images)
	if target.Annotations[constant.KBAppServiceVersionKey] != desired.Annotations[constant.KBAppServiceVersionKey] && len(next.Images) == 0 {
		// A template without an explicit version can inherit a new service image.
		current.Images = make(map[string]string)
		for _, containers := range [][]corev1.Container{newRuntime.Spec.Containers, newRuntime.Spec.InitContainers} {
			for _, container := range containers {
				current.Images[container.Name] = container.Image
			}
		}
	}
	for _, claim := range oldRuntime.VolumeClaimTemplates {
		index := slices.IndexFunc(newRuntime.VolumeClaimTemplates, func(next corev1.PersistentVolumeClaim) bool {
			return next.Name == claim.Name
		})
		if index < 0 {
			continue
		}
		resources := newRuntime.VolumeClaimTemplates[index].Spec.Resources
		if claim.Spec.Resources.Requests.Storage().Equal(*resources.Requests.Storage()) &&
			claim.Spec.Resources.Limits.Storage().Equal(*resources.Limits.Storage()) {
			continue
		}
		claim.Spec.Resources = resources
		index = slices.IndexFunc(current.VolumeClaimTemplates, func(existing corev1.PersistentVolumeClaim) bool {
			return existing.Name == claim.Name
		})
		if index < 0 {
			current.VolumeClaimTemplates = append(current.VolumeClaimTemplates, claim)
		} else {
			current.VolumeClaimTemplates[index] = claim
		}
	}
}

func boundConfigMapNames(its *workloads.InstanceSet) sets.Set[string] {
	names := sets.New[string]()
	volumes := sets.New[string]()
	for _, config := range its.Spec.Configs {
		if config.Volume != nil {
			volumes.Insert(config.Volume.VolumeName)
		}
	}
	for _, volume := range its.Spec.Template.Spec.Volumes {
		if volume.ConfigMap != nil && volume.ConfigMap.Name != "" && volumes.Has(volume.Name) {
			names.Insert(volume.ConfigMap.Name)
		}
	}
	retainInstanceTemplateConfigMaps(names, its.Spec.Instances)
	return names
}

func retainInstanceSetConfigMaps(retained sets.Set[string], its *workloads.InstanceSet) {
	if its == nil {
		return
	}
	retainPodConfigMaps(retained, &its.Spec.Template.Spec)
	retainInstanceTemplateConfigMaps(retained, its.Spec.Instances)
}

func retainInstanceTemplateConfigMaps(retained sets.Set[string], instances []workloads.InstanceTemplate) {
	for _, instance := range instances {
		for _, config := range instance.Configs {
			if config.Volume != nil {
				retained.Insert(config.Volume.ConfigMapName)
			}
		}
	}
}

func retainPodConfigMaps(retained sets.Set[string], spec *corev1.PodSpec) {
	for _, volume := range spec.Volumes {
		if volume.ConfigMap != nil {
			retained.Insert(volume.ConfigMap.Name)
		}
		if volume.Projected != nil {
			for _, source := range volume.Projected.Sources {
				if source.ConfigMap != nil {
					retained.Insert(source.ConfigMap.Name)
				}
			}
		}
	}
}
