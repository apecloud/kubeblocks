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
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

var errInvalidShardingScaleInSourceMaterial = errors.New("invalid sharding scale-in source material")

const shardingScaleInSourceProjectionVersionV1 = "kb.sharding.scalein.source-projection/v1"

type shardingScaleInPodRuntimeBinding struct {
	PodUID                types.UID
	AgentImageID          string
	AgentProcessUID       string
	AgentCapabilityDigest string
}

// shardingScaleInSourceMaterial is the source-owned part of immutable plan
// material. Request authority, deletion-guard identity, and action identity are
// deliberately supplied by later builders before a plan can be persisted.
type shardingScaleInSourceMaterial struct {
	Leaving               []appsv1.ShardingScaleInPlanMember
	Staying               []appsv1.ShardingScaleInPlanMember
	ProofExecutor         appsv1.ShardingScaleInProofExecutor
	ExecutorPrerequisites []appsv1.ShardingScaleInExecutorPrerequisite
}

type shardingScaleInCriticalProjection struct {
	Version         string                  `json:"version"`
	OwnerReferences []metav1.OwnerReference `json:"ownerReferences"`
	Value           any                     `json:"value"`
}

func buildShardingScaleInSourceMaterial(inventory *shardingScaleInDNSInventory,
	bindings []shardingScaleInPodRuntimeBinding,
) (*shardingScaleInSourceMaterial, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInSourceMaterial,
			fmt.Sprintf(format, args...))
	}
	if inventory == nil || inventory.Members == nil || inventory.Members.Topology == nil ||
		inventory.Members.Topology.Cluster == nil {
		return nil, invalid("DNS, member, topology, and Cluster inventory must be complete")
	}
	cluster := inventory.Members.Topology.Cluster
	if cluster.Namespace == "" || cluster.Name == "" || cluster.UID == "" ||
		cluster.Generation <= 0 {
		return nil, invalid("Cluster source identity must be complete")
	}
	if len(inventory.Leaving) == 0 || len(inventory.Staying) == 0 {
		return nil, invalid("leaving and staying DNS inventory must be non-empty")
	}
	if len(inventory.Leaving)+len(inventory.Staying) > 32 {
		return nil, invalid("source inventory contains more than 32 Components")
	}
	if err := validateShardingScaleInDNSMemberBinding(inventory); err != nil {
		return nil, err
	}

	bindingsByPodUID := make(map[types.UID]shardingScaleInPodRuntimeBinding, len(bindings))
	for _, binding := range bindings {
		if binding.PodUID == "" || !isShardingScaleInImageID(binding.AgentImageID) ||
			len(binding.AgentProcessUID) < 22 ||
			!isShardingScaleInSHA256(binding.AgentCapabilityDigest) {
			return nil, invalid("runtime binding identity must be complete")
		}
		if _, ok := bindingsByPodUID[binding.PodUID]; ok {
			return nil, invalid("runtime binding for Pod UID %q is duplicated", binding.PodUID)
		}
		bindingsByPodUID[binding.PodUID] = binding
	}

	material := &shardingScaleInSourceMaterial{}
	usedBindings := map[types.UID]struct{}{}
	componentUIDs := map[types.UID]struct{}{}
	componentNames := map[string]struct{}{}
	componentShortNames := map[string]struct{}{}
	podUIDs := map[types.UID]struct{}{}
	podNames := map[string]struct{}{}
	podFQDNs := map[string]struct{}{}
	prerequisiteKeys := map[string]struct{}{}
	prerequisiteUIDs := map[types.UID]struct{}{}
	podCount := 0

	buildRecords := func(records []shardingScaleInSourceDNS) (
		[]appsv1.ShardingScaleInPlanMember, error,
	) {
		members := make([]appsv1.ShardingScaleInPlanMember, 0, len(records))
		for i := range records {
			record := &records[i]
			member, err := buildShardingScaleInSourcePlanMember(
				cluster.Namespace, cluster.Name, &record.Member,
				bindingsByPodUID, usedBindings, componentUIDs, componentNames,
				componentShortNames, podUIDs, podNames, podFQDNs)
			if err != nil {
				return nil, err
			}
			members = append(members, member)
			podCount += len(member.Pods)

			prerequisites, err := buildShardingScaleInSourcePrerequisites(record)
			if err != nil {
				return nil, err
			}
			for _, prerequisite := range prerequisites {
				key := strings.Join([]string{
					prerequisite.APIVersion, prerequisite.Kind,
					prerequisite.Namespace, prerequisite.Name,
				}, "\x00")
				if _, ok := prerequisiteKeys[key]; ok {
					return nil, invalid("executor prerequisite %s %q is duplicated",
						prerequisite.Kind, prerequisite.Name)
				}
				if _, ok := prerequisiteUIDs[prerequisite.UID]; ok {
					return nil, invalid("executor prerequisite UID %q is duplicated",
						prerequisite.UID)
				}
				prerequisiteKeys[key] = struct{}{}
				prerequisiteUIDs[prerequisite.UID] = struct{}{}
				material.ExecutorPrerequisites = append(material.ExecutorPrerequisites, prerequisite)
			}
		}
		slices.SortFunc(members, compareShardingScaleInPlanMembers)
		return members, nil
	}

	var err error
	if material.Leaving, err = buildRecords(inventory.Leaving); err != nil {
		return nil, err
	}
	if material.Staying, err = buildRecords(inventory.Staying); err != nil {
		return nil, err
	}
	if len(usedBindings) != len(bindingsByPodUID) {
		return nil, invalid("runtime bindings must cover every exact plan Pod and no other Pod")
	}
	if len(componentUIDs)+podCount+len(material.ExecutorPrerequisites) > 512 {
		return nil, invalid("protected object count exceeds 512")
	}

	stayingPods := make([]appsv1.ShardingScaleInPlanPod, 0)
	for _, member := range material.Staying {
		stayingPods = append(stayingPods, member.Pods...)
	}
	slices.SortFunc(stayingPods, compareShardingScaleInPlanPods)
	if len(stayingPods) == 0 {
		return nil, invalid("staying inventory must contain at least one Pod")
	}
	material.ProofExecutor = appsv1.ShardingScaleInProofExecutor{
		PodName: stayingPods[0].Name,
		PodUID:  stayingPods[0].UID,
	}
	slices.SortFunc(material.ExecutorPrerequisites, compareShardingScaleInPrerequisites)
	return material, nil
}

