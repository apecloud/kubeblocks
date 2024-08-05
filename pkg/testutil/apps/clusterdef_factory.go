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
	f.Init("", name,
		&appsv1alpha1.ClusterDefinition{
			Spec: appsv1alpha1.ClusterDefinitionSpec{
				ComponentDefs: []appsv1alpha1.ClusterComponentDefinition{},
			},
		}, f)
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

func (factory *MockClusterDefFactory) AddHorizontalScalePolicy(policy appsv1alpha1.HorizontalScalePolicy) *MockClusterDefFactory {
	comp := factory.getLastCompDef()
	if comp == nil {
		return nil
	}
	comp.HorizontalScalePolicy = &policy
	return factory
}

func (factory *MockClusterDefFactory) getLastCompDef() *appsv1alpha1.ClusterComponentDefinition {
	l := len(factory.Get().Spec.ComponentDefs)
	if l == 0 {
		return nil
	}
	comps := factory.Get().Spec.ComponentDefs
	return &comps[l-1]
}

func (factory *MockClusterDefFactory) AddClusterTopology(topology appsv1alpha1.ClusterTopology) *MockClusterDefFactory {
	factory.Get().Spec.Topologies = append(factory.Get().Spec.Topologies, topology)
	return factory
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
