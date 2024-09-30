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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReconciliationViewSpec defines the desired state of ReconciliationView
type ReconciliationViewSpec struct {
	// TargetObject specifies the target Cluster object.
	// Default is the Cluster object with same namespace and name as this ReconciliationView object.
	//
	// +optional
	TargetObject *ObjectReference `json:"targetObject,omitempty"`

	// DryRun tells the Controller to simulate the reconciliation process with a new desired spec of the TargetObject.
	// And a reconciliation plan will be generated and described in the ReconciliationViewStatus.
	// The plan generation process will not impact the state of the TargetObject.
	//
	// +optional
	DryRun *DryRun `json:"dryRun,omitempty"`

	// StateEvaluationExpression specifies the state evaluation expression used during reconciliation progress observation.
	// The whole reconciliation process from the creation of the TargetObject to the deletion of it
	// is separated into several reconciliation cycles.
	// The StateEvaluationExpression is applied to the TargetObject,
	// and an evaluation result of true indicates the end of a reconciliation cycle.
	// StateEvaluationExpression overrides the builtin default value.
	//
	// +optional
	StateEvaluationExpression *StateEvaluationExpression `json:"stateEvaluationExpression,omitempty"`

	// Depth of the object tree.
	// Default is 0, means all the primary object and secondary objects.
	//
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	Depth *int32 `json:"depth,omitempty"`

	// Locale specifies the locale to use when localizing the reconciliation view.
	//
	// +optional
	Locale *string `json:"locale,omitempty"`
}

// ReconciliationViewStatus defines the observed state of ReconciliationView
type ReconciliationViewStatus struct {
	// DryRunResult specifies the dry-run result.
	//
	// +optional
	DryRunResult *DryRunResult `json:"dryRunResult,omitempty"`

	// InitialObjectTree specifies the initial object tree when the latest reconciliation cycle started.
	//
	InitialObjectTree *ObjectTreeNode `json:"initialObjectTree"`

	// CurrentState is the current state of the latest reconciliation cycle,
	// that is the reconciliation process from the end of last reconciliation cycle until now.
	//
	CurrentState ReconciliationCycleState `json:"currentState"`

	// DesiredState is the desired state of the latest reconciliation cycle.
	//
	// +optional
	DesiredState *ReconciliationCycleState `json:"desiredState,omitempty"`
}

// ObjectReference defines a reference to an object.
type ObjectReference struct {
	// Namespace of the referent.
	// Default is same as the ReconciliationView object.
	//
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the referent.
	// Default is same as the ReconciliationView object.
	//
	// +optional
	Name string `json:"name,omitempty"`
}

type DryRun struct {
	// DesiredSpec specifies the desired spec of the TargetObject.
	// The desired spec will be merged into the current spec by a strategic merge patch way to build the final spec,
	// and the reconciliation plan will be calculated by comparing the current spec to the final spec.
	// DesiredSpec should be a valid YAML string.
	//
	DesiredSpec string `json:"desiredSpec"`
}

// StateEvaluationExpression defines an object state evaluation expression.
// Currently supported types:
// CEL - Common Expression Language (https://cel.dev/).
type StateEvaluationExpression struct {
	// CELExpression specifies to use CEL to evaluation the object state.
	// The root object used in the expression is the primary object.
	//
	// +optional
	CELExpression *CELExpression `json:"celExpression,omitempty"`
}

// CELExpression defines a CEL expression.
type CELExpression struct {
	// Expression specifies the CEL expression.
	//
	Expression string `json:"expression"`
}

// DryRunResult defines a dry-run result.
type DryRunResult struct {
	// Phase specifies the current phase of the plan generation process.
	// Succeed - the plan is calculated successfully.
	// Failed - the plan can't be generated for some reason described in Reason.
	//
	// +kubebuilder:validation:Enum={Succeed,Failed}
	Phase DryRunPhase `json:"phase,omitempty"`

	// Reason specifies the reason when the Phase is Failed.
	//
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message specifies a description of the failure reason.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// DesiredSpecRevision specifies the revision of the DesiredSpec.
	//
	DesiredSpecRevision string `json:"desiredSpecRevision"`

	// ObservedTargetGeneration specifies the observed generation of the TargetObject.
	//
	ObservedTargetGeneration int64 `json:"observedTargetGeneration"`

	// SpecDiff describes the diff between the current spec and the final spec.
	// The whole spec struct will be compared and an example SpecDiff looks like:
	// {
	//		Affinity: {
	//    		PodAntiAffinity: "Preferred",
	//    		Tenancy: "SharedNode",
	//		},
	//  	ComponentSpecs: {
	//			{
	//  			ComponentDef: "postgresql",
	//    			Name: "postgresql",
	// -    		Replicas: 2,
	// +    		Replicas: 3,
	//    			Resources:
	//				{
	//      			Limits:
	//					{
	// -        			CPU: 500m,
	// +        			CPU: 800m,
	// -       				Memory: 512Mi,
	// +       				Memory: 768Mi,
	//					},
	//      			Requests:
	//					{
	// -      				CPU: 500m,
	// +       				CPU: 800m,
	// -      				Memory: 512Mi,
	// +       				Memory: 768Mi,
	//					},
	//				},
	//			},
	//		},
	// }
	//
	SpecDiff string `json:"specDiff"`

	// Plan describes the detail reconciliation process if the DesiredSpec is applied.
	//
	Plan ReconciliationCycleState `json:"plan"`
}

type DryRunPhase string

const (
	DryRunSucceedPhase DryRunPhase = "Succeed"
	DryRunFailedPhase  DryRunPhase = "Failed"
)

