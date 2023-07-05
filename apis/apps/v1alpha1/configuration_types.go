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

// ConfigurationSpec defines the desired state of Configuration
type ConfigurationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// clusterRef references clusterDefinition.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.clusterRef"
	ClusterRef string `json:"clusterRef"`

	// clusterComponentName references clusterDefinition.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.clusterRef"
	ClusterComponentName string `json:"clusterComponentName"`

	// Cluster referencing ClusterDefinition name. This is an immutable attribute.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	ClusterDefRef string `json:"clusterDefinitionRef"`

	// Cluster referencing ClusterVersion name.
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	ClusterVersionRef string `json:"clusterVersionRef,omitempty"`

	// customConfigurationItems describes user-defined config template.
	// +optional
	CustomConfigurationItems []CustomConfigurationItem `json:"customConfigurationItems,omitempty"`
}

type CustomConfigurationItem struct {
	// Specify the name of configuration template.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Specify the type of config template.
	ImportTemplateRef *RenderedConfigTemplateSpec `json:"importTemplateRef,omitempty"`

	// +optional
	ConfigParams map[string]ConfigParams `json:"configParams"`
}

// ConfigurationStatus defines the observed state of Configuration
type ConfigurationStatus struct {
	// phase is status of configuration.
	// +optional
	Phase ConfigurationPhase `json:"phase,omitempty"`

	// message field describes the reasons of abnormal status.
	// +optional
	Message string `json:"message,omitempty"`

	// observedGeneration is the latest generation observed for this
	// ClusterDefinition. It refers to the ConfigConstraint's generation, which is
	// updated by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Describe current state of cluster API Resource, like warning.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
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
