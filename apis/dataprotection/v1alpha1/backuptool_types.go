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

// BackupToolSpec defines the desired state of BackupTool
type BackupToolSpec struct {
	// Backup tool Container image name.
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// which kind for run a backup tool, supported values: job, statefulSet.
	// +kubebuilder:default=job
	DeployKind DeployKind `json:"deployKind,omitempty"`

	// the type of backup tool, file or pitr
	// +kubebuilder:validation:Enum={file,pitr}
	// +kubebuilder:default=file
	Type string `json:"type,omitempty"`

	// Compute Resources required by this container.
	// Cannot be updated.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

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

	// Array of command that apps can do database backup.
	// from invoke args
	// the order of commands follows the order of array.
	// +kubebuilder:validation:Required
	BackupCommands []string `json:"backupCommands"`

	// Array of command that apps can do database incremental backup.
	// like xtrabackup, that can performs an incremental backup file.
	// +optional
	IncrementalBackupCommands []string `json:"incrementalBackupCommands,omitempty"`

	// backup tool can support physical restore, in this case, restore must be RESTART database.
	// +optional
	Physical *PhysicalConfig `json:"physical,omitempty"`

	// backup tool can support logical restore, in this case, restore NOT RESTART database.
	// +optional
	Logical *LogicalConfig `json:"logical,omitempty"`
}

type LogicalConfig struct {
	BackupToolRestoreCommand `json:",inline"`

	// podScope defines the pod scope for restore from backup, supported values:
	// - 'All' will exec the restore command on all pods.
	// - 'ReadWrite' will pick a ReadWrite pod to exec the restore command.
	// +optional
	// +kubebuilder:default=All
	PodScope PodRestoreScope `json:"podScope,omitempty"`
}

type PhysicalConfig struct {
	BackupToolRestoreCommand `json:",inline"`

	// relyOnLogfile defines whether the current recovery relies on log files
	// +optional
	RelyOnLogfile bool `json:"relyOnLogfile,omitempty"`
}

// BackupToolRestoreCommand defines the restore commands of BackupTool
type BackupToolRestoreCommand struct {
	// Array of command that apps can perform database restore.
	// like xtrabackup, that can performs restore mysql from files.
	// +optional
	RestoreCommands []string `json:"restoreCommands"`

	// Array of incremental restore commands.
	// +optional
	IncrementalRestoreCommands []string `json:"incrementalRestoreCommands,omitempty"`
}

// BackupToolStatus defines the observed state of BackupTool
type BackupToolStatus struct {
	// TODO(dsj): define backup tool status.
}

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster

// BackupTool is the Schema for the backuptools API (defined by provider)
type BackupTool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupToolSpec   `json:"spec,omitempty"`
	Status BackupToolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupToolList contains a list of BackupTool
type BackupToolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BackupTool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupTool{}, &BackupToolList{})
}

func (physical *PhysicalConfig) GetPhysicalRestoreCommand() []string {
	if physical == nil {
		return nil
	}
	return physical.RestoreCommands
}

func (physical *PhysicalConfig) IsRelyOnLogfile() bool {
	return physical != nil && physical.RelyOnLogfile
}

func (logical *LogicalConfig) GetLogicalRestoreCommand() []string {
	if logical == nil {
		return nil
	}
	return logical.RestoreCommands
}