// ObjectTreeNode defines an object tree of the KubeBlocks Cluster.
type ObjectTreeNode struct {
	// Primary specifies reference of the primary object.
	//
	Primary corev1.ObjectReference `json:"primary"`

	// Secondaries describes all the secondary objects of this object, if any.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	// +optional
	Secondaries []*ObjectTreeNode `json:"secondaries,omitempty"`
}

// ObjectSummary defines the total and change of an object.
type ObjectSummary struct {
	// ObjectType of the object.
	//
	ObjectType ObjectType `json:"objectType"`

	// Total number of the object of type defined by ObjectType.
	//
	Total int32 `json:"total"`

	// ChangeSummary summarizes the change by comparing the final state to the current state of this type.
	// Nil means no change.
	//
	// +optional
	ChangeSummary *ObjectChangeSummary `json:"changeSummary,omitempty"`
}

// ObjectType defines an object type.
type ObjectType struct {
	// APIVersion of the type.
	//
	APIVersion string `json:"apiVersion"`

	// Kind of the type.
	//
	Kind string `json:"kind"`
}

// ObjectChangeSummary defines changes of an object.
type ObjectChangeSummary struct {
	// Added specifies the number of object will be added.
	//
	// +optional
	Added *int32 `json:"added,omitempty"`

	// Updated specifies the number of object will be updated.
	//
	// +optional
	Updated *int32 `json:"updated,omitempty"`

	// Deleted specifies the number of object will be deleted.
	//
	// +optional
	Deleted *int32 `json:"deleted,omitempty"`
}

// ObjectChange defines a detailed change of an object.
type ObjectChange struct {
	// ObjectReference specifies the Object this change described.
	//
	ObjectReference corev1.ObjectReference `json:"objectReference"`

	// ChangeType specifies the change type.
	// Event - specifies that this is a Kubernetes Event.
	// Creation - specifies that this is an object creation.
	// Update - specifies that this is an object update.
	// Deletion - specifies that this is an object deletion.
	//
	// +kubebuilder:validation:Enum={Event, Creation, Update, Deletion}
	ChangeType ObjectChangeType `json:"changeType"`

	// EventAttributes specifies the attributes of the event when ChangeType is Event.
	//
	// +optional
	EventAttributes *EventAttributes `json:"eventAttributes,omitempty"`

	// Revision specifies the revision of the object after this change.
	// Revision can be compared globally between all ObjectChanges of all Objects, to build a total order object change sequence.
	//
	Revision int64 `json:"revision"`

	// Timestamp is a timestamp representing the ReconciliationView Controller time when this change occurred.
	// It is not guaranteed to be set in happens-before order across separate changes.
	// It is represented in RFC3339 form and is in UTC.
	//
	// +optional
	Timestamp *metav1.Time `json:"timestamp,omitempty"`

	// Description describes the change in a user-friendly way.
	//
	Description string `json:"description"`

	// LocalDescription is the localized version of Description by using the Locale specified in `spec.locale`.
	// Empty if the `spec.locale` is not specified.
	//
	LocalDescription *string `json:"localDescription,omitempty"`
}

type ObjectChangeType string

const (
	ObjectCreationType ObjectChangeType = "Creation"
	ObjectUpdateType   ObjectChangeType = "Update"
	ObjectDeletionType ObjectChangeType = "Deletion"
	EventType          ObjectChangeType = "Event"
)

// EventAttributes defines attributes of the Event.
type EventAttributes struct {
	// Type of the Event.
	//
	Type string `json:"type"`

	// Reason of the Event.
	//
	Reason string `json:"reason"`
}

// ReconciliationCycleState defines the state of reconciliation cycle.
type ReconciliationCycleState struct {
	// Summary summarizes the ObjectTree and Changes.
	//
	Summary ObjectTreeDiffSummary `json:"summary"`

	// ObjectTree specifies the current object tree of the reconciliation cycle.
	// Ideally, ObjectTree should be same as applying Changes to InitialObjectTree.
	//
	ObjectTree *ObjectTreeNode `json:"objectTree"`

	// Changes describes the detail reconciliation progress.
	//
	Changes []ObjectChange `json:"changes"`
}

// ObjectTreeDiffSummary defines a summary of the diff of two object tree.
type ObjectTreeDiffSummary struct {
	// ObjectSummaries summarizes each object type.
	//
	ObjectSummaries []ObjectSummary `json:"objectSummaries"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all},shortName=view
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="TARGET_NS",type="string",JSONPath=".spec.targetObject.namespace",description="Target Object Namespace"
// +kubebuilder:printcolumn:name="TARGET_NAME",type="string",JSONPath=".spec.targetObject.name",description="Target Object Name"
// +kubebuilder:printcolumn:name="API_VERSION",type="string",JSONPath=".status.currentState.changes[-1].objectReference.apiVersion",description="Latest Changed Object API Version"
// +kubebuilder:printcolumn:name="KIND",type="string",JSONPath=".status.currentState.changes[-1].objectReference.kind",description="Latest Changed Object Kind"
// +kubebuilder:printcolumn:name="NAMESPACE",type="string",JSONPath=".status.currentState.changes[-1].objectReference.namespace",description="Latest Changed Object Namespace"
// +kubebuilder:printcolumn:name="NAME",type="string",JSONPath=".status.currentState.changes[-1].objectReference.name",description="Latest Changed Object Name"
// +kubebuilder:printcolumn:name="CHANGE",type="string",JSONPath=".status.currentState.changes[-1].description",description="Latest Change Description"

// ReconciliationView is the Schema for the reconciliationviews API
type ReconciliationView struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReconciliationViewSpec   `json:"spec,omitempty"`
	Status ReconciliationViewStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ReconciliationViewList contains a list of ReconciliationView
type ReconciliationViewList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReconciliationView `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReconciliationView{}, &ReconciliationViewList{})
}
