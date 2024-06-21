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
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// ClusterSpec defines the desired state of Cluster.
type ClusterSpec struct {
	// Specifies the name of the ClusterDefinition to use when creating a Cluster.
	//
	// This field enables users to create a Cluster based on a specific ClusterDefinition.
	// Which, in conjunction with the `topology` field, determine:
	//
	// - The Components to be included in the Cluster.
	// - The sequences in which the Components are created, updated, and terminate.
	//
	// This facilitates multiple-components management with predefined ClusterDefinition.
	//
	// Users with advanced requirements can bypass this general setting and specify more precise control over
	// the composition of the Cluster by directly referencing specific ComponentDefinitions for each component
	// within `componentSpecs[*].componentDef`.
	//
	// If this field is not provided, each component must be explicitly defined in `componentSpecs[*].componentDef`.
	//
	// Note: Once set, this field cannot be modified; it is immutable.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="clusterDefinitionRef is immutable"
	// +optional
	ClusterDefRef string `json:"clusterDefinitionRef,omitempty"`

	// Refers to the ClusterVersion name.
	//
	// Deprecated since v0.9, use ComponentVersion instead.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	ClusterVersionRef string `json:"clusterVersionRef,omitempty"`

	// Specifies the name of the ClusterTopology to be used when creating the Cluster.
	//
	// This field defines which set of Components, as outlined in the ClusterDefinition, will be used to
	// construct the Cluster based on the named topology.
	// The ClusterDefinition may list multiple topologies under `clusterdefinition.spec.topologies[*]`,
	// each tailored to different use cases or environments.
	//
	// If `topology` is not specified, the Cluster will use the default topology defined in the ClusterDefinition.
	//
	// Note: Once set during the Cluster creation, the `topology` field cannot be modified.
	// It establishes the initial composition and structure of the Cluster and is intended for one-time configuration.
	//
	// +kubebuilder:validation:MaxLength=32
	// +optional
	Topology string `json:"topology,omitempty"`

	// Specifies the behavior when a Cluster is deleted.
	// It defines how resources, data, and backups associated with a Cluster are managed during termination.
	// Choose a policy based on the desired level of resource cleanup and data preservation:
	//
	// - `DoNotTerminate`: Prevents deletion of the Cluster. This policy ensures that all resources remain intact.
	// - `Halt`: Deletes Cluster resources like Pods and Services but retains Persistent Volume Claims (PVCs),
	//   allowing for data preservation while stopping other operations.
	// - `Delete`: Extends the `Halt` policy by also removing PVCs, leading to a thorough cleanup while
	//   removing all persistent data.
	// - `WipeOut`: An aggressive policy that deletes all Cluster resources, including volume snapshots and
	//   backups in external storage.
	//   This results in complete data removal and should be used cautiously, primarily in non-production environments
	//   to avoid irreversible data loss.
	//
	// Warning: Choosing an inappropriate termination policy can result in data loss.
	// The `WipeOut` policy is particularly risky in production environments due to its irreversible nature.
	//
	// +kubebuilder:validation:Required
	TerminationPolicy TerminationPolicyType `json:"terminationPolicy"`

	// Specifies a list of ShardingSpec objects that manage the sharding topology for Cluster Components.
	// Each ShardingSpec organizes components into shards, with each shard corresponding to a Component.
	// Components within a shard are all based on a common ClusterComponentSpec template, ensuring uniform configurations.
	//
	// This field supports dynamic resharding by facilitating the addition or removal of shards
	// through the `shards` field in ShardingSpec.
	//
	// Note: `shardingSpecs` and `componentSpecs` cannot both be empty; at least one must be defined to configure a Cluster.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	// +optional
	ShardingSpecs []ShardingSpec `json:"shardingSpecs,omitempty"`

	// Specifies a list of ClusterComponentSpec objects used to define the individual Components that make up a Cluster.
	// This field allows for detailed configuration of each Component within the Cluster.
	//
	// Note: `shardingSpecs` and `componentSpecs` cannot both be empty; at least one must be defined to configure a Cluster.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	// +kubebuilder:validation:XValidation:rule="self.all(x, size(self.filter(c, c.name == x.name)) == 1)",message="duplicated component"
	// +kubebuilder:validation:XValidation:rule="self.all(x, size(self.filter(c, has(c.componentDef))) == 0) || self.all(x, size(self.filter(c, has(c.componentDef))) == size(self))",message="two kinds of definition API can not be used simultaneously"
	// +optional
	ComponentSpecs []ClusterComponentSpec `json:"componentSpecs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Defines a list of additional Services that are exposed by a Cluster.
	// This field allows Services of selected Components, either from `componentSpecs` or `shardingSpecs` to be exposed,
	// alongside Services defined with ComponentService.
	//
	// Services defined here can be referenced by other clusters using the ServiceRefClusterSelector.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Services []ClusterService `json:"services,omitempty"`

	// Defines a set of node affinity scheduling rules for the Cluster's Pods.
	// This field helps control the placement of Pods on nodes within the Cluster.
	//
	// +optional
	Affinity *Affinity `json:"affinity,omitempty"`

	// An array that specifies tolerations attached to the Cluster's Pods,
	// allowing them to be scheduled onto nodes with matching taints.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Specifies runtimeClassName for all Pods managed by this Cluster.
	//
	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`

	// Specifies the backup configuration of the Cluster.
	//
	// +optional
	Backup *ClusterBackup `json:"backup,omitempty"`

	// !!!!! The following fields may be deprecated in subsequent versions, please DO NOT rely on them for new requirements.

	// Describes how Pods are distributed across node.
	//
	// Deprecated since v0.9.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	Tenancy TenancyType `json:"tenancy,omitempty"`

	// Describes the availability policy, including zone, node, and none.
	//
	// Deprecated since v0.9.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	AvailabilityPolicy AvailabilityPolicyType `json:"availabilityPolicy,omitempty"`

	// Specifies the replicas of the first componentSpec, if the replicas of the first componentSpec is specified,
	// this value will be ignored.
	//
	// Deprecated since v0.9.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Specifies the resources of the first componentSpec, if the resources of the first componentSpec is specified,
	// this value will be ignored.
	//
	// Deprecated since v0.9.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	Resources ClusterResources `json:"resources,omitempty"`

	// Specifies the storage of the first componentSpec, if the storage of the first componentSpec is specified,
	// this value will be ignored.
	//
	// Deprecated since v0.9.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	Storage ClusterStorage `json:"storage,omitempty"`

	// The configuration of network.
	//
	// Deprecated since v0.9.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	Network *ClusterNetwork `json:"network,omitempty"`
}

