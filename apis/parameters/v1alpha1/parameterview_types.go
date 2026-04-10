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

	// Mode controls whether edits are allowed to be translated back into ComponentParameter patches.
	//
	// +kubebuilder:default="ReadWrite"
	// +optional
	Mode ParameterViewMode `json:"mode,omitempty"`

	// ResetToLatest requests the controller to discard the current draft in spec.content
	// and rebuild it from the latest observed effective content.
	//
	// This is a reset-style action rather than a git-style rebase:
	// 1. the current draft is dropped;
	// 2. spec.content is reconstructed from status.latest;
	// 3. status.base is advanced to the rebuilt content revision.
	//
	// +optional
	ResetToLatest bool `json:"resetToLatest,omitempty"`

	// Content is the current user-facing document for the selected file.
	//
	// Controllers treat Content as the user's current draft:
	// 1. on initialization, Content is populated from the effective content and
	//    status.base and status.latest point to the same revision;
	// 2. while status.base and status.latest remain equal, Content is based on the
	//    latest observed effective content;
	// 3. when status.latest advances and Content is still only a projection of
	//    status.base, the controller may auto-refresh Content and move status.base
	//    forward to status.latest;
	// 4. when Content has diverged into a real user draft, the controller preserves
	//    it and uses status.base as the draft base when replaying the draft onto
	//    status.latest or surfacing a conflict.
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

	// FileFormat identifies the file format used by the selected config file.
	// It is resolved and refreshed by the controller from template metadata.
	//
	// +optional
	FileFormat CfgFileFormat `json:"fileFormat,omitempty"`

	// Base records the effective content revision that spec.content is currently based on.
	// It is the draft base used to decide whether spec.content is still only a
	// projection of the source, or whether it has diverged into a real user draft.
	//
	// +optional
	Base ParameterViewRevision `json:"base,omitempty"`

	// Latest records the latest effective content revision observed by the controller.
	// When Base and Latest are equal, spec.content is based on the current latest
	// effective revision. When they differ, spec.content is either waiting to
	// auto-refresh because no real draft exists, or waiting for the controller to
	// replay or reject a preserved user draft against the newer latest revision.
	//
	// +optional
	Latest ParameterViewRevision `json:"latest,omitempty"`

	// Submissions records recent desired parameter submissions derived from this view.
	// Newer entries appear first. The controller may compact older entries, but it
	// keeps enough history for users to understand which changes were recently submitted.
	//
	// +optional
	Submissions []ParameterViewSubmission `json:"submissions,omitempty"`
}

// ParameterViewRevision identifies an effective content revision for a view.
type ParameterViewRevision struct {
	// Revision records the effective content revision associated with this view state.
	// The controller resolves it from the generated ConfigMap revision metadata when available.
	//
	// +optional
	Revision string `json:"revision,omitempty"`

	// ContentHash records the hash of the effective file content for this revision.
	//
	// +optional
	ContentHash string `json:"contentHash,omitempty"`
}

// ParameterViewSubmission records the most recent submission derived from a view draft.
type ParameterViewSubmission struct {
	// Revision records the effective content revision that the submission was based on.
	//
	// +optional
	Revision ParameterViewRevision `json:"revision,omitempty"`

	// SubmittedAt records when the submission entry was created or last refreshed.
	//
	// +optional
	SubmittedAt *metav1.Time `json:"submittedAt,omitempty"`

	// Assignments contains the desired simple parameter assignments submitted from the view.
	//
	// +optional
	Assignments map[string]*string `json:"assignments,omitempty"`

	// Result records the current observed outcome of this submission after it has
	// been handed off to the ComponentParameter controller.
	//
	// +optional
	Result ParameterViewSubmissionResult `json:"result,omitempty"`
}

// ParameterViewSubmissionResult records the current observed outcome of a submission.
type ParameterViewSubmissionResult struct {
	// Phase describes the current execution state of this submission.
	//
	// +optional
	Phase ParameterViewSubmissionPhase `json:"phase,omitempty"`

	// Reason is a stable, machine-friendly summary for the current submission result.
	//
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message provides a human-readable summary for the current submission result.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// UpdatedAt records when the result was last refreshed by the controller.
	//
	// +optional
	UpdatedAt *metav1.Time `json:"updatedAt,omitempty"`
}

// ParameterViewMode defines whether a ParameterView can be edited.
// +enum
// +kubebuilder:validation:Enum={ReadOnly,ReadWrite}
type ParameterViewMode string

const (
	ParameterViewReadOnlyMode  ParameterViewMode = "ReadOnly"
	ParameterViewReadWriteMode ParameterViewMode = "ReadWrite"
)

// ParameterViewPhase defines the current lifecycle state of a ParameterView.
// +enum
// +kubebuilder:validation:Enum={Synced,Conflict,Invalid,Applying}
type ParameterViewPhase string

const (
	// ParameterViewSyncedPhase means spec.content is aligned with the latest
	// effective content revision observed by the controller and there is no
	// pending submission, invalid draft, or unresolved conflict.
	ParameterViewSyncedPhase ParameterViewPhase = "Synced"

	// ParameterViewConflictPhase means the preserved draft in spec.content is
	// based on an older revision and the controller cannot automatically move it
	// forward to the latest observed effective content revision.
	ParameterViewConflictPhase ParameterViewPhase = "Conflict"

	// ParameterViewInvalidPhase means the current view or draft cannot be
	// processed, for example because the reference is invalid, the content type
	// is unsupported, or the draft cannot be translated into a valid desired
	// parameter patch.
	ParameterViewInvalidPhase ParameterViewPhase = "Invalid"

	// ParameterViewApplyingPhase means the controller has already derived and
	// submitted desired parameter updates from the current draft and is waiting
	// for the effective content to catch up.
	ParameterViewApplyingPhase ParameterViewPhase = "Applying"
)

// ParameterViewSubmissionPhase defines the observed execution state of a submission.
// +enum
// +kubebuilder:validation:Enum={Processing,Failed,Succeeded}
type ParameterViewSubmissionPhase string

const (
	// ParameterViewSubmissionProcessingPhase means the submission has already
	// been handed off to the ComponentParameter controller, and its final
	// execution outcome is not known yet.
	ParameterViewSubmissionProcessingPhase ParameterViewSubmissionPhase = "Processing"

	// ParameterViewSubmissionFailedPhase means the submission has reached a final
	// failure outcome in the ComponentParameter processing chain.
	ParameterViewSubmissionFailedPhase ParameterViewSubmissionPhase = "Failed"

	// ParameterViewSubmissionSucceededPhase means the submission has reached a
	// final successful outcome in the ComponentParameter processing chain.
	ParameterViewSubmissionSucceededPhase ParameterViewSubmissionPhase = "Succeeded"
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
