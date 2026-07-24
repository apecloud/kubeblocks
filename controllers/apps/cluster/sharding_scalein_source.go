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
	"reflect"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
)

const shardingScaleInDefaultShardTemplate = "@default"

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
	podsBefore, err := listFreshShardingScaleInPods(ctx, apiReader, topologyBefore, workloadsBefore)
	if err != nil {
		return nil, err
	}
	inventory, err := buildFreshShardingScaleInMembers(topologyBefore, workloadsBefore, podsBefore)
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
	podsAfter, err := listFreshShardingScaleInPods(ctx, apiReader, topologyBefore, workloadsAfter)
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

func listFreshShardingScaleInPods(ctx context.Context, apiReader client.Reader,
	topology *shardingScaleInTopologyInventory, workloads []workloadsv1.InstanceSet,
) ([]corev1.Pod, error) {
	list := &corev1.PodList{}
	if err := apiReader.List(ctx, list, client.InNamespace(topology.Cluster.Namespace)); err != nil {
		return nil, err
	}

	workloadsByOwner := make(map[string]struct{}, len(workloads))
	componentsByShortName := make(map[string]struct{}, len(topology.Components))
	for i := range workloads {
		workloadsByOwner[objectOwnerIdentity(workloads[i].Name, workloads[i].UID)] = struct{}{}
	}
	for i := range topology.Components {
		componentsByShortName[topology.Components[i].Labels[constant.KBAppComponentLabelKey]] = struct{}{}
	}

	result := make([]corev1.Pod, 0)
	for i := range list.Items {
		pod := &list.Items[i]
		owner := metav1.GetControllerOf(pod)
		_, ownerMatches := workloadsByOwner[ownerReferenceIdentity(owner)]
		_, componentMatches := componentsByShortName[pod.Labels[constant.KBAppComponentLabelKey]]
		labelMatches := pod.Labels[constant.AppInstanceLabelKey] == topology.Cluster.Name &&
			componentMatches
		if ownerMatches || labelMatches {
			result = append(result, *pod.DeepCopy())
		}
	}
	sortShardingScaleInPods(result)
	return result, nil
}

func buildFreshShardingScaleInMembers(topology *shardingScaleInTopologyInventory,
	workloads []workloadsv1.InstanceSet, pods []corev1.Pod,
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
			workload.Name != comp.Name {
			return nil, invalid("InstanceSet labels and name do not identify one exact Component")
		}
		owner := metav1.GetControllerOf(workload)
		if owner == nil || owner.APIVersion != appsv1.GroupVersion.String() ||
			owner.Kind != appsv1.ComponentKind || owner.Name != comp.Name || owner.UID != comp.UID {
			return nil, invalid("InstanceSet controller owner does not identify the exact Component")
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

	podsByComponentUID := make(map[types.UID][]shardingScaleInSourcePod, len(topology.Components))
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
			pod.Labels[constant.AppInstanceLabelKey] != topology.Cluster.Name {
			return nil, invalid("Pod labels do not identify one exact Component")
		}
		workload := workloadsByComponentUID[comp.UID]
		if workload == nil ||
			pod.Labels[instanceset.WorkloadsManagedByLabelKey] != workloadsv1.InstanceSetKind ||
			pod.Labels[instanceset.WorkloadsInstanceLabelKey] != workload.Name {
			return nil, invalid("Pod labels do not identify the exact InstanceSet")
		}
		owner := metav1.GetControllerOf(pod)
		if owner == nil || owner.APIVersion != workloadsv1.GroupVersion.String() ||
			owner.Kind != workloadsv1.InstanceSetKind || owner.Name != workload.Name ||
			owner.UID != workload.UID {
			return nil, invalid("Pod controller owner does not identify the exact InstanceSet")
		}

		fqdn := intctrlutil.PodFQDN(pod.Namespace, comp.Name, pod.Name)
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

func deepCopyShardingScaleInWorkloads(in []workloadsv1.InstanceSet) []workloadsv1.InstanceSet {
	out := make([]workloadsv1.InstanceSet, len(in))
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
		out[i] = shardingScaleInSourceMember{
			Component:         *in[i].Component.DeepCopy(),
			ShardTemplateName: in[i].ShardTemplateName,
			Workload:          *in[i].Workload.DeepCopy(),
			Pods:              deepCopyShardingScaleInSourcePods(in[i].Pods),
		}
	}
	return out
}

func sortShardingScaleInWorkloads(workloads []workloadsv1.InstanceSet) {
	slices.SortFunc(workloads, func(a, b workloadsv1.InstanceSet) int {
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
