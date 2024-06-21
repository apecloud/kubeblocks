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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// TODO: @wangyelei could refactor to ops group

// OpsRequestSpec defines the desired state of OpsRequest
//
// +kubebuilder:validation:XValidation:rule="has(self.cancel) && self.cancel ? (self.type in ['VerticalScaling', 'HorizontalScaling']) : true",message="forbidden to cancel the opsRequest which type not in ['VerticalScaling','HorizontalScaling']"
type OpsRequestSpec struct {
	// Specifies the name of the Cluster resource that this operation is targeting.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.clusterName"
	ClusterName string `json:"clusterName,omitempty"`

	// Deprecated: since v0.9, use clusterName instead.
	// Specifies the name of the Cluster resource that this operation is targeting.
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.clusterRef"
	ClusterRef string `json:"clusterRef,omitempty"`

	// Indicates whether the current operation should be canceled and terminated gracefully if it's in the
	// "Pending", "Creating", or "Running" state.
	//
	// This field applies only to "VerticalScaling" and "HorizontalScaling" opsRequests.
	//
	// Note: Setting `cancel` to true is irreversible; further modifications to this field are ineffective.
	//
	// +optional
	Cancel bool `json:"cancel,omitempty"`

	// Instructs the system to bypass pre-checks (including cluster state checks and customized pre-conditions hooks)
	// and immediately execute the opsRequest, except for the opsRequest of 'Start' type, which will still undergo
	// pre-checks even if `force` is true.
	//
	// This is useful for concurrent execution of 'VerticalScaling' and 'HorizontalScaling' opsRequests.
	// By setting `force` to true, you can bypass the default checks and demand these opsRequests to run
	// simultaneously.
	//
	// Note: Once set, the `force` field is immutable and cannot be updated.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.force"
	// +optional
	Force bool `json:"force,omitempty"`

	// Specifies the type of this operation. Supported types include "Start", "Stop", "Restart", "Switchover",
	// "VerticalScaling", "HorizontalScaling", "VolumeExpansion", "Reconfiguring", "Upgrade", "Backup", "Restore",
	// "Expose", "DataScript", "RebuildInstance", "Custom".
	//
	// Note: This field is immutable once set.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.type"
	Type OpsType `json:"type"`

	// Specifies the duration in seconds that an OpsRequest will remain in the system after successfully completing
	// (when `opsRequest.status.phase` is "Succeed") before automatic deletion.
	//
	// +optional
	TTLSecondsAfterSucceed int32 `json:"ttlSecondsAfterSucceed,omitempty"`

	// Specifies the maximum time in seconds that the OpsRequest will wait for its pre-conditions to be met
	// before it aborts the operation.
	// If set to 0 (default), pre-conditions must be satisfied immediately for the OpsRequest to proceed.
	//
	// +kubebuilder:default=0
	// +optional
	PreConditionDeadlineSeconds *int32 `json:"preConditionDeadlineSeconds,omitempty"`

	// Exactly one of its members must be set.
	SpecificOpsRequest `json:",inline"`
}

type SpecificOpsRequest struct {
	// Specifies the desired new version of the Cluster.
	//
	// Note: This field is immutable once set.
	//
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.upgrade"
	Upgrade *Upgrade `json:"upgrade,omitempty"`

	// Lists HorizontalScaling objects, each specifying scaling requirements for a Component,
	// including desired replica changes, configurations for new instances, modifications for existing instances,
	// and take offline/online the specified instances.
	//
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.horizontalScaling"
	HorizontalScalingList []HorizontalScaling `json:"horizontalScaling,omitempty"  patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// Lists VolumeExpansion objects, each specifying a component and its corresponding volumeClaimTemplates
	// that requires storage expansion.
	//
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	VolumeExpansionList []VolumeExpansion `json:"volumeExpansion,omitempty"  patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// Lists Components to be restarted.
	//
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.restart"
	// +kubebuilder:validation:MaxItems=1024
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	RestartList []ComponentOps `json:"restart,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// Lists Switchover objects, each specifying a Component to perform the switchover operation.
	//
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.switchover"
	SwitchoverList []Switchover `json:"switchover,omitempty"  patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// Lists VerticalScaling objects, each specifying a component and its desired compute resources for vertical scaling.
	//
	// +kubebuilder:validation:MaxItems=1024
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	VerticalScalingList []VerticalScaling `json:"verticalScaling,omitempty"  patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// Specifies a component and its configuration updates.
	//
	// This field is deprecated and replaced by `reconfigures`.
	//
	// +optional
	Reconfigure *Reconfigure `json:"reconfigure,omitempty"`

	// Lists Reconfigure objects, each specifying a Component and its configuration updates.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.reconfigure"
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	Reconfigures []Reconfigure `json:"reconfigures,omitempty"  patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// Lists Expose objects, each specifying a Component and its services to be exposed.
	//
	// +optional
	ExposeList []Expose `json:"expose,omitempty"`

	// Specifies the image and scripts for executing engine-specific operations such as creating databases or users.
	// It supports limited engines including MySQL, PostgreSQL, Redis, MongoDB.
	//
	// ScriptSpec has been replaced by the more versatile OpsDefinition.
	// It is recommended to use OpsDefinition instead.
	// ScriptSpec is deprecated and will be removed in a future version.
	//
	// +optional
	ScriptSpec *ScriptSpec `json:"scriptSpec,omitempty"`

	// Specifies the parameters to backup a Cluster.
	// +optional
	Backup *Backup `json:"backup,omitempty"`

	// Deprecated: since v0.9, use backup instead.
	// Specifies the parameters to backup a Cluster.
	// +optional
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	BackupSpec *Backup `json:"backupSpec,omitempty"`

	// Specifies the parameters to restore a Cluster.
	// Note that this restore operation will roll back cluster services.
	//
	// +optional
	Restore *Restore `json:"restore,omitempty"`

	// Deprecated: since v0.9, use restore instead.
	// Specifies the parameters to restore a Cluster.
	// Note that this restore operation will roll back cluster services.
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	RestoreSpec *Restore `json:"restoreSpec,omitempty"`

	// Specifies the parameters to rebuild some instances.
	// Rebuilding an instance involves restoring its data from a backup or another database replica.
	// The instances being rebuilt usually serve as standby in the cluster.
	// Hence rebuilding instances is often also referred to as "standby reconstruction".
	//
	// +optional
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.rebuildFrom"
	RebuildFrom []RebuildInstance `json:"rebuildFrom,omitempty"  patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`

	// Specifies a custom operation defined by OpsDefinition.
	//
	// +optional
	CustomOps *CustomOps `json:"custom,omitempty"`
}

