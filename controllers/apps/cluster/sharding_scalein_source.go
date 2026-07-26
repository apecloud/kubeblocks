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
	"errors"
	"fmt"
	"net/netip"
	"reflect"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

var (
	errInvalidShardingScaleInTopology        = errors.New("invalid sharding scale-in topology")
	errInvalidShardingScaleInMemberInventory = errors.New("invalid sharding scale-in member inventory")
	errInvalidShardingScaleInDNSInventory    = errors.New("invalid sharding scale-in DNS inventory")
)

const (
	shardingScaleInDefaultShardTemplate = "@default"
	shardingScaleInInstanceKind         = "Instance"
)

type shardingScaleInDesiredComponent struct {
	ShardTemplateName string
	Spec              appsv1.ClusterComponentSpec
}

// shardingScaleInTopologyInventory is a source-only candidate snapshot. It
// deliberately contains live objects rather than persisted plan material;
// later builders must close Pod, prerequisite, request, and capability identity
// before any status CAS or external action is allowed.
type shardingScaleInTopologyInventory struct {
	Cluster            *appsv1.Cluster
	Sharding           appsv1.ClusterSharding
	ShardingDefinition *appsv1.ShardingDefinition

	Components        []appsv1.Component
	DesiredComponents []shardingScaleInDesiredComponent
	Leaving           []appsv1.Component
	Staying           []appsv1.Component
}

type shardingScaleInSourcePod struct {
	Pod  corev1.Pod
	FQDN string
}

type shardingScaleInSourceMember struct {
	Component         appsv1.Component
	ShardTemplateName string
	Workload          workloadsv1.InstanceSet
	Instances         []workloadsv1.Instance
	Pods              []shardingScaleInSourcePod
}

// shardingScaleInMemberInventory closes live member and Pod identity only. It
// intentionally carries no kbagent process, image, capability, request, or
// prerequisite claims.
type shardingScaleInMemberInventory struct {
	Topology *shardingScaleInTopologyInventory
	Leaving  []shardingScaleInSourceMember
	Staying  []shardingScaleInSourceMember
}

type shardingScaleInSourceDNS struct {
	Member         shardingScaleInSourceMember
	Service        corev1.Service
	Endpoints      *corev1.Endpoints
	EndpointSlices []discoveryv1.EndpointSlice
}

// shardingScaleInDNSInventory proves the live DNS identity of every source
// member. It remains source-only; no execution prerequisite or persisted plan
// may be derived until later builders close the remaining live identities.
type shardingScaleInDNSInventory struct {
	Members *shardingScaleInMemberInventory
	Leaving []shardingScaleInSourceDNS
	Staying []shardingScaleInSourceDNS
}

func loadFreshShardingScaleInTopology(ctx context.Context, apiReader client.Reader,
	clusterKey types.NamespacedName, expectedClusterUID types.UID, shardingName string,
) (*shardingScaleInTopologyInventory, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInTopology, fmt.Sprintf(format, args...))
	}
	if apiReader == nil {
		return nil, invalid("APIReader must not be nil")
	}
	if clusterKey.Namespace == "" || clusterKey.Name == "" || expectedClusterUID == "" || shardingName == "" {
		return nil, invalid("Cluster key, expected UID, and sharding name must be complete")
	}

	cluster := &appsv1.Cluster{}
	if err := apiReader.Get(ctx, clusterKey, cluster); err != nil {
		return nil, err
	}
	if cluster.Namespace != clusterKey.Namespace || cluster.Name != clusterKey.Name ||
		cluster.UID != expectedClusterUID {
		return nil, invalid("fresh Cluster UID or name does not match the requested identity")
	}
	if cluster.Generation <= 0 || cluster.ResourceVersion == "" {
		return nil, invalid("fresh Cluster generation and resourceVersion must be non-empty")
	}
	if !cluster.DeletionTimestamp.IsZero() {
		return nil, invalid("fresh Cluster is deleting")
	}

	shardingSpec, err := exactClusterSharding(cluster, shardingName)
	if err != nil {
		return nil, err
	}
	if shardingSpec.Shards <= 0 || shardingSpec.Shards > 32 {
		return nil, invalid("desired shard count must be between 1 and 32")
	}
	if shardingSpec.ShardingDef == "" {
		return nil, invalid("sharding definition selector must not be empty")
	}

	shardingDef, err := resolveShardingDefinition(ctx, apiReader, shardingSpec.ShardingDef)
	if err != nil {
		return nil, err
	}
	if shardingDef == nil || shardingDef.Name == "" || shardingDef.UID == "" ||
		shardingDef.Generation <= 0 || !shardingDef.DeletionTimestamp.IsZero() {
		return nil, invalid("resolved ShardingDefinition identity must be complete and not deleting")
	}
	if shardingDef.Spec.LifecycleActions == nil ||
		shardingDef.Spec.LifecycleActions.ShardRemove == nil ||
		shardingDef.Spec.LifecycleActions.ShardRemove.ResultProtocol != appsv1.ShardingScaleInResultProtocolV2 {
		return nil, invalid("resolved ShardingDefinition must provide the typed shard-remove action")
	}
	if err := validateShardingShards(shardingDef, shardingSpec); err != nil {
		return nil, err
	}

	before, err := listFreshShardingScaleInComponents(ctx, apiReader, cluster, shardingName)
	if err != nil {
		return nil, err
	}
	if err := validateFreshShardingScaleInComponents(cluster, shardingDef, shardingName, before); err != nil {
		return nil, err
	}

	desiredByTemplate, err := sharding.BuildShardingCompSpecs(
		ctx, apiReader, cluster.Namespace, cluster.Name, shardingSpec)
	if err != nil {
		return nil, err
	}

	after, err := listFreshShardingScaleInComponents(ctx, apiReader, cluster, shardingName)
	if err != nil {
		return nil, err
	}
	if !sameShardingScaleInComponentSnapshot(before, after) {
		return nil, invalid("fresh Component snapshot changed while desired members were rebuilt")
	}

	desiredComponents, err := flattenShardingScaleInDesiredComponents(cluster.Name, desiredByTemplate)
	if err != nil {
		return nil, err
	}
	if len(desiredComponents) != int(shardingSpec.Shards) {
		return nil, invalid("desired Component count %d does not match desired shards %d",
			len(desiredComponents), shardingSpec.Shards)
	}

	currentByName := make(map[string]*appsv1.Component, len(after))
	for i := range after {
		currentByName[after[i].Name] = &after[i]
	}
	staying := make([]appsv1.Component, 0, len(desiredComponents))
	for i := range desiredComponents {
		name := component.FullName(cluster.Name, desiredComponents[i].Spec.Name)
		current, ok := currentByName[name]
		if !ok {
			return nil, invalid("desired topology would create Component %q", name)
		}
		staying = append(staying, *current.DeepCopy())
		delete(currentByName, name)
	}
	leaving := make([]appsv1.Component, 0, len(currentByName))
	for _, comp := range currentByName {
		leaving = append(leaving, *comp.DeepCopy())
	}
	if len(leaving) == 0 || len(staying) == 0 || len(leaving)+len(staying) != len(after) {
		return nil, invalid("scale-in requires non-empty, exhaustive leaving and staying member sets")
	}
	if len(after) > 32 {
		return nil, invalid("current sharding contains more than 32 Components")
	}

	sortShardingScaleInComponents(after)
	sortShardingScaleInComponents(leaving)
	sortShardingScaleInComponents(staying)
	return &shardingScaleInTopologyInventory{
		Cluster:            cluster.DeepCopy(),
		Sharding:           *shardingSpec.DeepCopy(),
		ShardingDefinition: shardingDef.DeepCopy(),
		Components:         deepCopyShardingScaleInComponents(after),
		DesiredComponents:  deepCopyShardingScaleInDesiredComponents(desiredComponents),
		Leaving:            leaving,
		Staying:            staying,
	}, nil
}

