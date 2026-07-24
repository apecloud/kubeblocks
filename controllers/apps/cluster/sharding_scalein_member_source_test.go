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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestLoadFreshShardingScaleInMembers(t *testing.T) {
	const (
		namespace    = "default"
		clusterName  = "cluster"
		shardingName = "shard"
	)

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, workloadsv1.AddToScheme(scheme))

	newObjects := func() (*appsv1.Cluster, []client.Object) {
		cluster := &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       namespace,
				Name:            clusterName,
				UID:             "cluster-uid",
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
		objects := []client.Object{
			cluster,
			&appsv1.ShardingDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "valkey",
					UID:             "sharding-definition-uid",
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
			},
		}
		for i := 0; i < 5; i++ {
			shortName := fmt.Sprintf("%s-%d", shardingName, i)
			fullName := component.FullName(clusterName, shortName)
			labels := constant.GetCompLabels(clusterName, shortName)
			labels[constant.KBAppShardingNameLabelKey] = shardingName
			labels[constant.ShardingDefLabelKey] = "valkey"
			if i == 4 {
				labels[constant.KBAppShardTemplateLabelKey] = "retired-blue"
			}
			comp := &appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       namespace,
					Name:            fullName,
					UID:             types.UID(fmt.Sprintf("component-uid-%d", i)),
					ResourceVersion: fmt.Sprintf("%d", 30+i),
					Generation:      int64(i + 1),
					Labels:          labels,
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(
						cluster, appsv1.SchemeGroupVersion.WithKind(appsv1.ClusterKind))},
				},
				Spec: appsv1.ComponentSpec{
					CompDef:  "valkey",
					Replicas: 1,
				},
			}
			workload := &workloadsv1.InstanceSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       namespace,
					Name:            fullName,
					UID:             types.UID(fmt.Sprintf("instanceset-uid-%d", i)),
					ResourceVersion: fmt.Sprintf("%d", 40+i),
					Generation:      int64(i + 1),
					Labels:          constant.GetCompLabels(clusterName, shortName),
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(
						comp, appsv1.SchemeGroupVersion.WithKind(appsv1.ComponentKind))},
				},
			}
			podLabels := constant.GetCompLabels(clusterName, shortName)
			podLabels[instanceset.WorkloadsManagedByLabelKey] = workloadsv1.InstanceSetKind
			podLabels[instanceset.WorkloadsInstanceLabelKey] = workload.Name
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       namespace,
					Name:            fullName + "-0",
					UID:             types.UID(fmt.Sprintf("pod-uid-%d", i)),
					ResourceVersion: fmt.Sprintf("%d", 50+i),
					Labels:          podLabels,
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(
						workload, workloadsv1.GroupVersion.WithKind(workloadsv1.InstanceSetKind))},
				},
			}
			objects = append(objects, comp, workload, pod)
		}
		return cluster, objects
	}
	load := func(t *testing.T, reader client.Reader, cluster *appsv1.Cluster) (
		*shardingScaleInMemberInventory, error,
	) {
		t.Helper()
		return loadFreshShardingScaleInMembers(context.Background(), reader,
			client.ObjectKeyFromObject(cluster), cluster.UID, shardingName)
	}

	t.Run("builds an exact source-only member and Pod inventory", func(t *testing.T) {
		cluster, objects := newObjects()
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

		inventory, err := load(t, reader, cluster)
		require.NoError(t, err)
		require.Len(t, inventory.Leaving, 2)
		require.Len(t, inventory.Staying, 3)
		require.Equal(t, shardingScaleInDefaultShardTemplate, inventory.Leaving[0].ShardTemplateName)
		require.Equal(t, "retired-blue", inventory.Leaving[1].ShardTemplateName)
		for _, member := range append(inventory.Leaving, inventory.Staying...) {
			require.Equal(t, member.Component.Name, member.Workload.Name)
			require.Len(t, member.Pods, 1)
			require.Equal(t, intctrlutil.PodFQDN(namespace, member.Component.Name, member.Pods[0].Pod.Name),
				member.Pods[0].FQDN)
		}
	})

	t.Run("rejects missing exact Pod labels or a wrong Pod owner", func(t *testing.T) {
		cluster, objects := newObjects()
		pod := objects[4].(*corev1.Pod)
		delete(pod.Labels, constant.KBAppComponentLabelKey)
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "Pod labels")

		cluster, objects = newObjects()
		pod = objects[4].(*corev1.Pod)
		pod.OwnerReferences[0].UID = "other-workload"
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "Pod controller owner")
	})

	t.Run("rejects missing exact workload ownership or a deleting object", func(t *testing.T) {
		cluster, objects := newObjects()
		workload := objects[3].(*workloadsv1.InstanceSet)
		workload.OwnerReferences[0].UID = "other-component"
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "InstanceSet controller owner")

		cluster, objects = newObjects()
		now := metav1.Now()
		pod := objects[4].(*corev1.Pod)
		pod.DeletionTimestamp = &now
		pod.Finalizers = []string{"test"}
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "Pod is deleting")
	})

	t.Run("rejects missing Component identity and duplicate workload UID", func(t *testing.T) {
		cluster, objects := newObjects()
		delete(objects[2].(*appsv1.Component).Labels, constant.KBAppComponentLabelKey)
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "exact component-name label")

		cluster, objects = newObjects()
		objects[6].(*workloadsv1.InstanceSet).UID = objects[3].(*workloadsv1.InstanceSet).UID
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "InstanceSet UID")
	})

	t.Run("rejects zero, duplicate, or more than five Pods per member", func(t *testing.T) {
		cluster, objects := newObjects()
		objects = append(objects[:4], objects[5:]...)
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "between 1 and 5 Pods")

		cluster, objects = newObjects()
		original := objects[4].(*corev1.Pod)
		for i := 1; i <= 5; i++ {
			pod := original.DeepCopy()
			pod.Name = fmt.Sprintf("%s-%d", objects[2].(*appsv1.Component).Name, i)
			pod.UID = types.UID(fmt.Sprintf("extra-pod-uid-%d", i))
			pod.ResourceVersion = fmt.Sprintf("%d", 60+i)
			objects = append(objects, pod)
		}
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "between 1 and 5 Pods")

		cluster, objects = newObjects()
		duplicate := objects[7].(*corev1.Pod)
		duplicate.UID = objects[4].(*corev1.Pod).UID
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "Pod UID")
	})

	t.Run("rejects a Pod snapshot that changes during inventory construction", func(t *testing.T) {
		cluster, objects := newObjects()
		base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		reader := &shardingScaleInPodMutatingReader{
			Client: base,
			mutateBeforeSecondPodList: func(ctx context.Context) error {
				pod := &corev1.Pod{}
				if err := base.Get(ctx, client.ObjectKeyFromObject(objects[4]), pod); err != nil {
					return err
				}
				pod.Annotations = map[string]string{"changed": "true"}
				return base.Update(ctx, pod)
			},
		}

		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "Pod snapshot changed")
	})

	t.Run("returns detached member, workload, and Pod copies", func(t *testing.T) {
		cluster, objects := newObjects()
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

		inventory, err := load(t, reader, cluster)
		require.NoError(t, err)
		componentName := inventory.Leaving[0].Component.Name
		workloadName := inventory.Leaving[0].Workload.Name
		podName := inventory.Leaving[0].Pods[0].Pod.Name
		objects[2].SetName("mutated-component")
		objects[3].SetName("mutated-workload")
		objects[4].SetName("mutated-pod")
		require.Equal(t, componentName, inventory.Leaving[0].Component.Name)
		require.Equal(t, workloadName, inventory.Leaving[0].Workload.Name)
		require.Equal(t, podName, inventory.Leaving[0].Pods[0].Pod.Name)
	})
}

type shardingScaleInPodMutatingReader struct {
	client.Client
	podListCalls              int
	mutateBeforeSecondPodList func(context.Context) error
}

func (r *shardingScaleInPodMutatingReader) List(ctx context.Context, list client.ObjectList,
	opts ...client.ListOption,
) error {
	if _, ok := list.(*corev1.PodList); ok {
		r.podListCalls++
		if r.podListCalls == 2 && r.mutateBeforeSecondPodList != nil {
			if err := r.mutateBeforeSecondPodList(ctx); err != nil {
				return err
			}
		}
	}
	return r.Client.List(ctx, list, opts...)
}

var _ client.Reader = (*shardingScaleInPodMutatingReader)(nil)
