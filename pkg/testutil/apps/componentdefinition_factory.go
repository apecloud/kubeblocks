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

package apps

import (
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type MockComponentDefinitionFactory struct {
	BaseFactory[appsv1alpha1.ComponentDefinition, *appsv1alpha1.ComponentDefinition, MockComponentDefinitionFactory]
}

func NewComponentDefinitionFactory(name string) *MockComponentDefinitionFactory {
	return NewComponentDefinitionFactoryExt(name, "", "", "", "")
}

func NewComponentDefinitionFactoryExt(name, provider, description, serviceKind, serviceVersion string) *MockComponentDefinitionFactory {
	f := &MockComponentDefinitionFactory{}
	f.Init("", name,
		&appsv1alpha1.ComponentDefinition{
			Spec: appsv1alpha1.ComponentDefinitionSpec{
				Provider:       provider,
				Description:    description,
				ServiceKind:    serviceKind,
				ServiceVersion: serviceVersion,
			},
		}, f)
	return f
}

func (f *MockComponentDefinitionFactory) SetDefaultSpec() *MockComponentDefinitionFactory {
	f.Get().Spec = defaultComponentDefSpec
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

func (f *MockComponentDefinitionFactory) AddVolume(name string, snapshot bool, watermark int) *MockComponentDefinitionFactory {
	vol := appsv1alpha1.ComponentVolume{
		Name:          name,
		NeedSnapshot:  snapshot,
		HighWatermark: watermark,
	}
	if f.Get().Spec.Volumes == nil {
		f.Get().Spec.Volumes = make([]appsv1alpha1.ComponentVolume, 0)
	}
	f.Get().Spec.Volumes = append(f.Get().Spec.Volumes, vol)
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
	svc := appsv1alpha1.Service{
		Name:         name,
		ServiceName:  serviceName,
		Spec:         serviceSpec,
		RoleSelector: roleSelector,
	}
	if f.Get().Spec.Services == nil {
		f.Get().Spec.Services = make([]appsv1alpha1.Service, 0)
	}
	f.Get().Spec.Services = append(f.Get().Spec.Services, svc)
	return f
}

func (f *MockComponentDefinitionFactory) AddConfigTemplate(name, configTemplateRef, configConstraintRef,
	namespace, volumeName string, asEnvFrom ...string) *MockComponentDefinitionFactory {
	config := appsv1alpha1.ComponentConfigSpec{
		ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
			Name:        name,
			TemplateRef: configTemplateRef,
			Namespace:   namespace,
			VolumeName:  volumeName,
		},
		ConfigConstraintRef: configConstraintRef,
		AsEnvFrom:           asEnvFrom,
	}
	if f.Get().Spec.Configs == nil {
		f.Get().Spec.Configs = make([]appsv1alpha1.ComponentConfigSpec, 0)
	}
	f.Get().Spec.Configs = append(f.Get().Spec.Configs, config)
	return f
}

func (f *MockComponentDefinitionFactory) AddConfigs(configs []appsv1alpha1.ComponentConfigSpec) *MockComponentDefinitionFactory {
	if f.Get().Spec.Configs == nil {
		f.Get().Spec.Configs = make([]appsv1alpha1.ComponentConfigSpec, 0)
	}
	f.Get().Spec.Configs = append(f.Get().Spec.Configs, configs...)
	return f
}

func (f *MockComponentDefinitionFactory) AddScripts(scripts []appsv1alpha1.ComponentTemplateSpec) *MockComponentDefinitionFactory {
	if f.Get().Spec.Scripts == nil {
		f.Get().Spec.Scripts = make([]appsv1alpha1.ComponentTemplateSpec, 0)
	}
	f.Get().Spec.Scripts = append(f.Get().Spec.Scripts, scripts...)
	return f
}

func (f *MockComponentDefinitionFactory) AddLogConfig(name, filePathPattern string) *MockComponentDefinitionFactory {
	logConfig := appsv1alpha1.LogConfig{
		FilePathPattern: filePathPattern,
		Name:            name,
	}
	if f.Get().Spec.LogConfigs == nil {
		f.Get().Spec.LogConfigs = make([]appsv1alpha1.LogConfig, 0)
	}
	f.Get().Spec.LogConfigs = append(f.Get().Spec.LogConfigs, logConfig)
	return f
}

func (f *MockComponentDefinitionFactory) SetMonitor(builtIn bool, scrapePort intstr.IntOrString, scrapePath string) *MockComponentDefinitionFactory {
	f.Get().Spec.Monitor = &appsv1alpha1.MonitorConfig{
		BuiltIn: builtIn,
		Exporter: &appsv1alpha1.ExporterConfig{
			ScrapePort: scrapePort,
			ScrapePath: scrapePath,
		},
	}
	return f
}

func (f *MockComponentDefinitionFactory) AddScriptTemplate(name, configTemplateRef, namespace, volumeName string,
	mode *int32) *MockComponentDefinitionFactory {
	script := appsv1alpha1.ComponentTemplateSpec{
		Name:        name,
		TemplateRef: configTemplateRef,
		Namespace:   namespace,
		VolumeName:  volumeName,
		DefaultMode: mode,
	}
	if f.Get().Spec.Scripts == nil {
		f.Get().Spec.Scripts = make([]appsv1alpha1.ComponentTemplateSpec, 0)
	}
	f.Get().Spec.Scripts = append(f.Get().Spec.Scripts, script)
	return f
}

func (f *MockComponentDefinitionFactory) SetPolicyRules(rules []rbacv1.PolicyRule) *MockComponentDefinitionFactory {
	f.Get().Spec.PolicyRules = rules
	return f
}

func (f *MockComponentDefinitionFactory) SetLabels(labels map[string]appsv1alpha1.BuiltInString) *MockComponentDefinitionFactory {
	f.Get().Spec.Labels = labels
	return f
}

func (f *MockComponentDefinitionFactory) AddSystemAccount(accountName string, initAccount bool, statement string) *MockComponentDefinitionFactory {
	account := appsv1alpha1.SystemAccount{
		Name:        accountName,
		InitAccount: initAccount,
		Statement:   statement,
	}
	if f.Get().Spec.SystemAccounts == nil {
		f.Get().Spec.SystemAccounts = make([]appsv1alpha1.SystemAccount, 0)
	}
	f.Get().Spec.SystemAccounts = append(f.Get().Spec.SystemAccounts, account)
	return f
}

func (f *MockComponentDefinitionFactory) AddConnectionCredential(name, serviceName, portName, accountName string) *MockComponentDefinitionFactory {
	credential := appsv1alpha1.ConnectionCredential{
		Name:        name,
		ServiceName: serviceName,
		PortName:    portName,
		AccountName: accountName,
	}
	if f.Get().Spec.ConnectionCredentials == nil {
		f.Get().Spec.ConnectionCredentials = make([]appsv1alpha1.ConnectionCredential, 0)
	}
	f.Get().Spec.ConnectionCredentials = append(f.Get().Spec.ConnectionCredentials, credential)
	return f
}

func (f *MockComponentDefinitionFactory) SetUpdateStrategy(strategy *appsv1alpha1.UpdateStrategy) *MockComponentDefinitionFactory {
	f.Get().Spec.UpdateStrategy = strategy
	return f
}

func (f *MockComponentDefinitionFactory) AddRole(name string, serviceable, writable bool) *MockComponentDefinitionFactory {
	role := appsv1alpha1.ReplicaRole{
		Name:        name,
		Serviceable: serviceable,
		Writable:    writable,
	}
	if f.Get().Spec.Roles == nil {
		f.Get().Spec.Roles = make([]appsv1alpha1.ReplicaRole, 0)
	}
	f.Get().Spec.Roles = append(f.Get().Spec.Roles, role)
	return f
}

func (f *MockComponentDefinitionFactory) SetRoleArbitrator(arbitrator *appsv1alpha1.RoleArbitrator) *MockComponentDefinitionFactory {
	f.Get().Spec.RoleArbitrator = arbitrator
	return f
}

func (f *MockComponentDefinitionFactory) SetLifecycleAction(name string, val interface{}) *MockComponentDefinitionFactory {
	if f.Get().Spec.LifecycleActions == nil {
		f.Get().Spec.LifecycleActions = &appsv1alpha1.ComponentLifecycleActions{}
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

func (f *MockComponentDefinitionFactory) AddContainerVolumeMounts(containerName string, volumeMounts []corev1.VolumeMount) *MockComponentDefinitionFactory {
	f.Get().Spec.Runtime.Containers = appendContainerVolumeMounts(f.Get().Spec.Runtime.Containers, containerName, volumeMounts)
	return f
}
