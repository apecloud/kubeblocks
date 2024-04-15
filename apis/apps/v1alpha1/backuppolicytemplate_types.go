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

// BackupPolicyTemplateSpec defines the desired state of BackupPolicyTemplate
type BackupPolicyTemplateSpec struct {
	// Specifies a reference to the ClusterDefinition name. This is an immutable attribute that cannot be changed after creation.
	//
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="clusterDefinitionRef is immutable"
	ClusterDefRef string `json:"clusterDefinitionRef"`

	// Represents an array of backup policy templates for the specified ComponentDefinition.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	BackupPolicies []BackupPolicy `json:"backupPolicies"`

	// Acts as a unique identifier for this BackupPolicyTemplate. This identifier will be used as a suffix for the automatically generated backupPolicy name.
	// It is required when multiple BackupPolicyTemplates exist to prevent backupPolicy override.
	//
	// +optional
	// +kubebuilder:validation:MaxLength=20
	Identifier string `json:"identifier,omitempty"`
}

type BackupPolicy struct {
	// References a componentDef defined in the ClusterDefinition spec.
	// Must comply with the IANA Service Naming rule.
	//
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	// +optional
	ComponentDefRef string `json:"componentDefRef,omitempty"`

	// Specifies that this componentDef is a shading component definition.
	// +optional
	IsSharding bool `json:"isSharding,omitempty"`

	// References to componentDefinitions.
	// Must comply with the IANA Service Naming rule.
	//
	// +optional
	ComponentDefs []string `json:"componentDefs,omitempty"`

	// The instance to be backed up.
	//
	// +optional
	Target TargetInstance `json:"target"`

	// Define the policy for backup scheduling.
	//
	// +optional
	Schedules []SchedulePolicy `json:"schedules,omitempty"`

	// Define the methods to be used for backups.
	//
	// +kubebuilder:validation:Required
	BackupMethods []BackupMethod `json:"backupMethods"`

	// Specifies the number of retries before marking the backup as failed.
	//
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`
}

type BackupMethod struct {
	// Method for backup
	dpv1alpha1.BackupMethod `json:",inline"`

	// Specifies the instance where the backup will be stored.
	//
	// +optional
	Target *TargetInstance `json:"target"`

	// Defines the mapping between the environment variables of the cluster and the keys of the environment values.
	//
	// +optional
	EnvMapping []EnvMappingVar `json:"envMapping,omitempty"`
}

type EnvMappingVar struct {
	// Specifies the environment variable key that requires mapping.
	//
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// Defines the source from which the environment variable value is derived.
	//
	// +kubebuilder:validation:Required
	ValueFrom ValueFrom `json:"valueFrom"`
}

type ValueFrom struct {
	// Maps to the environment value. This is an optional field.
	//
	// +optional
	ClusterVersionRef []ValueMapping `json:"clusterVersionRef,omitempty"`

	// Maps to the environment value. This is also an optional field.
	//
	// +optional
	ComponentDef []ValueMapping `json:"componentDef,omitempty"`
}

type ValueMapping struct {
	// Represents an array of ClusterVersion names that can be mapped to an environment variable value.
	//
	// +kubebuilder:validation:Required
	Names []string `json:"names"`

	// The value that corresponds to the specified ClusterVersion names.
	//
	// +kubebuilder:validation:Required
	MappingValue string `json:"mappingValue"`
}

type SchedulePolicy struct {

	// Specifies whether the backup schedule is enabled or not.
	//
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Defines the backup method name that is defined in backupPolicy.
	//
	// +kubebuilder:validation:Required
	BackupMethod string `json:"backupMethod"`

	// Represents the cron expression for schedule, with the timezone set in UTC.
	// Refer to https://en.wikipedia.org/wiki/Cron for more details.
	//
	// +kubebuilder:validation:Required
	CronExpression string `json:"cronExpression"`

	// Determines the duration for which the backup should be retained.
	// The controller will remove all backups that are older than the RetentionPeriod.
	// For instance, a RetentionPeriod of `30d` will retain only the backups from the last 30 days.
	// Sample duration format:
	//
	// - years: 	2y
	// - months: 	6mo
	// - days: 		30d
	// - hours: 	12h
	// - minutes: 	30m
	//
	// These durations can also be combined, for example: 30d12h30m.
	//
	// +optional
	// +kubebuilder:default="7d"
	RetentionPeriod dpv1alpha1.RetentionPeriod `json:"retentionPeriod,omitempty"`
}

type TargetInstance struct {
	// Specifies the instance of the corresponding role for backup. The roles can be:
	//
	// - Leader, Follower, or Leaner for the Consensus component.
	// - Primary or Secondary for the Replication component.
	//
	// Invalid roles of the component will be ignored. For example, if the workload type is Replication and the component's replicas is 1,
	// the secondary role is invalid. It will also be ignored when the component is Stateful or Stateless.
	//
	// The role will be transformed into a role LabelSelector for the BackupPolicy's target attribute.
	Role string `json:"role"`

	// Refers to spec.componentDef.systemAccounts.accounts[*].name in the ClusterDefinition.
	// The secret created by this account will be used to connect to the database.
	// If not set, the secret created by spec.ConnectionCredential of the ClusterDefinition will be used.
	//
	// It will be transformed into a secret for the BackupPolicy's target secret.
	//
	// +optional
	Account string `json:"account,omitempty"`

	// Specifies the PodSelectionStrategy to use when multiple pods are
	// selected for the backup target.
	// Valid values are:
	//
	// - Any: Selects any one pod that matches the labelsSelector.
	// - All: Selects all pods that match the labelsSelector.
	//
	// +optional
	Strategy dpv1alpha1.PodSelectionStrategy `json:"strategy,omitempty"`

	// Defines the connection credential key in the secret
	// created by spec.ConnectionCredential of the ClusterDefinition.
	// It will be ignored when the "account" is set.
	//
	// +optional
	ConnectionCredentialKey ConnectionCredentialKey `json:"connectionCredentialKey,omitempty"`
}

type ConnectionCredentialKey struct {
	// Represents the key of the password in the ConnectionCredential secret.
	// If not specified, the default key "password" is used.
	//
	// +optional
	PasswordKey *string `json:"passwordKey,omitempty"`

	// Represents the key of the username in the ConnectionCredential secret.
	// If not specified, the default key "username" is used.
	//
	// +optional
	UsernameKey *string `json:"usernameKey,omitempty"`

	// Defines the map key of the host in the connection credential secret.
	HostKey *string `json:"hostKey,omitempty"`

	// Indicates the map key of the port in the connection credential secret.
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
	// The metadata for the API version and kind of the BackupPolicyTemplate.
	metav1.TypeMeta `json:",inline"`

	// The metadata for the BackupPolicyTemplate object, including name, namespace, labels, and annotations.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Defines the desired state of the BackupPolicyTemplate.
	Spec BackupPolicyTemplateSpec `json:"spec,omitempty"`

	// Populated by the system, it represents the current information about the BackupPolicyTemplate.
	Status BackupPolicyTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupPolicyTemplateList contains a list of BackupPolicyTemplate
type BackupPolicyTemplateList struct {
	// Contains the metadata for the API objects, including the Kind and Version of the object.
	metav1.TypeMeta `json:",inline"`

	// Contains the metadata for the list objects, including the continue and remainingItemCount for the list.
	metav1.ListMeta `json:"metadata,omitempty"`

	// Contains the list of BackupPolicyTemplate.
	Items []BackupPolicyTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupPolicyTemplate{}, &BackupPolicyTemplateList{})
}
