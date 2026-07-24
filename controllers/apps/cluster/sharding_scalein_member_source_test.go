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
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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
					Labels:          constant.GetCompLabels(clusterName, shortName, labels),
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(
						comp, appsv1.SchemeGroupVersion.WithKind(appsv1.ComponentKind))},
				},
			}
			podLabels := constant.GetCompLabels(clusterName, shortName, labels)
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
				Spec: corev1.PodSpec{
					Hostname:  fullName + "-0",
					Subdomain: fullName + "-headless",
				},
			}
			objects = append(objects, comp, workload, pod)
		}
		return cluster, objects
	}
	enableInstanceAPI := func(objects []client.Object, memberIndex int) *workloadsv1.Instance {
		objectIndex := 2 + memberIndex*3
		comp := objects[objectIndex].(*appsv1.Component)
		workload := objects[objectIndex+1].(*workloadsv1.InstanceSet)
		pod := objects[objectIndex+2].(*corev1.Pod)
		comp.Spec.EnableInstanceAPI = ptr.To(true)
		workload.Spec.EnableInstanceAPI = ptr.To(true)

		instance := &workloadsv1.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       namespace,
				Name:            pod.Name,
				UID:             types.UID(fmt.Sprintf("instance-uid-%d", memberIndex)),
				ResourceVersion: fmt.Sprintf("%d", 70+memberIndex),
				Generation:      int64(memberIndex + 1),
				Labels:          make(map[string]string, len(pod.Labels)),
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(
					workload, workloadsv1.GroupVersion.WithKind(workloadsv1.InstanceSetKind))},
			},
			Spec: workloadsv1.InstanceSpec{
				InstanceSetName: workload.Name,
			},
		}
		for key, value := range pod.Labels {
			instance.Labels[key] = value
		}
		pod.Labels[constant.KBAppInstanceNameLabelKey] = instance.Name
		pod.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(
			instance, workloadsv1.GroupVersion.WithKind(shardingScaleInInstanceKind))}
		return instance
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

	t.Run("builds the exact InstanceSet Instance Pod chain when Instance API is enabled", func(t *testing.T) {
		cluster, objects := newObjects()
		instance := enableInstanceAPI(objects, 0)
		objects = append(objects, instance)
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

		inventory, err := load(t, reader, cluster)
		require.NoError(t, err)
		members := slices.Concat(inventory.Leaving, inventory.Staying)
		var instanceMember *shardingScaleInSourceMember
		for i := range members {
			if members[i].Component.UID == objects[2].(*appsv1.Component).UID {
				instanceMember = &members[i]
				break
			}
		}
		require.NotNil(t, instanceMember)
		require.Len(t, instanceMember.Instances, 1)
		require.Equal(t, instance.UID, instanceMember.Instances[0].UID)
		require.Equal(t, instance.UID, metav1.GetControllerOf(&instanceMember.Pods[0].Pod).UID)
	})

	t.Run("rejects an incomplete Instance API owner chain", func(t *testing.T) {
		cluster, objects := newObjects()
		instance := enableInstanceAPI(objects, 0)
		instance.OwnerReferences[0].UID = "other-workload"
		objects = append(objects, instance)
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "Instance controller owner")

		cluster, objects = newObjects()
		instance = enableInstanceAPI(objects, 0)
		instance.OwnerReferences[0].UID = "other-workload"
		instance.Labels[instanceset.WorkloadsInstanceLabelKey] = "other-workload"
		objects = append(objects, instance)
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "Instance labels and spec")

		cluster, objects = newObjects()
		instance = enableInstanceAPI(objects, 0)
		objects[4].(*corev1.Pod).OwnerReferences[0].UID = "other-instance"
		objects = append(objects, instance)
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "Pod controller owner")

		cluster, objects = newObjects()
		enableInstanceAPI(objects, 0)
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "Instance API")

		cluster, objects = newObjects()
		instance = enableInstanceAPI(objects, 0)
		objects[2].(*appsv1.Component).Spec.EnableInstanceAPI = ptr.To(false)
		objects = append(objects, instance)
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "EnableInstanceAPI")

		cluster, objects = newObjects()
		firstInstance := enableInstanceAPI(objects, 0)
		secondInstance := enableInstanceAPI(objects, 1)
		firstPod := objects[4].(*corev1.Pod)
		firstPod.Name = secondInstance.Name
		firstPod.Labels[constant.KBAppInstanceNameLabelKey] = secondInstance.Name
		firstPod.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(
			secondInstance, workloadsv1.GroupVersion.WithKind(shardingScaleInInstanceKind))}
		objects = append(objects[:7], objects[8:]...)
		objects = append(objects, firstInstance, secondInstance)
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "Pod controller owner")
	})

	t.Run("rejects extra owner-chain objects selected by any exact workload hint", func(t *testing.T) {
		cluster, objects := newObjects()
		instance := enableInstanceAPI(objects, 0)
		extraInstance := instance.DeepCopy()
		extraInstance.Name += "-extra"
		extraInstance.UID = "extra-instance-uid"
		extraInstance.ResourceVersion = "79"
		extraInstance.OwnerReferences[0].UID = "other-workload"
		extraInstance.Labels[constant.AppInstanceLabelKey] = "other-cluster"
		extraInstance.Labels[constant.KBAppComponentLabelKey] = "other-component"
		extraInstance.Labels[instanceset.WorkloadsInstanceLabelKey] = "other-workload"
		objects = append(objects, instance, extraInstance)
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "Instance labels")

		cluster, objects = newObjects()
		instance = enableInstanceAPI(objects, 0)
		extraPod := objects[4].(*corev1.Pod).DeepCopy()
		extraPod.Name += "-extra"
		extraPod.UID = "extra-pod-uid"
		extraPod.ResourceVersion = "89"
		extraPod.Spec.Hostname = extraPod.Name
		extraPod.OwnerReferences[0].UID = "other-instance"
		extraPod.Labels[constant.AppInstanceLabelKey] = "other-cluster"
		extraPod.Labels[constant.KBAppComponentLabelKey] = "other-component"
		extraPod.Labels[constant.KBAppInstanceNameLabelKey] = "other-instance"
		objects = append(objects, instance, extraPod)
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "Pod labels")
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

		cluster, objects = newObjects()
		pod = objects[4].(*corev1.Pod)
		pod.Labels[constant.KBAppShardingNameLabelKey] = "other-sharding"
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "Pod labels")
	})

	t.Run("rejects non-canonical Pod DNS identity", func(t *testing.T) {
		cluster, objects := newObjects()
		pod := objects[4].(*corev1.Pod)
		pod.Spec.Hostname = "other-hostname"
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "Pod hostname and subdomain")

		cluster, objects = newObjects()
		pod = objects[4].(*corev1.Pod)
		pod.Spec.Subdomain = "other-headless"
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "Pod hostname and subdomain")
	})

	t.Run("rejects missing exact workload ownership or a deleting object", func(t *testing.T) {
		cluster, objects := newObjects()
		workload := objects[3].(*workloadsv1.InstanceSet)
		workload.OwnerReferences[0].UID = "other-component"
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "InstanceSet controller owner")

		cluster, objects = newObjects()
		workload = objects[3].(*workloadsv1.InstanceSet)
		workload.Labels[constant.KBAppShardTemplateLabelKey] = "wrong-template"
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "InstanceSet labels")

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
			pod.Spec.Hostname = pod.Name
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

	t.Run("rejects an InstanceSet snapshot that changes during inventory construction", func(t *testing.T) {
		cluster, objects := newObjects()
		base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		reader := &shardingScaleInWorkloadMutatingReader{
			Client: base,
			mutateBeforeSecondWorkloadList: func(ctx context.Context) error {
				workload := &workloadsv1.InstanceSet{}
				if err := base.Get(ctx, client.ObjectKeyFromObject(objects[3]), workload); err != nil {
					return err
				}
				workload.Annotations = map[string]string{"changed": "true"}
				return base.Update(ctx, workload)
			},
		}

		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "InstanceSet snapshot changed")
	})

	t.Run("rejects an Instance snapshot that changes during inventory construction", func(t *testing.T) {
		cluster, objects := newObjects()
		instance := enableInstanceAPI(objects, 0)
		objects = append(objects, instance)
		base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		reader := &shardingScaleInInstanceMutatingReader{
			Client: base,
			mutateBeforeSecondInstanceList: func(ctx context.Context) error {
				current := &workloadsv1.Instance{}
				if err := base.Get(ctx, client.ObjectKeyFromObject(instance), current); err != nil {
					return err
				}
				current.Annotations = map[string]string{"changed": "true"}
				return base.Update(ctx, current)
			},
		}

		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "Instance snapshot changed")
	})

	t.Run("returns detached member, workload, and Pod copies", func(t *testing.T) {
		cluster, objects := newObjects()
		instance := enableInstanceAPI(objects, 0)
		objects = append(objects, instance)
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

		inventory, err := load(t, reader, cluster)
		require.NoError(t, err)
		members := slices.Concat(inventory.Leaving, inventory.Staying)
		var instanceMember *shardingScaleInSourceMember
		for i := range members {
			if members[i].Component.UID == objects[2].(*appsv1.Component).UID {
				instanceMember = &members[i]
				break
			}
		}
		require.NotNil(t, instanceMember)
		componentName := instanceMember.Component.Name
		workloadName := instanceMember.Workload.Name
		instanceName := instanceMember.Instances[0].Name
		podName := instanceMember.Pods[0].Pod.Name
		objects[2].SetName("mutated-component")
		objects[3].SetName("mutated-workload")
		objects[4].SetName("mutated-pod")
		instance.SetName("mutated-instance")
		require.Equal(t, componentName, instanceMember.Component.Name)
		require.Equal(t, workloadName, instanceMember.Workload.Name)
		require.Equal(t, instanceName, instanceMember.Instances[0].Name)
		require.Equal(t, podName, instanceMember.Pods[0].Pod.Name)
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

type shardingScaleInWorkloadMutatingReader struct {
	client.Client
	workloadListCalls              int
	mutateBeforeSecondWorkloadList func(context.Context) error
}

func (r *shardingScaleInWorkloadMutatingReader) List(ctx context.Context, list client.ObjectList,
	opts ...client.ListOption,
) error {
	if _, ok := list.(*workloadsv1.InstanceSetList); ok {
		r.workloadListCalls++
		if r.workloadListCalls == 2 && r.mutateBeforeSecondWorkloadList != nil {
			if err := r.mutateBeforeSecondWorkloadList(ctx); err != nil {
				return err
			}
		}
	}
	return r.Client.List(ctx, list, opts...)
}

var _ client.Reader = (*shardingScaleInWorkloadMutatingReader)(nil)

type shardingScaleInInstanceMutatingReader struct {
	client.Client
	instanceListCalls              int
	mutateBeforeSecondInstanceList func(context.Context) error
}

func (r *shardingScaleInInstanceMutatingReader) List(ctx context.Context, list client.ObjectList,
	opts ...client.ListOption,
) error {
	if _, ok := list.(*workloadsv1.InstanceList); ok {
		r.instanceListCalls++
		if r.instanceListCalls == 2 && r.mutateBeforeSecondInstanceList != nil {
			if err := r.mutateBeforeSecondInstanceList(ctx); err != nil {
				return err
			}
		}
	}
	return r.Client.List(ctx, list, opts...)
}

var _ client.Reader = (*shardingScaleInInstanceMutatingReader)(nil)
