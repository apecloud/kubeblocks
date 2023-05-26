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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupToolSpec defines the desired state of BackupTool
type BackupToolSpec struct {
	// Backup tool Container image name.
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// which kind for run a backup tool.
	// +kubebuilder:validation:Enum={job,daemon}
	// +kubebuilder:default=job
	DeployKind string `json:"deployKind,omitempty"`

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
	// +kubebuilder:validation:Required
	Physical BackupToolRestoreCommand `json:"physical"`

	// backup tool can support logical restore, in this case, restore NOT RESTART database.
	// +optional
	Logical *BackupToolRestoreCommand `json:"logical,omitempty"`
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
