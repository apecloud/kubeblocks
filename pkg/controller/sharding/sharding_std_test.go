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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

const testSeed = 1670750000

func newFakeReader(t *testing.T, objs ...client.Object) client.Reader {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func shardingComponent(clusterName, shardingName, id, templateName string) appsv1.Component {
	return appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", clusterName, shardingName, id),
			Namespace: "ns",
			Labels: map[string]string{
				constant.AppManagedByLabelKey:        constant.AppName,
				constant.AppInstanceLabelKey:         clusterName,
				constant.KBAppShardingNameLabelKey:   shardingName,
				constant.KBAppShardTemplateLabelKey:  templateName,
			},
		},
	}
}

// ============================================================
// types.go tests
// ============================================================

func TestShardIDGenerator_Allocate(t *testing.T) {
	rand.Seed(testSeed)

	t.Run("generates unique IDs", func(t *testing.T) {
		g := &shardIDGenerator{
			clusterName:  "cluster",
			shardingName: "shard",
		}
		seen := sets.New[string]()
		for i := 0; i < 10; i++ {
			id, err := g.allocate()
			require.NoError(t, err)
			assert.Len(t, id, ShardIDLength)
			name := fmt.Sprintf("cluster-shard-%s", id)
			assert.False(t, seen.Has(name), "duplicate ID generated: %s", id)
			seen.Insert(name)
		}
	})

	t.Run("avoids running names", func(t *testing.T) {
		g := &shardIDGenerator{
			clusterName:  "c",
			shardingName: "s",
			running:      []string{"c-s-abc"},
		}
		id, err := g.allocate()
		require.NoError(t, err)
		assert.NotEmpty(t, id)
		assert.NotEqual(t, "c-s-"+id, "c-s-abc")
	})

	t.Run("avoids offline names", func(t *testing.T) {
		g := &shardIDGenerator{
			clusterName:  "c",
			shardingName: "s",
			offline:      []string{"c-s-xyz"},
		}
		id, err := g.allocate()
		require.NoError(t, err)
		assert.NotEmpty(t, id)
	})

	t.Run("avoids takeOver names", func(t *testing.T) {
		g := &shardIDGenerator{
			clusterName:        "c",
			shardingName:       "s",
			takeOverByTemplate: []string{"c-s-ttt"},
		}
		id, err := g.allocate()
		require.NoError(t, err)
		assert.NotEmpty(t, id)
	})

	t.Run("initializes on first call", func(t *testing.T) {
		g := &shardIDGenerator{
			clusterName:  "c",
			shardingName: "s",
			running:      []string{"c-s-aaa"},
		}
		assert.False(t, g.initialized)
		_, err := g.allocate()
		require.NoError(t, err)
		assert.True(t, g.initialized)
	})
}

func TestShardTemplate_Align(t *testing.T) {
	rand.Seed(testSeed)

	t.Run("no change when count matches", func(t *testing.T) {
		tpl := &shardTemplate{
			name:     "test",
			count:    2,
			template: &appsv1.ClusterComponentSpec{Replicas: 3},
			shards: []*appsv1.ClusterComponentSpec{
				{Name: "shard-a"},
				{Name: "shard-b"},
			},
		}
		g := &shardIDGenerator{clusterName: "c", shardingName: "s"}
		err := tpl.align(g)
		require.NoError(t, err)
		assert.Len(t, tpl.shards, 2)
	})

	t.Run("creates shards when under count", func(t *testing.T) {
		tpl := &shardTemplate{
			name:     "test",
			count:    3,
			template: &appsv1.ClusterComponentSpec{Replicas: 3},
			shards:   []*appsv1.ClusterComponentSpec{},
		}
		g := &shardIDGenerator{clusterName: "c", shardingName: "s"}
		err := tpl.align(g)
		require.NoError(t, err)
		assert.Len(t, tpl.shards, 3)
		for _, s := range tpl.shards {
			assert.Equal(t, int32(3), s.Replicas)
			assert.Contains(t, s.Name, "s-")
		}
	})

	t.Run("deletes shards when over count", func(t *testing.T) {
		tpl := &shardTemplate{
			name:  "test",
			count: 1,
			shards: []*appsv1.ClusterComponentSpec{
				{Name: "s-aaa"},
				{Name: "s-bbb"},
				{Name: "s-ccc"},
			},
		}
		g := &shardIDGenerator{clusterName: "c", shardingName: "s"}
		err := tpl.align(g)
		require.NoError(t, err)
		assert.Len(t, tpl.shards, 1)
		assert.Equal(t, "s-aaa", tpl.shards[0].Name)
	})
}

