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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// clusterNormalizationTransformer handles the cluster API conversion.
type clusterNormalizationTransformer struct{}

var _ graph.Transformer = &clusterNormalizationTransformer{}

func (t *clusterNormalizationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.Cluster
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}

	var err error
	defer func() {
		setProvisioningStartedCondition(&cluster.Status.Conditions, cluster.Name, cluster.Generation, err)
	}()

	// resolve all components and shardings from topology or specified
	transCtx.components, transCtx.shardings, err = t.resolveCompsNShardings(transCtx, cluster)
	if err != nil {
		return err
	}

	// resolve sharding and component definitions referenced for shardings
	if err = t.resolveDefinitions4Shardings(transCtx); err != nil {
		return err
	}

	// resolve component definitions referenced for components
	if err = t.resolveDefinitions4Components(transCtx); err != nil {
		return err
	}

	// resolve sidecars for all components and shardings
	transCtx.sidecars, err = t.resolveSidecars(transCtx)
	if err != nil {
		return err
	}

	// write-back the resolved definitions and service versions to cluster spec.
	t.writeBackCompNShardingSpecs(transCtx)

	// build and validate all component specs for components and shardings
	if err = t.validateNBuildAllCompSpecs(transCtx, cluster); err != nil {
		return err
	}

	return nil
}

func (t *clusterNormalizationTransformer) resolveCompsNShardings(transCtx *clusterTransformContext,
	cluster *appsv1.Cluster) ([]*appsv1.ClusterComponentSpec, []*appsv1.ClusterSharding, error) {
	if withClusterTopology(cluster) {
		return t.resolveCompsNShardingsFromTopology(transCtx.clusterDef, cluster)
	}
	if withClusterUserDefined(cluster) {
		return t.resolveCompsNShardingsFromSpecified(transCtx, cluster)
	}
	return nil, nil, nil
}

func (t *clusterNormalizationTransformer) resolveCompsNShardingsFromTopology(clusterDef *appsv1.ClusterDefinition,
	cluster *appsv1.Cluster) ([]*appsv1.ClusterComponentSpec, []*appsv1.ClusterSharding, error) {
	topology := referredClusterTopology(clusterDef, cluster.Spec.Topology)
	if topology == nil {
		return nil, nil, fmt.Errorf("referred cluster topology not found : %s", cluster.Spec.Topology)
	}

	comps, err := t.resolveCompsFromTopology(*topology, cluster)
	if err != nil {
		return nil, nil, err
	}

	shardings, err := t.resolveShardingsFromTopology(*topology, cluster)
	if err != nil {
		return nil, nil, err
	}
	return comps, shardings, nil
}

func (t *clusterNormalizationTransformer) resolveCompsFromTopology(topology appsv1.ClusterTopology,
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

	specifiedCompSpecs := make(map[string]*appsv1.ClusterComponentSpec)
	for i, compSpec := range cluster.Spec.ComponentSpecs {
		specifiedCompSpecs[compSpec.Name] = cluster.Spec.ComponentSpecs[i].DeepCopy()
	}

	compSpecs := make([]*appsv1.ClusterComponentSpec, 0)
	for i := range topology.Components {
		comp := topology.Components[i]
		if _, ok := specifiedCompSpecs[comp.Name]; ok {
			compSpecs = append(compSpecs, mergeCompSpec(comp, specifiedCompSpecs[comp.Name]))
		} else {
			compSpecs = append(compSpecs, newCompSpec(comp))
		}
	}
	return compSpecs, nil
}

func (t *clusterNormalizationTransformer) resolveShardingsFromTopology(topology appsv1.ClusterTopology,
	cluster *appsv1.Cluster) ([]*appsv1.ClusterSharding, error) {
	newSharding := func(sharding appsv1.ClusterTopologySharding) *appsv1.ClusterSharding {
		return &appsv1.ClusterSharding{
			Name:        sharding.Name,
			ShardingDef: sharding.ShardingDef,
		}
	}

	mergeSharding := func(sharding appsv1.ClusterTopologySharding, spec *appsv1.ClusterSharding) *appsv1.ClusterSharding {
		if len(spec.ShardingDef) == 0 {
			spec.ShardingDef = sharding.ShardingDef
		}
		return spec
	}

	specified := make(map[string]*appsv1.ClusterSharding)
	for i, sharding := range cluster.Spec.Shardings {
		specified[sharding.Name] = cluster.Spec.Shardings[i].DeepCopy()
	}

	shardings := make([]*appsv1.ClusterSharding, 0)
	for i := range topology.Shardings {
		sharding := topology.Shardings[i]
		if _, ok := specified[sharding.Name]; ok {
			shardings = append(shardings, mergeSharding(sharding, specified[sharding.Name]))
		} else {
			shardings = append(shardings, newSharding(sharding))
		}
	}
	return shardings, nil
}

