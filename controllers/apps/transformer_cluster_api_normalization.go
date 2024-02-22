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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/apiconversion"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// ClusterAPINormalizationTransformer handles cluster and component API conversion.
type ClusterAPINormalizationTransformer struct{}

var _ graph.Transformer = &ClusterAPINormalizationTransformer{}

func (t *ClusterAPINormalizationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}

	// build all component specs
	transCtx.ComponentSpecs = make([]*appsv1alpha1.ClusterComponentSpec, 0)
	transCtx.ShardingComponentSpecs = make(map[string][]*appsv1alpha1.ClusterComponentSpec, 0)
	transCtx.Labels = make(map[string]map[string]string, 0)
	cluster := transCtx.Cluster

	for i := range cluster.Spec.ComponentSpecs {
		clusterComSpec := cluster.Spec.ComponentSpecs[i]
		transCtx.ComponentSpecs = append(transCtx.ComponentSpecs, &clusterComSpec)
		// inherit cluster labels
		transCtx.Labels[clusterComSpec.Name] = cluster.Labels
	}
	for i := range cluster.Spec.ShardingSpecs {
		shardingSpec := cluster.Spec.ShardingSpecs[i]
		genShardingCompSpecList, err := controllerutil.GenShardingCompSpecList(transCtx.Context, transCtx.Client, cluster, &shardingSpec)
		if err != nil {
			return err
		}
		transCtx.ShardingComponentSpecs[shardingSpec.Name] = genShardingCompSpecList
		for j := range genShardingCompSpecList {
			genShardCompSpec := genShardingCompSpecList[j]
			transCtx.ComponentSpecs = append(transCtx.ComponentSpecs, genShardCompSpec)
			transCtx.Labels[genShardCompSpec.Name] = controllerutil.MergeMetadataMaps(cluster.Labels, constant.GetShardingNameLabel(shardingSpec.Name))
		}
	}

	if compSpec := apiconversion.HandleSimplifiedClusterAPI(transCtx.ClusterDef, cluster); compSpec != nil {
		transCtx.ComponentSpecs = append(transCtx.ComponentSpecs, compSpec)
		transCtx.Labels[compSpec.Name] = cluster.Labels
	}

	// validate componentDef and componentDefRef
	if err := validateComponentDefNComponentDefRef(transCtx); err != nil {
		return err
	}

	// build all component definitions referenced
	if transCtx.ComponentDefs == nil {
		transCtx.ComponentDefs = make(map[string]*appsv1alpha1.ComponentDefinition)
	}
	for i, compSpec := range transCtx.ComponentSpecs {
		if len(compSpec.ComponentDef) == 0 {
			compDef, err := component.BuildComponentDefinition(transCtx.ClusterDef, transCtx.ClusterVer, compSpec)
			if err != nil {
				return err
			}
			virtualCompDefName := constant.GenerateVirtualComponentDefinition(compSpec.ComponentDefRef)
			transCtx.ComponentDefs[virtualCompDefName] = compDef
			transCtx.ComponentSpecs[i].ComponentDef = virtualCompDefName
		} else {
			// should be loaded at load resources transformer
			if _, ok := transCtx.ComponentDefs[compSpec.ComponentDef]; !ok {
				panic(fmt.Sprintf("runtime error - expected component definition object not found: %s", compSpec.ComponentDef))
			}
		}
	}
	return nil
}

func validateComponentDefNComponentDefRef(transCtx *clusterTransformContext) error {
	if len(transCtx.ComponentSpecs) == 0 {
		return nil
	}
	hasCompDef := false
	for _, compSpec := range transCtx.ComponentSpecs {
		if len(compSpec.ComponentDefRef) == 0 && len(compSpec.ComponentDef) == 0 {
			return fmt.Errorf("componentDef and componentDefRef cannot be both empty")
		}
		if len(compSpec.ComponentDef) == 0 && hasCompDef {
			return fmt.Errorf("all componentSpecs in the same cluster must either specify ComponentDef or omit ComponentDef simultaneously")
		}
		if len(compSpec.ComponentDef) > 0 {
			hasCompDef = true
		}
	}
	return nil
}
