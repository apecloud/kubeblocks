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

// PreflightSpec defines the desired state of Preflight
type PreflightSpec struct {
	// preflightSpec is defined by troubleshoot.sh and inherited by ApeCloud.
	troubleshoot.PreflightSpec `json:",inline"`
	// extendCollectors extends user defined collectors by ApeCloud.
	// +optional
	ExtendCollectors []*ExtendCollect `json:"extendCollectors,omitempty"`
	// extendAnalyzers extends user defined analyzers by ApeCloud.
	// +optional
	ExtendAnalyzers []*ExtendAnalyze `json:"extendAnalyzers,omitempty"`
}

// PreflightStatus defines the observed state of Preflight
type PreflightStatus struct {
	// preflightStatus is defined by troubleshoot.sh and inherited by ApeCloud.
	troubleshoot.PreflightStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion

// Preflight is the Schema for the preflights API
type Preflight struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PreflightSpec   `json:"spec,omitempty"`
	Status PreflightStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PreflightList contains a list of Preflight
type PreflightList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Preflight `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Preflight{}, &PreflightList{})
}
