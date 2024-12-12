/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=sdd
// +kubebuilder:printcolumn:name="TEMPLATE",type="string",JSONPath=".spec.template.compDef",description="template"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ShardingDefinition is the Schema for the shardingdefinitions API
type ShardingDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ShardingDefinitionSpec   `json:"spec,omitempty"`
	Status ShardingDefinitionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ShardingDefinitionList contains a list of ShardingDefinition
type ShardingDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ShardingDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ShardingDefinition{}, &ShardingDefinitionList{})
}

// ShardingDefinitionSpec defines the desired state of ShardingDefinition
type ShardingDefinitionSpec struct {
	// This field is immutable.
	//
	// +kubebuilder:validation:Required
	Template ShardingTemplate `json:"template"`

	// Defines the upper limit of the number of shards supported by the sharding.
	//
	// This field is immutable.
	//
	// +optional
	ShardsLimit *ShardsLimit `json:"shardsLimit,omitempty"`

	// Specifies the strategy for provisioning shards of the sharding. Only `Serial` and `Parallel` are supported.
	//
	// This field is immutable.
	//
	// +kubebuilder:default=Serial
	// +optional
	ProvisionStrategy *UpdateConcurrency `json:"provisionStrategy,omitempty"`

	// Specifies the strategy for updating shards of the sharding. Only `Serial` and `Parallel` are supported.
	//
	// This field is immutable.
	//
	// +kubebuilder:default=Serial
	// +optional
	UpdateStrategy *UpdateConcurrency `json:"updateStrategy,omitempty"`

	// Defines a set of hooks and procedures that customize the behavior of a sharding throughout its lifecycle.
	//
	// This field is immutable.
	//
	// +optional
	LifecycleActions *ShardingLifecycleActions `json:"lifecycleActions,omitempty"`

	// Defines the system accounts for the sharding.
	//
	// This field is immutable.
	//
	// +optional
	SystemAccounts []ShardingSystemAccount `json:"systemAccounts,omitempty"`

	// Defines the TLS for the sharding.
	//
	// This field is immutable.
	//
	// +optional
	TLS *ShardingTLS `json:"tls,omitempty"`
}

// ShardingDefinitionStatus defines the observed state of ShardingDefinition
type ShardingDefinitionStatus struct {
	// Refers to the most recent generation that has been observed for the ShardingDefinition.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the current status of the ShardingDefinition. Valid values include ``, `Available`, and `Unavailable`.
	// When the status is `Available`, the ShardingDefinition is ready and can be utilized by related objects.
	//
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Provides additional information about the current phase.
	//
	// +optional
	Message string `json:"message,omitempty"`
}

type ShardingTemplate struct {
	// The component definition(s) that the sharding is based on.
	//
	// The component definition can be specified using one of the following:
	//
	// - the full name
	// - the regular expression pattern ('^' will be added to the beginning of the pattern automatically)
	//
	// This field is immutable.
	//
	// +kubebuilder:validation:Required
	CompDef string `json:"compDef"`
}

// ShardsLimit defines the valid range of number of shards supported.
//
// +kubebuilder:validation:XValidation:rule="self.minShards >= 0 && self.maxShards <= 2048",message="the minimum and maximum limit of shards should be in the range of [0, 2048]"
// +kubebuilder:validation:XValidation:rule="self.minShards <= self.maxShards",message="the minimum shards limit should be no greater than the maximum"
type ShardsLimit struct {
	// The minimum limit of shards.
	//
	// +kubebuilder:validation:Required
	MinShards int32 `json:"minShards"`

	// The maximum limit of replicas.
	//
	// +kubebuilder:validation:Required
	MaxShards int32 `json:"maxShards"`
}

// ShardingLifecycleActions defines a collection of Actions for customizing the behavior of a sharding.
type ShardingLifecycleActions struct {
	// Specifies the hook to be executed after a sharding's creation.
	//
	// By setting `postProvision.preCondition`, you can determine the specific lifecycle stage at which
	// the action should trigger, available conditions for sharding include: `Immediately`, `ComponentReady`,
	// and `ClusterReady`. For sharding, the `ComponentReady` condition means all components of the sharding are ready.
	//
	// With `ComponentReady` being the default.
	//
	// The PostProvision Action is intended to run only once.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	PostProvision *Action `json:"postProvision,omitempty"`

	// Specifies the hook to be executed prior to terminating a sharding.
	//
	// The PreTerminate Action is intended to run only once.
	//
	// This action is executed immediately when a terminate operation for the sharding is initiated.
	// The actual termination and cleanup of the sharding and its associated resources will not proceed
	// until the PreTerminate action has completed successfully.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	PreTerminate *Action `json:"preTerminate,omitempty"`

	// Specifies the hook to be executed after a shard added.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	ShardAdd *Action `json:"shardAdd,omitempty"`

	// Specifies the hook to be executed prior to remove a shard.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	ShardRemove *Action `json:"shardRemove,omitempty"`
}

type ShardingSystemAccount struct {
	// The name of the system account defined in the sharding template.
	//
	// This field is immutable once set.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies whether the account is shared across all shards in the sharding.
	//
	// +optional
	Shared *bool `json:"shared,omitempty"`
}

type ShardingTLS struct {
	// Specifies whether the TLS configuration is shared across all shards in the sharding.
	//
	// +optional
	Shared *bool `json:"shared,omitempty"`
}
