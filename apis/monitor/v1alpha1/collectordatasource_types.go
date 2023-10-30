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

type ExporterRef struct {
	// exporterRef is the exporter to export metrics
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	ExporterNames []string `json:"exporterRefs"`
}

type MetricsCollector struct {
	ExporterRef `json:",inline"`

	//// containerName is the container name of the data source to collect
	//// +kubebuilder:validation:Required
	// ContainerName string `json:"containerName"`
	//
	//// component is the component name of the data source to collect
	//// +kubebuilder:validation:Required
	// Component string `json:"component"`

	// monitorType describes the monitor type, e.g: prometheus, mysql, pg, redis
	// +optional
	// TODO add validation for monitorType, support type?
	MonitorType string `json:"monitorType,omitempty"`

	// collectionInterval describes the collection interval.
	// +optional
	CollectionInterval string `json:"collectionInterval,omitempty"`

	// metricsSelector describes which metrics are collected.
	// +optional
	MetricsSelector []string `json:"metricsSelector,omitempty"`
}

type LogsCollector struct {
	ExporterRef `json:",inline"`

	// logTypes describes the logs types to collect, e.g: error, general, runninglog, slowlog, etc.
	// +optional
	LogTypes []string `json:"logTypes,omitempty"`
}

type ScrapeConfig struct {
	// externalLabels describes which labels are added to metrics.
	// +optional
	ExternalLabels map[string]string `json:"externalLabels,omitempty"`

	// containerName is the container name of the data source to collect
	// +kubebuilder:validation:Required
	ContainerName string `json:"containerName"`

	// logsCollector describes the logs collector
	// +optional
	Logs *LogsCollector `json:"logs"`

	// metricsCollector describes the metrics collector
	// +optional
	Metrics *MetricsCollector `json:"metrics"`
}

type CollectorSpec struct {
	// componentName is cluster component name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	ComponentName string `json:"componentName"`

	// scrapeConfigs describes the scrape configs
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	ScrapeConfigs []ScrapeConfig `json:"scrapeConfigs"`
}

// CollectorDataSourceSpec defines the desired state of CollectorDataSource
type CollectorDataSourceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// clusterRef references cluster.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.clusterRef"
	ClusterRef string `json:"clusterRef"`

	// collectorSpecs describes the collector specs
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	CollectorSpecs []CollectorSpec `json:"collectorSpecs"`

	// metricsCollector describes the metrics collector
	// +optional
	// MetricsCollector *MetricsCollectorSpec `json:"metricsCollectorSpec,omitempty"`
	//
	//// logsCollector describes the logs collector
	//// +optional
	// LogsCollector *LogsCollectorSpec `json:"logsCollectorSpec,omitempty"`
}

// CollectorDataSourceStatus defines the observed state of CollectorDataSource
type CollectorDataSourceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CollectorDataSource is the Schema for the collectordatasources API
type CollectorDataSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CollectorDataSourceSpec   `json:"spec,omitempty"`
	Status CollectorDataSourceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CollectorDataSourceList contains a list of CollectorDataSource
type CollectorDataSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CollectorDataSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CollectorDataSource{}, &CollectorDataSourceList{})
}