type ClusterBackup struct {
	// Specifies whether automated backup is enabled for the Cluster.
	//
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Determines the duration to retain backups. Backups older than this period are automatically removed.
	//
	// For example, RetentionPeriod of `30d` will keep only the backups of last 30 days.
	// Sample duration format:
	//
	// - years: 	2y
	// - months: 	6mo
	// - days: 		30d
	// - hours: 	12h
	// - minutes: 	30m
	//
	// You can also combine the above durations. For example: 30d12h30m.
	// Default value is 7d.
	//
	// +kubebuilder:default="7d"
	// +optional
	RetentionPeriod dpv1alpha1.RetentionPeriod `json:"retentionPeriod,omitempty"`

	// Specifies the backup method to use, as defined in backupPolicy.
	//
	// +kubebuilder:validation:Required
	Method string `json:"method"`

	// The cron expression for the schedule. The timezone is in UTC. See https://en.wikipedia.org/wiki/Cron.
	//
	// +optional
	CronExpression string `json:"cronExpression,omitempty"`

	// Specifies the maximum time in minutes that the system will wait to start a missed backup job.
	// If the scheduled backup time is missed for any reason, the backup job must start within this deadline.
	// Values must be between 0 (immediate execution) and 1440 (one day).
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1440
	// +optional
	StartingDeadlineMinutes *int64 `json:"startingDeadlineMinutes,omitempty"`

	// Specifies the name of the backupRepo. If not set, the default backupRepo will be used.
	//
	// +optional
	RepoName string `json:"repoName,omitempty"`

	// Specifies whether to enable point-in-time recovery.
	//
	// +kubebuilder:default=false
	// +optional
	PITREnabled *bool `json:"pitrEnabled,omitempty"`
}

// ClusterResources is deprecated since v0.9.
type ClusterResources struct {
	// Specifies the amount of CPU resource the Cluster needs.
	// For more information, refer to: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	//
	// +optional
	CPU resource.Quantity `json:"cpu,omitempty"`

	// Specifies the amount of memory resource the Cluster needs.
	// For more information, refer to: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	//
	// +optional
	Memory resource.Quantity `json:"memory,omitempty"`
}

// ClusterStorage is deprecated since v0.9.
type ClusterStorage struct {
	// Specifies the amount of storage the Cluster needs.
	// For more information, refer to: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	//
	// +optional
	Size resource.Quantity `json:"size,omitempty"`
}

// ResourceMeta encapsulates metadata and configuration for referencing ConfigMaps and Secrets as volumes.
type ResourceMeta struct {
	// Name is the name of the referenced ConfigMap or Secret object. It must conform to DNS label standards.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// MountPoint is the filesystem path where the volume will be mounted.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern:=`^/[a-z]([a-z0-9\-]*[a-z0-9])?$`
	MountPoint string `json:"mountPoint"`

	// SubPath specifies a path within the volume from which to mount.
	//
	// +optional
	SubPath string `json:"subPath,omitempty"`

	// AsVolumeFrom lists the names of containers in which the volume should be mounted.
	//
	// +listType=set
	// +optional
	AsVolumeFrom []string `json:"asVolumeFrom,omitempty"`
}

// SecretRef defines a reference to a Secret.
type SecretRef struct {
	ResourceMeta `json:",inline"`

	// Secret specifies the Secret to be mounted as a volume.
	//
	// +kubebuilder:validation:Required
	Secret corev1.SecretVolumeSource `json:"secret"`
}

// ConfigMapRef defines a reference to a ConfigMap.
type ConfigMapRef struct {
	ResourceMeta `json:",inline"`

	// ConfigMap specifies the ConfigMap to be mounted as a volume.
	//
	// +kubebuilder:validation:Required
	ConfigMap corev1.ConfigMapVolumeSource `json:"configMap"`
}

// UserResourceRefs defines references to user-defined Secrets and ConfigMaps.
type UserResourceRefs struct {
	// SecretRefs defines the user-defined Secrets.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	SecretRefs []SecretRef `json:"secretRefs,omitempty"`

	// ConfigMapRefs defines the user-defined ConfigMaps.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ConfigMapRefs []ConfigMapRef `json:"configMapRefs,omitempty"`
}

// InstanceTemplate allows customization of individual replica configurations in a Component.
type InstanceTemplate struct {
	// Name specifies the unique name of the instance Pod created using this InstanceTemplate.
	// This name is constructed by concatenating the Component's name, the template's name, and the instance's ordinal
	// using the pattern: $(cluster.name)-$(component.name)-$(template.name)-$(ordinal). Ordinals start from 0.
	// The specified name overrides any default naming conventions or patterns.
	//
	// +kubebuilder:validation:MaxLength=54
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the number of instances (Pods) to create from this InstanceTemplate.
	// This field allows setting how many replicated instances of the Component,
	// with the specific overrides in the InstanceTemplate, are created.
	// The default value is 1. A value of 0 disables instance creation.
	//
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Specifies a map of key-value pairs to be merged into the Pod's existing annotations.
	// Existing keys will have their values overwritten, while new keys will be added to the annotations.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Specifies a map of key-value pairs that will be merged into the Pod's existing labels.
	// Values for existing keys will be overwritten, and new keys will be added.
	//
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Specifies an override for the first container's image in the Pod.
	//
	// +optional
	Image *string `json:"image,omitempty"`

	// Specifies the name of the node where the Pod should be scheduled.
	// If set, the Pod will be directly assigned to the specified node, bypassing the Kubernetes scheduler.
	// This is useful for controlling Pod placement on specific nodes.
	//
	// Important considerations:
	// - `nodeName` bypasses default scheduling constraints (e.g., resource requirements, node selectors, affinity rules).
	// - It is the user's responsibility to ensure the node is suitable for the Pod.
	// - If the node is unavailable, the Pod will remain in "Pending" state until the node is available or the Pod is deleted.
	//
	// +optional
	NodeName *string `json:"nodeName,omitempty"`

	// Defines NodeSelector to override.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations specifies a list of tolerations to be applied to the Pod, allowing it to tolerate node taints.
	// This field can be used to add new tolerations or override existing ones.
	//
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Specifies an override for the resource requirements of the first container in the Pod.
	// This field allows for customizing resource allocation (CPU, memory, etc.) for the container.
	//
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Defines Env to override.
	// Add new or override existing envs.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Defines Volumes to override.
	// Add new or override existing volumes.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Defines VolumeMounts to override.
	// Add new or override existing volume mounts of the first container in the Pod.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Defines VolumeClaimTemplates to override.
	// Add new or override existing volume claim templates.
	// +optional
	VolumeClaimTemplates []ClusterComponentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty"`
}

