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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type MetricsDataSource struct {
	Enable bool `json:"enabled,omitempty"`

	CollectionInterval string   `json:"collectionInterval,omitempty"`
	EnabledMetrics     []string `json:"enabledMetrics,omitempty"`
}

type InputConfig struct {
	Include []string `json:"include,omitempty"`
}

type LogsDataSource struct {
	Enable bool `json:"enabled,omitempty"`

	LogCollector map[string]InputConfig `json:"logCollector,omitempty"`
}

type Container struct {

	// ContainerName is the container name of the data source to collect
	ContainerName string `json:"containerName,omitempty"`

	// CollectorName is the collector name of the data source to collect
	CollectorName string `json:"collectorName,omitempty"`

	// MetricsConfig is the metrics config
	Metrics *MetricsDataSource `json:"metrics,omitempty"`

	// LogsConfig is the logs config
	Logs *LogsDataSource `json:"logs,omitempty"`

	// ExternalLabels is the external labels added to the data source
	ExternalLabels map[string]string `json:"externalLabels,omitempty"`
}

type Component struct {
	ComponentName string `json:"componentName,omitempty"`

	Containers []Container `json:"containers,omitempty"`
}

// AppDataSourceSpec defines the desired state of AppDataSource
type AppDataSourceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Mode represents how the OTeld should be deployed (deployment, daemonset, statefulset or sidecar)
	// +optional
	Mode Mode `json:"mode,omitempty"`

	// NodeSelector is the node selector of the data source
	// only used when mode is deployment
	// +optional
	NodeSelector v1.NodeSelector `json:"nodeSelector,omitempty"`

	// ClusterName is the cluster name of the data source
	// +kubebuilder:validation:Required
	ClusterName string `json:"clusterName,omitempty"`

	// ComponentName is the component name of the data source
	// +kubebuilder:validation:Required
	Components []Component `json:"components,omitempty"`

	// CollectionInterval is the interval of the data source
	CollectionInterval string `json:"collectionInterval,omitempty"`
}

// AppDataSourceStatus defines the observed state of AppDataSource
type AppDataSourceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AppDataSource is the Schema for the AppDataSources API
type AppDataSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppDataSourceSpec   `json:"spec,omitempty"`
	Status AppDataSourceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AppDataSourceList contains a list of AppDataSource
type AppDataSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppDataSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AppDataSource{}, &AppDataSourceList{})
}
