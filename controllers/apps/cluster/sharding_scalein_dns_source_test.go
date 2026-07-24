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
	discoveryv1 "k8s.io/api/discovery/v1"
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
)

func TestLoadFreshShardingScaleInDNS(t *testing.T) {
	const (
		namespace    = "default"
		clusterName  = "cluster"
		shardingName = "shard"
	)

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, discoveryv1.AddToScheme(scheme))
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
			workloadLabels := constant.GetCompLabels(clusterName, shortName, labels)
			selector := map[string]string{
				constant.AppInstanceLabelKey:          clusterName,
				constant.KBAppComponentLabelKey:       shortName,
				instanceset.WorkloadsInstanceLabelKey: fullName,
			}
			workload := &workloadsv1.InstanceSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       namespace,
					Name:            fullName,
					UID:             types.UID(fmt.Sprintf("instanceset-uid-%d", i)),
					ResourceVersion: fmt.Sprintf("%d", 40+i),
					Generation:      int64(i + 1),
					Labels:          workloadLabels,
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(
						comp, appsv1.SchemeGroupVersion.WithKind(appsv1.ComponentKind))},
				},
				Spec: workloadsv1.InstanceSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: selector},
				},
			}
			podLabels := constant.GetCompLabels(clusterName, shortName, labels)
			for key, value := range selector {
				podLabels[key] = value
			}
			podLabels[instanceset.WorkloadsManagedByLabelKey] = workloadsv1.InstanceSetKind
			podLabels[constant.KBAppReleasePhaseKey] = constant.ReleasePhaseStable
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
				Status: corev1.PodStatus{
					PodIP: fmt.Sprintf("10.0.0.%d", i+1),
					PodIPs: []corev1.PodIP{{
						IP: fmt.Sprintf("10.0.0.%d", i+1),
					}},
				},
			}
			serviceSelector := make(map[string]string, len(selector)+1)
			for key, value := range selector {
				serviceSelector[key] = value
			}
			serviceSelector[constant.KBAppReleasePhaseKey] = constant.ReleasePhaseStable
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       namespace,
					Name:            fullName + "-headless",
					UID:             types.UID(fmt.Sprintf("service-uid-%d", i)),
					ResourceVersion: fmt.Sprintf("%d", 60+i),
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(
						workload, workloadsv1.GroupVersion.WithKind(workloadsv1.InstanceSetKind))},
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					ClusterIP:                corev1.ClusterIPNone,
					ClusterIPs:               []string{corev1.ClusterIPNone},
					IPFamilies:               []corev1.IPFamily{corev1.IPv4Protocol},
					IPFamilyPolicy:           ptr.To(corev1.IPFamilyPolicySingleStack),
					PublishNotReadyAddresses: true,
					Selector:                 serviceSelector,
				},
			}
			endpoints := &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       namespace,
					Name:            service.Name,
					UID:             types.UID(fmt.Sprintf("endpoints-uid-%d", i)),
					ResourceVersion: fmt.Sprintf("%d", 70+i),
				},
				Subsets: []corev1.EndpointSubset{{
					Addresses: []corev1.EndpointAddress{{
						IP:       pod.Status.PodIP,
						Hostname: pod.Name,
						TargetRef: &corev1.ObjectReference{
							Kind:      "Pod",
							Namespace: namespace,
							Name:      pod.Name,
							UID:       pod.UID,
						},
					}},
				}},
			}
			slice := &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       namespace,
					Name:            service.Name + "-ipv4",
					UID:             types.UID(fmt.Sprintf("endpointslice-uid-%d", i)),
					ResourceVersion: fmt.Sprintf("%d", 80+i),
					Labels: map[string]string{
						discoveryv1.LabelServiceName: service.Name,
					},
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(
						service, corev1.SchemeGroupVersion.WithKind("Service"))},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{{
					Addresses: []string{pod.Status.PodIP},
					Hostname:  ptr.To(pod.Name),
					TargetRef: &corev1.ObjectReference{
						Kind:      "Pod",
						Namespace: namespace,
						Name:      pod.Name,
						UID:       pod.UID,
					},
				}},
			}
			objects = append(objects, comp, workload, pod, service, endpoints, slice)
		}
		return cluster, objects
	}
	load := func(t *testing.T, reader client.Reader, cluster *appsv1.Cluster) (
		*shardingScaleInDNSInventory, error,
	) {
		t.Helper()
		return loadFreshShardingScaleInDNS(context.Background(), reader,
			client.ObjectKeyFromObject(cluster), cluster.UID, shardingName)
	}
	memberObjects := func(objects []client.Object, memberIndex int) (
		*workloadsv1.InstanceSet, *corev1.Pod, *corev1.Service,
		*corev1.Endpoints, *discoveryv1.EndpointSlice,
	) {
		offset := 2 + memberIndex*6
		return objects[offset+1].(*workloadsv1.InstanceSet),
			objects[offset+2].(*corev1.Pod),
			objects[offset+3].(*corev1.Service),
			objects[offset+4].(*corev1.Endpoints),
			objects[offset+5].(*discoveryv1.EndpointSlice)
	}

	t.Run("builds exact component-scoped DNS records", func(t *testing.T) {
		cluster, objects := newObjects()
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

		inventory, err := load(t, reader, cluster)
		require.NoError(t, err)
		require.Len(t, inventory.Leaving, 2)
		require.Len(t, inventory.Staying, 3)
		for _, record := range append(inventory.Leaving, inventory.Staying...) {
			require.Equal(t, record.Member.Workload.UID, metav1.GetControllerOf(&record.Service).UID)
			require.Equal(t, record.Service.Name, record.Member.Pods[0].Pod.Spec.Subdomain)
			require.NotNil(t, record.Endpoints)
			require.Len(t, record.EndpointSlices, 1)
			require.Equal(t, record.Service.UID, metav1.GetControllerOf(&record.EndpointSlices[0]).UID)
		}
	})

	t.Run("accepts an absent legacy Endpoints object", func(t *testing.T) {
		cluster, objects := newObjects()
		filtered := make([]client.Object, 0, len(objects))
		for _, object := range objects {
			if _, ok := object.(*corev1.Endpoints); !ok {
				filtered = append(filtered, object)
			}
		}
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(filtered...).Build()

		inventory, err := load(t, reader, cluster)
		require.NoError(t, err)
		for _, record := range append(inventory.Leaving, inventory.Staying...) {
			require.Nil(t, record.Endpoints)
		}
	})

	t.Run("rejects a non-canonical Service identity or selector", func(t *testing.T) {
		cluster, objects := newObjects()
		_, _, service, _, _ := memberObjects(objects, 0)
		service.OwnerReferences[0].UID = "other-workload"
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "Service controller owner")

		cluster, objects = newObjects()
		_, _, service, _, _ = memberObjects(objects, 0)
		service.Spec.PublishNotReadyAddresses = false
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "headless Service")

		cluster, objects = newObjects()
		_, _, service, _, _ = memberObjects(objects, 0)
		service.Spec.Selector["extra"] = "label"
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "Service selector")

		cluster, objects = newObjects()
		workload, _, _, _, _ := memberObjects(objects, 0)
		workload.Spec.Selector.MatchExpressions = []metav1.LabelSelectorRequirement{{
			Key:      "tier",
			Operator: metav1.LabelSelectorOpExists,
		}}
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "exactly representable by a Service")
	})

	t.Run("rejects a conflicting release-phase selector and accepts canonical forms", func(t *testing.T) {
		cluster, objects := newObjects()
		workload, _, _, _, _ := memberObjects(objects, 0)
		workload.Spec.Selector.MatchLabels[constant.KBAppReleasePhaseKey] = "canary"
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "release-phase")

		cluster, objects = newObjects()
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.NoError(t, err)

		cluster, objects = newObjects()
		workload, _, _, _, _ = memberObjects(objects, 0)
		workload.Spec.Selector.MatchLabels[constant.KBAppReleasePhaseKey] = constant.ReleasePhaseStable
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.NoError(t, err)
	})

	t.Run("rejects incomplete Endpoints Pod coverage", func(t *testing.T) {
		cluster, objects := newObjects()
		_, _, _, endpoints, _ := memberObjects(objects, 0)
		endpoints.Subsets[0].Addresses[0].TargetRef.UID = "other-pod"
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "Endpoints")

		cluster, objects = newObjects()
		_, pod, _, endpoints, _ := memberObjects(objects, 0)
		endpoints.Subsets[0].Addresses = append(endpoints.Subsets[0].Addresses,
			endpoints.Subsets[0].Addresses[0])
		endpoints.Subsets[0].Addresses[1].Hostname = pod.Name
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "Endpoints")
	})

	t.Run("rejects every mismatched EndpointSlice association and coverage path", func(t *testing.T) {
		cluster, objects := newObjects()
		_, _, _, _, slice := memberObjects(objects, 0)
		slice.Labels[discoveryv1.LabelServiceName] = "other-service"
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "EndpointSlice service-name label")

		cluster, objects = newObjects()
		_, _, _, _, slice = memberObjects(objects, 0)
		slice.OwnerReferences[0].UID = "other-service"
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "EndpointSlice controller owner")

		cluster, objects = newObjects()
		_, _, _, _, slice = memberObjects(objects, 0)
		slice.Endpoints[0].Hostname = ptr.To("other-hostname")
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "EndpointSlice")

		cluster, objects = newObjects()
		_, _, _, _, slice = memberObjects(objects, 0)
		duplicate := slice.DeepCopy()
		duplicate.Name += "-duplicate"
		duplicate.UID = "duplicate-slice-uid"
		duplicate.ResourceVersion = "99"
		objects = append(objects, duplicate)
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "EndpointSlice")
	})

	t.Run("accepts exact dual-stack EndpointSlices", func(t *testing.T) {
		cluster, objects := newObjects()
		_, pod, service, _, slice := memberObjects(objects, 0)
		service.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}
		service.Spec.IPFamilyPolicy = ptr.To(corev1.IPFamilyPolicyRequireDualStack)
		pod.Status.PodIPs = append(pod.Status.PodIPs, corev1.PodIP{IP: "fd00::1"})
		ipv6 := slice.DeepCopy()
		ipv6.Name = service.Name + "-ipv6"
		ipv6.UID = "ipv6-slice-uid"
		ipv6.ResourceVersion = "99"
		ipv6.AddressType = discoveryv1.AddressTypeIPv6
		ipv6.Endpoints[0].Addresses = []string{"fd00::1"}
		objects = append(objects, ipv6)
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

		inventory, err := load(t, reader, cluster)
		require.NoError(t, err)
		found := false
		for _, record := range append(inventory.Leaving, inventory.Staying...) {
			if record.Service.Name == service.Name {
				require.Len(t, record.EndpointSlices, 2)
				found = true
				break
			}
		}
		require.True(t, found)
	})

	t.Run("selects legacy Endpoints addresses by the Service primary IP family", func(t *testing.T) {
		cluster, objects := newObjects()
		_, pod, service, endpoints, slice := memberObjects(objects, 0)
		service.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv6Protocol, corev1.IPv4Protocol}
		service.Spec.IPFamilyPolicy = ptr.To(corev1.IPFamilyPolicyRequireDualStack)
		pod.Status.PodIPs = append(pod.Status.PodIPs, corev1.PodIP{IP: "fd00::1"})
		endpoints.Subsets[0].Addresses[0].IP = "fd00::1"
		ipv6 := slice.DeepCopy()
		ipv6.Name = service.Name + "-ipv6"
		ipv6.UID = "ipv6-slice-uid"
		ipv6.ResourceVersion = "99"
		ipv6.AddressType = discoveryv1.AddressTypeIPv6
		ipv6.Endpoints[0].Addresses = []string{"fd00::1"}
		objects = append(objects, ipv6)
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

		_, err := load(t, reader, cluster)
		require.NoError(t, err)

		cluster, objects = newObjects()
		_, pod, service, endpoints, slice = memberObjects(objects, 0)
		service.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv6Protocol, corev1.IPv4Protocol}
		service.Spec.IPFamilyPolicy = ptr.To(corev1.IPFamilyPolicyRequireDualStack)
		pod.Status.PodIPs = append(pod.Status.PodIPs, corev1.PodIP{IP: "fd00::1"})
		endpoints.Subsets[0].Addresses[0].IP = pod.Status.PodIP
		ipv6 = slice.DeepCopy()
		ipv6.Name = service.Name + "-ipv6"
		ipv6.UID = "ipv6-slice-uid"
		ipv6.ResourceVersion = "99"
		ipv6.AddressType = discoveryv1.AddressTypeIPv6
		ipv6.Endpoints[0].Addresses = []string{"fd00::1"}
		objects = append(objects, ipv6)
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "Service primary IP family")
	})

	t.Run("rejects inconsistent Pod address identity", func(t *testing.T) {
		cluster, objects := newObjects()
		_, pod, _, _, _ := memberObjects(objects, 0)
		pod.Status.PodIPs[0].IP = "10.0.0.200"
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "primary PodIP does not match")

		cluster, objects = newObjects()
		_, pod, _, _, _ = memberObjects(objects, 0)
		pod.Status.PodIPs = append(pod.Status.PodIPs, corev1.PodIP{IP: "10.0.0.200"})
		reader = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		_, err = load(t, reader, cluster)
		require.ErrorContains(t, err, "multiple addresses for IP family")
	})

	t.Run("rejects Service snapshot drift", func(t *testing.T) {
		cluster, objects := newObjects()
		_, _, service, _, _ := memberObjects(objects, 0)
		base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		reader := &shardingScaleInServiceMutatingReader{
			Client: base,
			mutateBeforeSecondServiceGet: func(ctx context.Context) error {
				current := &corev1.Service{}
				if err := base.Get(ctx, client.ObjectKeyFromObject(service), current); err != nil {
					return err
				}
				current.Annotations = map[string]string{"changed": "true"}
				return base.Update(ctx, current)
			},
		}

		_, err := load(t, reader, cluster)
		require.ErrorContains(t, err, "Service snapshot changed")
	})

	t.Run("returns detached DNS copies", func(t *testing.T) {
		cluster, objects := newObjects()
		_, _, service, endpoints, slice := memberObjects(objects, 0)
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

		inventory, err := load(t, reader, cluster)
		require.NoError(t, err)
		record := &inventory.Leaving[0]
		serviceName := record.Service.Name
		endpointsName := record.Endpoints.Name
		sliceName := record.EndpointSlices[0].Name
		service.Name = "mutated-service"
		endpoints.Name = "mutated-endpoints"
		slice.Name = "mutated-slice"
		require.Equal(t, serviceName, record.Service.Name)
		require.Equal(t, endpointsName, record.Endpoints.Name)
		require.Equal(t, sliceName, record.EndpointSlices[0].Name)
	})
}

type shardingScaleInServiceMutatingReader struct {
	client.Client
	serviceGetCalls              int
	mutateBeforeSecondServiceGet func(context.Context) error
}

func (r *shardingScaleInServiceMutatingReader) Get(ctx context.Context, key client.ObjectKey,
	object client.Object, opts ...client.GetOption,
) error {
	if _, ok := object.(*corev1.Service); ok {
		r.serviceGetCalls++
		if r.serviceGetCalls == 6 && r.mutateBeforeSecondServiceGet != nil {
			if err := r.mutateBeforeSecondServiceGet(ctx); err != nil {
				return err
			}
		}
	}
	return r.Client.Get(ctx, key, object, opts...)
}

var _ client.Reader = (*shardingScaleInServiceMutatingReader)(nil)
