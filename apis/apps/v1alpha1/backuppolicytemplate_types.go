/*
Copyright ApeCloud, Inc.

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
}

type BackupPolicy struct {
	// componentDefRef references componentDef defined in ClusterDefinition spec.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	ComponentDefRef string `json:"componentDefRef"`

	// ttl is a time string in days and ending with the 'd' or 'D' character to describe how long
	// the Backup should be retained.
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
	// the type of base backup, only support full and incremental.
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

	// connectionCredentialKeyword defines connection credential keyword in secret
	// which created by spec.ConnectionCredential of the ClusterDefinition.
	// it will be ignored when "account" is set.
	ConnectionCredentialKeyword ConnectionCredentialKeyword `json:"connectionCredentialKeyword,omitempty"`
}

type ConnectionCredentialKeyword struct {
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
