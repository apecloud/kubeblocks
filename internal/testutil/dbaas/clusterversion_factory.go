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
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

type MockClusterVersionFactory struct {
	TestCtx        *testutil.TestContext
	ClusterVersion *dbaasv1alpha1.ClusterVersion
}

func NewClusterVersionFactory(testCtx *testutil.TestContext, name string, cdRef string) *MockClusterVersionFactory {
	return &MockClusterVersionFactory{
		TestCtx: testCtx,
		ClusterVersion: &dbaasv1alpha1.ClusterVersion{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: dbaasv1alpha1.ClusterVersionSpec{
				ClusterDefinitionRef: cdRef,
				Components:           []dbaasv1alpha1.ClusterVersionComponent{},
			},
		},
	}
}

func (factory *MockClusterVersionFactory) WithRandomName() *MockClusterVersionFactory {
	key := GetRandomizedKey(factory.TestCtx, factory.ClusterVersion.Name)
	factory.ClusterVersion.Name = key.Name
	return factory
}

func (factory *MockClusterVersionFactory) AddComponent(compType string) *MockClusterVersionFactory {
	comp := dbaasv1alpha1.ClusterVersionComponent{
		Type: compType,
	}
	factory.ClusterVersion.Spec.Components = append(factory.ClusterVersion.Spec.Components, comp)
	return factory
}

func (factory *MockClusterVersionFactory) AddContainerShort(name string, image string) *MockClusterVersionFactory {
	comps := factory.ClusterVersion.Spec.Components
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		if comp.PodSpec == nil {
			comp.PodSpec = &corev1.PodSpec{}
		}
		comp.PodSpec.Containers = append(comp.PodSpec.Containers,
			corev1.Container{
				Name:  name,
				Image: image,
			})
		comps[len(comps)-1] = comp
	}
	factory.ClusterVersion.Spec.Components = comps
	return factory
}

func (factory *MockClusterVersionFactory) AddConfigTemplate(name string,
	configTplRef string, configConstraintRef string, volumeName string) *MockClusterVersionFactory {
	comps := factory.ClusterVersion.Spec.Components
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		comp.ConfigTemplateRefs = append(comp.ConfigTemplateRefs,
			dbaasv1alpha1.ConfigTemplate{
				Name:                name,
				ConfigTplRef:        configTplRef,
				ConfigConstraintRef: configConstraintRef,
				VolumeName:          volumeName,
			})
		comps[len(comps)-1] = comp
	}
	factory.ClusterVersion.Spec.Components = comps
	return factory
}

func (factory *MockClusterVersionFactory) Create() *MockClusterVersionFactory {
	testCtx := factory.TestCtx
	gomega.Expect(testCtx.CreateObj(testCtx.Ctx, factory.ClusterVersion)).Should(gomega.Succeed())
	return factory
}

func (factory *MockClusterVersionFactory) GetClusterVersion() *dbaasv1alpha1.ClusterVersion {
	return factory.ClusterVersion
}
