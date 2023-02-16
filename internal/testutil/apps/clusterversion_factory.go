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

type MockClusterVersionFactory struct {
	BaseFactory[appsv1alpha1.ClusterVersion, *appsv1alpha1.ClusterVersion, MockClusterVersionFactory]
}

func NewClusterVersionFactory(name, cdRef string) *MockClusterVersionFactory {
	f := &MockClusterVersionFactory{}
	f.init("", name,
		&appsv1alpha1.ClusterVersion{
			Spec: appsv1alpha1.ClusterVersionSpec{
				ClusterDefinitionRef: cdRef,
				ComponentVersions:    []appsv1alpha1.ClusterVersionComponent{},
			},
		}, f)
	return f
}

func (factory *MockClusterVersionFactory) AddComponent(compType string) *MockClusterVersionFactory {
	comp := appsv1alpha1.ClusterVersionComponent{
		ComponentDefRef: compType,
	}
	factory.get().Spec.ComponentVersions = append(factory.get().Spec.ComponentVersions, comp)
	return factory
}

func (factory *MockClusterVersionFactory) AddInitContainer(container corev1.Container) *MockClusterVersionFactory {
	comps := factory.get().Spec.ComponentVersions
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		if comp.PodSpec == nil {
			comp.PodSpec = &corev1.PodSpec{}
		}
		comp.PodSpec.InitContainers = append(comp.PodSpec.InitContainers, container)
		comps[len(comps)-1] = comp
	}
	factory.get().Spec.ComponentVersions = comps
	return factory
}

func (factory *MockClusterVersionFactory) AddInitContainerShort(name string, image string) *MockClusterVersionFactory {
	return factory.AddInitContainer(corev1.Container{
		Name:  name,
		Image: image,
	})
}

func (factory *MockClusterVersionFactory) AddContainer(container corev1.Container) *MockClusterVersionFactory {
	comps := factory.get().Spec.ComponentVersions
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		if comp.PodSpec == nil {
			comp.PodSpec = &corev1.PodSpec{}
		}
		comp.PodSpec.Containers = append(comp.PodSpec.Containers, container)
		comps[len(comps)-1] = comp
	}
	factory.get().Spec.ComponentVersions = comps
	return factory
}

func (factory *MockClusterVersionFactory) AddContainerShort(name string, image string) *MockClusterVersionFactory {
	return factory.AddContainer(corev1.Container{
		Name:  name,
		Image: image,
	})
}

func (factory *MockClusterVersionFactory) AddConfigTemplate(name string,
	configTplRef string, configConstraintRef string, volumeName string) *MockClusterVersionFactory {
	comps := factory.get().Spec.ComponentVersions
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		comp.ConfigTemplateRefs = append(comp.ConfigTemplateRefs,
			appsv1alpha1.ConfigTemplate{
				Name:                name,
				ConfigTplRef:        configTplRef,
				ConfigConstraintRef: configConstraintRef,
				VolumeName:          volumeName,
			})
		comps[len(comps)-1] = comp
	}
	factory.get().Spec.ComponentVersions = comps
	return factory
}
