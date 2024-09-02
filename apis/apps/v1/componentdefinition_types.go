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
// +kubebuilder:storageversion
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=cmpd
// +kubebuilder:printcolumn:name="SERVICE",type="string",JSONPath=".spec.serviceKind",description="service"
// +kubebuilder:printcolumn:name="SERVICE-VERSION",type="string",JSONPath=".spec.serviceVersion",description="service version"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ComponentDefinition serves as a reusable blueprint for creating Components,
// encapsulating essential static settings such as Component description,
// Pod templates, configuration file templates, scripts, parameter lists,
// injected environment variables and their sources, and event handlers.
// ComponentDefinition works in conjunction with dynamic settings from the ClusterComponentSpec,
// to instantiate Components during Cluster creation.
//
// Key aspects that can be defined in a ComponentDefinition include:
//
// - PodSpec template: Specifies the PodSpec template used by the Component.
// - Configuration templates: Specify the configuration file templates required by the Component.
// - Scripts: Provide the necessary scripts for Component management and operations.
// - Storage volumes: Specify the storage volumes and their configurations for the Component.
// - Pod roles: Outlines various roles of Pods within the Component along with their capabilities.
// - Exposed Kubernetes Services: Specify the Services that need to be exposed by the Component.
// - System accounts: Define the system accounts required for the Component.
// - Monitoring and logging: Configure the exporter and logging settings for the Component.
//
// ComponentDefinitions also enable defining reactive behaviors of the Component in response to events,
// such as member join/leave, Component addition/deletion, role changes, switch over, and more.
// This allows for automatic event handling, thus encapsulating complex behaviors within the Component.
//
// Referencing a ComponentDefinition when creating individual Components ensures inheritance of predefined configurations,
// promoting reusability and consistency across different deployments and cluster topologies.
type ComponentDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentDefinitionSpec   `json:"spec,omitempty"`
	Status ComponentDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ComponentDefinitionList contains a list of ComponentDefinition
type ComponentDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentDefinition{}, &ComponentDefinitionList{})
}

// ComponentDefinitionSpec defines the desired state of ComponentDefinition
type ComponentDefinitionSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of ComponentDefinition. Edit componentdefinition_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// ComponentDefinitionStatus defines the observed state of ComponentDefinition
type ComponentDefinitionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}
