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
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
)

var errInvalidShardingScaleInRequestAuthoritySource = errors.New("invalid sharding scale-in request authority source")

const shardingScaleInRequestSourceProjectionVersionV1 = "kb.sharding.scalein.request-source-projection/v1"

// shardingScaleInRequestAuthorityMaterial remains source-only. No status write
// or lifecycle action may consume it until the complete plan builder succeeds.
type shardingScaleInRequestAuthorityMaterial struct {
	PodRuntimeBindings []shardingScaleInPodRuntimeBinding
	RequestAuthority   appsv1.ShardingScaleInRequestAuthority
}

// shardingScaleInAgentCapability is an injected, read-only kbagent capability
// snapshot. The production capability transport is intentionally outside this
// source builder.
type shardingScaleInAgentCapability struct {
	PodUID types.UID `json:"podUID"`

	AgentProcessUID        string `json:"agentProcessUID"`
	AgentCapabilityDigest  string `json:"agentCapabilityDigest"`
	RegisteredActionDigest string `json:"registeredActionDigest"`

	StartupEnvironmentSchemaDigest string `json:"startupEnvironmentSchemaDigest"`
	LaunchSchemaDigest             string `json:"launchSchemaDigest"`
	PollSchemaDigest               string `json:"pollSchemaDigest"`
	CancelSchemaDigest             string `json:"cancelSchemaDigest"`

	BaseParameterDigest       string `json:"baseParameterDigest"`
	ServerConfigurationDigest string `json:"serverConfigurationDigest"`

	CredentialBindings []appsv1.ShardingScaleInExecutorCredentialBinding `json:"credentialBindings"`
}

type shardingScaleInAgentCapabilityReader interface {
	ReadShardingScaleInCapability(
		context.Context, types.NamespacedName, types.UID,
	) (shardingScaleInAgentCapability, error)
}

type shardingScaleInRequestAuthorityMember struct {
	Component appsv1.Component
	Workload  workloadsv1.InstanceSet
	Pods      []shardingScaleInSourcePod
	ShortName string
}

type shardingScaleInCredentialRequirement struct {
	VariableName string
	AccountName  string
	KeyName      string
	Selector     appsv1.CredentialVarSelector
}

type shardingScaleInCredentialResolution struct {
	VariableName             string
	CredentialSourceID       string
	CredentialSourceDigest   string
	RequiredKeyNames         []string
	ResolverProjectionDigest string
}

type shardingScaleInPodCapabilitySource struct {
	Pod        shardingScaleInLivePodRuntime
	Capability shardingScaleInAgentCapability
}

type shardingScaleInLivePodRuntime struct {
	Namespace       string
	Name            string
	UID             types.UID
	ResourceVersion string
	AgentImageID    string
}

func loadFreshShardingScaleInRequestAuthority(
	ctx context.Context,
	apiReader client.Reader,
	inventory *shardingScaleInDNSInventory,
	actionDigest string,
	capabilityReader shardingScaleInAgentCapabilityReader,
) (*shardingScaleInRequestAuthorityMaterial, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInRequestAuthoritySource,
			fmt.Sprintf(format, args...))
	}
	if apiReader == nil || capabilityReader == nil {
		return nil, invalid("APIReader and capability reader must not be nil")
	}
	if inventory == nil || inventory.Members == nil || inventory.Members.Topology == nil ||
		inventory.Members.Topology.Cluster == nil {
		return nil, invalid("DNS, member, topology, and Cluster inventory must be complete")
	}
	if !isShardingScaleInSHA256(actionDigest) {
		return nil, invalid("action digest must be a SHA256 digest")
	}
	cluster := inventory.Members.Topology.Cluster
	if cluster.Namespace == "" || cluster.Name == "" || cluster.UID == "" ||
		cluster.Generation <= 0 || cluster.ResourceVersion == "" ||
		!cluster.DeletionTimestamp.IsZero() {
		return nil, invalid("Cluster source identity must be complete and not deleting")
	}

	members, err := collectShardingScaleInRequestAuthorityMembers(inventory, cluster)
	if err != nil {
		return nil, err
	}
	podSources, err := loadShardingScaleInPodCapabilitySources(
		ctx, apiReader, capabilityReader, members, actionDigest)
	if err != nil {
		return nil, err
	}
	componentDefinitionSources, vars, err :=
		loadShardingScaleInComponentDefinitionVars(ctx, apiReader, members)
	if err != nil {
		return nil, err
	}
	varSources, servicePort, credentialRequirements, err :=
		buildShardingScaleInVarSources(cluster, members, vars)
	if err != nil {
		return nil, err
	}
	credentialSources, resolutionsByComponent, err :=
		loadShardingScaleInCredentialSources(
			ctx, apiReader, cluster, members, componentDefinitionSources,
			credentialRequirements)
	if err != nil {
		return nil, err
	}

	baseRecord, baseDigest, err := buildShardingScaleInBaseParameterRecord(servicePort)
	if err != nil {
		return nil, err
	}
	result := &shardingScaleInRequestAuthorityMaterial{
		PodRuntimeBindings: make([]shardingScaleInPodRuntimeBinding, 0, len(podSources)),
		RequestAuthority: appsv1.ShardingScaleInRequestAuthority{
			Version:                            appsv1.ShardingScaleInRequestAuthorityVersionV2,
			Builder:                            appsv1.ShardingScaleInRequestBuilderTypedV2,
			GenericLifecycleSynthesisForbidden: true,
			ActionName:                         shardingRemoveShardAction,
			ActionDefinitionDigest:             actionDigest,
			ComponentDefinitionSources:         componentDefinitionSources,
			VarSources:                         varSources,
			CredentialSources:                  credentialSources,
			ExecutorTemplates: make([]appsv1.ShardingScaleInExecutorTemplate, 0,
				len(podSources)),
		},
	}
	for _, member := range members {
		for _, sourcePod := range member.Pods {
			podSource, ok := podSources[sourcePod.Pod.UID]
			if !ok {
				return nil, invalid("exact Pod capability source is missing")
			}
			bindings, err := buildShardingScaleInExecutorCredentialBindings(
				podSource.Pod.UID, member.Component.UID,
				resolutionsByComponent[member.Component.UID])
			if err != nil {
				return nil, err
			}
			capability := podSource.Capability
			if capability.BaseParameterDigest != baseDigest {
				return nil, invalid("executor %q capability base parameter digest does not match",
					podSource.Pod.Name)
			}
			if !reflect.DeepEqual(capability.CredentialBindings, bindings) {
				return nil, invalid("executor %q capability credential bindings do not match",
					podSource.Pod.Name)
			}
			expectedServerDigest, err :=
				digestShardingScaleInAgentServerConfiguration(capability)
			if err != nil {
				return nil, err
			}
			if capability.ServerConfigurationDigest != expectedServerDigest {
				return nil, invalid("executor %q server configuration digest does not match",
					podSource.Pod.Name)
			}

			result.PodRuntimeBindings = append(result.PodRuntimeBindings,
				shardingScaleInPodRuntimeBinding{
					PodUID:                podSource.Pod.UID,
					AgentImageID:          podSource.Pod.AgentImageID,
					AgentProcessUID:       capability.AgentProcessUID,
					AgentCapabilityDigest: capability.AgentCapabilityDigest,
				})
			result.RequestAuthority.ExecutorTemplates = append(
				result.RequestAuthority.ExecutorTemplates,
				appsv1.ShardingScaleInExecutorTemplate{
					ExecutorPodUID:         podSource.Pod.UID,
					ExecutorComponentUID:   member.Component.UID,
					CredentialBindings:     bindings,
					BaseParameterRecordB64: baseRecord,
					BaseParameterDigest:    baseDigest,
					LaunchSchemaDigest:     capability.LaunchSchemaDigest,
					PollSchemaDigest:       capability.PollSchemaDigest,
					CancelSchemaDigest:     capability.CancelSchemaDigest,
					ServerRuntimeBinding: appsv1.ShardingScaleInServerRuntimeBinding{
						AgentProcessUID:                capability.AgentProcessUID,
						AgentImageID:                   podSource.Pod.AgentImageID,
						RegisteredActionDigest:         capability.RegisteredActionDigest,
						StartupEnvironmentSchemaDigest: capability.StartupEnvironmentSchemaDigest,
						ServerConfigurationDigest:      capability.ServerConfigurationDigest,
					},
				})
		}
	}
	slices.SortFunc(result.PodRuntimeBindings,
		func(a, b shardingScaleInPodRuntimeBinding) int {
			return strings.Compare(string(a.PodUID), string(b.PodUID))
		})
	slices.SortFunc(result.RequestAuthority.ExecutorTemplates,
		func(a, b appsv1.ShardingScaleInExecutorTemplate) int {
			return strings.Compare(string(a.ExecutorPodUID), string(b.ExecutorPodUID))
		})
	return result, nil
}

