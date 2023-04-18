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
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	ictrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// componentTransformer transforms all components to a K8s objects DAG
// TODO: remove cli and ctx, we should read all objects needed, and then do pure objects computation
// TODO: only replication set left
type componentTransformer struct {
	cc  clusterRefResources
	cli client.Client
	ctx ictrlutil.RequestCtx
}

func (c *componentTransformer) Transform(dag *graph.DAG) error {
	rootVertex, err := ictrltypes.FindRootVertex(dag)
	if err != nil {
		return err
	}
	origCluster, _ := rootVertex.ObjCopy.(*appsv1alpha1.Cluster)
	cluster, _ := rootVertex.Obj.(*appsv1alpha1.Cluster)

	// return fast when cluster is deleting
	if origCluster.IsDeleting() {
		return nil
	}

	dag4Component := graph.NewDAG()

	// create new components or update existed components
	err = c.transform4SpecUpdate(cluster, dag4Component)
	if err != nil {
		return err
	}

	// status existed components
	err = c.transform4StatusUpdate(cluster, dag4Component)
	if err != nil {
		return err
	}

	for _, v := range dag4Component.Vertices() {
		node, ok := v.(*ictrltypes.LifecycleVertex)
		if !ok {
			panic("runtime error, unexpected lifecycle vertex type")
		}
		if node.Obj == nil {
			panic("runtime error, nil vertex object")
		}
	}
	dag.Merge(dag4Component)

	return nil
}

func (c *componentTransformer) transform4SpecUpdate(cluster *appsv1alpha1.Cluster, dag *graph.DAG) error {
	compSpecMap := make(map[string]*appsv1alpha1.ClusterComponentSpec)
	for _, spec := range cluster.Spec.ComponentSpecs {
		compSpecMap[spec.Name] = &spec
	}
	compProto := sets.KeySet(compSpecMap)
	// TODO(refactor): should review that whether it is reasonable to use component status
	compStatus := sets.KeySet(cluster.Status.Components)

	createSet := compProto.Difference(compStatus)
	updateSet := compProto.Intersection(compStatus)
	deleteSet := compStatus.Difference(compProto)

	for compName := range createSet {
		comp, err := components.NewComponent(c.ctx, c.cli, &c.cc.cd, &c.cc.cv, cluster, compName, dag)
		if err != nil {
			return err
		}
		if err := comp.Create(c.ctx, c.cli); err != nil {
			return err
		}
	}

	for compName := range deleteSet {
		comp, err := components.NewComponent(c.ctx, c.cli, &c.cc.cd, &c.cc.cv, cluster, compName, dag)
		if err != nil {
			return err
		}
		if comp != nil {
			if err := comp.Delete(c.ctx, c.cli); err != nil {
				return err
			}
		}
	}

	for compName := range updateSet {
		comp, err := components.NewComponent(c.ctx, c.cli, &c.cc.cd, &c.cc.cv, cluster, compName, dag)
		if err != nil {
			return err
		}
		// TODO(refactor): will replace Update with specified operations(restart, reconfigure, h/v-scaling...) after ops-request forwards to cluster controller
		if err := comp.Update(c.ctx, c.cli); err != nil {
			return err
		}
	}

	return nil
}

func (c *componentTransformer) transform4StatusUpdate(cluster *appsv1alpha1.Cluster, dag *graph.DAG) error {
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		comp, err := components.NewComponent(c.ctx, c.cli, &c.cc.cd, &c.cc.cv, cluster, compSpec.Name, dag)
		if err != nil {
			return err
		}
		if err := comp.Status(c.ctx, c.cli); err != nil {
			return err
		}
	}
	return nil
}
