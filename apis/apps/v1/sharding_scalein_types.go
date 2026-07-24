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

package v1

import "k8s.io/apimachinery/pkg/types"

// ShardingScaleInPlanMaterial is the immutable, map-free input hashed by a scale-in plan ID.
type ShardingScaleInPlanMaterial struct {
	// ProtocolVersion identifies the typed shard scale-in contract.
	ProtocolVersion ShardingActionResultProtocol `json:"protocolVersion"`

	// ShardingName is the exact Cluster sharding name.
	ShardingName string `json:"shardingName"`

	// Source identifies the exact Cluster intent that produced this plan.
	Source ShardingScaleInPlanSource `json:"source"`

	// Action identifies the exact ShardingDefinition action contract.
	Action ShardingScaleInPlanAction `json:"action"`

	// DeletionGuard identifies the exact deletion-guard capability required by the plan.
	DeletionGuard ShardingScaleInDeletionGuardIdentity `json:"deletionGuard"`

	// RequestAuthority closes every source and executor identity used to build action requests.
	RequestAuthority ShardingScaleInRequestAuthority `json:"requestAuthority"`

	// Leaving lists the exact Components removed by the plan.
	//
	// +kubebuilder:validation:MinItems=1
	Leaving []ShardingScaleInPlanMember `json:"leaving"`

	// Staying lists the exact Components retained by the plan.
	//
	// +kubebuilder:validation:MinItems=1
	Staying []ShardingScaleInPlanMember `json:"staying"`

	// ProofExecutor is the immutable first canonical Pod in Staying.
	ProofExecutor ShardingScaleInProofExecutor `json:"proofExecutor"`

	// ExecutorPrerequisites lists exact deletion-critical objects used by plan executors.
	//
	// +kubebuilder:validation:MinItems=1
	ExecutorPrerequisites []ShardingScaleInExecutorPrerequisite `json:"executorPrerequisites"`
}

// ShardingScaleInPlanSource identifies the Cluster intent that produced a plan.
type ShardingScaleInPlanSource struct {
	ClusterNamespace string    `json:"clusterNamespace"`
	ClusterName      string    `json:"clusterName"`
	ClusterUID       types.UID `json:"clusterUID"`

	// +kubebuilder:validation:Minimum=1
	ClusterGeneration int64 `json:"clusterGeneration"`

	DesiredDigest string `json:"desiredDigest"`

	// +kubebuilder:validation:Minimum=1
	DesiredShards int32 `json:"desiredShards"`

	// +optional
	OptionalOpsRequestName string `json:"optionalOpsRequestName,omitempty"`

	// +optional
	OptionalOpsRequestUID types.UID `json:"optionalOpsRequestUID,omitempty"`
}

// ShardingScaleInPlanAction identifies the exact ShardingDefinition action.
type ShardingScaleInPlanAction struct {
	ShardingDefinitionName string    `json:"shardingDefinitionName"`
	ShardingDefinitionUID  types.UID `json:"shardingDefinitionUID"`

	// +kubebuilder:validation:Minimum=1
	ShardingDefinitionGeneration int64 `json:"shardingDefinitionGeneration"`

	ActionDigest   string                       `json:"actionDigest"`
	ResultProtocol ShardingActionResultProtocol `json:"resultProtocol"`
}

// ShardingScaleInDeletionGuardIdentity identifies the exact deletion-guard capability.
type ShardingScaleInDeletionGuardIdentity struct {
	Protocol            ShardingScaleInDeletionGuardProtocol `json:"protocol"`
	InstallationUID     types.UID                            `json:"installationUID"`
	PolicyRevision      string                               `json:"policyRevision"`
	ConfigurationDigest string                               `json:"configurationDigest"`
}

// ShardingScaleInDeletionGuardProtocol identifies the deletion-guard capability contract.
//
// +enum
// +kubebuilder:validation:Enum={scale-in-deletion-guard/v1}
type ShardingScaleInDeletionGuardProtocol string

const (
	ShardingScaleInDeletionGuardProtocolV1 ShardingScaleInDeletionGuardProtocol = "scale-in-deletion-guard/v1"
)

