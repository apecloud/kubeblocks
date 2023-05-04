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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
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

func (factory *MockClusterFactory) AddComponent(compName string, compDefName string) *MockClusterFactory {
	comp := appsv1alpha1.ClusterComponentSpec{
		Name:            compName,
		ComponentDefRef: compDefName,
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

func (factory *MockClusterFactory) SetServiceAccountName(serviceAccountName string) *MockClusterFactory {
	comps := factory.get().Spec.ComponentSpecs
	if len(comps) > 0 {
		comps[len(comps)-1].ServiceAccountName = serviceAccountName
	}
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

func (factory *MockClusterFactory) SetClassDefRef(classDefRef *appsv1alpha1.ClassDefRef) *MockClusterFactory {
	comps := factory.get().Spec.ComponentSpecs
	if len(comps) > 0 {
		comps[len(comps)-1].ClassDefRef = classDefRef
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
	pvcSpec appsv1alpha1.PersistentVolumeClaimSpec) *MockClusterFactory {
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

func (factory *MockClusterFactory) AddService(serviceName string, serviceType corev1.ServiceType) *MockClusterFactory {
	comps := factory.get().Spec.ComponentSpecs
	if len(comps) > 0 {
		comp := comps[len(comps)-1]
		comp.Services = append(comp.Services,
			appsv1alpha1.ClusterComponentService{
				Name:        serviceName,
				ServiceType: serviceType,
			})
		comps[len(comps)-1] = comp
	}
	factory.get().Spec.ComponentSpecs = comps
	return factory
}

func (factory *MockClusterFactory) AddRestorePointInTime(restoreTime metav1.Time, sourceCluster string) *MockClusterFactory {
	annotations := factory.get().Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[constant.RestoreFromTimeAnnotationKey] = restoreTime.Format(time.RFC3339)
	annotations[constant.RestoreFromSrcClusterAnnotationKey] = sourceCluster

	factory.get().Annotations = annotations
	return factory
}
