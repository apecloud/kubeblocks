/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	// Provides a description of any abnormal status.
	// +optional
	Message string `json:"message,omitempty"`

	// Indicates the current status of the configuration item.
	//
	// Possible values include "Creating", "Init", "Running", "Pending", "Merged", "MergeFailed", "FailedAndPause",
	// "Upgrading", "Deleting", "FailedAndRetry", "Finished".
	//
	// +optional
	Phase ParameterPhase `json:"phase,omitempty"`

	// Represents the latest generation observed for this
	// ClusterDefinition. It corresponds to the ConfigConstraint's generation, which is
	// updated by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Records the status of a reconfiguring operation if `opsRequest.spec.type` equals to "Reconfiguring".
	// +optional
	ReconfiguringStatus []ComponentReconfiguringStatus `json:"componentReconfiguringStatus"`
}

type ComponentParametersSpec struct {

	// Specifies the name of the Component.
	// +kubebuilder:validation:Required
	ComponentName string `json:"componentName"`

	// Specifies the user-defined configuration template or parameters.
	//
	// +optional
	Parameters ComponentParameters `json:"parameters,omitempty"`

	// Specifies the user-defined configuration template.
	//
	// When provided, the `importTemplateRef` overrides the default configuration template
	// specified in `configSpec.templateRef`.
	// This allows users to customize the configuration template according to their specific requirements.
	//
	// +optional
	CustomTemplates map[string]ConfigTemplateExtension `json:"userConfigTemplates,omitempty"`
}

type ComponentReconfiguringStatus struct {
	// Specifies the name of the Component.
	// +kubebuilder:validation:Required
	ComponentName string `json:"componentName"`

	// Indicates the current status of the configuration item.
	//
	// Possible values include "Creating", "Init", "Running", "Pending", "Merged", "MergeFailed", "FailedAndPause",
	// "Upgrading", "Deleting", "FailedAndRetry", "Finished".
	//
	// +optional
	Phase ParameterPhase `json:"phase,omitempty"`

	// Describes the status of the component reconfiguring.
	// +kubebuilder:validation:Required
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ParameterStatus []ReconfiguringStatus `json:"parameterStatus,omitempty"`
}

type ReconfiguringStatus struct {
	ConfigTemplateItemDetailStatus `json:",inline"`

	// Contains the updated parameters.
	//
	// +optional
	UpdatedParameters map[string]ParametersInFile `json:"updatedParameters,omitempty"`

	// Specifies the user-defined configuration template.
	//
	// When provided, the `importTemplateRef` overrides the default configuration template
	// specified in `configSpec.templateRef`.
	// This allows users to customize the configuration template according to their specific requirements.
	//
	// +optional
	CustomTemplate *ConfigTemplateExtension `json:"userConfigTemplates,omitempty"`
}
