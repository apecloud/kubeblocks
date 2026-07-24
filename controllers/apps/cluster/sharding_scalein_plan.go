/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package cluster

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

var errInvalidShardingScaleInPlanMaterial = errors.New("invalid sharding scale-in plan material")

const (
	shardingScaleInBaseParameterRecordVersionV1 = "kb.sharding.scalein.base-parameters/v1"

	shardingScaleInBaseParameterProtocolVersion = "KB_SHARD_SCALE_IN_PROTOCOL_VERSION"
	shardingScaleInBaseParameterServicePort     = "SERVICE_PORT"
)

type shardingScaleInBaseParameterRecord struct {
	Version    string                         `json:"version"`
	Parameters []shardingScaleInBaseParameter `json:"parameters"`
}

type shardingScaleInBaseParameter struct {
	Name     string `json:"name"`
	ValueB64 string `json:"valueB64"`
}

type shardingScaleInBaseParameterValues struct {
	protocolVersion string
	servicePort     string
}

type shardingScaleInRequestSourceSnapshot struct {
	Version                    string                                            `json:"version"`
	ComponentDefinitionSources []appsv1.ShardingScaleInComponentDefinitionSource `json:"componentDefinitionSources"`
	VarSources                 []appsv1.ShardingScaleInVarSource                 `json:"varSources"`
	CredentialSources          []appsv1.ShardingScaleInCredentialSource          `json:"credentialSources"`
	ExecutorCredentialBindings []shardingScaleInExecutorCredentialSnapshot       `json:"executorCredentialBindings"`
}

type shardingScaleInExecutorCredentialSnapshot struct {
	ExecutorPodUID     types.UID                                         `json:"executorPodUID"`
	CredentialBindings []appsv1.ShardingScaleInExecutorCredentialBinding `json:"credentialBindings"`
}

func buildShardingScaleInPlanMaterial(input *appsv1.ShardingScaleInPlanMaterial,
) (*appsv1.ShardingScaleInPlanMaterial, string, error) {
	if input == nil {
		return nil, "", fmt.Errorf("%w: material must not be nil", errInvalidShardingScaleInPlanMaterial)
	}
	material := input.DeepCopy()
	if err := canonicalizeShardingScaleInPlanMaterial(material); err != nil {
		return nil, "", err
	}
	if err := validateShardingScaleInPlanMaterial(material); err != nil {
		return nil, "", err
	}

	sourceSnapshot := shardingScaleInRequestSourceSnapshot{
		Version:                    "kb.sharding.scalein.request-source-snapshot/v1",
		ComponentDefinitionSources: material.RequestAuthority.ComponentDefinitionSources,
		VarSources:                 material.RequestAuthority.VarSources,
		CredentialSources:          material.RequestAuthority.CredentialSources,
		ExecutorCredentialBindings: make([]shardingScaleInExecutorCredentialSnapshot, 0,
			len(material.RequestAuthority.ExecutorTemplates)),
	}
	for _, template := range material.RequestAuthority.ExecutorTemplates {
		sourceSnapshot.ExecutorCredentialBindings = append(sourceSnapshot.ExecutorCredentialBindings,
			shardingScaleInExecutorCredentialSnapshot{
				ExecutorPodUID:     template.ExecutorPodUID,
				CredentialBindings: template.CredentialBindings,
			})
	}
	sourceDigest, err := digestShardingScaleInCanonicalJSON(sourceSnapshot)
	if err != nil {
		return nil, "", err
	}
	material.RequestAuthority.SourceSnapshotDigest = sourceDigest

	requestAuthority := material.RequestAuthority.DeepCopy()
	requestAuthority.RequestAuthorityDigest = ""
	requestDigest, err := digestShardingScaleInCanonicalJSON(requestAuthority)
	if err != nil {
		return nil, "", err
	}
	material.RequestAuthority.RequestAuthorityDigest = requestDigest

	planID, err := digestShardingScaleInCanonicalJSON(material)
	if err != nil {
		return nil, "", err
	}
	return material, planID, nil
}

