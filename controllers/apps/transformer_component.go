/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package apps

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
	if err != nil && !ictrlutil.IsDelayedRequeueError(err) {
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
	return err
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
	var delayedError error
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		dag := graph.NewDAG()
		comp, err := components.NewComponent(reqCtx, c.Client, clusterDef, clusterVer, cluster, compSpec.Name, dag)
		if err != nil {
			return err
		}
		if err := comp.Status(reqCtx, c.Client); err != nil {
			if !ictrlutil.IsDelayedRequeueError(err) {
				return err
			}
			if delayedError == nil {
				delayedError = err
			}
		}
		*dags = append(*dags, dag)
	}
	return delayedError
}
