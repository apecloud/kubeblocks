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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupSpec defines the desired state of Backup.
type BackupSpec struct {
	// Which backupPolicy is applied to perform this backup.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.backupPolicyName"
	BackupPolicyName string `json:"backupPolicyName"`

	// backupMethod specifies the backup method name that is defined in backupPolicy.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.backupMethod"
	BackupMethod string `json:"backupMethod"`

	// deletionPolicy determines whether the backup contents stored in backup repository
	// should be deleted when the backup custom resource is deleted.
	// Supported values are "Retain" and "Delete".
	// "Retain" means that the backup can not be deleted and remains in 'Deleting' phase.
	// "Delete" means that the backup content and its physical snapshot on backup repository are deleted.
	// +kubebuilder:validation:Enum=Delete;Retain
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Delete
	DeletionPolicy BackupDeletionPolicy `json:"deletionPolicy,omitempty"`

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
	RetentionPeriod RetentionPeriod `json:"retentionPeriod,omitempty"`

	// parentBackupName determines the parent backup name for incremental or
	// differential backup.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.parentBackupName"
	ParentBackupName string `json:"parentBackupName,omitempty"`
}

// BackupStatus defines the observed state of Backup.
type BackupStatus struct {
	// formatVersion is the backup format version, including major, minor and patch version.
	// +optional
	FormatVersion string `json:"formatVersion,omitempty"`

	// phase is the current state of the Backup.
	// +optional
	Phase BackupPhase `json:"phase,omitempty"`

	// expiration is when this backup is eligible for garbage collection.
	// 'null' means the Backup will NOT be cleaned except delete manual.
	// +optional
	Expiration *metav1.Time `json:"expiration,omitempty"`

	// startTimestamp records the time a backup was started.
	// The server's time is used for StartTimestamp.
	// +optional
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// completionTimestamp records the time a backup was completed.
	// Completion time is recorded even on failed backups.
	// The server's time is used for CompletionTimestamp.
	// +optional
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// The duration time of backup execution.
	// When converted to a string, the format is "1h2m0.5s".
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty"`

	// totalSize is the total size of backed up data size.
	// A string with capacity units in the format of "1Gi", "1Mi", "1Ki".
	// If no capacity unit is specified, it is assumed to be in bytes.
	// +optional
	TotalSize string `json:"totalSize,omitempty"`

	// failureReason is an error that caused the backup to fail.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// backupRepoName is the name of the backup repository.
	// +optional
	BackupRepoName string `json:"backupRepoName,omitempty"`

	// path is the directory inside the backup repository where the backup data is stored.
	// It is an absolute path in the backup repository.
	// +optional
	Path string `json:"path,omitempty"`

	// persistentVolumeClaimName is the name of the persistent volume claim that
	// is used to store the backup data.
	// +optional
	PersistentVolumeClaimName string `json:"persistentVolumeClaimName,omitempty"`

	// timeRange records the time range of backed up data, for PITR, this is the
	// time range of recoverable data.
	// +optional
	TimeRange *BackupTimeRange `json:"timeRange,omitempty"`

	// target records the target information for this backup.
	// +optional
	Target *BackupTarget `json:"target,omitempty"`

	// backupMethod records the backup method information for this backup.
	// Refer to BackupMethod for more details.
	// +optional
	BackupMethod *BackupMethod `json:"backupMethod,omitempty"`

	// actions records the actions information for this backup.
	// +optional
	Actions []ActionStatus `json:"actions,omitempty"`

	// volumeSnapshots records the volume snapshot status for the action.
	// +optional
	VolumeSnapshots []VolumeSnapshotStatus `json:"volumeSnapshots,omitempty"`
}

// BackupTimeRange records the time range of backed up data, for PITR, this is the
// time range of recoverable data.
type BackupTimeRange struct {
	// start records the start time of backup.
	// +optional
	Start *metav1.Time `json:"start,omitempty"`

	// end records the end time of backup.
	// +optional
	End *metav1.Time `json:"end,omitempty"`
}

// BackupDeletionPolicy describes a policy for end-of-life maintenance of backup content.
// +enum
// +kubebuilder:validation:Enum={Delete,Retain}
type BackupDeletionPolicy string

const (
	BackupDeletionPolicyDelete BackupDeletionPolicy = "Delete"
	BackupDeletionPolicyRetain BackupDeletionPolicy = "Retain"
)

// BackupPhase is a string representation of the lifecycle phase of a Backup.
// +enum
// +kubebuilder:validation:Enum={New,InProgress,Running,Completed,Failed,Deleting}
type BackupPhase string

