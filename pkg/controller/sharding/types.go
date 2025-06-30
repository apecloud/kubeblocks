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
	clusterName  string
	shardingName string
	running      []string
	offline      []string
	initialized  bool
	ids          sets.Set[string]
}

func (g *shardIDGenerator) allocate() (string, error) {
	if !g.initialized {
		g.ids = sets.New(g.running...).Insert(g.offline...)
		g.initialized = true
	}
	for i := 0; i < generateShardIDMaxRetryTimes; i++ {
		id := rand.String(ShardIDLength)
		name := fmt.Sprintf("%s-%s-%s", g.clusterName, g.shardingName, id)
		if !g.ids.Has(name) {
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

func (t *shardTemplate) align(generator *shardIDGenerator, shardingName string) error {
	diff := len(t.shards) - int(t.count)
	switch {
	case diff == 0:
		return nil
	case diff < 0:
		return t.create(generator, shardingName, diff*-1)
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
