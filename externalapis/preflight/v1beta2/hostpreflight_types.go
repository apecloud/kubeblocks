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

// HostPreflightSpec defines the desired state of HostPreflight
type HostPreflightSpec struct {
	// hostPreflightSpec is defined by troubleshoot.sh and inherited by ApeCloud.
	troubleshoot.HostPreflightSpec `json:",inline"`
	// extendCollectors extends user defined hostCollectors by ApeCloud.
	// +optional
	ExtendCollectors []*ExtendHostCollect `json:"extendCollectors,omitempty"`
	// extendAnalyzers extends user defined hostAnalyzers by ApeCloud.
	// +optional
	ExtendAnalyzers []*ExtendHostAnalyze `json:"extendAnalyzers,omitempty"`
}

// HostPreflightStatus defines the observed state of HostPreflight
type HostPreflightStatus struct {
	// hostPreflightStatus is defined by troubleshoot.sh and inherited by ApeCloud.
	troubleshoot.HostPreflightStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion

// HostPreflight is the Schema for the hostpreflights API
type HostPreflight struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostPreflightSpec   `json:"spec,omitempty"`
	Status HostPreflightStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HostPreflightList contains a list of HostPreflight
type HostPreflightList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostPreflight `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HostPreflight{}, &HostPreflightList{})
}
