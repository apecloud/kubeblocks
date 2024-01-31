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
)

// ClusterAPINormalizationTransformer handles cluster and component API conversion.
type ClusterAPINormalizationTransformer struct{}

var _ graph.Transformer = &ClusterAPINormalizationTransformer{}

func (t *ClusterAPINormalizationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}

	transCtx.ComponentSpecs = make([]*appsv1alpha1.ClusterComponentSpec, 0)
	transCtx.ShardingComponentSpecs = make(map[string][]*appsv1alpha1.ClusterComponentSpec, 0)
	transCtx.Labels = make(map[string]map[string]string, 0)

	// build all component specs
	transCtx.ComponentSpecs = t.buildCompSpecs(transCtx, transCtx.Cluster)

	// resolve all component definitions referenced
	return t.resolveCompDefinitions(transCtx, transCtx.Cluster)
}

// func shardingComps() {
//	for i := range cluster.Spec.ComponentSpecs {
//		clusterComSpec := cluster.Spec.ComponentSpecs[i]
//		transCtx.ComponentSpecs = append(transCtx.ComponentSpecs, &clusterComSpec)
//	}
//	for i := range cluster.Spec.ShardingSpecs {
//		shardingSpec := cluster.Spec.ShardingSpecs[i]
//		genShardingCompSpecList, err := controllerutil.GenShardingCompSpecList(transCtx.Context, transCtx.Client, cluster, &shardingSpec)
//		if err != nil {
//			return err
//		}
//		transCtx.ShardingComponentSpecs[shardingSpec.Name] = genShardingCompSpecList
//		for j := range genShardingCompSpecList {
//			genShardCompSpec := genShardingCompSpecList[j]
//			transCtx.ComponentSpecs = append(transCtx.ComponentSpecs, genShardCompSpec)
//			transCtx.Labels[genShardCompSpec.Name] = constant.GetShardingNameLabel(shardingSpec.Name)
//		}
//	}
//
//	if compSpec := apiconversion.HandleSimplifiedClusterAPI(transCtx.ClusterDef, cluster); compSpec != nil {
//		transCtx.ComponentSpecs = append(transCtx.ComponentSpecs, compSpec)
//	}
// }

func (t *ClusterAPINormalizationTransformer) buildCompSpecs(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster) []*appsv1alpha1.ClusterComponentSpec {
	if withClusterTopology(cluster) {
		return t.buildCompSpecs4ClusterTopology(transCtx.ClusterDef, cluster)
	}
	if withLegacyClusterDef(cluster) {
		return t.buildCompSpecs4LegacyCluster(cluster)
	}
	if withSimplifiedClusterAPI(cluster) {
		return t.buildCompSpecs4SimplifiedAPI(transCtx.ClusterDef, cluster)
	}
	return nil
}

