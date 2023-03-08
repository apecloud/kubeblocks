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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type clusterStatusTransformer struct {}

func (c *clusterStatusTransformer) Transform(dag *graph.DAG) error {
	// get root(cluster) vertex
	rootVertex := dag.Root()
	if rootVertex == nil {
		return fmt.Errorf("root vertex not found: %v", dag)
	}
	root, _ := rootVertex.(*lifecycleVertex)
	cluster, _ := root.obj.(*appsv1alpha1.Cluster)
	patch := client.MergeFrom(cluster.DeepCopy())
	// update generation
	cluster.Status.ObservedGeneration = cluster.Generation
	// TODO: update other status fields

	root.patch = patch
	return nil
}