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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReconciliationViewSpec defines the desired state of ReconciliationView
type ReconciliationViewSpec struct {
	// TargetObject specifies the target Cluster object.
	// Default is the Cluster object with same namespace and name as this ReconciliationView object.
	//
	// +optional
	TargetObject *ObjectReference `json:"targetObject,omitempty"`

	// Depth of the object tree.
	// Default is 1, means the top primary object only.
	//
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	Depth *int32 `json:"depth,omitempty"`

	// StateEvaluationExpression overrides the builtin default value.
	//
	// +optional
	StateEvaluationExpression *StateEvaluationExpression `json:"stateEvaluationExpression,omitempty"`

	// Locale specifies the locale to use when localizing the reconciliation view.
	//
	// +optional
	Locale *string `json:"locale,omitempty"`
}

// ReconciliationViewStatus defines the observed state of ReconciliationView
type ReconciliationViewStatus struct {
	// InitialObjectTree specifies the initial object tree.
	//
	InitialObjectTree *ObjectTreeNode `json:"initialObjectTree"`

	// DesiredObjectTree specifies the object tree if the spec.desiredSpec is applied.
	//
	// +optional
	DesiredObjectTree *ObjectTreeNode `json:"desiredObjectTree"`

	// CurrentObjectTree specifies the current object tree.
	// Ideally, CurrentObjectTree should be same as applying changes in View to InitialObjectTree.
	//
	CurrentObjectTree *ObjectTreeNode `json:"currentObjectTree"`

	// PlanSummary summarizes the desired state by comparing it to the initial state.
	//
	// +optional
	PlanSummary *PlanSummary `json:"planSummary"`

	// ViewSummary summarizes the current state by comparing it to the initial state.
	//
	ViewSummary ViewSummary `json:"viewSummary"`

	// Plan describes the detail reconciliation process when the current spec of the TargetObject is fully applied.
	//
	// +optional
	Plan []ObjectChange `json:"plan"`

	// View describes the detail reconciliation progress ongoing.
	//
	View []ObjectChange `json:"view"`
}

// ViewSummary defines a summary of reconciliation view.
type ViewSummary struct {
	// ObjectSummaries summarizes each object type.
	//
	ObjectSummaries []ObjectSummary `json:"objectSummaries"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

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
