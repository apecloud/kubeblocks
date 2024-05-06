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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NodeAwareScalerSpec defines the desired state of NodeAwareScaler
type NodeAwareScalerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of NodeAwareScaler. Edit nodeawarescaler_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// NodeAwareScalerStatus defines the observed state of NodeAwareScaler
type NodeAwareScalerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NodeAwareScaler is the Schema for the nodeawarescalers API
type NodeAwareScaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeAwareScalerSpec   `json:"spec,omitempty"`
	Status NodeAwareScalerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NodeAwareScalerList contains a list of NodeAwareScaler
type NodeAwareScalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeAwareScaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeAwareScaler{}, &NodeAwareScalerList{})
}