func collectShardingScaleInRequestAuthorityMembers(
	inventory *shardingScaleInDNSInventory,
	cluster *appsv1.Cluster,
) ([]shardingScaleInRequestAuthorityMember, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInRequestAuthoritySource,
			fmt.Sprintf(format, args...))
	}
	records := make([]shardingScaleInSourceDNS, 0,
		len(inventory.Leaving)+len(inventory.Staying))
	records = append(records, inventory.Leaving...)
	records = append(records, inventory.Staying...)
	if len(records) == 0 {
		return nil, invalid("request authority members must not be empty")
	}
	members := make([]shardingScaleInRequestAuthorityMember, 0, len(records))
	componentUIDs := map[types.UID]struct{}{}
	podUIDs := map[types.UID]struct{}{}
	for i := range records {
		member := records[i].Member
		if member.Component.Namespace != cluster.Namespace ||
			member.Component.Name == "" || member.Component.UID == "" ||
			member.Component.Generation <= 0 || member.Component.ResourceVersion == "" ||
			!member.Component.DeletionTimestamp.IsZero() ||
			member.Component.Spec.CompDef == "" ||
			member.Workload.Namespace != cluster.Namespace ||
			member.Workload.Name == "" || member.Workload.UID == "" ||
			member.Workload.ResourceVersion == "" ||
			!member.Workload.DeletionTimestamp.IsZero() ||
			len(member.Pods) == 0 {
			return nil, invalid("Component, InstanceSet, and Pod source identity must be complete")
		}
		if _, ok := componentUIDs[member.Component.UID]; ok {
			return nil, invalid("Component UID %q is duplicated", member.Component.UID)
		}
		shortName, err := component.ShortName(cluster.Name, member.Component.Name)
		if err != nil || shortName == "" {
			return nil, invalid("Component %q does not belong to Cluster %q",
				member.Component.Name, cluster.Name)
		}
		for _, sourcePod := range member.Pods {
			pod := sourcePod.Pod
			if pod.Namespace != cluster.Namespace || pod.Name == "" || pod.UID == "" ||
				pod.ResourceVersion == "" || !pod.DeletionTimestamp.IsZero() {
				return nil, invalid("Pod source identity must be complete and not deleting")
			}
			if _, ok := podUIDs[pod.UID]; ok {
				return nil, invalid("Pod UID %q is duplicated", pod.UID)
			}
			podUIDs[pod.UID] = struct{}{}
		}
		componentUIDs[member.Component.UID] = struct{}{}
		members = append(members, shardingScaleInRequestAuthorityMember{
			Component: *member.Component.DeepCopy(),
			Workload:  *member.Workload.DeepCopy(),
			Pods:      deepCopyShardingScaleInSourcePods(member.Pods),
			ShortName: shortName,
		})
	}
	slices.SortFunc(members, func(a, b shardingScaleInRequestAuthorityMember) int {
		return strings.Compare(
			a.Component.Name+"\x00"+string(a.Component.UID),
			b.Component.Name+"\x00"+string(b.Component.UID))
	})
	return members, nil
}