func canonicalizeShardingScaleInPlanMaterial(material *appsv1.ShardingScaleInPlanMaterial) error {
	sortMembers := func(members []appsv1.ShardingScaleInPlanMember) {
		for i := range members {
			slices.SortFunc(members[i].Pods, compareShardingScaleInPlanPods)
		}
		slices.SortFunc(members, compareShardingScaleInPlanMembers)
	}
	sortMembers(material.Leaving)
	sortMembers(material.Staying)

	slices.SortFunc(material.ExecutorPrerequisites, compareShardingScaleInPrerequisites)
	slices.SortFunc(material.RequestAuthority.ComponentDefinitionSources,
		func(a, b appsv1.ShardingScaleInComponentDefinitionSource) int {
			return strings.Compare(a.Name+"\x00"+string(a.UID), b.Name+"\x00"+string(b.UID))
		})
	if material.RequestAuthority.VarSources == nil {
		material.RequestAuthority.VarSources = []appsv1.ShardingScaleInVarSource{}
	}
	for i := range material.RequestAuthority.VarSources {
		source := &material.RequestAuthority.VarSources[i]
		if source.SourceObjects == nil {
			source.SourceObjects = []appsv1.ShardingScaleInRequestSourceObject{}
		}
		slices.SortFunc(source.SourceObjects, compareShardingScaleInRequestSourceObjects)
	}
	slices.SortFunc(material.RequestAuthority.VarSources,
		func(a, b appsv1.ShardingScaleInVarSource) int {
			return strings.Compare(a.VariableName, b.VariableName)
		})
	if len(material.RequestAuthority.CredentialSources) == 0 {
		material.RequestAuthority.CredentialSources = nil
	}
	credentialSourceIDRewrites := map[string]string{}
	credentialSourceDigests := map[string]string{}
	for i := range material.RequestAuthority.CredentialSources {
		source := &material.RequestAuthority.CredentialSources[i]
		oldSourceID := source.SourceID
		slices.Sort(source.KeyNames)
		sourceID, err := digestShardingScaleInCredentialSourceID(*source)
		if err != nil {
			return err
		}
		if previous, ok := credentialSourceIDRewrites[oldSourceID]; ok && previous != sourceID {
			return fmt.Errorf("%w: credential source ID %q is ambiguous",
				errInvalidShardingScaleInPlanMaterial, oldSourceID)
		}
		credentialSourceIDRewrites[oldSourceID] = sourceID
		source.SourceID = sourceID
		sourceDigest, err := digestShardingScaleInCredentialSource(*source)
		if err != nil {
			return err
		}
		source.CredentialSourceDigest = sourceDigest
		credentialSourceDigests[sourceID] = sourceDigest
	}
	slices.SortFunc(material.RequestAuthority.CredentialSources,
		func(a, b appsv1.ShardingScaleInCredentialSource) int {
			return strings.Compare(a.SourceID, b.SourceID)
		})
	slices.SortFunc(material.RequestAuthority.ExecutorTemplates,
		func(a, b appsv1.ShardingScaleInExecutorTemplate) int {
			return strings.Compare(string(a.ExecutorPodUID)+"\x00"+string(a.ExecutorComponentUID),
				string(b.ExecutorPodUID)+"\x00"+string(b.ExecutorComponentUID))
		})
	for i := range material.RequestAuthority.ExecutorTemplates {
		template := &material.RequestAuthority.ExecutorTemplates[i]
		if len(template.CredentialBindings) == 0 {
			template.CredentialBindings = nil
		}
		for j := range template.CredentialBindings {
			binding := &template.CredentialBindings[j]
			slices.Sort(binding.RequiredKeyNames)
			if sourceID, ok := credentialSourceIDRewrites[binding.CredentialSourceID]; ok {
				binding.CredentialSourceID = sourceID
			}
			if sourceDigest, ok := credentialSourceDigests[binding.CredentialSourceID]; ok {
				binding.CredentialSourceDigest = sourceDigest
			}
			bindingDigest, err := digestShardingScaleInExecutorCredentialBinding(
				*binding, template.ExecutorPodUID, template.ExecutorComponentUID)
			if err != nil {
				return err
			}
			binding.BindingDigest = bindingDigest
		}
		slices.SortFunc(template.CredentialBindings,
			func(a, b appsv1.ShardingScaleInExecutorCredentialBinding) int {
				return strings.Compare(
					a.VariableName+"\x00"+a.CredentialSourceID,
					b.VariableName+"\x00"+b.CredentialSourceID)
			})
		encoded, digest, _, err := canonicalizeShardingScaleInBaseParameterRecord(
			template.BaseParameterRecordB64, template.BaseParameterDigest)
		if err != nil {
			return fmt.Errorf("%w: executor %q base parameter record is invalid: %v",
				errInvalidShardingScaleInPlanMaterial, template.ExecutorPodUID, err)
		}
		template.BaseParameterRecordB64 = encoded
		template.BaseParameterDigest = digest
	}

	for i := range material.ExecutorPrerequisites {
		prerequisite := &material.ExecutorPrerequisites[i]
		digest, err := digestShardingScaleInPrerequisiteIdentity(*prerequisite)
		if err != nil {
			return err
		}
		prerequisite.IdentityDigest = digest
	}
	return nil
}

