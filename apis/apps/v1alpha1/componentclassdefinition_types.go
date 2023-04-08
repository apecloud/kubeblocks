/*
Copyright ApeCloud, Inc.

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

// ComponentClassDefinitionSpec defines the desired state of ComponentClassDefinition
type ComponentClassDefinitionSpec struct {
	// The class definition groups
	// +optional
	Groups []ComponentClassGroup `json:"groups,omitempty"`
}

type ComponentClassGroup struct {
	// ClassConstraintRef reference to the class constraint object
	// +required
	ClassConstraintRef string `json:"classConstraintRef,omitempty"`

	// Template for the class definition
	// +optional
	Template string `json:"template,omitempty"`

	// Template variables
	// +optional
	Vars []string `json:"vars,omitempty"`

	// Class Series
	// +optional
	Series []ComponentClassSeries `json:"series,omitempty"`
}

type ComponentClassSeries struct {
	// class name generator, following golang template
	// +optional
	Name string `json:"name,omitempty"`

	// +optional
	Classes []ComponentClass `json:"classes,omitempty"`
}

type ComponentClass struct {
	// class name
	// +optional
	Name string `json:"name,omitempty"`

	// class variables
	// +optional
	Args []string `json:"args,omitempty"`

	// the CPU of the class
	// +optional
	CPU string `json:"cpu,omitempty"`

	// the memory of the class
	// +optional
	Memory string `json:"memory,omitempty"`

	// the storage of the class
	// +optional
	Storage []DiskDef `json:"storage,omitempty"`

	// the variants of the class in different clouds.
	// +optional
	Variants []ProviderComponentClassDef `json:"variants,omitempty"`
}

type ProviderComponentClassDef struct {
	// cloud provider name
	// +required
	Provider string `json:"provider,omitempty"`

	// cloud provider specific variables
	// +optional
	Args []string `json:"args,omitempty"`
}

type DiskDef struct {
	Name  string `json:"name,omitempty"`
	Size  string `json:"size,omitempty"`
	Class string `json:"class,omitempty"`
}

// ComponentClassDefinitionStatus defines the observed state of ComponentClassDefinition
type ComponentClassDefinitionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ComponentClassDefinition is the Schema for the componentclassdefinitions API
type ComponentClassDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentClassDefinitionSpec   `json:"spec,omitempty"`
	Status ComponentClassDefinitionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ComponentClassDefinitionList contains a list of ComponentClassDefinition
type ComponentClassDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentClassDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentClassDefinition{}, &ComponentClassDefinitionList{})
}
