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
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

type MockComponentFactory struct {
	BaseFactory[appsv1.Component, *appsv1.Component, MockComponentFactory]
}

func NewComponentFactory(namespace, name, compDef string) *MockComponentFactory {
	f := &MockComponentFactory{}
	f.Init(namespace, name,
		&appsv1.Component{
			Spec: appsv1.ComponentSpec{
				TerminationPolicy: appsv1.WipeOut,
				CompDef:           compDef,
			},
		}, f)
	return f
}

func (factory *MockComponentFactory) SetServiceVersion(serviceVersion string) *MockComponentFactory {
	factory.Get().Spec.ServiceVersion = serviceVersion
	return factory
}

func (factory *MockComponentFactory) SetReplicas(replicas int32) *MockComponentFactory {
	factory.Get().Spec.Replicas = replicas
	return factory
}

func (factory *MockComponentFactory) SetServiceAccountName(serviceAccountName string) *MockComponentFactory {
	factory.Get().Spec.ServiceAccountName = serviceAccountName
	return factory
}

func (factory *MockComponentFactory) SetResources(resources corev1.ResourceRequirements) *MockComponentFactory {
	factory.Get().Spec.Resources = resources
	return factory
}

func (factory *MockComponentFactory) SetTLSConfig(enable bool, issuer *appsv1.Issuer) *MockComponentFactory {
	if enable {
		factory.Get().Spec.TLSConfig = &appsv1.TLSConfig{
			Enable: enable,
			Issuer: issuer,
		}
	}
	return factory
}

func (factory *MockComponentFactory) AddVolumeClaimTemplate(volumeName string,
	pvcSpec appsv1.PersistentVolumeClaimSpec) *MockComponentFactory {
	factory.Get().Spec.VolumeClaimTemplates = append(factory.Get().Spec.VolumeClaimTemplates, appsv1.ClusterComponentVolumeClaimTemplate{
		Name: volumeName,
		Spec: pvcSpec,
	})
	return factory
}

func (factory *MockComponentFactory) AddSystemAccount(name string, disabled bool, passwordConfig *appsv1.PasswordConfig, secretRef *appsv1.ProvisionSecretRef) *MockComponentFactory {
	if factory.Get().Spec.SystemAccounts == nil {
		factory.Get().Spec.SystemAccounts = make([]appsv1.ComponentSystemAccount, 0)
	}
	factory.Get().Spec.SystemAccounts = append(factory.Get().Spec.SystemAccounts,
		appsv1.ComponentSystemAccount{
			Name:           name,
			Disabled:       ptr.To(disabled),
			PasswordConfig: passwordConfig,
			SecretRef:      secretRef,
		})
	return factory
}

func (factory *MockComponentFactory) SetStop(stop *bool) *MockComponentFactory {
	factory.Get().Spec.Stop = stop
	return factory
}

func (factory *MockComponentFactory) AddInstances(instance appsv1.InstanceTemplate) *MockComponentFactory {
	if factory.Get().Spec.Instances == nil {
		factory.Get().Spec.Instances = make([]appsv1.InstanceTemplate, 0)
	}
	factory.Get().Spec.Instances = append(factory.Get().Spec.Instances, instance)
	return factory
}