// ShardingScaleInRequestAuthority closes every source used to construct typed action requests.
type ShardingScaleInRequestAuthority struct {
	Version ShardingScaleInRequestAuthorityVersion `json:"version"`
	Builder ShardingScaleInRequestBuilder          `json:"builder"`

	GenericLifecycleSynthesisForbidden bool `json:"genericLifecycleSynthesisForbidden"`

	ActionName             string `json:"actionName"`
	ActionDefinitionDigest string `json:"actionDefinitionDigest"`
	SourceSnapshotDigest   string `json:"sourceSnapshotDigest"`

	// +kubebuilder:validation:MinItems=1
	ComponentDefinitionSources []ShardingScaleInComponentDefinitionSource `json:"componentDefinitionSources"`

	VarSources []ShardingScaleInVarSource `json:"varSources"`

	// +optional
	CredentialSources []ShardingScaleInCredentialSource `json:"credentialSources,omitempty"`

	// +kubebuilder:validation:MinItems=1
	ExecutorTemplates []ShardingScaleInExecutorTemplate `json:"executorTemplates"`

	RequestAuthorityDigest string `json:"requestAuthorityDigest"`
}

// ShardingScaleInRequestAuthorityVersion identifies a request-authority contract.
//
// +enum
// +kubebuilder:validation:Enum={kb.sharding.scalein.request-authority/v1,kb.sharding.scalein.request-authority/v2}
type ShardingScaleInRequestAuthorityVersion string

const (
	ShardingScaleInRequestAuthorityVersionV1 ShardingScaleInRequestAuthorityVersion = "kb.sharding.scalein.request-authority/v1"
	ShardingScaleInRequestAuthorityVersionV2 ShardingScaleInRequestAuthorityVersion = "kb.sharding.scalein.request-authority/v2"
)

// ShardingScaleInRequestBuilder identifies the only request builder allowed by a plan.
//
// +enum
// +kubebuilder:validation:Enum={TypedShardScaleInV1,TypedShardScaleInV2}
type ShardingScaleInRequestBuilder string

const (
	ShardingScaleInRequestBuilderTypedV1 ShardingScaleInRequestBuilder = "TypedShardScaleInV1"
	ShardingScaleInRequestBuilderTypedV2 ShardingScaleInRequestBuilder = "TypedShardScaleInV2"
)

// ShardingScaleInComponentDefinitionSource identifies ComponentDefinition vars used at plan time.
type ShardingScaleInComponentDefinitionSource struct {
	Name       string    `json:"name"`
	UID        types.UID `json:"uid"`
	Generation int64     `json:"generation"`

	VarsProjectionDigest string `json:"varsProjectionDigest"`
}

// ShardingScaleInVarSource records one declared variable and its exact provenance.
type ShardingScaleInVarSource struct {
	VariableName string                         `json:"variableName"`
	ResolverKind ShardingScaleInVarResolverKind `json:"resolverKind"`
	Consumption  ShardingScaleInVarConsumption  `json:"consumption"`

	ResolvedNonSecretValueB64    string `json:"resolvedNonSecretValueB64"`
	ResolvedNonSecretValueDigest string `json:"resolvedNonSecretValueDigest"`

	SourceObjects []ShardingScaleInRequestSourceObject `json:"sourceObjects"`

	// SecretSource is retained only to decode legacy v1 material so v2
	// validation can reject it explicitly.
	// +optional
	SecretSource *ShardingScaleInSecretSource `json:"secretSource,omitempty"`
}

// ShardingScaleInVarResolverKind identifies a reviewed variable resolver.
//
// +enum
// +kubebuilder:validation:Enum={StaticValue,ClusterVarRef,ComponentVarRef,ResourceVarRef,CredentialVarRef}
type ShardingScaleInVarResolverKind string

const (
	ShardingScaleInVarResolverStaticValue      ShardingScaleInVarResolverKind = "StaticValue"
	ShardingScaleInVarResolverClusterVarRef    ShardingScaleInVarResolverKind = "ClusterVarRef"
	ShardingScaleInVarResolverComponentVarRef  ShardingScaleInVarResolverKind = "ComponentVarRef"
	ShardingScaleInVarResolverResourceVarRef   ShardingScaleInVarResolverKind = "ResourceVarRef"
	ShardingScaleInVarResolverCredentialVarRef ShardingScaleInVarResolverKind = "CredentialVarRef"
)