// ClusterStatus defines the observed state of the Cluster.
type ClusterStatus struct {
	// The most recent generation number of the Cluster object that has been observed by the controller.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// The current phase of the Cluster includes:
	// `Creating`, `Running`, `Updating`, `Stopping`, `Stopped`, `Deleting`, `Failed`, `Abnormal`.
	//
	// +optional
	Phase ClusterPhase `json:"phase,omitempty"`

	// Provides additional information about the current phase.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// Records the current status information of all Components within the Cluster.
	//
	// +optional
	Components map[string]ClusterComponentStatus `json:"components,omitempty"`

	// Represents the generation number of the referenced ClusterDefinition.
	//
	// +optional
	ClusterDefGeneration int64 `json:"clusterDefGeneration,omitempty"`

	// Represents a list of detailed status of the Cluster object.
	// Each condition in the list provides real-time information about certain aspect of the Cluster object.
	//
	// This field is crucial for administrators and developers to monitor and respond to changes within the Cluster.
	// It provides a history of state transitions and a snapshot of the current state that can be used for
	// automated logic or direct inspection.
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ShardingSpec defines how KubeBlocks manage dynamic provisioned shards.
// A typical design pattern for distributed databases is to distribute data across multiple shards,
// with each shard consisting of multiple replicas.
// Therefore, KubeBlocks supports representing a shard with a Component and dynamically instantiating Components
// using a template when shards are added.
// When shards are removed, the corresponding Components are also deleted.
type ShardingSpec struct {
	// Represents the common parent part of all shard names.
	// This identifier is included as part of the Service DNS name and must comply with IANA service naming rules.
	// It is used to generate the names of underlying Components following the pattern `$(shardingSpec.name)-$(ShardID)`.
	// ShardID is a random string that is appended to the Name to generate unique identifiers for each shard.
	// For example, if the sharding specification name is "my-shard" and the ShardID is "abc", the resulting Component name
	// would be "my-shard-abc".
	//
	// Note that the name defined in Component template(`shardingSpec.template.name`) will be disregarded
	// when generating the Component names of the shards. The `shardingSpec.name` field takes precedence.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=15
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`

	// The template for generating Components for shards, where each shard consists of one Component.
	// This field is of type ClusterComponentSpec, which encapsulates all the required details and
	// definitions for creating and managing the Components.
	// KubeBlocks uses this template to generate a set of identical Components or shards.
	// All the generated Components will have the same specifications and definitions as specified in the `template` field.
	//
	// This allows for the creation of multiple Components with consistent configurations,
	// enabling sharding and distribution of workloads across Components.
	//
	// +kubebuilder:validation:Required
	Template ClusterComponentSpec `json:"template"`

	// Specifies the desired number of shards.
	// Users can declare the desired number of shards through this field.
	// KubeBlocks dynamically creates and deletes Components based on the difference
	// between the desired and actual number of shards.
	// KubeBlocks provides lifecycle management for sharding, including:
	//
	// - Executing the postProvision Action defined in the ComponentDefinition when the number of shards increases.
	//   This allows for custom actions to be performed after a new shard is provisioned.
	// - Executing the preTerminate Action defined in the ComponentDefinition when the number of shards decreases.
	//   This enables custom cleanup or data migration tasks to be executed before a shard is terminated.
	//   Resources and data associated with the corresponding Component will also be deleted.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=2048
	Shards int32 `json:"shards,omitempty"`
}

// ClusterComponentSpec defines the specification of a Component within a Cluster.
// TODO +kubebuilder:validation:XValidation:rule="!has(oldSelf.componentDefRef) || has(self.componentDefRef)", message="componentDefRef is required once set"
// TODO +kubebuilder:validation:XValidation:rule="!has(oldSelf.componentDef) || has(self.componentDef)", message="componentDef is required once set"
type ClusterComponentSpec struct {
	// Specifies the Component's name.
	// It's part of the Service DNS name and must comply with the IANA service naming rule.
	// The name is optional when ClusterComponentSpec is used as a template (e.g., in `shardingSpec`),
	// but required otherwise.
	//
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	// TODO +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	// +optional
	Name string `json:"name"`

	// References a ClusterComponentDefinition defined in the `clusterDefinition.spec.componentDef` field.
	// Must comply with the IANA service naming rule.
	//
	// Deprecated since v0.9,
	// because defining Components in `clusterDefinition.spec.componentDef` field has been deprecated.
	// This field is replaced by the `componentDef` field, use `componentDef` instead.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	// TODO +kubebuilder:validation:XValidation:rule="self == oldSelf",message="componentDefRef is immutable"
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0, consider using the ComponentDef instead"
	// +optional
	ComponentDefRef string `json:"componentDefRef,omitempty"`

	// References the name of a ComponentDefinition object.
	// The ComponentDefinition specifies the behavior and characteristics of the Component.
	// If both `componentDefRef` and `componentDef` are provided,
	// the `componentDef` will take precedence over `componentDefRef`.
	//
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	ComponentDef string `json:"componentDef,omitempty"`

	// ServiceVersion specifies the version of the Service expected to be provisioned by this Component.
	// The version should follow the syntax and semantics of the "Semantic Versioning" specification (http://semver.org/).
	// If no version is specified, the latest available version will be used.
	//
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceVersion string `json:"serviceVersion,omitempty"`

	// References the class defined in ComponentClassDefinition.
	//
	// Deprecated since v0.9.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:deprecatedversion:warning="Due to the lack of practical use cases, this field is deprecated from KB 0.9.0."
	// +optional
	ClassDefRef *ClassDefRef `json:"classDefRef,omitempty"`

	// Defines a list of ServiceRef for a Component, enabling access to both external services and
	// Services provided by other Clusters.
	//
	// Types of services:
	//
	// - External services: Not managed by KubeBlocks or managed by a different KubeBlocks operator;
	//   Require a ServiceDescriptor for connection details.
	// - Services provided by a Cluster: Managed by the same KubeBlocks operator;
	//   identified using Cluster, Component and Service names.
	//
	// ServiceRefs with identical `serviceRef.name` in the same Cluster are considered the same.
	//
	// Example:
	// ```yaml
	// serviceRefs:
	//   - name: "redis-sentinel"
	//     serviceDescriptor:
	//       name: "external-redis-sentinel"
	//   - name: "postgres-cluster"
	//     clusterServiceSelector:
	//       cluster: "my-postgres-cluster"
	//       service:
	//         component: "postgresql"
	// ```
	// The example above includes ServiceRefs to an external Redis Sentinel service and a PostgreSQL Cluster.
	//
	// +optional
	ServiceRefs []ServiceRef `json:"serviceRefs,omitempty"`

	// Specifies which types of logs should be collected for the Component.
	// The log types are defined in the `componentDefinition.spec.logConfigs` field with the LogConfig entries.
	//
	// The elements in the `enabledLogs` array correspond to the names of the LogConfig entries.
	// For example, if the `componentDefinition.spec.logConfigs` defines LogConfig entries with
	// names "slow_query_log" and "error_log",
	// you can enable the collection of these logs by including their names in the `enabledLogs` array:
	// ```yaml
	// enabledLogs:
	// - slow_query_log
	// - error_log
	// ```
	//
	// +listType=set
	// +optional
	EnabledLogs []string `json:"enabledLogs,omitempty"`

	// Specifies Labels to override or add for underlying Pods.
	//
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Specifies Annotations to override or add for underlying Pods.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// List of environment variables to add.
	// These environment variables will be placed after the environment variables declared in the Pod.
	//
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Specifies the desired number of replicas in the Component for enhancing availability and durability, or load balancing.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`

	// Specifies a group of affinity scheduling rules for the Component.
	// It allows users to control how the Component's Pods are scheduled onto nodes in the K8s cluster.
	//
	// +optional
	Affinity *Affinity `json:"affinity,omitempty"`

	// Allows Pods to be scheduled onto nodes with matching taints.
	// Each toleration in the array allows the Pod to tolerate node taints based on
	// specified `key`, `value`, `effect`, and `operator`.
	//
	// - The `key`, `value`, and `effect` identify the taint that the toleration matches.
	// - The `operator` determines how the toleration matches the taint.
	//
	// Pods with matching tolerations are allowed to be scheduled on tainted nodes, typically reserved for specific purposes.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Specifies the resources required by the Component.
	// It allows defining the CPU, memory requirements and limits for the Component's containers.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Specifies a list of PersistentVolumeClaim templates that represent the storage requirements for the Component.
	// Each template specifies the desired characteristics of a persistent volume, such as storage class,
	// size, and access modes.
	// These templates are used to dynamically provision persistent volumes for the Component.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +optional
	VolumeClaimTemplates []ClusterComponentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// List of volumes to override.
	//
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Overrides services defined in referenced ComponentDefinition and expose endpoints that can be accessed by clients.
	//
	// +optional
	Services []ClusterComponentService `json:"services,omitempty"`

	// Overrides system accounts defined in referenced ComponentDefinition.
	//
	// +optional
	SystemAccounts []ComponentSystemAccount `json:"systemAccounts,omitempty"`

	// Defines the strategy for switchover and failover when workloadType is Replication.
	//
	// Deprecated since v0.9.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	SwitchPolicy *ClusterSwitchPolicy `json:"switchPolicy,omitempty"`

	// A boolean flag that indicates whether the Component should use Transport Layer Security (TLS)
	// for secure communication.
	// When set to true, the Component will be configured to use TLS encryption for its network connections.
	// This ensures that the data transmitted between the Component and its clients or other Components is encrypted
	// and protected from unauthorized access.
	// If TLS is enabled, the Component may require additional configuration, such as specifying TLS certificates and keys,
	// to properly set up the secure communication channel.
	//
	// +optional
	TLS bool `json:"tls,omitempty"`

	// Specifies the configuration for the TLS certificates issuer.
	// It allows defining the issuer name and the reference to the secret containing the TLS certificates and key.
	// The secret should contain the CA certificate, TLS certificate, and private key in the specified keys.
	// Required when TLS is enabled.
	//
	// +optional
	Issuer *Issuer `json:"issuer,omitempty"`

	// Specifies the name of the ServiceAccount required by the running Component.
	// This ServiceAccount is used to grant necessary permissions for the Component's Pods to interact
	// with other Kubernetes resources, such as modifying Pod labels or sending events.
	//
	// Defaults:
	// If not specified, KubeBlocks automatically assigns a default ServiceAccount named "kb-{cluster.name}",
	// bound to a default role installed together with KubeBlocks.
	//
	// Future Changes:
	// Future versions might change the default ServiceAccount creation strategy to one per Component,
	// potentially revising the naming to "kb-{cluster.name}-{component.name}".
	//
	// Users can override the automatic ServiceAccount assignment by explicitly setting the name of
	// an existed ServiceAccount in this field.
	//
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Defines the update strategy for the Component.
	//
	// Deprecated since v0.9.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	UpdateStrategy *UpdateStrategy `json:"updateStrategy,omitempty"`

	// Allows users to specify custom ConfigMaps and Secrets to be mounted as volumes
	// in the Cluster's Pods.
	// This is useful in scenarios where users need to provide additional resources to the Cluster, such as:
	//
	// - Mounting custom scripts or configuration files during Cluster startup.
	// - Mounting Secrets as volumes to provide sensitive information, like S3 AK/SK, to the Cluster.
	//
	// +optional
	UserResourceRefs *UserResourceRefs `json:"userResourceRefs,omitempty"`

	// Allows for the customization of configuration values for each instance within a Component.
	// An instance represent a single replica (Pod and associated K8s resources like PVCs, Services, and ConfigMaps).
	// While instances typically share a common configuration as defined in the ClusterComponentSpec,
	// they can require unique settings in various scenarios:
	//
	// For example:
	// - A database Component might require different resource allocations for primary and secondary instances,
	//   with primaries needing more resources.
	// - During a rolling upgrade, a Component may first update the image for one or a few instances,
	//   and then update the remaining instances after verifying that the updated instances are functioning correctly.
	//
	// InstanceTemplate allows for specifying these unique configurations per instance.
	// Each instance's name is constructed using the pattern: $(component.name)-$(template.name)-$(ordinal),
	// starting with an ordinal of 0.
	// It is crucial to maintain unique names for each InstanceTemplate to avoid conflicts.
	//
	// The sum of replicas across all InstanceTemplates should not exceed the total number of replicas specified for the Component.
	// Any remaining replicas will be generated using the default template and will follow the default naming rules.
	//
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Instances []InstanceTemplate `json:"instances,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Specifies the names of instances to be transitioned to offline status.
	//
	// Marking an instance as offline results in the following:
	//
	// 1. The associated Pod is stopped, and its PersistentVolumeClaim (PVC) is retained for potential
	//    future reuse or data recovery, but it is no longer actively used.
	// 2. The ordinal number assigned to this instance is preserved, ensuring it remains unique
	//    and avoiding conflicts with new instances.
	//
	// Setting instances to offline allows for a controlled scale-in process, preserving their data and maintaining
	// ordinal consistency within the Cluster.
	// Note that offline instances and their associated resources, such as PVCs, are not automatically deleted.
	// The administrator must manually manage the cleanup and removal of these resources when they are no longer needed.
	//
	// +optional
	OfflineInstances []string `json:"offlineInstances,omitempty"`

	// Determines whether metrics exporter information is annotated on the Component's headless Service.
	//
	// If set to true, the following annotations will not be patched into the Service:
	//
	// - "monitor.kubeblocks.io/path"
	// - "monitor.kubeblocks.io/port"
	// - "monitor.kubeblocks.io/scheme"
	//
	// These annotations allow the Prometheus installed by KubeBlocks to discover and scrape metrics from the exporter.
	//
	// +optional
	DisableExporter *bool `json:"disableExporter,omitempty"`

	// Deprecated since v0.9
	// Determines whether metrics exporter information is annotated on the Component's headless Service.
	//
	// If set to true, the following annotations will be patched into the Service:
	//
	// - "monitor.kubeblocks.io/path"
	// - "monitor.kubeblocks.io/port"
	// - "monitor.kubeblocks.io/scheme"
	//
	// These annotations allow the Prometheus installed by KubeBlocks to discover and scrape metrics from the exporter.
	//
	// +optional
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.10.0"
	Monitor *bool `json:"monitor,omitempty"`
}

type ComponentMessageMap map[string]string

// ClusterComponentStatus records Component status.
type ClusterComponentStatus struct {
	// Specifies the current state of the Component.
	Phase ClusterComponentPhase `json:"phase,omitempty"`

	// Records detailed information about the Component in its current phase.
	// The keys are either podName, deployName, or statefulSetName, formatted as 'ObjectKind/Name'.
	//
	// +optional
	Message ComponentMessageMap `json:"message,omitempty"`

	// Checks if all Pods of the Component are ready.
	//
	// +optional
	PodsReady *bool `json:"podsReady,omitempty"`

	// Indicates the time when all Component Pods became ready.
	// This is the readiness time of the last Component Pod.
	//
	// +optional
	PodsReadyTime *metav1.Time `json:"podsReadyTime,omitempty"`

	// Represents the status of the members.
	//
	// +optional
	MembersStatus []workloads.MemberStatus `json:"membersStatus,omitempty"`
}

// ClusterSwitchPolicy defines the switch policy for a Cluster.
//
// Deprecated since v0.9.
type ClusterSwitchPolicy struct {
	// Type specifies the type of switch policy to be applied.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Noop
	// +optional
	Type SwitchPolicyType `json:"type"`
}

type ClusterComponentVolumeClaimTemplate struct {
	// Refers to the name of a volumeMount defined in either:
	//
	// - `componentDefinition.spec.runtime.containers[*].volumeMounts`
	// - `clusterDefinition.spec.componentDefs[*].podSpec.containers[*].volumeMounts` (deprecated)
	//
	// The value of `name` must match the `name` field of a volumeMount specified in the corresponding `volumeMounts` array.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Defines the desired characteristics of a PersistentVolumeClaim that will be created for the volume
	// with the mount name specified in the `name` field.
	//
	// When a Pod is created for this ClusterComponent, a new PVC will be created based on the specification
	// defined in the `spec` field. The PVC will be associated with the volume mount specified by the `name` field.
	//
	// +optional
	Spec PersistentVolumeClaimSpec `json:"spec,omitempty"`
}

func (r *ClusterComponentVolumeClaimTemplate) toVolumeClaimTemplate() corev1.PersistentVolumeClaimTemplate {
	return corev1.PersistentVolumeClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.Name,
		},
		Spec: r.Spec.ToV1PersistentVolumeClaimSpec(),
	}
}

