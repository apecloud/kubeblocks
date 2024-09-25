/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// shardingComponentTransformer transforms all cluster.Spec.ComponentSpecs to mapping Component objects
type shardingComponentTransformer struct{}

var _ graph.Transformer = &shardingComponentTransformer{}

func (t *shardingComponentTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*shardingTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) || len(transCtx.Cluster.Spec.ShardingSpecs) == 0 {
		return nil
	}

	expectedShardingCompCount := 0
	for _, s := range transCtx.ShardingToComponentSpecs {
		expectedShardingCompCount += len(s)
	}
	allCompsUpToDate, err := checkAllCompsUpToDate(transCtx.Context, transCtx.Client, transCtx.Cluster, expectedShardingCompCount, withShardingDefined)
	if err != nil {
		return err
	}

	// if the cluster is not updating and all components are up-to-date, skip the reconciliation
	if !transCtx.OrigCluster.IsUpdating() && allCompsUpToDate {
		return nil
	}

	return t.reconcileShardingComponents(transCtx, dag)
}

func (t *shardingComponentTransformer) reconcileShardingComponents(transCtx *shardingTransformContext, dag *graph.DAG) error {
	cluster := transCtx.Cluster

	protoCompSpecMap := make(map[string]*appsv1.ClusterComponentSpec)
	protoCompToShardingName := make(map[string]string)
	for shardingName, compSpecs := range transCtx.ShardingToComponentSpecs {
		for _, compSpec := range compSpecs {
			protoCompSpecMap[compSpec.Name] = compSpec
			protoCompToShardingName[compSpec.Name] = shardingName
		}
	}

	protoCompSet := sets.KeySet(protoCompSpecMap)
	runningCompSet, err := component.GetClusterComponentShortNameSet(transCtx.Context, transCtx.Client, cluster, withoutShardingDefined)
	if err != nil {
		return err
	}

	createCompSet := protoCompSet.Difference(runningCompSet)
	updateCompSet := protoCompSet.Intersection(runningCompSet)
	deleteCompSet := runningCompSet.Difference(protoCompSet)

	// TODO: support sharding topology order

	// sharding component objects to be deleted (scale-in)
	if err := t.handleCompsDelete(transCtx, dag, deleteCompSet, false); err != nil {
		return err
	}

	if err := t.handleCompsUpdate(transCtx, dag, protoCompSpecMap, protoCompToShardingName, updateCompSet); err != nil {
		return err
	}

	// sharding component objects to be created
	if err := t.handleCompsCreate(transCtx, dag, protoCompSpecMap, protoCompToShardingName, createCompSet); err != nil {
		return err
	}

	return nil
}

func (t *shardingComponentTransformer) handleCompsDelete(transCtx *shardingTransformContext, dag *graph.DAG, deleteCompSet sets.Set[string], terminate bool) error {
	for compName := range deleteCompSet {
		if err := handCompDelete(transCtx, dag, compName, terminate); err != nil {
			return err
		}
	}
	return nil
}

func (t *shardingComponentTransformer) handleCompsCreate(transCtx *shardingTransformContext, dag *graph.DAG,
	protoCompSpecMap map[string]*appsv1.ClusterComponentSpec, protoCompToShardingName map[string]string, createCompSet sets.Set[string]) error {
	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for compName := range createCompSet {
		comp, err := component.BuildComponent(cluster, protoCompSpecMap[compName], nil, nil)
		if err != nil {
			return err
		}
		comp.Labels[constant.KBAppShardingNameLabelKey] = protoCompToShardingName[compName]
		// use SetOwnerReference instead of SetControllerReference
		if err := intctrlutil.SetOwnership(cluster, comp, rscheme, constant.DBClusterFinalizerName, true); err != nil {
			if _, ok := err.(*controllerutil.AlreadyOwnedError); ok {
				continue
			}
			return err
		}
		graphCli.Create(dag, comp)
	}
	return nil
}

func (t *shardingComponentTransformer) handleCompsUpdate(transCtx *shardingTransformContext, dag *graph.DAG,
	protoCompSpecMap map[string]*appsv1.ClusterComponentSpec, protoCompToShardingName map[string]string, updateCompSet sets.Set[string]) error {
	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for compName := range updateCompSet {
		runningComp, getErr := getRunningCompObject(transCtx.Context, transCtx.Client, cluster, compName)
		if getErr != nil {
			return getErr
		}
		comp, buildErr := component.BuildComponent(cluster, protoCompSpecMap[compName], nil, nil)
		if buildErr != nil {
			return buildErr
		}
		comp.Labels[constant.KBAppShardingNameLabelKey] = protoCompToShardingName[compName]
		if newCompObj := copyAndMergeComponent(runningComp, comp); newCompObj != nil {
			graphCli.Update(dag, runningComp, newCompObj)
		}
	}
	return nil
}

func handCompDelete(transCtx *shardingTransformContext, dag *graph.DAG, compName string, terminate bool) error {
	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)
	comp, err := getRunningCompObject(transCtx.Context, transCtx.Client, cluster, compName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if apierrors.IsNotFound(err) || model.IsObjectDeleting(comp) {
		return nil
	}
	transCtx.Logger.Info(fmt.Sprintf("deleting component %s", comp.Name))
	deleteCompVertex := graphCli.Do(dag, nil, comp, model.ActionDeletePtr(), nil)
	if !terminate { // scale-in
		compCopy := comp.DeepCopy()
		if comp.Annotations == nil {
			comp.Annotations = make(map[string]string)
		}
		// update the scale-in annotation to component before deleting
		comp.Annotations[constant.ComponentScaleInAnnotationKey] = trueVal
		graphCli.Do(dag, compCopy, comp, model.ActionUpdatePtr(), deleteCompVertex)
	}
	return nil
}
