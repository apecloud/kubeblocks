/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=cd
// +kubebuilder:printcolumn:name="Topologies",type="string",JSONPath=".status.topologies",description="topologies"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ClusterDefinition defines the topology for databases or storage systems,
// offering a variety of topological configurations to meet diverse deployment needs and scenarios.
//
// It includes a list of Components, each linked to a ComponentDefinition, which enhances reusability and reduce redundancy.
// For example, widely used components such as etcd and Zookeeper can be defined once and reused across multiple ClusterDefinitions,
// simplifying the setup of new systems.
//
// Additionally, ClusterDefinition also specifies the sequence of startup, upgrade, and shutdown for Components,
// ensuring a controlled and predictable management of component lifecycles.
type ClusterDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterDefinitionSpec   `json:"spec,omitempty"`
	Status ClusterDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterDefinitionList contains a list of ClusterDefinition
type ClusterDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterDefinition{}, &ClusterDefinitionList{})
}

// ClusterDefinitionSpec defines the desired state of ClusterDefinition.
type ClusterDefinitionSpec struct {
	// Topologies defines all possible topologies within the cluster.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	// +optional
	Topologies []ClusterTopology `json:"topologies,omitempty"`
}

// ClusterDefinitionStatus defines the observed state of ClusterDefinition
type ClusterDefinitionStatus struct {
	// Represents the most recent generation observed for this ClusterDefinition.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Specifies the current phase of the ClusterDefinition. Valid values are `empty`, `Available`, `Unavailable`.
	// When `Available`, the ClusterDefinition is ready and can be referenced by related objects.
	Phase Phase `json:"phase,omitempty"`

	// Provides additional information about the current phase.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// Topologies this ClusterDefinition supported.
	//
	// +optional
	Topologies string `json:"topologies,omitempty"`
}

// ClusterTopology represents the definition for a specific cluster topology.
type ClusterTopology struct {
	// Name is the unique identifier for the cluster topology.
	// Cannot be updated.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	Name string `json:"name"`

	// Components specifies the components in the topology.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	Components []ClusterTopologyComponent `json:"components"`

	// Specifies the sequence in which components within a cluster topology are
	// started, stopped, and upgraded.
	// This ordering is crucial for maintaining the correct dependencies and operational flow across components.
	//
	// +optional
	Orders *ClusterTopologyOrders `json:"orders,omitempty"`

	// Default indicates whether this topology serves as the default configuration.
	// When set to true, this topology is automatically used unless another is explicitly specified.
	//
	// +optional
	Default bool `json:"default,omitempty"`
}

// ClusterTopologyComponent defines a Component within a ClusterTopology.
type ClusterTopologyComponent struct {
	// Defines the unique identifier of the component within the cluster topology.
	// It follows IANA Service naming rules and is used as part of the Service's DNS name.
	// The name must start with a lowercase letter, can contain lowercase letters, numbers,
	// and hyphens, and must end with a lowercase letter or number.
	//
	// Cannot be updated once set.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=16
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Specifies the name or prefix of the ComponentDefinition custom resource(CR) that
	// defines the Component's characteristics and behavior.
	//
	// When a prefix is used, the system selects the ComponentDefinition CR with the latest version that matches the prefix.
	// This approach allows:
	//
	// 1. Precise selection by providing the exact name of a ComponentDefinition CR.
	// 2. Flexible and automatic selection of the most up-to-date ComponentDefinition CR by specifying a prefix.
	//
	// Once set, this field cannot be updated.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=64
	CompDef string `json:"compDef"`
}

// ClusterTopologyOrders manages the lifecycle of components within a cluster by defining their provisioning,
// terminating, and updating sequences.
// It organizes components into stages or groups, where each group indicates a set of components
// that can be managed concurrently.
// These groups are processed sequentially, allowing precise control based on component dependencies and requirements.
type ClusterTopologyOrders struct {
	// Specifies the order for creating and initializing components.
	// This is designed for components that depend on one another. Components without dependencies can be grouped together.
	//
	// Components that can be provisioned independently or have no dependencies can be listed together in the same stage,
	// separated by commas.
	//
	// +optional
	Provision []string `json:"provision,omitempty"`

	// Outlines the order for stopping and deleting components.
	// This sequence is designed for components that require a graceful shutdown or have interdependencies.
	//
	// Components that can be terminated independently or have no dependencies can be listed together in the same stage,
	// separated by commas.
	//
	// +optional
	Terminate []string `json:"terminate,omitempty"`

	// Update determines the order for updating components' specifications, such as image upgrades or resource scaling.
	// This sequence is designed for components that have dependencies or require specific update procedures.
	//
	// Components that can be updated independently or have no dependencies can be listed together in the same stage,
	// separated by commas.
	//
	// +optional
	Update []string `json:"update,omitempty"`
}