func TestShardTemplate_Create(t *testing.T) {
	rand.Seed(testSeed)

	tpl := &shardTemplate{
		name:     "test",
		template: &appsv1.ClusterComponentSpec{Replicas: 5},
		shards:   make([]*appsv1.ClusterComponentSpec, 0),
	}
	g := &shardIDGenerator{clusterName: "c", shardingName: "sn"}
	err := tpl.create(g, 2)
	require.NoError(t, err)
	assert.Len(t, tpl.shards, 2)
	for _, s := range tpl.shards {
		assert.Equal(t, int32(5), s.Replicas)
		assert.Contains(t, s.Name, "sn-")
	}
}

func TestShardTemplate_Delete(t *testing.T) {
	tpl := &shardTemplate{
		shards: []*appsv1.ClusterComponentSpec{
			{Name: "s-ccc"},
			{Name: "s-aaa"},
			{Name: "s-bbb"},
		},
	}
	err := tpl.delete(1)
	require.NoError(t, err)
	assert.Len(t, tpl.shards, 2)
	// After sorting and removing last, should keep "aaa" and "bbb"
	assert.Equal(t, "s-aaa", tpl.shards[0].Name)
	assert.Equal(t, "s-bbb", tpl.shards[1].Name)
}

// ============================================================
// utils.go tests
// ============================================================

func TestPrecheck(t *testing.T) {
	t.Run("valid sharding passes", func(t *testing.T) {
		sharding := &appsv1.ClusterSharding{
			Name:   "shard",
			Shards: 4,
			ShardTemplates: []appsv1.ShardTemplate{
				{Name: "tpl-a", Shards: ptr.To[int32](2)},
				{Name: "tpl-b", Shards: ptr.To[int32](1)},
			},
		}
		assert.NoError(t, precheck(sharding))
	})

	t.Run("no shard templates passes", func(t *testing.T) {
		sharding := &appsv1.ClusterSharding{
			Name:   "shard",
			Shards: 2,
		}
		assert.NoError(t, precheck(sharding))
	})

	t.Run("duplicate template name fails", func(t *testing.T) {
		sharding := &appsv1.ClusterSharding{
			Name:   "shard",
			Shards: 4,
			ShardTemplates: []appsv1.ShardTemplate{
				{Name: "tpl-a", Shards: ptr.To[int32](1)},
				{Name: "tpl-a", Shards: ptr.To[int32](1)},
			},
		}
		err := precheck(sharding)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "shard template name tpl-a is duplicated")
	})

	t.Run("sum of shard templates exceeds total fails", func(t *testing.T) {
		sharding := &appsv1.ClusterSharding{
			Name:   "shard",
			Shards: 2,
			ShardTemplates: []appsv1.ShardTemplate{
				{Name: "tpl-a", Shards: ptr.To[int32](2)},
				{Name: "tpl-b", Shards: ptr.To[int32](1)},
			},
		}
		err := precheck(sharding)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the sum of shards in shard templates is greater than the total shards")
	})

	t.Run("duplicate shard IDs fails", func(t *testing.T) {
		sharding := &appsv1.ClusterSharding{
			Name:   "shard",
			Shards: 4,
			ShardTemplates: []appsv1.ShardTemplate{
				{Name: "tpl-a", Shards: ptr.To[int32](1), ShardIDs: []string{"abc"}},
				{Name: "tpl-b", Shards: ptr.To[int32](1), ShardIDs: []string{"abc"}},
			},
		}
		err := precheck(sharding)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "shard id abc is duplicated")
	})
}

func TestShardNamesTakeOverByTemplate(t *testing.T) {
	sharding := &appsv1.ClusterSharding{
		Name: "shard",
		ShardTemplates: []appsv1.ShardTemplate{
			{Name: "tpl-a", ShardIDs: []string{"id1", "id2"}},
			{Name: "tpl-b", ShardIDs: []string{"id3"}},
		},
	}
	result := shardNamesTakeOverByTemplate("cluster", sharding)
	assert.Len(t, result, 3)
	expected := sets.New("cluster-shard-id1", "cluster-shard-id2", "cluster-shard-id3")
	for _, name := range result {
		assert.True(t, expected.Has(name), "unexpected name: %s", name)
	}
}