// ShardingScaleInVarConsumption classifies where a resolved variable may be consumed.
//
// +enum
// +kubebuilder:validation:Enum={TypedBase,DurableEnvelope,ServerStartupSecret,ServerReservedIdentity,ForbiddenForTypedScaleIn}
type ShardingScaleInVarConsumption string

const (
	ShardingScaleInVarConsumptionTypedBase              ShardingScaleInVarConsumption = "TypedBase"
	ShardingScaleInVarConsumptionDurableEnvelope        ShardingScaleInVarConsumption = "DurableEnvelope"
	ShardingScaleInVarConsumptionServerStartupSecret    ShardingScaleInVarConsumption = "ServerStartupSecret"
	ShardingScaleInVarConsumptionServerReservedIdentity ShardingScaleInVarConsumption = "ServerReservedIdentity"
	ShardingScaleInVarConsumptionForbidden              ShardingScaleInVarConsumption = "ForbiddenForTypedScaleIn"
)

// ShardingScaleInRequestSourceObject records one exact non-secret request source.
type ShardingScaleInRequestSourceObject struct {
	APIVersion       string                              `json:"apiVersion"`
	Kind             string                              `json:"kind"`
	Namespace        string                              `json:"namespace"`
	Name             string                              `json:"name"`
	UID              types.UID                           `json:"uid"`
	ProjectionKind   ShardingScaleInSourceProjectionKind `json:"projectionKind"`
	ProjectionDigest string                              `json:"projectionDigest"`
}

// ShardingScaleInSourceProjectionKind identifies a reviewed request-source projection.
//
// +enum
// +kubebuilder:validation:Enum={ClusterIdentity,ComponentIdentity,InstanceSetSpec}
type ShardingScaleInSourceProjectionKind string

const (
	ShardingScaleInSourceProjectionClusterIdentity   ShardingScaleInSourceProjectionKind = "ClusterIdentity"
	ShardingScaleInSourceProjectionComponentIdentity ShardingScaleInSourceProjectionKind = "ComponentIdentity"
	ShardingScaleInSourceProjectionInstanceSetSpec   ShardingScaleInSourceProjectionKind = "InstanceSetSpec"
)

// ShardingScaleInSecretSource is the legacy v1 inline credential source shape.
type ShardingScaleInSecretSource struct {
	APIVersion string    `json:"apiVersion"`
	Kind       string    `json:"kind"`
	Namespace  string    `json:"namespace"`
	Name       string    `json:"name"`
	UID        types.UID `json:"uid"`

	// +kubebuilder:validation:MinItems=1
	KeyNames []string `json:"keyNames"`
}

// ShardingScaleInCredentialSource records one canonical Secret identity and
// the exact union of key names used by executor bindings. It never contains
// Secret values or value digests.
type ShardingScaleInCredentialSource struct {
	SourceID string `json:"sourceID"`

	APIVersion string    `json:"apiVersion"`
	Kind       string    `json:"kind"`
	Namespace  string    `json:"namespace"`
	Name       string    `json:"name"`
	UID        types.UID `json:"uid"`

	// +kubebuilder:validation:MinItems=1
	KeyNames []string `json:"keyNames"`

	CredentialSourceDigest string `json:"credentialSourceDigest"`
}

// ShardingScaleInExecutorCredentialBinding binds one declared variable to a canonical Secret source.
type ShardingScaleInExecutorCredentialBinding struct {
	VariableName             string   `json:"variableName"`
	CredentialSourceID       string   `json:"credentialSourceID"`
	CredentialSourceDigest   string   `json:"credentialSourceDigest"`
	RequiredKeyNames         []string `json:"requiredKeyNames"`
	ResolverProjectionDigest string   `json:"resolverProjectionDigest"`
	BindingDigest            string   `json:"bindingDigest"`
}