func loadFreshShardingScaleInMembers(ctx context.Context, apiReader client.Reader,
	clusterKey types.NamespacedName, expectedClusterUID types.UID, shardingName string,
) (*shardingScaleInMemberInventory, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInMemberInventory,
			fmt.Sprintf(format, args...))
	}
	if apiReader == nil {
		return nil, invalid("APIReader must not be nil")
	}

	topologyBefore, err := loadFreshShardingScaleInTopology(
		ctx, apiReader, clusterKey, expectedClusterUID, shardingName)
	if err != nil {
		return nil, err
	}
	workloadsBefore, err := listFreshShardingScaleInWorkloads(ctx, apiReader, topologyBefore)
	if err != nil {
		return nil, err
	}
	instancesBefore, err := listFreshShardingScaleInInstances(
		ctx, apiReader, topologyBefore, workloadsBefore)
	if err != nil {
		return nil, err
	}
	podsBefore, err := listFreshShardingScaleInPods(
		ctx, apiReader, topologyBefore, workloadsBefore, instancesBefore)
	if err != nil {
		return nil, err
	}
	inventory, err := buildFreshShardingScaleInMembers(
		topologyBefore, workloadsBefore, instancesBefore, podsBefore)
	if err != nil {
		return nil, err
	}

	workloadsAfter, err := listFreshShardingScaleInWorkloads(ctx, apiReader, topologyBefore)
	if err != nil {
		return nil, err
	}
	if !sameShardingScaleInWorkloadSnapshot(workloadsBefore, workloadsAfter) {
		return nil, invalid("InstanceSet snapshot changed while member identity was built")
	}
	instancesAfter, err := listFreshShardingScaleInInstances(
		ctx, apiReader, topologyBefore, workloadsAfter)
	if err != nil {
		return nil, err
	}
	if !sameShardingScaleInInstanceSnapshot(instancesBefore, instancesAfter) {
		return nil, invalid("Instance snapshot changed while member identity was built")
	}
	podsAfter, err := listFreshShardingScaleInPods(
		ctx, apiReader, topologyBefore, workloadsAfter, instancesAfter)
	if err != nil {
		return nil, err
	}
	if !sameShardingScaleInPodSnapshot(podsBefore, podsAfter) {
		return nil, invalid("Pod snapshot changed while member identity was built")
	}

	topologyAfter, err := loadFreshShardingScaleInTopology(
		ctx, apiReader, clusterKey, expectedClusterUID, shardingName)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(topologyBefore, topologyAfter) {
		return nil, invalid("topology snapshot changed while member identity was built")
	}
	return deepCopyShardingScaleInMemberInventory(inventory), nil
}

func loadFreshShardingScaleInDNS(ctx context.Context, apiReader client.Reader,
	clusterKey types.NamespacedName, expectedClusterUID types.UID, shardingName string,
) (*shardingScaleInDNSInventory, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInDNSInventory,
			fmt.Sprintf(format, args...))
	}
	if apiReader == nil {
		return nil, invalid("APIReader must not be nil")
	}

	membersBefore, err := loadFreshShardingScaleInMembers(
		ctx, apiReader, clusterKey, expectedClusterUID, shardingName)
	if err != nil {
		return nil, err
	}
	dnsBefore, err := buildFreshShardingScaleInDNS(ctx, apiReader, membersBefore)
	if err != nil {
		return nil, err
	}
	dnsAfter, err := buildFreshShardingScaleInDNS(ctx, apiReader, membersBefore)
	if err != nil {
		return nil, err
	}
	if !sameShardingScaleInDNSSnapshot(dnsBefore, dnsAfter) {
		return nil, invalid("Service snapshot changed, or Endpoints/EndpointSlice snapshot changed while DNS identity was built")
	}

	membersAfter, err := loadFreshShardingScaleInMembers(
		ctx, apiReader, clusterKey, expectedClusterUID, shardingName)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(membersBefore, membersAfter) {
		return nil, invalid("member snapshot changed while DNS identity was built")
	}
	dnsBefore.Members = membersBefore
	return deepCopyShardingScaleInDNSInventory(dnsBefore), nil
}

type shardingScaleInDNSRecordLocation struct {
	leaving bool
	index   int
}

func buildFreshShardingScaleInDNS(ctx context.Context, apiReader client.Reader,
	members *shardingScaleInMemberInventory,
) (*shardingScaleInDNSInventory, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInDNSInventory,
			fmt.Sprintf(format, args...))
	}
	if apiReader == nil || members == nil || members.Topology == nil ||
		members.Topology.Cluster == nil {
		return nil, invalid("APIReader and member inventory must be complete")
	}

	buildRecords := func(sourceMembers []shardingScaleInSourceMember) (
		[]shardingScaleInSourceDNS, error,
	) {
		records := make([]shardingScaleInSourceDNS, 0, len(sourceMembers))
		for i := range sourceMembers {
			member := &sourceMembers[i]
			serviceKey := types.NamespacedName{
				Namespace: member.Workload.Namespace,
				Name:      member.Workload.Name + "-headless",
			}
			service := &corev1.Service{}
			if err := apiReader.Get(ctx, serviceKey, service); err != nil {
				return nil, err
			}

			var endpoints *corev1.Endpoints
			currentEndpoints := &corev1.Endpoints{}
			if err := apiReader.Get(ctx, serviceKey, currentEndpoints); err != nil {
				if !apierrors.IsNotFound(err) {
					return nil, err
				}
			} else {
				endpoints = currentEndpoints.DeepCopy()
			}
			records = append(records, shardingScaleInSourceDNS{
				Member:    deepCopyShardingScaleInSourceMember(*member),
				Service:   *service.DeepCopy(),
				Endpoints: endpoints,
			})
		}
		return records, nil
	}

	leaving, err := buildRecords(members.Leaving)
	if err != nil {
		return nil, err
	}
	staying, err := buildRecords(members.Staying)
	if err != nil {
		return nil, err
	}
	inventory := &shardingScaleInDNSInventory{
		Members: members,
		Leaving: leaving,
		Staying: staying,
	}

	byName := make(map[string]shardingScaleInDNSRecordLocation, len(leaving)+len(staying))
	byIdentity := make(map[string]shardingScaleInDNSRecordLocation, len(leaving)+len(staying))
	indexRecords := func(records []shardingScaleInSourceDNS, isLeaving bool) error {
		for i := range records {
			service := &records[i].Service
			location := shardingScaleInDNSRecordLocation{leaving: isLeaving, index: i}
			if _, ok := byName[service.Name]; ok {
				return invalid("Service name %q is duplicated", service.Name)
			}
			identity := objectOwnerIdentity(service.Name, service.UID)
			if _, ok := byIdentity[identity]; ok {
				return invalid("Service identity %q is duplicated", service.Name)
			}
			byName[service.Name] = location
			byIdentity[identity] = location
		}
		return nil
	}
	if err := indexRecords(inventory.Leaving, true); err != nil {
		return nil, err
	}
	if err := indexRecords(inventory.Staying, false); err != nil {
		return nil, err
	}

	sliceList := &discoveryv1.EndpointSliceList{}
	if err := apiReader.List(ctx, sliceList,
		client.InNamespace(members.Topology.Cluster.Namespace)); err != nil {
		return nil, err
	}
	recordAt := func(location shardingScaleInDNSRecordLocation) *shardingScaleInSourceDNS {
		if location.leaving {
			return &inventory.Leaving[location.index]
		}
		return &inventory.Staying[location.index]
	}
	for i := range sliceList.Items {
		slice := &sliceList.Items[i]
		labelLocation, labelMatches := byName[slice.Labels[discoveryv1.LabelServiceName]]
		ownerLocation, ownerMatches := byIdentity[ownerReferenceIdentity(metav1.GetControllerOf(slice))]
		if labelMatches && ownerMatches && labelLocation != ownerLocation {
			return nil, invalid("EndpointSlice %q ambiguously refers to two Services", slice.Name)
		}
		if !labelMatches && !ownerMatches {
			continue
		}
		location := labelLocation
		if ownerMatches {
			location = ownerLocation
		}
		record := recordAt(location)
		record.EndpointSlices = append(record.EndpointSlices, *slice.DeepCopy())
	}

	validateRecords := func(records []shardingScaleInSourceDNS) error {
		for i := range records {
			if err := validateFreshShardingScaleInDNSRecord(&records[i]); err != nil {
				return err
			}
			sortShardingScaleInEndpointSlices(records[i].EndpointSlices)
		}
		sortShardingScaleInDNSRecords(records)
		return nil
	}
	if err := validateRecords(inventory.Leaving); err != nil {
		return nil, err
	}
	if err := validateRecords(inventory.Staying); err != nil {
		return nil, err
	}
	return inventory, nil
}