func (t *clusterNormalizationTransformer) resolveCompsNShardingsFromSpecified(transCtx *clusterTransformContext,
	cluster *appsv1.Cluster) ([]*appsv1.ClusterComponentSpec, []*appsv1.ClusterSharding, error) {
	comps := make([]*appsv1.ClusterComponentSpec, 0)
	for i := range cluster.Spec.ComponentSpecs {
		comps = append(comps, cluster.Spec.ComponentSpecs[i].DeepCopy())
	}
	shardings := make([]*appsv1.ClusterSharding, 0)
	for i := range cluster.Spec.Shardings {
		shardings = append(shardings, cluster.Spec.Shardings[i].DeepCopy())
	}
	return comps, shardings, nil
}

func (t *clusterNormalizationTransformer) resolveDefinitions4Shardings(transCtx *clusterTransformContext) error {
	if len(transCtx.shardings) != 0 {
		transCtx.shardingDefs = make(map[string]*appsv1.ShardingDefinition)
		if transCtx.componentDefs == nil {
			transCtx.componentDefs = make(map[string]*appsv1.ComponentDefinition)
		}
		for i, sharding := range transCtx.shardings {
			shardingDef, compDef, serviceVersion, err := t.resolveShardingNCompDefinition(transCtx, sharding)
			if err != nil {
				return err
			}
			if shardingDef != nil {
				transCtx.shardingDefs[shardingDef.Name] = shardingDef
				// set the shardingDef as resolved
				transCtx.shardings[i].ShardingDef = shardingDef.Name
			}
			transCtx.componentDefs[compDef.Name] = compDef
			// set the componentDef and serviceVersion of template as resolved
			transCtx.shardings[i].Template.ComponentDef = compDef.Name
			transCtx.shardings[i].Template.ServiceVersion = serviceVersion
		}
	}
	return nil
}

func (t *clusterNormalizationTransformer) resolveShardingNCompDefinition(transCtx *clusterTransformContext,
	sharding *appsv1.ClusterSharding) (*appsv1.ShardingDefinition, *appsv1.ComponentDefinition, string, error) {
	comp, err := t.firstShardingComponent(transCtx, sharding)
	if err != nil {
		return nil, nil, "", err
	}

	var shardingDef *appsv1.ShardingDefinition
	shardingDefName := t.shardingDefinitionName(sharding, comp)
	if len(shardingDefName) > 0 {
		shardingDef, err = resolveShardingDefinition(transCtx.Context, transCtx.Client, shardingDefName)
		if err != nil {
			return nil, nil, "", err
		}
	}

	spec := sharding.Template
	compDef, serviceVersion, err := t.resolveCompDefinitionNServiceVersionWithComp(transCtx, &spec, comp)
	if err != nil {
		return nil, nil, "", err
	}

	return shardingDef, compDef, serviceVersion, err
}

func (t *clusterNormalizationTransformer) firstShardingComponent(transCtx *clusterTransformContext,
	sharding *appsv1.ClusterSharding) (*appsv1.Component, error) {
	var (
		ctx     = transCtx.Context
		cli     = transCtx.Client
		cluster = transCtx.Cluster
	)

	compList := &appsv1.ComponentList{}
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey:       cluster.Name,
		constant.KBAppShardingNameLabelKey: sharding.Name,
	}
	if err := cli.List(ctx, compList, client.InNamespace(cluster.Namespace), ml, client.Limit(1)); err != nil {
		return nil, err
	}
	if len(compList.Items) == 0 {
		return nil, nil
	}
	return &compList.Items[0], nil
}

func (t *clusterNormalizationTransformer) shardingDefinitionName(sharding *appsv1.ClusterSharding, comp *appsv1.Component) string {
	if comp != nil {
		shardingDefName, ok := comp.Labels[constant.ShardingDefLabelKey]
		if ok {
			return shardingDefName
		}
	}
	return sharding.ShardingDef
}

func (t *clusterNormalizationTransformer) resolveDefinitions4Components(transCtx *clusterTransformContext) error {
	if transCtx.componentDefs == nil {
		transCtx.componentDefs = make(map[string]*appsv1.ComponentDefinition)
	}
	for i, compSpec := range transCtx.components {
		compDef, serviceVersion, err := t.resolveCompDefinitionNServiceVersion(transCtx, compSpec)
		if err != nil {
			return err
		}
		transCtx.componentDefs[compDef.Name] = compDef
		// set the componentDef and serviceVersion as resolved
		transCtx.components[i].ComponentDef = compDef.Name
		transCtx.components[i].ServiceVersion = serviceVersion
	}
	return nil
}

