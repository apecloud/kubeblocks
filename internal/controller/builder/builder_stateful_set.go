/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package builder

import (
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	selector := builder.get().Spec.Selector
	if selector == nil {
		selector = &metav1.LabelSelector{}
		builder.get().Spec.Selector = selector
	}
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