func TestShardNamesTakeOverByTemplateMap(t *testing.T) {
	t.Run("maps shard IDs to template names", func(t *testing.T) {
		sharding := &appsv1.ClusterSharding{
			Name: "shard",
			ShardTemplates: []appsv1.ShardTemplate{
				{Name: "tpl-a", ShardIDs: []string{"id1", "id2"}},
				{Name: "tpl-b", ShardIDs: []string{"id3"}},
			},
		}
		result := shardNamesTakeOverByTemplateMap("cluster", sharding)
		assert.Len(t, result, 3)
		assert.Equal(t, "tpl-a", result["cluster-shard-id1"])
		assert.Equal(t, "tpl-a", result["cluster-shard-id2"])
		assert.Equal(t, "tpl-b", result["cluster-shard-id3"])
	})

	t.Run("empty shard templates returns empty", func(t *testing.T) {
		sharding := &appsv1.ClusterSharding{Name: "shard"}
		result := shardNamesTakeOverByTemplateMap("cluster", sharding)
		assert.Empty(t, result)
	})
}

func TestBuildShardTemplates(t *testing.T) {
	const (
		clusterName  = "cluster"
		shardingName = "shard"
	)

	t.Run("default template only", func(t *testing.T) {
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 2,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
		}
		templates := buildShardTemplates(clusterName, sharding, nil)
		assert.Len(t, templates, 1)
		assert.Equal(t, defaultShardTemplateName, templates[0].name)
		assert.Equal(t, int32(2), templates[0].count)
	})

	t.Run("template with overrides", func(t *testing.T) {
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 3,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
			ShardTemplates: []appsv1.ShardTemplate{
				{
					Name:     "custom",
					Shards:   ptr.To[int32](2),
					Replicas: ptr.To[int32](5),
				},
			},
		}
		templates := buildShardTemplates(clusterName, sharding, nil)
		assert.Len(t, templates, 2)
		// sorted by name: "" < "custom"
		assert.Equal(t, defaultShardTemplateName, templates[0].name)
		assert.Equal(t, int32(1), templates[0].count)
		assert.Equal(t, "custom", templates[1].name)
		assert.Equal(t, int32(2), templates[1].count)
		assert.Equal(t, int32(5), templates[1].template.Replicas)
	})

	t.Run("existing components assigned to correct template", func(t *testing.T) {
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 3,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
			ShardTemplates: []appsv1.ShardTemplate{
				{
					Name:   "custom",
					Shards: ptr.To[int32](1),
				},
			},
		}
		comps := []appsv1.Component{
			shardingComponent(clusterName, shardingName, "aaa", defaultShardTemplateName),
			shardingComponent(clusterName, shardingName, "bbb", "custom"),
		}
		templates := buildShardTemplates(clusterName, sharding, comps)
		assert.Len(t, templates, 2)
		// default template gets comp "aaa", custom gets "bbb"
		defaultTpl := templates[0]
		if defaultTpl.name != defaultShardTemplateName {
			defaultTpl = templates[1]
		}
		assert.Len(t, defaultTpl.shards, 1)
	})

	t.Run("skips deleting components", func(t *testing.T) {
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 2,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
		}
		now := metav1.Now()
		comps := []appsv1.Component{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:              fmt.Sprintf("%s-%s-aaa", clusterName, shardingName),
					DeletionTimestamp: &now,
					Finalizers:        []string{"test"},
					Labels: map[string]string{
						constant.KBAppShardTemplateLabelKey: defaultShardTemplateName,
					},
				},
			},
		}
		templates := buildShardTemplates(clusterName, sharding, comps)
		assert.Len(t, templates, 1)
		assert.Empty(t, templates[0].shards)
	})

	t.Run("skips offline components", func(t *testing.T) {
		comp := shardingComponent(clusterName, shardingName, "aaa", defaultShardTemplateName)
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 2,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
			Offline: []string{comp.Name},
		}
		templates := buildShardTemplates(clusterName, sharding, []appsv1.Component{comp})
		assert.Len(t, templates, 1)
		assert.Empty(t, templates[0].shards)
	})

	t.Run("shard template with zero shards is skipped", func(t *testing.T) {
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 2,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
			ShardTemplates: []appsv1.ShardTemplate{
				{Name: "zero", Shards: ptr.To[int32](0)},
			},
		}
		templates := buildShardTemplates(clusterName, sharding, nil)
		assert.Len(t, templates, 1)
		assert.Equal(t, defaultShardTemplateName, templates[0].name)
		assert.Equal(t, int32(2), templates[0].count)
	})

	t.Run("component with unknown template is ignored", func(t *testing.T) {
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 1,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
		}
		comps := []appsv1.Component{
			shardingComponent(clusterName, shardingName, "aaa", "nonexistent"),
		}
		templates := buildShardTemplates(clusterName, sharding, comps)
		assert.Len(t, templates, 1)
		assert.Empty(t, templates[0].shards)
	})

	t.Run("take over assigns default-labeled comp to named template", func(t *testing.T) {
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 2,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
			ShardTemplates: []appsv1.ShardTemplate{
				{
					Name:     "custom",
					Shards:   ptr.To[int32](1),
					ShardIDs: []string{"aaa"},
				},
			},
		}
		comp := shardingComponent(clusterName, shardingName, "aaa", defaultShardTemplateName)
		templates := buildShardTemplates(clusterName, sharding, []appsv1.Component{comp})
		var customTpl *shardTemplate
		for _, tpl := range templates {
			if tpl.name == "custom" {
				customTpl = tpl
			}
		}
		require.NotNil(t, customTpl)
		assert.Len(t, customTpl.shards, 1)
	})

	t.Run("mergeWithTemplate overrides all fields", func(t *testing.T) {
		env := []corev1.EnvVar{{Name: "X", Value: "Y"}}
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 2,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
			ShardTemplates: []appsv1.ShardTemplate{
				{
					Name:           "custom",
					Shards:         ptr.To[int32](1),
					Replicas:       ptr.To[int32](7),
					ServiceVersion: ptr.To("v2"),
					CompDef:        ptr.To("mydef"),
					Labels:         map[string]string{"l": "v"},
					Annotations:    map[string]string{"a": "v"},
					Env:            env,
				},
			},
		}
		templates := buildShardTemplates(clusterName, sharding, nil)
		var customTpl *shardTemplate
		for _, tpl := range templates {
			if tpl.name == "custom" {
				customTpl = tpl
			}
		}
		require.NotNil(t, customTpl)
		assert.Equal(t, int32(7), customTpl.template.Replicas)
		assert.Equal(t, "v2", customTpl.template.ServiceVersion)
		assert.Equal(t, "mydef", customTpl.template.ComponentDef)
		assert.Equal(t, map[string]string{"l": "v"}, customTpl.template.Labels)
		assert.Equal(t, map[string]string{"a": "v"}, customTpl.template.Annotations)
		assert.Equal(t, env, customTpl.template.Env)
	})
}

