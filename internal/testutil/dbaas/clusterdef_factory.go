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

type ComponentTplType string

const (
	StatefulMySQL8 ComponentTplType = "stateful-mysql-8.0"
	ConsensusMySQL ComponentTplType = "consensus-mysql"
	StatelessNginx ComponentTplType = "stateless-nginx"
)

type MockClusterDefFactory struct {
	TestCtx       *testutil.TestContext
	ClusterDef    *dbaasv1alpha1.ClusterDefinition
	clusterDefTpl *dbaasv1alpha1.ClusterDefinition
}

func NewClusterDefFactory(testCtx *testutil.TestContext, name string, cdType string) *MockClusterDefFactory {
	clusterDefTpl := NewCustomizedObj(testCtx, "resources/factory_cd.yaml", &dbaasv1alpha1.ClusterDefinition{})
	return &MockClusterDefFactory{
		TestCtx: testCtx,
		ClusterDef: &dbaasv1alpha1.ClusterDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{},
			},
			Spec: dbaasv1alpha1.ClusterDefinitionSpec{
				Type:       cdType,
				Components: []dbaasv1alpha1.ClusterDefinitionComponent{},
				ConnectionCredential: map[string]string{
					"username": "root",
					"password": "$(RANDOM_PASSWD)",
				},
			},
		},
		clusterDefTpl: clusterDefTpl,
	}
}

func (factory *MockClusterDefFactory) WithRandomName() *MockClusterDefFactory {
	key := GetRandomizedKey(factory.TestCtx, factory.ClusterDef.Name)
	factory.ClusterDef.Name = key.Name
	return factory
}

func (factory *MockClusterDefFactory) AddLabel(key string, value string) *MockClusterDefFactory {
	factory.ClusterDef.Labels[key] = value
	return factory
}

func (factory *MockClusterDefFactory) AddComponent(name ComponentTplType, rename string) *MockClusterDefFactory {
	for _, comp := range factory.clusterDefTpl.Spec.Components {
		if comp.TypeName == string(name) {
			comp.TypeName = rename
			factory.ClusterDef.Spec.Components = append(factory.ClusterDef.Spec.Components, comp)
			break
		}
	}
	return factory
}

func (factory *MockClusterDefFactory) SetDefaultReplicas(replicas int32) *MockClusterDefFactory {
	comps := factory.ClusterDef.Spec.Components
	if len(comps) > 0 {
		comps[len(comps)-1].DefaultReplicas = replicas
	}
	factory.ClusterDef.Spec.Components = comps
	return factory
}

func (factory *MockClusterDefFactory) AddConfigTemplate(name string,
	configTplRef string, configConstraintRef string, volumeName string) *MockClusterDefFactory {
	comps := factory.ClusterDef.Spec.Components
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
			})
		comps[len(comps)-1] = comp
	}
	factory.ClusterDef.Spec.Components = comps
	return factory
}

func (factory *MockClusterDefFactory) AddContainerEnv(containerName string, envVar corev1.EnvVar) *MockClusterDefFactory {
	comps := factory.ClusterDef.Spec.Components
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
	factory.ClusterDef.Spec.Components = comps
	return factory
}

func (factory *MockClusterDefFactory) SetConnectionCredential(
	connectionCredential map[string]string) *MockClusterDefFactory {
	factory.ClusterDef.Spec.ConnectionCredential = connectionCredential
	return factory
}

func (factory *MockClusterDefFactory) Create() *MockClusterDefFactory {
	testCtx := factory.TestCtx
	gomega.Expect(testCtx.CreateObj(testCtx.Ctx, factory.ClusterDef)).Should(gomega.Succeed())
	return factory
}

func (factory *MockClusterDefFactory) GetClusterDef() *dbaasv1alpha1.ClusterDefinition {
	return factory.ClusterDef
}
