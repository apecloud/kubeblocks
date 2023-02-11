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
	"context"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

type ComponentTplType string

const (
	StatefulMySQLComponent  ComponentTplType = "stateful-mysql"
	ConsensusMySQLComponent ComponentTplType = "consensus-mysql"
	StatelessNginxComponent ComponentTplType = "stateless-nginx"
)

type MockClusterDefFactory struct {
	ClusterDef *dbaasv1alpha1.ClusterDefinition
}

func NewClusterDefFactory(name string, cdType string) *MockClusterDefFactory {
	factory := &MockClusterDefFactory{
		ClusterDef: &dbaasv1alpha1.ClusterDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{},
			},
			Spec: dbaasv1alpha1.ClusterDefinitionSpec{
				Type:       cdType,
				Components: []dbaasv1alpha1.ClusterDefinitionComponent{},
			},
		},
	}
	factory.SetConnectionCredential(defaultConnectionCredential)
	return factory
}

func (factory *MockClusterDefFactory) WithRandomName() *MockClusterDefFactory {
	key := GetRandomizedKey("", factory.ClusterDef.Name)
	factory.ClusterDef.Name = key.Name
	return factory
}

func (factory *MockClusterDefFactory) AddLabels(keysAndValues ...string) *MockClusterDefFactory {
	for k, v := range withMap(keysAndValues...) {
		factory.ClusterDef.Labels[k] = v
	}
	return factory
}

func (factory *MockClusterDefFactory) AddComponent(tplType ComponentTplType, rename string) *MockClusterDefFactory {
	var component *dbaasv1alpha1.ClusterDefinitionComponent
	switch tplType {
	case StatefulMySQLComponent:
		component = &statefulMySQLComponent
	case ConsensusMySQLComponent:
		component = &consensusMySQLComponent
	case StatelessNginxComponent:
		component = &statelessNginxComponent
	}
	comps := factory.ClusterDef.Spec.Components
	comps = append(comps, *component)
	comps[len(comps)-1].TypeName = rename
	factory.ClusterDef.Spec.Components = comps
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

func (factory *MockClusterDefFactory) SetService(port int32) *MockClusterDefFactory {
	comps := factory.ClusterDef.Spec.Components
	if len(comps) > 0 {
		comps[len(comps)-1].Service = &corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Protocol: corev1.ProtocolTCP,
				Port:     port,
			}},
		}
	}
	factory.ClusterDef.Spec.Components = comps
	return factory
}

func (factory *MockClusterDefFactory) AddConfigTemplate(name string,
	configTplRef string, configConstraintRef string, volumeName string, mode *int32) *MockClusterDefFactory {
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
				DefaultMode:         mode,
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

func (factory *MockClusterDefFactory) Create(testCtx *testutil.TestContext) *MockClusterDefFactory {
	gomega.Expect(testCtx.CreateObj(testCtx.Ctx, factory.ClusterDef)).Should(gomega.Succeed())
	return factory
}

func (factory *MockClusterDefFactory) CreateCli(ctx context.Context, cli client.Client) *MockClusterDefFactory {
	gomega.Expect(cli.Create(ctx, factory.ClusterDef)).Should(gomega.Succeed())
	return factory
}

func (factory *MockClusterDefFactory) GetClusterDef() *dbaasv1alpha1.ClusterDefinition {
	return factory.ClusterDef
}
