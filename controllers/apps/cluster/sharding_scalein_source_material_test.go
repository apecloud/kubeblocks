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
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package cluster

import (
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

func TestBuildShardingScaleInSourceMaterial(t *testing.T) {
	inventory, bindings := newShardingScaleInSourceMaterialFixture()

	t.Run("builds exact members and component-scoped prerequisites", func(t *testing.T) {
		material, err := buildShardingScaleInSourceMaterial(inventory, bindings)
		require.NoError(t, err)
		require.Len(t, material.Leaving, 1)
		require.Len(t, material.Staying, 1)
		require.Equal(t, "cluster-shard-1", material.Leaving[0].ComponentName)
		require.Equal(t, "cluster-shard-0", material.Staying[0].ComponentName)
		require.Equal(t, appsv1.ShardingScaleInProofExecutor{
			PodName: "cluster-shard-0-0",
			PodUID:  "pod-0",
		}, material.ProofExecutor)
		require.Len(t, material.ExecutorPrerequisites, 10)

		kindCounts := map[string]int{}
		for _, prerequisite := range material.ExecutorPrerequisites {
			require.Equal(t, appsv1.ShardingScaleInPrerequisiteScopeComponent, prerequisite.Scope)
			require.NotEmpty(t, prerequisite.ComponentUID)
			require.True(t, isShardingScaleInSHA256(prerequisite.CriticalSpecDigest))
			require.True(t, isShardingScaleInSHA256(prerequisite.IdentityDigest))
			kindCounts[prerequisite.Kind]++
			switch prerequisite.Kind {
			case workloadsv1.InstanceSetKind, shardingScaleInInstanceKind:
				require.Equal(t, appsv1.ShardingScaleInPrerequisiteRolePodOwnerWorkload,
					prerequisite.Role)
			case "Service", "Endpoints", "EndpointSlice":
				require.Equal(t, appsv1.ShardingScaleInPrerequisiteRoleClusterDNS,
					prerequisite.Role)
			default:
				t.Fatalf("unexpected prerequisite kind %q", prerequisite.Kind)
			}
		}
		require.Equal(t, map[string]int{
			workloadsv1.InstanceSetKind: 2,
			shardingScaleInInstanceKind: 2,
			"Service":                   2,
			"Endpoints":                 2,
			"EndpointSlice":             2,
		}, kindCounts)
	})

	t.Run("requires one runtime binding for every exact Pod", func(t *testing.T) {
		_, err := buildShardingScaleInSourceMaterial(inventory, bindings[:1])
		require.ErrorContains(t, err, "runtime binding")

		extra := append(slices.Clone(bindings), shardingScaleInPodRuntimeBinding{
			PodUID:                "other-pod",
			AgentImageID:          "sha256:" + shardingScaleInTestDigestA,
			AgentProcessUID:       "0000000000000000000000-other",
			AgentCapabilityDigest: shardingScaleInTestDigestB,
		})
		_, err = buildShardingScaleInSourceMaterial(inventory, extra)
		require.ErrorContains(t, err, "runtime binding")
	})

	t.Run("requires DNS records to exactly exhaust their member snapshot", func(t *testing.T) {
		candidate := deepCopyShardingScaleInDNSInventory(inventory)
		candidate.Leaving[0].Member.Component.Generation++

		_, err := buildShardingScaleInSourceMaterial(candidate, bindings)
		require.ErrorContains(t, err, "exactly bind")

		candidate = deepCopyShardingScaleInDNSInventory(inventory)
		candidate.Members.Staying = nil
		_, err = buildShardingScaleInSourceMaterial(candidate, bindings)
		require.ErrorContains(t, err, "does not exhaust")
	})

	t.Run("omits an absent optional legacy Endpoints prerequisite", func(t *testing.T) {
		candidate := deepCopyShardingScaleInDNSInventory(inventory)
		candidate.Leaving[0].Endpoints = nil

		material, err := buildShardingScaleInSourceMaterial(candidate, bindings)
		require.NoError(t, err)
		require.Len(t, material.ExecutorPrerequisites, 9)
		require.NotContains(t, prerequisiteNames(material.ExecutorPrerequisites),
			"cluster-shard-1-headless/Endpoints")
	})

	t.Run("canonicalizes set-like source order", func(t *testing.T) {
		first := deepCopyShardingScaleInDNSInventory(inventory)
		second := deepCopyShardingScaleInDNSInventory(inventory)
		for i := range first.Leaving {
			addShardingScaleInSourceMaterialSlice(&first.Leaving[i], "a")
			addShardingScaleInSourceMaterialSlice(&first.Leaving[i], "b")
		}
		for i := range second.Leaving {
			addShardingScaleInSourceMaterialSlice(&second.Leaving[i], "b")
			addShardingScaleInSourceMaterialSlice(&second.Leaving[i], "a")
			slices.Reverse(second.Leaving[i].Service.OwnerReferences)
			slices.Reverse(second.Leaving[i].Member.Workload.OwnerReferences)
			slices.Reverse(second.Members.Leaving[i].Workload.OwnerReferences)
		}
		reversedBindings := slices.Clone(bindings)
		slices.Reverse(reversedBindings)

		firstMaterial, err := buildShardingScaleInSourceMaterial(first, bindings)
		require.NoError(t, err)
		secondMaterial, err := buildShardingScaleInSourceMaterial(second, reversedBindings)
		require.NoError(t, err)
		require.Equal(t, firstMaterial, secondMaterial)
	})

	t.Run("canonicalizes Service port map-list order", func(t *testing.T) {
		first := deepCopyShardingScaleInDNSInventory(inventory)
		second := deepCopyShardingScaleInDNSInventory(inventory)
		ports := []corev1.ServicePort{
			{Name: "client", Port: 6379, Protocol: corev1.ProtocolTCP},
			{Name: "gossip", Port: 16379, Protocol: corev1.ProtocolTCP},
		}
		first.Leaving[0].Service.Spec.Ports = slices.Clone(ports)
		second.Leaving[0].Service.Spec.Ports = slices.Clone(ports)
		slices.Reverse(second.Leaving[0].Service.Spec.Ports)

		firstMaterial, err := buildShardingScaleInSourceMaterial(first, bindings)
		require.NoError(t, err)
		secondMaterial, err := buildShardingScaleInSourceMaterial(second, bindings)
		require.NoError(t, err)
		require.Equal(t, firstMaterial, secondMaterial)
		require.Equal(t, "gossip", second.Leaving[0].Service.Spec.Ports[0].Name)
	})

	t.Run("canonicalizes Component and workload structural map lists", func(t *testing.T) {
		first := deepCopyShardingScaleInDNSInventory(inventory)
		second := deepCopyShardingScaleInDNSInventory(inventory)
		populateShardingScaleInSourceSpecMapLists(&first.Leaving[0].Member)
		populateShardingScaleInSourceSpecMapLists(&second.Leaving[0].Member)
		reverseShardingScaleInSourceSpecMapLists(&second.Leaving[0].Member)
		first.Members.Leaving[0] = first.Leaving[0].Member
		second.Members.Leaving[0] = second.Leaving[0].Member

		firstMaterial, err := buildShardingScaleInSourceMaterial(first, bindings)
		require.NoError(t, err)
		secondMaterial, err := buildShardingScaleInSourceMaterial(second, bindings)
		require.NoError(t, err)
		require.Equal(t, firstMaterial, secondMaterial)
	})

	t.Run("binds critical source projection bytes", func(t *testing.T) {
		first, err := buildShardingScaleInSourceMaterial(inventory, bindings)
		require.NoError(t, err)

		serviceChanged := deepCopyShardingScaleInDNSInventory(inventory)
		serviceChanged.Leaving[0].Service.Spec.Ports = []corev1.ServicePort{{
			Name: "client",
			Port: 6379,
		}}
		second, err := buildShardingScaleInSourceMaterial(serviceChanged, bindings)
		require.NoError(t, err)
		require.NotEqual(t,
			prerequisiteDigest(first.ExecutorPrerequisites, "cluster-shard-1-headless", "Service"),
			prerequisiteDigest(second.ExecutorPrerequisites, "cluster-shard-1-headless", "Service"))

		sliceChanged := deepCopyShardingScaleInDNSInventory(inventory)
		sliceChanged.Leaving[0].EndpointSlices[0].Endpoints[0].Addresses[0] = "10.0.0.99"
		third, err := buildShardingScaleInSourceMaterial(sliceChanged, bindings)
		require.NoError(t, err)
		require.NotEqual(t,
			prerequisiteDigest(first.ExecutorPrerequisites, "cluster-shard-1-headless-ipv4",
				"EndpointSlice"),
			prerequisiteDigest(third.ExecutorPrerequisites, "cluster-shard-1-headless-ipv4",
				"EndpointSlice"))
	})

	t.Run("accepts 512 protected objects and rejects 513 without truncation", func(t *testing.T) {
		candidate := deepCopyShardingScaleInDNSInventory(inventory)
		for i := 0; i < 498; i++ {
			addShardingScaleInSourceMaterialSlice(&candidate.Staying[0], fmt.Sprintf("extra-%03d", i))
		}
		material, err := buildShardingScaleInSourceMaterial(candidate, bindings)
		require.NoError(t, err)
		require.Equal(t, 512,
			len(material.Leaving)+len(material.Staying)+2+len(material.ExecutorPrerequisites))

		addShardingScaleInSourceMaterialSlice(&candidate.Staying[0], "overflow")
		_, err = buildShardingScaleInSourceMaterial(candidate, bindings)
		require.ErrorContains(t, err, "512")
	})
}

func TestDigestShardingScaleInCriticalProjectionCanonicalizesOwnerReferenceBooleans(
	t *testing.T,
) {
	firstValues := [2]bool{false, true}
	secondValues := [2]bool{true, false}
	newOwnerReferences := func(falseValue, trueValue *bool) []metav1.OwnerReference {
		return []metav1.OwnerReference{
			{
				APIVersion: "apps/v1",
				Kind:       "InstanceSet",
				Name:       "owner",
				UID:        "owner-uid",
				Controller: falseValue,
			},
			{
				APIVersion: "apps/v1",
				Kind:       "InstanceSet",
				Name:       "owner",
				UID:        "owner-uid",
				Controller: trueValue,
			},
		}
	}

	first, err := digestShardingScaleInCriticalProjection(
		newOwnerReferences(&firstValues[0], &firstValues[1]), "projection")
	require.NoError(t, err)
	second, err := digestShardingScaleInCriticalProjection(
		newOwnerReferences(&secondValues[1], &secondValues[0]), "projection")
	require.NoError(t, err)
	require.Equal(t, first, second)
}

func TestCanonicalizeShardingScaleInSourceSpecMapLists(t *testing.T) {
	t.Run("Component", func(t *testing.T) {
		first := appsv1.Component{
			Spec: appsv1.ComponentSpec{
				Resources: corev1.ResourceRequirements{
					Claims: shardingScaleInTestResourceClaims(),
				},
				Instances: []appsv1.InstanceTemplate{
					{
						Name:      "ordered-z",
						Resources: &corev1.ResourceRequirements{Claims: shardingScaleInTestResourceClaims()},
					},
					{
						Name:      "ordered-a",
						Resources: &corev1.ResourceRequirements{Claims: shardingScaleInTestResourceClaims()},
					},
				},
				Services: []appsv1.ComponentService{
					{Service: appsv1.Service{
						Name: "ordered-z",
						Spec: corev1.ServiceSpec{Ports: shardingScaleInTestServicePorts()},
					}},
					{Service: appsv1.Service{
						Name: "ordered-a",
						Spec: corev1.ServiceSpec{Ports: shardingScaleInTestServicePorts()},
					}},
				},
			},
		}
		second := first.DeepCopy()
		slices.Reverse(second.Spec.Resources.Claims)
		for i := range second.Spec.Instances {
			slices.Reverse(second.Spec.Instances[i].Resources.Claims)
		}
		for i := range second.Spec.Services {
			slices.Reverse(second.Spec.Services[i].Spec.Ports)
		}

		canonicalizeShardingScaleInComponentSpec(&first.Spec)
		canonicalizeShardingScaleInComponentSpec(&second.Spec)
		require.Equal(t, first.Spec, second.Spec)
		require.Equal(t, []string{"ordered-z", "ordered-a"}, []string{
			first.Spec.Instances[0].Name,
			first.Spec.Instances[1].Name,
		})
	})

	t.Run("InstanceSet", func(t *testing.T) {
		first := workloadsv1.InstanceSet{
			Spec: workloadsv1.InstanceSetSpec{
				Instances: []workloadsv1.InstanceTemplate{
					{
						Name:      "mapped-z",
						Resources: &corev1.ResourceRequirements{Claims: shardingScaleInTestResourceClaims()},
					},
					{
						Name:      "mapped-a",
						Resources: &corev1.ResourceRequirements{Claims: shardingScaleInTestResourceClaims()},
					},
				},
				Template: corev1.PodTemplateSpec{Spec: shardingScaleInTestPodSpec()},
			},
		}
		second := first.DeepCopy()
		slices.Reverse(second.Spec.Instances)
		for i := range second.Spec.Instances {
			slices.Reverse(second.Spec.Instances[i].Resources.Claims)
		}
		reverseShardingScaleInTestPodSpecMapLists(&second.Spec.Template.Spec)

		canonicalizeShardingScaleInInstanceSetSpec(&first.Spec)
		canonicalizeShardingScaleInInstanceSetSpec(&second.Spec)
		require.Equal(t, first.Spec, second.Spec)
		require.Equal(t, []string{"ordered-z", "ordered-a"}, []string{
			first.Spec.Template.Spec.Containers[0].Name,
			first.Spec.Template.Spec.Containers[1].Name,
		})
		require.Equal(t, []string{"second", "first"},
			first.Spec.Template.Spec.Containers[0].Args)
	})

	t.Run("Instance", func(t *testing.T) {
		first := workloadsv1.Instance{
			Spec: workloadsv1.InstanceSpec{
				Template: corev1.PodTemplateSpec{Spec: shardingScaleInTestPodSpec()},
				InstanceAssistantObjects: []workloadsv1.InstanceAssistantObject{{
					Service: &corev1.Service{
						Spec: corev1.ServiceSpec{Ports: shardingScaleInTestServicePorts()},
						Status: corev1.ServiceStatus{Conditions: []metav1.Condition{
							{Type: "z"},
							{Type: "a"},
						}},
					},
				}},
			},
		}
		second := first.DeepCopy()
		reverseShardingScaleInTestPodSpecMapLists(&second.Spec.Template.Spec)
		slices.Reverse(second.Spec.InstanceAssistantObjects[0].Service.Spec.Ports)
		slices.Reverse(second.Spec.InstanceAssistantObjects[0].Service.Status.Conditions)

		canonicalizeShardingScaleInInstanceSpec(&first.Spec)
		canonicalizeShardingScaleInInstanceSpec(&second.Spec)
		require.Equal(t, first.Spec, second.Spec)
		require.Equal(t, []string{"ordered-z", "ordered-a"}, []string{
			first.Spec.Template.Spec.Containers[0].Name,
			first.Spec.Template.Spec.Containers[1].Name,
		})
		require.Equal(t, []string{"second", "first"},
			first.Spec.Template.Spec.Containers[0].Args)
	})
}

func newShardingScaleInSourceMaterialFixture() (
	*shardingScaleInDNSInventory, []shardingScaleInPodRuntimeBinding,
) {
	newRecord := func(index int) shardingScaleInSourceDNS {
		componentName := fmt.Sprintf("cluster-shard-%d", index)
		component := appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:  "default",
				Name:       componentName,
				UID:        types.UID(fmt.Sprintf("component-%d", index)),
				Generation: int64(index + 1),
			},
			Spec: appsv1.ComponentSpec{
				CompDef:  "valkey",
				Replicas: 1,
			},
		}
		workload := workloadsv1.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      componentName,
				UID:       types.UID(fmt.Sprintf("instanceset-%d", index)),
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(
					&component, appsv1.GroupVersion.WithKind(appsv1.ComponentKind))},
			},
			Spec: workloadsv1.InstanceSetSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app": componentName,
				}},
			},
		}
		instance := workloadsv1.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      componentName + "-0",
				UID:       types.UID(fmt.Sprintf("instance-%d", index)),
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(
					&workload, workloadsv1.GroupVersion.WithKind(workloadsv1.InstanceSetKind))},
			},
		}
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      componentName + "-0",
				UID:       types.UID(fmt.Sprintf("pod-%d", index)),
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(
					&instance, workloadsv1.GroupVersion.WithKind(shardingScaleInInstanceKind))},
			},
			Spec: corev1.PodSpec{
				Hostname:  componentName + "-0",
				Subdomain: componentName + "-headless",
			},
		}
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      componentName + "-headless",
				UID:       types.UID(fmt.Sprintf("service-%d", index)),
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(
					&workload, workloadsv1.GroupVersion.WithKind(workloadsv1.InstanceSetKind))},
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: corev1.ClusterIPNone,
				Selector:  map[string]string{"app": componentName},
			},
		}
		endpoints := corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      service.Name,
				UID:       types.UID(fmt.Sprintf("endpoints-%d", index)),
			},
			Subsets: []corev1.EndpointSubset{{
				Addresses: []corev1.EndpointAddress{{
					IP:       fmt.Sprintf("10.0.0.%d", index+1),
					Hostname: pod.Name,
					TargetRef: &corev1.ObjectReference{
						Kind:      "Pod",
						Namespace: pod.Namespace,
						Name:      pod.Name,
						UID:       pod.UID,
					},
				}},
			}},
		}
		slice := discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: service.Namespace,
				Name:      service.Name + "-ipv4",
				UID:       types.UID(fmt.Sprintf("slice-%d", index)),
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(
					&service, corev1.SchemeGroupVersion.WithKind("Service"))},
			},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints: []discoveryv1.Endpoint{{
				Addresses: []string{fmt.Sprintf("10.0.0.%d", index+1)},
				Hostname:  &pod.Name,
				TargetRef: &corev1.ObjectReference{
					Kind:      "Pod",
					Namespace: pod.Namespace,
					Name:      pod.Name,
					UID:       pod.UID,
				},
			}},
		}
		return shardingScaleInSourceDNS{
			Member: shardingScaleInSourceMember{
				Component:         component,
				ShardTemplateName: "valkey",
				Workload:          workload,
				Instances:         []workloadsv1.Instance{instance},
				Pods: []shardingScaleInSourcePod{{
					Pod:  pod,
					FQDN: pod.Name + "." + pod.Spec.Subdomain + ".default.svc",
				}},
			},
			Service:        service,
			Endpoints:      &endpoints,
			EndpointSlices: []discoveryv1.EndpointSlice{slice},
		}
	}

	leaving := newRecord(1)
	staying := newRecord(0)
	members := &shardingScaleInMemberInventory{
		Leaving: []shardingScaleInSourceMember{leaving.Member},
		Staying: []shardingScaleInSourceMember{staying.Member},
		Topology: &shardingScaleInTopologyInventory{
			Cluster: &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
				Namespace:  "default",
				Name:       "cluster",
				UID:        "cluster-uid",
				Generation: 1,
			}},
		},
	}
	return &shardingScaleInDNSInventory{
			Members: members,
			Leaving: []shardingScaleInSourceDNS{leaving},
			Staying: []shardingScaleInSourceDNS{staying},
		}, []shardingScaleInPodRuntimeBinding{
			{
				PodUID:                "pod-1",
				AgentImageID:          "sha256:" + shardingScaleInTestDigestA,
				AgentProcessUID:       "000000000000000000000000-pod-1",
				AgentCapabilityDigest: shardingScaleInTestDigestB,
			},
			{
				PodUID:                "pod-0",
				AgentImageID:          "sha256:" + shardingScaleInTestDigestA,
				AgentProcessUID:       "000000000000000000000000-pod-0",
				AgentCapabilityDigest: shardingScaleInTestDigestB,
			},
		}
}

