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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LogsExporterSinkSpec defines the desired state of LogsExporterSink
type LogsExporterSinkSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of LogsExporterSink. Edit logsexportersink_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// LogsExporterSinkStatus defines the observed state of LogsExporterSink
type LogsExporterSinkStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// LogsExporterSink is the Schema for the logsexportersinks API
type LogsExporterSink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LogsExporterSinkSpec   `json:"spec,omitempty"`
	Status LogsExporterSinkStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LogsExporterSinkList contains a list of LogsExporterSink
type LogsExporterSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LogsExporterSink `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LogsExporterSink{}, &LogsExporterSinkList{})
}
