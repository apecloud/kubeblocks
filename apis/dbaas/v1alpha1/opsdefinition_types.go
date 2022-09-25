/*
Copyright 2022.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OpsDefinitionSpec defines the desired state of OpsDefinition
type OpsDefinitionSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ClusterDefinitionRef reference clusterDefinition resource
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ClusterDefinitionRef string `json:"clusterDefinitionRef"`

	// Type define the type of operation job
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum={Upgrade,VerticalScaling,VolumeExpansion,HorizontalScaling,Restart}
	Type OpsType `json:"type"`

	// Strategy the execution strategy of the operation,exclude operation type of Upgrade
	// +optional
	Strategy *Strategy `json:"strategies,omitempty"`
}

// OpsDefinitionStatus defines the observed state of OpsDefinition
type OpsDefinitionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum={Available,UnAvailable}
	Phase Phase `json:"phase"`

	// Message record the OpsDefinition details message in current phase
	// +optional
	Message string `json:"message,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	ClusterDefinitionStatusGeneration `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories={dbaas},scope=Cluster,shortName=od
//+kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="OpsDefinition Status."
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// OpsDefinition is the Schema for the opsdefinitions API
type OpsDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpsDefinitionSpec   `json:"spec,omitempty"`
	Status OpsDefinitionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpsDefinitionList contains a list of OpsDefinition
type OpsDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpsDefinition `json:"items"`
}

// Strategy Defines relevant policies for operations
// such as component execution sequence and failover policy
type Strategy struct {
	// Components is an array of component types, which defines the components that can be used by the operation .
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Components []OpsDefComponent `json:"components"`
}

type OpsDefComponent struct {
	// component type
	// +kubebuilder:validation:Required
	Type string `json:"type"`
}

func init() {
	SchemeBuilder.Register(&OpsDefinition{}, &OpsDefinitionList{})
}
