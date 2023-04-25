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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: @wangyelei could refactor to ops group

// OpsRequestSpec defines the desired state of OpsRequest
type OpsRequestSpec struct {
	// clusterRef references clusterDefinition.
	// +kubebuilder:validation:Required
	ClusterRef string `json:"clusterRef"`

	// type defines the operation type.
	// +kubebuilder:validation:Required
	Type OpsType `json:"type"`

	// ttlSecondsAfterSucceed OpsRequest will be deleted after TTLSecondsAfterSucceed second when OpsRequest.status.phase is Succeed.
	// +optional
	TTLSecondsAfterSucceed int32 `json:"ttlSecondsAfterSucceed,omitempty"`

	// upgrade specifies the cluster version by specifying clusterVersionRef.
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

	// reconfigure defines the variables that need to input when updating configuration.
	// +optional
	Reconfigure *Reconfigure `json:"reconfigure,omitempty"`

	// expose defines services the component needs to expose.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	ExposeList []Expose `json:"expose,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// cluster RestoreFrom backup or point in time
	// +optional
	RestoreFrom *RestoreFromSpec `json:"restoreFrom,omitempty"`
}

// ComponentOps defines the common variables of component scope operations.
type ComponentOps struct {
	// componentName cluster component name.
	// +kubebuilder:validation:Required
	ComponentName string `json:"componentName"`
}

// Upgrade defines the variables of upgrade operation.
type Upgrade struct {
	// clusterVersionRef references ClusterVersion name.
	// +kubebuilder:validation:Required
	ClusterVersionRef string `json:"clusterVersionRef"`
}

// VerticalScaling defines the variables that need to input when scaling compute resources.
type VerticalScaling struct {
	ComponentOps `json:",inline"`

	// resources specifies the computing resource size of verticalScaling.
	// +kubebuilder:pruning:PreserveUnknownFields
	corev1.ResourceRequirements `json:",inline"`

	// classDefRef reference class defined in ComponentClassDefinition.
	// +optional
	ClassDefRef *ClassDefRef `json:"classDefRef,omitempty"`
}

