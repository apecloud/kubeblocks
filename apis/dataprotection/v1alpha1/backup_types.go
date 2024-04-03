/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	// Specifies the backup policy to be applied for this backup.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.backupPolicyName"
	BackupPolicyName string `json:"backupPolicyName"`

	// Specifies the backup method name that is defined in the backup policy.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.backupMethod"
	BackupMethod string `json:"backupMethod"`

	// Determines whether the backup contents stored in the backup repository
	// should be deleted when the backup custom resource(CR) is deleted.
	// Supported values are `Retain` and `Delete`.
	//
	// - `Retain` means that the backup content and its physical snapshot on backup repository are kept.
	// - `Delete` means that the backup content and its physical snapshot on backup repository are deleted.
	//
	// TODO: for the retain policy, we should support in the future for only deleting
	//   the backup CR but retaining the backup contents in backup repository.
	//   The current implementation only prevent accidental deletion of backup data.
	//
	// +kubebuilder:validation:Enum=Delete;Retain
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Delete
	DeletionPolicy BackupDeletionPolicy `json:"deletionPolicy,omitempty"`

	// Determines a duration up to which the backup should be kept.
	// Controller will remove all backups that are older than the RetentionPeriod.
	// If not set, the backup will be kept forever.
	// For example, RetentionPeriod of `30d` will keep only the backups of last 30 days.
	// Sample duration format:
	//
	// - years: 	2y
	// - months: 	6mo
	// - days: 		30d
	// - hours: 	12h
	// - minutes: 	30m
	//
	// You can also combine the above durations. For example: 30d12h30m.
	//
	// +optional
	RetentionPeriod RetentionPeriod `json:"retentionPeriod,omitempty"`

	// Determines the parent backup name for incremental or differential backup.
	//
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.parentBackupName"
	ParentBackupName string `json:"parentBackupName,omitempty"`
}

// BackupStatus defines the observed state of Backup.
type BackupStatus struct {
	// Specifies the backup format version, which includes major, minor, and patch versions.
	//
	// +optional
	FormatVersion string `json:"formatVersion,omitempty"`

	// Indicates the current state of the backup operation.
	//
	// +optional
	Phase BackupPhase `json:"phase,omitempty"`

	// Indicates when this backup becomes eligible for garbage collection.
	// A 'null' value implies that the backup will not be cleaned up unless manually deleted.
	//
	// +optional
	Expiration *metav1.Time `json:"expiration,omitempty"`

	// Records the time when the backup operation was started.
	// The server's time is used for this timestamp.
	//
	// +optional
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// Records the time when the backup operation was completed.
	// This timestamp is recorded even if the backup operation fails.
	// The server's time is used for this timestamp.
	//
	// +optional
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// Records the duration of the backup operation.
	// When converted to a string, the format is "1h2m0.5s".
	//
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty"`

	// Records the total size of the data backed up.
	// The size is represented as a string with capacity units in the format of "1Gi", "1Mi", "1Ki".
	// If no capacity unit is specified, it is assumed to be in bytes.
	//
	// +optional
	TotalSize string `json:"totalSize,omitempty"`

	// Any error that caused the backup operation to fail.
	//
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// The name of the backup repository.
	//
	// +optional
	BackupRepoName string `json:"backupRepoName,omitempty"`

	// The directory within the backup repository where the backup data is stored.
	// This is an absolute path within the backup repository.
	//
	// +optional
	Path string `json:"path,omitempty"`

	// Records the path of the Kopia repository.
	//
	// +optional
	KopiaRepoPath string `json:"kopiaRepoPath,omitempty"`

	// Records the name of the persistent volume claim used to store the backup data.
	//
	// +optional
	PersistentVolumeClaimName string `json:"persistentVolumeClaimName,omitempty"`

	// Records the time range of the data backed up. For Point-in-Time Recovery (PITR),
	// this is the time range of recoverable data.
	//
	// +optional
	TimeRange *BackupTimeRange `json:"timeRange,omitempty"`

	// Records the target information for this backup.
	//
	// +optional
	Target *BackupStatusTarget `json:"target,omitempty"`

	// Records the targets information for this backup.
	//
	// +optional
	Targets []BackupStatusTarget `json:"targets,omitempty"`

	// Records the backup method information for this backup.
	// Refer to BackupMethod for more details.
	//
	// +optional
	BackupMethod *BackupMethod `json:"backupMethod,omitempty"`

	// Records the encryption config for this backup.
	//
	// +optional
	EncryptionConfig *EncryptionConfig `json:"encryptionConfig,omitempty"`

	// Records the actions status for this backup.
	//
	// +optional
	Actions []ActionStatus `json:"actions,omitempty"`

	// Records the volume snapshot status for the action.
	//
	// +optional
	VolumeSnapshots []VolumeSnapshotStatus `json:"volumeSnapshots,omitempty"`

	// Records any additional information for the backup.
	//
	// +optional
	Extras []map[string]string `json:"extras,omitempty"`
}

