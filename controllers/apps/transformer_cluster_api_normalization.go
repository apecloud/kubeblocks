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
	"fmt"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/apiconversion"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
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
	transCtx.GenerateComponentSpecs = make([]*GenerateComponentSpec, 0)
	cluster := transCtx.Cluster

	for i := range cluster.Spec.ComponentSpecs {
		clusterComSpec := cluster.Spec.ComponentSpecs[i]
		transCtx.GenerateComponentSpecs = append(transCtx.GenerateComponentSpecs, &GenerateComponentSpec{
			ComponentSpec: &clusterComSpec,
			Labels:        nil,
		})
	}
	for i := range cluster.Spec.ShardingSpecs {
		shardingSpec := cluster.Spec.ShardingSpecs[i]
		genShardingCompSpecList := sharding.GenShardingCompSpecList(&shardingSpec)
		for j := range genShardingCompSpecList {
			genShardCompSpec := genShardingCompSpecList[j]
			transCtx.GenerateComponentSpecs = append(transCtx.GenerateComponentSpecs, &GenerateComponentSpec{
				ComponentSpec: genShardCompSpec,
				Labels:        constant.GetShardTemplateNameLabel(shardingSpec.Name),
			})
		}
	}

	if compSpec := apiconversion.HandleSimplifiedClusterAPI(transCtx.ClusterDef, cluster); compSpec != nil {
		transCtx.GenerateComponentSpecs = append(transCtx.GenerateComponentSpecs, &GenerateComponentSpec{
			ComponentSpec: compSpec,
			Labels:        nil,
		})
	}

	// validate componentDef and componentDefRef
	if err := validateComponentDefNComponentDefRef(transCtx); err != nil {
		return err
	}

	// build all component definitions referenced
	if transCtx.ComponentDefs == nil {
		transCtx.ComponentDefs = make(map[string]*appsv1alpha1.ComponentDefinition)
	}
	for i, genCompSpec := range transCtx.GenerateComponentSpecs {
		if len(genCompSpec.ComponentSpec.ComponentDef) == 0 {
			compDef, err := component.BuildComponentDefinition(transCtx.ClusterDef, transCtx.ClusterVer, genCompSpec.ComponentSpec)
			if err != nil {
				return err
			}
			virtualCompDefName := constant.GenerateVirtualComponentDefinition(genCompSpec.ComponentSpec.ComponentDefRef)
			transCtx.ComponentDefs[virtualCompDefName] = compDef
			transCtx.GenerateComponentSpecs[i].ComponentSpec.ComponentDef = virtualCompDefName
		} else {
			// should be loaded at load resources transformer
			if _, ok := transCtx.ComponentDefs[genCompSpec.ComponentSpec.ComponentDef]; !ok {
				panic(fmt.Sprintf("runtime error - expected component definition object not found: %s", genCompSpec.ComponentSpec.ComponentDef))
			}
		}
	}
	return nil
}

func validateComponentDefNComponentDefRef(transCtx *clusterTransformContext) error {
	if len(transCtx.GenerateComponentSpecs) == 0 {
		return nil
	}
	hasCompDef := false
	for _, genCompSpec := range transCtx.GenerateComponentSpecs {
		if len(genCompSpec.ComponentSpec.ComponentDefRef) == 0 && len(genCompSpec.ComponentSpec.ComponentDef) == 0 {
			return fmt.Errorf("componentDef and componentDefRef cannot be both empty")
		}
		if len(genCompSpec.ComponentSpec.ComponentDef) == 0 && hasCompDef {
			return fmt.Errorf("all componentSpecs in the same cluster must either specify ComponentDef or omit ComponentDef simultaneously")
		}
		if len(genCompSpec.ComponentSpec.ComponentDef) > 0 {
			hasCompDef = true
		}
	}
	return nil
}
