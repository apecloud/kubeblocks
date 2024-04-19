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
	"k8s.io/apimachinery/pkg/util/intstr"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// ClusterSpec defines the desired state of Cluster.
type ClusterSpec struct {
	// Refers to the ClusterDefinition name.
	// If not specified, ComponentDef must be specified for each Component in ComponentSpecs.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="clusterDefinitionRef is immutable"
	// +optional
	ClusterDefRef string `json:"clusterDefinitionRef,omitempty"`

	// Refers to the ClusterVersion name.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	ClusterVersionRef string `json:"clusterVersionRef,omitempty"`

	// Topology specifies the topology to use for the cluster. If not specified, the default topology will be used.
	// Cannot be updated.
	//
	// +kubebuilder:validation:MaxLength=32
	// +optional
	Topology string `json:"topology,omitempty"`

	// Specifies the cluster termination policy.
	//
	// - DoNotTerminate will block delete operation.
	// - Halt will delete workload resources such as statefulset, deployment workloads but keep PVCs.
	// - Delete is based on Halt and deletes PVCs.
	// - WipeOut is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.
	//
	// +kubebuilder:validation:Required
	TerminationPolicy TerminationPolicyType `json:"terminationPolicy"`

	// List of ShardingSpec used to define components with a sharding topology structure that make up a cluster.
	// ShardingSpecs and ComponentSpecs cannot both be empty at the same time.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	// +optional
	ShardingSpecs []ShardingSpec `json:"shardingSpecs,omitempty"`

	// List of componentSpec used to define the components that make up a cluster.
	// ComponentSpecs and ShardingSpecs cannot both be empty at the same time.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	// +kubebuilder:validation:XValidation:rule="self.all(x, size(self.filter(c, c.name == x.name)) == 1)",message="duplicated component"
	// +kubebuilder:validation:XValidation:rule="self.all(x, size(self.filter(c, has(c.componentDef))) == 0) || self.all(x, size(self.filter(c, has(c.componentDef))) == size(self))",message="two kinds of definition API can not be used simultaneously"
	// +optional
	ComponentSpecs []ClusterComponentSpec `json:"componentSpecs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Defines the services to access a cluster.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Services []ClusterService `json:"services,omitempty"`

	// A group of affinity scheduling rules.
	//
	// +optional
	Affinity *Affinity `json:"affinity,omitempty"`

	// Attached to tolerate any taint that matches the triple `key,value,effect` using the matching operator `operator`.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Cluster backup configuration.
	//
	// +optional
	Backup *ClusterBackup `json:"backup,omitempty"`

	// !!!!! The following fields may be deprecated in subsequent versions, please DO NOT rely on them for new requirements.

	// Describes how pods are distributed across node.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	Tenancy TenancyType `json:"tenancy,omitempty"`

	// Describes the availability policy, including zone, node, and none.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	AvailabilityPolicy AvailabilityPolicyType `json:"availabilityPolicy,omitempty"`

	// Specifies the replicas of the first componentSpec, if the replicas of the first componentSpec is specified,
	// this value will be ignored.
	//
	//+kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Specifies the resources of the first componentSpec, if the resources of the first componentSpec is specified,
	// this value will be ignored.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	Resources ClusterResources `json:"resources,omitempty"`

	// Specifies the storage of the first componentSpec, if the storage of the first componentSpec is specified,
	// this value will be ignored.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	Storage ClusterStorage `json:"storage,omitempty"`

	// The configuration of monitor.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	Monitor ClusterMonitor `json:"monitor,omitempty"`

	// The configuration of network.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	Network *ClusterNetwork `json:"network,omitempty"`

	// Defines RuntimeClassName for all Pods managed by this cluster.
	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`
}