func loadShardingScaleInPodCapabilitySources(
	ctx context.Context,
	apiReader client.Reader,
	capabilityReader shardingScaleInAgentCapabilityReader,
	members []shardingScaleInRequestAuthorityMember,
	actionDigest string,
) (map[types.UID]shardingScaleInPodCapabilitySource, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInRequestAuthoritySource,
			fmt.Sprintf(format, args...))
	}
	result := map[types.UID]shardingScaleInPodCapabilitySource{}
	for _, member := range members {
		for _, sourcePod := range member.Pods {
			key := types.NamespacedName{
				Namespace: sourcePod.Pod.Namespace,
				Name:      sourcePod.Pod.Name,
			}
			before, err := readShardingScaleInLivePodRuntime(ctx, apiReader, key, sourcePod.Pod.UID)
			if err != nil {
				return nil, err
			}
			sourceSnapshot, err := snapshotShardingScaleInLivePodRuntime(&sourcePod.Pod)
			if err != nil {
				return nil, err
			}
			if !reflect.DeepEqual(sourceSnapshot, before) {
				return nil, invalid("Pod snapshot no longer matches the source inventory")
			}
			capabilityBefore, err := capabilityReader.ReadShardingScaleInCapability(
				ctx, key, sourcePod.Pod.UID)
			if err != nil {
				return nil, err
			}
			capabilityAfter, err := capabilityReader.ReadShardingScaleInCapability(
				ctx, key, sourcePod.Pod.UID)
			if err != nil {
				return nil, err
			}
			if !reflect.DeepEqual(capabilityBefore, capabilityAfter) {
				return nil, invalid("capability snapshot changed for Pod %q", sourcePod.Pod.Name)
			}
			if err := validateShardingScaleInAgentCapability(
				capabilityBefore, sourcePod.Pod.UID, actionDigest); err != nil {
				return nil, fmt.Errorf("Pod %q capability is invalid: %w",
					sourcePod.Pod.Name, err)
			}
			after, err := readShardingScaleInLivePodRuntime(ctx, apiReader, key, sourcePod.Pod.UID)
			if err != nil {
				return nil, err
			}
			if !reflect.DeepEqual(before, after) {
				return nil, invalid("Pod snapshot changed for Pod %q", sourcePod.Pod.Name)
			}
			result[sourcePod.Pod.UID] = shardingScaleInPodCapabilitySource{
				Pod:        before,
				Capability: capabilityBefore,
			}
		}
	}
	return result, nil
}

func readShardingScaleInLivePodRuntime(
	ctx context.Context,
	apiReader client.Reader,
	key types.NamespacedName,
	expectedUID types.UID,
) (shardingScaleInLivePodRuntime, error) {
	pod := &corev1.Pod{}
	if err := apiReader.Get(ctx, key, pod); err != nil {
		return shardingScaleInLivePodRuntime{}, err
	}
	if pod.Namespace != key.Namespace || pod.Name != key.Name || pod.UID != expectedUID {
		return shardingScaleInLivePodRuntime{}, fmt.Errorf(
			"%w: fresh Pod identity does not match",
			errInvalidShardingScaleInRequestAuthoritySource)
	}
	return snapshotShardingScaleInLivePodRuntime(pod)
}

func snapshotShardingScaleInLivePodRuntime(
	pod *corev1.Pod,
) (shardingScaleInLivePodRuntime, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInRequestAuthoritySource,
			fmt.Sprintf(format, args...))
	}
	if pod == nil || pod.Namespace == "" || pod.Name == "" || pod.UID == "" ||
		pod.ResourceVersion == "" || !pod.DeletionTimestamp.IsZero() ||
		pod.Status.Phase != corev1.PodRunning {
		return shardingScaleInLivePodRuntime{},
			invalid("fresh Pod identity must be complete, running, and not deleting")
	}
	var agentStatus *corev1.ContainerStatus
	for i := range pod.Status.ContainerStatuses {
		if pod.Status.ContainerStatuses[i].Name != kbagent.ContainerName {
			continue
		}
		if agentStatus != nil {
			return shardingScaleInLivePodRuntime{},
				invalid("fresh Pod has duplicate kbagent container status")
		}
		agentStatus = &pod.Status.ContainerStatuses[i]
	}
	if agentStatus == nil || agentStatus.State.Running == nil ||
		!isShardingScaleInImageID(agentStatus.ImageID) {
		return shardingScaleInLivePodRuntime{},
			invalid("fresh Pod kbagent runtime identity must be running and digest-qualified")
	}
	return shardingScaleInLivePodRuntime{
		Namespace:       pod.Namespace,
		Name:            pod.Name,
		UID:             pod.UID,
		ResourceVersion: pod.ResourceVersion,
		AgentImageID:    agentStatus.ImageID,
	}, nil
}

func validateShardingScaleInAgentCapability(
	capability shardingScaleInAgentCapability,
	expectedPodUID types.UID,
	actionDigest string,
) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInRequestAuthoritySource,
			fmt.Sprintf(format, args...))
	}
	if capability.PodUID != expectedPodUID || len(capability.AgentProcessUID) < 22 ||
		capability.RegisteredActionDigest != actionDigest ||
		!isShardingScaleInSHA256(capability.StartupEnvironmentSchemaDigest) ||
		!isShardingScaleInSHA256(capability.LaunchSchemaDigest) ||
		!isShardingScaleInSHA256(capability.PollSchemaDigest) ||
		!isShardingScaleInSHA256(capability.CancelSchemaDigest) ||
		!isShardingScaleInSHA256(capability.BaseParameterDigest) ||
		!isShardingScaleInSHA256(capability.ServerConfigurationDigest) ||
		!isShardingScaleInSHA256(capability.AgentCapabilityDigest) {
		return invalid("agent capability identity and digests must be complete")
	}
	for i, binding := range capability.CredentialBindings {
		if binding.VariableName == "" ||
			!isShardingScaleInSHA256(binding.CredentialSourceID) ||
			!isShardingScaleInSHA256(binding.CredentialSourceDigest) ||
			!isShardingScaleInSHA256(binding.ResolverProjectionDigest) ||
			!isShardingScaleInSHA256(binding.BindingDigest) ||
			len(binding.RequiredKeyNames) == 0 ||
			(i > 0 && capability.CredentialBindings[i-1].VariableName >= binding.VariableName) {
			return invalid("agent capability credential bindings must be sorted, unique, and complete")
		}
		for j, keyName := range binding.RequiredKeyNames {
			if keyName == "" || (j > 0 && binding.RequiredKeyNames[j-1] >= keyName) {
				return invalid("agent capability credential keys must be sorted and unique")
			}
		}
	}
	digest, err := digestShardingScaleInAgentCapability(capability)
	if err != nil {
		return err
	}
	if digest != capability.AgentCapabilityDigest {
		return invalid("agent capability digest does not match")
	}
	return nil
}

func digestShardingScaleInAgentCapability(
	capability shardingScaleInAgentCapability,
) (string, error) {
	capability.AgentCapabilityDigest = ""
	return digestShardingScaleInCanonicalJSON(capability)
}

func digestShardingScaleInAgentServerConfiguration(
	capability shardingScaleInAgentCapability,
) (string, error) {
	return digestShardingScaleInServerConfiguration(
		capability.PodUID,
		capability.RegisteredActionDigest,
		capability.StartupEnvironmentSchemaDigest,
		capability.BaseParameterDigest,
		capability.LaunchSchemaDigest,
		capability.PollSchemaDigest,
		capability.CancelSchemaDigest,
		capability.CredentialBindings,
	)
}

