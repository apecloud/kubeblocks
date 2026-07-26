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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

func TestLoadFreshShardingScaleInTopology(t *testing.T) {
	const (
		namespace    = "default"
		clusterName  = "cluster"
		shardingName = "shard"
	)

	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))

	newCluster := func() *appsv1.Cluster {
		return &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       namespace,
				Name:            clusterName,
				UID:             types.UID("cluster-uid"),
				ResourceVersion: "10",
				Generation:      7,
			},
			Spec: appsv1.ClusterSpec{
				Shardings: []appsv1.ClusterSharding{{
					Name:        shardingName,
					ShardingDef: "valkey",
					Shards:      3,
					Template: appsv1.ClusterComponentSpec{
						ComponentDef: "valkey",
						Replicas:     1,
					},
				}},
			},
		}
	}
	newShardingDefinition := func() *appsv1.ShardingDefinition {
		return &appsv1.ShardingDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "valkey",
				UID:             types.UID("sharding-definition-uid"),
				ResourceVersion: "20",
				Generation:      3,
			},
			Spec: appsv1.ShardingDefinitionSpec{
				LifecycleActions: &appsv1.ShardingLifecycleActions{
					ShardRemove: &appsv1.ShardingAction{
						ResultProtocol: appsv1.ShardingScaleInResultProtocolV2,
					},
				},
			},
		}
	}
	newComponent := func(cluster *appsv1.Cluster, index int) *appsv1.Component {
		name := component.FullName(clusterName, fmt.Sprintf("%s-%d", shardingName, index))
		return &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       namespace,
				Name:            name,
				UID:             types.UID(fmt.Sprintf("component-uid-%d", index)),
				ResourceVersion: fmt.Sprintf("%d", 30+index),
				Generation:      int64(index + 1),
				Labels: map[string]string{
					constant.AppManagedByLabelKey:      constant.AppName,
					constant.AppInstanceLabelKey:       clusterName,
					constant.KBAppShardingNameLabelKey: shardingName,
					constant.KBAppComponentLabelKey:    fmt.Sprintf("%s-%d", shardingName, index),
					constant.ShardingDefLabelKey:       "valkey",
				},
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(
					cluster, appsv1.SchemeGroupVersion.WithKind(appsv1.ClusterKind))},
			},
			Spec: appsv1.ComponentSpec{
				CompDef:  "valkey",
				Replicas: 1,
			},
		}
	}
	newObjects := func() (*appsv1.Cluster, []client.Object) {
		cluster := newCluster()
		objects := []client.Object{cluster, newShardingDefinition()}
		for i := 0; i < 5; i++ {
			objects = append(objects, newComponent(cluster, i))
		}
		return cluster, objects
	}
	load := func(t *testing.T, reader client.Reader, cluster *appsv1.Cluster) (
		*shardingScaleInTopologyInventory, error,
	) {
		t.Helper()
		return loadFreshShardingScaleInTopology(context.Background(), reader,
			client.ObjectKeyFromObject(cluster), cluster.UID, shardingName)
	}

	t.Run("classifies one exact fresh 5 to 3 member set", func(t *testing.T) {
		cluster, objects := newObjects()
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

		inventory, err := load(t, reader, cluster)
		require.NoError(t, err)
		require.Equal(t, cluster.UID, inventory.Cluster.UID)
		require.Equal(t, "valkey", inventory.ShardingDefinition.Name)
		require.Len(t, inventory.Components, 5)
		require.Len(t, inventory.Staying, 3)
		require.Len(t, inventory.Leaving, 2)
		require.Len(t, inventory.DesiredComponents, 3)
		for _, desired := range inventory.DesiredComponents {
			require.Equal(t, shardingScaleInDefaultShardTemplate, desired.ShardTemplateName)
		}
		require.Equal(t, int32(3), inventory.Sharding.Shards)

		all := map[string]struct{}{}
		for _, comp := range inventory.Staying {
			all[comp.Name] = struct{}{}
		}
		for _, comp := range inventory.Leaving {
			all[comp.Name] = struct{}{}
		}
		require.Len(t, all, 5)
	})

	t.Run("rejects a different live Cluster UID", func(t *testing.T) {
		cluster, objects := newObjects()
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

		_, err := loadFreshShardingScaleInTopology(context.Background(), reader,
			client.ObjectKeyFromObject(cluster), types.UID("replacement-cluster"), shardingName)
		require.ErrorContains(t, err, "Cluster UID")
	})

	t.Run("rejects a deleting Cluster or Component", func(t *testing.T) {
		cluster, objects := newObjects()
		now := metav1.Now()
		cluster.DeletionTimestamp = &now
		cluster.Finalizers = []string{"test"}
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "Cluster is deleting")

		cluster, objects = newObjects()
		objects[2].(*appsv1.Component).DeletionTimestamp = &now
		objects[2].(*appsv1.Component).Finalizers = []string{"test"}
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "Component is deleting")
	})

	t.Run("rejects a member without the exact Cluster controller owner", func(t *testing.T) {
		cluster, objects := newObjects()
		comp := objects[2].(*appsv1.Component)
		comp.OwnerReferences[0].UID = types.UID("other-cluster")
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "controller owner")
	})

	t.Run("rejects a name-matching member with a missing sharding label", func(t *testing.T) {
		cluster, objects := newObjects()
		delete(objects[2].(*appsv1.Component).Labels, constant.KBAppShardingNameLabelKey)
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "exact Cluster, sharding, and definition labels")
	})

	t.Run("rejects pending shard add and non-v2 action", func(t *testing.T) {
		cluster, objects := newObjects()
		objects[2].(*appsv1.Component).Annotations = map[string]string{
			shardingAddShardKey: "pending",
		}
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "pending shard-add")

		cluster, objects = newObjects()
		objects[1].(*appsv1.ShardingDefinition).Spec.LifecycleActions.ShardRemove.ResultProtocol = ""
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "typed shard-remove action")
	})

	t.Run("rejects a desired member that would require a create", func(t *testing.T) {
		cluster, objects := newObjects()
		cluster.Spec.Shardings[0].Offline = []string{
			objects[2].(*appsv1.Component).Name,
			objects[3].(*appsv1.Component).Name,
			objects[4].(*appsv1.Component).Name,
		}
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "would create Component")
	})

	t.Run("rejects a component snapshot that changes while desired members are rebuilt", func(t *testing.T) {
		cluster, objects := newObjects()
		base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		reader := &shardingScaleInMutatingReader{
			Client: base,
			mutateBeforeThirdList: func(ctx context.Context) error {
				comp := &appsv1.Component{}
				key := client.ObjectKeyFromObject(objects[2])
				if err := base.Get(ctx, key, comp); err != nil {
					return err
				}
				comp.Annotations = map[string]string{"changed": "true"}
				return base.Update(ctx, comp)
			},
		}

		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "Component snapshot changed")
	})

	t.Run("returns deep copies detached from later API object mutation", func(t *testing.T) {
		cluster, objects := newObjects()
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

		inventory, err := load(t, reader, cluster)
		require.NoError(t, err)
		before := inventory.Components[0].Name
		objects[2].SetName("mutated")
		require.Equal(t, before, inventory.Components[0].Name)
	})

	t.Run("preserves heterogeneous template identity while sorting desired members", func(t *testing.T) {
		desired, err := flattenShardingScaleInDesiredComponents(clusterName,
			map[string][]*appsv1.ClusterComponentSpec{
				"blue": {{
					Name: "shard-b",
				}},
				"": {{
					Name: "shard-a",
				}},
			})
		require.NoError(t, err)
		require.Equal(t, []string{"shard-a", "shard-b"},
			[]string{desired[0].Spec.Name, desired[1].Spec.Name})
		require.Equal(t, shardingScaleInDefaultShardTemplate, desired[0].ShardTemplateName)
		require.Equal(t, "blue", desired[1].ShardTemplateName)
	})
}

type shardingScaleInMutatingReader struct {
	client.Client
	listCalls             int
	mutateBeforeThirdList func(context.Context) error
}

func (r *shardingScaleInMutatingReader) List(ctx context.Context, list client.ObjectList,
	opts ...client.ListOption) error {
	r.listCalls++
	if r.listCalls == 3 && r.mutateBeforeThirdList != nil {
		if err := r.mutateBeforeThirdList(ctx); err != nil {
			return err
		}
	}
	return r.Client.List(ctx, list, opts...)
}

var _ client.Reader = (*shardingScaleInMutatingReader)(nil)