func validateShardingScaleInPlanMaterial(material *appsv1.ShardingScaleInPlanMaterial) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInPlanMaterial, fmt.Sprintf(format, args...))
	}

	if material.ProtocolVersion != appsv1.ShardingScaleInResultProtocolV2 {
		return invalid("protocolVersion must be %q", appsv1.ShardingScaleInResultProtocolV2)
	}
	if material.ShardingName == "" {
		return invalid("shardingName must not be empty")
	}
	source := material.Source
	if source.ClusterNamespace == "" || source.ClusterName == "" || source.ClusterUID == "" ||
		source.ClusterGeneration <= 0 || source.DesiredShards <= 0 || !isShardingScaleInSHA256(source.DesiredDigest) {
		return invalid("Cluster source identity, generation, desired shards, and desired digest must be valid")
	}
	if (source.OptionalOpsRequestName == "") != (source.OptionalOpsRequestUID == "") {
		return invalid("optional OpsRequest name and UID must either both be empty or both be set")
	}
	action := material.Action
	if action.ShardingDefinitionName == "" || action.ShardingDefinitionUID == "" ||
		action.ShardingDefinitionGeneration <= 0 || !isShardingScaleInSHA256(action.ActionDigest) ||
		action.ResultProtocol != appsv1.ShardingScaleInResultProtocolV2 {
		return invalid("action identity, generation, digest, and result protocol must be valid")
	}
	guard := material.DeletionGuard
	if guard.Protocol != appsv1.ShardingScaleInDeletionGuardProtocolV1 ||
		guard.InstallationUID == "" || guard.PolicyRevision == "" ||
		!isShardingScaleInSHA256(guard.ConfigurationDigest) {
		return invalid("deletion guard identity must be complete")
	}
	servicePort, err := validateShardingScaleInRequestAuthority(material)
	if err != nil {
		return err
	}
	if len(material.Leaving) == 0 || len(material.Staying) == 0 ||
		len(material.Leaving)+len(material.Staying) > 32 {
		return invalid("leaving and staying must be non-empty and contain at most 32 Components")
	}
	if int32(len(material.Staying)) != source.DesiredShards {
		return invalid("desiredShards must equal the number of staying Components")
	}

	componentNames := map[string]struct{}{}
	componentUIDs := map[types.UID]struct{}{}
	componentShortNames := map[string]struct{}{}
	podNames := map[string]struct{}{}
	podUIDs := map[types.UID]types.UID{}
	podFQDNs := map[string]struct{}{}
	componentPods := map[types.UID][]appsv1.ShardingScaleInPlanPod{}
	validateMembers := func(members []appsv1.ShardingScaleInPlanMember) error {
		for _, member := range members {
			if member.ComponentName == "" || member.ComponentUID == "" || member.ComponentGeneration <= 0 ||
				!isShardingScaleInSHA256(member.ComponentSpecDigest) ||
				member.ComponentShortName == "" || member.ShardTemplateName == "" ||
				len(member.Pods) == 0 || len(member.Pods) > 5 {
				return invalid("Component and Pod identities must be complete")
			}
			if _, ok := componentNames[member.ComponentName]; ok {
				return invalid("Component name %q is duplicated", member.ComponentName)
			}
			if _, ok := componentUIDs[member.ComponentUID]; ok {
				return invalid("Component UID %q is duplicated", member.ComponentUID)
			}
			if _, ok := componentShortNames[member.ComponentShortName]; ok {
				return invalid("Component short name %q is duplicated", member.ComponentShortName)
			}
			componentNames[member.ComponentName] = struct{}{}
			componentUIDs[member.ComponentUID] = struct{}{}
			componentShortNames[member.ComponentShortName] = struct{}{}
			componentPods[member.ComponentUID] = member.Pods
			for _, pod := range member.Pods {
				if pod.Name == "" || pod.UID == "" || pod.FQDN == "" ||
					!isShardingScaleInImageID(pod.AgentImageID) ||
					pod.AgentProcessUID == "" || !isShardingScaleInSHA256(pod.AgentCapabilityDigest) {
					return invalid("Pod identity and agent binding must be complete")
				}
				if _, ok := podNames[pod.Name]; ok {
					return invalid("Pod name %q is duplicated", pod.Name)
				}
				if _, ok := podUIDs[pod.UID]; ok {
					return invalid("Pod UID %q is duplicated", pod.UID)
				}
				if _, ok := podFQDNs[pod.FQDN]; ok {
					return invalid("Pod FQDN %q is duplicated", pod.FQDN)
				}
				podNames[pod.Name] = struct{}{}
				podUIDs[pod.UID] = member.ComponentUID
				podFQDNs[pod.FQDN] = struct{}{}
			}
		}
		return nil
	}
	if err := validateMembers(material.Leaving); err != nil {
		return err
	}
	if err := validateMembers(material.Staying); err != nil {
		return err
	}

	stayingPods := make([]appsv1.ShardingScaleInPlanPod, 0)
	for _, member := range material.Staying {
		stayingPods = append(stayingPods, member.Pods...)
	}
	slices.SortFunc(stayingPods, compareShardingScaleInPlanPods)
	if len(stayingPods) == 0 ||
		material.ProofExecutor.PodName != stayingPods[0].Name ||
		material.ProofExecutor.PodUID != stayingPods[0].UID {
		return invalid("proof executor must be the first canonical staying Pod")
	}
	if len(componentNames)+len(podNames)+len(material.ExecutorPrerequisites) > 512 {
		return invalid("protected object count exceeds 512")
	}
	if err := validateShardingScaleInExecutorPrerequisites(material, componentUIDs); err != nil {
		return err
	}
	if err := validateShardingScaleInExecutorTemplates(
		material, componentPods, podUIDs, servicePort); err != nil {
		return err
	}
	return nil
}

