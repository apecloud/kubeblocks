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
	ObjectTreeRule ObjectTreeRule `json:"objectTreeRule"`
	StateEvaluationExpression string `json:"stateEvaluationExpression"`
	I18nResourceRef *ObjectReference `json:"i18nResourceRef"`
}

// ReconciliationViewDefinitionStatus defines the observed state of ReconciliationViewDefinition
type ReconciliationViewDefinitionStatus struct {}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

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
