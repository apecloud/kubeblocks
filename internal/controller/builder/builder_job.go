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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type JobBuilder struct {
	BaseBuilder[batchv1.Job, *batchv1.Job, JobBuilder]
}

func NewJobBuilder(namespace, name string) *JobBuilder {
	builder := &JobBuilder{}
	builder.init(namespace, name, &batchv1.Job{}, builder)
	return builder
}

func (builder *JobBuilder) SetPodTemplateSpec(template corev1.PodTemplateSpec) *JobBuilder {
	builder.get().Spec.Template = template
	return builder
}

func (builder *JobBuilder) AddSelector(key, value string) *JobBuilder {
	selector := builder.get().Spec.Selector
	if selector == nil {
		selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{},
		}
	}
	selector.MatchLabels[key] = value
	builder.get().Spec.Selector = selector
	return builder
}
