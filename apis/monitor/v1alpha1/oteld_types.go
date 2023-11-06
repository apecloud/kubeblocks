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

type MemoryLimiter struct {
	// enabled indicates whether to enable memory limiter
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// config indicates the memory limiter config
	// +optional
	Config *MemoryLimiterConfig `json:",inline"`
}

type MemoryLimiterConfig struct {
	// MemoryLimitMiB is the maximum amount of memory, in MiB, targeted to be
	// allocated by the process.
	// +kubebuilder:validation:Required
	MemoryLimit uint32 `json:"memoryLimitMib,omitempty"`

	// MemorySpikeLimitMiB is the maximum, in MiB, spike expected between the
	// measurements of memory usage.
	// +kubebuilder:validation:Required
	MemorySpikeLimit uint32 `json:"memorySpikeLimitMib,omitempty"`

	// CheckInterval is the time between measurements of memory usage for the
	// purposes of avoiding going over the limits. if set to zero, no
	// checks will be performed.
	// +kubebuilder:validation:Required
	CheckInterval string `json:"checkInterval,omitempty"`
}

type BatchConfig struct {
	// Timeout sets the time after which a batch will be sent regardless of size.
	// When this is set to zero, batched data will be sent immediately.
	// +kubebuilder:validation:required
	Timeout string `json:"timeout,omitempty"`

	// SendBatchSize is the size of a batch which after hit, will trigger it to be sent.
	// When this is set to zero, the batch size is ignored and data will be sent immediately
	// subject to only send_batch_max_size.
	// kubebuilder:validation:required
	SendBatchSize int `json:"sendBatchSize,omitempty"`

	// SendBatchMaxSize is the maximum size of a batch. It must be larger than SendBatchSize.
	// Larger batches are split into smaller units.
	// Default value is 0, that means no maximum size.
	// +optional
	SendBatchMaxSize int `json:"sendBatchMaxSize,omitempty"`
}

type Batch struct {
	// enabled indicates whether to enable batch
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// config indicates the memory limiter config
	// +optional
	Config *BatchConfig `json:",inline"`
}

type NodeExporter struct {
	Enabled bool `json:"enabled,omitempty"`
}

type K8sClusterExporter struct {
	Enabled bool `json:"enabled,omitempty"`
}

type K8sKubeletExporter struct {
	Enabled       bool            `json:"enabled,omitempty"`
	MetricsFilter map[string]bool `json:"metricsFilter,omitempty"`
}

type PodLogs struct {
	Enabled bool `json:"enabled,omitempty"`
}

type SystemDataSource struct {
	// metricsExporterRef is the exporter to export system metrics
	// +optional
	MetricsExporterRef []string `json:"metricsExporterRef,omitempty"`

	// logsExporterRef is the exporter to export system logs
	// +optional
	LogsExporterRef []string `json:"logsExporterRef,omitempty"`

	// nodeExporter indicates how to collect node metrics
	// +optional
	NodeExporter *NodeExporter `json:"nodeExporter,omitempty"`

	// k8sClusterMetrics indicates how to collect k8s cluster metrics
	// +optional
	K8sClusterExporter *K8sClusterExporter `json:"k8sClusterExporter,omitempty"`

	// k8sKubeletExporter indicates how to collect kubelet states metrics
	// +optional
	K8sKubeletExporter *K8sKubeletExporter `json:"k8sKubeletExporter,omitempty"`

	// podLogs indicates how to collect pod logs
	// +optional
	PodLogs *PodLogs `json:"podLogs,omitempty"`

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

	// memoryLimiter is the memory limiter config that used to limit the memory usage of oteld
	// +optional
	MemoryLimiter MemoryLimiter `json:"memoryLimiter,omitempty"`

	// batch is the batch config that used to batch data before sending
	// +optional
	Batch Batch `json:"batch,omitempty"`

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
