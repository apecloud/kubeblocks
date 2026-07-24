/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

KubeBlocks is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

KubeBlocks is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with KubeBlocks.  If not, see <http://www.gnu.org/licenses/>.
*/

package cluster

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
)

var errInvalidShardingScaleInTopology = errors.New("invalid sharding scale-in topology")

const shardingScaleInDefaultShardTemplate = "@default"

type shardingScaleInDesiredComponent struct {
	ShardTemplateName string
	Spec              appsv1.ClusterComponentSpec
}

// shardingScaleInTopologyInventory is a source-only candidate snapshot. It
// deliberately contains live objects rather than persisted plan material;
// later builders must close Pod, prerequisite, request, and capability identity
// before any status CAS or external action is allowed.
type shardingScaleInTopologyInventory struct {
	Cluster            *appsv1.Cluster
	Sharding           appsv1.ClusterSharding
	ShardingDefinition *appsv1.ShardingDefinition

	Components        []appsv1.Component
	DesiredComponents []shardingScaleInDesiredComponent
	Leaving           []appsv1.Component
	Staying           []appsv1.Component
}

func loadFreshShardingScaleInTopology(ctx context.Context, apiReader client.Reader,
	clusterKey types.NamespacedName, expectedClusterUID types.UID, shardingName string,
) (*shardingScaleInTopologyInventory, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInTopology, fmt.Sprintf(format, args...))
	}
	if apiReader == nil {
		return nil, invalid("APIReader must not be nil")
	}
	if clusterKey.Namespace == "" || clusterKey.Name == "" || expectedClusterUID == "" || shardingName == "" {
		return nil, invalid("Cluster key, expected UID, and sharding name must be complete")
	}

	cluster := &appsv1.Cluster{}
	if err := apiReader.Get(ctx, clusterKey, cluster); err != nil {
		return nil, err
	}
	if cluster.Namespace != clusterKey.Namespace || cluster.Name != clusterKey.Name ||
		cluster.UID != expectedClusterUID {
		return nil, invalid("fresh Cluster UID or name does not match the requested identity")
	}
	if cluster.Generation <= 0 || cluster.ResourceVersion == "" {
		return nil, invalid("fresh Cluster generation and resourceVersion must be non-empty")
	}
	if !cluster.DeletionTimestamp.IsZero() {
		return nil, invalid("fresh Cluster is deleting")
	}

	shardingSpec, err := exactClusterSharding(cluster, shardingName)
	if err != nil {
		return nil, err
	}
	if shardingSpec.Shards <= 0 || shardingSpec.Shards > 32 {
		return nil, invalid("desired shard count must be between 1 and 32")
	}
	if shardingSpec.ShardingDef == "" {
		return nil, invalid("sharding definition selector must not be empty")
	}

	shardingDef, err := resolveShardingDefinition(ctx, apiReader, shardingSpec.ShardingDef)
	if err != nil {
		return nil, err
	}
	if shardingDef == nil || shardingDef.Name == "" || shardingDef.UID == "" ||
		shardingDef.Generation <= 0 || !shardingDef.DeletionTimestamp.IsZero() {
		return nil, invalid("resolved ShardingDefinition identity must be complete and not deleting")
	}
	if shardingDef.Spec.LifecycleActions == nil ||
		shardingDef.Spec.LifecycleActions.ShardRemove == nil ||
		shardingDef.Spec.LifecycleActions.ShardRemove.ResultProtocol != appsv1.ShardingScaleInResultProtocolV2 {
		return nil, invalid("resolved ShardingDefinition must provide the typed shard-remove action")
	}
	if err := validateShardingShards(shardingDef, shardingSpec); err != nil {
		return nil, err
	}

	before, err := listFreshShardingScaleInComponents(ctx, apiReader, cluster, shardingName)
	if err != nil {
		return nil, err
	}
	if err := validateFreshShardingScaleInComponents(cluster, shardingDef, shardingName, before); err != nil {
		return nil, err
	}

	desiredByTemplate, err := sharding.BuildShardingCompSpecs(
		ctx, apiReader, cluster.Namespace, cluster.Name, shardingSpec)
	if err != nil {
		return nil, err
	}

	after, err := listFreshShardingScaleInComponents(ctx, apiReader, cluster, shardingName)
	if err != nil {
		return nil, err
	}
	if !sameShardingScaleInComponentSnapshot(before, after) {
		return nil, invalid("fresh Component snapshot changed while desired members were rebuilt")
	}

	desiredComponents, err := flattenShardingScaleInDesiredComponents(cluster.Name, desiredByTemplate)
	if err != nil {
		return nil, err
	}
	if len(desiredComponents) != int(shardingSpec.Shards) {
		return nil, invalid("desired Component count %d does not match desired shards %d",
			len(desiredComponents), shardingSpec.Shards)
	}

	currentByName := make(map[string]*appsv1.Component, len(after))
	for i := range after {
		currentByName[after[i].Name] = &after[i]
	}
	staying := make([]appsv1.Component, 0, len(desiredComponents))
	for i := range desiredComponents {
		name := component.FullName(cluster.Name, desiredComponents[i].Spec.Name)
		current, ok := currentByName[name]
		if !ok {
			return nil, invalid("desired topology would create Component %q", name)
		}
		staying = append(staying, *current.DeepCopy())
		delete(currentByName, name)
	}
	leaving := make([]appsv1.Component, 0, len(currentByName))
	for _, comp := range currentByName {
		leaving = append(leaving, *comp.DeepCopy())
	}
	if len(leaving) == 0 || len(staying) == 0 || len(leaving)+len(staying) != len(after) {
		return nil, invalid("scale-in requires non-empty, exhaustive leaving and staying member sets")
	}
	if len(after) > 32 {
		return nil, invalid("current sharding contains more than 32 Components")
	}

	sortShardingScaleInComponents(after)
	sortShardingScaleInComponents(leaving)
	sortShardingScaleInComponents(staying)
	return &shardingScaleInTopologyInventory{
		Cluster:            cluster.DeepCopy(),
		Sharding:           *shardingSpec.DeepCopy(),
		ShardingDefinition: shardingDef.DeepCopy(),
		Components:         deepCopyShardingScaleInComponents(after),
		DesiredComponents:  deepCopyShardingScaleInDesiredComponents(desiredComponents),
		Leaving:            leaving,
		Staying:            staying,
	}, nil
}

