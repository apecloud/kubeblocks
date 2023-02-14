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

package dbaas

import (
	corev1 "k8s.io/api/core/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

type ComponentTplType string

const (
	StatefulMySQLComponent    ComponentTplType = "stateful-mysql"
	ConsensusMySQLComponent   ComponentTplType = "consensus-mysql"
	ReplicationRedisComponent ComponentTplType = "replication-redis"
	StatelessNginxComponent   ComponentTplType = "stateless-nginx"
)

type MockClusterDefFactory struct {
	BaseFactory[dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterDefinition, MockClusterDefFactory]
}

func NewClusterDefFactory(name, cdType string) *MockClusterDefFactory {
	f := &MockClusterDefFactory{}
	f.init("", name,
		&dbaasv1alpha1.ClusterDefinition{
			Spec: dbaasv1alpha1.ClusterDefinitionSpec{
				Type:       cdType,
				Components: []dbaasv1alpha1.ClusterDefinitionComponent{},
			},
		}, f)
	f.SetConnectionCredential(defaultConnectionCredential)
	return f
}

func (factory *MockClusterDefFactory) AddComponent(tplType ComponentTplType, rename string) *MockClusterDefFactory {
	var component *dbaasv1alpha1.ClusterDefinitionComponent
	switch tplType {
	case StatefulMySQLComponent:
		component = &statefulMySQLComponent
	case ConsensusMySQLComponent:
		component = &consensusMySQLComponent
	case ReplicationRedisComponent:
		component = &replicationRedisComponent
	case StatelessNginxComponent:
		component = &statelessNginxComponent
	}
	comps := factory.get().Spec.Components
	comps = append(comps, *component)
	comps[len(comps)-1].TypeName = rename
	factory.get().Spec.Components = comps
	return factory
}

func (factory *MockClusterDefFactory) SetDefaultReplicas(replicas int32) *MockClusterDefFactory {
	comps := factory.get().Spec.Components
	if len(comps) > 0 {
		comps[len(comps)-1].DefaultReplicas = replicas
	}
	factory.get().Spec.Components = comps
	return factory
}

func (factory *MockClusterDefFactory) SetService(port int32) *MockClusterDefFactory {
	comps := factory.get().Spec.Components
	if len(comps) > 0 {
		comps[len(comps)-1].Service = &corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Protocol: corev1.ProtocolTCP,
				Port:     port,
			}},
		}
	}
	factory.get().Spec.Components = comps
	return factory
}

func (factory *MockClusterDefFactory) AddConfigTemplate(name string,
	configTplRef string, configConstraintRef string, volumeName string, mode *int32) *MockClusterDefFactory {
	comps := factory.get().Spec.Components
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		if comp.ConfigSpec == nil {
			comp.ConfigSpec = &dbaasv1alpha1.ConfigurationSpec{}
		}
		comp.ConfigSpec.ConfigTemplateRefs = append(comp.ConfigSpec.ConfigTemplateRefs,
			dbaasv1alpha1.ConfigTemplate{
				Name:                name,
				ConfigTplRef:        configTplRef,
				ConfigConstraintRef: configConstraintRef,
				VolumeName:          volumeName,
				DefaultMode:         mode,
			})
		comps[len(comps)-1] = comp
	}
	factory.get().Spec.Components = comps
	return factory
}

func (factory *MockClusterDefFactory) AddContainerEnv(containerName string, envVar corev1.EnvVar) *MockClusterDefFactory {
	comps := factory.get().Spec.Components
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		for i, container := range comps[len(comps)-1].PodSpec.Containers {
			if container.Name == containerName {
				c := comps[len(comps)-1].PodSpec.Containers[i]
				c.Env = append(c.Env, envVar)
				comps[len(comps)-1].PodSpec.Containers[i] = c
				break
			}
		}
		comps[len(comps)-1] = comp
	}
	factory.get().Spec.Components = comps
	return factory
}

func (factory *MockClusterDefFactory) SetConnectionCredential(
	connectionCredential map[string]string) *MockClusterDefFactory {
	factory.get().Spec.ConnectionCredential = connectionCredential
	return factory
}

func (factory *MockClusterDefFactory) AddSystemAccountSpec(sysAccounts *dbaasv1alpha1.SystemAccountSpec) *MockClusterDefFactory {
	comps := factory.get().Spec.Components
	if len(comps) == 0 {
		return factory
	}

	comp := comps[len(comps)-1]
	comp.SystemAccounts = sysAccounts
	comps[len(comps)-1] = comp
	factory.get().Spec.Components = comps
	return factory
}

func (factory *MockClusterDefFactory) AddInitContainerVolumeMounts(containerName string, volumeMounts []corev1.VolumeMount) *MockClusterDefFactory {
	comps := factory.get().Spec.Components
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		comp.PodSpec.InitContainers = setContainerVolumeMounts(comp.PodSpec.InitContainers, containerName, volumeMounts)
		comps[len(comps)-1] = comp
	}
	factory.get().Spec.Components = comps
	return factory
}

func (factory *MockClusterDefFactory) AddContainerVolumeMounts(containerName string, volumeMounts []corev1.VolumeMount) *MockClusterDefFactory {
	comps := factory.get().Spec.Components
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		comp.PodSpec.Containers = setContainerVolumeMounts(comp.PodSpec.Containers, containerName, volumeMounts)
		comps[len(comps)-1] = comp
	}
	factory.get().Spec.Components = comps
	return factory
}

func setContainerVolumeMounts(containers []corev1.Container, targetContainerName string, volumeMounts []corev1.VolumeMount) []corev1.Container {
	for _, container := range containers {
		if container.Name == targetContainerName {
			container.VolumeMounts = append(container.VolumeMounts, volumeMounts...)
		}
	}
	return containers
}