type PersistentVolumeClaimSpec struct {
	// Contains the desired access modes the volume should have.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty" protobuf:"bytes,1,rep,name=accessModes,casttype=PersistentVolumeAccessMode"`

	// Represents the minimum resources the volume should have.
	// If the RecoverVolumeExpansionFailure feature is enabled, users are allowed to specify resource requirements that
	// are lower than the previous value but must still be higher than the capacity recorded in the status field of the claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`

	// The name of the StorageClass required by the claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1.
	//
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty" protobuf:"bytes,5,opt,name=storageClassName"`

	// Defines what type of volume is required by the claim, either Block or Filesystem.
	//
	// +optional
	VolumeMode *corev1.PersistentVolumeMode `json:"volumeMode,omitempty" protobuf:"bytes,6,opt,name=volumeMode,casttype=PersistentVolumeMode"`
}

// ToV1PersistentVolumeClaimSpec converts to corev1.PersistentVolumeClaimSpec.
func (r *PersistentVolumeClaimSpec) ToV1PersistentVolumeClaimSpec() corev1.PersistentVolumeClaimSpec {
	return corev1.PersistentVolumeClaimSpec{
		AccessModes:      r.AccessModes,
		Resources:        r.Resources,
		StorageClassName: r.getStorageClassName(viper.GetString(constant.CfgKeyDefaultStorageClass)),
		VolumeMode:       r.VolumeMode,
	}
}