type ClusterBackup struct {
	// Specifies whether automated backup is enabled.
	//
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Determines the duration for which the backup should be retained. All backups older than this period will be
	// removed by the controller.
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
	// You can also combine the above durations. For example: 30d12h30m
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

	// Defines the deadline in minutes for starting the backup job if it misses its scheduled time for any reason.
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

type ClusterResources struct {
	// Specifies the amount of processing power the cluster needs.
	// For more information, refer to: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	//
	// +optional
	CPU resource.Quantity `json:"cpu,omitempty"`

	// Specifies the amount of memory the cluster needs.
	// For more information, refer to: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	//
	// +optional
	Memory resource.Quantity `json:"memory,omitempty"`
}

type ClusterStorage struct {
	// Specifies the amount of storage the cluster needs.
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

	// Secret specifies the secret to be mounted as a volume.
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

// UserResourceRefs defines references to user-defined secrets and config maps.
type UserResourceRefs struct {
	// SecretRefs defines the user-defined secrets.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	SecretRefs []SecretRef `json:"secretRefs,omitempty"`

	// ConfigMapRefs defines the user-defined config maps.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ConfigMapRefs []ConfigMapRef `json:"configMapRefs,omitempty"`
}

// InstanceTemplate defines values to override in pod template.
type InstanceTemplate struct {
	// Specifies the name of the template.
	// Each instance of the template derives its name from the Component's Name, the template's Name and the instance's ordinal.
	// The constructed instance name follows the pattern $(component.name)-$(template.name)-$(ordinal).
	// The ordinal starts from 0 by default.
	//
	// +kubebuilder:validation:MaxLength=54
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Number of replicas of this template.
	// Default is 1.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Defines annotations to override.
	// Add new or override existing annotations.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Defines labels to override.
	// Add new or override existing labels.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Defines image to override.
	// Will override the first container's image of the pod.
	// +optional
	Image *string `json:"image,omitempty"`

	// Defines NodeName to override.
	// +optional
	NodeName *string `json:"nodeName,omitempty"`

	// Defines NodeSelector to override.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Defines Tolerations to override.
	// Add new or override existing tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Defines Resources to override.
	// Will override the first container's resources of the pod.
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
	// Add new or override existing volume mounts of the first container in the pod.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Defines VolumeClaimTemplates to override.
	// Add new or override existing volume claim templates.
	// +optional
	VolumeClaimTemplates []corev1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
}

// ClusterStatus defines the observed state of Cluster.
type ClusterStatus struct {
	// The most recent generation number that has been observed by the controller.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// The current phase of the Cluster.
	//
	// +optional
	Phase ClusterPhase `json:"phase,omitempty"`

	// Provides additional information about the current phase.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// Records the current status information of all components within the cluster.
	//
	// +optional
	Components map[string]ClusterComponentStatus `json:"components,omitempty"`

	// Represents the generation number of the referenced ClusterDefinition.
	//
	// +optional
	ClusterDefGeneration int64 `json:"clusterDefGeneration,omitempty"`

	// Describes the current state of the cluster API Resource, such as warnings.
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ShardingSpec defines the sharding spec.
type ShardingSpec struct {
	// Specifies the identifier for the sharding configuration. This identifier is included as part of the Service DNS
	// name and must comply with IANA Service Naming rules.
	// It is used to generate the names of underlying components following the pattern `$(ShardingSpec.Name)-$(ShardID)`.
	// Note that the name of the component template defined in ShardingSpec.Template.Name will be disregarded.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=15
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`

	// The blueprint for the components.
	// Generates a set of components (also referred to as shards) based on this template. All components or shards
	// generated will have identical specifications and definitions.
	//
	// +kubebuilder:validation:Required
	Template ClusterComponentSpec `json:"template"`

	// Specifies the number of components, all of which will have identical specifications and definitions.
	//
	// The number of replicas for each component should be defined by template.replicas.
	// The logical relationship between these components should be maintained by the components themselves.
	// KubeBlocks only provides lifecycle management for sharding, including:
	//
	// 1. Executing the postProvision Action defined in the ComponentDefinition when the number of shards increases,
	//    provided the conditions are met.
	// 2. Executing the preTerminate Action defined in the ComponentDefinition when the number of shards decreases,
	//    provided the conditions are met.
	//    Resources and data associated with the corresponding Component will also be deleted.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=2048
	Shards int32 `json:"shards,omitempty"`
}

// ClusterComponentSpec defines the specifications for a cluster component.
// TODO +kubebuilder:validation:XValidation:rule="!has(oldSelf.componentDefRef) || has(self.componentDefRef)", message="componentDefRef is required once set"
// TODO +kubebuilder:validation:XValidation:rule="!has(oldSelf.componentDef) || has(self.componentDef)", message="componentDef is required once set"
type ClusterComponentSpec struct {
	// Specifies the name of the cluster's component.
	// This name is also part of the Service DNS name and must comply with the IANA Service Naming rule.
	// When ClusterComponentSpec is referenced as a template, the name is optional. Otherwise, it is required.
	//
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	// TODO +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	// +optional
	Name string `json:"name"`

	// References the componentDef defined in the ClusterDefinition spec. Must comply with the IANA Service Naming rule.
	//
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	// TODO +kubebuilder:validation:XValidation:rule="self == oldSelf",message="componentDefRef is immutable"
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0, consider using the ComponentDef instead"
	// +optional
	ComponentDefRef string `json:"componentDefRef,omitempty"`

	// References the name of the ComponentDefinition.
	// If both componentDefRef and componentDef are provided, the componentDef will take precedence over componentDefRef.
	//
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	ComponentDef string `json:"componentDef,omitempty"`

	// ServiceVersion specifies the version of the service provisioned by the component.
	// The version should follow the syntax and semantics of the "Semantic Versioning" specification (http://semver.org/).
	// If not explicitly specified, the version defined in the referenced topology will be used.
	// If no version is specified in the topology, the latest available version will be used.
	//
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceVersion string `json:"serviceVersion,omitempty"`

	// References the class defined in ComponentClassDefinition.
	//
	// +kubebuilder:deprecatedversion:warning="Due to the lack of practical use cases, this field is deprecated from KB 0.9.0."
	// +optional
	ClassDefRef *ClassDefRef `json:"classDefRef,omitempty"`

	// Defines service references for the current component.
	//
	// Based on the referenced services, they can be categorized into two types:
	//
	// - Service provided by external sources: These services are provided by external sources and are not managed by KubeBlocks.
	//   They can be Kubernetes-based or non-Kubernetes services. For external services, an additional
	//   ServiceDescriptor object is needed to establish the service binding.
	// - Service provided by other KubeBlocks clusters: These services are provided by other KubeBlocks clusters.
	//   Binding to these services is done by specifying the name of the hosting cluster.
	//
	// Each type of service reference requires specific configurations and bindings to establish the connection and
	// interaction with the respective services.
	// Note that the ServiceRef has cluster-level semantic consistency, meaning that within the same Cluster, service
	// references with the same ServiceRef.Name are considered to be the same service. It is only allowed to bind to
	// the same Cluster or ServiceDescriptor.
	//
	// +optional
	ServiceRefs []ServiceRef `json:"serviceRefs,omitempty"`

	// To enable monitoring.
	//
	// +kubebuilder:default=false
	// +optional
	Monitor bool `json:"monitor,omitempty"`

	// Indicates which log file takes effect in the database cluster.
	//
	// +listType=set
	// +optional
	EnabledLogs []string `json:"enabledLogs,omitempty"`

	// Specifies the number of component replicas.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`

	// A group of affinity scheduling rules.
	//
	// +optional
	Affinity *Affinity `json:"affinity,omitempty"`

	// Attached to tolerate any taint that matches the triple `key,value,effect` using the matching operator `operator`.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Specifies the resources requests and limits of the workload.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Provides information for statefulset.spec.volumeClaimTemplates.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +optional
	VolumeClaimTemplates []ClusterComponentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Services overrides services defined in referenced ComponentDefinition.
	//
	// +optional
	Services []ClusterComponentService `json:"services,omitempty"`

	// Defines the strategy for switchover and failover when workloadType is Replication.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	SwitchPolicy *ClusterSwitchPolicy `json:"switchPolicy,omitempty"`

	// Enables or disables TLS certs.
	//
	// +optional
	TLS bool `json:"tls,omitempty"`

	// Defines provider context for TLS certs. Required when TLS is enabled.
	//
	// +optional
	Issuer *Issuer `json:"issuer,omitempty"`

	// Specifies the name of the ServiceAccount that the running component depends on.
	//
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Defines the update strategy for the component.
	// Not supported.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	UpdateStrategy *UpdateStrategy `json:"updateStrategy,omitempty"`

	// Defines the user-defined volumes.
	//
	// +optional
	UserResourceRefs *UserResourceRefs `json:"userResourceRefs,omitempty"`

	// Overrides values in default Template.
	//
	// Instance is the fundamental unit managed by KubeBlocks.
	// It represents a Pod with additional objects such as PVCs, Services, ConfigMaps, etc.
	// A component manages instances with a total count of Replicas,
	// and by default, all these instances are generated from the same template.
	// The InstanceTemplate provides a way to override values in the default template,
	// allowing the component to manage instances from different templates.
	//
	// The naming convention for instances (pods) based on the Component Name, InstanceTemplate Name, and ordinal.
	// The constructed instance name follows the pattern: $(component.name)-$(template.name)-$(ordinal).
	// By default, the ordinal starts from 0 for each InstanceTemplate.
	// It is important to ensure that the Name of each InstanceTemplate is unique.
	//
	// The sum of replicas across all InstanceTemplates should not exceed the total number of Replicas specified for the Component.
	// Any remaining replicas will be generated using the default template and will follow the default naming rules.
	//
	// +optional
	Instances []InstanceTemplate `json:"instances,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Specifies instances to be scaled in with dedicated names in the list.
	//
	// +optional
	OfflineInstances []string `json:"offlineInstances,omitempty"`
}

type ComponentMessageMap map[string]string

// ClusterComponentStatus records components status.
type ClusterComponentStatus struct {
	// Specifies the current state of the component.
	Phase ClusterComponentPhase `json:"phase,omitempty"`

	// Records detailed information about the component in its current phase.
	// The keys are either podName, deployName, or statefulSetName, formatted as 'ObjectKind/Name'.
	//
	// +optional
	Message ComponentMessageMap `json:"message,omitempty"`

	// Checks if all pods of the component are ready.
	//
	// +optional
	PodsReady *bool `json:"podsReady,omitempty"`

	// Indicates the time when all component pods became ready.
	// This is the readiness time of the last component pod.
	//
	// +optional
	PodsReadyTime *metav1.Time `json:"podsReadyTime,omitempty"`

	// Represents the status of the members.
	//
	// +optional
	MembersStatus []workloads.MemberStatus `json:"membersStatus,omitempty"`
}

// ClusterSwitchPolicy defines the switch policy for a cluster.
type ClusterSwitchPolicy struct {
	// Type specifies the type of switch policy to be applied.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Noop
	// +optional
	Type SwitchPolicyType `json:"type"`
}

type ClusterComponentVolumeClaimTemplate struct {
	// Refers to `clusterDefinition.spec.componentDefs.containers.volumeMounts.name`.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Defines the desired characteristics of a volume requested by a pod author.
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

	// Defines what type of volume is required by the claim.
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
	// Specifies the anti-affinity level of pods within a component.
	//
	// +kubebuilder:default=Preferred
	// +optional
	PodAntiAffinity PodAntiAffinity `json:"podAntiAffinity,omitempty"`

	// Represents the key of node labels.
	//
	// Nodes with a label containing this key and identical values are considered to be in the same topology.
	// This is used as the topology domain for pod anti-affinity and pod spread constraint.
	// Some well-known label keys, such as `kubernetes.io/hostname` and `topology.kubernetes.io/zone`, are often used
	// as TopologyKey, along with any other custom label key.
	//
	// +listType=set
	// +optional
	TopologyKeys []string `json:"topologyKeys,omitempty"`

	// Indicates that pods must be scheduled to the nodes with the specified node labels.
	//
	// +optional
	NodeLabels map[string]string `json:"nodeLabels,omitempty"`

	// Defines how pods are distributed across nodes.
	//
	// +kubebuilder:default=SharedNode
	// +optional
	Tenancy TenancyType `json:"tenancy,omitempty"`
}

type TLSConfig struct {
	// +kubebuilder:default=false
	// +optional
	Enable bool `json:"enable,omitempty"`

	// +optional
	Issuer *Issuer `json:"issuer,omitempty"`
}

// Issuer defines the TLS certificates issuer for the cluster.
type Issuer struct {
	// The issuer for TLS certificates.
	//
	// +kubebuilder:validation:Enum={KubeBlocks, UserProvided}
	// +kubebuilder:default=KubeBlocks
	// +kubebuilder:validation:Required
	Name IssuerName `json:"name"`

	// SecretRef is the reference to the TLS certificates secret.
	// It is required when the issuer is set to UserProvided.
	//
	// +optional
	SecretRef *TLSSecretRef `json:"secretRef,omitempty"`
}

// TLSSecretRef defines Secret contains Tls certs
type TLSSecretRef struct {
	// Name of the Secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// CA cert key in Secret
	// +kubebuilder:validation:Required
	CA string `json:"ca"`

	// Cert key in Secret
	// +kubebuilder:validation:Required
	Cert string `json:"cert"`

	// Key of TLS private key in Secret
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

type ClusterComponentService struct {
	// References the component service name defined in the ComponentDefinition.Spec.Services[x].Name.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=25
	Name string `json:"name"`

	// Determines how the Service is exposed. Valid options are ClusterIP, NodePort, and LoadBalancer.
	//
	// - `ClusterIP` allocates a cluster-internal IP address for load-balancing to endpoints. Endpoints are determined
	//    by the selector or if that is not specified, they are determined by manual construction of an Endpoints object
	//    or EndpointSlice objects. If clusterIP is "None", no virtual IP is allocated and the endpoints are published
	//    as a set of endpoints rather than a virtual IP.
	// - `NodePort` builds on ClusterIP and allocates a port on every node which routes to the same endpoints as the clusterIP.
	// - `LoadBalancer` builds on NodePort and creates an external load-balancer (if supported in the current cloud)
	//    which routes to the same endpoints as the clusterIP.
	//
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types.
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

	// Indicates whether to generate individual services for each pod.
	// If set to true, a separate service will be created for each pod in the cluster.
	//
	// +optional
	PodService *bool `json:"podService,omitempty"`
}

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

type ClusterMonitor struct {
	// Defines the frequency at which monitoring occurs. If set to 0, monitoring is disabled.
	//
	// +kubebuilder:validation:XIntOrString
	// +optional
	MonitoringInterval *intstr.IntOrString `json:"monitoringInterval,omitempty"`
}

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
	// It corresponds to the serviceRefDeclaration name defined in the clusterDefinition.componentDefs[*].serviceRefDeclarations[*].name.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the namespace of the referenced Cluster or ServiceDescriptor object.
	// If not specified, the namespace of the current cluster will be used.
	//
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// The name of the KubeBlocks cluster being referenced when a service provided by another KubeBlocks cluster is
	// being referenced.
	//
	// By default, the clusterDefinition.spec.connectionCredential secret corresponding to the referenced Cluster will
	// be used to bind to the current component. The connection credential secret should include and correspond to the
	// following fields: endpoint, port, username, and password when a KubeBlocks cluster is being referenced.
	//
	// Under this referencing approach, the ServiceKind and ServiceVersion of service reference declaration defined in
	// the ClusterDefinition will not be validated.
	// If both Cluster and ServiceDescriptor are specified, the Cluster takes precedence.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0"
	// +optional
	Cluster string `json:"cluster,omitempty"`

	// Specifies the cluster to reference.
	//
	// +optional
	ClusterServiceSelector *ServiceRefClusterSelector `json:"clusterServiceSelector,omitempty"`

	// The service descriptor of the service provided by external sources.
	//
	// When referencing a service provided by external sources, a ServiceDescriptor object is required to establish
	// the service binding.
	// The `serviceDescriptor.spec.serviceKind` and `serviceDescriptor.spec.serviceVersion` should match the serviceKind
	// and serviceVersion declared in the definition.
	//
	// If both Cluster and ServiceDescriptor are specified, the Cluster takes precedence.
	//
	// +optional
	ServiceDescriptor string `json:"serviceDescriptor,omitempty"`
}

type ServiceRefClusterSelector struct {
	// The name of the cluster to reference.
	//
	// +kubebuilder:validation:Required
	Cluster string `json:"cluster"`

	// The service to reference from the cluster.
	//
	// +optional
	Service *ServiceRefServiceSelector `json:"service,omitempty"`

	// The credential (SystemAccount) to reference from the cluster.
	//
	// +optional
	Credential *ServiceRefCredentialSelector `json:"credential,omitempty"`
}

type ServiceRefServiceSelector struct {
	// The name of the component where the service resides in.
	//
	// It is required when referencing a component service.
	//
	// +optional
	Component string `json:"component,omitempty"`

	// The name of the service to reference.
	//
	// Leave it empty to reference the default service. Set it to "headless" to reference the default headless service.
	// If the referenced service is a pod-service, there will be multiple service objects matched,
	// and the resolved value will be presented in the following format: service1.name,service2.name...
	//
	// +kubebuilder:validation:Required
	Service string `json:"service"`

	// The port name of the service to reference.
	//
	// If there is a non-zero node-port exist for the matched service port, the node-port will be selected first.
	// If the referenced service is a pod-service, there will be multiple service objects matched,
	// and the resolved value will be presented in the following format: service1.name:port1,service2.name:port2...
	//
	// +optional
	Port string `json:"port,omitempty"`
}

type ServiceRefCredentialSelector struct {
	// The name of the component where the credential resides in.
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

// Cluster is the Schema for the clusters API.
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