// BackupTimeRange records the time range of backed up data, for PITR, this is the
// time range of recoverable data.
type BackupTimeRange struct {
	// time zone, supports only zone offset, with a value range of "-12:59 ~ +13:00".
	//
	// +kubebuilder:validation:Pattern:=`^(\+|\-)(0[0-9]|1[0-3]):([0-5][0-9])$`
	// +optional
	TimeZone string `json:"timeZone,omitempty"`

	// Records the start time of the backup, in Coordinated Universal Time (UTC).
	//
	// +optional
	Start *metav1.Time `json:"start,omitempty"`

	// Records the end time of the backup, in Coordinated Universal Time (UTC).
	//
	// +optional
	End *metav1.Time `json:"end,omitempty"`
}

// BackupDeletionPolicy describes the policy for end-of-life maintenance of backup content.
// +enum
// +kubebuilder:validation:Enum={Delete,Retain}
type BackupDeletionPolicy string

const (
	BackupDeletionPolicyDelete BackupDeletionPolicy = "Delete"
	BackupDeletionPolicyRetain BackupDeletionPolicy = "Retain"
)

// BackupPhase describes the lifecycle phase of a Backup.
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
	// The name of the action.
	//
	// +optional
	Name string `json:"name,omitempty"`

	// Records the target pod name which has been backed up.
	TargetPodName string `json:"targetPodName,omitempty"`

	// The current phase of the action.
	//
	// +optional
	Phase ActionPhase `json:"phase,omitempty"`

	// Records the time an action was started.
	//
	// +optional
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// Records the time an action was completed.
	//
	// +optional
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// An error that caused the action to fail.
	//
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// The type of the action.
	//
	// +optional
	ActionType ActionType `json:"actionType,omitempty"`

	// Available replicas for statefulSet action.
	//
	// +optional
	AvailableReplicas *int32 `json:"availableReplicas,omitempty"`

	// The object reference for the action.
	//
	// +optional
	ObjectRef *corev1.ObjectReference `json:"objectRef,omitempty"`

	// The total size of backed up data size.
	// A string with capacity units in the format of "1Gi", "1Mi", "1Ki".
	// If no capacity unit is specified, it is assumed to be in bytes.
	//
	// +optional
	TotalSize string `json:"totalSize,omitempty"`

	// Records the time range of backed up data, for PITR, this is the time
	// range of recoverable data.
	//
	// +optional
	TimeRange *BackupTimeRange `json:"timeRange,omitempty"`

	// Records the volume snapshot status for the action.
	//
	// +optional
	VolumeSnapshots []VolumeSnapshotStatus `json:"volumeSnapshots,omitempty"`
}

type BackupStatusTarget struct {
	BackupTarget `json:",inline"`

	// Records the selected pods by the target info during backup.
	SelectedTargetPods []string `json:"selectedTargetPods,omitempty"`
}

type VolumeSnapshotStatus struct {
	// The name of the volume snapshot.
	//
	// +optional
	Name string `json:"name,omitempty"`

	// The name of the volume snapshot content.
	//
	// +optional
	ContentName string `json:"contentName,omitempty"`

	// The name of the volume.
	//
	// +optional
	VolumeName string `json:"volumeName,omitempty"`

	// The size of the volume snapshot.
	//
	// +optional
	Size string `json:"size,omitempty"`

	// Associates this volumeSnapshot with its corresponding target.
	TargetName string `json:"targetName,omitempty"`
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

func (r *Backup) GetTimeZone() string {
	s := r.Status
	if s.TimeRange != nil {
		return s.TimeRange.TimeZone
	}
	return ""
}