func digestShardingScaleInServerConfiguration(
	podUID types.UID,
	registeredActionDigest,
	startupEnvironmentSchemaDigest,
	baseParameterDigest,
	launchSchemaDigest,
	pollSchemaDigest,
	cancelSchemaDigest string,
	credentialBindings []appsv1.ShardingScaleInExecutorCredentialBinding,
) (string, error) {
	bindingDigests := make([]string, 0, len(credentialBindings))
	for _, binding := range credentialBindings {
		bindingDigests = append(bindingDigests, binding.BindingDigest)
	}
	slices.Sort(bindingDigests)
	return digestShardingScaleInCanonicalJSON(struct {
		Version                        string    `json:"version"`
		PodUID                         types.UID `json:"podUID"`
		RegisteredActionDigest         string    `json:"registeredActionDigest"`
		StartupEnvironmentSchemaDigest string    `json:"startupEnvironmentSchemaDigest"`
		BaseParameterDigest            string    `json:"baseParameterDigest"`
		LaunchSchemaDigest             string    `json:"launchSchemaDigest"`
		PollSchemaDigest               string    `json:"pollSchemaDigest"`
		CancelSchemaDigest             string    `json:"cancelSchemaDigest"`
		CredentialBindingDigests       []string  `json:"credentialBindingDigests"`
	}{
		Version:                        "kb.sharding.scalein.agent-server-configuration/v1",
		PodUID:                         podUID,
		RegisteredActionDigest:         registeredActionDigest,
		StartupEnvironmentSchemaDigest: startupEnvironmentSchemaDigest,
		BaseParameterDigest:            baseParameterDigest,
		LaunchSchemaDigest:             launchSchemaDigest,
		PollSchemaDigest:               pollSchemaDigest,
		CancelSchemaDigest:             cancelSchemaDigest,
		CredentialBindingDigests:       bindingDigests,
	})
}

func loadShardingScaleInComponentDefinitionVars(
	ctx context.Context,
	apiReader client.Reader,
	members []shardingScaleInRequestAuthorityMember,
) ([]appsv1.ShardingScaleInComponentDefinitionSource, []appsv1.EnvVar, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInRequestAuthoritySource,
			fmt.Sprintf(format, args...))
	}
	names := map[string]struct{}{}
	for _, member := range members {
		names[member.Component.Spec.CompDef] = struct{}{}
	}
	sortedNames := make([]string, 0, len(names))
	for name := range names {
		sortedNames = append(sortedNames, name)
	}
	slices.Sort(sortedNames)

	sources := make([]appsv1.ShardingScaleInComponentDefinitionSource, 0, len(sortedNames))
	var commonVars []appsv1.EnvVar
	for _, name := range sortedNames {
		key := types.NamespacedName{Name: name}
		before := &appsv1.ComponentDefinition{}
		if err := apiReader.Get(ctx, key, before); err != nil {
			return nil, nil, err
		}
		after := &appsv1.ComponentDefinition{}
		if err := apiReader.Get(ctx, key, after); err != nil {
			return nil, nil, err
		}
		if before.Name != name || before.Namespace != "" || before.UID == "" ||
			before.Generation <= 0 || before.ResourceVersion == "" ||
			!before.DeletionTimestamp.IsZero() ||
			after.Name != before.Name || after.UID != before.UID ||
			after.Generation != before.Generation ||
			after.ResourceVersion != before.ResourceVersion ||
			!after.DeletionTimestamp.IsZero() ||
			!reflect.DeepEqual(before.Spec.Vars, after.Spec.Vars) {
			return nil, nil, invalid("ComponentDefinition snapshot changed or is incomplete")
		}
		if commonVars == nil {
			commonVars = make([]appsv1.EnvVar, len(before.Spec.Vars))
			for i := range before.Spec.Vars {
				before.Spec.Vars[i].DeepCopyInto(&commonVars[i])
			}
		} else if !reflect.DeepEqual(commonVars, before.Spec.Vars) {
			return nil, nil, invalid("ComponentDefinitions must declare one exact variable contract")
		}
		varsDigest, err := digestShardingScaleInCanonicalJSON(struct {
			Version string          `json:"version"`
			Vars    []appsv1.EnvVar `json:"vars"`
		}{
			Version: shardingScaleInRequestSourceProjectionVersionV1,
			Vars:    before.Spec.Vars,
		})
		if err != nil {
			return nil, nil, err
		}
		sources = append(sources, appsv1.ShardingScaleInComponentDefinitionSource{
			Name:                 before.Name,
			UID:                  before.UID,
			Generation:           before.Generation,
			VarsProjectionDigest: varsDigest,
		})
	}
	if len(commonVars) == 0 {
		return nil, nil, invalid("ComponentDefinition vars must not be empty")
	}
	return sources, commonVars, nil
}

