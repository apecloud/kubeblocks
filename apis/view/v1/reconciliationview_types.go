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
	// ViewDefinition specifies the name of the ReconciliationViewDefinition.
	//
	ViewDefinition string `json:"viewDefinition"`

	// TargetObject specifies the target Cluster object.
	// Default is the Cluster object with same namespace and name as this ReconciliationView object.
	//
	// +optional
	TargetObject *ObjectReference `json:"targetObject,omitempty"`

	// WithReconciliationPlan specifies whether to show the reconciliation view with the plan.
	//
	// +optional
	WithReconciliationPlan bool `json:"withReconciliationPlan"`

	// StateEvaluationExpression overrides the value specified in ReconciliationViewDefinition.
	//
	// +optional
	StateEvaluationExpression *string `json:"stateEvaluationExpression,omitempty"`

	// Locale specifies the locale to use when localizing the reconciliation view.
	//
	// +optional
	Locale *string `json:"locale,omitempty"`
}

// ReconciliationViewStatus defines the observed state of ReconciliationView
type ReconciliationViewStatus struct {
	Plan *ReconciliationPlanStatus `json:"plan,omitempty"`

	Summary ViewSummary `json:"summary"`

	View ObjectReconciliationView `json:"view"`
}

type ViewSummary struct {
	ObjectSummaries []ObjectSummary `json:"objectSummaries"`
}

type ObjectReconciliationView struct {
	// ObjectReference specifies the Object this view described.
	//
	ObjectReference corev1.ObjectReference `json:"objectReference"`

	// Changes describes all changes related to this object, including the associated Events, in order.
	//
	Changes []ObjectChange `json:"changes"`

	// SecondaryObjectViews describes views of all the secondary objects of this object, if any.
	// The secondary objects are collected by the rules specified in ReconciliationViewDefinition.
	//
	SecondaryObjectViews []ObjectReconciliationView `json:"secondaryObjectViews,omitempty"`
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
