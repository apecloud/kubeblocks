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

// ClusterTopologySpec defines the desired state of ClusterTopology
type ClusterTopologySpec struct {
	// TODO: components the definition/topology defined

	// Topologies represents the different topologies within the cluster.
	// +required
	Topologies []ClusterTopologyDefinition `json:"topologies"`
}

// ClusterTopologyDefinition represents the configuration for a specific cluster topology.
type ClusterTopologyDefinition struct {
	// Name is the unique identifier for the cluster topology.
	// Cannot be updated.
	// +required
	Name string `json:"name"`

	// Components specifies the components in the topology.
	// +required
	Components []ClusterTopologyComponent `json:"components"`

	// Default indicates whether this topology is the default configuration.
	// + optional
	Default bool `json:"default,omitempty"`

	// Orders defines the order of components within the topology.
	// +optional
	Orders *ClusterTopologyComponentOrder `json:"orders,omitempty"`

	// RequiredVersion specifies the minimum version required for this topology.
	// +optional
	RequiredVersion string `json:"requiredVersion,omitempty"`

	//// services defines the default cluster services for this topology.
	//// +kubebuilder:pruning:PreserveUnknownFields
	//// +optional
	// Services []ClusterService `json:"services,omitempty"`

	// TODO: resource allocation strategy.
}

// ClusterTopologyComponent defines a component within a cluster topology.
type ClusterTopologyComponent struct {
	// Name defines the name of the component.
	// This name is also part of Service DNS name, following IANA Service Naming rules.
	// Cannot be updated.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=16
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// CompDef specifies the component definition to use, either as a specific name or a name prefix.
	// During instance provisioning, the system searches for matching component definitions based on the specified criteria.
	// The search order for component definitions is as follows:
	//   1. Prioritize component definitions within the current Addon.
	//   2. Consider component definitions already installed in the Kubernetes cluster.
	//   3. Optionally search for component definitions in the Addon repository if specified.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	CompDef string `json:"compDef"`

	// ServiceVersion specifies the version associated with the referenced component definition.
	// This field assists in determining the appropriate version of the component definition, considering multiple available definitions.
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceVersion string `json:"serviceVersion,omitempty"`

	// ServiceRefs define the service references for the component.
	// +optional
	ServiceRefs []ServiceRef `json:"serviceRefs,omitempty"`

	// Replicas specifies the default replicas for the component in this topology.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
}

// ClusterTopologyComponentOrder defines the order for components within a topology.
type ClusterTopologyComponentOrder struct {
	// StartupOrder defines the order in which components should be started in the cluster.
	// Components with the same order can be listed together, separated by commas.
	// +optional
	StartupOrder []string `json:"startupOrder,omitempty"`

	// ShutdownOrder defines the order in which components should be shut down in the cluster.
	// Components with the same order can be listed together, separated by commas.
	// +optional
	ShutdownOrder []string `json:"shutdownOrder,omitempty"`

	// UpdateOrder defines the order in which components should be updated in the cluster.
	// Components with the same order can be listed together, separated by commas.
	// +optional
	UpdateOrder []string `json:"updateOrder,omitempty"`

	// TODO: upgrade order?
}

// ClusterTopologyStatus defines the observed state of ClusterTopology
type ClusterTopologyStatus struct {
	// ObservedGeneration is the most recent generation observed for this ClusterTopology.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase valid values are ``, `Available`, 'Unavailable`.
	// Available is ClusterTopology become available, and can be used to create clusters.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Extra message for current phase.
	// +optional
	Message string `json:"message,omitempty"`

	// +optional
	Topologies string `json:"topologies,omitempty"`

	// +optional
	ExternalServices string `json:"externalServices,omitempty"`
}

// TODO:
//  1. how to display the aggregated topology and its service references line by line?
//  2. the services and versions supported

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=ct
// +kubebuilder:printcolumn:name="Topologies",type="string",JSONPath=".status.topologies",description="topologies"
// +kubebuilder:printcolumn:name="External-Service",type="string",JSONPath=".status.externalServices",description="external service referenced"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ClusterTopology is the Schema for the clustertopologies API
type ClusterTopology struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterTopologySpec   `json:"spec,omitempty"`
	Status ClusterTopologyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterTopologyList contains a list of ClusterTopology
type ClusterTopologyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterTopology `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterTopology{}, &ClusterTopologyList{})
}
