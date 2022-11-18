/*
Copyright ApeCloud Inc.

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

// ReconfigureRequestSpec defines the desired state of ReconfigureRequest
type ReconfigureRequestSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster

	// reference Cluster resource.
	// +kubebuilder:validation:Required
	ClusterRef string `json:"clusterRef,omitempty"`

	// reference ClusterDefinition resource.
	// +kubebuilder:validation:Required
	ComponentRef string `json:"componentRef,omitempty"`

	// +kubebuilder:validation:Required
	Configurations []Configuration `json:"configurations,omitempty"`
}

// ReconfigureRequestStatus defines the observed state of ReconfigureRequest
type ReconfigureRequestStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Phase is reconfigure request object status.
	// +optional
	Phase string `json:"phase,omitempty"`

	// Flows field defines the detailed process flow that is executed during reconfigure.
	// reference url: https://infracreate.feishu.cn/wiki/wikcn24AWAgXXBedVZZ0YgvjGuc
	// +optional
	Flows []ReconfigureStateInfo `json:"flows,omitempty"`

	// Pods field describes information about the pod that is being upgraded or has been successfully upgraded.
	// +optional
	Pods []ReconfigurePodStatus `json:"pods,omitempty"`

	// WaitPods field describes which Pods are still waiting to be upgraded.
	// +optional
	WaitPods []corev1.ObjectReference `json:"waitPods,omitempty"`
}

type ReconfigurePodStatus struct {
	// ProcessStartTime field describes when to start upgrade for pod.
	// +optional
	ProcessStartTime *metav1.Time `json:"processStartTime,omitempty"`

	// ProcessLatency field describes how long did the upgrade take.
	// +optional
	ProcessLatency *metav1.Duration `json:"processLatency,omitempty"`

	// PodRef field describes pod reference.
	// +optional
	PodRef *corev1.ObjectReference `json:"podRef,omitempty" protobuf:"bytes,4,opt,name=targetRef"`
}

type ReconfigureStateInfo struct {
	// StartTime field describes when to start upgrade for ops.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Step field describes status of the upgrade process.
	// +kubebuilder:validation:Required
	Step string `json:"step,omitempty"`

	// ErrorMessage field describes details of an error.
	// +optional
	ErrorMessage string `json:"errorMessage,omitempty"`
}

type Configuration struct {

	// Scope refers to the effective range of the updated parameter.
	// 1. If Scope = ScopeMemory, db engine will make the change specified by reconfigure operator for the life of the instance.
	// 	  The next time the database is bounced, for any reason, the change will be reverted to the default value.
	// 2. If Scope = ScopeFile, the change made in reconfigure operator will take place starting from the next startup but will not affect the current instance.
	// 3. If Scope = ScopeBoth, the operator will take effect immediately, and will make the change for the current instance and preserve it through any future bounces.
	// +kubebuilder:validation:Required
	// +kubebuilder:default:"ScopeBoth"
	// +kubebuilder:validation:Enum={ScopeBoth,ScopeFile,ScopeMemory}
	Scope ScopeType `json:"scope,omitempty"`

	// +optional
	Parameters []string `json:"parameters,omitempty"`

	// Files user creates or updates a file to configmap
	// +optional
	Files map[string]string `json:"files,omitempty"`

	// MountPoint is a volume name which file will mount to
	// +optional
	MountPoint string `json:"mountPoint,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories={dbaas,all},scope=Namespaced,shortName=upc
//+kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="Reconfigure Status."
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ReconfigureRequest is the Schema for the reconfigurerequests API
type ReconfigureRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReconfigureRequestSpec   `json:"spec,omitempty"`
	Status ReconfigureRequestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ReconfigureRequestList contains a list of ReconfigureRequest.
type ReconfigureRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReconfigureRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReconfigureRequest{}, &ReconfigureRequestList{})
}
