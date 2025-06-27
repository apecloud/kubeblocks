/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package cluster

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"golang.org/x/exp/maps"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// clusterNormalizationTransformer handles the cluster API conversion.
type clusterNormalizationTransformer struct{}

var _ graph.Transformer = &clusterNormalizationTransformer{}

func (t *clusterNormalizationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.Cluster
	if transCtx.OrigCluster.IsDeleting() {
		return nil
	}

	var err error
	defer func() {
		setProvisioningStartedCondition(&cluster.Status.Conditions, cluster.Name, cluster.Generation, err)
	}()

	// resolve all components and shardings from topology or specified
	transCtx.components, transCtx.shardings, err = t.resolveCompsNShardings(transCtx)
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

	if err = t.checkNPatchCRDAPIVersionKey(transCtx); err != nil {
		return err
	}

	// build component specs for shardings after resolving definitions
	transCtx.shardingComps, transCtx.shardingCompsWithTpl, err = t.buildShardingComps(transCtx)
	if err != nil {
		return err
	}

	if err = t.postcheck(transCtx); err != nil {
		return err
	}

	// write-back the resolved definitions and service versions to cluster spec.
	t.writeBackCompNShardingSpecs(transCtx)

	return nil
}

func (t *clusterNormalizationTransformer) resolveCompsNShardings(transCtx *clusterTransformContext) ([]*appsv1.ClusterComponentSpec, []*appsv1.ClusterSharding, error) {
	var (
		cluster = transCtx.Cluster
	)
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
	newComp := func(comp appsv1.ClusterTopologyComponent) *appsv1.ClusterComponentSpec {
		if comp.Template != nil && *comp.Template {
			return nil // don't new component spec for the template component automatically
		}
		return &appsv1.ClusterComponentSpec{
			Name:         comp.Name,
			ComponentDef: comp.CompDef,
		}
	}

	mergeComp := func(comp appsv1.ClusterTopologyComponent, compSpec *appsv1.ClusterComponentSpec) *appsv1.ClusterComponentSpec {
		if len(compSpec.ComponentDef) == 0 {
			compSpec.ComponentDef = comp.CompDef
		}
		return compSpec
	}

	matchedComps := func(comp appsv1.ClusterTopologyComponent) []*appsv1.ClusterComponentSpec {
		specs := make([]*appsv1.ClusterComponentSpec, 0)
		for i, spec := range cluster.Spec.ComponentSpecs {
			if clusterTopologyCompMatched(comp, spec.Name) {
				specs = append(specs, cluster.Spec.ComponentSpecs[i].DeepCopy())
			}
		}
		return specs
	}

	compSpecs := make([]*appsv1.ClusterComponentSpec, 0)
	for i := range topology.Components {
		comp := topology.Components[i]
		specs := matchedComps(comp)
		if len(specs) == 0 {
			spec := newComp(comp)
			if spec != nil {
				specs = append(specs, spec)
			}
		}
		for _, spec := range specs {
			compSpecs = append(compSpecs, mergeComp(comp, spec))
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
			templates, err := t.resolveShardingNCompDefinitions(transCtx, sharding)
			if err != nil {
				return err
			}
			for _, tpl := range templates {
				var (
					shardingDef    = tpl[0].(*appsv1.ShardingDefinition)
					compDef        = tpl[1].(*appsv1.ComponentDefinition)
					serviceVersion = tpl[2].(string)
					idx            = tpl[3].(int)
				)
				if shardingDef != nil {
					transCtx.shardingDefs[shardingDef.Name] = shardingDef
					// set the shardingDef as resolved
					if idx < 0 {
						transCtx.shardings[i].ShardingDef = shardingDef.Name
					} else {
						transCtx.shardings[i].ShardTemplates[idx].ShardingDef = ptr.To(shardingDef.Name)
					}
				}
				transCtx.componentDefs[compDef.Name] = compDef
				// set the componentDef and serviceVersion of template as resolved
				if idx < 0 {
					transCtx.shardings[i].Template.ComponentDef = compDef.Name
					transCtx.shardings[i].Template.ServiceVersion = serviceVersion
				} else {
					transCtx.shardings[i].ShardTemplates[idx].CompDef = ptr.To(compDef.Name)
					transCtx.shardings[i].ShardTemplates[idx].ServiceVersion = ptr.To(serviceVersion)
				}
			}
		}
	}
	return nil
}

