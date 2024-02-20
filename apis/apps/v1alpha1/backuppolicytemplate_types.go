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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

// BackupPolicyTemplateSpec defines the desired state of a BackupPolicyTemplate.
type BackupPolicyTemplateSpec struct {
	// Specifies a reference to the ClusterDefinition name. This is an immutable attribute.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="clusterDefinitionRef is immutable"
	ClusterDefRef string `json:"clusterDefinitionRef"`

	// Represents a list of backup policy templates for the specified ComponentDefinition.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	BackupPolicies []BackupPolicy `json:"backupPolicies"`

	// A unique identifier for this BackupPolicyTemplate.
	// This identifier will be used as the suffix of the automatically generated backupPolicy name.
	// It must be added when multiple BackupPolicyTemplates exist to prevent the generated backupPolicy from being overridden.
	// +optional
	// +kubebuilder:validation:MaxLength=20
	Identifier string `json:"identifier,omitempty"`
}

type BackupPolicy struct {
	// References a componentDef defined in the ClusterDefinition spec. Must comply with the IANA Service Naming rule.
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	// +optional
	ComponentDefRef string `json:"componentDefRef,omitempty"`

	// References to componentDefinitions. Must comply with the IANA Service Naming rule.
	// +optional
	ComponentDefs []string `json:"componentDefs,omitempty"`

	// The instance pod to be backed up.
	// +optional
	Target TargetInstance `json:"target"`

	// The policies for scheduling backups.
	// +optional
	Schedules []SchedulePolicy `json:"schedules,omitempty"`

	// The methods used for backups.
	// +kubebuilder:validation:Required
	BackupMethods []BackupMethod `json:"backupMethods"`

	// Specifies the number of retries before marking the backup as failed.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`
}

type BackupMethod struct {
	dpv1alpha1.BackupMethod `json:",inline"`

	// The instance to be backed up.
	// +optional
	Target *TargetInstance `json:"target"`

	// Defines the mapping of cluster variables to environment variable keys.
	// +optional
	EnvMapping []EnvMappingVar `json:"envMapping,omitempty"`
}

type EnvMappingVar struct {
	// The environment variable key that needs to be mapped.
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// Defines the source of the environment variable value.
	// +kubebuilder:validation:Required
	ValueFrom ValueFrom `json:"valueFrom"`
}

type ValueFrom struct {
	// Maps ClusterVersionRef to environment value.
	// +optional
	ClusterVersionRef []ValueMapping `json:"clusterVersionRef,omitempty"`

	// Maps ComponentDefinition to environment value.
	// +optional
	ComponentDef []ValueMapping `json:"componentDef,omitempty"`
}

type ValueMapping struct {
	// An array of ClusterVersion names that can be mapped to the environment value.
	// +kubebuilder:validation:Required
	Names []string `json:"names"`

	// The mapping value for the specified ClusterVersion names.
	// +kubebuilder:validation:Required
	MappingValue string `json:"mappingValue"`
}

type SchedulePolicy struct {
	// Specifies whether the backup schedule is enabled or not.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Specifies the backup method name that is defined in backupPolicy.
	// +kubebuilder:validation:Required
	BackupMethod string `json:"backupMethod"`

	// The cron expression for schedule, the timezone is in UTC.
	// Refer to https://en.wikipedia.org/wiki/Cron for more details.
	// +kubebuilder:validation:Required
	CronExpression string `json:"cronExpression"`

	// Determines a duration up to which the backup should be kept.
	// The controller will remove all backups that are older than the RetentionPeriod.
	// For example, a RetentionPeriod of `30d` will keep only the backups of the last 30 days.
	// The duration format is as follows:
	// - years: 2y
	// - months: 6mo
	// - days: 30d
	// - hours: 12h
	// - minutes: 30m
	// You can also combine the above durations. For example: 30d12h30m
	//
	// +optional
	// +kubebuilder:default="7d"
	RetentionPeriod dpv1alpha1.RetentionPeriod `json:"retentionPeriod,omitempty"`
}

type TargetInstance struct {
	// Specifies the role of the instance for backup. Valid roles include Leader, Follower, and Leaner for Consensus components,
	// and primary or secondary for Replication components.
	// Invalid roles will be ignored. For example, if the workload type is Replication
	// and the component's replicas is 1, the secondary role is invalid and will be ignored.
	// The role will be transformed into a LabelSelector for the BackupPolicy's target attribute.
	Role string `json:"role"`

	// Refers to spec.componentDef.systemAccounts.accounts[*].name in ClusterDefinition or spec.systemAccounts in ComponentDefinition.
	// The secret created by this account will be used to connect to the database.
	// If not set, the secret created by spec.ConnectionCredential of the ClusterDefinition will be used.
	// This will be transformed into a secret for the BackupPolicy's target secret.
	// +optional
	Account string `json:"account,omitempty"`

	// Specifies the strategy to use when multiple pods are selected for the backup target.
	// Valid values are:
	// - Any: select any one pod that matches the labelsSelector.
	// - All: select all pods that match the labelsSelector.
	// +optional
	Strategy dpv1alpha1.PodSelectionStrategy `json:"strategy,omitempty"`

	// Defines the connection credential key in the secret created by spec.ConnectionCredential of the ClusterDefinition.
	// This will be ignored when "account" is set.
	// +optional
	ConnectionCredentialKey ConnectionCredentialKey `json:"connectionCredentialKey,omitempty"`
}

type ConnectionCredentialKey struct {
	// Specifies the key of the password in the ConnectionCredential secret.
	// If not set, the default key is "password".
	// +optional
	PasswordKey *string `json:"passwordKey,omitempty"`

	// Specifies the key of the username in the ConnectionCredential secret.
	// If not set, the default key is "username".
	// +optional
	UsernameKey *string `json:"usernameKey,omitempty"`

	// Specifies the map key of the host in the connection credential secret.
	HostKey *string `json:"hostKey,omitempty"`

	// Specifies the map key of the port in the connection credential secret.
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
