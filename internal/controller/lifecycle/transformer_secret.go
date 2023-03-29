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
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// secretTransformer puts all the secrets at the beginning of the DAG
type secretTransformer struct{}

func (c *secretTransformer) Transform(dag *graph.DAG) error {
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