func exactClusterSharding(cluster *appsv1.Cluster, shardingName string) (*appsv1.ClusterSharding, error) {
	var found *appsv1.ClusterSharding
	for i := range cluster.Spec.Shardings {
		if cluster.Spec.Shardings[i].Name != shardingName {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("%w: Cluster sharding %q is duplicated",
				errInvalidShardingScaleInTopology, shardingName)
		}
		found = &cluster.Spec.Shardings[i]
	}
	if found == nil {
		return nil, fmt.Errorf("%w: Cluster sharding %q was not found",
			errInvalidShardingScaleInTopology, shardingName)
	}
	return found, nil
}

func validateFreshShardingScaleInComponents(cluster *appsv1.Cluster,
	shardingDef *appsv1.ShardingDefinition, shardingName string, components []appsv1.Component,
) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInTopology, fmt.Sprintf(format, args...))
	}
	if len(components) == 0 || len(components) > 32 {
		return invalid("current sharding must contain between 1 and 32 Components")
	}
	names := map[string]struct{}{}
	uids := map[types.UID]struct{}{}
	shortNames := map[string]struct{}{}
	for i := range components {
		comp := &components[i]
		if comp.Namespace != cluster.Namespace || comp.Name == "" || comp.UID == "" ||
			comp.ResourceVersion == "" || comp.Generation <= 0 {
			return invalid("Component identity, generation, and resourceVersion must be complete")
		}
		if !comp.DeletionTimestamp.IsZero() {
			return invalid("Component is deleting: %s/%s", comp.Namespace, comp.Name)
		}
		if comp.Labels[constant.AppInstanceLabelKey] != cluster.Name ||
			comp.Labels[constant.KBAppShardingNameLabelKey] != shardingName ||
			comp.Labels[constant.ShardingDefLabelKey] != shardingDef.Name {
			return invalid("Component %s/%s does not carry exact Cluster, sharding, and definition labels",
				comp.Namespace, comp.Name)
		}
		owner := metav1.GetControllerOf(comp)
		if owner == nil || owner.APIVersion != appsv1.GroupVersion.String() ||
			owner.Kind != appsv1.ClusterKind || owner.Name != cluster.Name || owner.UID != cluster.UID {
			return invalid("Component %s/%s does not have the exact Cluster controller owner",
				comp.Namespace, comp.Name)
		}
		if comp.Annotations[shardingAddShardKey] != "" {
			return invalid("Component %s/%s has a pending shard-add", comp.Namespace, comp.Name)
		}
		shortName, err := component.ShortName(cluster.Name, comp.Name)
		if err != nil || shortName == "" || !strings.HasPrefix(shortName, shardingName+"-") {
			return invalid("Component %s/%s has an invalid short name", comp.Namespace, comp.Name)
		}
		if _, ok := names[comp.Name]; ok {
			return invalid("Component name %q is duplicated", comp.Name)
		}
		if _, ok := uids[comp.UID]; ok {
			return invalid("Component UID %q is duplicated", comp.UID)
		}
		if _, ok := shortNames[shortName]; ok {
			return invalid("Component short name %q is duplicated", shortName)
		}
		names[comp.Name] = struct{}{}
		uids[comp.UID] = struct{}{}
		shortNames[shortName] = struct{}{}
	}
	return nil
}