// ComponentOps specifies the Component to be operated on.
type ComponentOps struct {
	// Specifies the name of the Component.
	// +kubebuilder:validation:Required
	ComponentName string `json:"componentName"`
}

type RebuildInstance struct {
	// Specifies the name of the Component.
	ComponentOps `json:",inline"`

	// Specifies the instances (Pods) that need to be rebuilt, typically operating as standbys.
	//
	// +kubebuilder:validation:Required
	Instances []Instance `json:"instances"`

	// Indicates the name of the Backup custom resource from which to recover the instance.
	// Defaults to an empty PersistentVolume if unspecified.
	//
	// Note:
	// - Only full physical backups are supported for multi-replica Components (e.g., 'xtrabackup' for MySQL).
	// - Logical backups (e.g., 'mysqldump' for MySQL) are unsupported in the current version.
	//
	// +optional
	BackupName string `json:"backupName,omitempty"`

	// Defines container environment variables for the restore process.
	// merged with the ones specified in the Backup and ActionSet resources.
	//
	// Merge priority: Restore env > Backup env > ActionSet env.
	//
	// Purpose: Some databases require different configurations when being restored as a standby
	// compared to being restored as a primary.
	// For example, when restoring MySQL as a replica, you need to set `skip_slave_start="ON"` for 5.7
	// or `skip_replica_start="ON"` for 8.0.
	// Allowing environment variables to be passed in makes it more convenient to control these behavioral differences
	// during the restore process.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	RestoreEnv []corev1.EnvVar `json:"restoreEnv,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

type Instance struct {
	// Pod name of the instance.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// The instance will rebuild on the specified node when the instance uses local PersistentVolume as the storage disk.
	// If not set, it will rebuild on a random node.
	// +optional
	TargetNodeName string `json:"targetNodeName,omitempty"`
}

type Switchover struct {
	// Specifies the name of the Component.
	ComponentOps `json:",inline"`

	// Specifies the instance to become the primary or leader during a switchover operation.
	//
	// The value of `instanceName` can be either:
	//
	// 1. "*" (wildcard value):
	// - Indicates no specific instance is designated as the primary or leader.
	// - Executes the switchover action from `clusterDefinition.componentDefs[*].switchoverSpec.withoutCandidate`.
	// - `clusterDefinition.componentDefs[x].switchoverSpec.withoutCandidate` must be defined when using "*".
	//
	// 2. A valid instance name (pod name):
	// - Designates a specific instance (pod) as the primary or leader.
	// - The name must match one of the pods in the component. Any non-valid pod name is considered invalid.
	// - Executes the switchover action from `clusterDefinition.componentDefs[*].switchoverSpec.withCandidate`.
	// - `clusterDefinition.componentDefs[*].switchoverSpec.withCandidate` must be defined when specifying a valid instance name.
	//
	// +kubebuilder:validation:Required
	InstanceName string `json:"instanceName"`
}

// Upgrade defines the parameters for an upgrade operation.
type Upgrade struct {
	// Deprecated: since v0.9 because ClusterVersion is deprecated.
	// Specifies the name of the target ClusterVersion for the upgrade.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	ClusterVersionRef *string `json:"clusterVersionRef,omitempty"`

	// Lists components to be upgrade based on desired ComponentDefinition and ServiceVersion.
	// From the perspective of cluster API, the reasonable combinations should be:
	// 1. (comp-def, service-ver) - upgrade to the specified service version and component definition, the user takes the responsibility to ensure that they are compatible.
	// 2. ("", service-ver) - upgrade to the specified service version, let the operator choose the latest compatible component definition.
	// 3. (comp-def, "") - upgrade to the specified component definition, let the operator choose the latest compatible service version.
	// 4. ("", "") - upgrade to the latest service version and component definition, the operator will ensure the compatibility between the selected versions.
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	// +kubebuilder:validation:MaxItems=1024
	// +optional
	Components []UpgradeComponent `json:"components,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`
}

// +kubebuilder:validation:XValidation:rule="has(self.componentDefinitionName) || has(self.serviceVersion)",message="at least one componentDefinitionName or serviceVersion"

type UpgradeComponent struct {
	// Specifies the name of the Component.
	ComponentOps `json:",inline"`

	// Specifies the name of the ComponentDefinition.
	// +kubebuilder:validation:MaxLength=64
	// +optional
	ComponentDefinitionName *string `json:"componentDefinitionName,omitempty"`

	// Specifies the version of the Service expected to be provisioned by this Component.
	// Referring to the ServiceVersion defined by the ComponentDefinition and ComponentVersion.
	// And ServiceVersion in ClusterComponentSpec is optional, when no version is specified,
	// use the latest available version in ComponentVersion.
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceVersion *string `json:"serviceVersion,omitempty"`
}

// VerticalScaling refers to the process of adjusting compute resources (e.g., CPU, memory) allocated to a Component.
// It defines the parameters required for the operation.
type VerticalScaling struct {
	// Specifies the name of the Component.
	ComponentOps `json:",inline"`

	// Defines the desired compute resources of the Component's instances.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	corev1.ResourceRequirements `json:",inline"`

	// Specifies the desired compute resources of the instance template that need to vertical scale.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	Instances []InstanceResourceTemplate `json:"instances,omitempty"  patchStrategy:"merge,retainKeys" patchMergeKey:"name"`
}

type InstanceResourceTemplate struct {
	// Refer to the instance template name of the component or sharding.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Defines the computational resource size for vertical scaling.
	// +kubebuilder:pruning:PreserveUnknownFields
	corev1.ResourceRequirements `json:",inline"`
}

type InstanceVolumeClaimTemplate struct {
	// Refer to the instance template name of the component or sharding.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// volumeClaimTemplates specifies the storage size and volumeClaimTemplate name.
	// +kubebuilder:validation:Required
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	VolumeClaimTemplates []OpsRequestVolumeClaimTemplate `json:"volumeClaimTemplates" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`
}

// VolumeExpansion encapsulates the parameters required for a volume expansion operation.
type VolumeExpansion struct {
	// Specifies the name of the Component.
	ComponentOps `json:",inline"`

	// Specifies a list of OpsRequestVolumeClaimTemplate objects, defining the volumeClaimTemplates
	// that are used to expand the storage and the desired storage size for each one.
	//
	// +kubebuilder:validation:Required
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	VolumeClaimTemplates []OpsRequestVolumeClaimTemplate `json:"volumeClaimTemplates" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Specifies the desired storage size of the instance template that need to volume expand.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	Instances []InstanceVolumeClaimTemplate `json:"instances,omitempty"  patchStrategy:"merge,retainKeys" patchMergeKey:"name"`
}

type OpsRequestVolumeClaimTemplate struct {
	// Specifies the desired storage size for the volume.
	//
	// +kubebuilder:validation:Required
	Storage resource.Quantity `json:"storage"`

	// Specify the name of the volumeClaimTemplate in the Component.
	// The specified name must match one of the volumeClaimTemplates defined
	// in the `clusterComponentSpec.volumeClaimTemplates` field.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// HorizontalScaling defines the parameters of a horizontal scaling operation.
type HorizontalScaling struct {
	// Specifies the name of the Component.
	ComponentOps `json:",inline"`

	// Deprecated: since v0.9, use scaleOut and scaleIn instead.
	// Specifies the number of replicas for the component. Cannot be used with "scaleIn" and "scaleOut".
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Specifies the replica changes for scaling out components and instance templates,
	// and brings offline instances back online. Can be used in conjunction with the "scaleIn" operation.
	// Note: Any configuration that deletes instances is considered invalid.
	//
	// +optional
	ScaleOut *ScaleOut `json:"scaleOut,omitempty"`

	// Specifies the replica changes for scaling in components and instance templates,
	// and takes specified instances offline. Can be used in conjunction with the "scaleOut" operation.
	// Note: Any configuration that creates instances is considered invalid.
	// +optional
	ScaleIn *ScaleIn `json:"scaleIn,omitempty"`
}

// ScaleOut defines the configuration for a scale-out operation.
type ScaleOut struct {

	// Modifies the replicas of the component and instance templates.
	ReplicaChanger `json:",inline"`

	// Defines the configuration for new instances added during scaling, including resource requirements, labels, annotations, etc.
	// New instances are created based on the provided instance templates.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	NewInstances []InstanceTemplate `json:"newInstances,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Specifies the instances in the offline list to bring back online.
	// +optional
	OfflineInstancesToOnline []string `json:"offlineInstancesToOnline,omitempty"`
}

// ScaleIn defines the configuration for a scale-in operation.
type ScaleIn struct {

	// Modifies the replicas of the component and instance templates.
	ReplicaChanger `json:",inline"`

	// Specifies the instance names that need to be taken offline.
	// +optional
	OnlineInstancesToOffline []string `json:"onlineInstancesToOffline,omitempty"`
}

// ReplicaChanger defines the parameters for changing the number of replicas.
type ReplicaChanger struct {
	// Specifies the replica changes for the component.
	// +kubebuilder:validation:Minimum=0
	ReplicaChanges *int32 `json:"replicaChanges,omitempty"`

	// Modifies the desired replicas count for existing InstanceTemplate.
	// if the inst
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	Instances []InstanceReplicasTemplate `json:"instances,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`
}

// InstanceReplicasTemplate defines the template for instance replicas.
type InstanceReplicasTemplate struct {
	// Specifies the name of the instance template.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the replica changes for the instance template.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	ReplicaChanges int32 `json:"replicaChanges"`
}

// Reconfigure defines the parameters for updating a Component's configuration.
type Reconfigure struct {
	// Specifies the name of the Component.
	ComponentOps `json:",inline"`

	// Contains a list of ConfigurationItem objects, specifying the Component's configuration template name,
	// upgrade policy, and parameter key-value pairs to be updated.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Configurations []ConfigurationItem `json:"configurations" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Indicates the duration for which the parameter changes are valid.
	// +optional
	// TTL *int64 `json:"ttl,omitempty"`

	// Specifies the time when the parameter changes should be applied.
	// +kubebuilder:validation:MaxLength=19
	// +kubebuilder:validation:MinLength=19
	// +kubebuilder:validation:Pattern:=`^([0-9]{2})/([0-9]{2})/([0-9]{4}) ([0-9]{2}):([0-9]{2}):([0-9]{2})$`
	// +optional
	// TriggeringTime *string `json:"triggeringTime,omitempty"`

	//  Identifies the component to be reconfigured.
	// +optional
	// Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

type ConfigurationItem struct {
	// Specifies the name of the configuration template.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Defines the upgrade policy for the configuration.
	//
	// +optional
	Policy *UpgradePolicy `json:"policy,omitempty"`

	// Sets the configuration files and their associated parameters that need to be updated.
	// It should contain at least one item.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=key
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=key
	Keys []ParameterConfig `json:"keys" patchStrategy:"merge,retainKeys" patchMergeKey:"key"`
}

type CustomOps struct {
	// Specifies the name of the OpsDefinition.
	//
	// +kubebuilder:validation:Required
	OpsDefinitionName string `json:"opsDefinitionName"`

	// Specifies the name of the ServiceAccount to be used for executing the custom operation.
	ServiceAccountName *string `json:"serviceAccountName,omitempty"`

	// Specifies the maximum number of components to be operated on concurrently to mitigate performance impact
	// on clusters with multiple components.
	//
	// It accepts an absolute number (e.g., 5) or a percentage of components to execute in parallel (e.g., "10%").
	// Percentages are rounded up to the nearest whole number of components.
	// For example, if "10%" results in less than one, it rounds up to 1.
	//
	// When unspecified, all components are processed simultaneously by default.
	//
	// Note: This feature is not implemented yet.
	//
	// +optional
	MaxConcurrentComponents intstr.IntOrString `json:"maxConcurrentComponents,omitempty"`

	// Specifies the components and their parameters for executing custom actions as defined in OpsDefinition.
	// Requires at least one component.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=1024
	// +patchMergeKey=componentName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentName
	CustomOpsComponents []CustomOpsComponent `json:"components"  patchStrategy:"merge,retainKeys" patchMergeKey:"componentName"`
}

type CustomOpsComponent struct {
	// Specifies the name of the Component.
	ComponentOps `json:",inline"`

	// Specifies the parameters that match the schema specified in the `opsDefinition.spec.parametersSchema`.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	Parameters []Parameter `json:"parameters,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`
}

type Parameter struct {
	// Specifies the identifier of the parameter as defined in the OpsDefinition.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Holds the data associated with the parameter.
	// If the parameter type is an array, the format should be "v1,v2,v3".
	// +kubebuilder:validation:Required
	Value string `json:"value"`
}

type ParameterPair struct {
	// Represents the name of the parameter that is to be updated.
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// Represents the parameter values that are to be updated.
	// If set to nil, the parameter defined by the Key field will be removed from the configuration file.
	// +optional
	Value *string `json:"value"`
}

type ParameterConfig struct {
	// Represents a key in the configuration template(as ConfigMap).
	// Each key in the ConfigMap corresponds to a specific configuration file.
	//
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// Specifies a list of key-value pairs representing parameters and their corresponding values
	// within a single configuration file.
	// This field is used to override or set the values of parameters without modifying the entire configuration file.
	//
	// Either the `parameters` field or the `fileContent` field must be set, but not both.
	//
	// +optional
	Parameters []ParameterPair `json:"parameters,omitempty"`

	// Specifies the content of the entire configuration file.
	// This field is used to update the complete configuration file.
	//
	// Either the `parameters` field or the `fileContent` field must be set, but not both.
	//
	// +optional
	FileContent string `json:"fileContent,omitempty"`
}

// ExposeSwitch Specifies the switch for the expose operation. This switch can be used to enable or disable the expose operation.
// +enum
// +kubebuilder:validation:Enum={Enable, Disable}
type ExposeSwitch string

const (
	EnableExposeSwitch  ExposeSwitch = "Enable"
	DisableExposeSwitch ExposeSwitch = "Disable"
)

type Expose struct {
	// Specifies the name of the Component.
	ComponentName string `json:"componentName,omitempty"`

	// Indicates whether the services will be exposed.
	// 'Enable' exposes the services. while 'Disable' removes the exposed Service.
	//
	// +kubebuilder:validation:Required
	Switch ExposeSwitch `json:"switch"`

	// Specifies a list of OpsService.
	// When an OpsService is exposed, a corresponding ClusterService will be added to `cluster.spec.services`.
	// On the other hand, when an OpsService is unexposed, the corresponding ClusterService will be removed
	// from `cluster.spec.services`.
	//
	// Note: If `componentName` is not specified, the `ports` and `selector` fields must be provided
	// in each OpsService definition.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minitems=0
	Services []OpsService `json:"services"`
}

// OpsService represents the parameters to dynamically create or remove a ClusterService in the `cluster.spec.services` array.
type OpsService struct {
	// Specifies the name of the Service. This name is used to set `clusterService.name`.
	//
	// Note: This field cannot be updated.
	//
	// +required
	Name string `json:"name"`

	// Contains cloud provider related parameters if ServiceType is LoadBalancer.
	//
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Specifies Port definitions that are to be exposed by a ClusterService.
	//
	// If not specified, the Port definitions from non-NodePort and non-LoadBalancer type ComponentService
	// defined in the ComponentDefinition (`componentDefinition.spec.services`) will be used.
	// If no matching ComponentService is found, the expose operation will fail.
	//
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#field-spec-ports
	//
	// +patchMergeKey=port
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=port
	// +listMapKey=protocol
	// +optional
	Ports []corev1.ServicePort `json:"ports,omitempty" patchStrategy:"merge" patchMergeKey:"port" protobuf:"bytes,1,rep,name=ports"`

	// Specifies a role to target with the service.
	// If specified, the service will only be exposed to pods with the matching role.
	//
	// Note: At least one of 'roleSelector' or 'podSelector' must be specified.
	// If both are specified, a pod must match both conditions to be selected.
	//
	// +optional
	RoleSelector string `json:"roleSelector,omitempty"`

	// Routes service traffic to pods with matching label keys and values.
	// If specified, the service will only be exposed to pods matching the selector.
	//
	// Note: At least one of 'roleSelector' or 'podSelector' must be specified.
	// If both are specified, a pod must match both conditions to be selected.
	//
	// +optional
	// +mapType=atomic
	PodSelector map[string]string `json:"podSelector,omitempty"`

	// Determines how the Service is exposed. Defaults to 'ClusterIP'.
	// Valid options are `ClusterIP`, `NodePort`, and `LoadBalancer`.
	//
	// - `ClusterIP`: allocates a cluster-internal IP address for load-balancing to endpoints.
	//    Endpoints are determined by the selector or if that is not specified,
	//    they are determined by manual construction of an Endpoints object or EndpointSlice objects.
	// - `NodePort`: builds on ClusterIP and allocates a port on every node which routes to the same endpoints as the clusterIP.
	// - `LoadBalancer`: builds on NodePort and creates an external load-balancer (if supported in the current cloud)
	//    which routes to the same endpoints as the clusterIP.
	//
	// Note: although K8s Service type allows the 'ExternalName' type, it is not a valid option for the expose operation.
	//
	// For more info, see:
	// https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types.
	//
	// +optional
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`

	// A list of IP families (e.g., IPv4, IPv6) assigned to this Service.
	//
	// Usually assigned automatically based on the cluster configuration and the `ipFamilyPolicy` field.
	// If specified manually, the requested IP family must be available in the cluster and allowed by the `ipFamilyPolicy`.
	// If the requested IP family is not available or not allowed, the Service creation will fail.
	//
	// Valid values:
	//
	// - "IPv4"
	// - "IPv6"
	//
	// This field may hold a maximum of two entries (dual-stack families, in either order).
	//
	// Common combinations of `ipFamilies` and `ipFamilyPolicy` are:
	//
	// - ipFamilies=[] + ipFamilyPolicy="PreferDualStack" :
	//   The Service prefers dual-stack but can fall back to single-stack if the cluster does not support dual-stack.
	//   The IP family is automatically assigned based on the cluster configuration.
	// - ipFamilies=["IPV4","IPV6"] + ipFamilyPolicy="RequiredDualStack" :
	//   The Service requires dual-stack and will only be created if the cluster supports both IPv4 and IPv6.
	//   The primary IP family is IPV4.
	// - ipFamilies=["IPV6","IPV4"] + ipFamilyPolicy="RequiredDualStack" :
	//   The Service requires dual-stack and will only be created if the cluster supports both IPv4 and IPv6.
	//   The primary IP family is IPV6.
	// - ipFamilies=["IPV4"] + ipFamilyPolicy="SingleStack" :
	//   The Service uses a single-stack with IPv4 only.
	// - ipFamilies=["IPV6"] + ipFamilyPolicy="SingleStack" :
	//   The Service uses a single-stack with IPv6 only.
	//
	// +listType=atomic
	// +optional
	IPFamilies []corev1.IPFamily `json:"ipFamilies,omitempty" protobuf:"bytes,19,opt,name=ipFamilies,casttype=IPFamily"`

	// Specifies whether the Service should use a single IP family (SingleStack) or two IP families (DualStack).
	//
	// Possible values:
	//
	// - 'SingleStack' (default) : The Service uses a single IP family.
	//   If no value is provided, IPFamilyPolicy defaults to SingleStack.
	// - 'PreferDualStack' : The Service prefers to use two IP families on dual-stack configured clusters
	//   or a single IP family on single-stack clusters.
	// - 'RequiredDualStack' : The Service requires two IP families on dual-stack configured clusters.
	//   If the cluster is not configured for dual-stack, the Service creation fails.
	//
	// +optional
	IPFamilyPolicy *corev1.IPFamilyPolicy `json:"ipFamilyPolicy,omitempty" protobuf:"bytes,17,opt,name=ipFamilyPolicy,casttype=IPFamilyPolicy"`
}

type RefNamespaceName struct {
	// Refers to the specific name of the resource.
	// +optional
	Name string `json:"name,omitempty"`

	// Refers to the specific namespace of the resource.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

type BackupRefSpec struct {
	// Refers to a reference backup that needs to be restored.
	// +optional
	Ref RefNamespaceName `json:"ref,omitempty"`
}

type PointInTimeRefSpec struct {
	// Refers to the specific time point for restoration, with UTC as the time zone.
	// +optional
	Time *metav1.Time `json:"time,omitempty"`

	// Refers to a reference source cluster that needs to be restored.
	// +optional
	Ref RefNamespaceName `json:"ref,omitempty"`
}

// ScriptSpec is a legacy feature for executing engine-specific operations such as creating databases or users.
// It supports limited engines including MySQL, PostgreSQL, Redis, MongoDB.
//
// ScriptSpec has been replaced by the more versatile OpsDefinition.
// It is recommended to use OpsDefinition instead. ScriptSpec is deprecated and will be removed in a future version.
type ScriptSpec struct {
	// Specifies the name of the Component.
	ComponentOps `json:",inline"`

	// Specifies the image to be used to execute scripts.
	//
	// By default, the image "apecloud/kubeblocks-datascript:latest" is used.
	//
	// +optional
	Image string `json:"image,omitempty"`

	// Defines the secret to be used to execute the script. If not specified, the default cluster root credential secret is used.
	// +optional
	Secret *ScriptSecret `json:"secret,omitempty"`

	// Defines the content of scripts to be executed.
	//
	// All scripts specified in this field will be executed in the order they are provided.
	//
	// Note: this field cannot be modified once set.
	//
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.scriptSpec.script"
	Script []string `json:"script,omitempty"`

	// Specifies the sources of the scripts to be executed.
	// Each script can be imported either from a ConfigMap or a Secret.
	//
	// All scripts obtained from the sources specified in this field will be executed after
	// any scripts provided in the `script` field.
	//
	// Execution order:
	// 1. Scripts provided in the `script` field, in the order of the scripts listed.
	// 2. Scripts imported from ConfigMaps, in the order of the sources listed.
	// 3. Scripts imported from Secrets, in the order of the sources listed.
	//
	// Note: this field cannot be modified once set.
	//
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.scriptSpec.scriptFrom"
	ScriptFrom *ScriptFrom `json:"scriptFrom,omitempty"`

	// Specifies the labels used to select the Pods on which the script should be executed.
	//
	// By default, the script is executed on the Pod associated with the service named "{clusterName}-{componentName}",
	// which typically routes to the Pod with the primary/leader role.
	//
	// However, some Components, such as Redis, do not synchronize account information between primary and secondary Pods.
	// In these cases, the script must be executed on all replica Pods matching the selector.
	//
	// Note: this field cannot be modified once set.
	//
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.scriptSpec.script.selector"
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

type Backup struct {
	// Specifies the name of the Backup custom resource.
	//
	// +optional
	BackupName string `json:"backupName,omitempty"`

	// Indicates the name of the BackupPolicy applied to perform this Backup.
	//
	// +optional
	BackupPolicyName string `json:"backupPolicyName,omitempty"`

	// Specifies the name of BackupMethod.
	// The specified BackupMethod must be defined in the BackupPolicy.
	//
	// +optional
	BackupMethod string `json:"backupMethod,omitempty"`

	// Determines whether the backup contents stored in backup repository
	// should be deleted when the Backup custom resource is deleted.
	// Supported values are `Retain` and `Delete`.
	// - `Retain` means that the backup content and its physical snapshot on backup repository are kept.
	// - `Delete` means that the backup content and its physical snapshot on backup repository are deleted.
	// +kubebuilder:validation:Enum=Delete;Retain
	// +kubebuilder:default=Delete
	// +optional
	DeletionPolicy string `json:"deletionPolicy,omitempty"`

	// Determines the duration for which the Backup custom resources should be retained.
	//
	// The controller will automatically remove all Backup objects that are older than the specified RetentionPeriod.
	// For example, RetentionPeriod of `30d` will keep only the Backup objects of last 30 days.
	// Sample duration format:
	//
	// - years: 2y
	// - months: 6mo
	// - days: 30d
	// - hours: 12h
	// - minutes: 30m
	//
	// You can also combine the above durations. For example: 30d12h30m.
	// If not set, the Backup objects will be kept forever.
	//
	// If the `deletionPolicy` is set to 'Delete', then the associated backup data will also be deleted
	// along with the Backup object.
	// Otherwise, only the Backup custom resource will be deleted.
	//
	// +optional
	RetentionPeriod string `json:"retentionPeriod,omitempty"`

	// If the specified BackupMethod is incremental, `parentBackupName` is required.
	//
	// +optional
	ParentBackupName string `json:"parentBackupName,omitempty"`
}

type Restore struct {
	// Specifies the name of the Backup custom resource.
	//
	// +kubebuilder:validation:Required
	BackupName string `json:"backupName"`

	// Specifies the point in time to which the restore should be performed.
	// Supported time formats:
	//
	// - RFC3339 format, e.g. "2023-11-25T18:52:53Z"
	// - A human-readable date-time format, e.g. "Jul 25,2023 18:52:53 UTC+0800"
	//
	RestorePointInTime string `json:"restorePointInTime,omitempty"`

	// Specifies the policy for restoring volume claims of a Component's Pods.
	// It determines whether the volume claims should be restored sequentially (one by one) or in parallel (all at once).
	// Support values:
	//
	// - "Serial"
	// - "Parallel"
	//
	// +kubebuilder:validation:Enum=Serial;Parallel
	// +kubebuilder:default=Parallel
	VolumeRestorePolicy string `json:"volumeRestorePolicy,omitempty"`

	// Controls the timing of PostReady actions during the recovery process.
	//
	// If false (default), PostReady actions execute when the Component reaches the "Running" state.
	// If true, PostReady actions are delayed until the entire Cluster is "Running,"
	// ensuring the cluster's overall stability before proceeding.
	//
	// This setting is useful for coordinating PostReady operations across the Cluster for optimal cluster conditions.
	DeferPostReadyUntilClusterRunning bool `json:"deferPostReadyUntilClusterRunning,omitempty"`
}

// ScriptSecret represents the secret that is used to execute the script.
type ScriptSecret struct {
	// Specifies the name of the secret.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`
	// Used to specify the username part of the secret.
	// +kubebuilder:default:="username"
	// +optional
	UsernameKey string `json:"usernameKey,omitempty"`
	// Used to specify the password part of the secret.
	// +kubebuilder:default:="password"
	// +optional
	PasswordKey string `json:"passwordKey,omitempty"`
}

// ScriptFrom specifies the source of the script to be executed, which can be either a ConfigMap or a Secret.
type ScriptFrom struct {
	// A list of ConfigMapKeySelector objects, each specifies a ConfigMap and a key containing the script.
	//
	// Note: This field cannot be modified once set.
	//
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.scriptSpec.scriptFrom.configMapRef"
	ConfigMapRef []corev1.ConfigMapKeySelector `json:"configMapRef,omitempty"`

	// A list of SecretKeySelector objects, each specifies a Secret and a key containing the script.
	//
	// Note: This field cannot be modified once set.
	//
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="forbidden to update spec.scriptSpec.scriptFrom.secretRef"
	SecretRef []corev1.SecretKeySelector `json:"secretRef,omitempty"`
}

// OpsRequestStatus represents the observed state of an OpsRequest.
type OpsRequestStatus struct {
	// Records the cluster generation after the OpsRequest action has been handled.
	// +optional
	ClusterGeneration int64 `json:"clusterGeneration,omitempty"`

	// Represents the phase of the OpsRequest.
	// Possible values include "Pending", "Creating", "Running", "Cancelling", "Cancelled", "Failed", "Succeed".
	Phase OpsPhase `json:"phase,omitempty"`

	// Represents the progress of the OpsRequest.
	// +kubebuilder:validation:Pattern:=`^(\d+|\-)/(\d+|\-)$`
	// +kubebuilder:default=-/-
	Progress string `json:"progress"`

	// Records the configuration prior to any changes.
	// +optional
	LastConfiguration LastConfiguration `json:"lastConfiguration,omitempty"`

	// Records the status information of Components changed due to the OpsRequest.
	// +optional
	Components map[string]OpsRequestComponentStatus `json:"components,omitempty"`

	// A collection of additional key-value pairs that provide supplementary information for the OpsRequest.
	Extras []map[string]string `json:"extras,omitempty"`

	// Records the time when the OpsRequest started processing.
	// +optional
	StartTimestamp metav1.Time `json:"startTimestamp,omitempty"`

	// Records the time when the OpsRequest was completed.
	// +optional
	CompletionTimestamp metav1.Time `json:"completionTimestamp,omitempty"`

	// Records the time when the OpsRequest was cancelled.
	// +optional
	CancelTimestamp metav1.Time `json:"cancelTimestamp,omitempty"`

	// Deprecated: Replaced by ReconfiguringStatusAsComponent.
	// Defines the status information of reconfiguring.
	// +optional
	ReconfiguringStatus *ReconfiguringStatus `json:"reconfiguringStatus,omitempty"`

	// Records the status of a reconfiguring operation if `opsRequest.spec.type` equals to "Reconfiguring".
	// +optional
	ReconfiguringStatusAsComponent map[string]*ReconfiguringStatus `json:"reconfiguringStatusAsComponent,omitempty"`

	// Describes the detailed status of the OpsRequest.
	// Possible condition types include "Cancelled", "WaitForProgressing", "Validated", "Succeed", "Failed", "Restarting",
	// "VerticalScaling", "HorizontalScaling", "VolumeExpanding", "Reconfigure", "Switchover", "Stopping", "Starting",
	// "VersionUpgrading", "Exposing", "ExecuteDataScript", "Backup", "InstancesRebuilding", "CustomOperation".
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="has(self.objectKey) || has(self.actionName)", message="at least one objectKey or actionName."

type ProgressStatusDetail struct {
	// Specifies the group to which the current object belongs to.
	// +optional
	Group string `json:"group,omitempty"`

	// `objectKey` uniquely identifies the object, which can be any K8s object, like a Pod, Job, Component, or PVC.
	// Either `objectKey` or `actionName` must be provided.
	// +optional
	ObjectKey string `json:"objectKey,omitempty"`

	// Indicates the name of an OpsAction, as defined in `opsDefinition.spec.actions[*].name`.
	// Either `objectKey` or `actionName` must be provided.
	// +optional
	ActionName string `json:"actionName,omitempty"`

	// Lists the tasks, such as Jobs or Pods, that carry out the action.
	// +optional
	ActionTasks []ActionTask `json:"actionTasks,omitempty"`

	// Represents the current processing state of the object, including "Processing", "Pending", "Failed", "Succeed"
	// +kubebuilder:validation:Required
	Status ProgressStatus `json:"status"`

	// Provides a human-readable explanation of the object's condition.
	// +optional
	Message string `json:"message,omitempty"`

	// Records the start time of object processing.
	// +optional
	StartTime metav1.Time `json:"startTime,omitempty"`

	// Records the completion time of object processing.
	// +optional
	EndTime metav1.Time `json:"endTime,omitempty"`
}

type ActionTask struct {
	// Represents the name of the task.
	// +kubebuilder:validation:Required
	ObjectKey string `json:"objectKey"`

	// Represents the namespace where the task is deployed.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// Indicates the current status of the task, including "Processing", "Failed", "Succeed".
	// +kubebuilder:validation:Required
	Status ActionTaskStatus `json:"status"`

	// The name of the Pod that the task is associated with or operates on.
	// +optional
	TargetPodName string `json:"targetPodName,omitempty"`

	// The count of retry attempts made for this task.
	// +optional
	Retries int32 `json:"retries,omitempty"`
}

// LastComponentConfiguration can be used to track and compare the desired state of the Component over time.
type LastComponentConfiguration struct {
	// Records the `replicas` of the Component prior to any changes.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Records the resources of the Component prior to any changes.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	corev1.ResourceRequirements `json:",inline,omitempty"`

	// Records the class of the Component prior to any changes.
	// Deprecated since v0.9.
	// +kubebuilder:deprecatedversion:warning="Due to the lack of practical use cases, this field is deprecated from KB 0.9.0."
	// +optional
	ClassDefRef *ClassDefRef `json:"classDefRef,omitempty"`

	// Records volumes' storage size of the Component prior to any changes.
	// +optional
	VolumeClaimTemplates []OpsRequestVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty"`

	// Records the ClusterComponentService list of the Component prior to any changes.
	// +optional
	Services []ClusterComponentService `json:"services,omitempty"`

	// Records the information about various types of resources associated with the Component prior to any changes.
	// Currently, only one type of resource is supported: "pods".
	// The "pods" key maps to a list of names of all Pods of the Component.
	// +optional
	TargetResources map[ComponentResourceKey][]string `json:"targetResources,omitempty"`

	// Records the InstanceTemplate list of the Component prior to any changes.
	// +optional
	Instances []InstanceTemplate `json:"instances,omitempty"`

	// Records the offline instances of the Component prior to any changes.
	// +optional
	OfflineInstances []string `json:"offlineInstances,omitempty"`

	// Records the version of the Service expected to be provisioned by this Component prior to any changes.
	// +optional
	ServiceVersion string `json:"serviceVersion,omitempty"`

	// Records the name of the ComponentDefinition prior to any changes.
	// +optional
	ComponentDefinitionName string `json:"componentDefinitionName,omitempty"`
}

type LastConfiguration struct {
	// Specifies the name of the ClusterVersion.
	// Deprecated and should be removed in the future version.
	// +optional
	ClusterVersionRef string `json:"clusterVersionRef,omitempty"`

	// Records the configuration of each Component prior to any changes.
	// +optional
	Components map[string]LastComponentConfiguration `json:"components,omitempty"`
}

type OpsRequestComponentStatus struct {
	// Records the current phase of the Component, mirroring `cluster.status.components[componentName].phase`.
	// Possible values include "Creating", "Running", "Updating", "Stopping", "Stopped", "Deleting", "Failed", "Abnormal".
	// +optional
	Phase ClusterComponentPhase `json:"phase,omitempty"`

	// Records the timestamp when the Component last transitioned to a "Failed" or "Abnormal" phase.
	// +optional
	LastFailedTime metav1.Time `json:"lastFailedTime,omitempty"`

	// Records the result of the preConditions check of the opsRequest, which determines subsequent steps.
	// +optional
	PreCheckResult *PreCheckResult `json:"preCheck,omitempty"`

	// Describes the progress details of objects or actions associated with the Component.
	// +optional
	ProgressDetails []ProgressStatusDetail `json:"progressDetails,omitempty"`

	// Records the workload type of Component in ClusterDefinition.
	// Deprecated and should be removed in the future version.
	// +optional
	WorkloadType WorkloadType `json:"workloadType,omitempty"`

	// Provides an explanation for the Component being in its current state.
	// +kubebuilder:validation:MaxLength=1024
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,5,opt,name=reason"`

	// Provides a human-readable message indicating details about this operation.
	// +kubebuilder:validation:MaxLength=32768
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,6,opt,name=message"`
}

type OverrideBy struct {
	// Indicates the name of the OpsRequest.
	// +optional
	OpsName string `json:"opsName"`

	LastComponentConfiguration `json:",inline"`
}

type PreCheckResult struct {
	// Indicates whether the preCheck operation passed or failed.
	// +kubebuilder:validation:Required
	Pass bool `json:"pass"`

	// Provides explanations related to the preCheck result in a human-readable format.
	// +optional
	Message string `json:"message,omitempty"`
}

type ReconfiguringStatus struct {
	// Describes the reconfiguring detail status.
	// Possible condition types include "Creating", "Init", "Running", "Pending", "Merged", "MergeFailed", "FailedAndPause",
	// "Upgrading", "Deleting", "FailedAndRetry", "Finished", "ReconfigurePersisting", "ReconfigurePersisted".
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Describes the status of the component reconfiguring.
	// +kubebuilder:validation:Required
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ConfigurationStatus []ConfigurationItemStatus `json:"configurationStatus"`
}

type ConfigurationItemStatus struct {
	// Indicates the name of the configuration template (as ConfigMap).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Records the UpgradePolicy of the configuration change operation.
	// +optional
	UpdatePolicy UpgradePolicy `json:"updatePolicy,omitempty"`

	// Represents the current state of the reconfiguration state machine.
	// Possible values include "Creating", "Init", "Running", "Pending", "Merged", "MergeFailed", "FailedAndPause",
	// "Upgrading", "Deleting", "FailedAndRetry", "Finished", "ReconfigurePersisting", "ReconfigurePersisted".
	// +optional
	Status string `json:"status,omitempty"`

	// Provides details about the operation.
	// +optional
	Message string `json:"message,omitempty"`

	// Records the number of pods successfully updated following a configuration change.
	// +kubebuilder:default=0
	// +optional
	SucceedCount int32 `json:"succeedCount"`

	// Represents the total count of pods intended to be updated by a configuration change.
	// +kubebuilder:default=-1
	// +optional
	ExpectedCount int32 `json:"expectedCount"`

	// Records the last state of the reconfiguration finite state machine.
	// Possible values include "None", "Retry", "Failed", "NotSupport", "FailedAndRetry".
	//
	// - "None" describes fsm has finished and quit.
	// - "Retry" describes fsm is running.
	// - "Failed" describes fsm is failed and exited.
	// - "NotSupport" describes fsm does not support the feature.
	// - "FailedAndRetry" describes fsm is failed in current state, but can be retried.
	// +optional
	LastAppliedStatus string `json:"lastStatus,omitempty"`

	// Stores the last applied configuration.
	// +optional
	LastAppliedConfiguration map[string]string `json:"lastAppliedConfiguration,omitempty"`

	// Contains the updated parameters.
	// +optional
	UpdatedParameters UpdatedParameters `json:"updatedParameters"`
}

// UpdatedParameters holds details about the modifications made to configuration parameters.
// Example:
//
// ```yaml
// updatedParameters:
//
//	updatedKeys:
//	  my.cnf: '{"mysqld":{"max_connections":"100"}}'
//
// ```
type UpdatedParameters struct {
	// Maps newly added configuration files to their content.
	// +optional
	AddedKeys map[string]string `json:"addedKeys,omitempty"`

	// Lists the name of configuration files that have been deleted.
	// +optional
	DeletedKeys map[string]string `json:"deletedKeys,omitempty"`

	// Maps the name of configuration files to their updated content, detailing the changes made.
	// +optional
	UpdatedKeys map[string]string `json:"updatedKeys,omitempty"`
}

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all},shortName=ops
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.type",description="Operation request type."
// +kubebuilder:printcolumn:name="CLUSTER",type="string",JSONPath=".spec.clusterName",description="Operand cluster."
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="Operation status phase."
// +kubebuilder:printcolumn:name="PROGRESS",type="string",JSONPath=".status.progress",description="Operation processing progress."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// OpsRequest is the Schema for the opsrequests API
type OpsRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpsRequestSpec   `json:"spec,omitempty"`
	Status OpsRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpsRequestList contains a list of OpsRequest
type OpsRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpsRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpsRequest{}, &OpsRequestList{})
}

func (c ComponentOps) GetComponentName() string {
	return c.ComponentName
}

// ToExposeListToMap build expose map
func (r OpsRequestSpec) ToExposeListToMap() map[string]Expose {
	exposeMap := make(map[string]Expose)
	for _, v := range r.ExposeList {
		exposeMap[v.ComponentName] = v
	}
	return exposeMap
}

func (r OpsRequestSpec) GetClusterName() string {
	if r.ClusterName != "" {
		return r.ClusterName
	}
	return r.ClusterRef
}

func (r OpsRequestSpec) GetBackup() *Backup {
	if r.Backup != nil {
		return r.Backup
	}
	return r.BackupSpec
}

func (r OpsRequestSpec) GetRestore() *Restore {
	if r.Restore != nil {
		return r.Restore
	}
	return r.RestoreSpec
}

func (p *ProgressStatusDetail) SetStatusAndMessage(status ProgressStatus, message string) {
	p.Message = message
	p.Status = status
}
