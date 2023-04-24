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

// ComponentTransformer transforms all components to a K8s objects DAG
type ComponentTransformer struct {
	client.Client
}

var _ graph.Transformer = &ComponentTransformer{}

func (c *ComponentTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	origCluster := transCtx.OrigCluster
	cluster := transCtx.Cluster
	if origCluster.IsDeleting() {
		return nil
	}

	clusterDef := transCtx.ClusterDef
	clusterVer := transCtx.ClusterVer
	reqCtx := ictrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}

	var err error
	dags4Component := make([]*graph.DAG, 0)
	if cluster.IsStatusUpdating() {
		// status existed components
		err = c.transform4StatusUpdate(reqCtx, clusterDef, clusterVer, cluster, &dags4Component)
	} else {
		// create new components or update existed components
		err = c.transform4SpecUpdate(reqCtx, clusterDef, clusterVer, cluster, &dags4Component)
	}
	if err != nil {
		return err
	}

	for _, subDag := range dags4Component {
		for _, v := range subDag.Vertices() {
			node, ok := v.(*ictrltypes.LifecycleVertex)
			if !ok {
				panic("runtime error, unexpected lifecycle vertex type")
			}
			if node.Obj == nil {
				panic("runtime error, nil vertex object")
			}
		}
		dag.Merge(subDag)
	}
	return nil
}

func (c *ComponentTransformer) transform4SpecUpdate(reqCtx ictrlutil.RequestCtx, clusterDef *appsv1alpha1.ClusterDefinition,
	clusterVer *appsv1alpha1.ClusterVersion, cluster *appsv1alpha1.Cluster, dags *[]*graph.DAG) error {
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
		dag := graph.NewDAG()
		comp, err := components.NewComponent(reqCtx, c.Client, clusterDef, clusterVer, cluster, compName, dag)
		if err != nil {
			return err
		}
		if err := comp.Create(reqCtx, c.Client); err != nil {
			return err
		}
		*dags = append(*dags, dag)
	}

	for compName := range deleteSet {
		dag := graph.NewDAG()
		comp, err := components.NewComponent(reqCtx, c.Client, clusterDef, clusterVer, cluster, compName, dag)
		if err != nil {
			return err
		}
		if comp != nil {
			if err := comp.Delete(reqCtx, c.Client); err != nil {
				return err
			}
		}
		*dags = append(*dags, dag)
	}

	for compName := range updateSet {
		dag := graph.NewDAG()
		comp, err := components.NewComponent(reqCtx, c.Client, clusterDef, clusterVer, cluster, compName, dag)
		if err != nil {
			return err
		}
		if err := comp.Update(reqCtx, c.Client); err != nil {
			return err
		}
		*dags = append(*dags, dag)
	}

	return nil
}

func (c *ComponentTransformer) transform4StatusUpdate(reqCtx ictrlutil.RequestCtx, clusterDef *appsv1alpha1.ClusterDefinition,
	clusterVer *appsv1alpha1.ClusterVersion, cluster *appsv1alpha1.Cluster, dags *[]*graph.DAG) error {
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		dag := graph.NewDAG()
		comp, err := components.NewComponent(reqCtx, c.Client, clusterDef, clusterVer, cluster, compSpec.Name, dag)
		if err != nil {
			return err
		}
		if err := comp.Status(reqCtx, c.Client); err != nil {
			return err
		}
		*dags = append(*dags, dag)
	}
	return nil
}