func validateShardingScaleInRequestAuthority(material *appsv1.ShardingScaleInPlanMaterial) (string, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInPlanMaterial, fmt.Sprintf(format, args...))
	}
	authority := material.RequestAuthority
	if authority.Version != appsv1.ShardingScaleInRequestAuthorityVersionV2 ||
		authority.Builder != appsv1.ShardingScaleInRequestBuilderTypedV2 ||
		!authority.GenericLifecycleSynthesisForbidden ||
		authority.ActionName != shardingRemoveShardAction ||
		authority.ActionDefinitionDigest != material.Action.ActionDigest {
		return "", invalid("request authority contract and action binding must be exact")
	}
	if len(authority.ComponentDefinitionSources) == 0 || len(authority.ExecutorTemplates) == 0 {
		return "", invalid("request authority sources and executor templates must not be empty")
	}
	componentDefinitionNames := map[string]struct{}{}
	componentDefinitionUIDs := map[types.UID]struct{}{}
	for i, source := range authority.ComponentDefinitionSources {
		if source.Name == "" || source.UID == "" || source.Generation <= 0 ||
			!isShardingScaleInSHA256(source.VarsProjectionDigest) {
			return "", invalid("ComponentDefinition source identity must be complete")
		}
		if _, ok := componentDefinitionNames[source.Name]; ok {
			return "", invalid("ComponentDefinition source name %q is duplicated", source.Name)
		}
		if _, ok := componentDefinitionUIDs[source.UID]; ok {
			return "", invalid("ComponentDefinition source UID %q is duplicated", source.UID)
		}
		componentDefinitionNames[source.Name] = struct{}{}
		componentDefinitionUIDs[source.UID] = struct{}{}
		if i > 0 && strings.Compare(
			authority.ComponentDefinitionSources[i-1].Name+"\x00"+
				string(authority.ComponentDefinitionSources[i-1].UID),
			source.Name+"\x00"+string(source.UID)) >= 0 {
			return "", invalid("ComponentDefinition sources must be sorted and unique")
		}
	}
	if len(authority.VarSources) == 0 {
		return "", invalid("request authority variable sources must not be empty")
	}
	var servicePort string
	credentialVariables := map[string]struct{}{}
	for i, source := range authority.VarSources {
		if source.SecretSource != nil {
			return "", invalid("variable %q legacy secretSource is forbidden in request authority v2",
				source.VariableName)
		}
		if source.VariableName == "" {
			return "", invalid("variable name must not be empty")
		}
		if i > 0 && authority.VarSources[i-1].VariableName == source.VariableName {
			return "", invalid("variable name %q is duplicated", source.VariableName)
		}
		switch source.ResolverKind {
		case appsv1.ShardingScaleInVarResolverStaticValue,
			appsv1.ShardingScaleInVarResolverClusterVarRef,
			appsv1.ShardingScaleInVarResolverComponentVarRef,
			appsv1.ShardingScaleInVarResolverResourceVarRef,
			appsv1.ShardingScaleInVarResolverCredentialVarRef:
		default:
			return "", invalid("variable %q has an unsupported resolver kind", source.VariableName)
		}
		switch source.Consumption {
		case appsv1.ShardingScaleInVarConsumptionTypedBase:
			if err := validateShardingScaleInEncodedDigest(
				source.ResolvedNonSecretValueB64, source.ResolvedNonSecretValueDigest); err != nil {
				return "", invalid("variable %q typed base is invalid: %v", source.VariableName, err)
			}
			if source.VariableName != shardingScaleInBaseParameterServicePort {
				return "", invalid("variable %q is not allowed in TypedBase", source.VariableName)
			}
			decoded, err := base64.StdEncoding.Strict().DecodeString(source.ResolvedNonSecretValueB64)
			if err != nil {
				return "", invalid("variable %q typed base is invalid: %v", source.VariableName, err)
			}
			servicePort = string(decoded)
		case appsv1.ShardingScaleInVarConsumptionDurableEnvelope,
			appsv1.ShardingScaleInVarConsumptionServerStartupSecret,
			appsv1.ShardingScaleInVarConsumptionServerReservedIdentity,
			appsv1.ShardingScaleInVarConsumptionForbidden:
			if source.ResolvedNonSecretValueB64 != "" || source.ResolvedNonSecretValueDigest != "" {
				return "", invalid("variable %q must not persist a resolved value", source.VariableName)
			}
		default:
			return "", invalid("variable %q has an unsupported consumption", source.VariableName)
		}
		switch source.ResolverKind {
		case appsv1.ShardingScaleInVarResolverCredentialVarRef:
			if source.Consumption != appsv1.ShardingScaleInVarConsumptionServerStartupSecret ||
				len(source.SourceObjects) != 0 {
				return "", invalid("credential variable %q must be a server startup secret", source.VariableName)
			}
			credentialVariables[source.VariableName] = struct{}{}
		case appsv1.ShardingScaleInVarResolverStaticValue:
			if len(source.SourceObjects) != 0 {
				return "", invalid("static variable %q must not have live source objects", source.VariableName)
			}
		}
		if source.Consumption == appsv1.ShardingScaleInVarConsumptionServerStartupSecret &&
			source.ResolverKind != appsv1.ShardingScaleInVarResolverCredentialVarRef {
			return "", invalid("server startup secret variable %q must use credentialVarRef", source.VariableName)
		}
		if source.ResolverKind != appsv1.ShardingScaleInVarResolverStaticValue &&
			source.ResolverKind != appsv1.ShardingScaleInVarResolverCredentialVarRef &&
			len(source.SourceObjects) == 0 {
			return "", invalid("variable %q must record its source objects", source.VariableName)
		}
		if err := validateShardingScaleInRequestSourceObjects(source.SourceObjects,
			material.Source.ClusterNamespace); err != nil {
			return "", err
		}
	}
	if servicePort == "" {
		return "", invalid("%q must be the only TypedBase variable", shardingScaleInBaseParameterServicePort)
	}
	if err := validateShardingScaleInServicePort(servicePort); err != nil {
		return "", invalid("%q is invalid: %v", shardingScaleInBaseParameterServicePort, err)
	}
	if err := validateShardingScaleInCredentialSources(
		authority.CredentialSources, material.Source.ClusterNamespace); err != nil {
		return "", err
	}
	if len(credentialVariables) == 0 && len(authority.CredentialSources) != 0 {
		return "", invalid("credential sources must be empty when no credential variables are declared")
	}
	return servicePort, nil
}