func (t *clusterNormalizationTransformer) resolveShardingNCompDefinitions(transCtx *clusterTransformContext, sharding *appsv1.ClusterSharding) ([][]any, error) {
	templates := make(map[string][]any)
	templates[""] = []any{sharding.ShardingDef, &sharding.Template, -1}
	for i, tpl := range sharding.ShardTemplates {
		shardingDefName := sharding.ShardingDef
		if tpl.ShardingDef != nil && len(*tpl.ShardingDef) > 0 {
			shardingDefName = *tpl.ShardingDef
		}
		template := sharding.Template.DeepCopy()
		if tpl.ServiceVersion != nil && len(*tpl.ServiceVersion) > 0 || tpl.CompDef != nil && len(*tpl.CompDef) > 0 {
			template.ComponentDef = ptr.Deref(tpl.CompDef, "")
			template.ServiceVersion = ptr.Deref(tpl.ServiceVersion, "")
		}
		templates[tpl.Name] = []any{shardingDefName, template, i}
	}

	result := make([][]any, 0)
	for name, tpl := range templates {
		comp, err := t.firstShardingComponent(transCtx, sharding.Name, name)
		if err != nil {
			return nil, err
		}

		var (
			shardingDefName = tpl[0].(string)
			spec            = tpl[1].(*appsv1.ClusterComponentSpec)
			idx             = tpl[2].(int)
		)

		var shardingDef *appsv1.ShardingDefinition
		shardingDefName = t.shardingDefinitionName(shardingDefName, comp)
		if len(shardingDefName) > 0 {
			shardingDef, err = resolveShardingDefinition(transCtx.Context, transCtx.Client, shardingDefName)
			if err != nil {
				return nil, err
			}
			if len(spec.ComponentDef) == 0 {
				spec.ComponentDef = shardingDef.Spec.Template.CompDef
			}
		}

		compDef, serviceVersion, err := t.resolveCompDefinitionNServiceVersionWithComp(transCtx, spec, comp)
		if err != nil {
			return nil, err
		}

		result = append(result, []any{shardingDef, compDef, serviceVersion, idx})
	}
	return result, nil
}

func (t *clusterNormalizationTransformer) firstShardingComponent(transCtx *clusterTransformContext, shardingName, shardTemplateName string) (*appsv1.Component, error) {
	var (
		ctx     = transCtx.Context
		cli     = transCtx.Client
		cluster = transCtx.Cluster
	)

	compList := &appsv1.ComponentList{}
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey:       cluster.Name,
		constant.KBAppShardingNameLabelKey: shardingName,
	}
	if len(shardTemplateName) > 0 {
		ml[constant.KBAppShardTemplateLabelKey] = shardTemplateName
	}
	if err := cli.List(ctx, compList, client.InNamespace(cluster.Namespace), ml, client.Limit(1)); err != nil {
		return nil, err
	}
	if len(compList.Items) == 0 {
		return nil, nil
	}
	return &compList.Items[0], nil
}

func (t *clusterNormalizationTransformer) shardingDefinitionName(defaultShardingDefName string, comp *appsv1.Component) string {
	if comp != nil {
		shardingDefName, ok := comp.Labels[constant.ShardingDefLabelKey]
		if ok {
			return shardingDefName
		}
	}
	return defaultShardingDefName
}

func (t *clusterNormalizationTransformer) resolveDefinitions4Components(transCtx *clusterTransformContext) error {
	if transCtx.componentDefs == nil {
		transCtx.componentDefs = make(map[string]*appsv1.ComponentDefinition)
	}
	for i := range transCtx.components {
		compDefs, err := t.resolveDefinitions4Component(transCtx, transCtx.components[i])
		if err != nil {
			return err
		}
		for j := range compDefs {
			transCtx.componentDefs[compDefs[j].Name] = compDefs[j]
		}
	}
	return nil
}

