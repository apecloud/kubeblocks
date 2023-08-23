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

type ServiceRef struct {
	// Specify the cluster of the referenced service.
	// +optional
	ClusterName string `json:"clusterName,omitempty"`

	// Specify the namespace of the referenced the serviceConnectionCredential object.
	// An empty namespace is equivalent to the "default" namespace.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
	// +kubebuilder:default="default"
	// +optional
	Namespace string `json:"namespace"`

	// Specify the name of the referenced the serviceConnectionCredential object.
	// +kubebuilder:validation:Required
	ServiceConnectionCredential string `json:"serviceConnectionCredential"`

	// TODO the field for test
	// Specify the endpoint of the referenced service.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
}

type LokiConfig struct {
	ServiceRef `json:",inline"`

	// Define retry strategy when loki service is unavailable.
	// If not set, not retry sending batches in case of export failure.
	// +optional
	RetryPolicyOnFailure *RetryPolicyOnFailure `json:"retryPolicyOnFailure"`

	// Define whether to not enqueue batches before sending to the consumerSender.
	// +optional
	SinkQueueConfig *SinkQueueConfig `json:"sinkQueueConfig"`
}

type S3Config struct {
}

type SinkSource struct {
	// lokiConfig defines the config of the loki
	// +optional
	LokiConfig *LokiConfig `json:"lokiConfig,omitempty"`

	// s3Config defines the config of the s3
	// +optional
	S3Config *S3Config `json:"s3Config,omitempty"`
}

// LogsExporterSinkSpec defines the desired state of LogsExporterSink
type LogsExporterSinkSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// type defines the type of the exporterSink
	// +kubebuilder:validation:Required
	Type LogsSinkType `json:"type"`

	// SinkSource describes the config of the exporterSink
	// +kubebuilder:validation:Required
	SinkSource `json:"inline"`
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
