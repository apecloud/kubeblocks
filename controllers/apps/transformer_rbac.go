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
	appsv1 "k8s.io/api/apps/v1"

	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
)

// RBACTransformer puts the rbac at the beginning of the DAG
type RBACTransformer struct{}

var _ graph.Transformer = &RBACTransformer{}

func (c *RBACTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster
	if cluster.IsDeleting() {
		return nil
	}

	root, err := ictrltypes.FindRootVertex(dag)
	if err != nil {
		return err
	}

	serviceAccount, err := builder.BuildServiceAccount(cluster)
	if err != nil {
		return err
	}
	saVertex := ictrltypes.LifecycleObjectCreate(dag, serviceAccount, root)

	role, err := builder.BuildRole(cluster)
	if err != nil {
		return err
	}
	roleVertex := ictrltypes.LifecycleObjectCreate(dag, role, root)

	roleBinding, err := builder.BuildRoleBinding(cluster)
	if err != nil {
		return err
	}
	rbVertex := ictrltypes.LifecycleObjectCreate(dag, roleBinding, root)
	dag.Connect(rbVertex, roleVertex)
	dag.Connect(rbVertex, saVertex)

	statefulSetVertices := ictrltypes.FindAll[*appsv1.StatefulSet](dag)
	for _, statefulSetVertex := range statefulSetVertices {
		// rbac must be created before statefulset
		dag.Connect(statefulSetVertex, rbVertex)
	}

	deploymentVertices := ictrltypes.FindAll[*appsv1.Deployment](dag)
	for _, deploymentVertex := range deploymentVertices {
		// rbac must be created before deployment
		dag.Connect(deploymentVertex, rbVertex)
	}
	return nil
}