func validateFreshShardingScaleInDNSRecord(record *shardingScaleInSourceDNS) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInDNSInventory,
			fmt.Sprintf(format, args...))
	}
	if record == nil {
		return invalid("DNS record must not be nil")
	}
	member := &record.Member
	service := &record.Service
	workload := &member.Workload
	expectedServiceName := workload.Name + "-headless"
	if service.Namespace != workload.Namespace || service.Name != expectedServiceName ||
		service.UID == "" || service.ResourceVersion == "" ||
		!service.DeletionTimestamp.IsZero() {
		return invalid("headless Service identity for InstanceSet %q is invalid", workload.Name)
	}
	if workload.Spec.DisableDefaultHeadlessService {
		return invalid("InstanceSet %q disables its default headless Service", workload.Name)
	}
	if !hasExactControllerOwner(service, workloadsv1.GroupVersion.String(),
		workloadsv1.InstanceSetKind, workload.Name, workload.UID) {
		return invalid("Service controller owner for %q is not the exact InstanceSet", service.Name)
	}
	if service.Spec.Type != corev1.ServiceTypeClusterIP ||
		service.Spec.ClusterIP != corev1.ClusterIPNone ||
		!reflect.DeepEqual(service.Spec.ClusterIPs, []string{corev1.ClusterIPNone}) ||
		!service.Spec.PublishNotReadyAddresses {
		return invalid("headless Service %q contract is invalid", service.Name)
	}
	if workload.Spec.Selector == nil || len(workload.Spec.Selector.MatchLabels) == 0 ||
		len(workload.Spec.Selector.MatchExpressions) != 0 {
		return invalid("InstanceSet %q selector must be non-empty and exactly representable by a Service",
			workload.Name)
	}
	if releasePhase, ok := workload.Spec.Selector.MatchLabels[constant.KBAppReleasePhaseKey]; ok &&
		releasePhase != constant.ReleasePhaseStable {
		return invalid("InstanceSet %q selector %q must be %q when present",
			workload.Name, constant.KBAppReleasePhaseKey, constant.ReleasePhaseStable)
	}
	expectedSelector := make(map[string]string, len(workload.Spec.Selector.MatchLabels)+1)
	for key, value := range workload.Spec.Selector.MatchLabels {
		expectedSelector[key] = value
	}
	expectedSelector[constant.KBAppReleasePhaseKey] = constant.ReleasePhaseStable
	if !reflect.DeepEqual(service.Spec.Selector, expectedSelector) {
		return invalid("Service selector for %q is not exact", service.Name)
	}
	if err := validateShardingScaleInServiceFamilies(service); err != nil {
		return err
	}
	for i := range member.Pods {
		if member.Pods[i].Pod.Spec.Subdomain != service.Name {
			return invalid("Pod %q subdomain does not match Service %q",
				member.Pods[i].Pod.Name, service.Name)
		}
	}
	if record.Endpoints != nil {
		if err := validateShardingScaleInEndpoints(record); err != nil {
			return err
		}
	}
	if len(record.EndpointSlices) == 0 {
		return invalid("EndpointSlice coverage for Service %q is empty", service.Name)
	}
	return validateShardingScaleInEndpointSlices(record)
}

func validateShardingScaleInServiceFamilies(service *corev1.Service) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInDNSInventory,
			fmt.Sprintf(format, args...))
	}
	if len(service.Spec.IPFamilies) < 1 || len(service.Spec.IPFamilies) > 2 {
		return invalid("Service %q must have one or two IP families", service.Name)
	}
	seen := map[corev1.IPFamily]struct{}{}
	for _, family := range service.Spec.IPFamilies {
		if family != corev1.IPv4Protocol && family != corev1.IPv6Protocol {
			return invalid("Service %q has unsupported IP family %q", service.Name, family)
		}
		if _, ok := seen[family]; ok {
			return invalid("Service %q duplicates IP family %q", service.Name, family)
		}
		seen[family] = struct{}{}
	}
	if len(service.Spec.IPFamilies) == 1 {
		if service.Spec.IPFamilyPolicy != nil &&
			*service.Spec.IPFamilyPolicy != corev1.IPFamilyPolicySingleStack &&
			*service.Spec.IPFamilyPolicy != corev1.IPFamilyPolicyPreferDualStack {
			return invalid("Service %q has an invalid single-family policy", service.Name)
		}
	} else if service.Spec.IPFamilyPolicy == nil ||
		(*service.Spec.IPFamilyPolicy != corev1.IPFamilyPolicyRequireDualStack &&
			*service.Spec.IPFamilyPolicy != corev1.IPFamilyPolicyPreferDualStack) {
		return invalid("Service %q has an invalid dual-stack policy", service.Name)
	}
	return nil
}

func validateShardingScaleInEndpoints(record *shardingScaleInSourceDNS) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInDNSInventory,
			fmt.Sprintf(format, args...))
	}
	endpoints := record.Endpoints
	service := &record.Service
	if endpoints.Namespace != service.Namespace || endpoints.Name != service.Name ||
		endpoints.UID == "" || endpoints.ResourceVersion == "" ||
		!endpoints.DeletionTimestamp.IsZero() {
		return invalid("Endpoints identity for Service %q is invalid", service.Name)
	}
	if len(service.Spec.IPFamilies) == 0 {
		return invalid("Service %q has no primary IP family", service.Name)
	}
	primaryFamily := service.Spec.IPFamilies[0]
	podsByName := make(map[string]*corev1.Pod, len(record.Member.Pods))
	primaryAddresses := make(map[string]string, len(record.Member.Pods))
	for i := range record.Member.Pods {
		pod := &record.Member.Pods[i].Pod
		if _, ok := podsByName[pod.Name]; ok {
			return invalid("Endpoints Pod %q is duplicated", pod.Name)
		}
		addresses, err := shardingScaleInPodAddresses(pod)
		if err != nil {
			return err
		}
		primaryAddress, ok := addresses[primaryFamily]
		if !ok {
			return invalid("Endpoints Pod %q has no address for Service primary IP family %q",
				pod.Name, primaryFamily)
		}
		podsByName[pod.Name] = pod
		primaryAddresses[pod.Name] = primaryAddress
	}
	seen := make(map[string]struct{}, len(podsByName))
	validateAddress := func(address corev1.EndpointAddress) error {
		if address.TargetRef == nil {
			return invalid("Endpoints %q address has no Pod TargetRef", endpoints.Name)
		}
		pod, ok := podsByName[address.TargetRef.Name]
		if !ok || address.TargetRef.Kind != "Pod" ||
			(address.TargetRef.APIVersion != "" && address.TargetRef.APIVersion != "v1") ||
			address.TargetRef.Namespace != pod.Namespace ||
			address.TargetRef.Name != pod.Name || address.TargetRef.UID != pod.UID {
			return invalid("Endpoints %q Pod TargetRef is not exact", endpoints.Name)
		}
		if _, ok := seen[pod.Name]; ok {
			return invalid("Endpoints %q duplicates Pod %q", endpoints.Name, pod.Name)
		}
		if address.IP != primaryAddresses[pod.Name] {
			return invalid("Endpoints %q address for Pod %q is not its Service primary IP family address",
				endpoints.Name, pod.Name)
		}
		if address.Hostname != pod.Name {
			return invalid("Endpoints %q hostname for Pod %q is not exact",
				endpoints.Name, pod.Name)
		}
		seen[pod.Name] = struct{}{}
		return nil
	}
	for i := range endpoints.Subsets {
		for j := range endpoints.Subsets[i].Addresses {
			if err := validateAddress(endpoints.Subsets[i].Addresses[j]); err != nil {
				return err
			}
		}
		for j := range endpoints.Subsets[i].NotReadyAddresses {
			if err := validateAddress(endpoints.Subsets[i].NotReadyAddresses[j]); err != nil {
				return err
			}
		}
	}
	if len(seen) != len(podsByName) {
		return invalid("Endpoints %q Pod coverage is incomplete", endpoints.Name)
	}
	return nil
}

