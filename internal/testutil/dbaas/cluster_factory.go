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

type ComponentTypeName string

type MockClusterFactory struct {
	Cluster *dbaasv1alpha1.Cluster
}

func NewClusterFactory(namespace string, name string, cdRef string, cvRef string) *MockClusterFactory {
	return &MockClusterFactory{
		Cluster: &dbaasv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    map[string]string{},
			},
			Spec: dbaasv1alpha1.ClusterSpec{
				ClusterDefRef:     cdRef,
				ClusterVersionRef: cvRef,
				Components:        []dbaasv1alpha1.ClusterComponent{},
				TerminationPolicy: dbaasv1alpha1.WipeOut,
			},
		},
	}
}

func (factory *MockClusterFactory) WithRandomName() *MockClusterFactory {
	key := GetRandomizedKey(factory.Cluster.Namespace, factory.Cluster.Name)
	factory.Cluster.Name = key.Name
	return factory
}

func (factory *MockClusterFactory) AddLabels(keysAndValues ...string) *MockClusterFactory {
	for k, v := range withMap(keysAndValues...) {
		factory.Cluster.Labels[k] = v
	}
	return factory
}

func (factory *MockClusterFactory) SetClusterAffinity(affinity *dbaasv1alpha1.Affinity) *MockClusterFactory {
	factory.Cluster.Spec.Affinity = affinity
	return factory
}

func (factory *MockClusterFactory) AddClusterToleration(toleration corev1.Toleration) *MockClusterFactory {
	tolerations := factory.Cluster.Spec.Tolerations
	if len(tolerations) == 0 {
		tolerations = []corev1.Toleration{}
	}
	tolerations = append(tolerations, toleration)
	factory.Cluster.Spec.Tolerations = tolerations
	return factory
}

func (factory *MockClusterFactory) AddComponent(compName string, compType string) *MockClusterFactory {
	comp := dbaasv1alpha1.ClusterComponent{
		Name: compName,
		Type: compType,
	}
	factory.Cluster.Spec.Components = append(factory.Cluster.Spec.Components, comp)
	return factory
}

func (factory *MockClusterFactory) SetReplicas(replicas int32) *MockClusterFactory {
	comps := factory.Cluster.Spec.Components
	if len(comps) > 0 {
		comps[len(comps)-1].Replicas = &replicas
	}
	factory.Cluster.Spec.Components = comps
	return factory
}

func (factory *MockClusterFactory) SetResources(resources corev1.ResourceRequirements) *MockClusterFactory {
	comps := factory.Cluster.Spec.Components
	if len(comps) > 0 {
		comps[len(comps)-1].Resources = resources
	}
	factory.Cluster.Spec.Components = comps
	return factory
}

func (factory *MockClusterFactory) SetComponentAffinity(affinity *dbaasv1alpha1.Affinity) *MockClusterFactory {
	comps := factory.Cluster.Spec.Components
	if len(comps) > 0 {
		comps[len(comps)-1].Affinity = affinity
	}
	factory.Cluster.Spec.Components = comps
	return factory
}

func (factory *MockClusterFactory) AddComponentToleration(toleration corev1.Toleration) *MockClusterFactory {
	comps := factory.Cluster.Spec.Components
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		tolerations := comp.Tolerations
		if len(tolerations) == 0 {
			tolerations = []corev1.Toleration{}
		}
		tolerations = append(tolerations, toleration)
		comp.Tolerations = tolerations
		comps[len(comps)-1] = comp
	}
	factory.Cluster.Spec.Components = comps
	return factory
}

func (factory *MockClusterFactory) AddVolumeClaimTemplate(volumeName string,
	pvcSpec *corev1.PersistentVolumeClaimSpec) *MockClusterFactory {
	comps := factory.Cluster.Spec.Components
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		comp.VolumeClaimTemplates = append(comp.VolumeClaimTemplates,
			dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{
				Name: volumeName,
				Spec: pvcSpec,
			})
		comps[len(comps)-1] = comp
	}
	factory.Cluster.Spec.Components = comps
	return factory
}

func (factory *MockClusterFactory) SetMonitor(monitor bool) *MockClusterFactory {
	comps := factory.Cluster.Spec.Components
	if len(comps) > 0 {
		comps[len(comps)-1].Monitor = monitor
	}
	factory.Cluster.Spec.Components = comps
	return factory
}

func (factory *MockClusterFactory) Create(testCtx *testutil.TestContext) *MockClusterFactory {
	gomega.Expect(testCtx.CreateObj(testCtx.Ctx, factory.Cluster)).Should(gomega.Succeed())
	return factory
}

func (factory *MockClusterFactory) CreateCli(ctx context.Context, cli client.Client) *MockClusterFactory {
	gomega.Expect(cli.Create(ctx, factory.Cluster)).Should(gomega.Succeed())
	return factory
}

func (factory *MockClusterFactory) GetCluster() *dbaasv1alpha1.Cluster {
	return factory.Cluster
}
