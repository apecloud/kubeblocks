/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lifecycle

import (
	"github.com/apecloud/kubeblocks/internal/constant"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
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
