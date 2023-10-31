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

// ActionSetSpec defines the desired state of ActionSet
type ActionSetSpec struct {
	// backupType specifies the backup type, supported values: Full, Continuous.
	// Full means full backup.
	// Incremental means back up data that have changed since the last backup (full or incremental).
	// Differential means back up data that have changed since the last full backup.
	// Continuous will back up the transaction log continuously, the PITR (Point in Time Recovery).
	// can be performed based on the continuous backup and full backup.
	// +kubebuilder:validation:Enum={Full,Incremental,Differential,Continuous}
	// +kubebuilder:default=Full
	// +kubebuilder:validation:Required
	BackupType BackupType `json:"backupType"`

	// List of environment variables to set in the container.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// List of sources to populate environment variables in the container.
	// The keys defined within a source must be a C_IDENTIFIER. All invalid keys
	// will be reported as an event when the container is starting. When a key exists in multiple
	// sources, the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	// Cannot be updated.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// backup specifies the backup action.
	// +optional
	Backup *BackupActionSpec `json:"backup,omitempty"`

	// restore specifies the restore action.
	// +optional
	Restore *RestoreActionSpec `json:"restore,omitempty"`
}

// ActionSetStatus defines the observed state of ActionSet
type ActionSetStatus struct {
	// phase - in list of [Available,Unavailable]
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// A human-readable message indicating details about why the ActionSet is in this phase.
	// +optional
	Message string `json:"message,omitempty"`

	// generation number
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// BackupType the backup type.
// +enum
// +kubebuilder:validation:Enum={Full,Incremental,Differential,Continuous}
type BackupType string

const (
	BackupTypeFull         BackupType = "Full"
	BackupTypeIncremental  BackupType = "Incremental"
	BackupTypeDifferential BackupType = "Differential"
	BackupTypeContinuous   BackupType = "Continuous"
)

type BackupActionSpec struct {
	// backupData specifies the backup data action.
	// +kubebuilder:validation:Required
	BackupData *BackupDataActionSpec `json:"backupData,omitempty"`

	// preBackup specifies a hook that should be executed before the backup.
	// +optional
	PreBackup []ActionSpec `json:"preBackup,omitempty"`

	// postBackup specifies a hook that should be executed after the backup.
	// +optional
	PostBackup []ActionSpec `json:"postBackup,omitempty"`
}

// BackupDataActionSpec defines how to back up data.
type BackupDataActionSpec struct {
	JobActionSpec `json:",inline"`

	// syncProgress specifies whether to sync the backup progress and its interval seconds.
	// +optional
	SyncProgress *SyncProgress `json:"syncProgress,omitempty"`
}

type SyncProgress struct {
	// enabled specifies whether to sync the backup progress. If enabled,
	// a sidecar container will be created to sync the backup progress to the
	// Backup CR status.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// intervalSeconds specifies the interval seconds to sync the backup progress.
	// +optional
	// +kubebuilder:default=60
	IntervalSeconds *int32 `json:"intervalSeconds,omitempty"`
}

// RestoreActionSpec defines how to restore data.
type RestoreActionSpec struct {
	// prepareData specifies the action to prepare data.
	// +optional
	PrepareData *JobActionSpec `json:"prepareData,omitempty"`

	// postReady specifies the action to execute after the data is ready.
	// +optional
	PostReady []ActionSpec `json:"postReady,omitempty"`
}

// ActionSpec defines an action that should be executed. Only one of the fields may be set.
type ActionSpec struct {
	// exec specifies the action should be executed by the pod exec API in a container.
	// +optional
	Exec *ExecActionSpec `json:"exec,omitempty"`

	// job specifies the action should be executed by a Kubernetes Job.
	// +optional
	Job *JobActionSpec `json:"job,omitempty"`
}

// ExecActionSpec is an action that uses the pod exec API to execute a command in a container
// in a pod.
type ExecActionSpec struct {
	// container is the container in the pod where the command should be executed.
	// If not specified, the pod's first container is used.
	// +optional
	Container string `json:"container,omitempty"`

	// Command is the command and arguments to execute.
	// +kubebuilder:validation:MinItems=1
	Command []string `json:"command"`

	// OnError specifies how should behave if it encounters an error executing this action.
	// +optional
	// +kubebuilder:default=Fail
	OnError ActionErrorMode `json:"onError,omitempty"`

	// Timeout defines the maximum amount of time should wait for the hook to complete before
	// considering the execution a failure.
	// +optional
	Timeout metav1.Duration `json:"timeout,omitempty"`
}

// JobActionSpec is an action that creates a Kubernetes Job to execute a command.
type JobActionSpec struct {
	// image specifies the image of backup container.
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// runOnTargetPodNode specifies whether to run the job workload on the
	// target pod node. If backup container should mount the target pod's
	// volumes, this field should be set to true. otherwise the target pod's
	// volumes will be ignored.
	// +optional
	// +kubebuilder:default=false
	RunOnTargetPodNode *bool `json:"runOnTargetPodNode,omitempty"`

	// command specifies the commands to back up the volume data.
	// +kubebuilder:validation:Required
	Command []string `json:"command"`

	// OnError specifies how should behave if it encounters an error executing
	// this action.
	// +optional
	// +kubebuilder:default=Fail
	OnError ActionErrorMode `json:"onError,omitempty"`
}

// ActionErrorMode defines how should treat an error from an action.
// +kubebuilder:validation:Enum=Continue;Fail
type ActionErrorMode string

const (
	// ActionErrorModeContinue means that an error from an action is acceptable.
	ActionErrorModeContinue ActionErrorMode = "Continue"

	// ActionErrorModeFail means that an error from an action is problematic.
	ActionErrorModeFail ActionErrorMode = "Fail"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=as
// +kubebuilder:printcolumn:name="BACKUP-TYPE",type="string",JSONPath=".spec.backupType"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ActionSet is the Schema for the actionsets API
type ActionSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ActionSetSpec   `json:"spec,omitempty"`
	Status ActionSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ActionSetList contains a list of ActionSet
type ActionSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ActionSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ActionSet{}, &ActionSetList{})
}

func (r *ActionSet) HasPrepareDataStage() bool {
	if r == nil || r.Spec.Restore == nil {
		return false
	}
	return r.Spec.Restore.PrepareData != nil
}

func (r *ActionSet) HasPostReadyStage() bool {
	if r == nil || r.Spec.Restore == nil {
		return false
	}
	return len(r.Spec.Restore.PostReady) > 0
}