func addShardingScaleInSourceMaterialSlice(record *shardingScaleInSourceDNS, suffix string) {
	slice := record.EndpointSlices[0].DeepCopy()
	slice.Name = record.Service.Name + "-" + suffix
	slice.UID = types.UID("slice-" + suffix)
	record.EndpointSlices = append(record.EndpointSlices, *slice)
}

func prerequisiteNames(prerequisites []appsv1.ShardingScaleInExecutorPrerequisite) []string {
	names := make([]string, 0, len(prerequisites))
	for _, prerequisite := range prerequisites {
		names = append(names, prerequisite.Name+"/"+prerequisite.Kind)
	}
	return names
}

func prerequisiteDigest(prerequisites []appsv1.ShardingScaleInExecutorPrerequisite,
	name, kind string,
) string {
	for _, prerequisite := range prerequisites {
		if prerequisite.Name == name && prerequisite.Kind == kind {
			return prerequisite.CriticalSpecDigest
		}
	}
	return ""
}

func shardingScaleInTestResourceClaims() []corev1.ResourceClaim {
	return []corev1.ResourceClaim{{Name: "z"}, {Name: "a"}}
}

func shardingScaleInTestServicePorts() []corev1.ServicePort {
	return []corev1.ServicePort{
		{Name: "gossip", Port: 16379, Protocol: corev1.ProtocolTCP},
		{Name: "client", Port: 6379, Protocol: corev1.ProtocolTCP},
	}
}

func shardingScaleInTestContainerPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: "gossip", ContainerPort: 16379, Protocol: corev1.ProtocolTCP},
		{Name: "client", ContainerPort: 6379, Protocol: corev1.ProtocolTCP},
	}
}

func shardingScaleInTestPodSpec() corev1.PodSpec {
	newContainer := func(name string) corev1.Container {
		return corev1.Container{
			Name:  name,
			Args:  []string{"second", "first"},
			Ports: shardingScaleInTestContainerPorts(),
			Resources: corev1.ResourceRequirements{
				Claims: shardingScaleInTestResourceClaims(),
			},
		}
	}
	return corev1.PodSpec{
		InitContainers: []corev1.Container{newContainer("init")},
		Containers: []corev1.Container{
			newContainer("ordered-z"),
			newContainer("ordered-a"),
		},
		EphemeralContainers: []corev1.EphemeralContainer{{
			EphemeralContainerCommon: corev1.EphemeralContainerCommon{
				Name:      "debug",
				Ports:     shardingScaleInTestContainerPorts(),
				Resources: corev1.ResourceRequirements{Claims: shardingScaleInTestResourceClaims()},
			},
		}},
		ResourceClaims:  []corev1.PodResourceClaim{{Name: "z"}, {Name: "a"}},
		SchedulingGates: []corev1.PodSchedulingGate{{Name: "z"}, {Name: "a"}},
		TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
			{
				TopologyKey:       "z",
				WhenUnsatisfiable: corev1.ScheduleAnyway,
			},
			{
				TopologyKey:       "a",
				WhenUnsatisfiable: corev1.DoNotSchedule,
			},
		},
	}
}