func validateShardingScaleInEndpointSlices(record *shardingScaleInSourceDNS) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInDNSInventory,
			fmt.Sprintf(format, args...))
	}
	service := &record.Service
	expectedByFamily := make(map[corev1.IPFamily]map[string]string,
		len(service.Spec.IPFamilies))
	for _, family := range service.Spec.IPFamilies {
		expectedByFamily[family] = make(map[string]string, len(record.Member.Pods))
	}
	for i := range record.Member.Pods {
		pod := &record.Member.Pods[i].Pod
		byFamily, err := shardingScaleInPodAddresses(pod)
		if err != nil {
			return err
		}
		for _, family := range service.Spec.IPFamilies {
			address, ok := byFamily[family]
			if !ok {
				return invalid("Pod %q has no address for Service IP family %q", pod.Name, family)
			}
			expectedByFamily[family][pod.Name] = address
		}
	}

	seen := make(map[corev1.IPFamily]map[string]struct{}, len(expectedByFamily))
	for family := range expectedByFamily {
		seen[family] = map[string]struct{}{}
	}
	for i := range record.EndpointSlices {
		slice := &record.EndpointSlices[i]
		if slice.Namespace != service.Namespace || slice.Name == "" ||
			slice.UID == "" || slice.ResourceVersion == "" ||
			!slice.DeletionTimestamp.IsZero() {
			return invalid("EndpointSlice identity for Service %q is invalid", service.Name)
		}
		if slice.Labels[discoveryv1.LabelServiceName] != service.Name {
			return invalid("EndpointSlice service-name label for %q is not exact", slice.Name)
		}
		if !hasExactControllerOwner(slice, corev1.SchemeGroupVersion.String(),
			"Service", service.Name, service.UID) {
			return invalid("EndpointSlice controller owner for %q is not the exact Service",
				slice.Name)
		}
		family, err := shardingScaleInEndpointSliceFamily(slice.AddressType)
		if err != nil {
			return err
		}
		expected, ok := expectedByFamily[family]
		if !ok {
			return invalid("EndpointSlice %q family is not served by Service %q",
				slice.Name, service.Name)
		}
		for j := range slice.Endpoints {
			endpoint := &slice.Endpoints[j]
			if endpoint.TargetRef == nil || endpoint.Hostname == nil ||
				len(endpoint.Addresses) != 1 {
				return invalid("EndpointSlice %q endpoint identity is incomplete", slice.Name)
			}
			podName := endpoint.TargetRef.Name
			expectedAddress, ok := expected[podName]
			if !ok || endpoint.TargetRef.Kind != "Pod" ||
				(endpoint.TargetRef.APIVersion != "" && endpoint.TargetRef.APIVersion != "v1") ||
				endpoint.TargetRef.Namespace != service.Namespace ||
				endpoint.TargetRef.UID == "" || *endpoint.Hostname != podName {
				return invalid("EndpointSlice %q Pod identity is not exact", slice.Name)
			}
			var expectedUID types.UID
			for k := range record.Member.Pods {
				if record.Member.Pods[k].Pod.Name == podName {
					expectedUID = record.Member.Pods[k].Pod.UID
					break
				}
			}
			if endpoint.TargetRef.UID != expectedUID {
				return invalid("EndpointSlice %q Pod UID is not exact", slice.Name)
			}
			actualFamily, err := shardingScaleInIPFamily(endpoint.Addresses[0])
			if err != nil || actualFamily != family ||
				endpoint.Addresses[0] != expectedAddress {
				return invalid("EndpointSlice %q Pod address is not exact", slice.Name)
			}
			if _, ok := seen[family][podName]; ok {
				return invalid("EndpointSlice coverage duplicates Pod %q for family %q",
					podName, family)
			}
			seen[family][podName] = struct{}{}
		}
	}
	for family, expected := range expectedByFamily {
		if len(seen[family]) != len(expected) {
			return invalid("EndpointSlice Pod coverage for family %q is incomplete", family)
		}
	}
	return nil
}

func shardingScaleInPodAddresses(pod *corev1.Pod) (
	map[corev1.IPFamily]string, error,
) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInDNSInventory,
			fmt.Sprintf(format, args...))
	}
	if pod == nil || pod.Status.PodIP == "" {
		return nil, invalid("Pod primary PodIP must be non-empty")
	}
	primaryFamily, err := shardingScaleInIPFamily(pod.Status.PodIP)
	if err != nil {
		return nil, invalid("Pod %q has invalid primary PodIP %q", pod.Name, pod.Status.PodIP)
	}
	if len(pod.Status.PodIPs) == 0 {
		return map[corev1.IPFamily]string{primaryFamily: pod.Status.PodIP}, nil
	}
	if pod.Status.PodIPs[0].IP != pod.Status.PodIP {
		return nil, invalid("Pod %q primary PodIP does not match its first PodIPs entry", pod.Name)
	}

	byFamily := make(map[corev1.IPFamily]string, len(pod.Status.PodIPs))
	for _, podIP := range pod.Status.PodIPs {
		family, err := shardingScaleInIPFamily(podIP.IP)
		if err != nil {
			return nil, invalid("Pod %q has invalid PodIP %q", pod.Name, podIP.IP)
		}
		if _, ok := byFamily[family]; ok {
			return nil, invalid("Pod %q has multiple addresses for IP family %q", pod.Name, family)
		}
		byFamily[family] = podIP.IP
	}
	return byFamily, nil
}

func shardingScaleInEndpointSliceFamily(addressType discoveryv1.AddressType) (
	corev1.IPFamily, error,
) {
	switch addressType {
	case discoveryv1.AddressTypeIPv4:
		return corev1.IPv4Protocol, nil
	case discoveryv1.AddressTypeIPv6:
		return corev1.IPv6Protocol, nil
	default:
		return "", fmt.Errorf("%w: EndpointSlice address type %q is unsupported",
			errInvalidShardingScaleInDNSInventory, addressType)
	}
}

func shardingScaleInIPFamily(address string) (corev1.IPFamily, error) {
	parsed, err := netip.ParseAddr(address)
	if err != nil || parsed.Zone() != "" || parsed.Is4In6() {
		if err == nil {
			err = fmt.Errorf("address %q is not a canonical unzoned IP", address)
		}
		return "", err
	}
	if parsed.Is4() {
		return corev1.IPv4Protocol, nil
	}
	if parsed.Is6() {
		return corev1.IPv6Protocol, nil
	}
	return "", fmt.Errorf("address %q has no supported IP family", address)
}

func hasExactControllerOwner(object metav1.Object, apiVersion, kind, name string,
	uid types.UID,
) bool {
	owner := metav1.GetControllerOf(object)
	return owner != nil && owner.APIVersion == apiVersion && owner.Kind == kind &&
		owner.Name == name && owner.UID == uid
}

func listFreshShardingScaleInWorkloads(ctx context.Context, apiReader client.Reader,
	topology *shardingScaleInTopologyInventory,
) ([]workloadsv1.InstanceSet, error) {
	list := &workloadsv1.InstanceSetList{}
	if err := apiReader.List(ctx, list, client.InNamespace(topology.Cluster.Namespace)); err != nil {
		return nil, err
	}

	componentsByOwner := make(map[string]struct{}, len(topology.Components))
	componentsByShortName := make(map[string]struct{}, len(topology.Components))
	for i := range topology.Components {
		comp := &topology.Components[i]
		componentsByOwner[objectOwnerIdentity(comp.Name, comp.UID)] = struct{}{}
		componentsByShortName[comp.Labels[constant.KBAppComponentLabelKey]] = struct{}{}
	}

	result := make([]workloadsv1.InstanceSet, 0, len(topology.Components))
	for i := range list.Items {
		workload := &list.Items[i]
		owner := metav1.GetControllerOf(workload)
		_, ownerMatches := componentsByOwner[ownerReferenceIdentity(owner)]
		_, componentMatches := componentsByShortName[workload.Labels[constant.KBAppComponentLabelKey]]
		labelMatches := workload.Labels[constant.AppInstanceLabelKey] == topology.Cluster.Name &&
			componentMatches
		if ownerMatches || labelMatches {
			result = append(result, *workload.DeepCopy())
		}
	}
	sortShardingScaleInWorkloads(result)
	return result, nil
}

