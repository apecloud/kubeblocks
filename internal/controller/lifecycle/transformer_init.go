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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

type initTransformer struct {
	cluster       *appsv1alpha1.Cluster
	originCluster *appsv1alpha1.Cluster
}

func (i *initTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	// put the cluster object first, it will be root vertex of DAG
	rootVertex := &lifecycleVertex{obj: i.cluster, oriObj: i.originCluster}
	dag.AddVertex(rootVertex)
	return nil
}
