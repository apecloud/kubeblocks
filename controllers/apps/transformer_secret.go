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
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
)

// SecretTransformer puts all the secrets at the beginning of the DAG
type SecretTransformer struct{}

var _ graph.Transformer = &SecretTransformer{}

func (c *SecretTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	var secretVertices, noneRootVertices []graph.Vertex
	secretVertices = ictrltypes.FindAll[*corev1.Secret](dag)
	noneRootVertices = ictrltypes.FindAllNot[*appsv1alpha1.Cluster](dag)
	for _, secretVertex := range secretVertices {
		secret, _ := secretVertex.(*ictrltypes.LifecycleVertex)
		secret.Immutable = true
		for _, vertex := range noneRootVertices {
			v, _ := vertex.(*ictrltypes.LifecycleVertex)
			// connect all none secret vertices to all secret vertices
			if _, ok := v.Obj.(*corev1.Secret); !ok {
				if *v.Action != *ictrltypes.ActionDeletePtr() {
					dag.Connect(vertex, secretVertex)
				}
			}
		}
	}
	return nil
}