func (t *ClusterAPINormalizationTransformer) buildCompSpecs4ClusterTopology(clusterDef *appsv1alpha1.ClusterDefinition,
	cluster *appsv1alpha1.Cluster) []*appsv1alpha1.ClusterComponentSpec {
	newCompSpec := func(comp appsv1alpha1.ClusterTopologyComponent) *appsv1alpha1.ClusterComponentSpec {
		return &appsv1alpha1.ClusterComponentSpec{
			Name:           comp.Name,
			ComponentDef:   comp.CompDef,
			ServiceVersion: comp.ServiceVersion,
			ServiceRefs:    comp.ServiceRefs,
		}
	}

	mergeCompSpec := func(comp appsv1alpha1.ClusterTopologyComponent, compSpec *appsv1alpha1.ClusterComponentSpec) *appsv1alpha1.ClusterComponentSpec {
		if len(compSpec.ComponentDef) == 0 {
			compSpec.ComponentDef = comp.CompDef
		}
		if len(compSpec.ServiceVersion) == 0 {
			compSpec.ServiceVersion = comp.ServiceVersion
		}
		serviceRefs := make(map[string]bool)
		for _, ref := range compSpec.ServiceRefs {
			serviceRefs[ref.Name] = true
		}
		for i, ref := range comp.ServiceRefs {
			if _, ok := serviceRefs[ref.Name]; ok {
				continue
			}
			compSpec.ServiceRefs = append(compSpec.ServiceRefs, comp.ServiceRefs[i])
		}
		return compSpec
	}

	// TODO: the default topology may be changed
	clusterTopology := referredClusterTopology(clusterDef, cluster.Spec.Topology)
	if clusterTopology == nil {
		panic(fmt.Sprintf("runtime error - cluster topology not found : %s", cluster.Spec.Topology))
	}

	specifiedCompSpecs := make(map[string]*appsv1alpha1.ClusterComponentSpec)
	for i, compSpec := range cluster.Spec.ComponentSpecs {
		specifiedCompSpecs[compSpec.Name] = cluster.Spec.ComponentSpecs[i].DeepCopy()
	}

	compSpecs := make([]*appsv1alpha1.ClusterComponentSpec, 0)
	for i := range clusterTopology.Components {
		comp := clusterTopology.Components[i]
		if _, ok := specifiedCompSpecs[comp.Name]; ok {
			compSpecs = append(compSpecs, mergeCompSpec(comp, specifiedCompSpecs[comp.Name]))
		} else {
			compSpecs = append(compSpecs, newCompSpec(comp))
		}
	}
	return compSpecs
}

func (t *ClusterAPINormalizationTransformer) buildCompSpecs4LegacyCluster(cluster *appsv1alpha1.Cluster) []*appsv1alpha1.ClusterComponentSpec {
	compSpecs := make([]*appsv1alpha1.ClusterComponentSpec, 0)
	for i := range cluster.Spec.ComponentSpecs {
		clusterComSpec := cluster.Spec.ComponentSpecs[i]
		compSpecs = append(compSpecs, &clusterComSpec)
	}
	return compSpecs
}

func (t *ClusterAPINormalizationTransformer) buildCompSpecs4SimplifiedAPI(clusterDef *appsv1alpha1.ClusterDefinition,
	cluster *appsv1alpha1.Cluster) []*appsv1alpha1.ClusterComponentSpec {
	return []*appsv1alpha1.ClusterComponentSpec{apiconversion.HandleSimplifiedClusterAPI(clusterDef, cluster)}
}

func (t *ClusterAPINormalizationTransformer) resolveCompDefinitions(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster) error {
	if transCtx.ComponentDefs == nil {
		transCtx.ComponentDefs = make(map[string]*appsv1alpha1.ComponentDefinition)
	}
	for i, compSpec := range transCtx.ComponentSpecs {
		if len(compSpec.ComponentDef) == 0 {
			compDef, err := t.buildCompDefinition4Legacy(transCtx, compSpec)
			if err != nil {
				return err
			}
			transCtx.ComponentDefs[compDef.Name] = compDef
			transCtx.ComponentSpecs[i].ComponentDef = compDef.Name
			compDef.Name = "" // TODO
		} else {
			compDef, serviceVersion, err := resolveCompDefinitionNServiceVersion(
				transCtx.Context, transCtx.Client, compSpec.ComponentDef, compSpec.ServiceVersion)
			if err != nil {
				return err
			}
			transCtx.ComponentDefs[compDef.Name] = compDef
			// set the componentDef and serviceVersion as resolved
			transCtx.ComponentSpecs[i].ComponentDef = compDef.Name
			transCtx.ComponentSpecs[i].ServiceVersion = serviceVersion
		}
	}
	return nil
}

func (t *ClusterAPINormalizationTransformer) buildCompDefinition4Legacy(transCtx *clusterTransformContext,
	compSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.ComponentDefinition, error) {
	compDef, err := component.BuildComponentDefinition(transCtx.ClusterDef, transCtx.ClusterVer, compSpec)
	if err != nil {
		return nil, err
	}
	compDef.Name = constant.GenerateVirtualComponentDefinition(compSpec.ComponentDefRef)
	return compDef, nil
}
