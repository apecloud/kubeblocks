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

// BackupPolicyTemplateSpec contains the settings in a BackupPolicyTemplate.
type BackupPolicyTemplateSpec struct {
	// Specifies the name of a ClusterDefinition.
	// This is an immutable attribute that cannot be changed after creation.
	// And this field is deprecated since v0.9, consider using the ComponentDef instead.
	//
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="clusterDefinitionRef is immutable"
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0, consider using the ComponentDef instead"
	ClusterDefRef string `json:"clusterDefinitionRef,omitempty"`

	// Represents an array of BackupPolicy templates, with each template corresponding to a specified ComponentDefinition
	// or to a group of ComponentDefinitions that are different versions of definitions of the same component.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	BackupPolicies []BackupPolicy `json:"backupPolicies"`

	// Specifies a unique identifier for the BackupPolicyTemplate.
	//
	// This identifier will be used as the suffix of the name of automatically generated BackupPolicy.
	// This prevents unintended overwriting of BackupPolicies due to name conflicts when multiple BackupPolicyTemplates
	// are present.
	// For instance, using "backup-policy" for regular backups and "backup-policy-hscale" for horizontal-scale ops
	// can differentiate the policies.
	//
	// +optional
	// +kubebuilder:validation:MaxLength=20
	Identifier string `json:"identifier,omitempty"`
}

// BackupPolicy is the template corresponding to a specified ComponentDefinition
// or to a group of ComponentDefinitions that are different versions of definitions of the same component.
type BackupPolicy struct {
	// Specifies the name of ClusterComponentDefinition defined in the ClusterDefinition.
	// Must comply with the IANA Service Naming rule.
	//
	// Deprecated since v0.9, should use `componentDefs` instead.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	// +optional
	ComponentDefRef string `json:"componentDefRef,omitempty"`

	// Specifies a list of names of ComponentDefinitions that the specified ClusterDefinition references.
	// They should be different versions of definitions of the same component,
	// thus allowing them to share a single BackupPolicy.
	// Each name must adhere to the IANA Service Naming rule.
	//
	// +optional
	ComponentDefs []string `json:"componentDefs,omitempty"`

	// Defines the selection criteria of instance to be backed up, and the connection credential to be used
	// during the backup process.
	//
	// +optional
	Target TargetInstance `json:"target"`

	// Defines the execution plans for backup tasks, specifying when and how backups should occur,
	// and the retention period of backup files.
	//
	// +optional
	Schedules []SchedulePolicy `json:"schedules,omitempty"`

	// Defines an array of BackupMethods to be used.
	//
	// +kubebuilder:validation:Required
	BackupMethods []BackupMethod `json:"backupMethods"`

	// Specifies the maximum number of retry attempts for a backup before it is considered a failure.
	//
	// +optional
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`
}

type BackupMethod struct {
	// Specifies the name of dataprotection.BackupMethod.
	dpv1alpha1.BackupMethod `json:",inline"`

	// If set, specifies the method for selecting the replica to be backed up using the criteria defined here.
	// If this field is not set, the selection method specified in `backupPolicy.target` is used.
	//
	// This field provides a way to override the global `backupPolicy.target` setting for specific BackupMethod.
	//
	// +optional
	Target *TargetInstance `json:"target"`

	// Specifies a mapping of an environment variable key to the appropriate version of the tool image
	// required for backups, as determined by ClusterVersion and ComponentDefinition.
	// The environment variable is then injected into the container executing the backup task.
	//
	// +optional
	EnvMapping []EnvMappingVar `json:"envMapping,omitempty"`
}

type EnvMappingVar struct {
	// Specifies the environment variable key in the mapping.
	//
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// Specifies the source used to derive the value of the environment variable,
	// which typically represents the tool image required for backup operation.
	//
	// +kubebuilder:validation:Required
	ValueFrom ValueFrom `json:"valueFrom"`
}

type ValueFrom struct {
	// Determine the appropriate version of the backup tool image from ClusterVersion.
	//
	// Deprecated since v0.9, since ClusterVersion is deprecated.
	// +optional
	ClusterVersionRef []ValueMapping `json:"clusterVersionRef,omitempty"`

	// Determine the appropriate version of the backup tool image from ComponentDefinition.
	//
	// +optional
	ComponentDef []ValueMapping `json:"componentDef,omitempty"`
}

type ValueMapping struct {
	// Represents an array of names of ClusterVersion or ComponentDefinition that can be mapped to
	// the appropriate version of the backup tool image.
	//
	// This mapping allows different versions of component images to correspond to specific versions of backup tool images.
	//
	// +kubebuilder:validation:Required
	Names []string `json:"names"`

	// Specifies the appropriate version of the backup tool image.
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
	// Specifies the role to select one or more replicas for backup.
	//
	// - If no replica with the specified role exists, the backup task will fail.
	//   Special case: If there is only one replica in the cluster, it will be used for backup,
	//   even if its role differs from the specified one.
	//   For example, if you specify backing up on a secondary replica, but the cluster is single-node
	//   with only one primary replica, the primary will be used for backup.
	//   Future versions will address this special case using role priorities.
	// - If multiple replicas satisfy the specified role, the choice (`Any` or `All`) will be made according to
	//   the `strategy` field below.
	Role string `json:"role"`

	// If `backupPolicy.componentDefs` is set, this field is required to specify the system account name.
	// This account must match one listed in `componentDefinition.spec.systemAccounts[*].name`.
	// The corresponding secret created by this account is used to connect to the database.
	//
	// If `backupPolicy.componentDefRef` (a legacy and deprecated API) is set, the secret defined in
	// `clusterDefinition.spec.ConnectionCredential` is used instead.
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

	// Specifies the keys of the connection credential secret defined in `clusterDefinition.spec.ConnectionCredential`.
	// It will be ignored when the `account` is set.
	//
	// +optional
	ConnectionCredentialKey ConnectionCredentialKey `json:"connectionCredentialKey,omitempty"`
}

type ConnectionCredentialKey struct {
	// Represents the key of the password in the connection credential secret.
	// If not specified, the default key "password" is used.
	//
	// +optional
	PasswordKey *string `json:"passwordKey,omitempty"`

	// Represents the key of the username in the connection credential secret.
	// If not specified, the default key "username" is used.
	//
	// +optional
	UsernameKey *string `json:"usernameKey,omitempty"`

	// Defines the key of the host in the connection credential secret.
	HostKey *string `json:"hostKey,omitempty"`

	// Indicates map key of the port in the connection credential secret.
	PortKey *string `json:"portKey,omitempty"`
}

// BackupPolicyTemplateStatus defines the observed state of BackupPolicyTemplate.
type BackupPolicyTemplateStatus struct {
}

// BackupPolicyTemplate should be provided by addon developers and is linked to a ClusterDefinition
// and its associated ComponentDefinitions.
// It is responsible for generating BackupPolicies for Components that require backup operations,
// also determining the suitable backup methods and strategies.
// This template is automatically selected based on the specified ClusterDefinition and ComponentDefinitions
// when a Cluster is created.
//
// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=bpt
// +kubebuilder:printcolumn:name="CLUSTER-DEFINITION",type="string",JSONPath=".spec.clusterDefinitionRef",description="ClusterDefinition referenced by cluster."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
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
