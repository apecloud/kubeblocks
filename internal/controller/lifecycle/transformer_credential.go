/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// credentialTransformer puts the credential Secret at the beginning of the DAG
type credentialTransformer struct{}

func (c *credentialTransformer) Transform(dag *graph.DAG) error {
	var secretVertices, noneRootVertices []graph.Vertex
	secretVertices = findAll[*corev1.Secret](dag)
	noneRootVertices = findAllNot[*appsv1alpha1.Cluster](dag)
	for _, secretVertex := range secretVertices {
		secret, _ := secretVertex.(*lifecycleVertex)
		secret.immutable = true
		for _, vertex := range noneRootVertices {
			v, _ := vertex.(*lifecycleVertex)
			// connect all none secret vertices to all secret vertices
			if _, ok := v.obj.(*corev1.Secret); !ok {
				dag.Connect(vertex, secretVertex)
			}
		}
	}
	return nil
}
