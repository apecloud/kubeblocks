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
	Enabled bool `json:"enabled,omitempty"`
}

type BatchConfig struct {
	Enabled bool `json:"enabled,omitempty"`
}

// OTeldSpec defines the desired state of CollectorDataSource
type OTeldSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Image is the image of the oteld
	Image string `json:"image,omitempty"`

	// LogsLevel is the level of the oteld collector logs
	LogsLevel string `json:"logLevel,omitempty"`

	// MetricsPort is the port of the oteld metrics
	MetricsPort int `json:"metricsPort,omitempty"`

	// CollectionInterval is the default collect interval of the oteld collection
	CollectionInterval string `json:"collectionInterval"`

	MemoryLimiter MemoryLimiterConfig `json:"memoryLimiter,omitempty"`

	Batch BatchConfig `json:"batch,omitempty"`

	// Resources is the resource requirements for the oteld
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Mode represents how the OTeld should be deployed (deployment, daemonset, statefulset or sidecar)
	Mode Mode `json:"mode,omitempty"`

	// UseConfigMap indicates whether to use configmap to store oteld config
	UseConfigMap bool `json:"useConfigMap"`

	// NodeSelector to schedule OpenTelemetry Collector pods.
	// This is only relevant to daemonset, statefulset, and deployment mode
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// OTeldStatus defines the observed state of CollectorDataSource
type OTeldStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
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
