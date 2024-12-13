/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

type ComponentDefinitionBuilder struct {
	BaseBuilder[appsv1.ComponentDefinition, *appsv1.ComponentDefinition, ComponentDefinitionBuilder]
}

func NewComponentDefinitionBuilder(name string) *ComponentDefinitionBuilder {
	builder := &ComponentDefinitionBuilder{}
	builder.init("", name,
		&appsv1.ComponentDefinition{
			Spec: appsv1.ComponentDefinitionSpec{},
		}, builder)
	return builder
}

// SetRuntime adds a new container to runtime, or updates it to @container if it's already existed.
// If @container is nil, the default MySQL container (defaultMySQLContainer) will be used.
func (builder *ComponentDefinitionBuilder) SetRuntime(container *corev1.Container) *ComponentDefinitionBuilder {
	if container == nil {
		return builder
	}
	if builder.get().Spec.Runtime.Containers == nil {
		builder.get().Spec.Runtime.Containers = make([]corev1.Container, 0)
	}
	for i, it := range builder.get().Spec.Runtime.Containers {
		if it.Name == container.Name {
			builder.get().Spec.Runtime.Containers[i] = *container
			return builder
		}
	}
	builder.get().Spec.Runtime.Containers = append(builder.get().Spec.Runtime.Containers, *container)
	return builder
}

func (builder *ComponentDefinitionBuilder) AddEnv(containerName string, envVar corev1.EnvVar) *ComponentDefinitionBuilder {
	for i, c := range builder.get().Spec.Runtime.Containers {
		if c.Name == containerName {
			builder.get().Spec.Runtime.Containers[i].Env = append(builder.get().Spec.Runtime.Containers[i].Env, envVar)
			break
		}
	}
	return builder
}

func (builder *ComponentDefinitionBuilder) AddVolumeMounts(containerName string, volumeMounts []corev1.VolumeMount) *ComponentDefinitionBuilder {
	for i, c := range builder.get().Spec.Runtime.Containers {
		if c.Name == containerName {
			mergedAddVolumeMounts(&builder.get().Spec.Runtime.Containers[i], volumeMounts)
			break
		}
	}
	return builder
}

func (builder *ComponentDefinitionBuilder) AddVar(v appsv1.EnvVar) *ComponentDefinitionBuilder {
	if builder.get().Spec.Vars == nil {
		builder.get().Spec.Vars = make([]appsv1.EnvVar, 0)
	}
	builder.get().Spec.Vars = append(builder.get().Spec.Vars, v)
	return builder
}

func (builder *ComponentDefinitionBuilder) AddVolume(name string, snapshot bool, watermark int) *ComponentDefinitionBuilder {
	vol := appsv1.ComponentVolume{
		Name:          name,
		NeedSnapshot:  snapshot,
		HighWatermark: watermark,
	}
	if builder.get().Spec.Volumes == nil {
		builder.get().Spec.Volumes = make([]appsv1.ComponentVolume, 0)
	}
	builder.get().Spec.Volumes = append(builder.get().Spec.Volumes, vol)
	return builder
}

func (builder *ComponentDefinitionBuilder) AddService(name, serviceName string, port int32, roleSelector string) *ComponentDefinitionBuilder {
	serviceSpec := corev1.ServiceSpec{
		Ports: []corev1.ServicePort{{
			Port: port,
		}},
	}
	return builder.AddServiceExt(name, serviceName, serviceSpec, roleSelector)
}

func (builder *ComponentDefinitionBuilder) AddServiceExt(name, serviceName string, serviceSpec corev1.ServiceSpec, roleSelector string) *ComponentDefinitionBuilder {
	svc := appsv1.ComponentService{
		Service: appsv1.Service{
			Name:         name,
			ServiceName:  serviceName,
			Spec:         serviceSpec,
			RoleSelector: roleSelector,
		},
	}
	if builder.get().Spec.Services == nil {
		builder.get().Spec.Services = make([]appsv1.ComponentService, 0)
	}
	builder.get().Spec.Services = append(builder.get().Spec.Services, svc)
	return builder
}

func (builder *ComponentDefinitionBuilder) AddConfigTemplate(name, configTemplateRef, configConstraintRef,
	namespace, volumeName string, injectEnvTo ...string) *ComponentDefinitionBuilder {
	config := appsv1.ComponentConfigSpec{
		ComponentTemplateSpec: appsv1.ComponentTemplateSpec{
			Name:        name,
			TemplateRef: configTemplateRef,
			Namespace:   namespace,
			VolumeName:  volumeName,
		},
		ConfigConstraintRef: configConstraintRef,
		InjectEnvTo:         injectEnvTo,
	}
	if builder.get().Spec.Configs == nil {
		builder.get().Spec.Configs = make([]appsv1.ComponentConfigSpec, 0)
	}
	builder.get().Spec.Configs = append(builder.get().Spec.Configs, config)
	return builder
}