func validateShardingScaleInCredentialSources(
	sources []appsv1.ShardingScaleInCredentialSource,
	clusterNamespace string,
) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInPlanMaterial, fmt.Sprintf(format, args...))
	}
	sourceIDs := map[string]struct{}{}
	secretKeys := map[string]struct{}{}
	secretUIDs := map[types.UID]struct{}{}
	for i, source := range sources {
		if source.SourceID == "" || !isShardingScaleInSHA256(source.SourceID) {
			return invalid("credential source ID must be a SHA256 digest")
		}
		if i > 0 && sources[i-1].SourceID >= source.SourceID {
			return invalid("credential sources must be sorted and unique")
		}
		if _, ok := sourceIDs[source.SourceID]; ok {
			return invalid("credential source %q is duplicated", source.SourceID)
		}
		if source.APIVersion != "v1" || source.Kind != "Secret" ||
			source.Namespace != clusterNamespace || source.Name == "" || source.UID == "" ||
			len(source.KeyNames) == 0 {
			return invalid("credential source %q Secret identity must be exact", source.SourceID)
		}
		for j, key := range source.KeyNames {
			if key == "" || (j > 0 && source.KeyNames[j-1] >= key) {
				return invalid("credential source %q Secret keys must be sorted, unique, and non-empty",
					source.SourceID)
			}
		}
		secretKey := strings.Join([]string{
			source.APIVersion, source.Kind, source.Namespace, source.Name,
		}, "\x00")
		if _, ok := secretKeys[secretKey]; ok {
			return invalid("credential Secret %q is duplicated", source.Name)
		}
		if _, ok := secretUIDs[source.UID]; ok {
			return invalid("credential Secret UID %q is duplicated", source.UID)
		}
		expectedID, err := digestShardingScaleInCredentialSourceID(source)
		if err != nil {
			return err
		}
		if source.SourceID != expectedID {
			return invalid("credential source %q digest does not match its identity", source.SourceID)
		}
		expectedDigest, err := digestShardingScaleInCredentialSource(source)
		if err != nil {
			return err
		}
		if source.CredentialSourceDigest != expectedDigest {
			return invalid("credential source %q content digest does not match", source.SourceID)
		}
		sourceIDs[source.SourceID] = struct{}{}
		secretKeys[secretKey] = struct{}{}
		secretUIDs[source.UID] = struct{}{}
	}
	return nil
}

func validateShardingScaleInRequestSourceObjects(
	objects []appsv1.ShardingScaleInRequestSourceObject, clusterNamespace string,
) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInPlanMaterial, fmt.Sprintf(format, args...))
	}
	objectKeys := map[string]struct{}{}
	objectUIDs := map[types.UID]struct{}{}
	for _, object := range objects {
		if object.APIVersion == "" || object.Kind == "" || object.Name == "" || object.UID == "" ||
			(object.Namespace != "" && object.Namespace != clusterNamespace) ||
			!isShardingScaleInSHA256(object.ProjectionDigest) {
			return invalid("request source object identity and projection must be complete")
		}
		expectedAPIVersion, expectedKind := "", ""
		switch object.ProjectionKind {
		case appsv1.ShardingScaleInSourceProjectionClusterIdentity:
			expectedAPIVersion, expectedKind = appsv1.GroupVersion.String(), appsv1.ClusterKind
		case appsv1.ShardingScaleInSourceProjectionComponentIdentity:
			expectedAPIVersion, expectedKind = appsv1.GroupVersion.String(), appsv1.ComponentKind
		case appsv1.ShardingScaleInSourceProjectionInstanceSetSpec:
			expectedAPIVersion, expectedKind = workloadsv1.GroupVersion.String(), workloadsv1.InstanceSetKind
		default:
			return invalid("request source object %q has an unsupported projection", object.Name)
		}
		if object.APIVersion != expectedAPIVersion || object.Kind != expectedKind ||
			object.Namespace != clusterNamespace {
			return invalid("request source object %q GVK and namespace must match its projection", object.Name)
		}
		key := strings.Join([]string{object.APIVersion, object.Kind, object.Namespace, object.Name}, "\x00")
		if _, ok := objectKeys[key]; ok {
			return invalid("request source object %q is duplicated", object.Name)
		}
		if _, ok := objectUIDs[object.UID]; ok {
			return invalid("request source object UID %q is duplicated", object.UID)
		}
		objectKeys[key] = struct{}{}
		objectUIDs[object.UID] = struct{}{}
	}
	return nil
}

