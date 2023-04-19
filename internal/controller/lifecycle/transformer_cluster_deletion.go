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
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	corev1 "k8s.io/api/core/v1"
)

// ClusterDeletionTransformer handles cluster deletion
type ClusterDeletionTransformer struct{}

func (t *ClusterDeletionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.OrigCluster
	if !isClusterDeleting(*cluster) {
		return nil
	}

	// list all objects owned by this cluster in cache, and delete them all
	// there is chance that objects leak occurs because of cache stale
	// ignore the problem currently
	// TODO: GC the leaked objects
	kinds := ownKinds()
	kinds = append(kinds, &corev1.PersistentVolumeClaimList{})
	snapshot, err := readCacheSnapshot(transCtx, *cluster, kinds...)
	if err != nil {
		return err
	}
	root, err := findRootVertex(dag)
	if err != nil {
		return err
	}
	for _, object := range snapshot {
		vertex := &lifecycleVertex{obj: object, action: actionPtr(DELETE)}
		dag.AddVertex(vertex)
		dag.Connect(root, vertex)
	}
	root.action = actionPtr(DELETE)

	// fast return, that is stopping the plan.Build() stage and jump to plan.Execute() directly
	return graph.ErrFastReturn
}

var _ graph.Transformer = &ClusterDeletionTransformer{}
