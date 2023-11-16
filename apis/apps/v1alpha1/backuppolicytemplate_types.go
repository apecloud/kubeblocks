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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
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
	Schedules []SchedulePolicy `json:"schedules,omitempty"`

	// backupMethods defines the backup methods.
	// +kubebuilder:validation:Required
	BackupMethods []BackupMethod `json:"backupMethods"`
}

type BackupMethod struct {
	dpv1alpha1.BackupMethod `json:",inline"`

	// envMapping defines the variables of cluster mapped to env values' keys.
	// +optional
	EnvMapping []EnvMappingVar `json:"envMapping,omitempty"`
}

type EnvMappingVar struct {
	// env key which needs to mapping.
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// valueFrom defines source of the env value.
	// +kubebuilder:validation:Required
	ValueFrom ValueFrom `json:"valueFrom"`
}

type ValueFrom struct {
	// mapped ClusterVersionRef to env value.
	// +kubebuilder:validation:Required
	ClusterVersionRef []ClusterVersionMapping `json:"clusterVersionRef"`
}

type ClusterVersionMapping struct {
	// the array of ClusterVersion name which can be mapped to the env value.
	// +kubebuilder:validation:Required
	Names []string `json:"names"`

	// mapping value for the specified ClusterVersion names.
	// +kubebuilder:validation:Required
	MappingValue string `json:"mappingValue"`
}

type SchedulePolicy struct {
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
	RetentionPeriod dpv1alpha1.RetentionPeriod `json:"retentionPeriod,omitempty"`
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

	// hostKey specifies the map key of the host in the connection credential secret.
	HostKey *string `json:"hostKey,omitempty"`

	// portKey specifies the map key of the port in the connection credential secret.
	// +kubebuilder:default=port
	PortKey *string `json:"portKey,omitempty"`
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
