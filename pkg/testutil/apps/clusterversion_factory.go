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

type MockClusterVersionFactory struct {
	BaseFactory[appsv1alpha1.ClusterVersion, *appsv1alpha1.ClusterVersion, MockClusterVersionFactory]
}

func NewClusterVersionFactory(name, cdRef string) *MockClusterVersionFactory {
	f := &MockClusterVersionFactory{}
	f.Init("", name,
		&appsv1alpha1.ClusterVersion{
			Spec: appsv1alpha1.ClusterVersionSpec{
				ClusterDefinitionRef: cdRef,
				ComponentVersions:    []appsv1alpha1.ClusterComponentVersion{},
			},
		}, f)
	return f
}

func (factory *MockClusterVersionFactory) AddComponentVersion(compDefName string) *MockClusterVersionFactory {
	comp := appsv1alpha1.ClusterComponentVersion{
		ComponentDefRef: compDefName,
	}
	factory.Get().Spec.ComponentVersions = append(factory.Get().Spec.ComponentVersions, comp)
	return factory
}

func (factory *MockClusterVersionFactory) AddInitContainer(container corev1.Container) *MockClusterVersionFactory {
	comps := factory.Get().Spec.ComponentVersions
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		comp.VersionsCtx.InitContainers = append(comp.VersionsCtx.InitContainers, container)
		comps[len(comps)-1] = comp
	}
	factory.Get().Spec.ComponentVersions = comps
	return factory
}

func (factory *MockClusterVersionFactory) AddInitContainerShort(name string, image string) *MockClusterVersionFactory {
	return factory.AddInitContainer(corev1.Container{
		Name:  name,
		Image: image,
	})
}

func (factory *MockClusterVersionFactory) AddContainer(container corev1.Container) *MockClusterVersionFactory {
	comps := factory.Get().Spec.ComponentVersions
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		comp.VersionsCtx.Containers = append(comp.VersionsCtx.Containers, container)
		comps[len(comps)-1] = comp
	}
	factory.Get().Spec.ComponentVersions = comps
	return factory
}

func (factory *MockClusterVersionFactory) AddContainerShort(name string, image string) *MockClusterVersionFactory {
	return factory.AddContainer(corev1.Container{
		Name:  name,
		Image: image,
	})
}

func (factory *MockClusterVersionFactory) AddConfigTemplate(name string,
	configTemplateRef string, configConstraintRef string, volumeName string) *MockClusterVersionFactory {
	comps := factory.Get().Spec.ComponentVersions
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		comp.ConfigSpecs = append(comp.ConfigSpecs,
			appsv1alpha1.ComponentConfigSpec{
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:        name,
					TemplateRef: configTemplateRef,
					VolumeName:  volumeName,
				},
				ConfigConstraintRef: configConstraintRef,
			})
		comps[len(comps)-1] = comp
	}
	factory.Get().Spec.ComponentVersions = comps
	return factory
}