func (t *clusterNormalizationTransformer) resolveDefinitions4Component(transCtx *clusterTransformContext,
	compSpec *appsv1.ClusterComponentSpec) ([]*appsv1.ComponentDefinition, error) {
	var (
		ctx      = transCtx.Context
		cli      = transCtx.Client
		cluster  = transCtx.Cluster
		compDefs = make([]*appsv1.ComponentDefinition, 0)
	)
	comp := &appsv1.Component{}
	err := cli.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: component.FullName(cluster.Name, compSpec.Name)}, comp)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		comp = nil
	}

	compDef, serviceVersion, err := t.resolveCompDefinitionNServiceVersionWithComp(transCtx, compSpec, comp)
	if err != nil {
		return nil, err
	}
	compDefs = append(compDefs, compDef)
	// set the componentDef and serviceVersion as resolved
	compSpec.ComponentDef = compDef.Name
	compSpec.ServiceVersion = serviceVersion

	for i, tpl := range compSpec.Instances {
		if len(tpl.ServiceVersion) == 0 && len(tpl.CompDef) == 0 {
			continue
		}
		compDef, serviceVersion, err = t.resolveCompDefinitionNServiceVersionWithTemplate(transCtx, compSpec, comp, &tpl)
		if err != nil {
			return nil, err
		}
		compDefs = append(compDefs, compDef)
		// set the componentDef and serviceVersion as resolved
		compSpec.Instances[i].CompDef = compDef.Name
		compSpec.Instances[i].ServiceVersion = serviceVersion
	}
	return compDefs, nil
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

func (t *clusterNormalizationTransformer) resolveCompDefinitionNServiceVersionWithTemplate(transCtx *clusterTransformContext,
	compSpec *appsv1.ClusterComponentSpec, comp *appsv1.Component, protoTpl *appsv1.InstanceTemplate) (*appsv1.ComponentDefinition, string, error) {
	var (
		ctx        = transCtx.Context
		cli        = transCtx.Client
		runningTpl *appsv1.InstanceTemplate
	)
	if comp != nil {
		for i, tpl := range comp.Spec.Instances {
			if tpl.Name == protoTpl.Name {
				runningTpl = &comp.Spec.Instances[i]
				break
			}
		}
	}

	serviceVersion := compSpec.ServiceVersion
	if len(protoTpl.ServiceVersion) > 0 {
		serviceVersion = protoTpl.ServiceVersion
	}
	compDefName := compSpec.ComponentDef
	if len(protoTpl.CompDef) > 0 {
		compDefName = protoTpl.CompDef
	}
	if comp == nil || runningTpl == nil || t.checkTemplateUpgrade(serviceVersion, compDefName, runningTpl) {
		return resolveCompDefinitionNServiceVersion(ctx, cli, compDefName, serviceVersion)
	}
	return resolveCompDefinitionNServiceVersion(ctx, cli, runningTpl.CompDef, runningTpl.ServiceVersion)
}

func (t *clusterNormalizationTransformer) checkCompUpgrade(compSpec *appsv1.ClusterComponentSpec, comp *appsv1.Component) bool {
	return compSpec.ServiceVersion != comp.Spec.ServiceVersion || compSpec.ComponentDef != comp.Spec.CompDef
}

func (t *clusterNormalizationTransformer) checkTemplateUpgrade(serviceVersion, compDefName string, runningTpl *appsv1.InstanceTemplate) bool {
	return serviceVersion != runningTpl.ServiceVersion || compDefName != runningTpl.CompDef
}

