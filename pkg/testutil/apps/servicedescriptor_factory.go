/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

type MockServiceDescriptorFactory struct {
	BaseFactory[appsv1.ServiceDescriptor, *appsv1.ServiceDescriptor, MockServiceDescriptorFactory]
}

func NewServiceDescriptorFactory(namespace, name string) *MockServiceDescriptorFactory {
	f := &MockServiceDescriptorFactory{}
	f.Init(namespace, name,
		&appsv1.ServiceDescriptor{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					constant.AppManagedByLabelKey: constant.AppName,
				},
			},
			Spec: appsv1.ServiceDescriptorSpec{},
		}, f)
	return f
}

func (factory *MockServiceDescriptorFactory) SetServiceKind(serviceKind string) *MockServiceDescriptorFactory {
	factory.Get().Spec.ServiceKind = serviceKind
	return factory
}

func (factory *MockServiceDescriptorFactory) SetServiceVersion(serviceVersion string) *MockServiceDescriptorFactory {
	factory.Get().Spec.ServiceVersion = serviceVersion
	return factory
}

func (factory *MockServiceDescriptorFactory) SetEndpoint(endpoint appsv1.CredentialVar) *MockServiceDescriptorFactory {
	factory.Get().Spec.Endpoint = &endpoint
	return factory
}

func (factory *MockServiceDescriptorFactory) SetHost(host appsv1.CredentialVar) *MockServiceDescriptorFactory {
	factory.Get().Spec.Host = &host
	return factory
}

func (factory *MockServiceDescriptorFactory) SetPort(port appsv1.CredentialVar) *MockServiceDescriptorFactory {
	factory.Get().Spec.Port = &port
	return factory
}

func (factory *MockServiceDescriptorFactory) SetPodFQDNs(podFQDNs appsv1.CredentialVar) *MockServiceDescriptorFactory {
	factory.Get().Spec.PodFQDNs = &podFQDNs
	return factory
}

func (factory *MockServiceDescriptorFactory) SetAuth(auth appsv1.ConnectionCredentialAuth) *MockServiceDescriptorFactory {
	factory.Get().Spec.Auth = &auth
	return factory
}
