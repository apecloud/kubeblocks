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

func getDefaultConnectionCredential() map[string]string {
	return map[string]string{
		"username":          "root",
		"SVC_FQDN":          "$(SVC_FQDN)",
		"HEADLESS_SVC_FQDN": "$(HEADLESS_SVC_FQDN)",
		"RANDOM_PASSWD":     "$(RANDOM_PASSWD)",
		"tcpEndpoint":       "tcp:$(SVC_FQDN):$(SVC_PORT_mysql)",
		"paxosEndpoint":     "paxos:$(SVC_FQDN):$(SVC_PORT_paxos)",
		"UUID":              "$(UUID)",
		"UUID_B64":          "$(UUID_B64)",
		"UUID_STR_B64":      "$(UUID_STR_B64)",
		"UUID_HEX":          "$(UUID_HEX)",
	}
}

func NewClusterDefFactory(name string) *MockClusterDefFactory {
	f := &MockClusterDefFactory{}
	f.Init("", name,
		&appsv1alpha1.ClusterDefinition{
			Spec: appsv1alpha1.ClusterDefinitionSpec{
				ComponentDefs: []appsv1alpha1.ClusterComponentDefinition{},
			},
		}, f)
	f.SetConnectionCredential(getDefaultConnectionCredential(), nil)
	return f
}

func NewClusterDefFactoryWithConnCredential(name, compDefName string) *MockClusterDefFactory {
	f := NewClusterDefFactory(name)
	f.AddComponentDef(StatefulMySQLComponent, compDefName)
	f.SetConnectionCredential(getDefaultConnectionCredential(), &defaultSvcSpec)
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
	factory.Get().Spec.ComponentDefs = append(factory.Get().Spec.ComponentDefs, *component)
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
	configTemplateRef, configConstraintRef, namespace, volumeName string, asEnvFrom ...string) *MockClusterDefFactory {
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
			AsEnvFrom:           asEnvFrom,
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
	factory.Get().Spec.ConnectionCredential = connectionCredential
	factory.SetServiceSpec(svc)
	return factory
}

func (factory *MockClusterDefFactory) get1stCompDef() *appsv1alpha1.ClusterComponentDefinition {
	if len(factory.Get().Spec.ComponentDefs) == 0 {
		return nil
	}
	return &factory.Get().Spec.ComponentDefs[0]
}

func (factory *MockClusterDefFactory) getLastCompDef() *appsv1alpha1.ClusterComponentDefinition {
	l := len(factory.Get().Spec.ComponentDefs)
	if l == 0 {
		return nil
	}
	comps := factory.Get().Spec.ComponentDefs
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

func (factory *MockClusterDefFactory) AddSwitchoverSpec(switchoverSpec *appsv1alpha1.SwitchoverSpec) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return factory
	}
	comp.SwitchoverSpec = switchoverSpec
	return factory
}

func (factory *MockClusterDefFactory) AddServiceRefDeclarations(serviceRefDeclarations []appsv1alpha1.ServiceRefDeclaration) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return factory
	}
	comp.ServiceRefDeclarations = serviceRefDeclarations
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

func (factory *MockClusterDefFactory) AddComponentRef(ref *appsv1alpha1.ComponentDefRef) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return factory
	}
	if len(comp.ComponentDefRef) == 0 {
		comp.ComponentDefRef = make([]appsv1alpha1.ComponentDefRef, 0)
	}
	comp.ComponentDefRef = append(comp.ComponentDefRef, *ref)
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

// There are default volumeMounts for containers in clusterdefinition in pusrpose of a simple & fast creation,
// but when mounts specified volumes in certain mountPaths, they may conflict with the default volumeMounts,
// so here provides a way to overwrite the default volumeMounts.
func appendContainerVolumeMounts(containers []corev1.Container, targetContainerName string, volumeMounts []corev1.VolumeMount) []corev1.Container {
	for index := range containers {
		c := &containers[index]
		// remove the duplicated volumeMounts and overwrite the default mount path
		if c.Name == targetContainerName {
			mergedAddVolumeMounts(c, volumeMounts)
			break
		}
	}
	return containers
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