func buildShardingScaleInVarSources(
	cluster *appsv1.Cluster,
	members []shardingScaleInRequestAuthorityMember,
	vars []appsv1.EnvVar,
) ([]appsv1.ShardingScaleInVarSource, string,
	[]shardingScaleInCredentialRequirement, error,
) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInRequestAuthoritySource,
			fmt.Sprintf(format, args...))
	}
	sources := make([]appsv1.ShardingScaleInVarSource, 0, len(vars))
	credentialRequirements := make([]shardingScaleInCredentialRequirement, 0)
	names := map[string]struct{}{}
	servicePort := ""
	for _, variable := range vars {
		if variable.Name == "" {
			return nil, "", nil, invalid("variable name must not be empty")
		}
		if _, ok := names[variable.Name]; ok {
			return nil, "", nil, invalid("variable %q is duplicated", variable.Name)
		}
		names[variable.Name] = struct{}{}
		if variable.Expression != nil {
			return nil, "", nil, invalid("variable %q expression is unsupported", variable.Name)
		}
		source := appsv1.ShardingScaleInVarSource{
			VariableName:  variable.Name,
			Consumption:   appsv1.ShardingScaleInVarConsumptionForbidden,
			SourceObjects: []appsv1.ShardingScaleInRequestSourceObject{},
		}
		if variable.ValueFrom == nil {
			source.ResolverKind = appsv1.ShardingScaleInVarResolverStaticValue
			if variable.Name == shardingScaleInBaseParameterServicePort {
				if variable.Value == "" {
					return nil, "", nil, invalid("SERVICE_PORT static value must not be empty")
				}
				encoded := base64.StdEncoding.EncodeToString([]byte(variable.Value))
				digest, err := digestShardingScaleInCanonicalBytes([]byte(variable.Value))
				if err != nil {
					return nil, "", nil, err
				}
				source.Consumption = appsv1.ShardingScaleInVarConsumptionTypedBase
				source.ResolvedNonSecretValueB64 = encoded
				source.ResolvedNonSecretValueDigest = digest
				servicePort = variable.Value
			}
			sources = append(sources, source)
			continue
		}
		if variable.Value != "" {
			return nil, "", nil, invalid("variable %q cannot set value and valueFrom", variable.Name)
		}
		resolverCount := countShardingScaleInVarResolvers(variable.ValueFrom)
		if resolverCount != 1 {
			return nil, "", nil, invalid("variable %q must have exactly one resolver", variable.Name)
		}
		switch {
		case variable.ValueFrom.CredentialVarRef != nil:
			selector := variable.ValueFrom.CredentialVarRef
			requirement, err := buildShardingScaleInCredentialRequirement(variable.Name, selector)
			if err != nil {
				return nil, "", nil, err
			}
			source.ResolverKind = appsv1.ShardingScaleInVarResolverCredentialVarRef
			source.Consumption = appsv1.ShardingScaleInVarConsumptionServerStartupSecret
			credentialRequirements = append(credentialRequirements, requirement)
		case variable.ValueFrom.ComponentVarRef != nil:
			selector := variable.ValueFrom.ComponentVarRef
			consumption, includeWorkload, err :=
				classifyShardingScaleInComponentVarSelector(selector)
			if err != nil {
				return nil, "", nil, err
			}
			source.ResolverKind = appsv1.ShardingScaleInVarResolverComponentVarRef
			source.Consumption = consumption
			source.SourceObjects, err =
				buildShardingScaleInMemberRequestSourceObjects(members, includeWorkload)
			if err != nil {
				return nil, "", nil, err
			}
		case variable.ValueFrom.ClusterVarRef != nil:
			if err := validateShardingScaleInClusterVarSelector(variable.ValueFrom.ClusterVarRef); err != nil {
				return nil, "", nil, err
			}
			source.ResolverKind = appsv1.ShardingScaleInVarResolverClusterVarRef
			source.SourceObjects = []appsv1.ShardingScaleInRequestSourceObject{
				buildShardingScaleInClusterRequestSourceObject(cluster),
			}
		case variable.ValueFrom.ResourceVarRef != nil:
			if err := validateShardingScaleInResourceVarSelector(variable.ValueFrom.ResourceVarRef); err != nil {
				return nil, "", nil, err
			}
			source.ResolverKind = appsv1.ShardingScaleInVarResolverResourceVarRef
			var err error
			source.SourceObjects, err =
				buildShardingScaleInMemberRequestSourceObjects(members, true)
			if err != nil {
				return nil, "", nil, err
			}
		default:
			return nil, "", nil, invalid("variable %q has an unsupported resolver", variable.Name)
		}
		sources = append(sources, source)
	}
	if servicePort == "" {
		return nil, "", nil, invalid("exactly one static SERVICE_PORT variable is required")
	}
	if err := validateShardingScaleInServicePort(servicePort); err != nil {
		return nil, "", nil, invalid("SERVICE_PORT is invalid: %v", err)
	}
	slices.SortFunc(sources, func(a, b appsv1.ShardingScaleInVarSource) int {
		return strings.Compare(a.VariableName, b.VariableName)
	})
	slices.SortFunc(credentialRequirements,
		func(a, b shardingScaleInCredentialRequirement) int {
			return strings.Compare(a.VariableName, b.VariableName)
		})
	return sources, servicePort, credentialRequirements, nil
}

func countShardingScaleInVarResolvers(source *appsv1.VarSource) int {
	if source == nil {
		return 0
	}
	count := 0
	values := []any{
		source.ConfigMapKeyRef, source.SecretKeyRef, source.HostNetworkVarRef,
		source.ServiceVarRef, source.CredentialVarRef, source.TLSVarRef,
		source.ServiceRefVarRef, source.ResourceVarRef, source.ComponentVarRef,
		source.ClusterVarRef,
	}
	for _, value := range values {
		if !reflect.ValueOf(value).IsNil() {
			count++
		}
	}
	return count
}

func buildShardingScaleInCredentialRequirement(
	variableName string,
	selector *appsv1.CredentialVarSelector,
) (shardingScaleInCredentialRequirement, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInRequestAuthoritySource,
			fmt.Sprintf(format, args...))
	}
	if selector == nil || selector.Name == "" ||
		selector.CompDef != "" || selector.MultipleClusterObjectOption != nil ||
		(selector.Optional != nil && *selector.Optional) {
		return shardingScaleInCredentialRequirement{},
			invalid("credential variable %q must bind a required current-Component account", variableName)
	}
	keyName := ""
	selected := 0
	if selector.Username != nil {
		if *selector.Username == appsv1.VarOptional {
			return shardingScaleInCredentialRequirement{},
				invalid("credential variable %q must be required", variableName)
		}
		keyName = constant.AccountNameForSecret
		selected++
	}
	if selector.Password != nil {
		if *selector.Password == appsv1.VarOptional {
			return shardingScaleInCredentialRequirement{},
				invalid("credential variable %q must be required", variableName)
		}
		keyName = constant.AccountPasswdForSecret
		selected++
	}
	if selected != 1 {
		return shardingScaleInCredentialRequirement{},
			invalid("credential variable %q must select exactly one credential key", variableName)
	}
	return shardingScaleInCredentialRequirement{
		VariableName: variableName,
		AccountName:  selector.Name,
		KeyName:      keyName,
		Selector:     *selector.DeepCopy(),
	}, nil
}

