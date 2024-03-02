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
	"encoding/json"

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

// ConfigurationItemDetail represents a specific configuration item within a configuration template.
type ConfigurationItemDetail struct {
	// Defines the unique identifier of the configuration template. It must be a string of maximum 63 characters, and can only include lowercase alphanumeric characters, hyphens, and periods. The name must start and end with an alphanumeric character.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Deprecated: No longer used. Please use 'Payload' instead. Previously represented the version of the configuration template.
	//
	// +optional
	Version string `json:"version,omitempty"`

	// Holds the configuration-related rerender. Preserves unknown fields and is optional.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Payload Payload `json:"payload,omitempty"`

	// Used to set the configuration template. It is optional.
	//
	// +optional
	ConfigSpec *ComponentConfigSpec `json:"configSpec"`

	// Specifies the configuration template. It is optional.
	//
	// +optional
	ImportTemplateRef *ConfigTemplateExtension `json:"importTemplateRef"`

	// Used to set the parameters to be updated. It is optional.
	//
	// +optional
	ConfigFileParams map[string]ConfigParams `json:"configFileParams,omitempty"`
}

// ConfigurationSpec defines the desired state of a Configuration resource.
type ConfigurationSpec struct {

	// Specifies the name of the cluster that this configuration is associated with.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.clusterRef"
	ClusterRef string `json:"clusterRef"`

	// Represents the name of the cluster component that this configuration pertains to.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.clusterRef"
	ComponentName string `json:"componentName"`

	// An array of ConfigurationItemDetail objects that describe user-defined configuration templates.
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

	// Indicates the current status of the configuration item. This field is optional.
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

// Represents the observed state of a Configuration resource.

type ConfigurationStatus struct {
	// This is a placeholder for additional fields that describe the observed state of the cluster.
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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

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

func (configuration *ConfigurationSpec) GetConfigurationItem(name string) *ConfigurationItemDetail {
	for i := range configuration.ConfigItemDetails {
		configItem := &configuration.ConfigItemDetails[i]
		if configItem.Name == name {
			return configItem
		}
	}
	return nil
}

func (configuration *ConfigurationSpec) GetConfigSpec(configSpecName string) *ComponentConfigSpec {
	if configItem := configuration.GetConfigurationItem(configSpecName); configItem != nil {
		return configItem.ConfigSpec
	}
	return nil
}

func (status *ConfigurationStatus) GetItemStatus(name string) *ConfigurationItemDetailStatus {
	for i := range status.ConfigurationItemStatus {
		itemStatus := &status.ConfigurationItemStatus[i]
		if itemStatus.Name == name {
			return itemStatus
		}
	}
	return nil
}

// MarshalJSON implements the Marshaler interface.
func (c *Payload) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.Data)
}

// UnmarshalJSON implements the Unmarshaler interface.
func (c *Payload) UnmarshalJSON(data []byte) error {
	var out map[string]interface{}
	err := json.Unmarshal(data, &out)
	if err != nil {
		return err
	}
	c.Data = out
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
// This exists here to work around https://github.com/kubernetes/code-generator/issues/50
func (c *Payload) DeepCopyInto(out *Payload) {
	bytes, err := json.Marshal(c.Data)
	if err != nil {
		// TODO how to process error: panic or ignore
		return // ignore
	}
	var clone map[string]interface{}
	err = json.Unmarshal(bytes, &clone)
	if err != nil {
		// TODO how to process error: panic or ignore
		return // ignore
	}
	out.Data = clone
}