// getStorageClassName returns PersistentVolumeClaimSpec.StorageClassName if a value is assigned; otherwise,
// it returns preferSC argument.
func (r *PersistentVolumeClaimSpec) getStorageClassName(preferSC string) *string {
	if r.StorageClassName != nil && *r.StorageClassName != "" {
		return r.StorageClassName
	}
	if preferSC != "" {
		return &preferSC
	}
	return nil
}

type Affinity struct {
	// Specifies the anti-affinity level of Pods within a Component.
	// It determines how pods should be spread across nodes to improve availability and performance.
	// It can have the following values: `Preferred` and `Required`.
	// The default value is `Preferred`.
	//
	// +kubebuilder:default=Preferred
	// +optional
	PodAntiAffinity PodAntiAffinity `json:"podAntiAffinity,omitempty"`

	// Represents the key of node labels used to define the topology domain for Pod anti-affinity
	// and Pod spread constraints.
	//
	// In K8s, a topology domain is a set of nodes that have the same value for a specific label key.
	// Nodes with labels containing any of the specified TopologyKeys and identical values are considered
	// to be in the same topology domain.
	//
	// Note: The concept of topology in the context of K8s TopologyKeys is different from the concept of
	// topology in the ClusterDefinition.
	//
	// When a Pod has anti-affinity or spread constraints specified, Kubernetes will attempt to schedule the
	// Pod on nodes with different values for the specified TopologyKeys.
	// This ensures that Pods are spread across different topology domains, promoting high availability and
	// reducing the impact of node failures.
	//
	// Some well-known label keys, such as `kubernetes.io/hostname` and `topology.kubernetes.io/zone`,
	// are often used as TopologyKey.
	// These keys represent the hostname and zone of a node, respectively.
	// By including these keys in the TopologyKeys list, Pods will be spread across nodes with
	// different hostnames or zones.
	//
	// In addition to the well-known keys, users can also specify custom label keys as TopologyKeys.
	// This allows for more flexible and custom topology definitions based on the specific needs
	// of the application or environment.
	//
	// The TopologyKeys field is a slice of strings, where each string represents a label key.
	// The order of the keys in the slice does not matter.
	//
	// +listType=set
	// +optional
	TopologyKeys []string `json:"topologyKeys,omitempty"`

	// Indicates the node labels that must be present on nodes for pods to be scheduled on them.
	// It is a map where the keys are the label keys and the values are the corresponding label values.
	// Pods will only be scheduled on nodes that have all the specified labels with the corresponding values.
	//
	// For example, if NodeLabels is set to {"nodeType": "ssd", "environment": "production"},
	// pods will only be scheduled on nodes that have both the "nodeType" label with value "ssd"
	// and the "environment" label with value "production".
	//
	// This field allows users to control Pod placement based on specific node labels.
	// It can be used to ensure that Pods are scheduled on nodes with certain characteristics,
	// such as specific hardware (e.g., SSD), environment (e.g., production, staging),
	// or any other custom labels assigned to nodes.
	//
	// +optional
	NodeLabels map[string]string `json:"nodeLabels,omitempty"`

	// Determines the level of resource isolation between Pods.
	// It can have the following values: `SharedNode` and `DedicatedNode`.
	//
	// - SharedNode: Allow that multiple Pods may share the same node, which is the default behavior of K8s.
	// - DedicatedNode: Each Pod runs on a dedicated node, ensuring that no two Pods share the same node.
	//   In other words, if a Pod is already running on a node, no other Pods will be scheduled on that node.
	//   Which provides a higher level of isolation and resource guarantee for Pods.
	//
	//  The default value is `SharedNode`.
	//
	// +kubebuilder:default=SharedNode
	// +optional
	Tenancy TenancyType `json:"tenancy,omitempty"`
}

