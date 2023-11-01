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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type MemoryLimiterConfig struct {
	// MemoryLimit is the memory limit of the oteld
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

type BatchConfig struct {
	// enabled indicates whether to enable batch
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

type SystemDataSource struct {
	// metricsExporterRef is the exporter to export system metrics
	// +optional
	MetricsExporterRef []string `json:"metricsExporterRef,omitempty"`

	// logsExporterRef is the exporter to export system logs
	// +optional
	LogsExporterRef []string `json:"logsExporterRef,omitempty"`

	// enabledNodeMetrics indicates whether to collect node metrics
	// +optional
	EnabledNodeExporter bool `json:"enabledNodeExporter,omitempty"`
	// enabledK8sClusterMetrics indicates whether to collect k8s apiService metrics
	// +optional
	EnabledK8sClusterExporter bool `json:"enabledK8SClusterExporter,omitempty"`

	// enabledK8sKubeletExporter indicates whether to collect kubelet states metrics
	// +optional
	EnabledK8sKubeletExporter bool `json:"enabledK8SKubeletExporter,omitempty"`

	// enabledPodLogs indicates whether to collect pod logs
	// +optional
	EnabledPodLogs bool `json:"enabledPodLogs,omitempty"`

	// collectionInterval is the interval of the data source
	// +kubebuilder:default="15s"
	// +optional
	CollectionInterval string `json:"collectionInterval,omitempty"`
}

// OTeldSpec defines the desired state of CollectorDataSource
type OTeldSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// image is the image of the oteld
	// +kubebuilder:validation:Required
	Image string `json:"image,omitempty"`

	// logsLevel is the level of the oteld collector logs
	// +kubebuilder:default="info"
	// +kubebuilder:validation:Enum={debug,info,warn,error,dpanic,panic,fatal}
	// +optional
	LogsLevel string `json:"logLevel,omitempty"`

	// metricsPort is the port of the oteld metrics, the port must be larger than 1024.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=8888
	// +kubebuilder:validation:Minimum=1024
	MetricsPort int `json:"metricsPort,omitempty"`

	// collectionInterval is the default collect interval of the oteld collection
	// +kubebuilder:default="15s"
	// +optional
	CollectionInterval string `json:"collectionInterval,omitempty"`

	// +optional
	MemoryLimiter MemoryLimiterConfig `json:"memoryLimiter,omitempty"`

	// +optional
	Batch BatchConfig `json:"batch,omitempty"`

	// resources is the resource requirements for the oteld
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// useConfigMap indicates whether to use configmap to store oteld config
	// +kubebuilder:default=true
	// +optional
	UseConfigMap bool `json:"useConfigMap,omitempty"`

	// nodeSelector to schedule OpenTelemetry Collector pods.
	// This is only relevant to daemonset, statefulset, and deployment mode
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// globalLabels is the external labels add to OTeld metrics and logs
	// +optional
	GlobalLabels map[string]string `json:"globalLabels,omitempty"`

	// systemDataSource is the system data source
	// +optional
	SystemDataSource *SystemDataSource `json:"systemDataSource,omitempty"`
}

// OTeldStatus defines the observed state of CollectorDataSource
type OTeldStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// observedGeneration is the most recent generation observed for this StatefulSet. It corresponds to the
	// StatefulSet's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,1,opt,name=observedGeneration"`

	// message describes cluster details message in current phase.
	// +optional
	Message string `json:"message,omitempty"`

	// conditions describes oteld detail status.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// OTeld is the Schema for the collectordatasources API
type OTeld struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OTeldSpec   `json:"spec,omitempty"`
	Status OTeldStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OTeldList contains a list of CollectorDataSource
type OTeldList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OTeld `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OTeld{}, &OTeldList{})
}

func (in *OTeld) UseSecret() bool {
	return !in.Spec.UseConfigMap
}