func validateShardingScaleInExecutorPrerequisites(material *appsv1.ShardingScaleInPlanMaterial,
	componentUIDs map[types.UID]struct{},
) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInPlanMaterial, fmt.Sprintf(format, args...))
	}
	if len(material.ExecutorPrerequisites) == 0 {
		return invalid("executor prerequisites must not be empty")
	}
	prerequisiteKeys := map[string]struct{}{}
	prerequisiteUIDs := map[types.UID]struct{}{}
	for _, prerequisite := range material.ExecutorPrerequisites {
		if prerequisite.APIVersion == "" || prerequisite.Kind == "" ||
			prerequisite.Namespace != material.Source.ClusterNamespace ||
			prerequisite.Name == "" || prerequisite.UID == "" ||
			!isShardingScaleInSHA256(prerequisite.CriticalSpecDigest) ||
			!isShardingScaleInSHA256(prerequisite.IdentityDigest) {
			return invalid("executor prerequisite identity and digests must be complete")
		}
		switch prerequisite.Role {
		case appsv1.ShardingScaleInPrerequisiteRolePodOwnerWorkload,
			appsv1.ShardingScaleInPrerequisiteRoleClusterDNS,
			appsv1.ShardingScaleInPrerequisiteRoleKBAgentEndpoint:
		default:
			return invalid("executor prerequisite %q has an unsupported role", prerequisite.Name)
		}
		switch prerequisite.Scope {
		case appsv1.ShardingScaleInPrerequisiteScopeComponent:
			if _, ok := componentUIDs[prerequisite.ComponentUID]; !ok {
				return invalid("Component prerequisite %q must bind a plan Component UID", prerequisite.Name)
			}
		case appsv1.ShardingScaleInPrerequisiteScopeShared:
			if prerequisite.ComponentUID != "" {
				return invalid("shared prerequisite %q must not bind a Component UID", prerequisite.Name)
			}
		default:
			return invalid("executor prerequisite %q has an unsupported scope", prerequisite.Name)
		}
		key := strings.Join([]string{
			prerequisite.APIVersion, prerequisite.Kind, prerequisite.Namespace, prerequisite.Name,
		}, "\x00")
		if _, ok := prerequisiteKeys[key]; ok {
			return invalid("executor prerequisite %q is duplicated", prerequisite.Name)
		}
		if _, ok := prerequisiteUIDs[prerequisite.UID]; ok {
			return invalid("executor prerequisite UID %q is duplicated", prerequisite.UID)
		}
		prerequisiteKeys[key] = struct{}{}
		prerequisiteUIDs[prerequisite.UID] = struct{}{}
	}
	return nil
}

func validateShardingScaleInExecutorTemplates(material *appsv1.ShardingScaleInPlanMaterial,
	componentPods map[types.UID][]appsv1.ShardingScaleInPlanPod, podUIDs map[types.UID]types.UID,
	servicePort string,
) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInPlanMaterial, fmt.Sprintf(format, args...))
	}
	templates := material.RequestAuthority.ExecutorTemplates
	if len(templates) != len(podUIDs) {
		return invalid("executor templates must cover every plan Pod exactly once")
	}
	credentialVariables := map[string]struct{}{}
	for _, source := range material.RequestAuthority.VarSources {
		if source.ResolverKind == appsv1.ShardingScaleInVarResolverCredentialVarRef {
			credentialVariables[source.VariableName] = struct{}{}
		}
	}
	credentialSources := map[string]appsv1.ShardingScaleInCredentialSource{}
	for _, source := range material.RequestAuthority.CredentialSources {
		credentialSources[source.SourceID] = source
	}
	usedCredentialSources := map[string]struct{}{}
	requiredKeysBySource := map[string]map[string]struct{}{}
	seen := map[types.UID]struct{}{}
	for _, template := range templates {
		componentUID, ok := podUIDs[template.ExecutorPodUID]
		if !ok || componentUID != template.ExecutorComponentUID {
			return invalid("executor template must bind an exact plan Pod and Component")
		}
		if _, ok := seen[template.ExecutorPodUID]; ok {
			return invalid("executor Pod UID %q is duplicated", template.ExecutorPodUID)
		}
		seen[template.ExecutorPodUID] = struct{}{}
		if len(template.CredentialBindings) != len(credentialVariables) {
			return invalid("executor %q credential bindings must cover every credential variable exactly once",
				template.ExecutorPodUID)
		}
		boundVariables := map[string]struct{}{}
		for i, binding := range template.CredentialBindings {
			if binding.VariableName == "" ||
				!isShardingScaleInSHA256(binding.CredentialSourceID) ||
				!isShardingScaleInSHA256(binding.CredentialSourceDigest) ||
				!isShardingScaleInSHA256(binding.ResolverProjectionDigest) ||
				!isShardingScaleInSHA256(binding.BindingDigest) ||
				len(binding.RequiredKeyNames) == 0 {
				return invalid("executor %q credential binding must be complete", template.ExecutorPodUID)
			}
			if i > 0 && template.CredentialBindings[i-1].VariableName >= binding.VariableName {
				return invalid("executor %q credential variables must be sorted and unique",
					template.ExecutorPodUID)
			}
			if _, ok := credentialVariables[binding.VariableName]; !ok {
				return invalid("executor %q credential variable %q is not declared",
					template.ExecutorPodUID, binding.VariableName)
			}
			if _, ok := boundVariables[binding.VariableName]; ok {
				return invalid("executor %q credential variable %q is duplicated",
					template.ExecutorPodUID, binding.VariableName)
			}
			source, ok := credentialSources[binding.CredentialSourceID]
			if !ok {
				return invalid("executor %q credential source %q is not declared",
					template.ExecutorPodUID, binding.CredentialSourceID)
			}
			if binding.CredentialSourceDigest != source.CredentialSourceDigest {
				return invalid("executor %q credential source digest does not match",
					template.ExecutorPodUID)
			}
			for j, keyName := range binding.RequiredKeyNames {
				if keyName == "" || (j > 0 && binding.RequiredKeyNames[j-1] >= keyName) {
					return invalid("executor %q credential keys must be sorted, unique, and non-empty",
						template.ExecutorPodUID)
				}
				if !slices.Contains(source.KeyNames, keyName) {
					return invalid("executor %q credential key %q is not declared by its source",
						template.ExecutorPodUID, keyName)
				}
				if requiredKeysBySource[source.SourceID] == nil {
					requiredKeysBySource[source.SourceID] = map[string]struct{}{}
				}
				requiredKeysBySource[source.SourceID][keyName] = struct{}{}
			}
			expectedBindingDigest, err := digestShardingScaleInExecutorCredentialBinding(
				binding, template.ExecutorPodUID, template.ExecutorComponentUID)
			if err != nil {
				return err
			}
			if binding.BindingDigest != expectedBindingDigest {
				return invalid("executor %q credential binding digest does not match",
					template.ExecutorPodUID)
			}
			boundVariables[binding.VariableName] = struct{}{}
			usedCredentialSources[binding.CredentialSourceID] = struct{}{}
		}
		_, _, values, err := canonicalizeShardingScaleInBaseParameterRecord(
			template.BaseParameterRecordB64, template.BaseParameterDigest)
		if err != nil {
			return invalid("executor %q base parameter record is invalid: %v", template.ExecutorPodUID, err)
		}
		if values.protocolVersion != string(appsv1.ShardingScaleInResultProtocolV2) {
			return invalid("executor %q base parameter protocol must be %q",
				template.ExecutorPodUID, appsv1.ShardingScaleInResultProtocolV2)
		}
		if values.servicePort != servicePort {
			return invalid("executor %q base parameter SERVICE_PORT must match TypedBase",
				template.ExecutorPodUID)
		}
		if !isShardingScaleInSHA256(template.LaunchSchemaDigest) ||
			!isShardingScaleInSHA256(template.PollSchemaDigest) ||
			!isShardingScaleInSHA256(template.CancelSchemaDigest) {
			return invalid("executor %q schema digests must be valid", template.ExecutorPodUID)
		}
		binding := template.ServerRuntimeBinding
		if len(binding.AgentProcessUID) < 22 || !isShardingScaleInImageID(binding.AgentImageID) ||
			binding.RegisteredActionDigest != material.Action.ActionDigest ||
			!isShardingScaleInSHA256(binding.StartupEnvironmentSchemaDigest) ||
			!isShardingScaleInSHA256(binding.ServerConfigurationDigest) {
			return invalid("executor %q server runtime binding must be complete", template.ExecutorPodUID)
		}
		expectedServerDigest, err := digestShardingScaleInServerConfiguration(
			template.ExecutorPodUID,
			binding.RegisteredActionDigest,
			binding.StartupEnvironmentSchemaDigest,
			template.BaseParameterDigest,
			template.LaunchSchemaDigest,
			template.PollSchemaDigest,
			template.CancelSchemaDigest,
			template.CredentialBindings,
		)
		if err != nil {
			return err
		}
		if binding.ServerConfigurationDigest != expectedServerDigest {
			return invalid("executor %q server configuration digest does not match",
				template.ExecutorPodUID)
		}
		pods := componentPods[componentUID]
		idx := slices.IndexFunc(pods, func(pod appsv1.ShardingScaleInPlanPod) bool {
			return pod.UID == template.ExecutorPodUID
		})
		if idx < 0 || pods[idx].AgentImageID != binding.AgentImageID ||
			pods[idx].AgentProcessUID != binding.AgentProcessUID {
			return invalid("executor %q server binding must match the plan Pod", template.ExecutorPodUID)
		}
	}
	for _, source := range material.RequestAuthority.CredentialSources {
		if _, ok := usedCredentialSources[source.SourceID]; !ok {
			return invalid("credential source is not referenced: %q", source.SourceID)
		}
		requiredKeys := make([]string, 0, len(requiredKeysBySource[source.SourceID]))
		for keyName := range requiredKeysBySource[source.SourceID] {
			requiredKeys = append(requiredKeys, keyName)
		}
		slices.Sort(requiredKeys)
		if !slices.Equal(source.KeyNames, requiredKeys) {
			return invalid("credential source %q key set must equal the exact binding union",
				source.SourceID)
		}
	}
	return nil
}

