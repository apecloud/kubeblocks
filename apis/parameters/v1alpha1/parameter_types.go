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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks}
// +kubebuilder:printcolumn:name="CLUSTER",type="string",JSONPath=".spec.clusterName",description="cluster name"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="config status phase."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Parameter is the Schema for the parameters API
type Parameter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ParameterSpec   `json:"spec,omitempty"`
	Status ParameterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ParameterList contains a list of Parameter
type ParameterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Parameter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Parameter{}, &ParameterList{})
}

// ParameterSpec defines the desired state of Parameter
type ParameterSpec struct {
	// Specifies the name of the Cluster resource that this operation is targeting.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.clusterName"
	ClusterName string `json:"clusterName,omitempty"`

	// Lists ComponentParametersSpec objects, each specifying a Component and its parameters and template updates.
	//
	// +kubebuilder:validation:Required
	ComponentParameters []ComponentParametersSpec `json:"componentParameters"`
}

// ParameterStatus defines the observed state of Parameter
type ParameterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// TODO:
}

type ComponentParametersSpec struct {

	// Specifies the name of the Component.
	// +kubebuilder:validation:Required
	ComponentName string `json:"componentName"`

	// Specifies the user-defined configuration template or parameters.
	//
	// +optional
	ComponentParameters ComponentParameters `json:"parameters,omitempty"`

	// Specifies the user-defined configuration template.
	//
	// When provided, the `importTemplateRef` overrides the default configuration template
	// specified in `configSpec.templateRef`.
	// This allows users to customize the configuration template according to their specific requirements.
	//
	// +optional
	CustomTemplates *appsv1.ConfigTemplateExtension `json:"userConfigTemplates,omitempty"`
}