func (t *clusterNormalizationTransformer) resolveCompDefinitionNServiceVersion(transCtx *clusterTransformContext,
	compSpec *appsv1.ClusterComponentSpec) (*appsv1.ComponentDefinition, string, error) {
	var (
		ctx     = transCtx.Context
		cli     = transCtx.Client
		cluster = transCtx.Cluster
	)
	comp := &appsv1.Component{}
	err := cli.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: component.FullName(cluster.Name, compSpec.Name)}, comp)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, "", err
	}

	if apierrors.IsNotFound(err) {
		return t.resolveCompDefinitionNServiceVersionWithComp(transCtx, compSpec, nil)
	}
	return t.resolveCompDefinitionNServiceVersionWithComp(transCtx, compSpec, comp)
}

func (t *clusterNormalizationTransformer) resolveCompDefinitionNServiceVersionWithComp(transCtx *clusterTransformContext,
	compSpec *appsv1.ClusterComponentSpec, comp *appsv1.Component) (*appsv1.ComponentDefinition, string, error) {
	var (
		ctx = transCtx.Context
		cli = transCtx.Client
	)
	if comp == nil || t.checkCompUpgrade(compSpec, comp) {
		return resolveCompDefinitionNServiceVersion(ctx, cli, compSpec.ComponentDef, compSpec.ServiceVersion)
	}
	return resolveCompDefinitionNServiceVersion(ctx, cli, comp.Spec.CompDef, comp.Spec.ServiceVersion)
}

func (t *clusterNormalizationTransformer) checkCompUpgrade(compSpec *appsv1.ClusterComponentSpec, comp *appsv1.Component) bool {
	return compSpec.ServiceVersion != comp.Spec.ServiceVersion || compSpec.ComponentDef != comp.Spec.CompDef
}

func (t *clusterNormalizationTransformer) resolveSidecars(transCtx *clusterTransformContext) (map[string][]*appsv1.SidecarDefinition, error) {
	// component definitions used
	compDefs := sets.New[string]()
	for _, spec := range transCtx.components {
		compDefs.Insert(spec.ComponentDef)
	}
	for _, spec := range transCtx.shardings {
		compDefs.Insert(spec.Template.ComponentDef)
	}
	return matchedSidecarDef4CompDefs(transCtx.Context, transCtx.Client, sets.List(compDefs))
}

func (t *clusterNormalizationTransformer) writeBackCompNShardingSpecs(transCtx *clusterTransformContext) {
	if len(transCtx.components) > 0 {
		comps := make([]appsv1.ClusterComponentSpec, 0)
		for i := range transCtx.components {
			comps = append(comps, *transCtx.components[i])
		}
		transCtx.Cluster.Spec.ComponentSpecs = comps
	}
	if len(transCtx.shardings) > 0 {
		shardings := make([]appsv1.ClusterSharding, 0)
		for i := range transCtx.shardings {
			shardings = append(shardings, *transCtx.shardings[i])
		}
		transCtx.Cluster.Spec.Shardings = shardings
	}
}

func (t *clusterNormalizationTransformer) validateNBuildAllCompSpecs(transCtx *clusterTransformContext, cluster *appsv1.Cluster) error {
	var err error
	if err = t.validateCompNShardingUnique(transCtx); err != nil {
		return err
	}

	if err = t.validateShardingShards(transCtx); err != nil {
		return err
	}

	transCtx.shardingComps, err = t.buildShardingComps(transCtx, cluster)
	if err != nil {
		return err
	}
	return nil
}

func (t *clusterNormalizationTransformer) validateCompNShardingUnique(transCtx *clusterTransformContext) error {
	if len(transCtx.shardings) == 0 || len(transCtx.components) == 0 {
		return nil
	}

	names := sets.New[string]()
	for _, comp := range transCtx.components {
		names.Insert(comp.Name)
	}
	for _, sharding := range transCtx.shardings {
		if names.Has(sharding.Name) {
			return fmt.Errorf(`duplicate name "%s" between spec.compSpecs and spec.shardings`, sharding.Name)
		}
	}
	return nil
}

func (t *clusterNormalizationTransformer) validateShardingShards(transCtx *clusterTransformContext) error {
	for _, sharding := range transCtx.shardings {
		shardingDef, ok := transCtx.shardingDefs[sharding.ShardingDef]
		if ok && shardingDef != nil {
			if err := validateShardingShards(shardingDef, sharding); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *clusterNormalizationTransformer) buildShardingComps(transCtx *clusterTransformContext, cluster *appsv1.Cluster) (map[string][]*appsv1.ClusterComponentSpec, error) {
	shardingComps := make(map[string][]*appsv1.ClusterComponentSpec)
	for _, sharding := range transCtx.shardings {
		comps, err := controllerutil.GenShardingCompSpecList(transCtx.Context, transCtx.Client, cluster, sharding)
		if err != nil {
			return nil, err
		}
		shardingComps[sharding.Name] = comps
	}
	return shardingComps, nil
}
