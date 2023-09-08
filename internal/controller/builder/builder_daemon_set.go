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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DaemonSetBuilder struct {
	BaseBuilder[appsv1.DaemonSet, *appsv1.DaemonSet, DaemonSetBuilder]
}

func NewDaemonSetBuilder(namespace, name string) *DaemonSetBuilder {
	builder := &DaemonSetBuilder{}
	builder.init(namespace, name, &appsv1.DaemonSet{}, builder)
	return builder
}

func (builder *DaemonSetBuilder) SetSelector(selector *metav1.LabelSelector) *DaemonSetBuilder {
	builder.get().Spec.Selector = selector
	return builder
}

func (builder *DaemonSetBuilder) SetTemplate(template corev1.PodTemplateSpec) *DaemonSetBuilder {
	builder.get().Spec.Template = template
	return builder
}

func (builder *DaemonSetBuilder) SetUpdateStrategy(strategy appsv1.DaemonSetUpdateStrategy) *DaemonSetBuilder {
	builder.get().Spec.UpdateStrategy = strategy
	return builder
}

func (builder *DaemonSetBuilder) AddLabelsInMap(labels map[string]string) *DaemonSetBuilder {
	l := builder.object.GetLabels()
	if l == nil {
		l = make(map[string]string)
	}
	for k, v := range labels {
		l[k] = v
	}
	builder.object.SetLabels(l)
	return builder.concreteBuilder
}

func (builder *DaemonSetBuilder) AddMatchLabelsInMap(labels map[string]string) *DaemonSetBuilder {
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