func canonicalizeShardingScaleInBaseParameterRecord(encoded, digest string,
) (string, string, shardingScaleInBaseParameterValues, error) {
	var values shardingScaleInBaseParameterValues
	if err := validateShardingScaleInEncodedDigest(encoded, digest); err != nil {
		return "", "", values, err
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return "", "", values, err
	}

	record := shardingScaleInBaseParameterRecord{}
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return "", "", values, fmt.Errorf("invalid fixed-field record: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return "", "", values, errors.New("fixed-field record must contain one JSON value")
	}
	if record.Version != shardingScaleInBaseParameterRecordVersionV1 {
		return "", "", values, fmt.Errorf("version must be %q",
			shardingScaleInBaseParameterRecordVersionV1)
	}
	seen := map[string]struct{}{}
	for _, parameter := range record.Parameters {
		if _, ok := seen[parameter.Name]; ok {
			return "", "", values, fmt.Errorf("base parameter %q is duplicated", parameter.Name)
		}
		seen[parameter.Name] = struct{}{}
		value, err := base64.StdEncoding.Strict().DecodeString(parameter.ValueB64)
		if err != nil || len(value) == 0 {
			return "", "", values, fmt.Errorf("base parameter %q must have non-empty canonical base64",
				parameter.Name)
		}
		switch parameter.Name {
		case shardingScaleInBaseParameterProtocolVersion:
			values.protocolVersion = string(value)
		case shardingScaleInBaseParameterServicePort:
			values.servicePort = string(value)
		default:
			return "", "", values, fmt.Errorf("base parameter %q is not allowed", parameter.Name)
		}
	}
	if len(record.Parameters) != 2 {
		return "", "", values, errors.New("base parameter key set must contain exactly two keys")
	}
	if values.protocolVersion == "" || values.servicePort == "" {
		return "", "", values, errors.New("base parameter key set is incomplete")
	}
	if err := validateShardingScaleInServicePort(values.servicePort); err != nil {
		return "", "", values, err
	}

	slices.SortFunc(record.Parameters, func(a, b shardingScaleInBaseParameter) int {
		return strings.Compare(a.Name, b.Name)
	})
	canonical, err := json.Marshal(record)
	if err != nil {
		return "", "", values, err
	}
	sum := sha256.Sum256(canonical)
	return base64.StdEncoding.EncodeToString(canonical), hex.EncodeToString(sum[:]), values, nil
}