func listFreshShardingScaleInInstances(ctx context.Context, apiReader client.Reader,
	topology *shardingScaleInTopologyInventory, workloads []workloadsv1.InstanceSet,
) ([]workloadsv1.Instance, error) {
	list := &workloadsv1.InstanceList{}
	if err := apiReader.List(ctx, list, client.InNamespace(topology.Cluster.Namespace)); err != nil {
		return nil, err
	}

	workloadsByOwner := make(map[string]struct{}, len(workloads))
	workloadsByName := make(map[string]struct{}, len(workloads))
	componentsByShortName := make(map[string]struct{}, len(topology.Components))
	for i := range workloads {
		workloadsByOwner[objectOwnerIdentity(workloads[i].Name, workloads[i].UID)] = struct{}{}
		workloadsByName[workloads[i].Name] = struct{}{}
	}
	for i := range topology.Components {
		componentsByShortName[topology.Components[i].Labels[constant.KBAppComponentLabelKey]] =
			struct{}{}
	}

	result := make([]workloadsv1.Instance, 0)
	for i := range list.Items {
		instance := &list.Items[i]
		owner := metav1.GetControllerOf(instance)
		_, ownerMatches := workloadsByOwner[ownerReferenceIdentity(owner)]
		_, componentMatches := componentsByShortName[instance.Labels[constant.KBAppComponentLabelKey]]
		labelMatches := instance.Labels[constant.AppInstanceLabelKey] ==
			topology.Cluster.Name && componentMatches
		_, workloadLabelMatches := workloadsByName[instance.Labels[instanceset.WorkloadsInstanceLabelKey]]
		_, workloadSpecMatches := workloadsByName[instance.Spec.InstanceSetName]
		if ownerMatches || labelMatches || workloadLabelMatches || workloadSpecMatches {
			result = append(result, *instance.DeepCopy())
		}
	}
	sortShardingScaleInInstances(result)
	return result, nil
}

func listFreshShardingScaleInPods(ctx context.Context, apiReader client.Reader,
	topology *shardingScaleInTopologyInventory, workloads []workloadsv1.InstanceSet,
	instances []workloadsv1.Instance,
) ([]corev1.Pod, error) {
	list := &corev1.PodList{}
	if err := apiReader.List(ctx, list, client.InNamespace(topology.Cluster.Namespace)); err != nil {
		return nil, err
	}

	podOwners := make(map[string]struct{}, len(workloads)+len(instances))
	workloadNames := make(map[string]struct{}, len(workloads))
	instanceNames := make(map[string]struct{}, len(instances))
	componentsByShortName := make(map[string]struct{}, len(topology.Components))
	for i := range workloads {
		podOwners[objectOwnerIdentity(workloads[i].Name, workloads[i].UID)] = struct{}{}
		workloadNames[workloads[i].Name] = struct{}{}
	}
	for i := range instances {
		podOwners[objectOwnerIdentity(instances[i].Name, instances[i].UID)] = struct{}{}
		instanceNames[instances[i].Name] = struct{}{}
	}
	for i := range topology.Components {
		componentsByShortName[topology.Components[i].Labels[constant.KBAppComponentLabelKey]] = struct{}{}
	}

	result := make([]corev1.Pod, 0)
	for i := range list.Items {
		pod := &list.Items[i]
		owner := metav1.GetControllerOf(pod)
		_, ownerMatches := podOwners[ownerReferenceIdentity(owner)]
		_, componentMatches := componentsByShortName[pod.Labels[constant.KBAppComponentLabelKey]]
		labelMatches := pod.Labels[constant.AppInstanceLabelKey] == topology.Cluster.Name &&
			componentMatches
		_, workloadLabelMatches := workloadNames[pod.Labels[instanceset.WorkloadsInstanceLabelKey]]
		_, instanceLabelMatches := instanceNames[pod.Labels[constant.KBAppInstanceNameLabelKey]]
		_, instanceNameMatches := instanceNames[pod.Name]
		if ownerMatches || labelMatches || workloadLabelMatches ||
			instanceLabelMatches || instanceNameMatches {
			result = append(result, *pod.DeepCopy())
		}
	}
	sortShardingScaleInPods(result)
	return result, nil
}

