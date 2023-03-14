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

package apps

import (
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type ComponentTplType string

const (
	StatefulMySQLComponent    ComponentTplType = "stateful-mysql"
	ConsensusMySQLComponent   ComponentTplType = "consensus-mysql"
	ReplicationRedisComponent ComponentTplType = "replication-redis"
	StatelessNginxComponent   ComponentTplType = "stateless-nginx"
)

type MockClusterDefFactory struct {
	BaseFactory[appsv1alpha1.ClusterDefinition, *appsv1alpha1.ClusterDefinition, MockClusterDefFactory]
}

func NewClusterDefFactory(name string) *MockClusterDefFactory {
	f := &MockClusterDefFactory{}
	f.init("", name,
		&appsv1alpha1.ClusterDefinition{
			Spec: appsv1alpha1.ClusterDefinitionSpec{
				ComponentDefs: []appsv1alpha1.ClusterComponentDefinition{},
			},
		}, f)
	f.SetConnectionCredential(defaultConnectionCredential, nil)
	return f
}

func NewClusterDefFactoryWithConnCredential(name string) *MockClusterDefFactory {
	f := NewClusterDefFactory(name)
	f.AddComponent(StatefulMySQLComponent, "conn-cred")
	f.SetConnectionCredential(defaultConnectionCredential, &defaultSvcSpec)
	return f
}

func (factory *MockClusterDefFactory) AddComponent(tplType ComponentTplType, newName string) *MockClusterDefFactory {
	var component *appsv1alpha1.ClusterComponentDefinition
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
	factory.get().Spec.ComponentDefs = append(factory.get().Spec.ComponentDefs, *component)
	comp := factory.getLastCompDef()
	comp.Name = newName
	return factory
}

func (factory *MockClusterDefFactory) AddServicePort(port int32) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return nil
	}
	comp.Service = &corev1.ServiceSpec{
		Ports: []corev1.ServicePort{{
			Protocol: corev1.ProtocolTCP,
			Port:     port,
		}},
	}
	return factory
}

func (factory *MockClusterDefFactory) AddConfigTemplate(name,
	configTplRef, configConstraintRef, namespace, volumeName string, mode *int32) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return nil
	}
	if comp.ConfigSpec == nil {
		comp.ConfigSpec = &appsv1alpha1.ConfigurationSpec{}
	}
	comp.ConfigSpec.ConfigTemplateRefs = append(comp.ConfigSpec.ConfigTemplateRefs,
		appsv1alpha1.ConfigTemplate{
			Name:                name,
			ConfigTplRef:        configTplRef,
			ConfigConstraintRef: configConstraintRef,
			Namespace:           namespace,
			VolumeName:          volumeName,
			DefaultMode:         mode,
		})
	return factory
}

func (factory *MockClusterDefFactory) AddLogConfig(name, filePathPattern string) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return nil
	}
	comp.LogConfigs = append(comp.LogConfigs, appsv1alpha1.LogConfig{
		FilePathPattern: filePathPattern,
		Name:            name,
	})
	return factory
}

func (factory *MockClusterDefFactory) AddContainerEnv(containerName string, envVar corev1.EnvVar) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return nil
	}
	for i, container := range comp.PodSpec.Containers {
		if container.Name == containerName {
			c := comp.PodSpec.Containers[i]
			c.Env = append(c.Env, envVar)
			comp.PodSpec.Containers[i] = c
			break
		}
	}
	return factory
}

func (factory *MockClusterDefFactory) SetConnectionCredential(
	connectionCredential map[string]string, svc *corev1.ServiceSpec) *MockClusterDefFactory {
	factory.get().Spec.ConnectionCredential = connectionCredential
	factory.SetServiceSpec(svc)
	return factory
}

func (factory *MockClusterDefFactory) get1stCompDef() *appsv1alpha1.ClusterComponentDefinition {
	if len(factory.get().Spec.ComponentDefs) == 0 {
		return nil
	}
	return &factory.get().Spec.ComponentDefs[0]
}

func (factory *MockClusterDefFactory) getLastCompDef() *appsv1alpha1.ClusterComponentDefinition {
	l := len(factory.get().Spec.ComponentDefs)
	if l == 0 {
		return nil
	}
	comps := factory.get().Spec.ComponentDefs
	return &comps[l-1]
}

func (factory *MockClusterDefFactory) SetServiceSpec(svc *corev1.ServiceSpec) *MockClusterDefFactory {
	comp := factory.get1stCompDef()
	if comp == nil {
		return factory
	}
	comp.Service = svc
	return factory
}

func (factory *MockClusterDefFactory) AddSystemAccountSpec(sysAccounts *appsv1alpha1.SystemAccountSpec) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return factory
	}
	comp.SystemAccounts = sysAccounts
	return factory
}

func (factory *MockClusterDefFactory) AddInitContainerVolumeMounts(containerName string, volumeMounts []corev1.VolumeMount) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return factory
	}
	comp.PodSpec.InitContainers = appendContainerVolumeMounts(comp.PodSpec.InitContainers, containerName, volumeMounts)
	return factory
}

func (factory *MockClusterDefFactory) AddContainerVolumeMounts(containerName string, volumeMounts []corev1.VolumeMount) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return factory
	}
	comp.PodSpec.Containers = appendContainerVolumeMounts(comp.PodSpec.Containers, containerName, volumeMounts)
	return factory
}

func (factory *MockClusterDefFactory) AddReplicationSpec(replicationSpec *appsv1alpha1.ReplicationSpec) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return factory
	}
	comp.ReplicationSpec = replicationSpec
	return factory
}

func appendContainerVolumeMounts(containers []corev1.Container, targetContainerName string, volumeMounts []corev1.VolumeMount) []corev1.Container {
	for index := range containers {
		c := containers[index]
		// remove the duplicated volumeMounts and overwrite the default mount path
		if c.Name == targetContainerName {
			mergedVolumeMounts := make([]corev1.VolumeMount, 0)
			volumeMountsMap := make(map[string]corev1.VolumeMount)
			for _, v := range c.VolumeMounts {
				volumeMountsMap[v.Name] = v
			}
			for _, v := range volumeMounts {
				volumeMountsMap[v.Name] = v
			}
			for _, v := range volumeMountsMap {
				mergedVolumeMounts = append(mergedVolumeMounts, v)
			}
			c.VolumeMounts = mergedVolumeMounts
			containers[index] = c
			break
		}
	}
	return containers
}
