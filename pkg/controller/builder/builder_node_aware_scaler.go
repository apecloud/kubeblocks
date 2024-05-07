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
	experimental "github.com/apecloud/kubeblocks/apis/experimental/v1alpha1"
)

type NodeAwareScalerBuilder struct {
	BaseBuilder[experimental.NodeAwareScaler, *experimental.NodeAwareScaler, NodeAwareScalerBuilder]
}

func NewNodeAwareScalerBuilder(namespace, name string) *NodeAwareScalerBuilder {
	builder := &NodeAwareScalerBuilder{}
	builder.init(namespace, name, &experimental.NodeAwareScaler{}, builder)
	return builder
}

func (builder *NodeAwareScalerBuilder) SetTargetClusterName(clusterName string) *NodeAwareScalerBuilder {
	builder.get().Spec.TargetClusterName = clusterName
	return builder
}

func (builder *NodeAwareScalerBuilder) SetTargetComponentNames(componentNames []string) *NodeAwareScalerBuilder {
	builder.get().Spec.TargetComponentNames = componentNames
	return builder
}
