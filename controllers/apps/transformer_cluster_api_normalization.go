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
	"maps"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

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
	transCtx.Labels = t.buildCompLabelsInheritedFromCluster(transCtx, cluster)
	transCtx.Annotations = t.buildCompAnnotationsInheritedFromCluster(transCtx, cluster)

	// resolve all component definitions referenced
	if err = t.resolveCompDefinitions(transCtx); err != nil {
		return err
	}

	// update the resolved component definitions and service versions to cluster spec.
	t.updateCompSpecs(transCtx)

	return nil
}

func (t *ClusterAPINormalizationTransformer) buildCompSpecs(transCtx *clusterTransformContext,
	cluster *appsv1alpha1.Cluster) ([]*appsv1alpha1.ClusterComponentSpec, error) {
	if withClusterTopology(cluster) {
		return t.buildCompSpecs4Topology(transCtx.ClusterDef, cluster)
	}
	if withClusterUserDefined(cluster) || withClusterLegacyDefinition(cluster) {
		return t.buildCompSpecs4Specified(transCtx, cluster)
	}
	if withClusterSimplifiedAPI(cluster) {
		return t.buildCompSpecs4SimplifiedAPI(transCtx.ClusterDef, cluster), nil
	}
	return nil, nil
}

func (t *ClusterAPINormalizationTransformer) buildCompSpecs4Topology(clusterDef *appsv1alpha1.ClusterDefinition,
	cluster *appsv1alpha1.Cluster) ([]*appsv1alpha1.ClusterComponentSpec, error) {
	newCompSpec := func(comp appsv1alpha1.ClusterTopologyComponent) *appsv1alpha1.ClusterComponentSpec {
		return &appsv1alpha1.ClusterComponentSpec{
			Name:         comp.Name,
			ComponentDef: comp.CompDef,
		}
	}

	mergeCompSpec := func(comp appsv1alpha1.ClusterTopologyComponent, compSpec *appsv1alpha1.ClusterComponentSpec) *appsv1alpha1.ClusterComponentSpec {
		if len(compSpec.ComponentDef) == 0 {
			compSpec.ComponentDef = comp.CompDef
		}
		return compSpec
	}

	clusterTopology := referredClusterTopology(clusterDef, cluster.Spec.Topology)
	if clusterTopology == nil {
		return nil, fmt.Errorf("referred cluster topology not found : %s", cluster.Spec.Topology)
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
	return compSpecs, nil
}

func (t *ClusterAPINormalizationTransformer) buildCompSpecs4Specified(transCtx *clusterTransformContext,
	cluster *appsv1alpha1.Cluster) ([]*appsv1alpha1.ClusterComponentSpec, error) {
	compSpecs := make([]*appsv1alpha1.ClusterComponentSpec, 0)
	for i := range cluster.Spec.ComponentSpecs {
		compSpecs = append(compSpecs, cluster.Spec.ComponentSpecs[i].DeepCopy())
	}
	if cluster.Spec.ShardingSpecs != nil {
		shardingCompSpecs, err := t.buildCompSpecs4Sharding(transCtx, cluster)
		if err != nil {
			return nil, err
		}
		compSpecs = append(compSpecs, shardingCompSpecs...)
	}
	return compSpecs, nil
}

func (t *ClusterAPINormalizationTransformer) buildCompSpecs4Sharding(transCtx *clusterTransformContext,
	cluster *appsv1alpha1.Cluster) ([]*appsv1alpha1.ClusterComponentSpec, error) {
	compSpecs := make([]*appsv1alpha1.ClusterComponentSpec, 0)
	if transCtx.ShardingComponentSpecs == nil {
		transCtx.ShardingComponentSpecs = make(map[string][]*appsv1alpha1.ClusterComponentSpec, 0)
	}
	for i, sharding := range cluster.Spec.ShardingSpecs {
		shardingComps, err := controllerutil.GenShardingCompSpecList(transCtx.Context, transCtx.Client, cluster, &cluster.Spec.ShardingSpecs[i])
		if err != nil {
			return nil, err
		}
		compSpecs = append(compSpecs, shardingComps...)
		transCtx.ShardingComponentSpecs[sharding.Name] = shardingComps
	}
	return compSpecs, nil
}

func (t *ClusterAPINormalizationTransformer) buildCompSpecs4SimplifiedAPI(clusterDef *appsv1alpha1.ClusterDefinition,
	cluster *appsv1alpha1.Cluster) []*appsv1alpha1.ClusterComponentSpec {
	return []*appsv1alpha1.ClusterComponentSpec{apiconversion.HandleSimplifiedClusterAPI(clusterDef, cluster)}
}

func (t *ClusterAPINormalizationTransformer) buildCompLabelsInheritedFromCluster(transCtx *clusterTransformContext,
	cluster *appsv1alpha1.Cluster) map[string]map[string]string {
	clusterLabels := filterReservedLabels(cluster.Labels)
	labels := make(map[string]map[string]string)
	for _, compSpec := range transCtx.ComponentSpecs {
		labels[compSpec.Name] = maps.Clone(clusterLabels)
	}
	for name, shardingCompSpecs := range transCtx.ShardingComponentSpecs {
		for _, compSpec := range shardingCompSpecs {
			labels[compSpec.Name] = controllerutil.MergeMetadataMaps(clusterLabels, constant.GetShardingNameLabel(name))
		}
	}
	return labels
}

func (t *ClusterAPINormalizationTransformer) buildCompAnnotationsInheritedFromCluster(transCtx *clusterTransformContext,
	cluster *appsv1alpha1.Cluster) map[string]map[string]string {
	clusterAnnotations := filterReservedAnnotations(cluster.Annotations)
	annotations := make(map[string]map[string]string)
	for _, compSpec := range transCtx.ComponentSpecs {
		annotations[compSpec.Name] = maps.Clone(clusterAnnotations)
	}
	return annotations
}

func (t *ClusterAPINormalizationTransformer) resolveCompDefinitions(transCtx *clusterTransformContext) error {
	if transCtx.ComponentDefs == nil {
		transCtx.ComponentDefs = make(map[string]*appsv1alpha1.ComponentDefinition)
	}
	for i, compSpec := range transCtx.ComponentSpecs {
		compDef, serviceVersion, err := t.resolveCompDefinitionNServiceVersion(transCtx, compSpec)
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

func (t *ClusterAPINormalizationTransformer) resolveCompDefinitionNServiceVersion(transCtx *clusterTransformContext,
	compSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.ComponentDefinition, string, error) {
	if withClusterLegacyDefinition(transCtx.Cluster) || withClusterSimplifiedAPI(transCtx.Cluster) {
		compDef, err := t.buildCompDefinition4Legacy(transCtx, compSpec)
		// compDef.Name = ""
		return compDef, "", err
	}
	return t.resolveCompDefinitionNServiceVersionWithUpgrade(transCtx, compSpec)
}

func (t *ClusterAPINormalizationTransformer) buildCompDefinition4Legacy(transCtx *clusterTransformContext,
	compSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.ComponentDefinition, error) {
	compDef, err := component.BuildComponentDefinition(transCtx.ClusterDef, compSpec)
	if err != nil {
		return nil, err
	}
	compDef.Name = constant.GenerateVirtualComponentDefinition(compSpec.ComponentDefRef)
	return compDef, nil
}

func (t *ClusterAPINormalizationTransformer) resolveCompDefinitionNServiceVersionWithUpgrade(transCtx *clusterTransformContext,
	compSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.ComponentDefinition, string, error) {
	var (
		ctx     = transCtx.Context
		cli     = transCtx.Client
		cluster = transCtx.Cluster
	)
	comp := &appsv1alpha1.Component{}
	err := cli.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: component.FullName(cluster.Name, compSpec.Name)}, comp)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, "", err
	}

	if apierrors.IsNotFound(err) || t.checkCompUpgrade(compSpec, comp) {
		return resolveCompDefinitionNServiceVersion(ctx, cli, compSpec.ComponentDef, compSpec.ServiceVersion)
	}
	return resolveCompDefinitionNServiceVersion(ctx, cli, comp.Spec.CompDef, comp.Spec.ServiceVersion)
}