func listFreshShardingScaleInComponents(ctx context.Context, apiReader client.Reader,
	cluster *appsv1.Cluster, shardingName string,
) ([]appsv1.Component, error) {
	list := &appsv1.ComponentList{}
	if err := apiReader.List(ctx, list, client.InNamespace(cluster.Namespace)); err != nil {
		return nil, err
	}

	regularComponentNames := make(map[string]struct{}, len(cluster.Spec.ComponentSpecs))
	for i := range cluster.Spec.ComponentSpecs {
		if cluster.Spec.ComponentSpecs[i].Name != "" {
			regularComponentNames[component.FullName(cluster.Name, cluster.Spec.ComponentSpecs[i].Name)] =
				struct{}{}
		}
	}

	components := make([]appsv1.Component, 0)
	for i := range list.Items {
		comp := &list.Items[i]
		labelMatches := comp.Labels[constant.KBAppShardingNameLabelKey] == shardingName
		shortName, err := component.ShortName(cluster.Name, comp.Name)
		nameMatches := err == nil && strings.HasPrefix(shortName, shardingName+"-")
		_, regularComponent := regularComponentNames[comp.Name]
		if labelMatches || (nameMatches && !regularComponent) {
			components = append(components, *comp.DeepCopy())
		}
	}
	return components, nil
}

func flattenShardingScaleInDesiredComponents(clusterName string,
	desiredByTemplate map[string][]*appsv1.ClusterComponentSpec,
) ([]shardingScaleInDesiredComponent, error) {
	components := make([]shardingScaleInDesiredComponent, 0)
	names := map[string]struct{}{}
	for templateName, templateSpecs := range desiredByTemplate {
		if templateName == "" {
			templateName = shardingScaleInDefaultShardTemplate
		}
		for i := range templateSpecs {
			if templateSpecs[i] == nil || templateSpecs[i].Name == "" {
				return nil, fmt.Errorf("%w: desired Component name must not be empty",
					errInvalidShardingScaleInTopology)
			}
			spec := templateSpecs[i].DeepCopy()
			fullName := component.FullName(clusterName, spec.Name)
			if _, ok := names[fullName]; ok {
				return nil, fmt.Errorf("%w: desired Component name %q is duplicated",
					errInvalidShardingScaleInTopology, fullName)
			}
			names[fullName] = struct{}{}
			components = append(components, shardingScaleInDesiredComponent{
				ShardTemplateName: templateName,
				Spec:              *spec,
			})
		}
	}
	slices.SortFunc(components, func(a, b shardingScaleInDesiredComponent) int {
		return strings.Compare(a.Spec.Name, b.Spec.Name)
	})
	return components, nil
}

func sameShardingScaleInComponentSnapshot(a, b []appsv1.Component) bool {
	aCopy := deepCopyShardingScaleInComponents(a)
	bCopy := deepCopyShardingScaleInComponents(b)
	sortShardingScaleInComponents(aCopy)
	sortShardingScaleInComponents(bCopy)
	return reflect.DeepEqual(aCopy, bCopy)
}

func deepCopyShardingScaleInComponents(in []appsv1.Component) []appsv1.Component {
	out := make([]appsv1.Component, len(in))
	for i := range in {
		out[i] = *in[i].DeepCopy()
	}
	return out
}

func deepCopyShardingScaleInDesiredComponents(
	in []shardingScaleInDesiredComponent,
) []shardingScaleInDesiredComponent {
	out := make([]shardingScaleInDesiredComponent, len(in))
	for i := range in {
		out[i] = shardingScaleInDesiredComponent{
			ShardTemplateName: in[i].ShardTemplateName,
			Spec:              *in[i].Spec.DeepCopy(),
		}
	}
	return out
}

func sortShardingScaleInComponents(components []appsv1.Component) {
	slices.SortFunc(components, func(a, b appsv1.Component) int {
		return strings.Compare(a.Name, b.Name)
	})
}
