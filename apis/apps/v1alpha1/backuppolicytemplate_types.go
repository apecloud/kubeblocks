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

// BackupPolicyTemplateSpec defines the desired state of BackupPolicyTemplate
type BackupPolicyTemplateSpec struct {
	// clusterDefinitionRef references ClusterDefinition name, this is an immutable attribute.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="clusterDefinitionRef is immutable"
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
	// componentDefRef references componentDef defined in ClusterDefinition spec. Need to
	// comply with IANA Service Naming rule.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	ComponentDefRef string `json:"componentDefRef"`

	// target instance for backup.
	// +optional
	Target TargetInstance `json:"target"`

	// schedule policy for backup.
	// +optional
	Schedules []Schedule `json:"schedules,omitempty"`

	// backupMethods defines the backup methods.
	// +kubebuilder:validation:Required
	BackupMethods []BackupMethod `json:"backupMethods"`
}

// RetentionPeriod represents a duration in the format "1y2mo3w4d5h6m", where
// y=year, mo=month, w=week, d=day, h=hour, m=minute.
type RetentionPeriod string

type Schedule struct {
	// enabled specifies whether the backup schedule is enabled or not.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// backupMethod specifies the backup method name that is defined in backupPolicy.
	// +kubebuilder:validation:Required
	BackupMethod string `json:"backupMethod"`

	// the cron expression for schedule, the timezone is in UTC.
	// see https://en.wikipedia.org/wiki/Cron.
	// +kubebuilder:validation:Required
	CronExpression string `json:"cronExpression"`

	// the number of automatic backups to retain. Value must be non-negative integer.
	// 0 means NO limit on the number of backups.
	// +kubebuilder:default=7
	// +optional
	BackupsHistoryLimit int32 `json:"backupsHistoryLimit,omitempty"`

	// retentionPeriod determines a duration up to which the backup should be kept.
	// controller will remove all backups that are older than the RetentionPeriod.
	// For example, RetentionPeriod of `30d` will keep only the backups of last 30 days.
	// Sample duration format:
	// - years: 	2y
	// - months: 	6mo
	// - days: 		30d
	// - hours: 	12h
	// - minutes: 	30m
	// You can also combine the above durations. For example: 30d12h30m
	// +optional
	// +kubebuilder:default="7d"
	RetentionPeriod RetentionPeriod `json:"retentionPeriod,omitempty"`
}

// BackupMethod defines the backup method.
type BackupMethod struct {
	// the name of backup method.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// snapshotVolumes specifies whether to take snapshots of persistent volumes.
	// if true, the BackupScript is not required, the controller will use the CSI
	// volume snapshotter to create the snapshot.
	// +optional
	// +kubebuilder:default=false
	SnapshotVolumes *bool `json:"snapshotVolumes,omitempty"`

	// actionSetName refers to the ActionSet object that defines the backup actions.
	// For volume snapshot backup, the actionSet is not required, the controller
	// will use the CSI volume snapshotter to create the snapshot.
	// +optional
	ActionSetName string `json:"actionSetName,omitempty"`

	// targetVolumes specifies which volumes from the target should be mounted in
	// the backup workload.
	// +optional
	TargetVolumes *TargetVolumeInfo `json:"targetVolumes,omitempty"`

	// env specifies the environment variables for the backup workload.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// runtimeSettings specifies runtime settings for the backup workload container.
	// +optional
	RuntimeSettings *RuntimeSettings `json:"runtimeSettings,omitempty"`
}

type RuntimeSettings struct {
	// resources specifies the resource required by container.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// TargetVolumeInfo specifies the volumes and their mounts of the targeted application
// that should be mounted in backup workload.
type TargetVolumeInfo struct {
	// Volumes indicates the list of volumes of targeted application that should
	// be mounted on the backup job.
	// +optional
	Volumes []string `json:"volumes,omitempty"`

	// volumeMounts specifies the mount for the volumes specified in `Volumes` section.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
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

// BackupPolicyTemplateStatus defines the observed state of BackupPolicyTemplate
type BackupPolicyTemplateStatus struct {
}

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
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