func buildFreshShardingScaleInMembers(topology *shardingScaleInTopologyInventory,
	workloads []workloadsv1.InstanceSet, instances []workloadsv1.Instance, pods []corev1.Pod,
) (*shardingScaleInMemberInventory, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInMemberInventory,
			fmt.Sprintf(format, args...))
	}

	componentsByShortName := make(map[string]*appsv1.Component, len(topology.Components))
	for i := range topology.Components {
		comp := &topology.Components[i]
		shortName := comp.Labels[constant.KBAppComponentLabelKey]
		componentsByShortName[shortName] = comp
	}

	workloadsByComponentUID := make(map[types.UID]*workloadsv1.InstanceSet, len(workloads))
	workloadUIDs := make(map[types.UID]struct{}, len(workloads))
	for i := range workloads {
		workload := &workloads[i]
		if workload.Namespace != topology.Cluster.Namespace || workload.Name == "" ||
			workload.UID == "" || workload.ResourceVersion == "" || workload.Generation <= 0 {
			return nil, invalid("InstanceSet identity, generation, and resourceVersion must be complete")
		}
		if !workload.DeletionTimestamp.IsZero() {
			return nil, invalid("InstanceSet is deleting: %s/%s", workload.Namespace, workload.Name)
		}
		shortName := workload.Labels[constant.KBAppComponentLabelKey]
		comp := componentsByShortName[shortName]
		if comp == nil || workload.Labels[constant.AppManagedByLabelKey] != constant.AppName ||
			workload.Labels[constant.AppInstanceLabelKey] != topology.Cluster.Name ||
			workload.Labels[constant.KBAppShardingNameLabelKey] !=
				comp.Labels[constant.KBAppShardingNameLabelKey] ||
			workload.Labels[constant.KBAppShardTemplateLabelKey] !=
				comp.Labels[constant.KBAppShardTemplateLabelKey] ||
			workload.Name != comp.Name {
			return nil, invalid("InstanceSet labels and name do not identify one exact Component")
		}
		owner := metav1.GetControllerOf(workload)
		if owner == nil || owner.APIVersion != appsv1.GroupVersion.String() ||
			owner.Kind != appsv1.ComponentKind || owner.Name != comp.Name || owner.UID != comp.UID {
			return nil, invalid("InstanceSet controller owner does not identify the exact Component")
		}
		if ptr.Deref(workload.Spec.EnableInstanceAPI, false) !=
			ptr.Deref(comp.Spec.EnableInstanceAPI, false) {
			return nil, invalid("InstanceSet EnableInstanceAPI does not match the exact Component")
		}
		if workloadsByComponentUID[comp.UID] != nil {
			return nil, invalid("Component %q has more than one InstanceSet", comp.Name)
		}
		if _, ok := workloadUIDs[workload.UID]; ok {
			return nil, invalid("InstanceSet UID %q is duplicated", workload.UID)
		}
		workloadsByComponentUID[comp.UID] = workload
		workloadUIDs[workload.UID] = struct{}{}
	}
	for i := range topology.Components {
		if workloadsByComponentUID[topology.Components[i].UID] == nil {
			return nil, invalid("Component %q must have exactly one InstanceSet",
				topology.Components[i].Name)
		}
	}

	instancesByComponentUID := make(map[types.UID][]workloadsv1.Instance,
		len(topology.Components))
	instancesByName := make(map[string]*workloadsv1.Instance, len(instances))
	instanceUIDs := make(map[types.UID]struct{}, len(instances))
	for i := range instances {
		instance := &instances[i]
		if instance.Namespace != topology.Cluster.Namespace || instance.Name == "" ||
			instance.UID == "" || instance.ResourceVersion == "" || instance.Generation <= 0 {
			return nil, invalid("Instance identity, generation, and resourceVersion must be complete")
		}
		if !instance.DeletionTimestamp.IsZero() {
			return nil, invalid("Instance is deleting: %s/%s", instance.Namespace, instance.Name)
		}
		shortName := instance.Labels[constant.KBAppComponentLabelKey]
		comp := componentsByShortName[shortName]
		if comp == nil || instance.Labels[constant.AppManagedByLabelKey] != constant.AppName ||
			instance.Labels[constant.AppInstanceLabelKey] != topology.Cluster.Name ||
			instance.Labels[constant.KBAppShardingNameLabelKey] !=
				comp.Labels[constant.KBAppShardingNameLabelKey] ||
			instance.Labels[constant.KBAppShardTemplateLabelKey] !=
				comp.Labels[constant.KBAppShardTemplateLabelKey] {
			return nil, invalid("Instance labels do not identify one exact Component")
		}
		workload := workloadsByComponentUID[comp.UID]
		if workload == nil ||
			instance.Labels[instanceset.WorkloadsManagedByLabelKey] !=
				workloadsv1.InstanceSetKind ||
			instance.Labels[instanceset.WorkloadsInstanceLabelKey] != workload.Name ||
			instance.Spec.InstanceSetName != workload.Name {
			return nil, invalid("Instance labels and spec do not identify the exact InstanceSet")
		}
		if !ptr.Deref(workload.Spec.EnableInstanceAPI, false) {
			return nil, invalid("Instance exists while Instance API is disabled for %q", workload.Name)
		}
		owner := metav1.GetControllerOf(instance)
		if owner == nil || owner.APIVersion != workloadsv1.GroupVersion.String() ||
			owner.Kind != workloadsv1.InstanceSetKind || owner.Name != workload.Name ||
			owner.UID != workload.UID {
			return nil, invalid("Instance controller owner does not identify the exact InstanceSet")
		}
		if instancesByName[instance.Name] != nil {
			return nil, invalid("Instance name %q is duplicated", instance.Name)
		}
		if _, ok := instanceUIDs[instance.UID]; ok {
			return nil, invalid("Instance UID %q is duplicated", instance.UID)
		}
		instancesByName[instance.Name] = instance
		instanceUIDs[instance.UID] = struct{}{}
		instancesByComponentUID[comp.UID] = append(
			instancesByComponentUID[comp.UID], *instance.DeepCopy())
	}
	for i := range topology.Components {
		comp := &topology.Components[i]
		workload := workloadsByComponentUID[comp.UID]
		memberInstances := instancesByComponentUID[comp.UID]
		if ptr.Deref(workload.Spec.EnableInstanceAPI, false) {
			if len(memberInstances) < 1 || len(memberInstances) > 5 {
				return nil, invalid("Component %q with Instance API must have between 1 and 5 Instances",
					comp.Name)
			}
		} else if len(memberInstances) != 0 {
			return nil, invalid("Component %q has Instances while Instance API is disabled", comp.Name)
		}
		sortShardingScaleInInstances(memberInstances)
		instancesByComponentUID[comp.UID] = memberInstances
	}

	podsByComponentUID := make(map[types.UID][]shardingScaleInSourcePod, len(topology.Components))
	podInstanceUIDs := make(map[types.UID]struct{}, len(pods))
	podNames := make(map[string]struct{}, len(pods))
	podUIDs := make(map[types.UID]struct{}, len(pods))
	podFQDNs := make(map[string]struct{}, len(pods))
	for i := range pods {
		pod := &pods[i]
		if pod.Namespace != topology.Cluster.Namespace || pod.Name == "" ||
			pod.UID == "" || pod.ResourceVersion == "" {
			return nil, invalid("Pod identity and resourceVersion must be complete")
		}
		if !pod.DeletionTimestamp.IsZero() {
			return nil, invalid("Pod is deleting: %s/%s", pod.Namespace, pod.Name)
		}
		shortName := pod.Labels[constant.KBAppComponentLabelKey]
		comp := componentsByShortName[shortName]
		if comp == nil || pod.Labels[constant.AppManagedByLabelKey] != constant.AppName ||
			pod.Labels[constant.AppInstanceLabelKey] != topology.Cluster.Name ||
			pod.Labels[constant.KBAppShardingNameLabelKey] !=
				comp.Labels[constant.KBAppShardingNameLabelKey] ||
			pod.Labels[constant.KBAppShardTemplateLabelKey] !=
				comp.Labels[constant.KBAppShardTemplateLabelKey] {
			return nil, invalid("Pod labels do not identify one exact Component")
		}
		workload := workloadsByComponentUID[comp.UID]
		if workload == nil ||
			pod.Labels[instanceset.WorkloadsManagedByLabelKey] != workloadsv1.InstanceSetKind ||
			pod.Labels[instanceset.WorkloadsInstanceLabelKey] != workload.Name {
			return nil, invalid("Pod labels do not identify the exact InstanceSet")
		}
		owner := metav1.GetControllerOf(pod)
		if ptr.Deref(workload.Spec.EnableInstanceAPI, false) {
			instance := instancesByName[pod.Name]
			if instance == nil ||
				instance.Spec.InstanceSetName != workload.Name ||
				instance.Labels[constant.KBAppComponentLabelKey] != shortName ||
				pod.Labels[constant.KBAppInstanceNameLabelKey] != instance.Name ||
				owner == nil || owner.APIVersion != workloadsv1.GroupVersion.String() ||
				owner.Kind != shardingScaleInInstanceKind || owner.Name != instance.Name ||
				owner.UID != instance.UID {
				return nil, invalid("Pod controller owner does not identify the exact Instance")
			}
			if _, ok := podInstanceUIDs[instance.UID]; ok {
				return nil, invalid("Instance %q owns more than one Pod", instance.Name)
			}
			podInstanceUIDs[instance.UID] = struct{}{}
		} else if owner == nil || owner.APIVersion != workloadsv1.GroupVersion.String() ||
			owner.Kind != workloadsv1.InstanceSetKind || owner.Name != workload.Name ||
			owner.UID != workload.UID {
			return nil, invalid("Pod controller owner does not identify the exact InstanceSet")
		}

		expectedSubdomain := constant.GenerateDefaultComponentHeadlessServiceName(
			topology.Cluster.Name, shortName)
		if pod.Spec.Hostname != pod.Name || pod.Spec.Subdomain != expectedSubdomain {
			return nil, invalid("Pod hostname and subdomain do not identify the exact headless Service")
		}
		fqdn := fmt.Sprintf("%s.%s", pod.Spec.Hostname,
			intctrlutil.ServiceFQDN(pod.Namespace, pod.Spec.Subdomain))
		if fqdn == "" {
			return nil, invalid("Pod FQDN must not be empty")
		}
		if _, ok := podNames[pod.Name]; ok {
			return nil, invalid("Pod name %q is duplicated", pod.Name)
		}
		if _, ok := podUIDs[pod.UID]; ok {
			return nil, invalid("Pod UID %q is duplicated", pod.UID)
		}
		if _, ok := podFQDNs[fqdn]; ok {
			return nil, invalid("Pod FQDN %q is duplicated", fqdn)
		}
		podNames[pod.Name] = struct{}{}
		podUIDs[pod.UID] = struct{}{}
		podFQDNs[fqdn] = struct{}{}
		podsByComponentUID[comp.UID] = append(podsByComponentUID[comp.UID],
			shardingScaleInSourcePod{Pod: *pod.DeepCopy(), FQDN: fqdn})
	}
	for i := range topology.Components {
		comp := &topology.Components[i]
		memberPods := podsByComponentUID[comp.UID]
		if len(memberPods) < 1 || len(memberPods) > 5 {
			return nil, invalid("Component %q must have between 1 and 5 Pods", comp.Name)
		}
		workload := workloadsByComponentUID[comp.UID]
		if ptr.Deref(workload.Spec.EnableInstanceAPI, false) &&
			len(memberPods) != len(instancesByComponentUID[comp.UID]) {
			return nil, invalid("Component %q Instance API owner chain is incomplete", comp.Name)
		}
		sortShardingScaleInSourcePods(memberPods)
		podsByComponentUID[comp.UID] = memberPods
	}

	desiredTemplates := make(map[string]string, len(topology.DesiredComponents))
	for i := range topology.DesiredComponents {
		desired := &topology.DesiredComponents[i]
		desiredTemplates[component.FullName(topology.Cluster.Name, desired.Spec.Name)] =
			desired.ShardTemplateName
	}
	newMember := func(comp appsv1.Component, staying bool) (shardingScaleInSourceMember, error) {
		templateName := comp.Labels[constant.KBAppShardTemplateLabelKey]
		if staying {
			var ok bool
			templateName, ok = desiredTemplates[comp.Name]
			if !ok || templateName == "" {
				return shardingScaleInSourceMember{},
					invalid("staying Component %q has no desired shard-template identity", comp.Name)
			}
		} else if templateName == "" {
			templateName = shardingScaleInDefaultShardTemplate
		}
		workload := workloadsByComponentUID[comp.UID]
		return shardingScaleInSourceMember{
			Component:         *comp.DeepCopy(),
			ShardTemplateName: templateName,
			Workload:          *workload.DeepCopy(),
			Instances:         deepCopyShardingScaleInInstances(instancesByComponentUID[comp.UID]),
			Pods:              deepCopyShardingScaleInSourcePods(podsByComponentUID[comp.UID]),
		}, nil
	}

	inventory := &shardingScaleInMemberInventory{
		Topology: deepCopyShardingScaleInTopologyInventory(topology),
		Leaving:  make([]shardingScaleInSourceMember, 0, len(topology.Leaving)),
		Staying:  make([]shardingScaleInSourceMember, 0, len(topology.Staying)),
	}
	for i := range topology.Leaving {
		member, err := newMember(topology.Leaving[i], false)
		if err != nil {
			return nil, err
		}
		inventory.Leaving = append(inventory.Leaving, member)
	}
	for i := range topology.Staying {
		member, err := newMember(topology.Staying[i], true)
		if err != nil {
			return nil, err
		}
		inventory.Staying = append(inventory.Staying, member)
	}
	sortShardingScaleInSourceMembers(inventory.Leaving)
	sortShardingScaleInSourceMembers(inventory.Staying)
	return inventory, nil
}

