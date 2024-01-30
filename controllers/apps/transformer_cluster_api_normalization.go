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
	"context"
	"fmt"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/version"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

	// build all component specs
	transCtx.ComponentSpecs = make([]*appsv1alpha1.ClusterComponentSpec, 0)
	transCtx.ShardingComponentSpecs = make(map[string][]*appsv1alpha1.ClusterComponentSpec, 0)
	transCtx.Labels = make(map[string]map[string]string, 0)

	cluster := transCtx.Cluster
	transCtx.ComponentSpecs = t.buildCompSpecs(transCtx, cluster)

	// resolve all component definitions referenced
	return t.resolveCompDefinitions(transCtx, cluster)
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
			compDef, serviceVersion, err := t.resolveCompDefinitionNServiceVersion(transCtx, compSpec)
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

func (t *ClusterAPINormalizationTransformer) resolveCompDefinitionNServiceVersion(transCtx *clusterTransformContext,
	compSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.ComponentDefinition, string, error) {
	var (
		compDef        *appsv1alpha1.ComponentDefinition
		serviceVersion string
	)
	compDefs, err := t.listCompDefinitions(transCtx, compSpec.ComponentDef)
	if err != nil {
		return compDef, serviceVersion, err
	}

	serviceVersionToCompDefs, err := t.buildServiceVersionToCompDefsMapping(transCtx.Context, transCtx.Client, compDefs, compSpec.ServiceVersion)
	if err != nil {
		return compDef, serviceVersion, err
	}

	// use specified service version or the latest.
	serviceVersion = compSpec.ServiceVersion
	if len(compSpec.ServiceVersion) == 0 {
		serviceVersions := maps.Keys(serviceVersionToCompDefs)
		slices.Sort(serviceVersions)
		serviceVersion = serviceVersions[len(serviceVersions)-1]
	}

	compatibleCompDefs := serviceVersionToCompDefs[serviceVersion]
	if len(compatibleCompDefs) == 0 {
		return compDef, serviceVersion, fmt.Errorf("no matched component definition found: %s", compSpec.ComponentDef)
	}

	compatibleCompDefNames := maps.Keys(compatibleCompDefs)
	slices.Sort(compatibleCompDefNames)
	compatibleCompDefName := compatibleCompDefNames[len(compatibleCompDefNames)-1]

	return compatibleCompDefs[compatibleCompDefName], serviceVersion, nil
}

func (t *ClusterAPINormalizationTransformer) listCompDefinitions(transCtx *clusterTransformContext, compDef string) ([]*appsv1alpha1.ComponentDefinition, error) {
	compDefList := &appsv1alpha1.ComponentDefinitionList{}
	if err := transCtx.Client.List(transCtx.Context, compDefList); err != nil {
		return nil, err
	}
	compDefsFullyMatched := make([]*appsv1alpha1.ComponentDefinition, 0)
	compDefsPrefixMatched := make([]*appsv1alpha1.ComponentDefinition, 0)
	for i, item := range compDefList.Items {
		if item.Name == compDef {
			compDefsFullyMatched = append(compDefsFullyMatched, &compDefList.Items[i])
		}
		if strings.HasPrefix(item.Name, compDef) {
			compDefsPrefixMatched = append(compDefsPrefixMatched, &compDefList.Items[i])
		}
	}
	if len(compDefsFullyMatched) > 0 {
		return compDefsFullyMatched, nil
	}
	return compDefsPrefixMatched, nil
}

func (t *ClusterAPINormalizationTransformer) buildServiceVersionToCompDefsMapping(ctx context.Context, cli client.Reader,
	compDefs []*appsv1alpha1.ComponentDefinition, serviceVersion string) (map[string]map[string]*appsv1alpha1.ComponentDefinition, error) {
	result := make(map[string]map[string]*appsv1alpha1.ComponentDefinition)

	insert := func(version string, compDef *appsv1alpha1.ComponentDefinition) {
		if _, ok := result[version]; !ok {
			result[version] = make(map[string]*appsv1alpha1.ComponentDefinition)
		}
		result[version][compDef.Name] = compDef
	}

	checkedInsert := func(version string, compDef *appsv1alpha1.ComponentDefinition) {
		if len(serviceVersion) == 0 {
			insert(version, compDef)
		} else if compareServiceVersion(serviceVersion, version) {
			insert(version, compDef)
		}
	}

	for _, compDef := range compDefs {
		compVersions, err := compatibleCompVersions(ctx, cli, compDef)
		if err != nil {
			return nil, err
		}

		serviceVersions := sets.New[string]()
		for _, compVersion := range compVersions {
			serviceVersions = serviceVersions.Union(compatibleServiceVersions(compDef, compVersion))
		}

		for version := range serviceVersions {
			checkedInsert(version, compDef)
		}
	}
	return result, nil
}

// compatibleCompVersions returns all component versions that are compatible with specified component definition.
func compatibleCompVersions(ctx context.Context, cli client.Reader, compDef *appsv1alpha1.ComponentDefinition) ([]*appsv1alpha1.ComponentVersion, error) {
	compVersionList := &appsv1alpha1.ComponentVersionList{}
	labels := client.MatchingLabels{
		compDef.Name: compDef.Name,
	}
	if err := cli.List(ctx, compVersionList, labels); err != nil {
		return nil, err
	}

	if len(compVersionList.Items) == 0 {
		return nil, nil
	}

	compVersions := make([]*appsv1alpha1.ComponentVersion, 0)
	for i, compVersion := range compVersionList.Items {
		if compVersion.Status.Phase != appsv1alpha1.AvailablePhase {
			return nil, fmt.Errorf("matched ComponentVersion %s is not available", compVersion.Name)
		}
		compVersions = append(compVersions, &compVersionList.Items[i])
	}
	return compVersions, nil
}

// compatibleServiceVersions returns service versions that are compatible with specified component definition.
func compatibleServiceVersions(compDef *appsv1alpha1.ComponentDefinition, compVersion *appsv1alpha1.ComponentVersion) sets.Set[string] {
	prefixMatch := func(prefix string) bool {
		return strings.HasPrefix(compDef.Name, prefix)
	}
	releases := make(map[string]bool, 0)
	for _, rule := range compVersion.Spec.CompatibilityRules {
		if slices.IndexFunc(rule.CompDefs, prefixMatch) >= 0 {
			for _, release := range rule.Releases {
				releases[release] = true
			}
		}
	}
	serviceVersions := sets.New[string]()
	for _, release := range compVersion.Spec.Releases {
		if releases[release.Name] {
			serviceVersions = serviceVersions.Insert(release.ServiceVersion)
		}
	}
	return serviceVersions
}

// TODO
func compareServiceVersion(required, provide string) bool {
	ret, err := version.MustParseSemantic(required).Compare(provide)
	return err == nil && ret == 0
}