func TestBuildShardingCompSpecs(t *testing.T) {
	const (
		clusterName  = "cluster"
		shardingName = "shard"
	)

	rand.Seed(testSeed)

	t.Run("provision with no running components", func(t *testing.T) {
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 2,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
		}
		specs, err := buildShardingCompSpecs(clusterName, sharding, nil)
		require.NoError(t, err)
		assert.Len(t, specs, 1)
		assert.Contains(t, specs, defaultShardTemplateName)
		assert.Len(t, specs[defaultShardTemplateName], 2)
	})

	t.Run("precheck error propagates", func(t *testing.T) {
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 1,
			ShardTemplates: []appsv1.ShardTemplate{
				{Name: "a", Shards: ptr.To[int32](2)},
			},
		}
		_, err := buildShardingCompSpecs(clusterName, sharding, nil)
		assert.Error(t, err)
	})

	t.Run("scale out adds new shards", func(t *testing.T) {
		comp := shardingComponent(clusterName, shardingName, "xxx", defaultShardTemplateName)
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 3,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
		}
		specs, err := buildShardingCompSpecs(clusterName, sharding, []appsv1.Component{comp})
		require.NoError(t, err)
		assert.Len(t, specs[defaultShardTemplateName], 3)
	})

	t.Run("scale in removes shards", func(t *testing.T) {
		comps := []appsv1.Component{
			shardingComponent(clusterName, shardingName, "aaa", defaultShardTemplateName),
			shardingComponent(clusterName, shardingName, "bbb", defaultShardTemplateName),
			shardingComponent(clusterName, shardingName, "ccc", defaultShardTemplateName),
		}
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 1,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
		}
		specs, err := buildShardingCompSpecs(clusterName, sharding, comps)
		require.NoError(t, err)
		assert.Len(t, specs[defaultShardTemplateName], 1)
	})
}

