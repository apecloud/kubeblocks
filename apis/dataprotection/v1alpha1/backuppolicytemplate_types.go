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
)

// BackupPolicyTemplateSpec contains the settings in a BackupPolicyTemplate.
type BackupPolicyTemplateSpec struct {

	// Defines the type of well-known service protocol that the BackupPolicyTemplate provides, and it is optional.
	// Some examples of well-known service protocols include:
	//
	// - "MySQL": Indicates that the Component provides a MySQL database service.
	// - "PostgreSQL": Indicates that the Component offers a PostgreSQL database service.
	// - "Redis": Signifies that the Component functions as a Redis key-value store.
	// - "ETCD": Denotes that the Component serves as an ETCD distributed key-value store
	//
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceKind string `json:"serviceKind,omitempty"`

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
	BackupMethods []BackupMethodTPL `json:"backupMethods"`

	// Specifies the maximum number of retry attempts for a backup before it is considered a failure.
	//
	// +optional
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`
}

type BackupMethodTPL struct {
	// The name of backup method.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Specifies whether to take snapshots of persistent volumes. If true,
	// the ActionSetName is not required, the controller will use the CSI volume
	// snapshotter to create the snapshot.
	//
	// +optional
	// +kubebuilder:default=false
	SnapshotVolumes *bool `json:"snapshotVolumes,omitempty"`

	// Refers to the ActionSet object that defines the backup actions.
	// For volume snapshot backup, the actionSet is not required, the controller
	// will use the CSI volume snapshotter to create the snapshot.
	// +optional
	ActionSetName string `json:"actionSetName,omitempty"`

	// Specifies which volumes from the target should be mounted in the backup workload.
	//
	// +optional
	TargetVolumes *TargetVolumeInfo `json:"targetVolumes,omitempty"`

	// Specifies the environment variables for the backup workload.
	//
	// +optional
	Env []EnvVar `json:"env,omitempty"`

	// Specifies runtime settings for the backup workload container.
	//
	// +optional
	RuntimeSettings *RuntimeSettings `json:"runtimeSettings,omitempty"`

	// If set, specifies the method for selecting the replica to be backed up using the criteria defined here.
	// If this field is not set, the selection method specified in `backupPolicy.target` is used.
	//
	// This field provides a way to override the global `backupPolicy.target` setting for specific BackupMethod.
	//
	// +optional
	Target *TargetInstance `json:"target"`
}

type EnvVar struct {
	// Specifies the environment variable key.
	//
	// +kubebuilder:validation:Required
	Name string `json:"value"`

	// Specifies the environment variable value.
	//
	// +optional
	Value *string `json:"value,omitempty"`

	// Specifies the source used to determine the value of the environment variable.
	// Cannot be used if value is not empty.
	//
	// +optional
	ValueFrom *ValueFrom `json:"valueFrom,omitempty"`
}

type ValueFrom struct {
	// Determine the appropriate version of the backup tool image from service version.
	// +optional
	VersionMapping []VersionMapping `json:"versionMapping,omitempty"`
}

type VersionMapping struct {
	// Represents an array of the service version that can be mapped to the appropriate value.
	// Each name in the list can represent an exact name, a name prefix, or a regular expression pattern.
	//
	// For example:
	//
	// - "8.0.33": Matches the exact name "8.0.33"
	// - "8.0": Matches all names starting with "8.0"
	// - "^8.0.\d{1,2}$": Matches all names starting with "8.0." followed by one or two digits.
	//
	// +kubebuilder:validation:Required
	ServiceVersions []string `json:"serviceVersions"`

	// Specifies a mapping value based on service version.
	// Typically used to set up the tools image required for backup operations.
	//
	// +kubebuilder:validation:Required
	MappedValue string `json:"mappedValue"`
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

	// Specifies the fallback role to select one replica for backup, this only takes effect when the
	// `strategy` field below is set to `Any`.
	//
	// +optional
	FallbackRole string `json:"fallbackRole,omitempty"`

	// If `backupPolicy.componentDefs` is set, this field is required to specify the system account name.
	// This account must match one listed in `componentDefinition.spec.systemAccounts[*].name`.
	// The corresponding secret created by this account is used to connect to the database.
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
	Strategy PodSelectionStrategy `json:"strategy,omitempty"`

	// Specifies the container port in the target pod.
	// If not specified, the first container and its first port will be used.
	//
	// +optional
	ContainerPort *ContainerPort `json:"containerPort,omitempty"`
}

// BackupPolicyTemplateStatus defines the observed state of BackupPolicyTemplate.
type BackupPolicyTemplateStatus struct {
	// Represents the most recent generation observed for this BackupPolicyTemplate.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Specifies the current phase of the BackupPolicyTemplate. Valid values are `empty`, `Available`, `Unavailable`.
	// When `Available`, the BackupPolicyTemplate is ready and can be referenced by related objects.
	Phase Phase `json:"phase,omitempty"`

	// Provides additional information about the current phase.
	//
	// +optional
	Message string `json:"message,omitempty"`
}

// BackupPolicyTemplate should be provided by addon developers.
// It is responsible for generating BackupPolicies for the addon that requires backup operations,
// also determining the suitable backup methods and strategies.
//
// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=bpt
// +kubebuilder:printcolumn:name="SERVICE-KIND",type="string",JSONPath=".spec.serviceKind",description="service kind of the backupPolicyTemplate."
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
