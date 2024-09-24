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
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// clusterAPINormalizationTransformer handles cluster and component API conversion.
type clusterAPINormalizationTransformer struct{}

var _ graph.Transformer = &clusterAPINormalizationTransformer{}

func (t *clusterAPINormalizationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.Cluster
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}

	var err error
	defer func() {
		setProvisioningStartedCondition(&cluster.Status.Conditions, cluster.Name, cluster.Generation, err)
	}()

	// build all component specs
	transCtx.ComponentSpecs, err = t.buildCompSpecs(transCtx, cluster)
	if err != nil {
		return err
	}

	// resolve all component definitions referenced
	if err = t.resolveCompDefinitions(transCtx); err != nil {
		return err
	}

	// update the resolved component definitions and service versions to cluster spec.
	t.updateCompSpecs(transCtx)

	return nil
}

func (t *clusterAPINormalizationTransformer) buildCompSpecs(transCtx *clusterTransformContext,
	cluster *appsv1.Cluster) ([]*appsv1.ClusterComponentSpec, error) {
	if withClusterTopology(cluster) {
		return t.buildCompSpecs4Topology(transCtx.ClusterDef, cluster)
	}
	if withClusterUserDefined(cluster) {
		return t.buildCompSpecs4Specified(cluster), nil
	}
	return nil, nil
}

func (t *clusterAPINormalizationTransformer) buildCompSpecs4Topology(clusterDef *appsv1.ClusterDefinition,
	cluster *appsv1.Cluster) ([]*appsv1.ClusterComponentSpec, error) {
	newCompSpec := func(comp appsv1.ClusterTopologyComponent) *appsv1.ClusterComponentSpec {
		return &appsv1.ClusterComponentSpec{
			Name:         comp.Name,
			ComponentDef: comp.CompDef,
		}
	}

	mergeCompSpec := func(comp appsv1.ClusterTopologyComponent, compSpec *appsv1.ClusterComponentSpec) *appsv1.ClusterComponentSpec {
		if len(compSpec.ComponentDef) == 0 {
			compSpec.ComponentDef = comp.CompDef
		}
		return compSpec
	}

	clusterTopology := referredClusterTopology(clusterDef, cluster.Spec.Topology)
	if clusterTopology == nil {
		return nil, fmt.Errorf("referred cluster topology not found : %s", cluster.Spec.Topology)
	}

	specifiedCompSpecs := make(map[string]*appsv1.ClusterComponentSpec)
	for i, compSpec := range cluster.Spec.ComponentSpecs {
		specifiedCompSpecs[compSpec.Name] = cluster.Spec.ComponentSpecs[i].DeepCopy()
	}

	compSpecs := make([]*appsv1.ClusterComponentSpec, 0)
	for i := range clusterTopology.Components {
		comp := clusterTopology.Components[i]
		if _, ok := specifiedCompSpecs[comp.Name]; ok {
			compSpecs = append(compSpecs, mergeCompSpec(comp, specifiedCompSpecs[comp.Name]))
		} else {
			compSpecs = append(compSpecs, newCompSpec(comp))
		}
	}
	return compSpecs, nil
}

func (t *clusterAPINormalizationTransformer) buildCompSpecs4Specified(cluster *appsv1.Cluster) []*appsv1.ClusterComponentSpec {
	compSpecs := make([]*appsv1.ClusterComponentSpec, 0)
	for i := range cluster.Spec.ComponentSpecs {
		compSpecs = append(compSpecs, cluster.Spec.ComponentSpecs[i].DeepCopy())
	}
	return compSpecs
}

func (t *clusterAPINormalizationTransformer) resolveCompDefinitions(transCtx *clusterTransformContext) error {
	if transCtx.ComponentDefs == nil {
		transCtx.ComponentDefs = make(map[string]*appsv1.ComponentDefinition)
	}
	for i, compSpec := range transCtx.ComponentSpecs {
		compDef, serviceVersion, err := compSpecResolveCompDefinitionNServiceVersion(transCtx.Context, transCtx.Client, transCtx.Cluster, compSpec)
		if err != nil {
			return err
		}
		transCtx.ComponentDefs[compDef.Name] = compDef
		// set the componentDef and serviceVersion as resolved
		transCtx.ComponentSpecs[i].ComponentDef = compDef.Name
		transCtx.ComponentSpecs[i].ServiceVersion = serviceVersion
	}
	return nil
}

func (t *clusterAPINormalizationTransformer) updateCompSpecs(transCtx *clusterTransformContext) {
	if withClusterTopology(transCtx.Cluster) {
		t.updateCompSpecs4Topology(transCtx)
	}
	if withClusterUserDefined(transCtx.Cluster) {
		t.updateCompSpecs4Specified(transCtx)
	}
}

func (t *clusterAPINormalizationTransformer) updateCompSpecs4Topology(transCtx *clusterTransformContext) {
	var (
		cluster = transCtx.Cluster
	)
	compSpecs := make([]appsv1.ClusterComponentSpec, 0)
	for i := range transCtx.ComponentSpecs {
		compSpecs = append(compSpecs, appsv1.ClusterComponentSpec{
			Name:           transCtx.ComponentSpecs[i].Name,
			ComponentDef:   transCtx.ComponentSpecs[i].ComponentDef,
			ServiceVersion: transCtx.ComponentSpecs[i].ServiceVersion,
		})
	}
	for i, compSpec := range cluster.Spec.ComponentSpecs {
		for j := range compSpecs {
			if compSpec.Name == compSpecs[j].Name {
				compSpecs[j] = cluster.Spec.ComponentSpecs[i]
				compSpecs[j].ComponentDef = transCtx.ComponentSpecs[j].ComponentDef
				compSpecs[j].ServiceVersion = transCtx.ComponentSpecs[j].ServiceVersion
				break
			}
		}
	}
	cluster.Spec.ComponentSpecs = compSpecs
}

func (t *clusterAPINormalizationTransformer) updateCompSpecs4Specified(transCtx *clusterTransformContext) {
	var (
		resolvedCompSpecs = transCtx.ComponentSpecs
		cluster           = transCtx.Cluster
	)
	for i := range cluster.Spec.ComponentSpecs {
		cluster.Spec.ComponentSpecs[i].ComponentDef = resolvedCompSpecs[i].ComponentDef
		cluster.Spec.ComponentSpecs[i].ServiceVersion = resolvedCompSpecs[i].ServiceVersion
	}
}

func compSpecResolveCompDefinitionNServiceVersion(ctx context.Context, cli client.Reader, cluster *appsv1.Cluster,
	compSpec *appsv1.ClusterComponentSpec) (*appsv1.ComponentDefinition, string, error) {
	comp := &appsv1.Component{}
	err := cli.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: component.FullName(cluster.Name, compSpec.Name)}, comp)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, "", err
	}

	if apierrors.IsNotFound(err) || checkCompUpgrade(compSpec, comp) {
		return resolveCompDefinitionNServiceVersion(ctx, cli, compSpec.ComponentDef, compSpec.ServiceVersion)
	}
	return resolveCompDefinitionNServiceVersion(ctx, cli, comp.Spec.CompDef, comp.Spec.ServiceVersion)
}

func checkCompUpgrade(compSpec *appsv1.ClusterComponentSpec, comp *appsv1.Component) bool {
	return compSpec.ServiceVersion != comp.Spec.ServiceVersion || compSpec.ComponentDef != comp.Spec.CompDef
}
