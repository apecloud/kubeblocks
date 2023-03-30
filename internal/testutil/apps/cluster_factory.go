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

type MockClusterFactory struct {
	BaseFactory[appsv1alpha1.Cluster, *appsv1alpha1.Cluster, MockClusterFactory]
}

func NewClusterFactory(namespace, name, cdRef, cvRef string) *MockClusterFactory {
	f := &MockClusterFactory{}
	f.init(namespace, name,
		&appsv1alpha1.Cluster{
			Spec: appsv1alpha1.ClusterSpec{
				ClusterDefRef:     cdRef,
				ClusterVersionRef: cvRef,
				ComponentSpecs:    []appsv1alpha1.ClusterComponentSpec{},
				TerminationPolicy: appsv1alpha1.WipeOut,
			},
		}, f)
	return f
}

func (factory *MockClusterFactory) SetClusterAffinity(affinity *appsv1alpha1.Affinity) *MockClusterFactory {
	factory.get().Spec.Affinity = affinity
	return factory
}

func (factory *MockClusterFactory) AddClusterToleration(toleration corev1.Toleration) *MockClusterFactory {
	tolerations := factory.get().Spec.Tolerations
	if len(tolerations) == 0 {
		tolerations = []corev1.Toleration{}
	}
	tolerations = append(tolerations, toleration)
	factory.get().Spec.Tolerations = tolerations
	return factory
}

func (factory *MockClusterFactory) AddComponent(compName string, compType string) *MockClusterFactory {
	comp := appsv1alpha1.ClusterComponentSpec{
		Name:            compName,
		ComponentDefRef: compType,
	}
	factory.get().Spec.ComponentSpecs = append(factory.get().Spec.ComponentSpecs, comp)
	return factory
}

func (factory *MockClusterFactory) SetReplicas(replicas int32) *MockClusterFactory {
	comps := factory.get().Spec.ComponentSpecs
	if len(comps) > 0 {
		comps[len(comps)-1].Replicas = replicas
	}
	factory.get().Spec.ComponentSpecs = comps
	return factory
}

func (factory *MockClusterFactory) SetResources(resources corev1.ResourceRequirements) *MockClusterFactory {
	comps := factory.get().Spec.ComponentSpecs
	if len(comps) > 0 {
		comps[len(comps)-1].Resources = resources
	}
	factory.get().Spec.ComponentSpecs = comps
	return factory
}

func (factory *MockClusterFactory) SetComponentAffinity(affinity *appsv1alpha1.Affinity) *MockClusterFactory {
	comps := factory.get().Spec.ComponentSpecs
	if len(comps) > 0 {
		comps[len(comps)-1].Affinity = affinity
	}
	factory.get().Spec.ComponentSpecs = comps
	return factory
}

func (factory *MockClusterFactory) SetEnabledLogs(logName ...string) *MockClusterFactory {
	comps := factory.get().Spec.ComponentSpecs
	if len(comps) > 0 {
		comps[len(comps)-1].EnabledLogs = logName
	}
	factory.get().Spec.ComponentSpecs = comps
	return factory
}

func (factory *MockClusterFactory) AddComponentToleration(toleration corev1.Toleration) *MockClusterFactory {
	comps := factory.get().Spec.ComponentSpecs
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
	factory.get().Spec.ComponentSpecs = comps
	return factory
}

func (factory *MockClusterFactory) AddVolumeClaimTemplate(volumeName string,
	pvcSpec *corev1.PersistentVolumeClaimSpec) *MockClusterFactory {
	comps := factory.get().Spec.ComponentSpecs
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		comp.VolumeClaimTemplates = append(comp.VolumeClaimTemplates,
			appsv1alpha1.ClusterComponentVolumeClaimTemplate{
				Name: volumeName,
				Spec: pvcSpec,
			})
		comps[len(comps)-1] = comp
	}
	factory.get().Spec.ComponentSpecs = comps
	return factory
}

func (factory *MockClusterFactory) SetMonitor(monitor bool) *MockClusterFactory {
	comps := factory.get().Spec.ComponentSpecs
	if len(comps) > 0 {
		comps[len(comps)-1].Monitor = monitor
	}
	factory.get().Spec.ComponentSpecs = comps
	return factory
}

func (factory *MockClusterFactory) SetPrimaryIndex(primaryIndex int32) *MockClusterFactory {
	comps := factory.get().Spec.ComponentSpecs
	if len(comps) > 0 {
		comps[len(comps)-1].PrimaryIndex = &primaryIndex
	}
	factory.get().Spec.ComponentSpecs = comps
	return factory
}

func (factory *MockClusterFactory) SetSwitchPolicy(switchPolicy *appsv1alpha1.ClusterSwitchPolicy) *MockClusterFactory {
	comps := factory.get().Spec.ComponentSpecs
	if len(comps) > 0 {
		comps[len(comps)-1].SwitchPolicy = switchPolicy
	}
	factory.get().Spec.ComponentSpecs = comps
	return factory
}

func (factory *MockClusterFactory) SetTLS(tls bool) *MockClusterFactory {
	comps := factory.get().Spec.ComponentSpecs
	if len(comps) > 0 {
		comps[len(comps)-1].TLS = tls
	}
	factory.get().Spec.ComponentSpecs = comps
	return factory
}

func (factory *MockClusterFactory) SetIssuer(issuer *appsv1alpha1.Issuer) *MockClusterFactory {
	comps := factory.get().Spec.ComponentSpecs
	if len(comps) > 0 {
		comps[len(comps)-1].Issuer = issuer
	}
	factory.get().Spec.ComponentSpecs = comps
	return factory
}

func (factory *MockClusterFactory) AddService(service appsv1alpha1.ClusterComponentService) *MockClusterFactory {
	comps := factory.get().Spec.ComponentSpecs
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		comp.Services = append(comp.Services, service)
		comps[len(comps)-1] = comp
	}
	factory.get().Spec.ComponentSpecs = comps
	return factory
}