const (
	// BackupPhaseNew means the backup has been created but not yet processed by
	// the BackupController.
	BackupPhaseNew BackupPhase = "New"

	// BackupPhaseRunning means the backup is currently executing.
	BackupPhaseRunning BackupPhase = "Running"

	// BackupPhaseCompleted means the backup has run successfully without errors.
	BackupPhaseCompleted BackupPhase = "Completed"

	// BackupPhaseFailed means the backup ran but encountered an error that
	// prevented it from completing successfully.
	BackupPhaseFailed BackupPhase = "Failed"

	// BackupPhaseDeleting means the backup and all its associated data are being deleted.
	BackupPhaseDeleting BackupPhase = "Deleting"
)

type ActionStatus struct {
	// name is the name of the action.
	Name string `json:"name,omitempty"`

	// phase is the current state of the action.
	// +optional
	Phase ActionPhase `json:"phase,omitempty"`

	// startTimestamp records the time an action was started.
	// +optional
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// completionTimestamp records the time an action was completed.
	// +optional
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// failureReason is an error that caused the backup to fail.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// actionType is the type of the action.
	// +optional
	ActionType ActionType `json:"actionType,omitempty"`

	// availableReplicas available replicas for statefulSet action.
	// +optional
	AvailableReplicas *int32 `json:"availableReplicas,omitempty"`

	// objectRef is the object reference for the action.
	// +optional
	ObjectRef *corev1.ObjectReference `json:"objectRef,omitempty"`

	// totalSize is the total size of backed up data size.
	// A string with capacity units in the format of "1Gi", "1Mi", "1Ki".
	// +optional
	TotalSize string `json:"totalSize,omitempty"`

	// timeRange records the time range of backed up data, for PITR, this is the
	// time range of recoverable data.
	// +optional
	TimeRange *BackupTimeRange `json:"timeRange,omitempty"`

	// volumeSnapshots records the volume snapshot status for the action.
	// +optional
	VolumeSnapshots []VolumeSnapshotStatus `json:"volumeSnapshots,omitempty"`
}

type VolumeSnapshotStatus struct {
	// name is the name of the volume snapshot.
	Name string `json:"name,omitempty"`

	// contentName is the name of the volume snapshot content.
	ContentName string `json:"contentName,omitempty"`

	// volumeName is the name of the volume.
	// +optional
	VolumeName string `json:"volumeName,omitempty"`

	// size is the size of the volume snapshot.
	// +optional
	Size string `json:"size,omitempty"`
}

type ActionPhase string

const (
	// ActionPhaseNew means the action has been created but not yet processed by
	// the BackupController.
	ActionPhaseNew ActionPhase = "New"

	// ActionPhaseRunning means the action is currently executing.
	ActionPhaseRunning ActionPhase = "Running"

	// ActionPhaseCompleted means the action has run successfully without errors.
	ActionPhaseCompleted ActionPhase = "Completed"

	// ActionPhaseFailed means the action ran but encountered an error that
	ActionPhaseFailed ActionPhase = "Failed"
)

type ActionType string

const (
	ActionTypeJob         ActionType = "Job"
	ActionTypeStatefulSet ActionType = "StatefulSet"
	ActionTypeNone        ActionType = ""
)

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Namespaced
// +kubebuilder:printcolumn:name="POLICY",type=string,JSONPath=`.spec.backupPolicyName`
// +kubebuilder:printcolumn:name="METHOD",type=string,JSONPath=`.spec.backupMethod`
// +kubebuilder:printcolumn:name="REPO",type=string,JSONPath=`.status.backupRepoName`
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="TOTAL-SIZE",type=string,JSONPath=`.status.totalSize`
// +kubebuilder:printcolumn:name="DURATION",type=string,JSONPath=`.status.duration`
// +kubebuilder:printcolumn:name="CREATION-TIME",type=string,JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="COMPLETION-TIME",type=string,JSONPath=`.status.completionTimestamp`
// +kubebuilder:printcolumn:name="EXPIRATION-TIME",type=string,JSONPath=`.status.expiration`

// Backup is the Schema for the backups API.
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupSpec   `json:"spec,omitempty"`
	Status BackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupList contains a list of Backup.
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backup{}, &BackupList{})
}

// GetStartTime gets the backup start time. Default return status.startTimestamp,
// unless status.timeRange.startTime is not nil.
func (r *Backup) GetStartTime() *metav1.Time {
	s := r.Status
	if s.TimeRange != nil && s.TimeRange.Start != nil {
		return s.TimeRange.Start
	}
	return s.StartTimestamp
}

// GetEndTime gets the backup end time. Default return status.completionTimestamp,
// unless status.timeRange.endTime is not nil.
func (r *Backup) GetEndTime() *metav1.Time {
	s := r.Status
	if s.TimeRange != nil && s.TimeRange.End != nil {
		return s.TimeRange.End
	}
	return s.CompletionTimestamp
}