func TestBuildShardingCompSpecs_WithClient(t *testing.T) {
	const (
		clusterName  = "cluster"
		shardingName = "shard"
		ns           = "ns"
	)

	comp := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-abc", clusterName, shardingName),
			Namespace: ns,
			Labels: map[string]string{
				constant.AppManagedByLabelKey:       constant.AppName,
				constant.AppInstanceLabelKey:        clusterName,
				constant.KBAppShardingNameLabelKey:  shardingName,
				constant.KBAppShardTemplateLabelKey: defaultShardTemplateName,
			},
		},
	}
	reader := newFakeReader(t, comp)

	sharding := &appsv1.ClusterSharding{
		Name:   shardingName,
		Shards: 2,
		Template: appsv1.ClusterComponentSpec{
			Replicas: 3,
		},
	}
	specs, err := BuildShardingCompSpecs(context.Background(), reader, ns, clusterName, sharding)
	require.NoError(t, err)
	assert.Len(t, specs[defaultShardTemplateName], 2)
}

func TestListShardingComponents(t *testing.T) {
	const (
		clusterName  = "cluster"
		shardingName = "shard"
		ns           = "ns"
	)

	comp1 := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-aaa", clusterName, shardingName),
			Namespace: ns,
			Labels: map[string]string{
				constant.AppManagedByLabelKey:      constant.AppName,
				constant.AppInstanceLabelKey:       clusterName,
				constant.KBAppShardingNameLabelKey: shardingName,
			},
		},
	}
	comp2 := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-bbb", clusterName, shardingName),
			Namespace: ns,
			Labels: map[string]string{
				constant.AppManagedByLabelKey:      constant.AppName,
				constant.AppInstanceLabelKey:       clusterName,
				constant.KBAppShardingNameLabelKey: shardingName,
			},
		},
	}
	otherComp := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-other-ccc", clusterName),
			Namespace: ns,
			Labels: map[string]string{
				constant.AppManagedByLabelKey:      constant.AppName,
				constant.AppInstanceLabelKey:       clusterName,
				constant.KBAppShardingNameLabelKey: "other",
			},
		},
	}
	reader := newFakeReader(t, comp1, comp2, otherComp)

	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns},
	}
	comps, err := ListShardingComponents(context.Background(), reader, cluster, shardingName)
	require.NoError(t, err)
	assert.Len(t, comps, 2)
}

// ============================================================
// legacy.go tests
// ============================================================

func TestParseCompShortName4Test(t *testing.T) {
	t.Run("valid name", func(t *testing.T) {
		name, err := parseCompShortName4Test("cluster", "cluster-shard-abc")
		require.NoError(t, err)
		assert.Equal(t, "shard-abc", name)
	})

	t.Run("no prefix errors", func(t *testing.T) {
		_, err := parseCompShortName4Test("cluster", "other-shard-abc")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "has no cluster name as prefix")
	})
}

func TestRemoveOfflineShards4Test(t *testing.T) {
	t.Run("no offline returns same", func(t *testing.T) {
		shards := []*appsv1.ClusterComponentSpec{
			{Name: "shard-a"},
			{Name: "shard-b"},
		}
		result := removeOfflineShards4Test(shards, nil)
		assert.Len(t, result, 2)
	})

	t.Run("removes offline shards", func(t *testing.T) {
		shards := []*appsv1.ClusterComponentSpec{
			{Name: "shard-a"},
			{Name: "shard-b"},
			{Name: "shard-c"},
		}
		result := removeOfflineShards4Test(shards, []string{"shard-b"})
		assert.Len(t, result, 2)
		for _, s := range result {
			assert.NotEqual(t, "shard-b", s.Name)
		}
	})

	t.Run("empty offline list", func(t *testing.T) {
		shards := []*appsv1.ClusterComponentSpec{
			{Name: "shard-a"},
		}
		result := removeOfflineShards4Test(shards, []string{})
		assert.Len(t, result, 1)
	})
}

