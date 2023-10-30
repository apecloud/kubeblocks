/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ConfigurationItemDetail struct {
	// Specify the name of configuration template.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// version is the version of configuration template.
	// +optional
	Version string `json:"version,omitempty"`

	// configSpec is used to set the configuration template.
	// +optional
	ConfigSpec *ComponentConfigSpec `json:"configSpec"`

	// Specify the configuration template.
	// +optional
	ImportTemplateRef *ConfigTemplateExtension `json:"importTemplateRef"`

	// configFileParams is used to set the parameters to be updated.
	// +optional
	ConfigFileParams map[string]ConfigParams `json:"configFileParams,omitempty"`
}

// ConfigurationSpec defines the desired state of Configuration
type ConfigurationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// clusterRef references Cluster name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.clusterRef"
	ClusterRef string `json:"clusterRef"`

	// componentName is cluster component name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.clusterRef"
	ComponentName string `json:"componentName"`

	// customConfigurationItems describes user-defined config template.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ConfigItemDetails []ConfigurationItemDetail `json:"configItemDetails,omitempty"`
}

type ReconcileDetail struct {
	// policy is the policy of the latest execution.
	// +optional
	Policy string `json:"policy"`
	// execResult is the result of the latest execution.
	// +optional
	ExecResult string `json:"execResult"`

	// currentRevision is the current revision of configurationItem.
	// +optional
	CurrentRevision string `json:"currentRevision,omitempty"`

	// succeedCount is the number of pods for which configuration changes were successfully executed.
	// +kubebuilder:default=-1
	// +optional
	SucceedCount int32 `json:"succeedCount,omitempty"`

	// expectedCount is the number of pods that need to be executed for configuration changes.
	// +kubebuilder:default=-1
	// +optional
	ExpectedCount int32 `json:"expectedCount,omitempty"`

	// errMessage is the error message when the configuration change execution fails.
	// +optional
	ErrMessage string `json:"errMessage,omitempty"`
}

type ConfigurationItemDetailStatus struct {
	// name is a config template name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// phase is status of configurationItem.
	// +optional
	Phase ConfigurationPhase `json:"phase,omitempty"`

	// lastDoneRevision is the last done revision of configurationItem.
	// +optional
	LastDoneRevision string `json:"lastDoneRevision,omitempty"`

	// currentRevision is the current revision of configurationItem.
	// +optional
	// CurrentRevision string `json:"currentRevision,omitempty"`

	// updateRevision is the update revision of configurationItem.
	// +optional
	UpdateRevision string `json:"updateRevision,omitempty"`

	// message field describes the reasons of abnormal status.
	// +optional
	Message *string `json:"message,omitempty"`

	// reconcileDetail describes the details of the configuration change execution.
	// +optional
	ReconcileDetail *ReconcileDetail `json:"reconcileDetail,omitempty"`
}

// ConfigurationStatus defines the observed state of Configuration
type ConfigurationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// message field describes the reasons of abnormal status.
	// +optional
	Message string `json:"message,omitempty"`

	// observedGeneration is the latest generation observed for this
	// ClusterDefinition. It refers to the ConfigConstraint's generation, which is
	// updated by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// conditions describes opsRequest detail status.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// configurationStatus describes the status of the component reconfiguring.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ConfigurationItemStatus []ConfigurationItemDetailStatus `json:"configurationStatus"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Configuration is the Schema for the configurations API
type Configuration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigurationSpec   `json:"spec,omitempty"`
	Status ConfigurationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

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

func (status *ConfigurationStatus) GetItemStatus(name string) *ConfigurationItemDetailStatus {
	for i := range status.ConfigurationItemStatus {
		itemStatus := &status.ConfigurationItemStatus[i]
		if itemStatus.Name == name {
			return itemStatus
		}
	}
	return nil
}
