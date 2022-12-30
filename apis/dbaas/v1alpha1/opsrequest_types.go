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
	// clusterRef reference clusterDefinition.
	// +kubebuilder:validation:Required
	ClusterRef string `json:"clusterRef"`

	// type defines the operation type.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum={Upgrade,VerticalScaling,VolumeExpansion,HorizontalScaling,Restart}
	Type OpsType `json:"type"`

	// ttlSecondsAfterSucceed OpsRequest will be deleted after TTLSecondsAfterSucceed second when OpsRequest.status.phase is Succeed.
	// +optional
	TTLSecondsAfterSucceed int32 `json:"ttlSecondsAfterSucceed,omitempty"`

	// upgrade specify the cluster version by specifying clusterVersionRef.
	// +optional
	Upgrade *Upgrade `json:"upgrade,omitempty"`

	// horizontalScaling defines what component need to horizontal scale the specified replicas.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	HorizontalScalingList []HorizontalScaling `json:"horizontalScaling,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// volumeExpansion defines what component and volumeClaimTemplate need to expand the specified storage.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	VolumeExpansionList []VolumeExpansion `json:"volumeExpansion,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// restart the specified component.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	RestartList []ComponentOps `json:"restart,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// verticalScaling defines what component need to vertical scale the specified compute resources.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	VerticalScalingList []VerticalScaling `json:"verticalScaling,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`
}

// ComponentOps defines the common variables of component scope operations.
type ComponentOps struct {
	// componentName cluster component name.
	// +kubebuilder:validation:Required
	ComponentName string `json:"componentName"`
}

// Upgrade defines the variables of upgrade operation.
type Upgrade struct {
	// clusterVersionRef reference ClusterVersion name.
	// +kubebuilder:validation:Required
	ClusterVersionRef string `json:"clusterVersionRef"`
}

// VerticalScaling defines the variables that need to input when scaling compute resources.
type VerticalScaling struct {
	ComponentOps `json:",inline"`

	// resources specify the computing resource size of verticalScaling.
	// +kubebuilder:validation:Required
	*corev1.ResourceRequirements `json:",inline"`
}

// VolumeExpansion defines the variables of volume expansion operation.
type VolumeExpansion struct {
	ComponentOps `json:",inline"`

	// volumeClaimTemplates specify the storage size and volumeClaimTemplate name.
	// +kubebuilder:validation:Required
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	VolumeClaimTemplates []OpsRequestVolumeClaimTemplate `json:"volumeClaimTemplates" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`
}