func (t *clusterNormalizationTransformer) buildShardingComps(transCtx *clusterTransformContext) (map[string][]*appsv1.ClusterComponentSpec, map[string]map[string][]*appsv1.ClusterComponentSpec, error) {
	cluster := transCtx.Cluster
	shardingComps := make(map[string][]*appsv1.ClusterComponentSpec, 0)
	shardingCompsWithTpl := make(map[string]map[string][]*appsv1.ClusterComponentSpec)
	for _, spec := range transCtx.shardings {
		tplComps, err := sharding.BuildShardingCompSpecs(transCtx.Context, transCtx.Client, cluster.Namespace, cluster.Name, spec)
		if err != nil {
			return nil, nil, err
		}
		shardingCompsWithTpl[spec.Name] = tplComps
		for tpl, comps := range tplComps {
			if len(comps) > 0 {
				shardingComps[spec.Name] = append(shardingComps[spec.Name], tplComps[tpl]...)
			}
		}
	}
	return shardingComps, shardingCompsWithTpl, nil
}

func (t *clusterNormalizationTransformer) postcheck(transCtx *clusterTransformContext) error {
	if err := t.validateCompNShardingUnique(transCtx); err != nil {
		return err
	}
	if err := t.validateShardingShards(transCtx); err != nil {
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

func (t *clusterNormalizationTransformer) checkNPatchCRDAPIVersionKey(transCtx *clusterTransformContext) error {
	// get the v1Alpha1Cluster from the annotations
	v1Alpha1Cluster, err := appsv1alpha1.GetV1Alpha1ClusterFromIncrementConverter(transCtx.Cluster)
	if err != nil {
		return err
	}
	getCRDAPIVersion := func() (string, error) {
		apiVersion := transCtx.Cluster.Annotations[constant.CRDAPIVersionAnnotationKey]
		if len(apiVersion) > 0 {
			return apiVersion, nil
		}
		if v1Alpha1Cluster != nil && len(v1Alpha1Cluster.Spec.ClusterDefRef) > 0 {
			return appsv1alpha1.GroupVersion.String(), nil
		}

		// get the CRD API version from the annotations of the clusterDef or componentDefs
		apiVersions := map[string][]string{}
		from := func(name string, annotations map[string]string) {
			key := annotations[constant.CRDAPIVersionAnnotationKey]
			apiVersions[key] = append(apiVersions[key], name)
		}

		if transCtx.clusterDef != nil {
			from(transCtx.clusterDef.Name, transCtx.clusterDef.Annotations)
		} else {
			for _, compDef := range transCtx.componentDefs {
				from(compDef.Name, compDef.Annotations)
			}
			for _, shardingDef := range transCtx.shardingDefs {
				from(shardingDef.Name, shardingDef.Annotations)
			}
		}
		switch {
		case len(apiVersions) > 1:
			return "", fmt.Errorf("multiple CRD API versions found: %v", apiVersions)
		case len(apiVersions) == 1:
			return maps.Keys(apiVersions)[0], nil
		default:
			return "", nil
		}
	}

	apiVersion, err := getCRDAPIVersion()
	if err != nil {
		return err
	}
	if transCtx.Cluster.Annotations == nil {
		transCtx.Cluster.Annotations = make(map[string]string)
	}
	transCtx.Cluster.Annotations[constant.CRDAPIVersionAnnotationKey] = apiVersion
	if controllerutil.IsAPIVersionSupported(apiVersion) {
		return nil
	}
	if v1Alpha1Cluster != nil && len(v1Alpha1Cluster.Spec.ClusterVersionRef) > 0 {
		// revert the topology to empty
		transCtx.Cluster.Spec.Topology = ""
	}
	return graph.ErrPrematureStop // un-supported CRD API version, stop the transformation
}

// referredClusterTopology returns the cluster topology which has name @name.
func referredClusterTopology(clusterDef *appsv1.ClusterDefinition, name string) *appsv1.ClusterTopology {
	if clusterDef != nil {
		if len(name) == 0 {
			return defaultClusterTopology(clusterDef)
		}
		for i, topology := range clusterDef.Spec.Topologies {
			if topology.Name == name {
				return &clusterDef.Spec.Topologies[i]
			}
		}
	}
	return nil
}

// defaultClusterTopology returns the default cluster topology in specified cluster definition.
func defaultClusterTopology(clusterDef *appsv1.ClusterDefinition) *appsv1.ClusterTopology {
	for i, topology := range clusterDef.Spec.Topologies {
		if topology.Default {
			return &clusterDef.Spec.Topologies[i]
		}
	}
	return nil
}

func clusterTopologyCompMatched(comp appsv1.ClusterTopologyComponent, compName string) bool {
	if comp.Name == compName {
		return true
	}
	if comp.Template != nil && *comp.Template {
		return strings.HasPrefix(compName, comp.Name)
	}
	return false
}

// resolveShardingDefinition resolves and returns the specific sharding definition object supported.
func resolveShardingDefinition(ctx context.Context, cli client.Reader, shardingDefName string) (*appsv1.ShardingDefinition, error) {
	shardingDefs, err := listShardingDefinitionsWithPattern(ctx, cli, shardingDefName)
	if err != nil {
		return nil, err
	}
	if len(shardingDefs) == 0 {
		return nil, fmt.Errorf("no sharding definition found for the specified name: %s", shardingDefName)
	}

	m := make(map[string]int)
	for i, def := range shardingDefs {
		m[def.Name] = i
	}
	// choose the latest one
	names := maps.Keys(m)
	slices.Sort(names)
	latestName := names[len(names)-1]

	return shardingDefs[m[latestName]], nil
}

// listShardingDefinitionsWithPattern returns all sharding definitions whose names match the given pattern
func listShardingDefinitionsWithPattern(ctx context.Context, cli client.Reader, name string) ([]*appsv1.ShardingDefinition, error) {
	shardingDefList := &appsv1.ShardingDefinitionList{}
	if err := cli.List(ctx, shardingDefList); err != nil {
		return nil, err
	}
	fullyMatched := make([]*appsv1.ShardingDefinition, 0)
	patternMatched := make([]*appsv1.ShardingDefinition, 0)
	for i, item := range shardingDefList.Items {
		if item.Name == name {
			fullyMatched = append(fullyMatched, &shardingDefList.Items[i])
		}
		if component.PrefixOrRegexMatched(item.Name, name) {
			patternMatched = append(patternMatched, &shardingDefList.Items[i])
		}
	}
	if len(fullyMatched) > 0 {
		return fullyMatched, nil
	}
	return patternMatched, nil
}

func validateShardingShards(shardingDef *appsv1.ShardingDefinition, sharding *appsv1.ClusterSharding) error {
	var (
		limit  = shardingDef.Spec.ShardsLimit
		shards = sharding.Shards
	)
	if limit == nil || (shards >= limit.MinShards && shards <= limit.MaxShards) {
		return nil
	}
	return shardsOutOfLimitError(sharding.Name, shards, *limit)
}

func shardsOutOfLimitError(shardingName string, shards int32, limit appsv1.ShardsLimit) error {
	return fmt.Errorf("shards %d out-of-limit [%d, %d], sharding: %s", shards, limit.MinShards, limit.MaxShards, shardingName)
}

// resolveCompDefinitionNServiceVersion resolves and returns the specific component definition object and the service version supported.
func resolveCompDefinitionNServiceVersion(ctx context.Context, cli client.Reader, compDefName, serviceVersion string) (*appsv1.ComponentDefinition, string, error) {
	var (
		compDef *appsv1.ComponentDefinition
	)
	compDefs, err := listCompDefinitionsWithPattern(ctx, cli, compDefName)
	if err != nil {
		return compDef, serviceVersion, err
	}

	// mapping from <service version> to <[]*appsv1.ComponentDefinition>
	serviceVersionToCompDefs, err := serviceVersionToCompDefinitions(ctx, cli, compDefs, serviceVersion)
	if err != nil {
		return compDef, serviceVersion, err
	}

	// use specified service version or the latest.
	if len(serviceVersion) == 0 {
		serviceVersions := maps.Keys(serviceVersionToCompDefs)
		if len(serviceVersions) > 0 {
			slices.SortFunc(serviceVersions, serviceVersionComparator)
			serviceVersion = serviceVersions[len(serviceVersions)-1]
		}
	}

	// component definitions that support the service version
	compatibleCompDefs := serviceVersionToCompDefs[serviceVersion]
	if len(compatibleCompDefs) == 0 {
		return compDef, serviceVersion, fmt.Errorf(`no matched component definition found with componentDef "%s" and serviceVersion "%s"`, compDefName, serviceVersion)
	}

	// choose the latest one
	compatibleCompDefNames := maps.Keys(compatibleCompDefs)
	slices.Sort(compatibleCompDefNames)
	compatibleCompDefName := compatibleCompDefNames[len(compatibleCompDefNames)-1]

	return compatibleCompDefs[compatibleCompDefName], serviceVersion, nil
}

// listCompDefinitionsWithPattern returns all component definitions whose names match the given pattern
func listCompDefinitionsWithPattern(ctx context.Context, cli client.Reader, name string) ([]*appsv1.ComponentDefinition, error) {
	compDefList := &appsv1.ComponentDefinitionList{}
	if err := cli.List(ctx, compDefList); err != nil {
		return nil, err
	}
	compDefsFullyMatched := make([]*appsv1.ComponentDefinition, 0)
	compDefsPatternMatched := make([]*appsv1.ComponentDefinition, 0)
	for i, item := range compDefList.Items {
		if item.Name == name {
			compDefsFullyMatched = append(compDefsFullyMatched, &compDefList.Items[i])
		}
		if component.PrefixOrRegexMatched(item.Name, name) {
			compDefsPatternMatched = append(compDefsPatternMatched, &compDefList.Items[i])
		}
	}
	if len(compDefsFullyMatched) > 0 {
		return compDefsFullyMatched, nil
	}
	return compDefsPatternMatched, nil
}

func serviceVersionToCompDefinitions(ctx context.Context, cli client.Reader,
	compDefs []*appsv1.ComponentDefinition, serviceVersion string) (map[string]map[string]*appsv1.ComponentDefinition, error) {
	result := make(map[string]map[string]*appsv1.ComponentDefinition)

	insert := func(version string, compDef *appsv1.ComponentDefinition) {
		if _, ok := result[version]; !ok {
			result[version] = make(map[string]*appsv1.ComponentDefinition)
		}
		result[version][compDef.Name] = compDef
	}

	checkedInsert := func(version string, compDef *appsv1.ComponentDefinition) error {
		match, err := component.CompareServiceVersion(serviceVersion, version)
		if err == nil && match {
			insert(version, compDef)
		}
		return err
	}

	for _, compDef := range compDefs {
		compVersions, err := component.CompatibleCompVersions4Definition(ctx, cli, compDef)
		if err != nil {
			return nil, err
		}

		serviceVersions := sets.New[string]()
		// add definition's service version as default, in case there is no component versions provided
		if compDef.Spec.ServiceVersion != "" {
			serviceVersions.Insert(compDef.Spec.ServiceVersion)
		}
		for _, compVersion := range compVersions {
			serviceVersions = serviceVersions.Union(compatibleServiceVersions4Definition(compDef, compVersion))
		}

		for version := range serviceVersions {
			if err = checkedInsert(version, compDef); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

// compatibleServiceVersions4Definition returns all service versions that are compatible with specified component definition.
func compatibleServiceVersions4Definition(compDef *appsv1.ComponentDefinition, compVersion *appsv1.ComponentVersion) sets.Set[string] {
	match := func(pattern string) bool {
		return component.PrefixOrRegexMatched(compDef.Name, pattern)
	}
	releases := make(map[string]bool, 0)
	for _, rule := range compVersion.Spec.CompatibilityRules {
		if slices.IndexFunc(rule.CompDefs, match) >= 0 {
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

func serviceVersionComparator(a, b string) int {
	if len(a) == 0 {
		return -1
	}
	if len(b) == 0 {
		return 1
	}
	v, err1 := version.ParseSemantic(a)
	if err1 != nil {
		panic(fmt.Sprintf("runtime error - invalid service version in comparator: %s", err1.Error()))
	}
	ret, err2 := v.Compare(b)
	if err2 != nil {
		panic(fmt.Sprintf("runtime error - invalid service version in comparator: %s", err2.Error()))
	}
	return ret
}
