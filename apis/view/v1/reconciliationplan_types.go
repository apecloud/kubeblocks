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

// ReconciliationPlanSpec defines the desired state of ReconciliationPlan
type ReconciliationPlanSpec struct {
	// ViewDefinition specifies the name of the ReconciliationViewDefinition.
	//
	ViewDefinition string `json:"viewDefinition"`

	// TargetObject specifies the target Cluster object.
	// Default is the Cluster object with same namespace and name as this ReconciliationPlan object.
	//
	// +optional
	TargetObject *ObjectReference `json:"targetObject,omitempty"`

	// DesiredSpec specifies desired spec of the Cluster object.
	// The desired spec will be merged into the current spec in the same way as `kubectl apply` to build the final spec,
	// and the reconciliation plan will be calculated by comparing the current spec to the final spec.
	// DesiredSpec should be a valid YAML string.
	//
	DesiredSpec string `json:"desiredSpec"`

	// Depth of the object tree.
	// Default is 1, means the top primary object only.
	//
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	Depth *int32 `json:"depth,omitempty"`

	// StateEvaluationExpression overrides the value specified in ReconciliationViewDefinition.
	//
	// +optional
	StateEvaluationExpression *StateEvaluationExpression `json:"stateEvaluationExpression,omitempty"`

	// Locale specifies the locale to use when localizing the reconciliation plan.
	//
	// +optional
	Locale *string `json:"locale,omitempty"`
}

// ReconciliationPlanStatus defines the observed state of ReconciliationPlan
type ReconciliationPlanStatus struct {
	// ObservedPlanGeneration specifies the observed generation of the ReconciliationPlan object.
	//
	ObservedPlanGeneration int64 `json:"observedPlanGeneration"`

	// ObservedTargetGeneration specifies the observed generation of the target object.
	//
	ObservedTargetGeneration int64 `json:"observedTargetGeneration"`

	// Phase specifies the current phase of the plan calculation process.
	// Succeed - the plan is calculated successfully.
	// Failed - the plan can't be calculated for some reason described in Reason.
	//
	// +optional
	// +kubebuilder:validation:Enum={Succeed,Failed}
	Phase string `json:"phase,omitempty"`

	// Reason specifies the detail reason when the Phase is Failed.
	//
	// +optional
	Reason string `json:"reason,omitempty"`

	// CurrentObjectTree specifies the current object tree.
	//
	CurrentObjectTree ObjectTreeNode `json:"currentObjectTree"`

	// DesiredObjectTree specifies the object tree if the spec.desiredSpec is applied.
	//
	DesiredObjectTree ObjectTreeNode `json:"desiredObjectTree"`

	// Summary summarizes the final state to give an overview of what will happen if the reconciliation plan is executed.
	//
	Summary PlanSummary `json:"summary"`

	// Plan describes the detail reconciliation process if the DesiredSpec specified in ReconciliationPlanSpec is applied.
	//
	Plan []ObjectChange `json:"plan"`
}

// PlanSummary defines a summary of the reconciliation plan.
type PlanSummary struct {
	// SpecChange describes the change between the current spec and the final spec.
	// The whole spec struct will be compared and an example SpecChange looks like:
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
	SpecChange string `json:"specChange"`

	// ObjectSummaries summarizes each object type.
	//
	ObjectSummaries []ObjectSummary `json:"objectSummaries"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ReconciliationPlan is the Schema for the reconciliationplans API
type ReconciliationPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReconciliationPlanSpec   `json:"spec,omitempty"`
	Status ReconciliationPlanStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ReconciliationPlanList contains a list of ReconciliationPlan
type ReconciliationPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReconciliationPlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReconciliationPlan{}, &ReconciliationPlanList{})
}
