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
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

const (
	ShardIDLength = 3
)

const (
	generateShardIDMaxRetryTimes = 1000000
	defaultShardTemplateName     = ""
)

type shardIDGenerator struct {
	clusterName  string
	shardingName string
	running      []appsv1.Component
	offline      []string
	initialized  bool
	ids          sets.Set[string]
}

func (g *shardIDGenerator) allocate() (string, error) {
	if !g.initialized {
		g.init()
	}
	for i := 0; i < generateShardIDMaxRetryTimes; i++ {
		id := rand.String(ShardIDLength)
		name := fmt.Sprintf("%s-%s-%s", g.clusterName, g.shardingName, id)
		if !g.ids.Has(name) {
			return id, nil
		}
	}
	return "", fmt.Errorf("failed to allocate a unique shard id")
}

func (g *shardIDGenerator) init() {
	g.ids = sets.Set[string]{}
	for _, comp := range g.running {
		g.ids.Insert(comp.Name)
	}
	for _, name := range g.offline {
		g.ids.Insert(name)
	}
	g.initialized = true
}

func buildShardTemplates(clusterName string, sharding *appsv1.ClusterSharding, comps []appsv1.Component) []*shardTemplate {
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
	for _, comp := range comps {
		if model.IsObjectDeleting(&comp) || offline.Has(comp.Name) {
			continue
		}
		if comp.Labels == nil {
			continue
		}
		tplName, ok := comp.Labels[constant.KBAppShardTemplateLabelKey]
		if !ok {
			continue
		}
		idx, ok := nameToIndex[tplName]
		if !ok {
			continue
		}
		spec := templates[idx].template.DeepCopy()
		spec.Name, _ = strings.CutPrefix(comp.Name, fmt.Sprintf("%s-", clusterName))
		templates[idx].shards = append(templates[idx].shards, spec)
	}

	return templates
}

type shardTemplate struct {
	name     string
	count    int32
	template *appsv1.ClusterComponentSpec
	shards   []*appsv1.ClusterComponentSpec
}

func (t *shardTemplate) align(generator *shardIDGenerator, shardingName string) error {
	diff := len(t.shards) - int(t.count)
	switch {
	case diff == 0:
		return nil
	case diff < 0:
		return t.create(generator, shardingName, diff)
	default:
		return t.delete(diff)
	}
}

func (t *shardTemplate) create(generator *shardIDGenerator, shardingName string, cnt int) error {
	for i := 0; i < cnt; i++ {
		id, err := generator.allocate()
		if err != nil {
			return err
		}
		spec := t.template.DeepCopy()
		spec.Name = fmt.Sprintf("%s-%s", shardingName, id)
		t.shards = append(t.shards, spec)
	}
	return nil
}

func (t *shardTemplate) delete(cnt int) error {
	slices.SortFunc(t.shards, func(a, b *appsv1.ClusterComponentSpec) int {
		return strings.Compare(a.Name, b.Name)
	})
	t.shards = t.shards[:len(t.shards)-cnt]
	return nil
}