type OpsRequestVolumeClaimTemplate struct {
	// Request storage size.
	// +kubebuilder:validation:Required
	Storage resource.Quantity `json:"storage"`

	// name reference volumeClaimTemplate name from cluster components.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// HorizontalScaling defines the variables of horizontal scaling operation
type HorizontalScaling struct {
	ComponentOps `json:",inline"`

	// replicas for the workloads.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas"`
}

// OpsRequestStatus defines the observed state of OpsRequest
type OpsRequestStatus struct {
	// observedGeneration is the most recent generation observed for this
	// Cluster. It corresponds to the Cluster's generation, which is
	// updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// phase describe OpsRequest phase.
	// +kubebuilder:validation:Enum={Pending,Running,Failed,Succeed}
	Phase Phase `json:"phase,omitempty"`

	// components define the recorded the status information of changed components for operation request.
	// +optional
	Components map[string]OpsRequestStatusComponent `json:"components,omitempty"`

	// startTimestamp The time when the OpsRequest started processing.
	// +optional
	StartTimestamp *metav1.Time `json:"StartTimestamp,omitempty"`

	// completionTimestamp defines the OpsRequest completion time.
	// +optional
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// conditions describe opsRequest detail status.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type OpsRequestStatusComponent struct {
	// phase describe the component phase, reference ClusterDefinition.status.component.phase.
	// +kubebuilder:validation:Enum={Running,Failed,Abnormal,Creating,Updating,Deleting,Deleted,VolumeExpanding}
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// volumeClaimTemplates describe the volumeClaimTemplates status when spec.type is VolumeExpansion
	// +optional
	VolumeClaimTemplates map[string]*VolumeClaimTemplateStatus `json:"volumeClaimTemplates,omitempty"`
}

type VolumeClaimTemplateStatus struct {
	StatusMessage `json:",inline"`

	// Request storage size.
	// +optional
	RequestStorage resource.Quantity `json:"requestStorage,omitempty"`

	// persistentVolumeClaimStatus describe the persistentVolumeClaim status
	// +optional
	PersistentVolumeClaimStatus map[string]StatusMessage `json:"persistentVolumeClaims,omitempty"`
}

type StatusMessage struct {
	// +optional
	Message string `json:"message,omitempty"`

	// +kubebuilder:validation:Enum={Running,Pending,Failed,Succeed}
	// +optional
	Status Phase `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories={dbaas,all},shortName=ops
//+kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.type",description="Operation request type."
//+kubebuilder:printcolumn:name="CLUSTER",type="string",JSONPath=".spec.clusterRef",description="Operand cluster."
//+kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="Operation status phase."
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

// GetComponentNameMap if the operations is within the scope of component, this function should be implemented
func (r *OpsRequest) GetComponentNameMap() map[string]struct{} {
	switch r.Spec.Type {
	case RestartType:
		return r.GetRestartComponentNameMap()
	case VerticalScalingType:
		return r.GetVerticalScalingComponentNameMap()
	case HorizontalScalingType:
		return r.GetHorizontalScalingComponentNameMap()
	case VolumeExpansionType:
		return r.GetVolumeExpansionComponentNameMap()
	default:
		return nil
	}
}

// GetRestartComponentNameMap get the component name map with restart operation.
func (r *OpsRequest) GetRestartComponentNameMap() map[string]struct{} {
	componentNameMap := make(map[string]struct{})
	for _, v := range r.Spec.RestartList {
		componentNameMap[v.ComponentName] = struct{}{}
	}
	return componentNameMap
}

// GetVerticalScalingComponentNameMap get the component name map with vertical scaling operation.
func (r *OpsRequest) GetVerticalScalingComponentNameMap() map[string]struct{} {
	componentNameMap := make(map[string]struct{})
	for _, v := range r.Spec.VerticalScalingList {
		componentNameMap[v.ComponentName] = struct{}{}
	}
	return componentNameMap
}

// CovertVerticalScalingListToMap covert OpsRequest.spec.verticalScaling list to map
func (r *OpsRequest) CovertVerticalScalingListToMap() map[string]VerticalScaling {
	verticalScalingMap := make(map[string]VerticalScaling)
	for _, v := range r.Spec.VerticalScalingList {
		verticalScalingMap[v.ComponentName] = v
	}
	return verticalScalingMap
}

// GetHorizontalScalingComponentNameMap get the component name map with horizontal scaling operation.
func (r *OpsRequest) GetHorizontalScalingComponentNameMap() map[string]struct{} {
	componentNameMap := make(map[string]struct{})
	for _, v := range r.Spec.HorizontalScalingList {
		componentNameMap[v.ComponentName] = struct{}{}
	}
	return componentNameMap
}

// CovertHorizontalScalingListToMap covert OpsRequest.spec.horizontalScaling list to map
func (r *OpsRequest) CovertHorizontalScalingListToMap() map[string]HorizontalScaling {
	verticalScalingMap := make(map[string]HorizontalScaling)
	for _, v := range r.Spec.HorizontalScalingList {
		verticalScalingMap[v.ComponentName] = v
	}
	return verticalScalingMap
}

// GetVolumeExpansionComponentNameMap get the component name map with volume expansion operation.
func (r *OpsRequest) GetVolumeExpansionComponentNameMap() map[string]struct{} {
	componentNameMap := make(map[string]struct{})
	for _, v := range r.Spec.VolumeExpansionList {
		componentNameMap[v.ComponentName] = struct{}{}
	}
	return componentNameMap
}

// CovertVolumeExpansionListToMap covert volumeExpansionList to map
func (r *OpsRequest) CovertVolumeExpansionListToMap() map[string]VolumeExpansion {
	volumeExpansionMap := make(map[string]VolumeExpansion)
	for _, v := range r.Spec.VolumeExpansionList {
		volumeExpansionMap[v.ComponentName] = v
	}
	return volumeExpansionMap
}