func validateShardingScaleInDNSMemberBinding(inventory *shardingScaleInDNSInventory) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInSourceMaterial,
			fmt.Sprintf(format, args...))
	}
	validateGroup := func(group string, expected []shardingScaleInSourceMember,
		records []shardingScaleInSourceDNS,
	) error {
		if len(expected) != len(records) {
			return invalid("%s DNS inventory does not exhaust the member inventory", group)
		}
		expectedByUID := make(map[types.UID]shardingScaleInSourceMember, len(expected))
		for _, member := range expected {
			if member.Component.UID == "" {
				return invalid("%s member Component UID must not be empty", group)
			}
			if _, ok := expectedByUID[member.Component.UID]; ok {
				return invalid("%s member Component UID %q is duplicated",
					group, member.Component.UID)
			}
			expectedByUID[member.Component.UID] = member
		}
		for _, record := range records {
			expectedMember, ok := expectedByUID[record.Member.Component.UID]
			if !ok || !reflect.DeepEqual(expectedMember, record.Member) {
				return invalid("%s DNS record does not exactly bind its member snapshot", group)
			}
			delete(expectedByUID, record.Member.Component.UID)
		}
		if len(expectedByUID) != 0 {
			return invalid("%s DNS inventory does not exhaust the member inventory", group)
		}
		return nil
	}
	if err := validateGroup("leaving", inventory.Members.Leaving, inventory.Leaving); err != nil {
		return err
	}
	return validateGroup("staying", inventory.Members.Staying, inventory.Staying)
}

