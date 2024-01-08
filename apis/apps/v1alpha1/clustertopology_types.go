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
	// Components specifies components in the cluster.
	// +required
	Components []ClusterTopologyComponent `json:"components"`

	// Orders defines the orders for components in the cluster.
	// +optional
	Orders *ClusterTopologyOrders `json:"orders,omitempty"`

	//// services defines the default cluster services for this topology.
	//// +kubebuilder:pruning:PreserveUnknownFields
	//// +optional
	// Services []ClusterService `json:"services,omitempty"`

	// TODO: resource allocation strategy.
}

type ClusterTopologyComponent struct {
	// Name defines the name of the component.
	// This name is also part of Service DNS name, following IANA Service Naming rules.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`

	// CompDef is the name of the component definition to use.
	// +kubebuilder:validation:Required
	CompDef string `json:"compDef"`

	// CompVersion is the component version associated with the specified component definition.
	// +optional
	CompVersion string `json:"compVersion"`

	// Replicas specifies the default replicas for the component in this topology.
	// +optional
	Replicas *int32 `json:"replicas"`

	// ServiceRefs define the service references for the component.
	// +optional
	ServiceRefs []ServiceRef `json:"serviceRefs,omitempty"`
}

// ClusterTopologyOrders defines the orders for components in the cluster.
type ClusterTopologyOrders struct {
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
}

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=ct
// +kubebuilder:printcolumn:name="COMPONENTS",type="string",JSONPath=".spec.components[*].name",description="components"
// +kubebuilder:printcolumn:name="EXTERNAL-REFERENCE",type="string",JSONPath=".spec.components[*].serviceRefs[*].name",description="external service reference"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

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
