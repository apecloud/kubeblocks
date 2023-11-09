/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

// TODO: @wangyelei could refactor to ops group

// OpsRequestSpec defines the desired state of OpsRequest
// +kubebuilder:validation:XValidation:rule="has(self.cancel) && self.cancel ? (self.type in ['VerticalScaling', 'HorizontalScaling']) : true",message="forbidden to cancel the opsRequest which type not in ['VerticalScaling','HorizontalScaling']"
type OpsRequestSpec struct {
	// clusterRef references clusterDefinition.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.clusterRef"
	ClusterRef string `json:"clusterRef"`

	// cancel defines the action to cancel the Pending/Creating/Running opsRequest, supported types: [VerticalScaling, HorizontalScaling].
	// once cancel is set to true, this opsRequest will be canceled and modifying this property again will not take effect.
	// +optional
	Cancel bool `json:"cancel,omitempty"`

	// type defines the operation type.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.type"
	Type OpsType `json:"type"`

	// ttlSecondsAfterSucceed OpsRequest will be deleted after TTLSecondsAfterSucceed second when OpsRequest.status.phase is Succeed.
	// +optional
	TTLSecondsAfterSucceed int32 `json:"ttlSecondsAfterSucceed,omitempty"`

	// upgrade specifies the cluster version by specifying clusterVersionRef.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.upgrade"
	Upgrade *Upgrade `json:"upgrade,omitempty"`

	// Deprecate: replace by update cluster command

	// horizontalScaling defines what component need to horizontal scale the specified replicas.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.horizontalScaling"
	HorizontalScalingList []HorizontalScaling `json:"horizontalScaling,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// Deprecate: replace by update cluster command.
	// Note: Quantity struct can not do immutable check by CEL.

	// volumeExpansion defines what component and volumeClaimTemplate need to expand the specified storage.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	VolumeExpansionList []VolumeExpansion `json:"volumeExpansion,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// restart the specified components.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.restart"
	RestartList []ComponentOps `json:"restart,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// switchover the specified components.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.switchover"
	SwitchoverList []Switchover `json:"switchover,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// Deprecate: replace by update cluster command.
	// Note: Quantity struct can not do immutable check by CEL.

	// verticalScaling defines what component need to vertical scale the specified compute resources.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	VerticalScalingList []VerticalScaling `json:"verticalScaling,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// reconfigure defines the variables that need to input when updating configuration.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.reconfigure"
	// +kubebuilder:validation:XValidation:rule="self.configurations.size() > 0", message="Value can not be empty"
	Reconfigure *Reconfigure `json:"reconfigure,omitempty"`

	// expose defines services the component needs to expose.
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.expose"
	ExposeList []Expose `json:"expose,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// cluster RestoreFrom backup or point in time
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.restoreFrom"
	RestoreFrom *RestoreFromSpec `json:"restoreFrom,omitempty"`

	// ttlSecondsBeforeAbort OpsRequest will wait at most TTLSecondsBeforeAbort seconds for start-conditions to be met.
	// If not specified, the default value is 0, which means that the start-conditions must be met immediately.
	// +kubebuilder:default=0
	// +optional
	TTLSecondsBeforeAbort *int32 `json:"ttlSecondsBeforeAbort,omitempty"`

	// scriptSpec defines the script to be executed.
	// +optional
	ScriptSpec *ScriptSpec `json:"scriptSpec,omitempty"`

	// backupSpec defines how to backup the cluster.
	// +optional
	BackupSpec *BackupSpec `json:"backupSpec,omitempty"`

	// restoreSpec defines how to restore the cluster.
	// +optional
	RestoreSpec *RestoreSpec `json:"restoreSpec,omitempty"`
}

// ComponentOps defines the common variables of component scope operations.
type ComponentOps struct {
	// componentName cluster component name.
	// +kubebuilder:validation:Required
	ComponentName string `json:"componentName"`
}

type Switchover struct {
	ComponentOps `json:",inline"`

	// instanceName is used to specify the candidate primary or leader instanceName for switchover.
	// If instanceName is set to "*", it means that no specific primary or leader is specified for the switchover,
	// and the switchoverAction defined in clusterDefinition.componentDefs[x].switchoverSpec.withoutCandidate will be executed,
	// It is required that clusterDefinition.componentDefs[x].switchoverSpec.withoutCandidate is not empty.
	// If instanceName is set to a valid instanceName other than "*", it means that a specific candidate primary or leader is specified for the switchover.
	// the value of instanceName can be obtained using `kbcli cluster list-instances`, any other value is invalid.
	// In this case, the `switchoverAction` defined in clusterDefinition.componentDefs[x].switchoverSpec.withCandidate will be executed,
	// and it is required that clusterDefinition.componentDefs[x].switchoverSpec.withCandidate is not empty.
	// +kubebuilder:validation:Required
	InstanceName string `json:"instanceName"`
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

// Reconfigure defines the variables that need to input when updating configuration.
type Reconfigure struct {
	ComponentOps `json:",inline"`

	// configurations defines which components perform the operation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Configurations []ConfigurationItem `json:"configurations" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

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

type ConfigurationItem struct {
	// name is a config template name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// policy defines the upgrade policy.
	// +optional
	Policy *UpgradePolicy `json:"policy,omitempty"`

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
	// +optional
	Value *string `json:"value"`
}

type ParameterConfig struct {
	// key indicates the key name of ConfigMap.
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// Setting the list of parameters for a single configuration file.
	// update specified the parameters.
	// +optional
	Parameters []ParameterPair `json:"parameters,omitempty"`

	// fileContent indicates the configuration file content.
	// update whole file.
	// +optional
	FileContent string `json:"fileContent,omitempty"`
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

// ScriptSpec defines the script to be executed. It is not a general purpose script executor.
// It is designed to execute the script to perform some specific operations, such as create database, create user, etc.
// It is applicable for engines, such as MySQL, PostgreSQL, Redis, MongoDB, etc.
type ScriptSpec struct {
	ComponentOps `json:",inline"`
	// exec command with image, by default use the image of kubeblocks-datascript.
	// +optional
	Image string `json:"image,omitempty"`
	// secret defines the secret to be used to execute the script.
	// If not specified, the default cluster root credential secret will be used.
	// +optional
	Secret *ScriptSecret `json:"secret,omitempty"`
	// script defines the script to be executed.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.scriptSpec.script"
	Script []string `json:"script,omitempty"`
	// scriptFrom defines the script to be executed from configMap or secret.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.scriptSpec.scriptFrom"
	ScriptFrom *ScriptFrom `json:"scriptFrom,omitempty"`
	// KubeBlocks, by default, will execute the script on the primary pod, with role=leader.
	// There are some exceptions, such as Redis, which does not synchronize accounts info between primary and secondary.
	// In this case, we need to execute the script on all pods, matching the selector.
	// selector indicates the components on which the script is executed.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.scriptSpec.script.selector"
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

type BackupSpec struct {
	// backupName is the name of the backup.
	// +optional
	BackupName string `json:"backupName,omitempty"`

	// Which backupPolicy is applied to perform this backup
	// +optional
	BackupPolicyName string `json:"backupPolicyName,omitempty"`

	// Backup method name that is defined in backupPolicy.
	// +optional
	BackupMethod string `json:"backupMethod,omitempty"`

	// deletionPolicy determines whether the backup contents stored in backup repository
	// should be deleted when the backup custom resource is deleted.
	// Supported values are "Retain" and "Delete".
	// "Retain" means that the backup content and its physical snapshot on backup repository are kept.
	// "Delete" means that the backup content and its physical snapshot on backup repository are deleted.
	// +kubebuilder:validation:Enum=Delete;Retain
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Delete
	// +optional
	DeletionPolicy string `json:"deletionPolicy,omitempty"`

	// retentionPeriod determines a duration up to which the backup should be kept.
	// Controller will remove all backups that are older than the RetentionPeriod.
	// For example, RetentionPeriod of `30d` will keep only the backups of last 30 days.
	// Sample duration format:
	// - years: 	2y
	// - months: 	6mo
	// - days: 		30d
	// - hours: 	12h
	// - minutes: 	30m
	// You can also combine the above durations. For example: 30d12h30m.
	// If not set, the backup will be kept forever.
	// +optional
	RetentionPeriod string `json:"retentionPeriod,omitempty"`

	// if backupType is incremental, parentBackupName is required.
	// +optional
	ParentBackupName string `json:"parentBackupName,omitempty"`
}

type RestoreSpec struct {
	// backupName is the name of the backup.
	// +kubebuilder:validation:Required
	BackupName string `json:"backupName"`

	// restoreTime point in time to restore
	RestoreTimeStr string `json:"restoreTimeStr,omitempty"`

	// the volume claim restore policy, support values: [Serial, Parallel]
	// +kubebuilder:validation:Enum=Serial;Parallel
	// +kubebuilder:default=Serial
	VolumeRestorePolicy string `json:"volumeRestorePolicy,omitempty"`
}

// ScriptSecret defines the secret to be used to execute the script.
type ScriptSecret struct {
	// name is the name of the secret.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`
	// usernameKey field is used to specify the username of the secret.
	// +kubebuilder:default:="username"
	// +optional
	UsernameKey string `json:"usernameKey,omitempty"`
	// passwordKey field is used to specify the password of the secret.
	// +kubebuilder:default:="password"
	// +optional
	PasswordKey string `json:"passwordKey,omitempty"`
}

// ScriptFrom defines the script to be executed from configMap or secret.
type ScriptFrom struct {
	// configMapRef defines the configMap to be executed.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.scriptSpec.scriptFrom.configMapRef"
	ConfigMapRef []corev1.ConfigMapKeySelector `json:"configMapRef,omitempty"`
	// secretRef defines the secret to be executed.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.scriptSpec.scriptFrom.secretRef"
	SecretRef []corev1.SecretKeySelector `json:"secretRef,omitempty"`
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

	// CancelTimestamp defines cancel time.
	// +optional
	CancelTimestamp metav1.Time `json:"cancelTimestamp,omitempty"`

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

	// lastFailedTime is the last time the component phase transitioned to Failed or Abnormal.
	// +optional
	LastFailedTime metav1.Time `json:"lastFailedTime,omitempty"`

	// progressDetails describes the progress details of the component for this operation.
	// +optional
	ProgressDetails []ProgressStatusDetail `json:"progressDetails,omitempty"`

	// workloadType references workload type of component in ClusterDefinition.
	// +optional
	WorkloadType WorkloadType `json:"workloadType,omitempty"`

	// reason describes the reason for the component phase.
	// +kubebuilder:validation:MaxLength=1024
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,5,opt,name=reason"`

	// message is a human-readable message indicating details about this operation.
	// +kubebuilder:validation:MaxLength=32768
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,6,opt,name=message"`
}

type ReconfiguringStatus struct {
	// configurationStatus describes the status of the component reconfiguring.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ConfigurationStatus []ConfigurationItemStatus `json:"configurationStatus"`
}

type ConfigurationItemStatus struct {
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

	// message describes the details about this operation.
	// +optional
	Message string `json:"message,omitempty"`

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

// +genclient
// +k8s:openapi-gen=true
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

// GetSwitchoverComponentNameSet gets the component name map with switchover operation.
func (r OpsRequestSpec) GetSwitchoverComponentNameSet() ComponentNameSet {
	set := make(ComponentNameSet)
	for _, v := range r.SwitchoverList {
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

// GetDataScriptComponentNameSet gets the component name map with switchover operation.
func (r OpsRequestSpec) GetDataScriptComponentNameSet() ComponentNameSet {
	set := make(ComponentNameSet)
	set[r.ScriptSpec.ComponentName] = struct{}{}
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
	case SwitchoverType:
		return r.Spec.GetSwitchoverComponentNameSet()
	case DataScriptType:
		return r.Spec.GetDataScriptComponentNameSet()
	default:
		return nil
	}
}

func (p *ProgressStatusDetail) SetStatusAndMessage(status ProgressStatus, message string) {
	p.Message = message
	p.Status = status
}