type TLSConfig struct {
	// A boolean flag that indicates whether the Component should use Transport Layer Security (TLS)
	// for secure communication.
	// When set to true, the Component will be configured to use TLS encryption for its network connections.
	// This ensures that the data transmitted between the Component and its clients or other Components is encrypted
	// and protected from unauthorized access.
	// If TLS is enabled, the Component may require additional configuration,
	// such as specifying TLS certificates and keys, to properly set up the secure communication channel.
	//
	// +kubebuilder:default=false
	// +optional
	Enable bool `json:"enable,omitempty"`

	// Specifies the configuration for the TLS certificates issuer.
	// It allows defining the issuer name and the reference to the secret containing the TLS certificates and key.
	// The secret should contain the CA certificate, TLS certificate, and private key in the specified keys.
	// Required when TLS is enabled.
	//
	// +optional
	Issuer *Issuer `json:"issuer,omitempty"`
}

// Issuer defines the TLS certificates issuer for the Cluster.
type Issuer struct {
	// The issuer for TLS certificates.
	// It only allows two enum values: `KubeBlocks` and `UserProvided`.
	//
	// - `KubeBlocks` indicates that the self-signed TLS certificates generated by the KubeBlocks Operator will be used.
	// - `UserProvided` means that the user is responsible for providing their own CA, Cert, and Key.
	//   In this case, the user-provided CA certificate, server certificate, and private key will be used
	//   for TLS communication.
	//
	// +kubebuilder:validation:Enum={KubeBlocks, UserProvided}
	// +kubebuilder:default=KubeBlocks
	// +kubebuilder:validation:Required
	Name IssuerName `json:"name"`

	// SecretRef is the reference to the secret that contains user-provided certificates.
	// It is required when the issuer is set to `UserProvided`.
	//
	// +optional
	SecretRef *TLSSecretRef `json:"secretRef,omitempty"`
}