func classifyShardingScaleInComponentVarSelector(
	selector *appsv1.ComponentVarSelector,
) (appsv1.ShardingScaleInVarConsumption, bool, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInRequestAuthoritySource,
			fmt.Sprintf(format, args...))
	}
	if selector == nil || selector.CompDef != "" || selector.Name != "" ||
		selector.MultipleClusterObjectOption != nil ||
		(selector.Optional != nil && *selector.Optional) {
		return "", false, invalid("component variable must bind the required current Component")
	}
	selected := 0
	consumption := appsv1.ShardingScaleInVarConsumptionServerReservedIdentity
	includeWorkload := false
	identityOptions := []*appsv1.VarOption{
		selector.ComponentName, selector.ShortName, selector.ServiceVersion,
	}
	for _, option := range identityOptions {
		if option != nil {
			if *option == appsv1.VarOptional {
				return "", false, invalid("component variable must be required")
			}
			selected++
		}
	}
	rosterOptions := []*appsv1.VarOption{
		selector.Replicas, selector.PodNames, selector.PodFQDNs,
	}
	for _, option := range rosterOptions {
		if option != nil {
			if *option == appsv1.VarOptional {
				return "", false, invalid("component variable must be required")
			}
			selected++
			consumption = appsv1.ShardingScaleInVarConsumptionDurableEnvelope
			includeWorkload = true
		}
	}
	roledOptions := []*appsv1.RoledVar{
		selector.PodNamesForRole, selector.PodFQDNsForRole,
	}
	for _, option := range roledOptions {
		if option != nil {
			if option.Role == "" ||
				(option.Option != nil && *option.Option == appsv1.VarOptional) {
				return "", false, invalid("role-scoped component variable must be required and name a role")
			}
			selected++
			consumption = appsv1.ShardingScaleInVarConsumptionDurableEnvelope
			includeWorkload = true
		}
	}
	if selected != 1 {
		return "", false, invalid("component variable must select exactly one field")
	}
	return consumption, includeWorkload, nil
}

func validateShardingScaleInClusterVarSelector(
	selector *appsv1.ClusterVarSelector,
) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInRequestAuthoritySource,
			fmt.Sprintf(format, args...))
	}
	if selector == nil {
		return invalid("cluster variable selector must not be nil")
	}
	selected := 0
	for _, option := range []*appsv1.VarOption{
		selector.Namespace, selector.ClusterName, selector.ClusterUID,
	} {
		if option != nil {
			if *option == appsv1.VarOptional {
				return invalid("cluster variable must be required")
			}
			selected++
		}
	}
	if selected != 1 {
		return invalid("cluster variable must select exactly one field")
	}
	return nil
}

func validateShardingScaleInResourceVarSelector(
	selector *appsv1.ResourceVarSelector,
) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInRequestAuthoritySource,
			fmt.Sprintf(format, args...))
	}
	if selector == nil || selector.CompDef != "" || selector.Name != "" ||
		selector.MultipleClusterObjectOption != nil ||
		(selector.Optional != nil && *selector.Optional) {
		return invalid("resource variable must bind the required current Component")
	}
	selected := 0
	for _, option := range []*appsv1.VarOption{
		selector.CPU, selector.CPULimit, selector.Memory, selector.MemoryLimit,
	} {
		if option != nil {
			if *option == appsv1.VarOptional {
				return invalid("resource variable must be required")
			}
			selected++
		}
	}
	if selector.Storage != nil {
		if selector.Storage.Name == "" ||
			(selector.Storage.Option != nil && *selector.Storage.Option == appsv1.VarOptional) {
			return invalid("resource storage variable must be required and name a volume")
		}
		selected++
	}
	if selected != 1 {
		return invalid("resource variable must select exactly one field")
	}
	return nil
}

func buildShardingScaleInClusterRequestSourceObject(
	cluster *appsv1.Cluster,
) appsv1.ShardingScaleInRequestSourceObject {
	digest, _ := digestShardingScaleInCanonicalJSON(struct {
		Version         string    `json:"version"`
		Namespace       string    `json:"namespace"`
		Name            string    `json:"name"`
		UID             types.UID `json:"uid"`
		Generation      int64     `json:"generation"`
		ResourceVersion string    `json:"resourceVersion"`
	}{
		Version:         shardingScaleInRequestSourceProjectionVersionV1,
		Namespace:       cluster.Namespace,
		Name:            cluster.Name,
		UID:             cluster.UID,
		Generation:      cluster.Generation,
		ResourceVersion: cluster.ResourceVersion,
	})
	return appsv1.ShardingScaleInRequestSourceObject{
		APIVersion:       appsv1.GroupVersion.String(),
		Kind:             appsv1.ClusterKind,
		Namespace:        cluster.Namespace,
		Name:             cluster.Name,
		UID:              cluster.UID,
		ProjectionKind:   appsv1.ShardingScaleInSourceProjectionClusterIdentity,
		ProjectionDigest: digest,
	}
}

func buildShardingScaleInMemberRequestSourceObjects(
	members []shardingScaleInRequestAuthorityMember,
	includeWorkload bool,
) ([]appsv1.ShardingScaleInRequestSourceObject, error) {
	objects := make([]appsv1.ShardingScaleInRequestSourceObject, 0, len(members)*2)
	for _, member := range members {
		componentDigest, err := digestShardingScaleInCanonicalJSON(struct {
			Version         string    `json:"version"`
			Namespace       string    `json:"namespace"`
			Name            string    `json:"name"`
			UID             types.UID `json:"uid"`
			Generation      int64     `json:"generation"`
			ResourceVersion string    `json:"resourceVersion"`
			CompDef         string    `json:"compDef"`
			ServiceVersion  string    `json:"serviceVersion"`
			Replicas        int32     `json:"replicas"`
		}{
			Version:         shardingScaleInRequestSourceProjectionVersionV1,
			Namespace:       member.Component.Namespace,
			Name:            member.Component.Name,
			UID:             member.Component.UID,
			Generation:      member.Component.Generation,
			ResourceVersion: member.Component.ResourceVersion,
			CompDef:         member.Component.Spec.CompDef,
			ServiceVersion:  member.Component.Spec.ServiceVersion,
			Replicas:        member.Component.Spec.Replicas,
		})
		if err != nil {
			return nil, err
		}
		objects = append(objects, appsv1.ShardingScaleInRequestSourceObject{
			APIVersion:       appsv1.GroupVersion.String(),
			Kind:             appsv1.ComponentKind,
			Namespace:        member.Component.Namespace,
			Name:             member.Component.Name,
			UID:              member.Component.UID,
			ProjectionKind:   appsv1.ShardingScaleInSourceProjectionComponentIdentity,
			ProjectionDigest: componentDigest,
		})
		if !includeWorkload {
			continue
		}
		workload := member.Workload.DeepCopy()
		canonicalizeShardingScaleInInstanceSetSpec(&workload.Spec)
		workloadDigest, err := digestShardingScaleInCanonicalJSON(struct {
			Version         string                      `json:"version"`
			Namespace       string                      `json:"namespace"`
			Name            string                      `json:"name"`
			UID             types.UID                   `json:"uid"`
			Generation      int64                       `json:"generation"`
			ResourceVersion string                      `json:"resourceVersion"`
			Spec            workloadsv1.InstanceSetSpec `json:"spec"`
		}{
			Version:         shardingScaleInRequestSourceProjectionVersionV1,
			Namespace:       workload.Namespace,
			Name:            workload.Name,
			UID:             workload.UID,
			Generation:      workload.Generation,
			ResourceVersion: workload.ResourceVersion,
			Spec:            workload.Spec,
		})
		if err != nil {
			return nil, err
		}
		objects = append(objects, appsv1.ShardingScaleInRequestSourceObject{
			APIVersion:       workloadsv1.GroupVersion.String(),
			Kind:             workloadsv1.InstanceSetKind,
			Namespace:        workload.Namespace,
			Name:             workload.Name,
			UID:              workload.UID,
			ProjectionKind:   appsv1.ShardingScaleInSourceProjectionInstanceSetSpec,
			ProjectionDigest: workloadDigest,
		})
	}
	slices.SortFunc(objects, compareShardingScaleInRequestSourceObjects)
	return objects, nil
}

