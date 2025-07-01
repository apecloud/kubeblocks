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

package sharding

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func BuildShardingCompSpecs(ctx context.Context, cli client.Reader,
	namespace, clusterName string, sharding *appsv1.ClusterSharding) (map[string][]*appsv1.ClusterComponentSpec, error) {
	if err := precheck(sharding); err != nil {
		return nil, err
	}
	shardingComps, err := listShardingComponents(ctx, cli, namespace, clusterName, sharding.Name)
	if err != nil {
		return nil, err
	}
	return buildShardingCompSpecs(clusterName, sharding, shardingComps)
}

func ListShardingComponents(ctx context.Context, cli client.Reader, cluster *appsv1.Cluster, shardingName string) ([]appsv1.Component, error) {
	return listShardingComponents(ctx, cli, cluster.Namespace, cluster.Name, shardingName)
}

func precheck(sharding *appsv1.ClusterSharding) error {
	shards := int32(0)
	shardIDs := sets.NewString()
	for _, tpl := range sharding.ShardTemplates {
		shards += ptr.Deref(tpl.Shards, 0)
		for _, id := range tpl.ShardIDs {
			if shardIDs.Has(id) {
				return fmt.Errorf("shard id %s is duplicated", id)
			}
		}
		shardIDs.Insert(tpl.ShardIDs...)
	}
	if shards > sharding.Shards {
		return fmt.Errorf("the sum of shards in shard templates is greater than the total shards: %d vs %d", sharding.Shards, shards)
	}
	return nil
}

func buildShardingCompSpecs(clusterName string, sharding *appsv1.ClusterSharding, shardingComps []appsv1.Component) (map[string][]*appsv1.ClusterComponentSpec, error) {
	compNames := make([]string, 0)
	for _, comp := range shardingComps {
		compNames = append(compNames, comp.Name)
	}

	generator := &shardIDGenerator{
		clusterName:        clusterName,
		shardingName:       sharding.Name,
		running:            compNames,
		offline:            sharding.Offline,
		takeOverByTemplate: shardNamesTakeOverByTemplate(clusterName, sharding),
	}

	templates := buildShardTemplates(clusterName, sharding, shardingComps)
	for i := range templates {
		if err := templates[i].align(generator); err != nil {
			return nil, err
		}
	}

	shards := map[string][]*appsv1.ClusterComponentSpec{}
	for i, tpl := range templates {
		shards[tpl.name] = templates[i].shards
	}
	return shards, nil
}

func listShardingComponents(ctx context.Context, cli client.Reader, namespace, clusterName, shardingName string) ([]appsv1.Component, error) {
	compList := &appsv1.ComponentList{}
	labels := constant.GetClusterLabels(clusterName, map[string]string{constant.KBAppShardingNameLabelKey: shardingName})
	if err := cli.List(ctx, compList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}
	return compList.Items, nil
}

func buildShardTemplates(clusterName string, sharding *appsv1.ClusterSharding, shardingComps []appsv1.Component) []*shardTemplate {
	mergeWithTemplate := func(tpl *appsv1.ShardTemplate) *appsv1.ClusterComponentSpec {
		spec := sharding.Template.DeepCopy()
		if tpl.ServiceVersion != nil || tpl.CompDef != nil {
			spec.ServiceVersion = ptr.Deref(tpl.ServiceVersion, "")
			spec.ComponentDef = ptr.Deref(tpl.CompDef, "")
		}
		if tpl.Replicas != nil {
			spec.Replicas = *tpl.Replicas
		}
		if tpl.Labels != nil {
			spec.Labels = tpl.Labels
		}
		if tpl.Annotations != nil {
			spec.Annotations = tpl.Annotations
		}
		if tpl.Env != nil {
			spec.Env = tpl.Env
		}
		if tpl.SchedulingPolicy != nil {
			spec.SchedulingPolicy = tpl.SchedulingPolicy
		}
		if tpl.Resources != nil {
			spec.Resources = *tpl.Resources
		}
		if tpl.VolumeClaimTemplates != nil {
			spec.VolumeClaimTemplates = tpl.VolumeClaimTemplates
		}
		if tpl.Instances != nil {
			spec.Instances = tpl.Instances
		}
		if tpl.FlatInstanceOrdinal != nil {
			spec.FlatInstanceOrdinal = *tpl.FlatInstanceOrdinal
		}
		return spec
	}

	templates := make([]*shardTemplate, 0)
	nameToIndex := map[string]int{}
	cnt := int32(0)
	for i, tpl := range sharding.ShardTemplates {
		if ptr.Deref(tpl.Shards, 0) <= 0 {
			continue
		}
		template := &shardTemplate{
			name:     tpl.Name,
			count:    ptr.Deref(tpl.Shards, 0),
			template: mergeWithTemplate(&sharding.ShardTemplates[i]),
			shards:   make([]*appsv1.ClusterComponentSpec, 0),
		}
		templates = append(templates, template)
		cnt += template.count
		nameToIndex[tpl.Name] = len(templates) - 1
	}
	if cnt < sharding.Shards {
		templates = append(templates, &shardTemplate{
			name:     defaultShardTemplateName,
			count:    sharding.Shards - cnt,
			template: &sharding.Template,
			shards:   make([]*appsv1.ClusterComponentSpec, 0),
		})
		nameToIndex[defaultShardTemplateName] = len(templates) - 1
	}

	offline := sets.New(sharding.Offline...)
	takeOverByTemplate := shardNamesTakeOverByTemplateMap(clusterName, sharding)
	for _, comp := range shardingComps {
		if model.IsObjectDeleting(&comp) || offline.Has(comp.Name) {
			continue
		}
		tplName := defaultShardTemplateName
		if comp.Labels != nil {
			if name, ok := comp.Labels[constant.KBAppShardTemplateLabelKey]; ok {
				tplName = name
			}
		}
		if tplName == defaultShardTemplateName {
			if name, ok := takeOverByTemplate[comp.Name]; ok {
				tplName = name
			}
		}
		idx, ok := nameToIndex[tplName]
		if !ok {
			continue // ignore the component
		}
		spec := templates[idx].template.DeepCopy()
		spec.Name, _ = strings.CutPrefix(comp.Name, fmt.Sprintf("%s-", clusterName))
		templates[idx].shards = append(templates[idx].shards, spec)
	}

	slices.SortFunc(templates, func(a, b *shardTemplate) int {
		return strings.Compare(a.name, b.name)
	})

	return templates
}

func shardNamesTakeOverByTemplate(clusterName string, sharding *appsv1.ClusterSharding) []string {
	result := make([]string, 0)
	for name := range maps.Keys(shardNamesTakeOverByTemplateMap(clusterName, sharding)) {
		result = append(result, name)
	}
	return result
}

func shardNamesTakeOverByTemplateMap(clusterName string, sharding *appsv1.ClusterSharding) map[string]string {
	result := make(map[string]string)
	for _, tpl := range sharding.ShardTemplates {
		for _, id := range tpl.ShardIDs {
			result[fmt.Sprintf("%s-%s-%s", clusterName, sharding.Name, id)] = tpl.Name
		}
	}
	return result
}
