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

package builder

import (
	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

type ClusterBuilder struct {
	BaseBuilder[appsv1.Cluster, *appsv1.Cluster, ClusterBuilder]
}

func NewClusterBuilder(namespace, name string) *ClusterBuilder {
	builder := &ClusterBuilder{}
	builder.init(namespace, name, &appsv1.Cluster{}, builder)
	return builder
}

func (builder *ClusterBuilder) SetComponentSpecs(specs []appsv1.ClusterComponentSpec) *ClusterBuilder {
	builder.get().Spec.ComponentSpecs = specs
	return builder
}

func (builder *ClusterBuilder) SetResourceVersion(resourceVersion string) *ClusterBuilder {
	builder.get().ResourceVersion = resourceVersion
	return builder
}
