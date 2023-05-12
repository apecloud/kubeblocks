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
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type ComponentDefTplType string

const (
	StatefulMySQLComponent    ComponentDefTplType = "stateful-mysql"
	ConsensusMySQLComponent   ComponentDefTplType = "consensus-mysql"
	ReplicationRedisComponent ComponentDefTplType = "replication-redis"
	StatelessNginxComponent   ComponentDefTplType = "stateless-nginx"
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
	f.AddComponentDef(StatefulMySQLComponent, "conn-cred")
	f.SetConnectionCredential(defaultConnectionCredential, &defaultSvcSpec)
	return f
}

func (factory *MockClusterDefFactory) AddComponentDef(tplType ComponentDefTplType, compDefName string) *MockClusterDefFactory {
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
	comp.Name = compDefName
	return factory
}

func (factory *MockClusterDefFactory) AddServicePort(port int32) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return nil
	}
	comp.Service = &appsv1alpha1.ServiceSpec{
		Ports: []appsv1alpha1.ServicePort{{
			Protocol: corev1.ProtocolTCP,
			Port:     port,
		}},
	}
	return factory
}

func (factory *MockClusterDefFactory) AddScriptTemplate(name,
	configTemplateRef, namespace, volumeName string, mode *int32) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return nil
	}
	comp.ScriptSpecs = append(comp.ScriptSpecs,
		appsv1alpha1.ComponentTemplateSpec{
			Name:        name,
			TemplateRef: configTemplateRef,
			Namespace:   namespace,
			VolumeName:  volumeName,
			DefaultMode: mode,
		})
	return factory
}

func (factory *MockClusterDefFactory) AddConfigTemplate(name,
	configTemplateRef, configConstraintRef, namespace, volumeName string) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return nil
	}
	comp.ConfigSpecs = append(comp.ConfigSpecs,
		appsv1alpha1.ComponentConfigSpec{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:        name,
				TemplateRef: configTemplateRef,
				Namespace:   namespace,
				VolumeName:  volumeName,
			},
			ConfigConstraintRef: configConstraintRef,
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

func (factory *MockClusterDefFactory) AddHorizontalScalePolicy(policy appsv1alpha1.HorizontalScalePolicy) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return nil
	}
	comp.HorizontalScalePolicy = &policy
	return factory
}

func (factory *MockClusterDefFactory) SetConnectionCredential(
	connectionCredential map[string]string, svc *appsv1alpha1.ServiceSpec) *MockClusterDefFactory {
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

func (factory *MockClusterDefFactory) SetServiceSpec(svc *appsv1alpha1.ServiceSpec) *MockClusterDefFactory {
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

func (factory *MockClusterDefFactory) AddReplicationSpec(replicationSpec *appsv1alpha1.ReplicationSetSpec) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return factory
	}
	comp.ReplicationSpec = replicationSpec
	return factory
}

// There are default volumeMounts for containers in clusterdefinition in pusrpose of a simple & fast creation,
// but when mounts specified volumes in certain mountPaths, they may conflict with the default volumeMounts,
// so here provides a way to overwrite the default volumeMounts.
func appendContainerVolumeMounts(containers []corev1.Container, targetContainerName string, volumeMounts []corev1.VolumeMount) []corev1.Container {
	for index := range containers {
		c := &containers[index]
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
			break
		}
	}
	return containers
}

func (factory *MockClusterDefFactory) AddConstraints(constraint *appsv1alpha1.Constraints) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return factory
	}
	comp.Constraints = constraint
	return factory
}

func (factory *MockClusterDefFactory) AddComponentRef(ref *appsv1alpha1.ComponentRef) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return factory
	}
	if len(comp.ComponentRef) == 0 {
		comp.ComponentRef = make([]*appsv1alpha1.ComponentRef, 0)
	}
	comp.ComponentRef = append(comp.ComponentRef, ref)
	return factory
}

func (factory *MockClusterDefFactory) AddNamedServicePort(name string, port int32) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return nil
	}
	if comp.Service != nil {
		comp.Service.Ports = append(comp.Service.Ports, appsv1alpha1.ServicePort{
			Name:     name,
			Protocol: corev1.ProtocolTCP,
			Port:     port,
		})
		return factory
	}
	comp.Service = &appsv1alpha1.ServiceSpec{
		Ports: []appsv1alpha1.ServicePort{{
			Name:     name,
			Protocol: corev1.ProtocolTCP,
			Port:     port,
		}},
	}
	return factory

}