func TestGenRandomShardName4Test(t *testing.T) {
	rand.Seed(testSeed)

	t.Run("generates unique name", func(t *testing.T) {
		existing := sets.New[string]()
		name, err := genRandomShardName4Test("shard", existing)
		require.NoError(t, err)
		assert.Contains(t, name, "shard-")
		assert.Len(t, name, len("shard-")+ShardIDLength)
	})

	t.Run("avoids existing names", func(t *testing.T) {
		existing := sets.New[string]()
		name1, err := genRandomShardName4Test("shard", existing)
		require.NoError(t, err)
		existing.Insert(name1)
		name2, err := genRandomShardName4Test("shard", existing)
		require.NoError(t, err)
		assert.NotEqual(t, name1, name2)
	})
}

func TestListShardingCompSpecs4Test(t *testing.T) {
	const (
		clusterName  = "cluster"
		shardingName = "shard"
		ns           = "default"
	)

	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns},
	}

	t.Run("nil sharding returns nil", func(t *testing.T) {
		reader := newFakeReader(t)
		result, err := listShardingCompSpecs4Test(context.Background(), reader, cluster, nil, false)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns undeleted components", func(t *testing.T) {
		comp := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-abc", clusterName, shardingName),
				Namespace: ns,
				Labels: map[string]string{
					constant.AppManagedByLabelKey:      constant.AppName,
					constant.AppInstanceLabelKey:       clusterName,
					constant.KBAppShardingNameLabelKey: shardingName,
				},
			},
		}
		reader := newFakeReader(t, comp)
		sharding := &appsv1.ClusterSharding{
			Name: shardingName,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
		}
		result, err := listShardingCompSpecs4Test(context.Background(), reader, cluster, sharding, false)
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "shard-abc", result[0].Name)
	})

	t.Run("includeDeleting includes deleting components", func(t *testing.T) {
		now := metav1.NewTime(time.Now())
		comp := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:              fmt.Sprintf("%s-%s-abc", clusterName, shardingName),
				Namespace:         ns,
				DeletionTimestamp: &now,
				Finalizers:        []string{"test"},
				Labels: map[string]string{
					constant.AppManagedByLabelKey:      constant.AppName,
					constant.AppInstanceLabelKey:       clusterName,
					constant.KBAppShardingNameLabelKey: shardingName,
				},
			},
		}
		reader := newFakeReader(t, comp)
		sharding := &appsv1.ClusterSharding{
			Name: shardingName,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
		}
		// Without includeDeleting
		result, err := listShardingCompSpecs4Test(context.Background(), reader, cluster, sharding, false)
		require.NoError(t, err)
		assert.Len(t, result, 0)

		// With includeDeleting
		result, err = listShardingCompSpecs4Test(context.Background(), reader, cluster, sharding, true)
		require.NoError(t, err)
		assert.Len(t, result, 1)
	})
}