// ShardingScaleInExecutorTemplate records an immutable request base for one executor.
type ShardingScaleInExecutorTemplate struct {
	ExecutorPodUID       types.UID `json:"executorPodUID"`
	ExecutorComponentUID types.UID `json:"executorComponentUID"`

	// +optional
	CredentialBindings []ShardingScaleInExecutorCredentialBinding `json:"credentialBindings,omitempty"`

	BaseParameterRecordB64 string `json:"baseParameterRecordB64"`
	BaseParameterDigest    string `json:"baseParameterDigest"`
	LaunchSchemaDigest     string `json:"launchSchemaDigest"`
	PollSchemaDigest       string `json:"pollSchemaDigest"`
	CancelSchemaDigest     string `json:"cancelSchemaDigest"`

	ServerRuntimeBinding ShardingScaleInServerRuntimeBinding `json:"serverRuntimeBinding"`
}

// ShardingScaleInServerRuntimeBinding identifies the exact kbagent process and server configuration.
type ShardingScaleInServerRuntimeBinding struct {
	AgentProcessUID                string `json:"agentProcessUID"`
	AgentImageID                   string `json:"agentImageID"`
	RegisteredActionDigest         string `json:"registeredActionDigest"`
	StartupEnvironmentSchemaDigest string `json:"startupEnvironmentSchemaDigest"`
	ServerConfigurationDigest      string `json:"serverConfigurationDigest"`
}

// ShardingScaleInPlanMember identifies one immutable Component and its Pods.
type ShardingScaleInPlanMember struct {
	ComponentName string    `json:"componentName"`
	ComponentUID  types.UID `json:"componentUID"`

	// +kubebuilder:validation:Minimum=1
	ComponentGeneration int64 `json:"componentGeneration"`

	ComponentSpecDigest string `json:"componentSpecDigest"`
	ComponentShortName  string `json:"componentShortName"`
	ShardTemplateName   string `json:"shardTemplateName"`

	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=5
	Pods []ShardingScaleInPlanPod `json:"pods"`
}

// ShardingScaleInPlanPod identifies one immutable Pod and its kbagent runtime binding.
type ShardingScaleInPlanPod struct {
	Name string    `json:"name"`
	UID  types.UID `json:"uid"`
	FQDN string    `json:"fqdn"`

	AgentImageID          string `json:"agentImageID"`
	AgentProcessUID       string `json:"agentProcessUID"`
	AgentCapabilityDigest string `json:"agentCapabilityDigest"`
}

// ShardingScaleInProofExecutor identifies the immutable read-only proof executor.
type ShardingScaleInProofExecutor struct {
	PodName string    `json:"podName"`
	PodUID  types.UID `json:"podUID"`
}

// ShardingScaleInExecutorPrerequisite identifies one deletion-critical executor object.
type ShardingScaleInExecutorPrerequisite struct {
	APIVersion string                           `json:"apiVersion"`
	Kind       string                           `json:"kind"`
	Namespace  string                           `json:"namespace"`
	Name       string                           `json:"name"`
	UID        types.UID                        `json:"uid"`
	Role       ShardingScaleInPrerequisiteRole  `json:"role"`
	Scope      ShardingScaleInPrerequisiteScope `json:"scope"`

	// +optional
	ComponentUID types.UID `json:"componentUID,omitempty"`

	CriticalSpecDigest string `json:"criticalSpecDigest"`
	IdentityDigest     string `json:"identityDigest"`
}

// ShardingScaleInPrerequisiteRole identifies a reviewed executor prerequisite role.
//
// +enum
// +kubebuilder:validation:Enum={PodOwnerWorkload,ClusterDNS,KBAgentEndpoint}
type ShardingScaleInPrerequisiteRole string

const (
	ShardingScaleInPrerequisiteRolePodOwnerWorkload ShardingScaleInPrerequisiteRole = "PodOwnerWorkload"
	ShardingScaleInPrerequisiteRoleClusterDNS       ShardingScaleInPrerequisiteRole = "ClusterDNS"
	ShardingScaleInPrerequisiteRoleKBAgentEndpoint  ShardingScaleInPrerequisiteRole = "KBAgentEndpoint"
)

// ShardingScaleInPrerequisiteScope identifies whether a prerequisite is Component-local or shared.
//
// +enum
// +kubebuilder:validation:Enum={Component,Shared}
type ShardingScaleInPrerequisiteScope string

const (
	ShardingScaleInPrerequisiteScopeComponent ShardingScaleInPrerequisiteScope = "Component"
	ShardingScaleInPrerequisiteScopeShared    ShardingScaleInPrerequisiteScope = "Shared"
)