func buildShardingScaleInSourcePlanMember(clusterNamespace, clusterName string,
	source *shardingScaleInSourceMember,
	bindings map[types.UID]shardingScaleInPodRuntimeBinding,
	usedBindings, componentUIDs map[types.UID]struct{},
	componentNames, componentShortNames map[string]struct{},
	podUIDs map[types.UID]struct{},
	podNames, podFQDNs map[string]struct{},
) (appsv1.ShardingScaleInPlanMember, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInSourceMaterial,
			fmt.Sprintf(format, args...))
	}
	if source == nil || source.Component.Namespace != clusterNamespace ||
		source.Component.Name == "" ||
		source.Component.UID == "" || source.Component.Generation <= 0 ||
		source.ShardTemplateName == "" || len(source.Pods) == 0 || len(source.Pods) > 5 {
		return appsv1.ShardingScaleInPlanMember{},
			invalid("Component, shard template, and Pod source identity must be complete")
	}
	if _, ok := componentUIDs[source.Component.UID]; ok {
		return appsv1.ShardingScaleInPlanMember{},
			invalid("Component UID %q is duplicated", source.Component.UID)
	}
	if _, ok := componentNames[source.Component.Name]; ok {
		return appsv1.ShardingScaleInPlanMember{},
			invalid("Component name %q is duplicated", source.Component.Name)
	}
	componentUIDs[source.Component.UID] = struct{}{}
	componentNames[source.Component.Name] = struct{}{}

	shortName, err := component.ShortName(clusterName, source.Component.Name)
	if err != nil || shortName == "" {
		return appsv1.ShardingScaleInPlanMember{},
			invalid("Component %q does not belong to Cluster %q", source.Component.Name, clusterName)
	}
	if _, ok := componentShortNames[shortName]; ok {
		return appsv1.ShardingScaleInPlanMember{},
			invalid("Component short name %q is duplicated", shortName)
	}
	componentShortNames[shortName] = struct{}{}
	componentSpec := source.Component.Spec.DeepCopy()
	canonicalizeShardingScaleInComponentSpec(componentSpec)
	componentSpecDigest, err := digestShardingScaleInCanonicalJSON(struct {
		Version string               `json:"version"`
		Spec    appsv1.ComponentSpec `json:"spec"`
	}{
		Version: shardingScaleInSourceProjectionVersionV1,
		Spec:    *componentSpec,
	})
	if err != nil {
		return appsv1.ShardingScaleInPlanMember{}, err
	}

	member := appsv1.ShardingScaleInPlanMember{
		ComponentName:       source.Component.Name,
		ComponentUID:        source.Component.UID,
		ComponentGeneration: source.Component.Generation,
		ComponentSpecDigest: componentSpecDigest,
		ComponentShortName:  shortName,
		ShardTemplateName:   source.ShardTemplateName,
		Pods:                make([]appsv1.ShardingScaleInPlanPod, 0, len(source.Pods)),
	}
	for _, sourcePod := range source.Pods {
		pod := sourcePod.Pod
		if pod.Namespace != source.Component.Namespace || pod.Name == "" || pod.UID == "" ||
			sourcePod.FQDN == "" {
			return appsv1.ShardingScaleInPlanMember{},
				invalid("Pod source identity and FQDN must be complete")
		}
		if _, ok := podUIDs[pod.UID]; ok {
			return appsv1.ShardingScaleInPlanMember{},
				invalid("Pod UID %q is duplicated", pod.UID)
		}
		if _, ok := podNames[pod.Name]; ok {
			return appsv1.ShardingScaleInPlanMember{},
				invalid("Pod name %q is duplicated", pod.Name)
		}
		if _, ok := podFQDNs[sourcePod.FQDN]; ok {
			return appsv1.ShardingScaleInPlanMember{},
				invalid("Pod FQDN %q is duplicated", sourcePod.FQDN)
		}
		binding, ok := bindings[pod.UID]
		if !ok {
			return appsv1.ShardingScaleInPlanMember{},
				invalid("runtime binding is missing for Pod UID %q", pod.UID)
		}
		podUIDs[pod.UID] = struct{}{}
		podNames[pod.Name] = struct{}{}
		podFQDNs[sourcePod.FQDN] = struct{}{}
		usedBindings[pod.UID] = struct{}{}
		member.Pods = append(member.Pods, appsv1.ShardingScaleInPlanPod{
			Name:                  pod.Name,
			UID:                   pod.UID,
			FQDN:                  sourcePod.FQDN,
			AgentImageID:          binding.AgentImageID,
			AgentProcessUID:       binding.AgentProcessUID,
			AgentCapabilityDigest: binding.AgentCapabilityDigest,
		})
	}
	slices.SortFunc(member.Pods, compareShardingScaleInPlanPods)
	return member, nil
}

