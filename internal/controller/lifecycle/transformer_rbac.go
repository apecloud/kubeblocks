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

package lifecycle

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// RbacTransformer puts the rbac at the beginning of the DAG
type RbacTransformer struct{}

func (c *RbacTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	origCluster := transCtx.OrigCluster
	cluster := transCtx.Cluster
	if isClusterDeleting(*origCluster) {
		return nil
	}

	params := builder.BuilderParams{
		Cluster: cluster,
	}
	serviceAccount, role, roleBinding, _ := builder.BuildRbac(params)
	saVertex := &lifecycleVertex{obj: serviceAccount}
	dag.AddVertex(saVertex)
	roleVertex := &lifecycleVertex{obj: role}
	dag.AddVertex(roleVertex)
	rbVertex := &lifecycleVertex{obj: roleBinding}
	dag.AddVertex(rbVertex)
	dag.Connect(rbVertex, roleVertex)
	dag.Connect(rbVertex, saVertex)
	secretVertices := findAll[*corev1.Secret](dag)
	for _, secretVertex := range secretVertices {
		// connect all secret vertices to rbac vertices
		dag.Connect(secretVertex, rbVertex)
	}
	return nil
}

var _ graph.Transformer = &RbacTransformer{}