func exactClusterSharding(cluster *appsv1.Cluster, shardingName string) (*appsv1.ClusterSharding, error) {
	var found *appsv1.ClusterSharding
	for i := range cluster.Spec.Shardings {
		if cluster.Spec.Shardings[i].Name != shardingName {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("%w: Cluster sharding %q is duplicated",
				errInvalidShardingScaleInTopology, shardingName)
		}
		found = &cluster.Spec.Shardings[i]
	}
	if found == nil {
		return nil, fmt.Errorf("%w: Cluster sharding %q was not found",
			errInvalidShardingScaleInTopology, shardingName)
	}
	return found, nil
}

func validateFreshShardingScaleInComponents(cluster *appsv1.Cluster,
	shardingDef *appsv1.ShardingDefinition, shardingName string, components []appsv1.Component,
) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInTopology, fmt.Sprintf(format, args...))
	}
	if len(components) == 0 || len(components) > 32 {
		return invalid("current sharding must contain between 1 and 32 Components")
	}
	names := map[string]struct{}{}
	uids := map[types.UID]struct{}{}
	shortNames := map[string]struct{}{}
	for i := range components {
		comp := &components[i]
		if comp.Namespace != cluster.Namespace || comp.Name == "" || comp.UID == "" ||
			comp.ResourceVersion == "" || comp.Generation <= 0 {
			return invalid("Component identity, generation, and resourceVersion must be complete")
		}
		if !comp.DeletionTimestamp.IsZero() {
			return invalid("Component is deleting: %s/%s", comp.Namespace, comp.Name)
		}
		if comp.Labels[constant.AppInstanceLabelKey] != cluster.Name ||
			comp.Labels[constant.KBAppShardingNameLabelKey] != shardingName ||
			comp.Labels[constant.ShardingDefLabelKey] != shardingDef.Name {
			return invalid("Component %s/%s does not carry exact Cluster, sharding, and definition labels",
				comp.Namespace, comp.Name)
		}
		owner := metav1.GetControllerOf(comp)
		if owner == nil || owner.APIVersion != appsv1.GroupVersion.String() ||
			owner.Kind != appsv1.ClusterKind || owner.Name != cluster.Name || owner.UID != cluster.UID {
			return invalid("Component %s/%s does not have the exact Cluster controller owner",
				comp.Namespace, comp.Name)
		}
		if comp.Annotations[shardingAddShardKey] != "" {
			return invalid("Component %s/%s has a pending shard-add", comp.Namespace, comp.Name)
		}
		shortName, err := component.ShortName(cluster.Name, comp.Name)
		if err != nil || shortName == "" || !strings.HasPrefix(shortName, shardingName+"-") {
			return invalid("Component %s/%s has an invalid short name", comp.Namespace, comp.Name)
		}
		if comp.Labels[constant.KBAppComponentLabelKey] != shortName {
			return invalid("Component %s/%s does not carry its exact component-name label",
				comp.Namespace, comp.Name)
		}
		if _, ok := names[comp.Name]; ok {
			return invalid("Component name %q is duplicated", comp.Name)
		}
		if _, ok := uids[comp.UID]; ok {
			return invalid("Component UID %q is duplicated", comp.UID)
		}
		if _, ok := shortNames[shortName]; ok {
			return invalid("Component short name %q is duplicated", shortName)
		}
		names[comp.Name] = struct{}{}
		uids[comp.UID] = struct{}{}
		shortNames[shortName] = struct{}{}
	}
	return nil
}

func listFreshShardingScaleInComponents(ctx context.Context, apiReader client.Reader,
	cluster *appsv1.Cluster, shardingName string,
) ([]appsv1.Component, error) {
	list := &appsv1.ComponentList{}
	if err := apiReader.List(ctx, list, client.InNamespace(cluster.Namespace)); err != nil {
		return nil, err
	}

	regularComponentNames := make(map[string]struct{}, len(cluster.Spec.ComponentSpecs))
	for i := range cluster.Spec.ComponentSpecs {
		if cluster.Spec.ComponentSpecs[i].Name != "" {
			regularComponentNames[component.FullName(cluster.Name, cluster.Spec.ComponentSpecs[i].Name)] =
				struct{}{}
		}
	}

	components := make([]appsv1.Component, 0)
	for i := range list.Items {
		comp := &list.Items[i]
		labelMatches := comp.Labels[constant.KBAppShardingNameLabelKey] == shardingName
		shortName, err := component.ShortName(cluster.Name, comp.Name)
		nameMatches := err == nil && strings.HasPrefix(shortName, shardingName+"-")
		_, regularComponent := regularComponentNames[comp.Name]
		if labelMatches || (nameMatches && !regularComponent) {
			components = append(components, *comp.DeepCopy())
		}
	}
	return components, nil
}

func flattenShardingScaleInDesiredComponents(clusterName string,
	desiredByTemplate map[string][]*appsv1.ClusterComponentSpec,
) ([]shardingScaleInDesiredComponent, error) {
	components := make([]shardingScaleInDesiredComponent, 0)
	names := map[string]struct{}{}
	for templateName, templateSpecs := range desiredByTemplate {
		if templateName == "" {
			templateName = shardingScaleInDefaultShardTemplate
		}
		for i := range templateSpecs {
			if templateSpecs[i] == nil || templateSpecs[i].Name == "" {
				return nil, fmt.Errorf("%w: desired Component name must not be empty",
					errInvalidShardingScaleInTopology)
			}
			spec := templateSpecs[i].DeepCopy()
			fullName := component.FullName(clusterName, spec.Name)
			if _, ok := names[fullName]; ok {
				return nil, fmt.Errorf("%w: desired Component name %q is duplicated",
					errInvalidShardingScaleInTopology, fullName)
			}
			names[fullName] = struct{}{}
			components = append(components, shardingScaleInDesiredComponent{
				ShardTemplateName: templateName,
				Spec:              *spec,
			})
		}
	}
	slices.SortFunc(components, func(a, b shardingScaleInDesiredComponent) int {
		return strings.Compare(a.Spec.Name, b.Spec.Name)
	})
	return components, nil
}

func sameShardingScaleInComponentSnapshot(a, b []appsv1.Component) bool {
	aCopy := deepCopyShardingScaleInComponents(a)
	bCopy := deepCopyShardingScaleInComponents(b)
	sortShardingScaleInComponents(aCopy)
	sortShardingScaleInComponents(bCopy)
	return reflect.DeepEqual(aCopy, bCopy)
}

func deepCopyShardingScaleInComponents(in []appsv1.Component) []appsv1.Component {
	out := make([]appsv1.Component, len(in))
	for i := range in {
		out[i] = *in[i].DeepCopy()
	}
	return out
}

func deepCopyShardingScaleInDesiredComponents(
	in []shardingScaleInDesiredComponent,
) []shardingScaleInDesiredComponent {
	out := make([]shardingScaleInDesiredComponent, len(in))
	for i := range in {
		out[i] = shardingScaleInDesiredComponent{
			ShardTemplateName: in[i].ShardTemplateName,
			Spec:              *in[i].Spec.DeepCopy(),
		}
	}
	return out
}

func sortShardingScaleInComponents(components []appsv1.Component) {
	slices.SortFunc(components, func(a, b appsv1.Component) int {
		return strings.Compare(a.Name, b.Name)
	})
}

func objectOwnerIdentity(name string, uid types.UID) string {
	return name + "\x00" + string(uid)
}