func buildShardingScaleInSourcePrerequisites(record *shardingScaleInSourceDNS) (
	[]appsv1.ShardingScaleInExecutorPrerequisite, error,
) {
	if record == nil {
		return nil, fmt.Errorf("%w: DNS record must not be nil",
			errInvalidShardingScaleInSourceMaterial)
	}
	member := &record.Member
	prerequisites := make([]appsv1.ShardingScaleInExecutorPrerequisite, 0,
		4+len(member.Instances)+len(record.EndpointSlices))
	appendPrerequisite := func(apiVersion, kind string, object metav1.Object,
		role appsv1.ShardingScaleInPrerequisiteRole, projection any,
	) error {
		if object == nil || object.GetNamespace() == "" || object.GetName() == "" ||
			object.GetUID() == "" || object.GetNamespace() != member.Component.Namespace {
			return fmt.Errorf("%w: %s prerequisite identity must be complete",
				errInvalidShardingScaleInSourceMaterial, kind)
		}
		criticalSpecDigest, err := digestShardingScaleInCriticalProjection(
			object.GetOwnerReferences(), projection)
		if err != nil {
			return err
		}
		prerequisite := appsv1.ShardingScaleInExecutorPrerequisite{
			APIVersion:         apiVersion,
			Kind:               kind,
			Namespace:          object.GetNamespace(),
			Name:               object.GetName(),
			UID:                object.GetUID(),
			Role:               role,
			Scope:              appsv1.ShardingScaleInPrerequisiteScopeComponent,
			ComponentUID:       member.Component.UID,
			CriticalSpecDigest: criticalSpecDigest,
		}
		identityDigest, err := digestShardingScaleInPrerequisiteIdentity(prerequisite)
		if err != nil {
			return err
		}
		prerequisite.IdentityDigest = identityDigest
		prerequisites = append(prerequisites, prerequisite)
		return nil
	}

	workload := member.Workload.DeepCopy()
	canonicalizeShardingScaleInInstanceSetSpec(&workload.Spec)
	if err := appendPrerequisite(workloadsv1.GroupVersion.String(),
		workloadsv1.InstanceSetKind, workload,
		appsv1.ShardingScaleInPrerequisiteRolePodOwnerWorkload, workload.Spec); err != nil {
		return nil, err
	}
	for i := range member.Instances {
		instance := member.Instances[i].DeepCopy()
		canonicalizeShardingScaleInInstanceSpec(&instance.Spec)
		if err := appendPrerequisite(workloadsv1.GroupVersion.String(),
			shardingScaleInInstanceKind, instance,
			appsv1.ShardingScaleInPrerequisiteRolePodOwnerWorkload, instance.Spec); err != nil {
			return nil, err
		}
	}

	service := record.Service.DeepCopy()
	canonicalizeShardingScaleInServiceSpec(&service.Spec)
	if err := appendPrerequisite(corev1.SchemeGroupVersion.String(), "Service", service,
		appsv1.ShardingScaleInPrerequisiteRoleClusterDNS, service.Spec); err != nil {
		return nil, err
	}
	if record.Endpoints != nil {
		endpoints := record.Endpoints.DeepCopy()
		canonicalizeShardingScaleInEndpointSubsets(endpoints.Subsets)
		if err := appendPrerequisite(corev1.SchemeGroupVersion.String(), "Endpoints", endpoints,
			appsv1.ShardingScaleInPrerequisiteRoleClusterDNS, endpoints.Subsets); err != nil {
			return nil, err
		}
	}
	for i := range record.EndpointSlices {
		endpointSlice := record.EndpointSlices[i].DeepCopy()
		canonicalizeShardingScaleInEndpointSlice(endpointSlice)
		projection := struct {
			AddressType discoveryv1.AddressType    `json:"addressType"`
			Ports       []discoveryv1.EndpointPort `json:"ports"`
			Endpoints   []discoveryv1.Endpoint     `json:"endpoints"`
		}{
			AddressType: endpointSlice.AddressType,
			Ports:       endpointSlice.Ports,
			Endpoints:   endpointSlice.Endpoints,
		}
		if err := appendPrerequisite(discoveryv1.SchemeGroupVersion.String(), "EndpointSlice",
			endpointSlice, appsv1.ShardingScaleInPrerequisiteRoleClusterDNS, projection); err != nil {
			return nil, err
		}
	}
	return prerequisites, nil
}

