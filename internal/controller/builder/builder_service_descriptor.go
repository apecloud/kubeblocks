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

package builder

import (
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type ServiceDescriptorBuilder struct {
	BaseBuilder[appsv1alpha1.ServiceDescriptor, *appsv1alpha1.ServiceDescriptor, ServiceDescriptorBuilder]
}

func NewServiceDescriptorBuilder(namespace, name string) *ServiceDescriptorBuilder {
	builder := &ServiceDescriptorBuilder{}
	builder.init(namespace, name, &appsv1alpha1.ServiceDescriptor{}, builder)
	return builder
}

func (builder *ServiceDescriptorBuilder) SetKind(kind string) *ServiceDescriptorBuilder {
	builder.get().Spec.Kind = kind
	return builder
}

func (builder *ServiceDescriptorBuilder) SetVersion(version string) *ServiceDescriptorBuilder {
	builder.get().Spec.Version = version
	return builder
}

func (builder *ServiceDescriptorBuilder) SetEndpoint(endpoint appsv1alpha1.CredentialVar) *ServiceDescriptorBuilder {
	builder.get().Spec.Endpoint = &endpoint
	return builder
}

func (builder *ServiceDescriptorBuilder) SetAuth(auth appsv1alpha1.ConnectionCredentialAuth) *ServiceDescriptorBuilder {
	builder.get().Spec.Auth = &auth
	return builder
}

func (builder *ServiceDescriptorBuilder) SetPort(port appsv1alpha1.CredentialVar) *ServiceDescriptorBuilder {
	builder.get().Spec.Port = &port
	return builder
}

func (builder *ServiceDescriptorBuilder) SetExtra(extra map[string]string) *ServiceDescriptorBuilder {
	builder.get().Spec.Extra = extra
	return builder
}
