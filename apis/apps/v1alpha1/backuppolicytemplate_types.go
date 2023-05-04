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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupPolicyTemplateSpec defines the desired state of BackupPolicyTemplate
type BackupPolicyTemplateSpec struct {
	// clusterDefinitionRef references ClusterDefinition name, this is an immutable attribute.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	ClusterDefRef string `json:"clusterDefinitionRef"`

	// backupPolicies is a list of backup policy template for the specified componentDefinition.
	// +patchMergeKey=componentDefRef
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentDefRef
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	BackupPolicies []BackupPolicy `json:"backupPolicies"`

	// Identifier is a unique identifier for this BackupPolicyTemplate.
	// this identifier will be the suffix of the automatically generated backupPolicy name.
	// and must be added when multiple BackupPolicyTemplates exist,
	// otherwise the generated backupPolicy override will occur.
	// +optional
	// +kubebuilder:validation:MaxLength=20
	Identifier string `json:"identifier,omitempty"`
}

type BackupPolicy struct {
	// componentDefRef references componentDef defined in ClusterDefinition spec.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	ComponentDefRef string `json:"componentDefRef"`

	// ttl is a time string ending with the 'd'|'D'|'h'|'H' character to describe how long
	// the Backup should be retained. if not set, will be retained forever.
	// +kubebuilder:validation:Pattern:=`^\d+[d|D|h|H]$`
	// +optional
	TTL *string `json:"ttl,omitempty"`

	// schedule policy for backup.
	// +optional
	Schedule Schedule `json:"schedule,omitempty"`

	// the policy for snapshot backup.
	// +optional
	Snapshot *SnapshotPolicy `json:"snapshot,omitempty"`

	// the policy for full backup.
	// +optional
	Full *CommonBackupPolicy `json:"full,omitempty"`

	// the policy for incremental backup.
	// +optional
	Incremental *CommonBackupPolicy `json:"incremental,omitempty"`
}

type Schedule struct {
	// schedule policy for base backup.
	// +optional
	BaseBackup *BaseBackupSchedulePolicy `json:"baseBackup,omitempty"`

	// schedule policy for incremental backup.
	// +optional
	Incremental *SchedulePolicy `json:"incremental,omitempty"`
}

type BaseBackupSchedulePolicy struct {
	SchedulePolicy `json:",inline"`
	// the type of base backup, only support full and snapshot.
	// +kubebuilder:validation:Required
	Type BaseBackupType `json:"type"`
}

type SchedulePolicy struct {
	// the cron expression for schedule, the timezone is in UTC. see https://en.wikipedia.org/wiki/Cron.
	// +kubebuilder:validation:Required
	CronExpression string `json:"cronExpression"`

	// enable or disable the schedule.
	// +kubebuilder:validation:Required
	Enable bool `json:"enable"`
}

type SnapshotPolicy struct {
	BasePolicy `json:",inline"`

	// execute hook commands for backup.
	// +optional
	Hooks *BackupPolicyHook `json:"hooks,omitempty"`
}

type CommonBackupPolicy struct {
	BasePolicy `json:",inline"`

	// which backup tool to perform database backup, only support one tool.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	BackupToolName string `json:"backupToolName,omitempty"`
}

type BasePolicy struct {
	// target instance for backup.
	// +optional
	Target TargetInstance `json:"target"`

	// the number of automatic backups to retain. Value must be non-negative integer.
	// 0 means NO limit on the number of backups.
	// +kubebuilder:default=7
	// +optional
	BackupsHistoryLimit int32 `json:"backupsHistoryLimit,omitempty"`

	// count of backup stop retries on fail.
	// +optional
	OnFailAttempted int32 `json:"onFailAttempted,omitempty"`

	// define how to update metadata for backup status.
	// +optional
	BackupStatusUpdates []BackupStatusUpdate `json:"backupStatusUpdates,omitempty"`
}

type TargetInstance struct {
	// select instance of corresponding role for backup, role are:
	// - the name of Leader/Follower/Leaner for Consensus component.
	// - primary or secondary for Replication component.
	// finally, invalid role of the component will be ignored.
	// such as if workload type is Replication and component's replicas is 1,
	// the secondary role is invalid. and it also will be ignored when component is Stateful/Stateless.
	// the role will be transformed to a role LabelSelector for BackupPolicy's target attribute.
	// +optional
	Role string `json:"role"`

	// refer to spec.componentDef.systemAccounts.accounts[*].name in ClusterDefinition.
	// the secret created by this account will be used to connect the database.
	// if not set, the secret created by spec.ConnectionCredential of the ClusterDefinition will be used.
	// it will be transformed to a secret for BackupPolicy's target secret.
	// +optional
	Account string `json:"account,omitempty"`

	// connectionCredentialKey defines connection credential key in secret
	// which created by spec.ConnectionCredential of the ClusterDefinition.
	// it will be ignored when "account" is set.
	ConnectionCredentialKey ConnectionCredentialKey `json:"connectionCredentialKey,omitempty"`
}

type ConnectionCredentialKey struct {
	// the key of password in the ConnectionCredential secret.
	// if not set, the default key is "password".
	// +optional
	PasswordKey *string `json:"passwordKey,omitempty"`

	// the key of username in the ConnectionCredential secret.
	// if not set, the default key is "username".
	// +optional
	UsernameKey *string `json:"usernameKey,omitempty"`
}

// BackupPolicyHook defines for the database execute commands before and after backup.
type BackupPolicyHook struct {
	// pre backup to perform commands
	// +optional
	PreCommands []string `json:"preCommands,omitempty"`

	// post backup to perform commands
	// +optional
	PostCommands []string `json:"postCommands,omitempty"`

	// exec command with image
	// +optional
	Image string `json:"image,omitempty"`

	// which container can exec command
	// +optional
	ContainerName string `json:"containerName,omitempty"`
}

type BackupStatusUpdate struct {
	// specify the json path of backup object for patch.
	// example: manifests.backupLog -- means patch the backup json path of status.manifests.backupLog.
	// +optional
	Path string `json:"path,omitempty"`

	// which container name that kubectl can execute.
	// +optional
	ContainerName string `json:"containerName,omitempty"`

	// the shell Script commands to collect backup status metadata.
	// The script must exist in the container of ContainerName and the output format must be set to JSON.
	// Note that outputting to stderr may cause the result format to not be in JSON.
	// +optional
	Script string `json:"script,omitempty"`

	// when to update the backup status, pre: before backup, post: after backup
	// +optional
	UpdateStage BackupStatusUpdateStage `json:"updateStage,omitempty"`
}

// BackupPolicyTemplateStatus defines the observed state of BackupPolicyTemplate
type BackupPolicyTemplateStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=bpt
// +kubebuilder:printcolumn:name="CLUSTER-DEFINITION",type="string",JSONPath=".spec.clusterDefinitionRef",description="ClusterDefinition referenced by cluster."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// BackupPolicyTemplate is the Schema for the BackupPolicyTemplates API (defined by provider)
type BackupPolicyTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupPolicyTemplateSpec   `json:"spec,omitempty"`
	Status BackupPolicyTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupPolicyTemplateList contains a list of BackupPolicyTemplate
type BackupPolicyTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BackupPolicyTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupPolicyTemplate{}, &BackupPolicyTemplateList{})
}