func reverseShardingScaleInTestPodSpecMapLists(spec *corev1.PodSpec) {
	for i := range spec.InitContainers {
		slices.Reverse(spec.InitContainers[i].Ports)
		slices.Reverse(spec.InitContainers[i].Resources.Claims)
	}
	for i := range spec.Containers {
		slices.Reverse(spec.Containers[i].Ports)
		slices.Reverse(spec.Containers[i].Resources.Claims)
	}
	for i := range spec.EphemeralContainers {
		slices.Reverse(spec.EphemeralContainers[i].Ports)
		slices.Reverse(spec.EphemeralContainers[i].Resources.Claims)
	}
	slices.Reverse(spec.ResourceClaims)
	slices.Reverse(spec.SchedulingGates)
	slices.Reverse(spec.TopologySpreadConstraints)
}

func populateShardingScaleInSourceSpecMapLists(member *shardingScaleInSourceMember) {
	member.Component.Spec.Resources.Claims = shardingScaleInTestResourceClaims()
	member.Component.Spec.Instances = []appsv1.InstanceTemplate{{
		Name:      "ordered",
		Resources: &corev1.ResourceRequirements{Claims: shardingScaleInTestResourceClaims()},
	}}
	member.Component.Spec.Services = []appsv1.ComponentService{{Service: appsv1.Service{
		Name: "ordered",
		Spec: corev1.ServiceSpec{Ports: shardingScaleInTestServicePorts()},
	}}}
	member.Workload.Spec.Instances = []workloadsv1.InstanceTemplate{
		{
			Name:      "mapped-z",
			Resources: &corev1.ResourceRequirements{Claims: shardingScaleInTestResourceClaims()},
		},
		{
			Name:      "mapped-a",
			Resources: &corev1.ResourceRequirements{Claims: shardingScaleInTestResourceClaims()},
		},
	}
	member.Workload.Spec.Template.Spec = shardingScaleInTestPodSpec()
	member.Instances[0].Spec.Template.Spec = shardingScaleInTestPodSpec()
	member.Instances[0].Spec.InstanceAssistantObjects = []workloadsv1.InstanceAssistantObject{{
		Service: &corev1.Service{
			Spec: corev1.ServiceSpec{Ports: shardingScaleInTestServicePorts()},
			Status: corev1.ServiceStatus{Conditions: []metav1.Condition{
				{Type: "z"},
				{Type: "a"},
			}},
		},
	}}
}

func reverseShardingScaleInSourceSpecMapLists(member *shardingScaleInSourceMember) {
	slices.Reverse(member.Component.Spec.Resources.Claims)
	slices.Reverse(member.Component.Spec.Instances[0].Resources.Claims)
	slices.Reverse(member.Component.Spec.Services[0].Spec.Ports)
	slices.Reverse(member.Workload.Spec.Instances)
	for i := range member.Workload.Spec.Instances {
		slices.Reverse(member.Workload.Spec.Instances[i].Resources.Claims)
	}
	reverseShardingScaleInTestPodSpecMapLists(&member.Workload.Spec.Template.Spec)
	reverseShardingScaleInTestPodSpecMapLists(&member.Instances[0].Spec.Template.Spec)
	slices.Reverse(member.Instances[0].Spec.InstanceAssistantObjects[0].Service.Spec.Ports)
	slices.Reverse(member.Instances[0].Spec.InstanceAssistantObjects[0].Service.Status.Conditions)
}
