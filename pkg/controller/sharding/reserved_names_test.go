/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package sharding

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func TestBuildShardingCompSpecsKeepsReservedNamesInTheirTemplates(t *testing.T) {
	const (
		clusterName       = "redis"
		shardingName      = "redis-cluster"
		shardTemplateName = "hot"
		defaultName       = "redis-redis-cluster-abc"
		hotName           = "redis-redis-cluster-xyz"
	)
	sharding := &appsv1.ClusterSharding{
		Name:   shardingName,
		Shards: 2,
		Template: appsv1.ClusterComponentSpec{
			Replicas: 3,
		},
		ShardTemplates: []appsv1.ShardTemplate{{
			Name:     shardTemplateName,
			Shards:   ptr.To[int32](1),
			Replicas: ptr.To[int32](5),
		}},
	}

	specs, err := buildShardingCompSpecsWithReservedNames(clusterName, sharding, nil, map[string][]string{
		defaultShardTemplateName: {defaultName},
		shardTemplateName:        {hotName},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := specs[defaultShardTemplateName]; len(got) != 1 || got[0].Name != "redis-cluster-abc" ||
		got[0].Replicas != 3 {
		t.Fatalf("default template specs=%#v", got)
	}
	if got := specs[shardTemplateName]; len(got) != 1 || got[0].Name != "redis-cluster-xyz" ||
		got[0].Replicas != 5 {
		t.Fatalf("named template specs=%#v", got)
	}
}

func TestBuildShardingCompSpecsDoesNotAllocateAnotherTemplatesReservedName(t *testing.T) {
	const (
		seed              = 1670750000
		clusterName       = "redis"
		shardingName      = "redis-cluster"
		shardTemplateName = "hot"
		reservedHotName   = "redis-redis-cluster-bvj"
	)
	sharding := &appsv1.ClusterSharding{
		Name:     shardingName,
		Shards:   2,
		Template: appsv1.ClusterComponentSpec{Replicas: 3},
		ShardTemplates: []appsv1.ShardTemplate{{
			Name:     shardTemplateName,
			Shards:   ptr.To[int32](1),
			Replicas: ptr.To[int32](5),
		}},
	}
	rand.Seed(seed)
	specs, err := buildShardingCompSpecsWithReservedNames(clusterName, sharding, nil, map[string][]string{
		shardTemplateName: {reservedHotName},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := specs[shardTemplateName]; len(got) != 1 || got[0].Name != "redis-cluster-bvj" {
		t.Fatalf("named template specs=%#v", got)
	}
	if got := specs[defaultShardTemplateName]; len(got) != 1 || got[0].Name == "redis-cluster-bvj" {
		t.Fatalf("default template consumed another template's reserved name: %#v", got)
	}
}

func TestBuildShardingCompSpecsRejectsInvalidReservedNames(t *testing.T) {
	sharding := &appsv1.ClusterSharding{
		Name:     "redis-cluster",
		Shards:   1,
		Template: appsv1.ClusterComponentSpec{Replicas: 1},
	}
	tests := []struct {
		name     string
		reserved map[string][]string
	}{
		{
			name: "wrong cluster prefix",
			reserved: map[string][]string{
				defaultShardTemplateName: {"other-redis-cluster-abc"},
			},
		},
		{
			name: "invalid generated shard id",
			reserved: map[string][]string{
				defaultShardTemplateName: {"redis-redis-cluster-abcd"},
			},
		},
		{
			name: "duplicate across templates",
			reserved: map[string][]string{
				defaultShardTemplateName: {"redis-redis-cluster-abc"},
				"other":                  {"redis-redis-cluster-abc"},
			},
		},
		{
			name: "unknown template",
			reserved: map[string][]string{
				"unknown": {"redis-redis-cluster-abc"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := buildShardingCompSpecsWithReservedNames("redis", sharding, nil, test.reserved); err == nil {
				t.Fatal("invalid reserved shard names were accepted")
			}
		})
	}
}
