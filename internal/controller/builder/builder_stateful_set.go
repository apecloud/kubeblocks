/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package builder

import (
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type StatefulSetBuilder struct {
	BaseBuilder[apps.StatefulSet, *apps.StatefulSet, StatefulSetBuilder]
}

func NewStatefulSetBuilder(namespace, name string) *StatefulSetBuilder {
	builder := &StatefulSetBuilder{}
	builder.init(namespace, name, &apps.StatefulSet{}, builder)
	return builder
}

func (builder *StatefulSetBuilder) AddMatchLabel(key, value string) *StatefulSetBuilder {
	labels := make(map[string]string, 1)
	labels[key] = value
	return builder.AddMatchLabelsInMap(labels)
}

func (builder *StatefulSetBuilder) AddMatchLabels(keyValues ...string) *StatefulSetBuilder {
	return builder.AddMatchLabelsInMap(WithMap(keyValues...))
}

func (builder *StatefulSetBuilder) AddMatchLabelsInMap(labels map[string]string) *StatefulSetBuilder {
	matchLabels := builder.get().Spec.Selector.MatchLabels
	if matchLabels == nil {
		matchLabels = make(map[string]string, len(labels))
	}
	for k, v := range labels {
		matchLabels[k] = v
	}
	builder.get().Spec.Selector.MatchLabels = matchLabels
	return builder
}

func (builder *StatefulSetBuilder) SetServiceName(serviceName string) *StatefulSetBuilder {
	builder.get().Spec.ServiceName = serviceName
	return builder
}

func (builder *StatefulSetBuilder) SetReplicas(replicas int32) *StatefulSetBuilder {
	builder.get().Spec.Replicas = &replicas
	return builder
}

func (builder *StatefulSetBuilder) SetMinReadySeconds(minReadySeconds int32) *StatefulSetBuilder {
	builder.get().Spec.MinReadySeconds = minReadySeconds
	return builder
}

func (builder *StatefulSetBuilder) SetPodManagementPolicy(policy apps.PodManagementPolicyType) *StatefulSetBuilder {
	builder.get().Spec.PodManagementPolicy = policy
	return builder
}

func (builder *StatefulSetBuilder) SetTemplate(template corev1.PodTemplateSpec) *StatefulSetBuilder {
	builder.get().Spec.Template = template
	return builder
}

func (builder *StatefulSetBuilder) AddVolumeClaimTemplates(templates ...corev1.PersistentVolumeClaim) *StatefulSetBuilder {
	templateList := builder.get().Spec.VolumeClaimTemplates
	templateList = append(templateList, templates...)
	builder.get().Spec.VolumeClaimTemplates = templateList
	return builder
}

func (builder *StatefulSetBuilder) SetVolumeClaimTemplates(templates ...corev1.PersistentVolumeClaim) *StatefulSetBuilder {
	builder.get().Spec.VolumeClaimTemplates = templates
	return builder
}

func (builder *StatefulSetBuilder) SetUpdateStrategyType(strategyType apps.StatefulSetUpdateStrategyType) *StatefulSetBuilder {
	builder.get().Spec.UpdateStrategy.Type = strategyType
	return builder
}