func (t *ClusterAPINormalizationTransformer) checkCompUpgrade(compSpec *appsv1alpha1.ClusterComponentSpec, comp *appsv1alpha1.Component) bool {
	return compSpec.ServiceVersion != comp.Spec.ServiceVersion || compSpec.ComponentDef != comp.Spec.CompDef
}

func (t *ClusterAPINormalizationTransformer) updateCompSpecs(transCtx *clusterTransformContext) {
	var (
		cluster = transCtx.Cluster
	)
	if withClusterLegacyDefinition(cluster) || withClusterSimplifiedAPI(cluster) {
		return
	}
	if withClusterTopology(cluster) {
		t.updateCompSpecs4Topology(transCtx)
	}
	if withClusterUserDefined(cluster) {
		t.updateCompSpecs4Specified(transCtx)
	}
}

func (t *ClusterAPINormalizationTransformer) updateCompSpecs4Topology(transCtx *clusterTransformContext) {
	var (
		cluster = transCtx.Cluster
	)
	compSpecs := make([]appsv1alpha1.ClusterComponentSpec, 0)
	for i := range transCtx.ComponentSpecs {
		compSpecs = append(compSpecs, appsv1alpha1.ClusterComponentSpec{
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

func (t *ClusterAPINormalizationTransformer) updateCompSpecs4Specified(transCtx *clusterTransformContext) {
	var (
		resolvedCompSpecs = transCtx.ComponentSpecs
		idx               = 0
		cluster           = transCtx.Cluster
	)
	for i := range cluster.Spec.ComponentSpecs {
		cluster.Spec.ComponentSpecs[i].ComponentDef = resolvedCompSpecs[i].ComponentDef
		cluster.Spec.ComponentSpecs[i].ServiceVersion = resolvedCompSpecs[i].ServiceVersion
	}
	idx += len(cluster.Spec.ComponentSpecs)

	for i, sharding := range cluster.Spec.ShardingSpecs {
		cluster.Spec.ShardingSpecs[i].Template.ComponentDef = resolvedCompSpecs[idx].ComponentDef
		cluster.Spec.ShardingSpecs[i].Template.ServiceVersion = resolvedCompSpecs[idx].ServiceVersion
		idx += int(sharding.Shards)
	}
}

// filterReservedEntries filters out reserved keys from a map based on a provided set of reserved keys
func filterReservedEntries(entries map[string]string, reservedKeys []string) map[string]string {
	reservedSet := make(map[string]struct{}, len(reservedKeys))
	for _, key := range reservedKeys {
		reservedSet[key] = struct{}{}
	}

	filteredEntries := make(map[string]string)
	for key, value := range entries {
		if _, exists := reservedSet[key]; !exists {
			filteredEntries[key] = value
		}
	}
	return filteredEntries
}

func filterReservedLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}
	return filterReservedEntries(labels, constant.GetKBReservedLabelKeys())
}

func filterReservedAnnotations(annotations map[string]string) map[string]string {
	if annotations == nil {
		return nil
	}
	return filterReservedEntries(annotations, constant.GetKBReservedAnnotationKeys())
}
