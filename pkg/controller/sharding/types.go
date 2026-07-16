/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

const (
	ShardIDLength = 3
)

const (
	generateShardIDMaxRetryTimes = 1000000
	defaultShardTemplateName     = ""
)

type shardIDGenerator struct {
	clusterName        string
	shardingName       string
	running            []string
	offline            []string
	takeOverByTemplate []string
	reservedByTemplate map[string][]string
	reservedIndexes    map[string]int
	reservedNames      sets.Set[string]
	initialized        bool
	ids                sets.Set[string]
}

func (g *shardIDGenerator) allocate(shardTemplateName string) (string, error) {
	if !g.initialized {
		g.ids = sets.New(g.running...).Insert(g.offline...).Insert(g.takeOverByTemplate...)
		g.reservedIndexes = map[string]int{}
		g.reservedNames = sets.New[string]()
		for _, names := range g.reservedByTemplate {
			g.reservedNames.Insert(names...)
		}
		g.initialized = true
	}
	reserved := g.reservedByTemplate[shardTemplateName]
	for g.reservedIndexes[shardTemplateName] < len(reserved) {
		idx := g.reservedIndexes[shardTemplateName]
		g.reservedIndexes[shardTemplateName]++
		name := reserved[idx]
		if g.ids.Has(name) {
			continue
		}
		id, ok := strings.CutPrefix(name, fmt.Sprintf("%s-%s-", g.clusterName, g.shardingName))
		if !ok || id == "" {
			return "", fmt.Errorf("reserved shard name %q does not belong to cluster %q and sharding %q",
				name, g.clusterName, g.shardingName)
		}
		g.ids.Insert(name)
		return id, nil
	}
	for i := 0; i < generateShardIDMaxRetryTimes; i++ {
		id := rand.String(ShardIDLength)
		name := fmt.Sprintf("%s-%s-%s", g.clusterName, g.shardingName, id)
		if !g.ids.Has(name) && !g.reservedNames.Has(name) {
			g.ids.Insert(name)
			return id, nil
		}
	}
	return "", fmt.Errorf("failed to allocate a unique shard id")
}

type shardTemplate struct {
	name     string
	count    int32
	template *appsv1.ClusterComponentSpec
	shards   []*appsv1.ClusterComponentSpec
}

func (t *shardTemplate) align(generator *shardIDGenerator) error {
	diff := len(t.shards) - int(t.count)
	switch {
	case diff == 0:
		return nil
	case diff < 0:
		return t.create(generator, diff*-1)
	default:
		return t.delete(diff)
	}
}

func (t *shardTemplate) create(generator *shardIDGenerator, cnt int) error {
	for i := 0; i < cnt; i++ {
		id, err := generator.allocate(t.name)
		if err != nil {
			return err
		}
		spec := t.template.DeepCopy()
		spec.Name = fmt.Sprintf("%s-%s", generator.shardingName, id)
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
