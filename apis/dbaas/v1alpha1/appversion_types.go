/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AppVersionSpec defines the desired state of AppVersion
type AppVersionSpec struct {
	// ref ClusterDefinition
	// +kubebuilder:validation:Required
	ClusterDefinitionRef string `json:"clusterDefinitionRef,omitempty"`

	// +kubebuilder:validation:MinItems=1
	Components []AppVersionComponent `json:"components,omitempty"`
}

// AppVersionStatus defines the observed state of AppVersion
type AppVersionStatus struct {
	// phase - in list of [Available,UnAvailable,Deleting]
	// +kubebuilder:validation:Enum={Available,UnAvailable,Deleting}
	Phase Phase `json:"phase,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
	// generation number
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	ClusterDefinitionStatusGeneration `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories={dbaas},scope=Cluster
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="status phase"

// AppVersion is the Schema for the appversions API
type AppVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppVersionSpec   `json:"spec,omitempty"`
	Status AppVersionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AppVersionList contains a list of AppVersion
type AppVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppVersion `json:"items"`
}

type AppVersionComponent struct {
	// component type in ClusterDefinition
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=12
	Type string `json:"type"`

	// if not nil, will replace ClusterDefinitionSpec.Containers in ClusterDefinition
	// +optional
	Containers []corev1.Container `json:"containers,omitempty"`

	// if not nil, will replace ClusterDefinitionSpec.Serivce in ClusterDefinition
	// +optional
	Service corev1.ServiceSpec `json:"service,omitempty"`
}

func init() {
	SchemeBuilder.Register(&AppVersion{}, &AppVersionList{})
}
