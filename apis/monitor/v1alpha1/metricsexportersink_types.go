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

// MetricsExporterSinkSpec defines the desired state of MetricsExporterSink
type MetricsExporterSinkSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of MetricsExporterSink. Edit metricsexportersink_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// MetricsExporterSinkStatus defines the observed state of MetricsExporterSink
type MetricsExporterSinkStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MetricsExporterSink is the Schema for the metricsexportersinks API
type MetricsExporterSink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MetricsExporterSinkSpec   `json:"spec,omitempty"`
	Status MetricsExporterSinkStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MetricsExporterSinkList contains a list of MetricsExporterSink
type MetricsExporterSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetricsExporterSink `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MetricsExporterSink{}, &MetricsExporterSinkList{})
}
