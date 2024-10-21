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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=scd
// +kubebuilder:printcolumn:name="Owner",type="string",JSONPath=".spec.owner",description="owner"
// +kubebuilder:printcolumn:name="Selector",type="string",JSONPath=".spec.selectors",description="selectors"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// SidecarDefinition is the Schema for the sidecardefinitions API
type SidecarDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SidecarDefinitionSpec   `json:"spec,omitempty"`
	Status SidecarDefinitionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SidecarDefinitionList contains a list of SidecarDefinition
type SidecarDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SidecarDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SidecarDefinition{}, &SidecarDefinitionList{})
}

// SidecarDefinitionSpec defines the desired state of SidecarDefinition
type SidecarDefinitionSpec struct {
	// Specifies the name of the sidecar.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the component definition that the sidecar belongs to.
	//
	// For a specific cluster object, if there is any components provided by the component definition of @owner,
	// the sidecar will be created and injected into the components which are provided by
	// the component definition of @selectors automatically.
	//
	// This field is immutable.
	//
	// +kubebuilder:validation:Required
	Owner string `json:"owner"`

	// TODO: some control strategies
	//   1. optional or required
	//   2. dynamic or static
	//   3. granularity: component or some features

	// Specifies the component definition of components that the sidecar along with.
	//
	// This field is immutable.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	Selectors []string `json:"selectors"`

	// List of containers for the sidecar.
	//
	// Cannot be updated.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	Containers []corev1.Container `json:"containers"`

	// Defines variables which are needed by the sidecar.
	//
	// This field is immutable.
	//
	// +optional
	Vars []EnvVar `json:"vars,omitempty"`

	// Specifies the configuration file templates used by the Sidecar.
	//
	// This field is immutable.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	Configs []ComponentConfigSpec `json:"configs,omitempty"`

	// Specifies the scripts used by the Sidecar.
	//
	// This field is immutable.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	Scripts []ComponentTemplateSpec `json:"scripts,omitempty"`

	// TODO:
	//   1. services, volumes, service-refs, etc.
	//   2. how to share resources from main container if needed?
}

// SidecarDefinitionStatus defines the observed state of SidecarDefinition
type SidecarDefinitionStatus struct {
	// Refers to the most recent generation that has been observed for the SidecarDefinition.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the current status of the SidecarDefinition. Valid values include ``, `Available`, and `Unavailable`.
	// When the status is `Available`, the SidecarDefinition is ready and can be utilized by related objects.
	//
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Provides additional information about the current phase.
	//
	// +optional
	Message string `json:"message,omitempty"`

	Owners string `json:"owners,omitempty"`

	Selectors string `json:"selectors,omitempty"`
}
