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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},shortName={pv,pview}
// +kubebuilder:printcolumn:name="PARAMETER",type="string",JSONPath=".spec.parameterRef.name",description="referenced parameter name"
// +kubebuilder:printcolumn:name="TEMPLATE",type="string",JSONPath=".spec.templateName",description="config template name"
// +kubebuilder:printcolumn:name="FILE",type="string",JSONPath=".spec.fileName",description="config file name"
// +kubebuilder:printcolumn:name="MODE",type="string",JSONPath=".spec.mode",description="view mode"
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ParameterView is the Schema for the parameterviews API.
type ParameterView struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ParameterViewSpec   `json:"spec,omitempty"`
	Status ParameterViewStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ParameterViewList contains a list of ParameterView.
type ParameterViewList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ParameterView `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ParameterView{}, &ParameterViewList{})
}

// ParameterViewSpec defines the desired state of ParameterView.
type ParameterViewSpec struct {
	// ParameterRef identifies the ComponentParameter edited through this view.
	//
	// +kubebuilder:validation:Required
	ParameterRef corev1.LocalObjectReference `json:"parameterRef"`

	// TemplateName identifies the config template inside the referenced ComponentParameter.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	TemplateName string `json:"templateName"`

	// FileName identifies the config file inside the selected template.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	FileName string `json:"fileName"`

	// FileFormat identifies the file format used by the selected config file.
	// When omitted, the controller resolves it from the referenced template metadata.
	//
	// +optional
	FileFormat CfgFileFormat `json:"fileFormat,omitempty"`

	// Mode controls whether edits are allowed to be translated back into ComponentParameter patches.
	//
	// +kubebuilder:default="ReadWrite"
	// +optional
	Mode ParameterViewMode `json:"mode,omitempty"`

	// SourceGeneration captures the ComponentParameter generation used to build the current view.
	// Controllers should reject stale writes when this value no longer matches the source object.
	// It is typically populated and refreshed by the controller.
	//
	// +optional
	SourceGeneration int64 `json:"sourceGeneration,omitempty"`

	// ContentHash optionally captures the effective source content used to build the current view.
	// It is typically populated and refreshed by the controller.
	//
	// +optional
	ContentHash string `json:"contentHash,omitempty"`

	// Content is the document for the selected file.
	//
	// +optional
	Content ParameterViewContent `json:"content,omitempty"`
}

// ParameterViewStatus defines the observed state of ParameterView.
type ParameterViewStatus struct {
	// ObservedGeneration is the most recent ParameterView generation observed by the controller.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase describes the current state of this view.
	//
	// +optional
	Phase ParameterViewPhase `json:"phase,omitempty"`

	// Message provides a human-readable summary for the current state.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions captures detailed reconciliation and validation status.
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ParameterViewMode defines whether a ParameterView can be edited.
// +enum
// +kubebuilder:validation:Enum={ReadOnly,ReadWrite}
type ParameterViewMode string

const (
	ParameterViewReadOnlyMode  ParameterViewMode = "ReadOnly"
	ParameterViewReadWriteMode ParameterViewMode = "ReadWrite"
)

// ParameterViewPhase defines the lifecycle state of a ParameterView.
// +enum
// +kubebuilder:validation:Enum={Pending,Ready,Conflict,Invalid,Applying}
type ParameterViewPhase string

const (
	ParameterViewPendingPhase  ParameterViewPhase = "Pending"
	ParameterViewReadyPhase    ParameterViewPhase = "Ready"
	ParameterViewConflictPhase ParameterViewPhase = "Conflict"
	ParameterViewInvalidPhase  ParameterViewPhase = "Invalid"
	ParameterViewApplyingPhase ParameterViewPhase = "Applying"
)

// ParameterViewContent describes the editable document shown to users.
type ParameterViewContent struct {
	// Type declares how Text should be parsed and rendered.
	//
	// +kubebuilder:default="PlainText"
	// +optional
	Type ParameterViewContentType `json:"type,omitempty"`

	// Text is the user-facing document content.
	//
	// When type is PlainText, Text is the raw editable file content.
	// When type is MarkerLine, each line is expected to begin with a marker such as
	// "[D]", "[S]", "[I]", or "[U]". The controller is responsible for parsing the
	// view document into raw file content before translating edits into ComponentParameter
	// patches.
	//
	// +optional
	Text string `json:"text,omitempty"`
}

// ParameterViewContentType defines how ParameterView content should be interpreted.
// +enum
// +kubebuilder:validation:Enum={PlainText,MarkerLine}
type ParameterViewContentType string

const (
	PlainTextParameterViewContentType  ParameterViewContentType = "PlainText"
	MarkerLineParameterViewContentType ParameterViewContentType = "MarkerLine"
)

// ParameterViewContentMarker indicates how a marker line should be interpreted.
//
// D means a dynamic parameter managed by ParametersDefinition.
// S means a static parameter managed by ParametersDefinition.
// I means an immutable parameter that is rendered but not editable.
// U means unmanaged content such as comments, section headers, blank lines, or
// file fragments that are not defined by the current ParametersDefinition.
// +enum
// +kubebuilder:validation:Enum={D,S,I,U}
type ParameterViewContentMarker string

const (
	DynamicParameterViewContentMarker   ParameterViewContentMarker = "D"
	StaticParameterViewContentMarker    ParameterViewContentMarker = "S"
	ImmutableParameterViewContentMarker ParameterViewContentMarker = "I"
	UnmanagedParameterViewContentMarker ParameterViewContentMarker = "U"
)
