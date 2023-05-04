/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package v1beta2

import (
	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PreflightSpec defines the desired state of Preflight
type PreflightSpec struct {
	// preflightSpec is defined by troubleshoot.sh and inherited by ApeCloud.
	troubleshoot.PreflightSpec `json:",inline"`
	// extendCollectors extends user defined collectors by ApeCloud.
	// +optional
	ExtendCollectors []*ExtendCollect `json:"extendCollectors,omitempty"`
	// extendAnalyzers extends user defined analyzers by ApeCloud.
	// +optional
	ExtendAnalyzers []*ExtendAnalyze `json:"extendAnalyzers,omitempty"`
}

// PreflightStatus defines the observed state of Preflight
type PreflightStatus struct {
	// preflightStatus is defined by troubleshoot.sh and inherited by ApeCloud.
	troubleshoot.PreflightStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion

// Preflight is the Schema for the preflights API
type Preflight struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PreflightSpec   `json:"spec,omitempty"`
	Status PreflightStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PreflightList contains a list of Preflight
type PreflightList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Preflight `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Preflight{}, &PreflightList{})
}
