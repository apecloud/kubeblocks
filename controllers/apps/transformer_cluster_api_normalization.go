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
	cluster := transCtx.Cluster
	transCtx.ComponentSpecs = t.buildCompSpecs(transCtx, cluster)

	// build all component definitions referenced
	return t.buildCompDefinitions(transCtx, cluster)
}

func (t *ClusterAPINormalizationTransformer) buildCompSpecs(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster) []*appsv1alpha1.ClusterComponentSpec {
	if withClusterTopology(cluster) {
		return t.buildCompSpecs4ClusterTopology(transCtx.ClusterDef, cluster)
	}
	if legacyClusterDef(cluster) {
		return t.buildCompSpecs4LegacyCluster(cluster)
	}
	if apiconversion.HasSimplifiedClusterAPI(cluster) {
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

	clusterTopology := referredClusterTopology(clusterDef, cluster.Spec.Topology)
	if clusterTopology == nil {
		panic(fmt.Sprintf("runtime error - cluster topology not found : %s", cluster.Spec.Topology))
	}

	specifiedCompSpecs := make(map[string]*appsv1alpha1.ClusterComponentSpec)
	for i, compSpec := range cluster.Spec.ComponentSpecs {
		specifiedCompSpecs[compSpec.Name] = &cluster.Spec.ComponentSpecs[i]
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

func (t *ClusterAPINormalizationTransformer) buildCompDefinitions(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster) error {
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
			compDef, err := t.loadNCheckCompDefinition(transCtx, compSpec)
			if err != nil {
				return err
			}
			transCtx.ComponentDefs[compDef.Name] = compDef
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

func (t *ClusterAPINormalizationTransformer) loadNCheckCompDefinition(transCtx *clusterTransformContext,
	compSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.ComponentDefinition, error) {
	compDef, err := t.resolveCompDefinition(transCtx, compSpec.ComponentDef, compSpec.ServiceVersion)
	if err != nil {
		return nil, err
	}
	if compDef.Status.Phase != appsv1alpha1.AvailablePhase {
		return nil, fmt.Errorf("referred ComponentDefinition is unavailable: %s", compDef.Name)
	}
	return compDef, nil
}

func (t *ClusterAPINormalizationTransformer) resolveCompDefinition(transCtx *clusterTransformContext,
	compDef, serviceVersion string) (*appsv1alpha1.ComponentDefinition, error) {
	compDefList := &appsv1alpha1.ComponentDefinitionList{}
	if err := transCtx.Client.List(transCtx.Context, compDefList); err != nil {
		return nil, err
	}

	compDefs := make([]*appsv1alpha1.ComponentDefinition, 0)
	for i, item := range compDefList.Items {
		if !strings.HasPrefix(item.Name, compDef) {
			continue
		}
		compatible, err := t.compatibleWithServiceVersion(transCtx.Context, transCtx.Client, &item, serviceVersion)
		if err != nil {
			return nil, err
		}
		if compatible {
			compDefs = append(compDefs, &compDefList.Items[i])
		}
	}
	if len(compDefs) == 0 {
		return nil, fmt.Errorf("no matched component definition found: %s", compDef)
	}

	slices.SortFunc(compDefs, func(a, b *appsv1alpha1.ComponentDefinition) bool {
		return a.Name < b.Name
	})

	return compDefs[len(compDefs)-1], nil
}

// compatibleWithServiceVersion checks whether the @compDef is compatible with the @serviceVersion
func (t *ClusterAPINormalizationTransformer) compatibleWithServiceVersion(ctx context.Context, cli client.Reader,
	compDef *appsv1alpha1.ComponentDefinition, serviceVersion string) (bool, error) {
	compVersions, err := compatibleCompVersions(ctx, cli, compDef)
	if err != nil {
		return false, err
	}
	for _, compVersion := range compVersions {
		for _, version := range t.compatibleServiceVersions(compVersion, compDef) {
			if compareServiceVersion(serviceVersion, version) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (t *ClusterAPINormalizationTransformer) compatibleServiceVersions(compVersion *appsv1alpha1.ComponentVersion,
	compDef *appsv1alpha1.ComponentDefinition) []string {
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
	serviceVersions := make(map[string]any, 0)
	for _, release := range compVersion.Spec.Releases {
		if releases[release.Name] {
			serviceVersions[release.ServiceVersion] = nil
		}
	}
	return maps.Keys(serviceVersions)
}