func digestShardingScaleInCriticalProjection(ownerReferences []metav1.OwnerReference,
	value any,
) (string, error) {
	owners := slices.Clone(ownerReferences)
	slices.SortFunc(owners, func(a, b metav1.OwnerReference) int {
		left := strings.Join([]string{a.APIVersion, a.Kind, a.Name, string(a.UID),
			shardingScaleInOptionalBoolKey(a.Controller),
			shardingScaleInOptionalBoolKey(a.BlockOwnerDeletion)}, "\x00")
		right := strings.Join([]string{b.APIVersion, b.Kind, b.Name, string(b.UID),
			shardingScaleInOptionalBoolKey(b.Controller),
			shardingScaleInOptionalBoolKey(b.BlockOwnerDeletion)}, "\x00")
		return strings.Compare(left, right)
	})
	return digestShardingScaleInCanonicalJSON(shardingScaleInCriticalProjection{
		Version:         shardingScaleInSourceProjectionVersionV1,
		OwnerReferences: owners,
		Value:           value,
	})
}

func shardingScaleInOptionalBoolKey(value *bool) string {
	if value == nil {
		return "0"
	}
	if !*value {
		return "1"
	}
	return "2"
}

// These canonicalizers sort only structural-schema map lists. Atomic and
// ordered lists, including container order and command arguments, stay intact.
func canonicalizeShardingScaleInComponentSpec(spec *appsv1.ComponentSpec) {
	if spec == nil {
		return
	}
	canonicalizeShardingScaleInResourceRequirements(&spec.Resources)
	for i := range spec.Instances {
		if spec.Instances[i].Resources != nil {
			canonicalizeShardingScaleInResourceRequirements(spec.Instances[i].Resources)
		}
	}
	for i := range spec.Services {
		canonicalizeShardingScaleInServiceSpec(&spec.Services[i].Spec)
	}
}

func canonicalizeShardingScaleInInstanceSetSpec(spec *workloadsv1.InstanceSetSpec) {
	if spec == nil {
		return
	}
	slices.SortFunc(spec.Instances, func(a, b workloadsv1.InstanceTemplate) int {
		return strings.Compare(a.Name, b.Name)
	})
	for i := range spec.Instances {
		if spec.Instances[i].Resources != nil {
			canonicalizeShardingScaleInResourceRequirements(spec.Instances[i].Resources)
		}
	}
	canonicalizeShardingScaleInPodSpec(&spec.Template.Spec)
}

func canonicalizeShardingScaleInInstanceSpec(spec *workloadsv1.InstanceSpec) {
	if spec == nil {
		return
	}
	canonicalizeShardingScaleInPodSpec(&spec.Template.Spec)
	for i := range spec.InstanceAssistantObjects {
		service := spec.InstanceAssistantObjects[i].Service
		if service == nil {
			continue
		}
		canonicalizeShardingScaleInServiceSpec(&service.Spec)
		slices.SortFunc(service.Status.Conditions, func(a, b metav1.Condition) int {
			return strings.Compare(a.Type, b.Type)
		})
	}
}

func canonicalizeShardingScaleInPodSpec(spec *corev1.PodSpec) {
	if spec == nil {
		return
	}
	for i := range spec.InitContainers {
		canonicalizeShardingScaleInContainer(&spec.InitContainers[i])
	}
	for i := range spec.Containers {
		canonicalizeShardingScaleInContainer(&spec.Containers[i])
	}
	for i := range spec.EphemeralContainers {
		container := &spec.EphemeralContainers[i].EphemeralContainerCommon
		canonicalizeShardingScaleInContainerFields(
			container.Ports, &container.Resources)
	}
	slices.SortFunc(spec.ResourceClaims, func(a, b corev1.PodResourceClaim) int {
		return strings.Compare(a.Name, b.Name)
	})
	slices.SortFunc(spec.SchedulingGates, func(a, b corev1.PodSchedulingGate) int {
		return strings.Compare(a.Name, b.Name)
	})
	slices.SortFunc(spec.TopologySpreadConstraints,
		func(a, b corev1.TopologySpreadConstraint) int {
			left := strings.Join([]string{a.TopologyKey, string(a.WhenUnsatisfiable)}, "\x00")
			right := strings.Join([]string{b.TopologyKey, string(b.WhenUnsatisfiable)}, "\x00")
			return strings.Compare(left, right)
		})
}

