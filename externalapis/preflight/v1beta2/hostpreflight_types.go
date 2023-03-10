/*
Copyright ApeCloud, Inc.

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

package v1beta2

import (
	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HostPreflightSpec defines the desired state of HostPreflight
type HostPreflightSpec struct {
	// hostPreflightSpec is defined by troubleshoot.sh and inherited by ApeCloud.
	troubleshoot.HostPreflightSpec `json:",inline"`
	// extendCollectors extends user defined hostCollectors by ApeCloud.
	// +optional
	ExtendCollectors []*ExtendHostCollect `json:"extendCollectors,omitempty"`
	// extendAnalyzers extends user defined hostAnalyzers by ApeCloud.
	// +optional
	ExtendAnalyzers []*ExtendHostAnalyze `json:"extendAnalyzers,omitempty"`
}

// HostPreflightStatus defines the observed state of HostPreflight
type HostPreflightStatus struct {
	// hostPreflightStatus is defined by troubleshoot.sh and inherited by ApeCloud.
	troubleshoot.HostPreflightStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion

// HostPreflight is the Schema for the hostpreflights API
type HostPreflight struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostPreflightSpec   `json:"spec,omitempty"`
	Status HostPreflightStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HostPreflightList contains a list of HostPreflight
type HostPreflightList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostPreflight `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HostPreflight{}, &HostPreflightList{})
}
