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
// +kubebuilder:printcolumn:name="SERVICE",type="string",JSONPath=".spec.serviceKind",description="service"
// +kubebuilder:printcolumn:name="SERVICE-VERSION",type="string",JSONPath=".spec.serviceVersion",description="service version"
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
	// +kubebuilder:validation:Required
	Template ShardingTemplate `json:"template"`

	// Defines the upper limit of the number of shards supported by the sharding.
	//
	// This field is immutable.
	//
	// +optional
	ShardsLimit *ShardsLimit `json:"shardsLimit,omitempty"`

	// +kubebuilder:default=Serial
	// +optional
	ProvisionStrategy *UpdateStrategy `json:"provisionStrategy,omitempty"`

	// +kubebuilder:default=Serial
	// +optional
	UpdateStrategy *UpdateStrategy `json:"updateStrategy,omitempty"`

	// Defines a set of hooks and procedures that customize the behavior of a sharding throughout its lifecycle.
	//
	// This field is immutable.
	//
	// +optional
	LifecycleActions *ShardingLifecycleActions `json:"lifecycleActions,omitempty"`

	// Defines additional Services to expose the sharding's endpoints.
	//
	// This field is immutable.
	//
	// +optional
	Services []ShardingService `json:"services,omitempty"`

	// Defines the system accounts for the sharding.
	//
	// This field is immutable.
	//
	// +optional
	SystemAccounts []ShardingSystemAccount `json:"systemAccounts,omitempty"`
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
	// Specifies the hook to be executed after a shard's creation.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	ShardProvision *Action `json:"shardProvision,omitempty"`

	// Specifies the hook to be executed prior to terminating a shard.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	ShardTerminate *Action `json:"shardTerminate,omitempty"`
}

type ShardingService struct {
	// The name of the service defined in the sharding template.
	//
	// This field is immutable once set.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=25
	Name string `json:"name"`

	// Specifies whether the service is shared across shards.
	//
	// +optional
	Shared bool `json:"shared,omitempty"`
}

type ShardingSystemAccount struct {
	// The name of the system account defined in the sharding template.
	//
	// This field is immutable once set.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies whether the account is shared across shards.
	//
	// +optional
	Shared bool `json:"shared,omitempty"`
}
