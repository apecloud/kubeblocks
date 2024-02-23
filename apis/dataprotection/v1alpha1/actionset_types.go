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

// ActionSetSpec defines the desired state of ActionSet
type ActionSetSpec struct {
	// Specifies the backup type. Supported values include:
	//
	// - `Full` for a full backup.
	// - `Incremental` back up data that have changed since the last backup (either full or incremental).
	// - `Differential` back up data that has changed since the last full backup.
	// - `Continuous` back up transaction logs continuously, such as MySQL binlog, PostgreSQL WAL, etc.
	//
	// Continuous backup is essential for implementing Point-in-Time Recovery (PITR).
	//
	// +kubebuilder:validation:Enum={Full,Incremental,Differential,Continuous}
	// +kubebuilder:default=Full
	// +kubebuilder:validation:Required
	BackupType BackupType `json:"backupType"`

	// Specifies a list of environment variables to be set in the container.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// Specifies a list of sources to populate environment variables in the container.
	// The keys within a source must be a C_IDENTIFIER. Any invalid keys will be
	// reported as an event when the container starts. If a key exists in multiple
	// sources, the value from the last source will take precedence. Any values
	// defined by an Env with a duplicate key will take precedence.
	//
	// This field cannot be updated.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// Specifies the backup action.
	//
	// +optional
	Backup *BackupActionSpec `json:"backup,omitempty"`

	// Specifies the restore action.
	//
	// +optional
	Restore *RestoreActionSpec `json:"restore,omitempty"`
}

// ActionSetStatus defines the observed state of ActionSet
type ActionSetStatus struct {
	// Indicates the phase of the ActionSet. This can be either 'Available' or 'Unavailable'.
	//
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Provides a human-readable explanation detailing the reason for the current phase of the ActionSet.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// Represents the generation number that has been observed by the controller.
	//
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
	// Represents the action to be performed for backing up data.
	//
	// +kubebuilder:validation:Required
	BackupData *BackupDataActionSpec `json:"backupData,omitempty"`

	// Represents a set of actions that should be executed before the backup process begins.
	//
	// +optional
	PreBackup []ActionSpec `json:"preBackup,omitempty"`

	// Represents a set of actions that should be executed after the backup process has completed.
	//
	// +optional
	PostBackup []ActionSpec `json:"postBackup,omitempty"`

	// Represents a custom deletion action that can be executed before the built-in deletion action.
	// Note: The preDelete action job will ignore the env/envFrom.
	//
	// +optional
	PreDeleteBackup *BaseJobActionSpec `json:"preDelete,omitempty"`
}

// BackupDataActionSpec defines how to back up data.
type BackupDataActionSpec struct {
	JobActionSpec `json:",inline"`

	// Determines if the backup progress should be synchronized and the interval
	// for synchronization in seconds.
	//
	// +optional
	SyncProgress *SyncProgress `json:"syncProgress,omitempty"`
}

type SyncProgress struct {
	// Determines if the backup progress should be synchronized. If set to true,
	// a sidecar container will be instantiated to synchronize the backup progress with the
	// Backup Custom Resource (CR) status.
	//
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Defines the interval in seconds for synchronizing the backup progress.
	//
	// +optional
	// +kubebuilder:default=60
	IntervalSeconds *int32 `json:"intervalSeconds,omitempty"`
}

// RestoreActionSpec defines how to restore data.
type RestoreActionSpec struct {
	// Specifies the action required to prepare data for restoration.
	//
	// +optional
	PrepareData *JobActionSpec `json:"prepareData,omitempty"`

	// Specifies the actions that should be executed after the data has been prepared and is ready for restoration.
	//
	// +optional
	PostReady []ActionSpec `json:"postReady,omitempty"`
}

// ActionSpec defines an action that should be executed. Only one of the fields may be set.
type ActionSpec struct {
	// Specifies that the action should be executed using the pod's exec API within a container.
	//
	// +optional
	Exec *ExecActionSpec `json:"exec,omitempty"`

	// Specifies that the action should be executed by a Kubernetes Job.
	//
	// +optional
	Job *JobActionSpec `json:"job,omitempty"`
}

// ExecActionSpec is an action that uses the pod exec API to execute a command in a container
// in a pod.
type ExecActionSpec struct {
	// Specifies the container within the pod where the command should be executed.
	// If not specified, the first container in the pod is used by default.
	//
	// +optional
	Container string `json:"container,omitempty"`

	// Defines the command and arguments to be executed.
	//
	// +kubebuilder:validation:MinItems=1
	Command []string `json:"command"`

	// Indicates how to behave if an error is encountered during the execution of this action.
	//
	// +optional
	// +kubebuilder:default=Fail
	OnError ActionErrorMode `json:"onError,omitempty"`

	// Specifies the maximum duration to wait for the hook to complete before
	// considering the execution a failure.
	//
	// +optional
	Timeout metav1.Duration `json:"timeout,omitempty"`
}

// JobActionSpec is an action that creates a Kubernetes Job to execute a command.
type JobActionSpec struct {
	BaseJobActionSpec `json:",inline"`

	// Determines whether to run the job workload on the target pod node.
	// If the backup container needs to mount the target pod's volumes, this field
	// should be set to true. Otherwise, the target pod's volumes will be ignored.
	//
	// +optional
	// +kubebuilder:default=false
	RunOnTargetPodNode *bool `json:"runOnTargetPodNode,omitempty"`

	// Indicates how to behave if an error is encountered during the execution of this action.
	//
	// +optional
	// +kubebuilder:default=Fail
	OnError ActionErrorMode `json:"onError,omitempty"`
}

// BaseJobActionSpec is an action that creates a Kubernetes Job to execute a command.
type BaseJobActionSpec struct {
	// Specifies the image of the backup container.
	//
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Defines the commands to back up the volume data.
	//
	// +kubebuilder:validation:Required
	Command []string `json:"command"`
}

// ActionErrorMode defines how to handle an error from an action.
// Currently, only the Fail mode is supported, but the Continue mode will be supported in the future.
//
// +kubebuilder:validation:Enum=Continue;Fail
type ActionErrorMode string

const (
	// ActionErrorModeContinue signifies that an error from an action is acceptable and can be ignored.
	ActionErrorModeContinue ActionErrorMode = "Continue"

	// ActionErrorModeFail signifies that an error from an action is problematic and should be treated as a failure.
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