func loadShardingScaleInCredentialSources(
	ctx context.Context,
	apiReader client.Reader,
	cluster *appsv1.Cluster,
	members []shardingScaleInRequestAuthorityMember,
	componentDefinitionSources []appsv1.ShardingScaleInComponentDefinitionSource,
	requirements []shardingScaleInCredentialRequirement,
) ([]appsv1.ShardingScaleInCredentialSource,
	map[types.UID][]shardingScaleInCredentialResolution, error,
) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInRequestAuthoritySource,
			fmt.Sprintf(format, args...))
	}
	componentDefinitions := make(map[string]appsv1.ShardingScaleInComponentDefinitionSource,
		len(componentDefinitionSources))
	for _, source := range componentDefinitionSources {
		componentDefinitions[source.Name] = source
	}
	sourcesByID := map[string]appsv1.ShardingScaleInCredentialSource{}
	sourceIDsByComponentAccount := map[string]string{}
	resolutionsByComponent :=
		make(map[types.UID][]shardingScaleInCredentialResolution, len(members))
	for _, member := range members {
		requirementsByAccount := map[string][]shardingScaleInCredentialRequirement{}
		for _, requirement := range requirements {
			requirementsByAccount[requirement.AccountName] =
				append(requirementsByAccount[requirement.AccountName], requirement)
		}
		synthesizedComponent := &component.SynthesizedComponent{
			Namespace:   cluster.Namespace,
			ClusterName: cluster.Name,
			Name:        member.ShortName,
			CompDefName: member.Component.Spec.CompDef,
		}
		accountNames := make([]string, 0, len(requirementsByAccount))
		for accountName := range requirementsByAccount {
			accountNames = append(accountNames, accountName)
		}
		slices.Sort(accountNames)
		for _, accountName := range accountNames {
			keyNames := make([]string, 0, len(requirementsByAccount[accountName]))
			secretKey := types.NamespacedName{}
			for _, requirement := range requirementsByAccount[accountName] {
				provenance, err := component.ResolveCredentialVarRefProvenance(
					ctx, apiReader, synthesizedComponent,
					requirement.VariableName, requirement.Selector)
				if err != nil {
					return nil, nil, invalid(
						"credential resolver failed for variable %q: %v",
						requirement.VariableName, err)
				}
				if provenance.Namespace != cluster.Namespace ||
					provenance.SecretName == "" ||
					provenance.KeyName != requirement.KeyName {
					return nil, nil, invalid(
						"credential resolver provenance for variable %q does not match its declaration",
						requirement.VariableName)
				}
				resolvedSecretKey := types.NamespacedName{
					Namespace: provenance.Namespace,
					Name:      provenance.SecretName,
				}
				if secretKey.Name != "" && secretKey != resolvedSecretKey {
					return nil, nil, invalid(
						"credential account %q resolved to more than one Secret",
						accountName)
				}
				secretKey = resolvedSecretKey
				keyNames = append(keyNames, provenance.KeyName)
			}
			slices.Sort(keyNames)
			keyNames = slices.Compact(keyNames)
			if secretKey.Name == "" {
				return nil, nil, invalid(
					"credential account %q did not resolve to a Secret",
					accountName)
			}
			before, err := readShardingScaleInSecretSource(
				ctx, apiReader, secretKey, keyNames)
			if err != nil {
				return nil, nil, err
			}
			after, err := readShardingScaleInSecretSource(
				ctx, apiReader, secretKey, keyNames)
			if err != nil {
				return nil, nil, err
			}
			if !reflect.DeepEqual(before, after) {
				return nil, nil, invalid("credential Secret snapshot changed for %q", secretKey.Name)
			}
			sourceID, err := digestShardingScaleInCredentialSourceID(before)
			if err != nil {
				return nil, nil, err
			}
			before.SourceID = sourceID
			if existing, ok := sourcesByID[sourceID]; ok {
				if existing.APIVersion != before.APIVersion ||
					existing.Kind != before.Kind ||
					existing.Namespace != before.Namespace ||
					existing.Name != before.Name ||
					existing.UID != before.UID {
					return nil, nil, invalid("credential source ID collision")
				}
				existing.KeyNames = append(existing.KeyNames, before.KeyNames...)
				slices.Sort(existing.KeyNames)
				existing.KeyNames = slices.Compact(existing.KeyNames)
				sourcesByID[sourceID] = existing
			} else {
				sourcesByID[sourceID] = before
			}
			sourceIDsByComponentAccount[string(member.Component.UID)+"\x00"+accountName] = sourceID
		}
	}

	sources := make([]appsv1.ShardingScaleInCredentialSource, 0, len(sourcesByID))
	for sourceID, source := range sourcesByID {
		source.SourceID = sourceID
		sourceDigest, err := digestShardingScaleInCredentialSource(source)
		if err != nil {
			return nil, nil, err
		}
		source.CredentialSourceDigest = sourceDigest
		sourcesByID[sourceID] = source
		sources = append(sources, source)
	}
	slices.SortFunc(sources, func(a, b appsv1.ShardingScaleInCredentialSource) int {
		return strings.Compare(a.SourceID, b.SourceID)
	})

	for _, member := range members {
		componentDefinition, ok := componentDefinitions[member.Component.Spec.CompDef]
		if !ok {
			return nil, nil, invalid("ComponentDefinition source %q is missing",
				member.Component.Spec.CompDef)
		}
		for _, requirement := range requirements {
			sourceID := sourceIDsByComponentAccount[string(member.Component.UID)+"\x00"+requirement.AccountName]
			source, ok := sourcesByID[sourceID]
			if !ok {
				return nil, nil, invalid("credential source for Component %q is missing",
					member.Component.Name)
			}
			requiredKeyNames := []string{requirement.KeyName}
			resolverDigest, err := digestShardingScaleInCredentialResolverProjection(
				componentDefinition, member.Component.UID, requirement,
				sourceID, requiredKeyNames)
			if err != nil {
				return nil, nil, err
			}
			resolutionsByComponent[member.Component.UID] =
				append(resolutionsByComponent[member.Component.UID],
					shardingScaleInCredentialResolution{
						VariableName:             requirement.VariableName,
						CredentialSourceID:       sourceID,
						CredentialSourceDigest:   source.CredentialSourceDigest,
						RequiredKeyNames:         requiredKeyNames,
						ResolverProjectionDigest: resolverDigest,
					})
		}
		slices.SortFunc(resolutionsByComponent[member.Component.UID],
			func(a, b shardingScaleInCredentialResolution) int {
				return strings.Compare(a.VariableName, b.VariableName)
			})
	}
	return sources, resolutionsByComponent, nil
}

