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
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// ClusterDeletionTransformer handles cluster deletion
type ClusterDeletionTransformer struct{}

func (t *ClusterDeletionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.OrigCluster
	if !isClusterDeleting(*cluster) {
		return nil
	}

	if !controllerutil.ContainsFinalizer(cluster, dbClusterFinalizerName) {
		return graph.ErrFastReturn
	}

	// list all objects owned by this cluster in cache, and delete them all
	// there is chance that objects leak occurs because of cache stale
	// ignore the problem currently
	// TODO: GC the leaked objects
	snapshot, err := t.readCacheSnapshot(transCtx, *cluster)
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

// TODO: @free6om dedup after model abstraction done
// read all objects owned by our cluster
func (t *ClusterDeletionTransformer) readCacheSnapshot(transCtx *ClusterTransformContext, cluster appsv1alpha1.Cluster) (clusterSnapshot, error) {
	// list what kinds of object cluster owns
	kinds := ownKinds()
	snapshot := make(clusterSnapshot)
	ml := client.MatchingLabels{constant.AppInstanceLabelKey: cluster.GetName()}
	inNS := client.InNamespace(cluster.Namespace)
	for _, list := range kinds {
		if err := transCtx.Client.List(transCtx.Context, list, inNS, ml); err != nil {
			return nil, err
		}
		// reflect get list.Items
		items := reflect.ValueOf(list).Elem().FieldByName("Items")
		l := items.Len()
		for i := 0; i < l; i++ {
			// get the underlying object
			object := items.Index(i).Addr().Interface().(client.Object)
			// put to snapshot if owned by our cluster
			if isOwnerOf(&cluster, object, scheme) {
				name, err := getGVKName(object, scheme)
				if err != nil {
					return nil, err
				}
				snapshot[*name] = object
			}
		}
	}

	return snapshot, nil
}

var _ graph.Transformer = &ClusterDeletionTransformer{}
