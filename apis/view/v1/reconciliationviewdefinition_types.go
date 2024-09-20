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

// ReconciliationViewDefinitionSpec defines the desired state of ReconciliationViewDefinition
type ReconciliationViewDefinitionSpec struct {
	// OwnershipRules specifies ownership rules to build the object tree.
	// A primary object and all its recursive secondary objects compose an object tree.
	//
	OwnershipRules []OwnershipRule `json:"ownershipRules"`

	// StateEvaluationExpression is used to evaluate whether the primary object has reach its desired state
	// by applying the expression to the object tree.
	//
	StateEvaluationExpression StateEvaluationExpression `json:"stateEvaluationExpression"`

	// I18nResourceRef specifies the ConfigMap object that contains the i18n resources
	// which are used to make the View/Plan Status more user-friendly.
	//
	// +optional
	I18nResourceRef *ObjectReference `json:"i18nResourceRef"`

	// Locale specifies the default locale.
	//
	// +optional
	Locale *string `json:"locale"`
}

// ReconciliationViewDefinitionStatus defines the observed state of ReconciliationViewDefinition
type ReconciliationViewDefinitionStatus struct{}

// OwnershipRule defines an ownership rule between primary resource and its secondary resources.
type OwnershipRule struct {
	// Primary specifies the primary object type.
	//
	Primary ObjectType `json:"primary"`

	// OwnedResources specifies all the secondary resources of Primary.
	//
	OwnedResources []OwnedResource `json:"ownedResources"`
}

// OwnedResource defines a secondary resource and the ownership criteria between its primary resource.
type OwnedResource struct {
	// Secondary specifies the secondary object type.
	//
	Secondary ObjectType `json:"secondary"`

	// Criteria specifies the ownership criteria with its primary resource.
	//
	Criteria OwnershipCriteria `json:"criteria"`
}

// OwnershipCriteria defines an ownership criteria.
// Only one of SelectorCriteria, LabelCriteria or BuiltinRelationshipCriteria should be configured.
type OwnershipCriteria struct {
	// SelectorCriteria specifies the selector field path in the primary object.
	// For example, if the StatefulSet is the primary resource, selector will be "spec.selector".
	// The selector field should be a map[string]string
	// or LabelSelector (https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/label-selector/#LabelSelector)
	//
	// +optional
	SelectorCriteria *FieldPath `json:"selectorCriteria,omitempty"`

	// LabelCriteria specifies the labels used to select the secondary objects.
	// The value of each k-v pair can contain placeholder that will be replaced by the ReconciliationView Controller.
	// Placeholder is formatted as "$(PLACEHOLDER)".
	// Currently supported PLACEHOLDER:
	// primary.name - the name of the primary object.
	//
	// +optional
	LabelCriteria map[string]string `json:"labelCriteria,omitempty"`

	// BuiltinRelationshipCriteria specifies to use the well-known builtin relationship between some primary resource with its secondary resources.
	// Currently supported resources:
	// PVC-PV pair.
	//
	// +optional
	BuiltinRelationshipCriteria *bool `json:"builtinRelationshipCriteria,omitempty"`

	// Validation specifies the method to validate the OwnerReference of secondary resources.
	//
	// +kubebuilder:validation:Enum={Controller, Owner, None}
	// +kubebuilder:default=Controller
	// +optional
	Validation ValidationType `json:"validation,omitempty"`
}

// FieldPath defines a field path.
type FieldPath struct {
	// Path of the field.
	//
	Path string `json:"path"`
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

// ValidationType specifies the method to validate the OwnerReference of secondary resources.
type ValidationType string

const (
	// ControllerValidation requires the secondary resource to have the primary resource
	// in its OwnerReference with controller set to true.
	ControllerValidation ValidationType = "Controller"

	// OwnerValidation requires the secondary resource to have the primary resource
	// in its OwnerReference.
	OwnerValidation ValidationType = "Owner"

	// NoValidation means no validation is performed on the OwnerReference.
	NoValidation ValidationType = "None"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=rvd

// ReconciliationViewDefinition is the Schema for the reconciliationviewdefinitions API
type ReconciliationViewDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReconciliationViewDefinitionSpec   `json:"spec,omitempty"`
	Status ReconciliationViewDefinitionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ReconciliationViewDefinitionList contains a list of ReconciliationViewDefinition
type ReconciliationViewDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReconciliationViewDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReconciliationViewDefinition{}, &ReconciliationViewDefinitionList{})
}