func canonicalizeShardingScaleInContainer(container *corev1.Container) {
	if container == nil {
		return
	}
	canonicalizeShardingScaleInContainerFields(container.Ports, &container.Resources)
}

func canonicalizeShardingScaleInContainerFields(ports []corev1.ContainerPort,
	resources *corev1.ResourceRequirements,
) {
	slices.SortFunc(ports, func(a, b corev1.ContainerPort) int {
		if a.ContainerPort < b.ContainerPort {
			return -1
		}
		if a.ContainerPort > b.ContainerPort {
			return 1
		}
		return strings.Compare(string(a.Protocol), string(b.Protocol))
	})
	canonicalizeShardingScaleInResourceRequirements(resources)
}

func canonicalizeShardingScaleInResourceRequirements(
	resources *corev1.ResourceRequirements,
) {
	if resources == nil {
		return
	}
	slices.SortFunc(resources.Claims, func(a, b corev1.ResourceClaim) int {
		return strings.Compare(a.Name, b.Name)
	})
}

func canonicalizeShardingScaleInServiceSpec(spec *corev1.ServiceSpec) {
	if spec == nil {
		return
	}
	slices.SortFunc(spec.Ports, func(a, b corev1.ServicePort) int {
		if a.Port < b.Port {
			return -1
		}
		if a.Port > b.Port {
			return 1
		}
		return strings.Compare(string(a.Protocol), string(b.Protocol))
	})
}

func canonicalizeShardingScaleInEndpointSubsets(subsets []corev1.EndpointSubset) {
	addressKey := func(address corev1.EndpointAddress) string {
		target := ""
		if address.TargetRef != nil {
			target = strings.Join([]string{
				address.TargetRef.APIVersion, address.TargetRef.Kind, address.TargetRef.Namespace,
				address.TargetRef.Name, string(address.TargetRef.UID),
			}, "\x00")
		}
		nodeName := ""
		if address.NodeName != nil {
			nodeName = *address.NodeName
		}
		return strings.Join([]string{address.IP, address.Hostname, nodeName, target}, "\x00")
	}
	portKey := func(port corev1.EndpointPort) string {
		appProtocol := ""
		if port.AppProtocol != nil {
			appProtocol = *port.AppProtocol
		}
		return fmt.Sprintf("%s\x00%s\x00%d\x00%s",
			port.Name, port.Protocol, port.Port, appProtocol)
	}
	for i := range subsets {
		slices.SortFunc(subsets[i].Addresses, func(a, b corev1.EndpointAddress) int {
			return strings.Compare(addressKey(a), addressKey(b))
		})
		slices.SortFunc(subsets[i].NotReadyAddresses, func(a, b corev1.EndpointAddress) int {
			return strings.Compare(addressKey(a), addressKey(b))
		})
		slices.SortFunc(subsets[i].Ports, func(a, b corev1.EndpointPort) int {
			return strings.Compare(portKey(a), portKey(b))
		})
	}
	slices.SortFunc(subsets, func(a, b corev1.EndpointSubset) int {
		left, _ := digestShardingScaleInCanonicalJSON(a)
		right, _ := digestShardingScaleInCanonicalJSON(b)
		return strings.Compare(left, right)
	})
}

func canonicalizeShardingScaleInEndpointSlice(endpointSlice *discoveryv1.EndpointSlice) {
	for i := range endpointSlice.Endpoints {
		slices.Sort(endpointSlice.Endpoints[i].Addresses)
		if endpointSlice.Endpoints[i].Hints != nil {
			slices.SortFunc(endpointSlice.Endpoints[i].Hints.ForZones,
				func(a, b discoveryv1.ForZone) int {
					return strings.Compare(a.Name, b.Name)
				})
		}
	}
	slices.SortFunc(endpointSlice.Ports, func(a, b discoveryv1.EndpointPort) int {
		left, _ := digestShardingScaleInCanonicalJSON(a)
		right, _ := digestShardingScaleInCanonicalJSON(b)
		return strings.Compare(left, right)
	})
	slices.SortFunc(endpointSlice.Endpoints, func(a, b discoveryv1.Endpoint) int {
		left, _ := digestShardingScaleInCanonicalJSON(a)
		right, _ := digestShardingScaleInCanonicalJSON(b)
		return strings.Compare(left, right)
	})
}
