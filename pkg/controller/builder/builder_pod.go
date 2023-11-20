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

import corev1 "k8s.io/api/core/v1"

type PodBuilder struct {
	BaseBuilder[corev1.Pod, *corev1.Pod, PodBuilder]
}

func NewPodBuilder(namespace, name string) *PodBuilder {
	builder := &PodBuilder{}
	builder.init(namespace, name, &corev1.Pod{}, builder)
	return builder
}

func (builder *PodBuilder) SetContainers(containers []corev1.Container) *PodBuilder {
	builder.get().Spec.Containers = containers
	return builder
}

func (builder *PodBuilder) AddContainer(container corev1.Container) *PodBuilder {
	containers := builder.get().Spec.Containers
	containers = append(containers, container)
	builder.get().Spec.Containers = containers
	return builder
}

func (builder *PodBuilder) AddVolumes(volumes ...corev1.Volume) *PodBuilder {
	builder.get().Spec.Volumes = append(builder.get().Spec.Volumes, volumes...)
	return builder
}

func (builder *PodBuilder) SetRestartPolicy(policy corev1.RestartPolicy) *PodBuilder {
	builder.get().Spec.RestartPolicy = policy
	return builder
}

func (builder *PodBuilder) SetSecurityContext(ctx corev1.PodSecurityContext) *PodBuilder {
	builder.get().Spec.SecurityContext = &ctx
	return builder
}

func (builder *PodBuilder) AddTolerations(tolerations ...corev1.Toleration) *PodBuilder {
	builder.get().Spec.Tolerations = append(builder.get().Spec.Tolerations, tolerations...)
	return builder
}

func (builder *PodBuilder) AddServiceAccount(serviceAccount string) *PodBuilder {
	builder.get().Spec.ServiceAccountName = serviceAccount
	return builder
}

func (builder *PodBuilder) SetNodeSelector(nodeSelector map[string]string) *PodBuilder {
	builder.get().Spec.NodeSelector = nodeSelector
	return builder
}
