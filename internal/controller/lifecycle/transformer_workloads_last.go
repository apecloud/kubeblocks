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
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/apecloud/kubeblocks/internal/constant"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// WorkloadsLastTransformer have workload objects placed last
type WorkloadsLastTransformer struct{}

func (c *WorkloadsLastTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	workloadKinds := sets.New(constant.StatefulSetKind, constant.DeploymentKind)
	var workloadsVertices, noneRootVertices []graph.Vertex

	workloadsVertices = findAll[*appv1.StatefulSet](dag)
	workloadsVertices = append(workloadsVertices, findAll[*appv1.Deployment](dag))
	noneRootVertices = findAllNot[*appsv1alpha1.Cluster](dag)

	for _, workloadV := range workloadsVertices {
		workload, _ := workloadV.(*lifecycleVertex)
		for _, vertex := range noneRootVertices {
			v, _ := vertex.(*lifecycleVertex)
			// connect all workloads vertices to all none workloads vertices
			if !workloadKinds.Has(v.obj.GetObjectKind().GroupVersionKind().Kind) {
				dag.Connect(workload, vertex)
			}
		}
	}
	return nil
}

var _ graph.Transformer = &WorkloadsLastTransformer{}
