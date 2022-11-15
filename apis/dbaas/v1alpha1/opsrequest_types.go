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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpsRequestSpec defines the desired state of OpsRequest
type OpsRequestSpec struct {
	// ClusterRef reference clusterDefinition resource
	// +kubebuilder:validation:Required
	ClusterRef string `json:"clusterRef"`

	// Type defines the operation type
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum={Upgrade,VerticalScaling,VolumeExpansion,HorizontalScaling,Restart}
	Type OpsType `json:"type"`

	// TTLSecondsAfterSucceed OpsRequest will be deleted after TTLSecondsAfterSucceed second when OpsRequest.status.phase is Running
	// +optional
	TTLSecondsAfterSucceed int32 `json:"ttlSecondsAfterSucceed,omitempty"`

	// ClusterOps defines cluster level operations, like Upgrade
	// +optional
	ClusterOps *ClusterOps `json:"clusterOps,omitempty"`

	// ComponentOpsList defines operations in component scope, like VolumeExpansion,VerticalScaling,HorizontalScaling
	// +optional
	ComponentOpsList []ComponentOps `json:"componentOps,omitempty"`
}

// OpsRequestStatus defines the observed state of OpsRequest
type OpsRequestStatus struct {
	// observedGeneration is the most recent generation observed for this
	// Cluster. It corresponds to the Cluster's generation, which is
	// updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +kubebuilder:validation:Enum={Pending,Running,Failed,Succeed}
	Phase Phase `json:"phase,omitempty"`

	// Components record the status information of components with spec.componentOps.componentNames
	// +optional
	Components map[string]OpsRequestStatusComponent `json:"components,omitempty"`

	// this means when start processing OpsRequest, status.Phase is Running
	// +optional
	StartTimestamp *metav1.Time `json:"StartTimestamp,omitempty"`

	// the OpsRequest completion time
	// +optional
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// describe opsRequest detail status
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type ClusterOps struct {
	// +kubebuilder:validation:Required
	Upgrade *Upgrade `json:"upgrade"`
}

type ComponentOps struct {
	// ComponentNames defines which components perform the operation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	ComponentNames []string `json:"componentNames"`

	// VolumeExpansion defines the variables that need to be input when expanding a volume.
	// +optional
	VolumeExpansion []VolumeExpansion `json:"volumeExpansion,omitempty"`

	// VerticalScaling defines the variables that need to be input when scaling compute resources.
	// +optional
	VerticalScaling *corev1.ResourceRequirements `json:"verticalScaling,omitempty"`

	// HorizontalScaling defines the variables that need to be input when scaling replicas.
	// +optional
	HorizontalScaling *HorizontalScaling `json:"horizontalScaling,omitempty"`
}

type Upgrade struct {
	// AppVersionRef reference AppVersion.
	// +kubebuilder:validation:Required
	AppVersionRef string `json:"appVersionRef"`
}

type VolumeExpansion struct {
	// The request storage size.
	// +kubebuilder:validation:Required
	Storage resource.Quantity `json:"storage"`

	// ClusterComponentVolumeClaimTemplate.Name
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

type HorizontalScaling struct {
	// Replicas for the workloads.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`
}

type OpsRequestStatusComponent struct {
	// Phase  describe the component phase, Reference ClusterDefinition.status.component.phase.
	// +kubebuilder:validation:Enum={Running,Failed,Abnormal,Creating,Updating,Deleting,Deleted}
	// +optional
	Phase Phase `json:"phase,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories={dbaas,all},shortName=ops
//+kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.type",description="Operation request type."
//+kubebuilder:printcolumn:name="CLUSTER",type="string",JSONPath=".spec.clusterRef",description="Operand cluster."
//+kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="Operation status phase."
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// OpsRequest is the Schema for the opsrequests API
type OpsRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpsRequestSpec   `json:"spec,omitempty"`
	Status OpsRequestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpsRequestList contains a list of OpsRequest
type OpsRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpsRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpsRequest{}, &OpsRequestList{})
}