func readShardingScaleInSecretSource(
	ctx context.Context,
	apiReader client.Reader,
	key types.NamespacedName,
	requiredKeyNames []string,
) (appsv1.ShardingScaleInCredentialSource, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInRequestAuthoritySource,
			fmt.Sprintf(format, args...))
	}
	secret := &corev1.Secret{}
	if err := apiReader.Get(ctx, key, secret); err != nil {
		return appsv1.ShardingScaleInCredentialSource{}, err
	}
	if secret.Namespace != key.Namespace || secret.Name != key.Name ||
		secret.UID == "" || secret.ResourceVersion == "" ||
		!secret.DeletionTimestamp.IsZero() {
		return appsv1.ShardingScaleInCredentialSource{},
			invalid("credential Secret identity must be complete and not deleting")
	}
	for _, keyName := range requiredKeyNames {
		if _, ok := secret.Data[keyName]; !ok {
			return appsv1.ShardingScaleInCredentialSource{},
				invalid("credential key %q is missing from Secret %q", keyName, secret.Name)
		}
	}
	return appsv1.ShardingScaleInCredentialSource{
		APIVersion: corev1.SchemeGroupVersion.String(),
		Kind:       "Secret",
		Namespace:  secret.Namespace,
		Name:       secret.Name,
		UID:        secret.UID,
		KeyNames:   slices.Clone(requiredKeyNames),
	}, nil
}

func digestShardingScaleInCredentialResolverProjection(
	componentDefinition appsv1.ShardingScaleInComponentDefinitionSource,
	executorComponentUID types.UID,
	requirement shardingScaleInCredentialRequirement,
	credentialSourceID string,
	requiredKeyNames []string,
) (string, error) {
	return digestShardingScaleInCanonicalJSON(struct {
		Version                       string                       `json:"version"`
		ComponentDefinitionName       string                       `json:"componentDefinitionName"`
		ComponentDefinitionUID        types.UID                    `json:"componentDefinitionUID"`
		ComponentDefinitionGeneration int64                        `json:"componentDefinitionGeneration"`
		VarsProjectionDigest          string                       `json:"varsProjectionDigest"`
		ExecutorComponentUID          types.UID                    `json:"executorComponentUID"`
		VariableName                  string                       `json:"variableName"`
		CredentialVarRefSelector      appsv1.CredentialVarSelector `json:"credentialVarRefSelector"`
		CredentialSourceID            string                       `json:"credentialSourceID"`
		RequiredKeyNames              []string                     `json:"requiredKeyNames"`
	}{
		Version:                       "kb.sharding.scalein.credential-resolver-projection/v1",
		ComponentDefinitionName:       componentDefinition.Name,
		ComponentDefinitionUID:        componentDefinition.UID,
		ComponentDefinitionGeneration: componentDefinition.Generation,
		VarsProjectionDigest:          componentDefinition.VarsProjectionDigest,
		ExecutorComponentUID:          executorComponentUID,
		VariableName:                  requirement.VariableName,
		CredentialVarRefSelector:      requirement.Selector,
		CredentialSourceID:            credentialSourceID,
		RequiredKeyNames:              requiredKeyNames,
	})
}

func buildShardingScaleInExecutorCredentialBindings(
	executorPodUID, executorComponentUID types.UID,
	resolutions []shardingScaleInCredentialResolution,
) ([]appsv1.ShardingScaleInExecutorCredentialBinding, error) {
	bindings := make([]appsv1.ShardingScaleInExecutorCredentialBinding, 0, len(resolutions))
	for _, resolution := range resolutions {
		binding := appsv1.ShardingScaleInExecutorCredentialBinding{
			VariableName:             resolution.VariableName,
			CredentialSourceID:       resolution.CredentialSourceID,
			CredentialSourceDigest:   resolution.CredentialSourceDigest,
			RequiredKeyNames:         slices.Clone(resolution.RequiredKeyNames),
			ResolverProjectionDigest: resolution.ResolverProjectionDigest,
		}
		bindingDigest, err := digestShardingScaleInExecutorCredentialBinding(
			binding, executorPodUID, executorComponentUID)
		if err != nil {
			return nil, err
		}
		binding.BindingDigest = bindingDigest
		bindings = append(bindings, binding)
	}
	slices.SortFunc(bindings,
		func(a, b appsv1.ShardingScaleInExecutorCredentialBinding) int {
			return strings.Compare(a.VariableName, b.VariableName)
		})
	return bindings, nil
}

func buildShardingScaleInBaseParameterRecord(
	servicePort string,
) (string, string, error) {
	record := shardingScaleInBaseParameterRecord{
		Version: shardingScaleInBaseParameterRecordVersionV1,
		Parameters: []shardingScaleInBaseParameter{
			{
				Name: shardingScaleInBaseParameterProtocolVersion,
				ValueB64: base64.StdEncoding.EncodeToString(
					[]byte(appsv1.ShardingScaleInResultProtocolV2)),
			},
			{
				Name:     shardingScaleInBaseParameterServicePort,
				ValueB64: base64.StdEncoding.EncodeToString([]byte(servicePort)),
			},
		},
	}
	data, err := json.Marshal(record)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(data)
	encoded, digest, _, err := canonicalizeShardingScaleInBaseParameterRecord(
		base64.StdEncoding.EncodeToString(data), hex.EncodeToString(sum[:]))
	return encoded, digest, err
}

func digestShardingScaleInCanonicalBytes(value []byte) (string, error) {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:]), nil
}