func validateShardingScaleInServicePort(value string) error {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != value {
		return errors.New("SERVICE_PORT must be a canonical decimal from 1 through 65535")
	}
	return nil
}

func validateShardingScaleInEncodedDigest(encoded, digest string) error {
	if encoded == "" || !isShardingScaleInSHA256(digest) {
		return errors.New("encoded value and SHA256 digest must be non-empty")
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("invalid canonical base64: %w", err)
	}
	sum := sha256.Sum256(decoded)
	if hex.EncodeToString(sum[:]) != digest {
		return errors.New("SHA256 digest does not match the decoded value")
	}
	return nil
}

func digestShardingScaleInPrerequisiteIdentity(
	prerequisite appsv1.ShardingScaleInExecutorPrerequisite,
) (string, error) {
	prerequisite.IdentityDigest = ""
	return digestShardingScaleInCanonicalJSON(prerequisite)
}

func digestShardingScaleInCredentialSourceID(
	source appsv1.ShardingScaleInCredentialSource,
) (string, error) {
	return digestShardingScaleInCanonicalJSON(struct {
		Version    string    `json:"version"`
		APIVersion string    `json:"apiVersion"`
		Kind       string    `json:"kind"`
		Namespace  string    `json:"namespace"`
		Name       string    `json:"name"`
		UID        types.UID `json:"uid"`
	}{
		Version:    "kb.sharding.scalein.credential-source-id/v1",
		APIVersion: source.APIVersion,
		Kind:       source.Kind,
		Namespace:  source.Namespace,
		Name:       source.Name,
		UID:        source.UID,
	})
}

func digestShardingScaleInCredentialSource(
	source appsv1.ShardingScaleInCredentialSource,
) (string, error) {
	return digestShardingScaleInCanonicalJSON(struct {
		Version  string   `json:"version"`
		SourceID string   `json:"sourceID"`
		KeyNames []string `json:"keyNames"`
	}{
		Version:  "kb.sharding.scalein.credential-source/v1",
		SourceID: source.SourceID,
		KeyNames: source.KeyNames,
	})
}

func digestShardingScaleInExecutorCredentialBinding(
	binding appsv1.ShardingScaleInExecutorCredentialBinding,
	executorPodUID, executorComponentUID types.UID,
) (string, error) {
	binding.BindingDigest = ""
	return digestShardingScaleInCanonicalJSON(struct {
		Version              string                                          `json:"version"`
		ExecutorPodUID       types.UID                                       `json:"executorPodUID"`
		ExecutorComponentUID types.UID                                       `json:"executorComponentUID"`
		Binding              appsv1.ShardingScaleInExecutorCredentialBinding `json:"binding"`
	}{
		Version:              "kb.sharding.scalein.credential-binding/v1",
		ExecutorPodUID:       executorPodUID,
		ExecutorComponentUID: executorComponentUID,
		Binding:              binding,
	})
}

func digestShardingScaleInCanonicalJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("%w: canonical JSON: %v", errInvalidShardingScaleInPlanMaterial, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func isShardingScaleInSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func isShardingScaleInImageID(value string) bool {
	const algorithm = "sha256:"
	index := strings.LastIndex(value, algorithm)
	return index >= 0 && isShardingScaleInSHA256(value[index+len(algorithm):])
}

func compareShardingScaleInPlanMembers(a, b appsv1.ShardingScaleInPlanMember) int {
	return strings.Compare(a.ComponentName+"\x00"+string(a.ComponentUID),
		b.ComponentName+"\x00"+string(b.ComponentUID))
}

func compareShardingScaleInPlanPods(a, b appsv1.ShardingScaleInPlanPod) int {
	return strings.Compare(a.Name+"\x00"+string(a.UID), b.Name+"\x00"+string(b.UID))
}

func compareShardingScaleInRequestSourceObjects(
	a, b appsv1.ShardingScaleInRequestSourceObject,
) int {
	left := strings.Join([]string{a.APIVersion, a.Kind, a.Namespace, a.Name, string(a.UID)}, "\x00")
	right := strings.Join([]string{b.APIVersion, b.Kind, b.Namespace, b.Name, string(b.UID)}, "\x00")
	return strings.Compare(left, right)
}

func compareShardingScaleInPrerequisites(
	a, b appsv1.ShardingScaleInExecutorPrerequisite,
) int {
	left := strings.Join([]string{
		a.APIVersion, a.Kind, a.Namespace, a.Name, string(a.UID), string(a.Role), string(a.Scope),
		string(a.ComponentUID),
	}, "\x00")
	right := strings.Join([]string{
		b.APIVersion, b.Kind, b.Namespace, b.Name, string(b.UID), string(b.Role), string(b.Scope),
		string(b.ComponentUID),
	}, "\x00")
	return strings.Compare(left, right)
}
