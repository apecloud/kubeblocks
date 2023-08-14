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

type Prometheus struct {
}

type MetricsExporter struct {
	Prometheus *Prometheus `json:"prometheus"`
}

type S3Config struct {
	// prefix = metric/logs
	// filePrefix = slow/error/auditlog
	// path metric/year=XXXX/month=XX/day=XX/hour=XX/minute=XX/filePrefix
	Region     string `json:"region"`
	Bucket     string `json:"bucket"`
	Prefix     string `json:"prefix"`
	Partition  string `json:"partition"`
	FilePrefix string `json:"filePrefix"`
}

type S3Credentials struct {
	Secret    string `json:"secret"`
	Namespace string `json:"namespace"`
}

type AWSS3Config struct {
	S3Config    `json:"inline"`
	Credentials S3Credentials `json:"credentials"`
}

type LokiConfig struct {
	Endpoint    string       `json:"endpoint"`
	RetryConfig *RetryConfig `json:"retryConfig"`
	QueueConfig QueueConfig  `json:"queueConfig"`
}

type LogsExporter struct {
	S3Config   *AWSS3Config `json:"s3Config"`
	LokiConfig *LokiConfig  `json:"lokiConfig"`
}

// AgamottoExporterSpec defines the desired state of AgamottoExporter
type AgamottoExporterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	LogsExporter   *LogsExporter    `json:"logsExporter"`
	MetricExporter *MetricsExporter `json:"metricsExporter"`
}

// AgamottoExporterStatus defines the observed state of AgamottoExporter
type AgamottoExporterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// AgamottoExporter is the Schema for the agamottoexporters API
type AgamottoExporter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgamottoExporterSpec   `json:"spec,omitempty"`
	Status AgamottoExporterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AgamottoExporterList contains a list of AgamottoExporter
type AgamottoExporterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgamottoExporter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgamottoExporter{}, &AgamottoExporterList{})
}
