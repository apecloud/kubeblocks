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

package apps

import (
	"reflect"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

type MockComponentDefinitionFactory struct {
	BaseFactory[kbappsv1.ComponentDefinition, *kbappsv1.ComponentDefinition, MockComponentDefinitionFactory]
}

func NewComponentDefinitionFactory(name string) *MockComponentDefinitionFactory {
	return NewComponentDefinitionFactoryExt(name, "", "", "", "")
}

func NewComponentDefinitionFactoryExt(name, provider, description, serviceKind, serviceVersion string) *MockComponentDefinitionFactory {
	f := &MockComponentDefinitionFactory{}
	f.Init("", name,
		&kbappsv1.ComponentDefinition{
			Spec: kbappsv1.ComponentDefinitionSpec{
				Provider:       provider,
				Description:    description,
				ServiceKind:    serviceKind,
				ServiceVersion: serviceVersion,
			},
		}, f)
	return f
}

func (f *MockComponentDefinitionFactory) SetDescription(description string) *MockComponentDefinitionFactory {
	f.Get().Spec.Description = description
	return f
}

func (f *MockComponentDefinitionFactory) SetServiceKind(serviceKind string) *MockComponentDefinitionFactory {
	f.Get().Spec.ServiceKind = serviceKind
	return f
}

func (f *MockComponentDefinitionFactory) SetServiceVersion(serviceVersion string) *MockComponentDefinitionFactory {
	f.Get().Spec.ServiceVersion = serviceVersion
	return f
}

func (f *MockComponentDefinitionFactory) SetDefaultSpec() *MockComponentDefinitionFactory {
	f.Get().Spec = defaultComponentDefSpec
	return f
}

func (f *MockComponentDefinitionFactory) SetBackupPolicyTemplateName(backupPolicyTemplateName string) *MockComponentDefinitionFactory {
	f.Get().Spec.BackupPolicyTemplateName = backupPolicyTemplateName
	return f
}

// SetRuntime adds a new container to runtime, or updates it to @container if it's already existed.
// If @container is nil, the default MySQL container (defaultMySQLContainer) will be used.
func (f *MockComponentDefinitionFactory) SetRuntime(container *corev1.Container) *MockComponentDefinitionFactory {
	if container == nil {
		container = &defaultMySQLContainer
	}
	if f.Get().Spec.Runtime.Containers == nil {
		f.Get().Spec.Runtime.Containers = make([]corev1.Container, 0)
	}
	for i, it := range f.Get().Spec.Runtime.Containers {
		if it.Name == container.Name {
			f.Get().Spec.Runtime.Containers[i] = *container
			return f
		}
	}
	f.Get().Spec.Runtime.Containers = append(f.Get().Spec.Runtime.Containers, *container)
	return f
}

func (f *MockComponentDefinitionFactory) AddEnv(containerName string, envVar corev1.EnvVar) *MockComponentDefinitionFactory {
	for i, c := range f.Get().Spec.Runtime.Containers {
		if c.Name == containerName {
			f.Get().Spec.Runtime.Containers[i].Env = append(f.Get().Spec.Runtime.Containers[i].Env, envVar)
			break
		}
	}
	return f
}

func (f *MockComponentDefinitionFactory) AddVolumeMounts(containerName string, volumeMounts []corev1.VolumeMount) *MockComponentDefinitionFactory {
	for i, c := range f.Get().Spec.Runtime.Containers {
		if c.Name == containerName {
			mergedAddVolumeMounts(&f.Get().Spec.Runtime.Containers[i], volumeMounts)
			break
		}
	}
	return f
}

func (f *MockComponentDefinitionFactory) AddVar(v kbappsv1.EnvVar) *MockComponentDefinitionFactory {
	if f.Get().Spec.Vars == nil {
		f.Get().Spec.Vars = make([]kbappsv1.EnvVar, 0)
	}
	f.Get().Spec.Vars = append(f.Get().Spec.Vars, v)
	return f
}

func (f *MockComponentDefinitionFactory) AddVolume(name string, snapshot bool, watermark int) *MockComponentDefinitionFactory {
	vol := kbappsv1.ComponentVolume{
		Name:          name,
		NeedSnapshot:  snapshot,
		HighWatermark: watermark,
	}
	if f.Get().Spec.Volumes == nil {
		f.Get().Spec.Volumes = make([]kbappsv1.ComponentVolume, 0)
	}
	f.Get().Spec.Volumes = append(f.Get().Spec.Volumes, vol)
	return f
}

func (f *MockComponentDefinitionFactory) AddHostNetworkContainerPort(container string, ports []string) *MockComponentDefinitionFactory {
	containerPort := kbappsv1.HostNetworkContainerPort{
		Container: container,
		Ports:     ports,
	}
	if f.Get().Spec.HostNetwork == nil {
		f.Get().Spec.HostNetwork = &kbappsv1.HostNetwork{}
	}
	f.Get().Spec.HostNetwork.ContainerPorts = append(f.Get().Spec.HostNetwork.ContainerPorts, containerPort)
	return f
}

func (f *MockComponentDefinitionFactory) AddService(name, serviceName string, port int32, roleSelector string) *MockComponentDefinitionFactory {
	serviceSpec := corev1.ServiceSpec{
		Ports: []corev1.ServicePort{{
			Port: port,
		}},
	}
	return f.AddServiceExt(name, serviceName, serviceSpec, roleSelector)
}

func (f *MockComponentDefinitionFactory) AddServiceExt(name, serviceName string, serviceSpec corev1.ServiceSpec, roleSelector string) *MockComponentDefinitionFactory {
	svc := kbappsv1.ComponentService{
		Service: kbappsv1.Service{
			Name:         name,
			ServiceName:  serviceName,
			Spec:         serviceSpec,
			RoleSelector: roleSelector,
		},
	}
	if f.Get().Spec.Services == nil {
		f.Get().Spec.Services = make([]kbappsv1.ComponentService, 0)
	}
	f.Get().Spec.Services = append(f.Get().Spec.Services, svc)
	return f
}

func (f *MockComponentDefinitionFactory) AddConfigTemplate(name, configTemplateRef, configConstraintRef,
	namespace, volumeName string, injectEnvTo ...string) *MockComponentDefinitionFactory {
	config := kbappsv1.ComponentConfigSpec{
		ComponentTemplateSpec: kbappsv1.ComponentTemplateSpec{
			Name:        name,
			TemplateRef: configTemplateRef,
			Namespace:   namespace,
			VolumeName:  volumeName,
		},
		ConfigConstraintRef: configConstraintRef,
		InjectEnvTo:         injectEnvTo,
	}
	if f.Get().Spec.Configs == nil {
		f.Get().Spec.Configs = make([]kbappsv1.ComponentConfigSpec, 0)
	}
	f.Get().Spec.Configs = append(f.Get().Spec.Configs, config)
	return f
}

func (f *MockComponentDefinitionFactory) AddConfigs(configs []kbappsv1.ComponentConfigSpec) *MockComponentDefinitionFactory {
	if f.Get().Spec.Configs == nil {
		f.Get().Spec.Configs = make([]kbappsv1.ComponentConfigSpec, 0)
	}
	f.Get().Spec.Configs = append(f.Get().Spec.Configs, configs...)
	return f
}

func (f *MockComponentDefinitionFactory) AddScripts(scripts []kbappsv1.ComponentTemplateSpec) *MockComponentDefinitionFactory {
	if f.Get().Spec.Scripts == nil {
		f.Get().Spec.Scripts = make([]kbappsv1.ComponentTemplateSpec, 0)
	}
	f.Get().Spec.Scripts = append(f.Get().Spec.Scripts, scripts...)
	return f
}

func (f *MockComponentDefinitionFactory) AddLogConfig(name, filePathPattern string) *MockComponentDefinitionFactory {
	logConfig := kbappsv1.LogConfig{
		FilePathPattern: filePathPattern,
		Name:            name,
	}
	if f.Get().Spec.LogConfigs == nil {
		f.Get().Spec.LogConfigs = make([]kbappsv1.LogConfig, 0)
	}
	f.Get().Spec.LogConfigs = append(f.Get().Spec.LogConfigs, logConfig)
	return f
}

// func (f *MockComponentDefinitionFactory) SetMonitor(builtIn bool, scrapePort intstr.IntOrString, scrapePath string) *MockComponentDefinitionFactory {
// 	f.Get().Spec.Monitor = &kbappsv1.MonitorConfig{
// 		BuiltIn: builtIn,
// 		Exporter: &kbappsv1.ExporterConfig{
// 			ScrapePort: scrapePort,
// 			ScrapePath: scrapePath,
// 		},
// 	}
// 	return f
// }

func (f *MockComponentDefinitionFactory) AddScriptTemplate(name, configTemplateRef, namespace, volumeName string,
	mode *int32) *MockComponentDefinitionFactory {
	script := kbappsv1.ComponentTemplateSpec{
		Name:        name,
		TemplateRef: configTemplateRef,
		Namespace:   namespace,
		VolumeName:  volumeName,
		DefaultMode: mode,
	}
	if f.Get().Spec.Scripts == nil {
		f.Get().Spec.Scripts = make([]kbappsv1.ComponentTemplateSpec, 0)
	}
	f.Get().Spec.Scripts = append(f.Get().Spec.Scripts, script)
	return f
}

func (f *MockComponentDefinitionFactory) SetPolicyRules(rules []rbacv1.PolicyRule) *MockComponentDefinitionFactory {
	f.Get().Spec.PolicyRules = rules
	return f
}

func (f *MockComponentDefinitionFactory) SetLabels(labels map[string]string) *MockComponentDefinitionFactory {
	f.Get().Spec.Labels = labels
	return f
}

func (f *MockComponentDefinitionFactory) SetReplicasLimit(minReplicas, maxReplicas int32) *MockComponentDefinitionFactory {
	f.Get().Spec.ReplicasLimit = &kbappsv1.ReplicasLimit{
		MinReplicas: minReplicas,
		MaxReplicas: maxReplicas,
	}
	return f
}

func (f *MockComponentDefinitionFactory) AddSystemAccount(accountName string, initAccount bool, statement string) *MockComponentDefinitionFactory {
	account := kbappsv1.SystemAccount{
		Name:        accountName,
		InitAccount: initAccount,
		Statement:   statement,
	}
	if f.Get().Spec.SystemAccounts == nil {
		f.Get().Spec.SystemAccounts = make([]kbappsv1.SystemAccount, 0)
	}
	f.Get().Spec.SystemAccounts = append(f.Get().Spec.SystemAccounts, account)
	return f
}

func (f *MockComponentDefinitionFactory) SetUpdateStrategy(strategy *kbappsv1.UpdateStrategy) *MockComponentDefinitionFactory {
	f.Get().Spec.UpdateStrategy = strategy
	return f
}

func (f *MockComponentDefinitionFactory) SetPodManagementPolicy(policy *appsv1.PodManagementPolicyType) *MockComponentDefinitionFactory {
	f.Get().Spec.PodManagementPolicy = policy
	return f
}

func (f *MockComponentDefinitionFactory) AddRole(name string, serviceable, writable bool) *MockComponentDefinitionFactory {
	role := kbappsv1.ReplicaRole{
		Name:        name,
		Serviceable: serviceable,
		Writable:    writable,
	}
	if f.Get().Spec.Roles == nil {
		f.Get().Spec.Roles = make([]kbappsv1.ReplicaRole, 0)
	}
	f.Get().Spec.Roles = append(f.Get().Spec.Roles, role)
	return f
}

func (f *MockComponentDefinitionFactory) SetLifecycleAction(name string, val interface{}) *MockComponentDefinitionFactory {
	if f.Get().Spec.LifecycleActions == nil {
		f.Get().Spec.LifecycleActions = &kbappsv1.ComponentLifecycleActions{}
	}
	obj := f.Get().Spec.LifecycleActions
	t := reflect.TypeOf(obj).Elem()
	for i := 0; i < t.NumField(); i++ {
		fieldName := t.Field(i).Name
		if strings.EqualFold(fieldName, name) {
			fieldValue := reflect.ValueOf(obj).Elem().FieldByName(fieldName)
			switch {
			case reflect.TypeOf(val) == nil || reflect.ValueOf(val).IsZero():
				fieldValue.Set(reflect.Zero(fieldValue.Type()))
			case fieldValue.IsValid() && fieldValue.Type().AssignableTo(reflect.TypeOf(val)):
				fieldValue.Set(reflect.ValueOf(val))
			default:
				panic("not assignable")
			}
			break
		}
	}
	return f
}

func (f *MockComponentDefinitionFactory) AddServiceRef(name, serviceKind, serviceVersion string) *MockComponentDefinitionFactory {
	serviceRef := kbappsv1.ServiceRefDeclaration{
		Name: name,
		ServiceRefDeclarationSpecs: []kbappsv1.ServiceRefDeclarationSpec{
			{
				ServiceKind:    serviceKind,
				ServiceVersion: serviceVersion,
			},
		},
	}
	if f.Get().Spec.ServiceRefDeclarations == nil {
		f.Get().Spec.ServiceRefDeclarations = make([]kbappsv1.ServiceRefDeclaration, 0)
	}
	f.Get().Spec.ServiceRefDeclarations = append(f.Get().Spec.ServiceRefDeclarations, serviceRef)
	return f
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
