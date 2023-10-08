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
	f.init("", name,
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

// SetRuntime adds a new container to runtime, or updates it to @container if it's already existed.
// If @container is nil, the default MySQL container (defaultMySQLContainer) will be used.
func (f *MockComponentDefinitionFactory) SetRuntime(container *corev1.Container) *MockComponentDefinitionFactory {
	if container == nil {
		container = &defaultMySQLContainer
	}
	if f.get().Spec.Runtime.Containers == nil {
		f.get().Spec.Runtime.Containers = make([]corev1.Container, 0)
	}
	for i, it := range f.get().Spec.Runtime.Containers {
		if it.Name == container.Name {
			f.get().Spec.Runtime.Containers[i] = *container
			return f
		}
	}
	f.get().Spec.Runtime.Containers = append(f.get().Spec.Runtime.Containers, *container)
	return f
}

func (f *MockComponentDefinitionFactory) AddEnv(containerName string, envVar corev1.EnvVar) *MockComponentDefinitionFactory {
	for i, c := range f.get().Spec.Runtime.Containers {
		if c.Name == containerName {
			f.get().Spec.Runtime.Containers[i].Env = append(f.get().Spec.Runtime.Containers[i].Env, envVar)
			break
		}
	}
	return f
}

func (f *MockComponentDefinitionFactory) AddVolumeMounts(containerName string, volumeMounts []corev1.VolumeMount) *MockComponentDefinitionFactory {
	for i, c := range f.get().Spec.Runtime.Containers {
		if c.Name == containerName {
			mergedAddVolumeMounts(&f.get().Spec.Runtime.Containers[i], volumeMounts)
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
	if f.get().Spec.Volumes == nil {
		f.get().Spec.Volumes = make([]appsv1alpha1.ComponentVolume, 0)
	}
	f.get().Spec.Volumes = append(f.get().Spec.Volumes, vol)
	return f
}

func (f *MockComponentDefinitionFactory) AddService(name, serviceName string, port int32, roleSelector []string) *MockComponentDefinitionFactory {
	serviceSpec := corev1.ServiceSpec{
		Ports: []corev1.ServicePort{{
			Port: port,
		}},
	}
	return f.AddServiceExt(name, serviceName, roleSelector, serviceSpec)
}

func (f *MockComponentDefinitionFactory) AddServiceExt(name, serviceName string, roleSelector []string, serviceSpec corev1.ServiceSpec) *MockComponentDefinitionFactory {
	svc := appsv1alpha1.ComponentService{
		Name:         name,
		ServiceName:  appsv1alpha1.BuiltInString(serviceName),
		ServiceSpec:  serviceSpec,
		RoleSelector: roleSelector,
	}
	if f.get().Spec.Services == nil {
		f.get().Spec.Services = make([]appsv1alpha1.ComponentService, 0)
	}
	f.get().Spec.Services = append(f.get().Spec.Services, svc)
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
	if f.get().Spec.Configs == nil {
		f.get().Spec.Configs = make([]appsv1alpha1.ComponentConfigSpec, 0)
	}
	f.get().Spec.Configs = append(f.get().Spec.Configs, config)
	return f
}

func (f *MockComponentDefinitionFactory) AddLogConfig(name, filePathPattern string) *MockComponentDefinitionFactory {
	logConfig := appsv1alpha1.LogConfig{
		FilePathPattern: filePathPattern,
		Name:            name,
	}
	if f.get().Spec.LogConfigs == nil {
		f.get().Spec.LogConfigs = make([]appsv1alpha1.LogConfig, 0)
	}
	f.get().Spec.LogConfigs = append(f.get().Spec.LogConfigs, logConfig)
	return f
}

func (f *MockComponentDefinitionFactory) SetMonitor(builtIn bool, scrapePort intstr.IntOrString, scrapePath string) *MockComponentDefinitionFactory {
	f.get().Spec.Monitor = &appsv1alpha1.MonitorConfig{
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
	if f.get().Spec.Scripts == nil {
		f.get().Spec.Scripts = make([]appsv1alpha1.ComponentTemplateSpec, 0)
	}
	f.get().Spec.Scripts = append(f.get().Spec.Scripts, script)
	return f
}

func (f *MockComponentDefinitionFactory) SetPolicyRules(rules []rbacv1.PolicyRule) *MockComponentDefinitionFactory {
	f.get().Spec.PolicyRules = rules
	return f
}

func (f *MockComponentDefinitionFactory) SetLabels(labels map[string]appsv1alpha1.BuiltInString) *MockComponentDefinitionFactory {
	f.get().Spec.Labels = labels
	return f
}

func (f *MockComponentDefinitionFactory) AddSystemAccount(accountName string, isSystemInitAccount bool, statement string) *MockComponentDefinitionFactory {
	account := appsv1alpha1.ComponentSystemAccount{
		Name:                accountName,
		IsSystemInitAccount: isSystemInitAccount,
		Statement:           statement,
	}
	if f.get().Spec.SystemAccounts == nil {
		f.get().Spec.SystemAccounts = make([]appsv1alpha1.ComponentSystemAccount, 0)
	}
	f.get().Spec.SystemAccounts = append(f.get().Spec.SystemAccounts, account)
	return f
}

func (f *MockComponentDefinitionFactory) AddConnectionCredential(name, serviceName, portName, accountName string) *MockComponentDefinitionFactory {
	credential := appsv1alpha1.ConnectionCredential{
		Name:        name,
		ServiceName: serviceName,
		PortName:    portName,
		AccountName: accountName,
	}
	if f.get().Spec.ConnectionCredentials == nil {
		f.get().Spec.ConnectionCredentials = make([]appsv1alpha1.ConnectionCredential, 0)
	}
	f.get().Spec.ConnectionCredentials = append(f.get().Spec.ConnectionCredentials, credential)
	return f
}

func (f *MockComponentDefinitionFactory) SetUpdateStrategy(strategy *appsv1alpha1.UpdateStrategy) *MockComponentDefinitionFactory {
	f.get().Spec.UpdateStrategy = strategy
	return f
}

func (f *MockComponentDefinitionFactory) AddRole(name string, serviceable, writable bool) *MockComponentDefinitionFactory {
	role := appsv1alpha1.ComponentReplicaRole{
		Name:        name,
		Serviceable: serviceable,
		Writable:    writable,
	}
	if f.get().Spec.Roles == nil {
		f.get().Spec.Roles = make([]appsv1alpha1.ComponentReplicaRole, 0)
	}
	f.get().Spec.Roles = append(f.get().Spec.Roles, role)
	return f
}

func (f *MockComponentDefinitionFactory) SetRoleArbitrator(arbitrator *appsv1alpha1.ComponentRoleArbitrator) *MockComponentDefinitionFactory {
	f.get().Spec.RoleArbitrator = arbitrator
	return f
}

func (f *MockComponentDefinitionFactory) SetLifecycleAction(name string, val interface{}) *MockComponentDefinitionFactory {
	obj := &f.get().Spec.LifecycleActions
	t := reflect.TypeOf(reflect.ValueOf(obj).Elem())
	for i := 0; i < t.NumField(); i++ {
		fieldName := t.Field(i).Name
		if strings.ToUpper(fieldName) == strings.ToUpper(name) {
			fieldValue := reflect.ValueOf(obj).Elem().FieldByName(fieldName)
			if fieldValue.IsValid() && fieldValue.Type().AssignableTo(reflect.TypeOf(val)) {
				fieldValue.Set(reflect.ValueOf(val))
			} else {
				panic("not assignable")
			}
			break
		}
	}
	return f
}
