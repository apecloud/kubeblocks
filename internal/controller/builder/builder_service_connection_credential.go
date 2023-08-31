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

type ServiceConnectionCredentialBuilder struct {
	BaseBuilder[appsv1alpha1.ServiceConnectionCredential, *appsv1alpha1.ServiceConnectionCredential, ServiceConnectionCredentialBuilder]
}

func NewServiceConnectionCredentialBuilder(namespace, name string) *ServiceConnectionCredentialBuilder {
	builder := &ServiceConnectionCredentialBuilder{}
	builder.init(namespace, name, &appsv1alpha1.ServiceConnectionCredential{}, builder)
	return builder
}

func (builder *ServiceConnectionCredentialBuilder) SetKind(kind string) *ServiceConnectionCredentialBuilder {
	builder.get().Spec.Kind = kind
	return builder
}

func (builder *ServiceConnectionCredentialBuilder) SetVersion(version string) *ServiceConnectionCredentialBuilder {
	builder.get().Spec.Version = version
	return builder
}

func (builder *ServiceConnectionCredentialBuilder) SetEndpoint(endpoint appsv1alpha1.CredentialVar) *ServiceConnectionCredentialBuilder {
	builder.get().Spec.Endpoint = &endpoint
	return builder
}

func (builder *ServiceConnectionCredentialBuilder) SetAuth(auth appsv1alpha1.ConnectionCredentialAuth) *ServiceConnectionCredentialBuilder {
	builder.get().Spec.Auth = &auth
	return builder
}

func (builder *ServiceConnectionCredentialBuilder) SetPort(port appsv1alpha1.CredentialVar) *ServiceConnectionCredentialBuilder {
	builder.get().Spec.Port = &port
	return builder
}

func (builder *ServiceConnectionCredentialBuilder) PutExtra(extra map[string]string) *ServiceConnectionCredentialBuilder {
	builder.get().Spec.Extra = extra
	return builder
}
