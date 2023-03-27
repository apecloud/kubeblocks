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
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// componentTransformer transforms all components to a K8s objects DAG
// TODO: remove cli and ctx, we should read all objects needed, and then do pure objects computation
// TODO: only replication set left
type componentTransformer struct {
	cc  clusterRefResources
	cli client.Client
	ctx intctrlutil.RequestCtx
}

func (c *componentTransformer) Transform(dag *graph.DAG) error {
	rootVertex, err := findRootVertex(dag)
	if err != nil {
		return err
	}
	origCluster, _ := rootVertex.oriObj.(*appsv1alpha1.Cluster)
	cluster, _ := rootVertex.obj.(*appsv1alpha1.Cluster)

	// return fast when cluster is deleting
	if isClusterDeleting(*origCluster) {
		return nil
	}

	compSpecMap := make(map[string]*appsv1alpha1.ClusterComponentSpec)
	for _, spec := range cluster.Spec.ComponentSpecs {
		compSpecMap[spec.Name] = &spec
	}
	compProto := sets.KeySet(compSpecMap)
	// TODO: should review that whether it is reasonable to use component status
	compStatus := sets.KeySet(cluster.Status.Components)

	createSet := compProto.Difference(compStatus)
	updateSet := compProto.Intersection(compStatus)
	deleteSet := compStatus.Difference(compProto)

	dag4Component := graph.NewDAG()
	for compName := range createSet {
		comp, err := NewComponent(&c.cc.cd, &c.cc.cv, cluster, compName, dag4Component)
		if err != nil {
			return err
		}
		if err := comp.Create(c.ctx, c.cli); err != nil {
			return err
		}
	}

	for compName := range deleteSet {
		comp, err := NewComponent(&c.cc.cd, &c.cc.cv, cluster, compName, dag4Component)
		if err != nil {
			return err
		}
		if err := comp.Delete(c.ctx, c.cli); err != nil {
			return err
		}
	}

	for compName := range updateSet {
		comp, err := NewComponent(&c.cc.cd, &c.cc.cv, cluster, compName, dag4Component)
		if err != nil {
			return err
		}
		// TODO: will replace Update with specified operations(restart, reconfigure, h/v-scaling...) after ops-request forwards to cluster controller
		if err := comp.Update(c.ctx, c.cli); err != nil {
			return err
		}
	}

	for _, v := range dag4Component.Vertices() {
		node, ok := v.(*lifecycleVertex)
		if !ok {
			panic("runtime error, unexpected lifecycle vertex type")
		}
		if node.obj == nil {
			panic("runtime error, nil vertex object")
		}
	}
	dag.Merge(dag4Component)

	fmt.Printf("cluster: %s, dag: %s\n", cluster.GetName(), dag)

	return nil
}