func ownerReferenceIdentity(owner *metav1.OwnerReference) string {
	if owner == nil {
		return ""
	}
	return objectOwnerIdentity(owner.Name, owner.UID)
}

func sameShardingScaleInWorkloadSnapshot(a, b []workloadsv1.InstanceSet) bool {
	aCopy := deepCopyShardingScaleInWorkloads(a)
	bCopy := deepCopyShardingScaleInWorkloads(b)
	sortShardingScaleInWorkloads(aCopy)
	sortShardingScaleInWorkloads(bCopy)
	return reflect.DeepEqual(aCopy, bCopy)
}

func sameShardingScaleInPodSnapshot(a, b []corev1.Pod) bool {
	aCopy := deepCopyShardingScaleInPods(a)
	bCopy := deepCopyShardingScaleInPods(b)
	sortShardingScaleInPods(aCopy)
	sortShardingScaleInPods(bCopy)
	return reflect.DeepEqual(aCopy, bCopy)
}

func sameShardingScaleInInstanceSnapshot(a, b []workloadsv1.Instance) bool {
	aCopy := deepCopyShardingScaleInInstances(a)
	bCopy := deepCopyShardingScaleInInstances(b)
	sortShardingScaleInInstances(aCopy)
	sortShardingScaleInInstances(bCopy)
	return reflect.DeepEqual(aCopy, bCopy)
}

func deepCopyShardingScaleInTopologyInventory(
	in *shardingScaleInTopologyInventory,
) *shardingScaleInTopologyInventory {
	if in == nil {
		return nil
	}
	return &shardingScaleInTopologyInventory{
		Cluster:            in.Cluster.DeepCopy(),
		Sharding:           *in.Sharding.DeepCopy(),
		ShardingDefinition: in.ShardingDefinition.DeepCopy(),
		Components:         deepCopyShardingScaleInComponents(in.Components),
		DesiredComponents:  deepCopyShardingScaleInDesiredComponents(in.DesiredComponents),
		Leaving:            deepCopyShardingScaleInComponents(in.Leaving),
		Staying:            deepCopyShardingScaleInComponents(in.Staying),
	}
}

func deepCopyShardingScaleInMemberInventory(
	in *shardingScaleInMemberInventory,
) *shardingScaleInMemberInventory {
	if in == nil {
		return nil
	}
	return &shardingScaleInMemberInventory{
		Topology: deepCopyShardingScaleInTopologyInventory(in.Topology),
		Leaving:  deepCopyShardingScaleInSourceMembers(in.Leaving),
		Staying:  deepCopyShardingScaleInSourceMembers(in.Staying),
	}
}

func deepCopyShardingScaleInDNSInventory(
	in *shardingScaleInDNSInventory,
) *shardingScaleInDNSInventory {
	if in == nil {
		return nil
	}
	return &shardingScaleInDNSInventory{
		Members: deepCopyShardingScaleInMemberInventory(in.Members),
		Leaving: deepCopyShardingScaleInDNSRecords(in.Leaving),
		Staying: deepCopyShardingScaleInDNSRecords(in.Staying),
	}
}

func deepCopyShardingScaleInDNSRecords(
	in []shardingScaleInSourceDNS,
) []shardingScaleInSourceDNS {
	out := make([]shardingScaleInSourceDNS, len(in))
	for i := range in {
		out[i] = shardingScaleInSourceDNS{
			Member:         deepCopyShardingScaleInSourceMember(in[i].Member),
			Service:        *in[i].Service.DeepCopy(),
			EndpointSlices: deepCopyShardingScaleInEndpointSlices(in[i].EndpointSlices),
		}
		if in[i].Endpoints != nil {
			out[i].Endpoints = in[i].Endpoints.DeepCopy()
		}
	}
	return out
}

func deepCopyShardingScaleInSourceMember(
	in shardingScaleInSourceMember,
) shardingScaleInSourceMember {
	return shardingScaleInSourceMember{
		Component:         *in.Component.DeepCopy(),
		ShardTemplateName: in.ShardTemplateName,
		Workload:          *in.Workload.DeepCopy(),
		Instances:         deepCopyShardingScaleInInstances(in.Instances),
		Pods:              deepCopyShardingScaleInSourcePods(in.Pods),
	}
}

func deepCopyShardingScaleInEndpointSlices(
	in []discoveryv1.EndpointSlice,
) []discoveryv1.EndpointSlice {
	out := make([]discoveryv1.EndpointSlice, len(in))
	for i := range in {
		out[i] = *in[i].DeepCopy()
	}
	return out
}

func sameShardingScaleInDNSSnapshot(
	a, b *shardingScaleInDNSInventory,
) bool {
	aCopy := deepCopyShardingScaleInDNSInventory(a)
	bCopy := deepCopyShardingScaleInDNSInventory(b)
	if aCopy != nil {
		aCopy.Members = nil
		sortShardingScaleInDNSRecords(aCopy.Leaving)
		sortShardingScaleInDNSRecords(aCopy.Staying)
	}
	if bCopy != nil {
		bCopy.Members = nil
		sortShardingScaleInDNSRecords(bCopy.Leaving)
		sortShardingScaleInDNSRecords(bCopy.Staying)
	}
	return reflect.DeepEqual(aCopy, bCopy)
}

func deepCopyShardingScaleInWorkloads(in []workloadsv1.InstanceSet) []workloadsv1.InstanceSet {
	out := make([]workloadsv1.InstanceSet, len(in))
	for i := range in {
		out[i] = *in[i].DeepCopy()
	}
	return out
}

func deepCopyShardingScaleInInstances(in []workloadsv1.Instance) []workloadsv1.Instance {
	out := make([]workloadsv1.Instance, len(in))
	for i := range in {
		out[i] = *in[i].DeepCopy()
	}
	return out
}

func deepCopyShardingScaleInPods(in []corev1.Pod) []corev1.Pod {
	out := make([]corev1.Pod, len(in))
	for i := range in {
		out[i] = *in[i].DeepCopy()
	}
	return out
}

func deepCopyShardingScaleInSourcePods(in []shardingScaleInSourcePod) []shardingScaleInSourcePod {
	out := make([]shardingScaleInSourcePod, len(in))
	for i := range in {
		out[i] = shardingScaleInSourcePod{
			Pod:  *in[i].Pod.DeepCopy(),
			FQDN: in[i].FQDN,
		}
	}
	return out
}

func deepCopyShardingScaleInSourceMembers(
	in []shardingScaleInSourceMember,
) []shardingScaleInSourceMember {
	out := make([]shardingScaleInSourceMember, len(in))
	for i := range in {
		out[i] = deepCopyShardingScaleInSourceMember(in[i])
	}
	return out
}

func sortShardingScaleInWorkloads(workloads []workloadsv1.InstanceSet) {
	slices.SortFunc(workloads, func(a, b workloadsv1.InstanceSet) int {
		return strings.Compare(a.Name, b.Name)
	})
}

func sortShardingScaleInInstances(instances []workloadsv1.Instance) {
	slices.SortFunc(instances, func(a, b workloadsv1.Instance) int {
		return strings.Compare(a.Name, b.Name)
	})
}

func sortShardingScaleInPods(pods []corev1.Pod) {
	slices.SortFunc(pods, func(a, b corev1.Pod) int {
		return strings.Compare(a.Name, b.Name)
	})
}

func sortShardingScaleInSourcePods(pods []shardingScaleInSourcePod) {
	slices.SortFunc(pods, func(a, b shardingScaleInSourcePod) int {
		return strings.Compare(a.Pod.Name, b.Pod.Name)
	})
}

func sortShardingScaleInSourceMembers(members []shardingScaleInSourceMember) {
	slices.SortFunc(members, func(a, b shardingScaleInSourceMember) int {
		return strings.Compare(a.Component.Name, b.Component.Name)
	})
}

func sortShardingScaleInDNSRecords(records []shardingScaleInSourceDNS) {
	for i := range records {
		sortShardingScaleInEndpointSlices(records[i].EndpointSlices)
	}
	slices.SortFunc(records, func(a, b shardingScaleInSourceDNS) int {
		return strings.Compare(a.Service.Name, b.Service.Name)
	})
}

func sortShardingScaleInEndpointSlices(endpointSlices []discoveryv1.EndpointSlice) {
	slices.SortFunc(endpointSlices, func(a, b discoveryv1.EndpointSlice) int {
		return strings.Compare(a.Name, b.Name)
	})
}
