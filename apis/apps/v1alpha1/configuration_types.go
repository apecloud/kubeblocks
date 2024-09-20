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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Payload struct {
	// Holds the payload data. This field is optional and can contain any type of data.
	// Not included in the JSON representation of the object.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Data map[string]any `json:"-"`
}

// ConfigurationItemDetail corresponds to settings of a configuration template (a ConfigMap).
type ConfigurationItemDetail struct {
	// Defines the unique identifier of the configuration template.
	//
	// It must be a string of maximum 63 characters, and can only include lowercase alphanumeric characters,
	// hyphens, and periods.
	// The name must start and end with an alphanumeric character.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// External controllers can trigger a configuration rerender by modifying this field.
	//
	// Note: Currently, the `payload` field is opaque and its content is not interpreted by the system.
	// Modifying this field will cause a rerender, regardless of the specific content of this field.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Payload Payload `json:"payload,omitempty"`

	// Specifies the name of the configuration template (a ConfigMap), ConfigConstraint, and other miscellaneous options.
	//
	// The configuration template is a ConfigMap that contains multiple configuration files.
	// Each configuration file is stored as a key-value pair within the ConfigMap.
	//
	// ConfigConstraint allows defining constraints and validation rules for configuration parameters.
	// It ensures that the configuration adheres to certain requirements and limitations.
	//
	// +optional
	ConfigSpec *ComponentConfigSpec `json:"configSpec,omitempty"`

	// Specifies the user-defined configuration template.
	//
	// When provided, the `importTemplateRef` overrides the default configuration template
	// specified in `configSpec.templateRef`.
	// This allows users to customize the configuration template according to their specific requirements.
	//
	// +optional
	ImportTemplateRef *ConfigTemplateExtension `json:"importTemplateRef,omitempty"`

	// Specifies the user-defined configuration parameters.
	//
	// When provided, the parameter values in `configFileParams` override the default configuration parameters.
	// This allows users to override the default configuration according to their specific needs.
	//
	// +optional
	ConfigFileParams map[string]ConfigParams `json:"configFileParams,omitempty"`
}

// ConfigurationSpec defines the desired state of ComponentConfiguration
type ConfigurationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Specifies the name of the Cluster that this configuration is associated with.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.clusterRef"
	// +optional
	ClusterName string `json:"clusterName,omitempty"`

	// Represents the name of the Component that this configuration pertains to.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.clusterRef"
	ComponentName string `json:"componentName"`

	// ConfigItemDetails is an array of ConfigurationItemDetail objects.
	//
	// Each ConfigurationItemDetail corresponds to a configuration template,
	// which is a ConfigMap that contains multiple configuration files.
	// Each configuration file is stored as a key-value pair within the ConfigMap.
	//
	// The ConfigurationItemDetail includes information such as:
	//
	// - The configuration template (a ConfigMap)
	// - The corresponding ConfigConstraint (constraints and validation rules for the configuration)
	// - Volume mounts (for mounting the configuration files)
	//
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ConfigItemDetails []ConfigurationItemDetail `json:"configItemDetails,omitempty"`
}

type ReconcileDetail struct {
	// Represents the policy applied during the most recent execution.
	//
	// +optional
	Policy string `json:"policy"`

	// Represents the outcome of the most recent execution.
	//
	// +optional
	ExecResult string `json:"execResult"`

	// Represents the current revision of the configuration item.
	//
	// +optional
	CurrentRevision string `json:"currentRevision,omitempty"`

	// Represents the number of pods where configuration changes were successfully applied.
	//
	// +kubebuilder:default=-1
	// +optional
	SucceedCount int32 `json:"succeedCount,omitempty"`

	// Represents the total number of pods that require execution of configuration changes.
	//
	// +kubebuilder:default=-1
	// +optional
	ExpectedCount int32 `json:"expectedCount,omitempty"`

	// Represents the error message generated when the execution of configuration changes fails.
	//
	// +optional
	ErrMessage string `json:"errMessage,omitempty"`
}

type ConfigurationItemDetailStatus struct {
	// Specifies the name of the configuration template. It is a required field and must be a string of maximum 63 characters.
	// The name should only contain lowercase alphanumeric characters, hyphens, or periods. It should start and end with an alphanumeric character.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Indicates the current status of the configuration item.
	//
	// Possible values include "Creating", "Init", "Running", "Pending", "Merged", "MergeFailed", "FailedAndPause",
	// "Upgrading", "Deleting", "FailedAndRetry", "Finished".
	//
	// +optional
	Phase ConfigurationPhase `json:"phase,omitempty"`

	// Represents the last completed revision of the configuration item. This field is optional.
	//
	// +optional
	LastDoneRevision string `json:"lastDoneRevision,omitempty"`

	// Represents the current revision of the configuration item. This field is optional.
	//
	// +optional
	// CurrentRevision string `json:"currentRevision,omitempty"`

	// Represents the updated revision of the configuration item. This field is optional.
	//
	// +optional
	UpdateRevision string `json:"updateRevision,omitempty"`

	// Provides a description of any abnormal status. This field is optional.
	//
	// +optional
	Message *string `json:"message,omitempty"`

	// Provides detailed information about the execution of the configuration change. This field is optional.
	//
	// +optional
	ReconcileDetail *ReconcileDetail `json:"reconcileDetail,omitempty"`
}

// ConfigurationStatus defines the observed state of ComponentConfiguration
type ConfigurationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Provides a description of any abnormal status.
	// +optional
	Message string `json:"message,omitempty"`

	// Represents the latest generation observed for this
	// ClusterDefinition. It corresponds to the ConfigConstraint's generation, which is
	// updated by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Provides detailed status information for opsRequest.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Provides the status of each component undergoing reconfiguration.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ConfigurationItemStatus []ConfigurationItemDetailStatus `json:"configurationStatus"`
}

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all},shortName=config
// +kubebuilder:printcolumn:name="CLUSTER",type="string",JSONPath=".spec.clusterName",description="cluster name"
// +kubebuilder:printcolumn:name="COMPONENT",type="string",JSONPath=".spec.componentName",description="component name"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="config status phase."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Configuration is the Schema for the configurations API
type Configuration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigurationSpec   `json:"spec,omitempty"`
	Status ConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConfigurationList contains a list of Configuration
type ConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Configuration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Configuration{}, &ConfigurationList{})
}