// VolumeExpansion defines the variables of volume expansion operation.
type VolumeExpansion struct {
	ComponentOps `json:",inline"`

	// volumeClaimTemplates specifies the storage size and volumeClaimTemplate name.
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

	// name references volumeClaimTemplate name from cluster components.
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

type Reconfigure struct {
	ComponentOps `json:",inline"`

	// configurations defines which components perform the operation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Configurations []Configuration `json:"configurations" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// TTL(Time to Live) defines the time period during which changing parameters is valid.
	// +optional
	// TTL *int64 `json:"ttl,omitempty"`

	// triggeringTime defines the time at which the changing parameter to be applied.
	// +kubebuilder:validation:MaxLength=19
	// +kubebuilder:validation:MinLength=19
	// +kubebuilder:validation:Pattern:=`^([0-9]{2})/([0-9]{2})/([0-9]{4}) ([0-9]{2}):([0-9]{2}):([0-9]{2})$`
	// +optional
	// TriggeringTime *string `json:"triggeringTime,omitempty"`

	// selector indicates the component for reconfigure
	// +optional
	// Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

type Configuration struct {
	// name is a config template name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// keys is used to set the parameters to be updated.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=key
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=key
	Keys []ParameterConfig `json:"keys" patchStrategy:"merge,retainKeys" patchMergeKey:"key"`
}

type ParameterPair struct {
	// key is name of the parameter to be updated.
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// parameter values to be updated.
	// if set nil, the parameter defined by the key field will be deleted from the configuration file.
	// +kubebuilder:validation:Required
	Value *string `json:"value"`
}

type ParameterConfig struct {
	// key indicates the key name of ConfigMap.
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// Setting the list of parameters for a single configuration file.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Parameters []ParameterPair `json:"parameters"`
}

type Expose struct {
	ComponentOps `json:",inline"`

	// Setting the list of services to be exposed.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minitems=0
	Services []ClusterComponentService `json:"services"`
}

type RestoreFromSpec struct {
	// use the backup name and component name for restore, support for multiple components' recovery.
	// +optional
	Backup []BackupRefSpec `json:"backup,omitempty"`

	// specified the point in time to recovery
	// +optional
	PointInTime *PointInTimeRefSpec `json:"pointInTime,omitempty"`
}

type RefNamespaceName struct {
	// specified the name
	// +optional
	Name string `json:"name,omitempty"`

	// specified the namespace
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

type BackupRefSpec struct {
	// specify a reference backup to restore
	// +optional
	Ref RefNamespaceName `json:"ref,omitempty"`
}

type PointInTimeRefSpec struct {
	// specify the time point to restore, with UTC as the time zone.
	// +optional
	Time *metav1.Time `json:"time,omitempty"`

	// specify a reference source cluster to restore
	// +optional
	Ref RefNamespaceName `json:"ref,omitempty"`
}

// OpsRequestStatus defines the observed state of OpsRequest
type OpsRequestStatus struct {

	// ClusterGeneration records the cluster generation after handling the opsRequest action.
	// +optional
	ClusterGeneration int64 `json:"clusterGeneration,omitempty"`

	// phase describes OpsRequest phase.
	Phase OpsPhase `json:"phase,omitempty"`

	// +kubebuilder:validation:Pattern:=`^(\d+|\-)/(\d+|\-)$`
	// +kubebuilder:default=-/-
	Progress string `json:"progress"`

	// lastConfiguration records the last configuration before this operation take effected.
	// +optional
	LastConfiguration LastConfiguration `json:"lastConfiguration,omitempty"`

	// components defines the recorded the status information of changed components for operation request.
	// +optional
	Components map[string]OpsRequestComponentStatus `json:"components,omitempty"`

	// startTimestamp The time when the OpsRequest started processing.
	// +optional
	StartTimestamp metav1.Time `json:"startTimestamp,omitempty"`

	// completionTimestamp defines the OpsRequest completion time.
	// +optional
	CompletionTimestamp metav1.Time `json:"completionTimestamp,omitempty"`

	// reconfiguringStatus defines the status information of reconfiguring.
	// +optional
	ReconfiguringStatus *ReconfiguringStatus `json:"reconfiguringStatus,omitempty"`

	// conditions describes opsRequest detail status.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type ProgressStatusDetail struct {
	// group describes which group the current object belongs to.
	// if the objects of a component belong to the same group, we can ignore it.
	// +optional
	Group string `json:"group,omitempty"`

	// objectKey is the unique key of the object.
	// +kubebuilder:validation:Required
	ObjectKey string `json:"objectKey"`

	// status describes the state of processing the object.
	// +kubebuilder:validation:Required
	Status ProgressStatus `json:"status"`

	// message is a human readable message indicating details about the object condition.
	// +optional
	Message string `json:"message,omitempty"`

	// startTime is the start time of object processing.
	// +optional
	StartTime metav1.Time `json:"startTime,omitempty"`

	// endTime is the completion time of object processing.
	// +optional
	EndTime metav1.Time `json:"endTime,omitempty"`
}

type LastComponentConfiguration struct {
	// replicas are the last replicas of the component.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// the last resources of the component.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	corev1.ResourceRequirements `json:",inline,omitempty"`

	// classDefRef reference class defined in ComponentClassDefinition.
	// +optional
	ClassDefRef *ClassDefRef `json:"classDefRef,omitempty"`

	// volumeClaimTemplates records the last volumeClaimTemplates of the component.
	// +optional
	VolumeClaimTemplates []OpsRequestVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty"`

	// services records the last services of the component.
	// +optional
	Services []ClusterComponentService `json:"services,omitempty"`

	// targetResources records the affecting target resources information for the component.
	// resource key is in list of [pods].
	// +optional
	TargetResources map[ComponentResourceKey][]string `json:"targetResources,omitempty"`
}

type LastConfiguration struct {
	// clusterVersionRef references ClusterVersion name.
	// +optional
	ClusterVersionRef string `json:"clusterVersionRef,omitempty"`

	// components records last configuration of the component.
	// +optional
	Components map[string]LastComponentConfiguration `json:"components,omitempty"`
}

type OpsRequestComponentStatus struct {
	// phase describes the component phase, reference Cluster.status.component.phase.
	// +optional
	Phase ClusterComponentPhase `json:"phase,omitempty"`

	// progressDetails describes the progress details of the component for this operation.
	// +optional
	ProgressDetails []ProgressStatusDetail `json:"progressDetails,omitempty"`

	// workloadType references workload type of component in ClusterDefinition.
	// +optional
	WorkloadType WorkloadType `json:"workloadType,omitempty"`
}

type ReconfiguringStatus struct {
	// configurationStatus describes the status of the component reconfiguring.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ConfigurationStatus []ConfigurationStatus `json:"configurationStatus"`
}

type ConfigurationStatus struct {
	// name is a config template name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// updatePolicy describes the policy of reconfiguring.
	// +optional
	UpdatePolicy UpgradePolicy `json:"updatePolicy,omitempty"`

	// status describes the current state of the reconfiguring state machine.
	// +optional
	Status string `json:"status,omitempty"`

	// succeedCount describes the number of successful reconfiguring.
	// +kubebuilder:default=0
	// +optional
	SucceedCount int32 `json:"succeedCount"`

	// expectedCount describes the number of expected reconfiguring.
	// +kubebuilder:default=-1
	// +optional
	ExpectedCount int32 `json:"expectedCount"`

	// lastStatus describes the last status for the reconfiguring controller.
	// +optional
	LastAppliedStatus string `json:"lastStatus,omitempty"`

	// LastAppliedConfiguration describes the last configuration.
	// +optional
	LastAppliedConfiguration map[string]string `json:"lastAppliedConfiguration,omitempty"`

	// updatedParameters describes the updated parameters.
	// +optional
	UpdatedParameters UpdatedParameters `json:"updatedParameters"`
}

type UpdatedParameters struct {
	// addedKeys describes the key added.
	// +optional
	AddedKeys map[string]string `json:"addedKeys,omitempty"`

	// deletedKeys describes the key deleted.
	// +optional
	DeletedKeys map[string]string `json:"deletedKeys,omitempty"`

	// updatedKeys describes the key updated.
	// +optional
	UpdatedKeys map[string]string `json:"updatedKeys,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all},shortName=ops
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.type",description="Operation request type."
// +kubebuilder:printcolumn:name="CLUSTER",type="string",JSONPath=".spec.clusterRef",description="Operand cluster."
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="Operation status phase."
// +kubebuilder:printcolumn:name="PROGRESS",type="string",JSONPath=".status.progress",description="Operation processing progress."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// OpsRequest is the Schema for the opsrequests API
type OpsRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpsRequestSpec   `json:"spec,omitempty"`
	Status OpsRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpsRequestList contains a list of OpsRequest
type OpsRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpsRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpsRequest{}, &OpsRequestList{})
}

// GetRestartComponentNameSet gets the component name map with restart operation.
func (r OpsRequestSpec) GetRestartComponentNameSet() ComponentNameSet {
	set := make(ComponentNameSet)
	for _, v := range r.RestartList {
		set[v.ComponentName] = struct{}{}
	}
	return set
}

// GetVerticalScalingComponentNameSet gets the component name map with vertical scaling operation.
func (r OpsRequestSpec) GetVerticalScalingComponentNameSet() ComponentNameSet {
	set := make(ComponentNameSet)
	for _, v := range r.VerticalScalingList {
		set[v.ComponentName] = struct{}{}
	}
	return set
}

// ToVerticalScalingListToMap converts OpsRequest.spec.verticalScaling list to map
func (r OpsRequestSpec) ToVerticalScalingListToMap() map[string]VerticalScaling {
	verticalScalingMap := make(map[string]VerticalScaling)
	for _, v := range r.VerticalScalingList {
		verticalScalingMap[v.ComponentName] = v
	}
	return verticalScalingMap
}

// GetHorizontalScalingComponentNameSet gets the component name map with horizontal scaling operation.
func (r OpsRequestSpec) GetHorizontalScalingComponentNameSet() ComponentNameSet {
	set := make(ComponentNameSet)
	for _, v := range r.HorizontalScalingList {
		set[v.ComponentName] = struct{}{}
	}
	return set
}

// ToHorizontalScalingListToMap converts OpsRequest.spec.horizontalScaling list to map
func (r OpsRequestSpec) ToHorizontalScalingListToMap() map[string]HorizontalScaling {
	verticalScalingMap := make(map[string]HorizontalScaling)
	for _, v := range r.HorizontalScalingList {
		verticalScalingMap[v.ComponentName] = v
	}
	return verticalScalingMap
}

// GetVolumeExpansionComponentNameSet gets the component name map with volume expansion operation.
func (r OpsRequestSpec) GetVolumeExpansionComponentNameSet() ComponentNameSet {
	set := make(ComponentNameSet)
	for _, v := range r.VolumeExpansionList {
		set[v.ComponentName] = struct{}{}
	}
	return set
}

// ToVolumeExpansionListToMap converts volumeExpansionList to map
func (r OpsRequestSpec) ToVolumeExpansionListToMap() map[string]VolumeExpansion {
	volumeExpansionMap := make(map[string]VolumeExpansion)
	for _, v := range r.VolumeExpansionList {
		volumeExpansionMap[v.ComponentName] = v
	}
	return volumeExpansionMap
}

// ToExposeListToMap build expose map
func (r OpsRequestSpec) ToExposeListToMap() map[string]Expose {
	exposeMap := make(map[string]Expose)
	for _, v := range r.ExposeList {
		exposeMap[v.ComponentName] = v
	}
	return exposeMap
}

// GetReconfiguringComponentNameSet gets the component name map with reconfiguring operation.
func (r OpsRequestSpec) GetReconfiguringComponentNameSet() ComponentNameSet {
	if r.Reconfigure == nil {
		return nil
	}
	return ComponentNameSet{
		r.Reconfigure.ComponentName: {},
	}
}

func (r OpsRequestSpec) GetExposeComponentNameSet() ComponentNameSet {
	set := make(ComponentNameSet)
	for _, v := range r.ExposeList {
		set[v.ComponentName] = struct{}{}
	}
	return set
}

// GetUpgradeComponentNameSet gets the component name map with upgrade operation.
func (r *OpsRequest) GetUpgradeComponentNameSet() ComponentNameSet {
	if r == nil || r.Spec.Upgrade == nil {
		return nil
	}
	set := make(ComponentNameSet)
	for k := range r.Status.Components {
		set[k] = struct{}{}
	}
	return set
}

// GetComponentNameSet if the operations are within the scope of component, this function should be implemented
func (r *OpsRequest) GetComponentNameSet() ComponentNameSet {
	switch r.Spec.Type {
	case RestartType:
		return r.Spec.GetRestartComponentNameSet()
	case VerticalScalingType:
		return r.Spec.GetVerticalScalingComponentNameSet()
	case HorizontalScalingType:
		return r.Spec.GetHorizontalScalingComponentNameSet()
	case VolumeExpansionType:
		return r.Spec.GetVolumeExpansionComponentNameSet()
	case UpgradeType:
		return r.GetUpgradeComponentNameSet()
	case ReconfiguringType:
		return r.Spec.GetReconfiguringComponentNameSet()
	case ExposeType:
		return r.Spec.GetExposeComponentNameSet()
	default:
		return nil
	}
}

func (p *ProgressStatusDetail) SetStatusAndMessage(status ProgressStatus, message string) {
	p.Message = message
	p.Status = status
}