// TLSSecretRef defines Secret contains Tls certs
type TLSSecretRef struct {
	// Name of the Secret that contains user-provided certificates.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Key of CA cert in Secret
	// +kubebuilder:validation:Required
	CA string `json:"ca"`

	// Key of Cert in Secret
	// +kubebuilder:validation:Required
	Cert string `json:"cert"`

	// Key of TLS private key in Secret
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

type ClusterComponentService struct {
	// References the ComponentService name defined in the `componentDefinition.spec.services[*].name`.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=25
	Name string `json:"name"`

	// Determines how the Service is exposed. Valid options are `ClusterIP`, `NodePort`, and `LoadBalancer`.
	//
	// - `ClusterIP` allocates a Cluster-internal IP address for load-balancing to endpoints.
	//    Endpoints are determined by the selector or if that is not specified,
	//    they are determined by manual construction of an Endpoints object or EndpointSlice objects.
	// - `NodePort` builds on ClusterIP and allocates a port on every node which routes to the same endpoints as the ClusterIP.
	// - `LoadBalancer` builds on NodePort and creates an external load-balancer (if supported in the current cloud)
	//    which routes to the same endpoints as the ClusterIP.
	//
	// Note: although K8s Service type allows the 'ExternalName' type, it is not a valid option for ClusterComponentService.
	//
	// For more info, see:
	// https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types.
	//
	// +kubebuilder:default=ClusterIP
	// +kubebuilder:validation:Enum={ClusterIP,NodePort,LoadBalancer}
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`

	// If ServiceType is LoadBalancer, cloud provider related parameters can be put here.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Indicates whether to generate individual Services for each Pod.
	// If set to true, a separate Service will be created for each Pod in the Cluster.
	//
	// +optional
	PodService *bool `json:"podService,omitempty"`
}

type ComponentSystemAccount struct {
	// The name of the system account.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the policy for generating the account's password.
	//
	// This field is immutable once set.
	//
	// +optional
	PasswordConfig *PasswordConfig `json:"passwordConfig,omitempty"`

	// Refers to the secret from which data will be copied to create the new account.
	//
	// This field is immutable once set.
	//
	// +optional
	SecretRef *ProvisionSecretRef `json:"secretRef,omitempty"`
}

// ClassDefRef is deprecated since v0.9.
type ClassDefRef struct {
	// Specifies the name of the ComponentClassDefinition.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	Name string `json:"name,omitempty"`

	// Defines the name of the class that is defined in the ComponentClassDefinition.
	//
	// +kubebuilder:validation:Required
	Class string `json:"class"`
}

// ClusterNetwork is deprecated since v0.9.
type ClusterNetwork struct {
	// Indicates whether the host network can be accessed. By default, this is set to false.
	//
	// +kubebuilder:default=false
	// +optional
	HostNetworkAccessible bool `json:"hostNetworkAccessible,omitempty"`

	// Indicates whether the network is accessible to the public. By default, this is set to false.
	//
	// +kubebuilder:default=false
	// +optional
	PubliclyAccessible bool `json:"publiclyAccessible,omitempty"`
}

type ServiceRef struct {
	// Specifies the identifier of the service reference declaration.
	// It corresponds to the serviceRefDeclaration name defined in either:
	//
	// - `componentDefinition.spec.serviceRefDeclarations[*].name`
	// - `clusterDefinition.spec.componentDefs[*].serviceRefDeclarations[*].name` (deprecated)
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the namespace of the referenced Cluster or the namespace of the referenced ServiceDescriptor object.
	// If not provided, the referenced Cluster and ServiceDescriptor will be searched in the namespace of the current
	// Cluster by default.
	//
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Specifies the name of the KubeBlocks Cluster being referenced.
	// This is used when services from another KubeBlocks Cluster are consumed.
	//
	// By default, the referenced KubeBlocks Cluster's `clusterDefinition.spec.connectionCredential`
	// will be utilized to bind to the current Component. This credential should include:
	// `endpoint`, `port`, `username`, and `password`.
	//
	// Note:
	//
	// - The `ServiceKind` and `ServiceVersion` specified in the service reference within the
	//   ClusterDefinition are not validated when using this approach.
	// - If both `cluster` and `serviceDescriptor` are present, `cluster` will take precedence.
	//
	// Deprecated since v0.9 since `clusterDefinition.spec.connectionCredential` is deprecated,
	// use `clusterServiceSelector` instead.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	Cluster string `json:"cluster,omitempty"`

	// References a service provided by another KubeBlocks Cluster.
	// It specifies the ClusterService and the account credentials needed for access.
	//
	// +optional
	ClusterServiceSelector *ServiceRefClusterSelector `json:"clusterServiceSelector,omitempty"`

	// Specifies the name of the ServiceDescriptor object that describes a service provided by external sources.
	//
	// When referencing a service provided by external sources, a ServiceDescriptor object is required to establish
	// the service binding.
	// The `serviceDescriptor.spec.serviceKind` and `serviceDescriptor.spec.serviceVersion` should match the serviceKind
	// and serviceVersion declared in the definition.
	//
	// If both `cluster` and `serviceDescriptor` are specified, the `cluster` takes precedence.
	//
	// +optional
	ServiceDescriptor string `json:"serviceDescriptor,omitempty"`
}

type ServiceRefClusterSelector struct {
	// The name of the Cluster being referenced.
	//
	// +kubebuilder:validation:Required
	Cluster string `json:"cluster"`

	// Identifies a ClusterService from the list of Services defined in `cluster.spec.services` of the referenced Cluster.
	//
	// +optional
	Service *ServiceRefServiceSelector `json:"service,omitempty"`

	// Specifies the SystemAccount to authenticate and establish a connection with the referenced Cluster.
	// The SystemAccount should be defined in `componentDefinition.spec.systemAccounts`
	// of the Component providing the service in the referenced Cluster.
	//
	// +optional
	Credential *ServiceRefCredentialSelector `json:"credential,omitempty"`
}

type ServiceRefServiceSelector struct {
	// The name of the Component where the Service resides in.
	//
	// It is required when referencing a Component's Service.
	//
	// +optional
	Component string `json:"component,omitempty"`

	// The name of the Service to be referenced.
	//
	// Leave it empty to reference the default Service. Set it to "headless" to reference the default headless Service.
	//
	// If the referenced Service is of pod-service type (a Service per Pod), there will be multiple Service objects matched,
	// and the resolved value will be presented in the following format: service1.name,service2.name...
	//
	// +kubebuilder:validation:Required
	Service string `json:"service"`

	// The port name of the Service to be referenced.
	//
	// If there is a non-zero node-port exist for the matched Service port, the node-port will be selected first.
	//
	// If the referenced Service is of pod-service type (a Service per Pod), there will be multiple Service objects matched,
	// and the resolved value will be presented in the following format: service1.name:port1,service2.name:port2...
	//
	// +optional
	Port string `json:"port,omitempty"`
}

type ServiceRefCredentialSelector struct {
	// The name of the Component where the credential resides in.
	//
	// +kubebuilder:validation:Required
	Component string `json:"component"`

	// The name of the credential (SystemAccount) to reference.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all}
// +kubebuilder:printcolumn:name="CLUSTER-DEFINITION",type="string",JSONPath=".spec.clusterDefinitionRef",description="ClusterDefinition referenced by cluster."
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.clusterVersionRef",description="Cluster Application Version."
// +kubebuilder:printcolumn:name="TERMINATION-POLICY",type="string",JSONPath=".spec.terminationPolicy",description="Cluster termination policy."
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="Cluster Status."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Cluster offers a unified management interface for a wide variety of database and storage systems:
//
// - Relational databases: MySQL, PostgreSQL, MariaDB
// - NoSQL databases: Redis, MongoDB
// - KV stores: ZooKeeper, etcd
// - Analytics systems: ElasticSearch, OpenSearch, ClickHouse, Doris, StarRocks, Solr
// - Message queues: Kafka, Pulsar
// - Distributed SQL: TiDB, OceanBase
// - Vector databases: Qdrant, Milvus, Weaviate
// - Object storage: Minio
//
// KubeBlocks utilizes an abstraction layer to encapsulate the characteristics of these diverse systems.
// A Cluster is composed of multiple Components, each defined by vendors or KubeBlocks Addon developers via ComponentDefinition,
// arranged in Directed Acyclic Graph (DAG) topologies.
// The topologies, defined in a ClusterDefinition, coordinate reconciliation across Cluster's lifecycle phases:
// Creating, Running, Updating, Stopping, Stopped, Deleting.
// Lifecycle management ensures that each Component operates in harmony, executing appropriate actions at each lifecycle stage.
//
// For sharded-nothing architecture, the Cluster supports managing multiple shards,
// each shard managed by a separate Component, supporting dynamic resharding.
//
// The Cluster object is aimed to maintain the overall integrity and availability of a database cluster,
// serves as the central control point, abstracting the complexity of multiple-component management,
// and providing a unified interface for cluster-wide operations.
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster.
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}

func (r Cluster) IsDeleting() bool {
	if r.GetDeletionTimestamp().IsZero() {
		return false
	}
	return r.Spec.TerminationPolicy != DoNotTerminate
}

func (r Cluster) IsUpdating() bool {
	return r.Status.ObservedGeneration != r.Generation
}

func (r Cluster) IsStatusUpdating() bool {
	return !r.IsDeleting() && !r.IsUpdating()
}

// GetVolumeClaimNames gets all PVC names of component compName.
//
// r.Spec.GetComponentByName(compName).VolumeClaimTemplates[*].Name will be used if no claimNames provided
//
// nil return if:
// 1. component compName not found or
// 2. len(VolumeClaimTemplates)==0 or
// 3. any claimNames not found
func (r *Cluster) GetVolumeClaimNames(compName string, claimNames ...string) []string {
	if r == nil {
		return nil
	}
	comp := r.Spec.GetComponentByName(compName)
	if comp == nil {
		return nil
	}
	if len(comp.VolumeClaimTemplates) == 0 {
		return nil
	}
	if len(claimNames) == 0 {
		for _, template := range comp.VolumeClaimTemplates {
			claimNames = append(claimNames, template.Name)
		}
	}
	allExist := true
	for _, name := range claimNames {
		found := false
		for _, template := range comp.VolumeClaimTemplates {
			if template.Name == name {
				found = true
				break
			}
		}
		if !found {
			allExist = false
			break
		}
	}
	if !allExist {
		return nil
	}

	pvcNames := make([]string, 0)
	for _, claimName := range claimNames {
		for i := 0; i < int(comp.Replicas); i++ {
			pvcName := fmt.Sprintf("%s-%s-%s-%d", claimName, r.Name, compName, i)
			pvcNames = append(pvcNames, pvcName)
		}
	}
	return pvcNames
}

// GetComponentByName gets component by name.
func (r ClusterSpec) GetComponentByName(componentName string) *ClusterComponentSpec {
	for _, v := range r.ComponentSpecs {
		if v.Name == componentName {
			return &v
		}
	}
	return nil
}

func (r ClusterSpec) GetShardingByName(shardingName string) *ShardingSpec {
	for _, v := range r.ShardingSpecs {
		if v.Name == shardingName {
			return &v
		}
	}
	return nil
}

// GetComponentDefRefName gets the name of referenced component definition.
func (r ClusterSpec) GetComponentDefRefName(componentName string) string {
	for _, component := range r.ComponentSpecs {
		if componentName == component.Name {
			return component.ComponentDefRef
		}
	}
	return ""
}

// ValidateEnabledLogs validates enabledLogs config in cluster.yaml, and returns metav1.Condition when detecting invalid values.
func (r ClusterSpec) ValidateEnabledLogs(cd *ClusterDefinition) error {
	message := make([]string, 0)
	for _, comp := range r.ComponentSpecs {
		invalidLogNames := cd.ValidateEnabledLogConfigs(comp.ComponentDefRef, comp.EnabledLogs)
		if len(invalidLogNames) == 0 {
			continue
		}
		message = append(message, fmt.Sprintf("EnabledLogs: %s are not defined in Component: %s of the clusterDefinition", invalidLogNames, comp.Name))
	}
	if len(message) > 0 {
		return errors.New(strings.Join(message, ";"))
	}
	return nil
}

// GetDefNameMappingComponents returns ComponentDefRef name mapping ClusterComponentSpec.
func (r ClusterSpec) GetDefNameMappingComponents() map[string][]ClusterComponentSpec {
	m := map[string][]ClusterComponentSpec{}
	for _, c := range r.ComponentSpecs {
		v := m[c.ComponentDefRef]
		v = append(v, c)
		m[c.ComponentDefRef] = v
	}
	return m
}

// GetMessage gets message map deep copy object.
func (r ClusterComponentStatus) GetMessage() ComponentMessageMap {
	messageMap := map[string]string{}
	for k, v := range r.Message {
		messageMap[k] = v
	}
	return messageMap
}

// SetMessage overrides message map object.
func (r *ClusterComponentStatus) SetMessage(messageMap ComponentMessageMap) {
	if r == nil {
		return
	}
	r.Message = messageMap
}

// SetObjectMessage sets K8s workload message to component status message map.
func (r *ClusterComponentStatus) SetObjectMessage(objectKind, objectName, message string) {
	if r == nil {
		return
	}
	if r.Message == nil {
		r.Message = map[string]string{}
	}
	messageKey := fmt.Sprintf("%s/%s", objectKind, objectName)
	r.Message[messageKey] = message
}

// GetObjectMessage gets the k8s workload message in component status message map
func (r ClusterComponentStatus) GetObjectMessage(objectKind, objectName string) string {
	messageKey := fmt.Sprintf("%s/%s", objectKind, objectName)
	return r.Message[messageKey]
}

// SetObjectMessage sets k8s workload message to component status message map
func (r ComponentMessageMap) SetObjectMessage(objectKind, objectName, message string) {
	if r == nil {
		return
	}
	messageKey := fmt.Sprintf("%s/%s", objectKind, objectName)
	r[messageKey] = message
}

// SetComponentStatus does safe operation on ClusterStatus.Components map object update.
func (r *ClusterStatus) SetComponentStatus(name string, status ClusterComponentStatus) {
	r.checkedInitComponentsMap()
	r.Components[name] = status
}

func (r *ClusterStatus) checkedInitComponentsMap() {
	if r.Components == nil {
		r.Components = map[string]ClusterComponentStatus{}
	}
}

// ToVolumeClaimTemplates convert r.VolumeClaimTemplates to []corev1.PersistentVolumeClaimTemplate.
func (r *ClusterComponentSpec) ToVolumeClaimTemplates() []corev1.PersistentVolumeClaimTemplate {
	if r == nil {
		return nil
	}
	var ts []corev1.PersistentVolumeClaimTemplate
	for _, t := range r.VolumeClaimTemplates {
		ts = append(ts, t.toVolumeClaimTemplate())
	}
	return ts
}

func (r *ClusterComponentSpec) GetDisableExporter() *bool {
	if r.DisableExporter != nil {
		return r.DisableExporter
	}

	toPointer := func(b bool) *bool {
		p := b
		return &p
	}

	// Compatible with previous versions of kb
	if r.Monitor != nil {
		return toPointer(!*r.Monitor)
	}
	return nil
}

func (t *InstanceTemplate) GetName() string {
	return t.Name
}

func (t *InstanceTemplate) GetReplicas() int32 {
	if t.Replicas != nil {
		return *t.Replicas
	}
	return defaultInstanceTemplateReplicas
}

// GetClusterUpRunningPhases returns Cluster running or partially running phases.
func GetClusterUpRunningPhases() []ClusterPhase {
	return []ClusterPhase{
		RunningClusterPhase,
		AbnormalClusterPhase,
		FailedClusterPhase, // REVIEW/TODO: single component with single pod component are handled as FailedClusterPhase, ought to remove this.
	}
}

// GetReconfiguringRunningPhases return Cluster running or partially running phases.
func GetReconfiguringRunningPhases() []ClusterPhase {
	return []ClusterPhase{
		RunningClusterPhase,
		UpdatingClusterPhase, // enable partial running for reconfiguring
		AbnormalClusterPhase,
		FailedClusterPhase,
	}
}

// GetComponentTerminalPhases return Cluster's component terminal phases.
func GetComponentTerminalPhases() []ClusterComponentPhase {
	return []ClusterComponentPhase{
		RunningClusterCompPhase,
		StoppedClusterCompPhase,
		FailedClusterCompPhase,
		AbnormalClusterCompPhase,
	}
}

// GetComponentUpRunningPhase returns component running or partially running phases.
func GetComponentUpRunningPhase() []ClusterComponentPhase {
	return []ClusterComponentPhase{
		RunningClusterCompPhase,
		AbnormalClusterCompPhase,
		FailedClusterCompPhase,
	}
}

// ComponentPodsAreReady checks if the pods of component are ready.
func ComponentPodsAreReady(podsAreReady *bool) bool {
	return podsAreReady != nil && *podsAreReady
}

// GetInstanceTemplateName get the instance template name by instance name.
func GetInstanceTemplateName(clusterName, componentName, instanceName string) string {
	workloadPrefix := fmt.Sprintf("%s-%s", clusterName, componentName)
	compInsKey := instanceName[:strings.LastIndex(instanceName, "-")]
	if compInsKey == workloadPrefix {
		return ""
	}
	return strings.Replace(compInsKey, workloadPrefix+"-", "", 1)
}
