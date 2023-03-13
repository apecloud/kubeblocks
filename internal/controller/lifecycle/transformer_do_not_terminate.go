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
	"fmt"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

type doNotTerminateTransformer struct {}

func (d *doNotTerminateTransformer) Transform(dag *graph.DAG) error {
	root := dag.Root()
	if root == nil {
		return fmt.Errorf("root vertex not found: %v", dag)
	}
	rootVertex, _ := root.(*lifecycleVertex)
	cluster, _ := rootVertex.obj.(*appsv1alpha1.Cluster)

	if cluster.DeletionTimestamp.IsZero() {
		return nil
	}
	if cluster.Spec.TerminationPolicy != appsv1alpha1.DoNotTerminate {
		return nil
	}
	vertices, err := findAllNot[*appsv1alpha1.Cluster](dag)
	if err != nil {
		return err
	}
	for _, vertex := range vertices {
		dag.RemoveVertex(vertex)
	}
	return nil
}
