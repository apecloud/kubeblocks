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
	corev1 "k8s.io/api/core/v1"
)

type ContainerBuilder struct {
	object *corev1.Container
}

func NewContainerBuilder(name string) *ContainerBuilder {
	builder := &ContainerBuilder{}
	builder.init(name, &corev1.Container{})
	return builder
}

func (builder *ContainerBuilder) init(name string, obj *corev1.Container) {
	obj.Name = name
	builder.object = obj
}

func (builder *ContainerBuilder) get() *corev1.Container {
	return builder.object
}

func (builder *ContainerBuilder) GetObject() *corev1.Container {
	return builder.object
}

func (builder *ContainerBuilder) AddCommands(commands ...string) *ContainerBuilder {
	builder.get().Command = append(builder.get().Command, commands...)
	return builder
}

func (builder *ContainerBuilder) AddArgs(args ...string) *ContainerBuilder {
	builder.get().Args = append(builder.get().Args, args...)
	return builder
}

func (builder *ContainerBuilder) AddEnv(env ...corev1.EnvVar) *ContainerBuilder {
	builder.get().Env = append(builder.get().Env, env...)
	return builder
}

func (builder *ContainerBuilder) SetImage(image string) *ContainerBuilder {
	builder.get().Image = image
	return builder
}

func (builder *ContainerBuilder) SetImagePullPolicy(policy corev1.PullPolicy) *ContainerBuilder {
	builder.get().ImagePullPolicy = policy
	return builder
}

func (builder *ContainerBuilder) AddVolumeMounts(mounts ...corev1.VolumeMount) *ContainerBuilder {
	builder.get().VolumeMounts = append(builder.get().VolumeMounts, mounts...)
	return builder
}

func (builder *ContainerBuilder) SetSecurityContext(ctx corev1.SecurityContext) *ContainerBuilder {
	builder.get().SecurityContext = &ctx
	return builder
}

func (builder *ContainerBuilder) SetResources(resources corev1.ResourceRequirements) *ContainerBuilder {
	builder.get().Resources = resources
	return builder
}

func (builder *ContainerBuilder) AddPorts(ports ...corev1.ContainerPort) *ContainerBuilder {
	builder.get().Ports = append(builder.get().Ports, ports...)
	return builder
}

func (builder *ContainerBuilder) SetReadinessProbe(probe corev1.Probe) *ContainerBuilder {
	builder.get().ReadinessProbe = &probe
	return builder
}

func (builder *ContainerBuilder) SetLivenessProbe(probe corev1.Probe) *ContainerBuilder {
	builder.get().LivenessProbe = &probe
	return builder
}

func (builder *ContainerBuilder) SetStartupProbe(probe corev1.Probe) *ContainerBuilder {
	builder.get().StartupProbe = &probe
	return builder
}