func TestGenShardingCompSpecList4Test(t *testing.T) {
	const (
		clusterName  = "cluster"
		shardingName = "shard"
		ns           = "default"
	)

	rand.Seed(testSeed)

	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns},
	}

	t.Run("provision creates new shards", func(t *testing.T) {
		reader := newFakeReader(t)
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 2,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
		}
		specs, err := GenShardingCompSpecList4Test(context.Background(), reader, cluster, sharding)
		require.NoError(t, err)
		assert.Len(t, specs, 2)
	})

	t.Run("existing shards match count returns as-is", func(t *testing.T) {
		comp := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-abc", clusterName, shardingName),
				Namespace: ns,
				Labels: map[string]string{
					constant.AppManagedByLabelKey:      constant.AppName,
					constant.AppInstanceLabelKey:       clusterName,
					constant.KBAppShardingNameLabelKey: shardingName,
				},
			},
		}
		reader := newFakeReader(t, comp)
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 1,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
		}
		specs, err := GenShardingCompSpecList4Test(context.Background(), reader, cluster, sharding)
		require.NoError(t, err)
		assert.Len(t, specs, 1)
	})

	t.Run("scale in trims shards", func(t *testing.T) {
		comp1 := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-aaa", clusterName, shardingName),
				Namespace: ns,
				Labels: map[string]string{
					constant.AppManagedByLabelKey:      constant.AppName,
					constant.AppInstanceLabelKey:       clusterName,
					constant.KBAppShardingNameLabelKey: shardingName,
				},
			},
		}
		comp2 := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-bbb", clusterName, shardingName),
				Namespace: ns,
				Labels: map[string]string{
					constant.AppManagedByLabelKey:      constant.AppName,
					constant.AppInstanceLabelKey:       clusterName,
					constant.KBAppShardingNameLabelKey: shardingName,
				},
			},
		}
		reader := newFakeReader(t, comp1, comp2)
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 1,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
		}
		specs, err := GenShardingCompSpecList4Test(context.Background(), reader, cluster, sharding)
		require.NoError(t, err)
		assert.Len(t, specs, 1)
	})

	t.Run("offline shards excluded", func(t *testing.T) {
		comp := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-abc", clusterName, shardingName),
				Namespace: ns,
				Labels: map[string]string{
					constant.AppManagedByLabelKey:      constant.AppName,
					constant.AppInstanceLabelKey:       clusterName,
					constant.KBAppShardingNameLabelKey: shardingName,
				},
			},
		}
		reader := newFakeReader(t, comp)
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 1,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
			Offline: []string{fmt.Sprintf("%s-%s-abc", clusterName, shardingName)},
		}
		specs, err := GenShardingCompSpecList4Test(context.Background(), reader, cluster, sharding)
		require.NoError(t, err)
		// offline comp is excluded, so a new one is created
		assert.Len(t, specs, 1)
		assert.NotEqual(t, "shard-abc", specs[0].Name)
	})

	t.Run("invalid offline name returns error", func(t *testing.T) {
		reader := newFakeReader(t)
		sharding := &appsv1.ClusterSharding{
			Name:   shardingName,
			Shards: 1,
			Template: appsv1.ClusterComponentSpec{
				Replicas: 3,
			},
			Offline: []string{"no-prefix-match"},
		}
		_, err := GenShardingCompSpecList4Test(context.Background(), reader, cluster, sharding)
		assert.Error(t, err)
	})
}

func TestListNCheckShardingComponents4Test(t *testing.T) {
	const (
		clusterName  = "cluster"
		shardingName = "shard"
		ns           = "default"
	)

	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns},
	}
	sharding := &appsv1.ClusterSharding{Name: shardingName}

	t.Run("separates undeleted and deleting", func(t *testing.T) {
		now := metav1.NewTime(time.Now())
		comp1 := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-aaa", clusterName, shardingName),
				Namespace: ns,
				Labels: map[string]string{
					constant.AppManagedByLabelKey:      constant.AppName,
					constant.AppInstanceLabelKey:       clusterName,
					constant.KBAppShardingNameLabelKey: shardingName,
				},
			},
		}
		comp2 := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:              fmt.Sprintf("%s-%s-bbb", clusterName, shardingName),
				Namespace:         ns,
				DeletionTimestamp: &now,
				Finalizers:        []string{"test"},
				Labels: map[string]string{
					constant.AppManagedByLabelKey:      constant.AppName,
					constant.AppInstanceLabelKey:       clusterName,
					constant.KBAppShardingNameLabelKey: shardingName,
				},
			},
		}
		reader := newFakeReader(t, comp1, comp2)
		undeleted, deleting, err := listNCheckShardingComponents4Test(context.Background(), reader, cluster, sharding)
		require.NoError(t, err)
		assert.Len(t, undeleted, 1)
		assert.Len(t, deleting, 1)
		assert.Equal(t, comp1.Name, undeleted[0].Name)
		assert.Equal(t, comp2.Name, deleting[0].Name)
	})
}

func TestListShardingCompSpecs_Public(t *testing.T) {
	const (
		clusterName  = "cluster"
		shardingName = "shard"
		ns           = "default"
	)

	comp := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-abc", clusterName, shardingName),
			Namespace: ns,
			Labels: map[string]string{
				constant.AppManagedByLabelKey:      constant.AppName,
				constant.AppInstanceLabelKey:       clusterName,
				constant.KBAppShardingNameLabelKey: shardingName,
			},
		},
	}
	reader := newFakeReader(t, comp)
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: ns},
	}
	sharding := &appsv1.ClusterSharding{
		Name: shardingName,
		Template: appsv1.ClusterComponentSpec{
			Replicas: 3,
		},
	}
	result, err := ListShardingCompSpecs(context.Background(), reader, cluster, sharding)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "shard-abc", result[0].Name)
}