func (builder *ComponentDefinitionBuilder) AddLogConfig(name, filePathPattern string) *ComponentDefinitionBuilder {
	logConfig := appsv1.LogConfig{
		FilePathPattern: filePathPattern,
		Name:            name,
	}
	if builder.get().Spec.LogConfigs == nil {
		builder.get().Spec.LogConfigs = make([]appsv1.LogConfig, 0)
	}
	builder.get().Spec.LogConfigs = append(builder.get().Spec.LogConfigs, logConfig)
	return builder
}

func (builder *ComponentDefinitionBuilder) AddScriptTemplate(name, configTemplateRef, namespace, volumeName string,
	mode *int32) *ComponentDefinitionBuilder {
	script := appsv1.ComponentTemplateSpec{
		Name:        name,
		TemplateRef: configTemplateRef,
		Namespace:   namespace,
		VolumeName:  volumeName,
		DefaultMode: mode,
	}
	if builder.get().Spec.Scripts == nil {
		builder.get().Spec.Scripts = make([]appsv1.ComponentTemplateSpec, 0)
	}
	builder.get().Spec.Scripts = append(builder.get().Spec.Scripts, script)
	return builder
}

func (builder *ComponentDefinitionBuilder) SetPolicyRules(rules []rbacv1.PolicyRule) *ComponentDefinitionBuilder {
	builder.get().Spec.PolicyRules = rules
	return builder
}

func (builder *ComponentDefinitionBuilder) SetLabels(labels map[string]string) *ComponentDefinitionBuilder {
	builder.get().Spec.Labels = labels
	return builder
}

func (builder *ComponentDefinitionBuilder) SetReplicasLimit(minReplicas, maxReplicas int32) *ComponentDefinitionBuilder {
	builder.get().Spec.ReplicasLimit = &appsv1.ReplicasLimit{
		MinReplicas: minReplicas,
		MaxReplicas: maxReplicas,
	}
	return builder
}

func (builder *ComponentDefinitionBuilder) AddSystemAccount(accountName string, initAccount bool, statement string) *ComponentDefinitionBuilder {
	account := appsv1.SystemAccount{
		Name:        accountName,
		InitAccount: initAccount,
		Statement:   statement,
	}
	if builder.get().Spec.SystemAccounts == nil {
		builder.get().Spec.SystemAccounts = make([]appsv1.SystemAccount, 0)
	}
	builder.get().Spec.SystemAccounts = append(builder.get().Spec.SystemAccounts, account)
	return builder
}

func (builder *ComponentDefinitionBuilder) SetUpdateStrategy(strategy *appsv1.UpdateStrategy) *ComponentDefinitionBuilder {
	builder.get().Spec.UpdateStrategy = strategy
	return builder
}

func (builder *ComponentDefinitionBuilder) AddRole(
	name string, updatePriority int, participatesInQuorum bool,
) *ComponentDefinitionBuilder {
	role := appsv1.ReplicaRole{
		Name:                 name,
		UpdatePriority:       updatePriority,
		ParticipatesInQuorum: participatesInQuorum,
	}
	if builder.get().Spec.Roles == nil {
		builder.get().Spec.Roles = make([]appsv1.ReplicaRole, 0)
	}
	builder.get().Spec.Roles = append(builder.get().Spec.Roles, role)
	return builder
}

func (builder *ComponentDefinitionBuilder) SetLifecycleAction(name string, val interface{}) *ComponentDefinitionBuilder {
	obj := &builder.get().Spec.LifecycleActions
	t := reflect.TypeOf(reflect.ValueOf(obj).Elem())
	for i := 0; i < t.NumField(); i++ {
		fieldName := t.Field(i).Name
		if strings.EqualFold(fieldName, name) {
			fieldValue := reflect.ValueOf(obj).Elem().FieldByName(fieldName)
			if fieldValue.IsValid() && fieldValue.Type().AssignableTo(reflect.TypeOf(val)) {
				fieldValue.Set(reflect.ValueOf(val))
			} else {
				panic("not assignable")
			}
			break
		}
	}
	return builder
}

func mergedAddVolumeMounts(c *corev1.Container, volumeMounts []corev1.VolumeMount) {
	table := make(map[string]corev1.VolumeMount)
	for _, v := range c.VolumeMounts {
		table[v.Name] = v
	}
	for _, v := range volumeMounts {
		table[v.Name] = v
	}

	mounts := make([]corev1.VolumeMount, 0)
	for _, v := range table {
		mounts = append(mounts, v)
	}
	c.VolumeMounts = mounts
}
