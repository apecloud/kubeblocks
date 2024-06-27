/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
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

// ClusterSpec defines the desired state of Cluster
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

	// Specifies runtimeClassName for all Pods managed by this Cluster.
	//
	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`

	// Specifies the scheduling policy for the Cluster.
	//
	// +optional
	SchedulingPolicy *SchedulingPolicy `json:"schedulingPolicy,omitempty"`

	// Defines a list of additional Services that are exposed by a Cluster.
	// This field allows Services of selected Components, either from `componentSpecs` or `shardingSpecs` to be exposed,
	// alongside Services defined with ComponentService.
	//
	// Services defined here can be referenced by other clusters using the ServiceRefClusterSelector.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Services []ClusterService `json:"services,omitempty"`

	// Specifies the backup configuration of the Cluster.
	//
	// +optional
	Backup *ClusterBackup `json:"backup,omitempty"`
}

// ClusterStatus defines the observed state of Cluster
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

	// Represents a list of detailed status of the Cluster object.
	// Each condition in the list provides real-time information about certain aspect of the Cluster object.
	//
	// This field is crucial for administrators and developers to monitor and respond to changes within the Cluster.
	// It provides a history of state transitions and a snapshot of the current state that can be used for
	// automated logic or direct inspection.
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Represents the generation number of the referenced ClusterDefinition.
	//
	// +optional
	ClusterDefGeneration int64 `json:"clusterDefGeneration,omitempty"`

	// Records the current status information of all Components within the Cluster.
	//
	// +optional
	// Components map[string]ClusterComponentStatus `json:"components,omitempty"` // TODO(v1.0): remove it
}

// TerminationPolicyType defines termination policy types.
//
// +enum
// +kubebuilder:validation:Enum={DoNotTerminate,Halt,Delete,WipeOut}
type TerminationPolicyType string

const (
	// DoNotTerminate will block delete operation.
	DoNotTerminate TerminationPolicyType = "DoNotTerminate"

	// Halt will delete workload resources such as statefulset, deployment workloads but keep PVCs.
	Halt TerminationPolicyType = "Halt"

	// Delete is based on Halt and deletes PVCs.
	Delete TerminationPolicyType = "Delete"

	// WipeOut is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.
	WipeOut TerminationPolicyType = "WipeOut"
)

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

	// Specifies the scheduling policy for the Component.
	//
	// +optional
	SchedulingPolicy *SchedulingPolicy `json:"schedulingPolicy,omitempty"`

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
	VolumeClaimTemplates []ClusterComponentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty"` // TODO(v1.0)

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

	// Specifies the configuration content of a config template.
	//
	// +optional
	Configs []ClusterComponentConfig `json:"configs,omitempty"` // TODO(v1.0)

	// Specifies the TLS configuration for the Component, including:
	//
	// - A boolean flag that indicates whether the Component should use Transport Layer Security (TLS) for secure communication.
	// - An optional field that specifies the configuration for the TLS certificates issuer when TLS is enabled.
	//   It allows defining the issuer name and the reference to the secret containing the TLS certificates and key.
	//	 The secret should contain the CA certificate, TLS certificate, and private key in the specified keys.
	//
	// +optional
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`

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

	// Specifies the password for the account.
	// The password can be generated based on certain rules or retrieved from a Secret.
	//
	// This field is immutable once set.
	//
	// +kubebuilder:validation:Required
	Password SystemAccountPassword `json:"password,omitempty"`
}

// ClusterComponentConfig represents a config with its source bound.
type ClusterComponentConfig struct {
	// The name of the config.
	//
	// +optional
	Name *string `json:"name,omitempty"`

	// The source of the config.
	ClusterComponentConfigSource `json:",inline"`
}

// ClusterComponentConfigSource represents the source of a config.
type ClusterComponentConfigSource struct {
	// ConfigMap source for the config.
	//
	// +optional
	ConfigMap *corev1.ConfigMapVolumeSource `json:"configMap,omitempty"`

	// TODO: support more diverse sources:
	// - Config template of other components within the same cluster
	// - Config template of components from other clusters
	// - Secret
	// - Local file
}

// ClusterService defines a service that is exposed externally, allowing entities outside the cluster to access it.
// For example, external applications, or other Clusters.
// And another Cluster managed by the same KubeBlocks operator can resolve the address exposed by a ClusterService
// using the `serviceRef` field.
//
// When a Component needs to access another Cluster's ClusterService using the `serviceRef` field,
// it must also define the service type and version information in the `componentDefinition.spec.serviceRefDeclarations`
// section.
type ClusterService struct {
	Service `json:",inline"`

	// Extends the ServiceSpec.Selector by allowing the specification of a sharding name, which is defined in
	// `cluster.spec.shardingSpecs[*].name`, to be used as a selector for the service.
	// Note that this and the `componentSelector` are mutually exclusive and cannot be set simultaneously.
	//
	// +optional
	ShardingSelector string `json:"shardingSelector,omitempty"`

	// Extends the ServiceSpec.Selector by allowing the specification of a component, to be used as a selector for the service.
	// Note that this and the `shardingSelector` are mutually exclusive and cannot be set simultaneously.
	//
	// +optional
	ComponentSelector string `json:"componentSelector,omitempty"`
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
	RetentionPeriod string `json:"retentionPeriod,omitempty"` // TODO(v1.0): retentionPeriod

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

// ClusterPhase defines the phase of the Cluster within the .status.phase field.
//
// +enum
// +kubebuilder:validation:Enum={Creating,Running,Updating,Stopping,Stopped,Deleting,Failed,Abnormal}
type ClusterPhase string

const (
	// CreatingClusterPhase represents all components are in `Creating` phase.
	CreatingClusterPhase ClusterPhase = "Creating"

	// RunningClusterPhase represents all components are in `Running` phase, indicates that the cluster is functioning properly.
	RunningClusterPhase ClusterPhase = "Running"

	// UpdatingClusterPhase represents all components are in `Creating`, `Running` or `Updating` phase, and at least one
	// component is in `Creating` or `Updating` phase, indicates that the cluster is undergoing an update.
	UpdatingClusterPhase ClusterPhase = "Updating"

	// StoppingClusterPhase represents at least one component is in `Stopping` phase, indicates that the cluster is in
	// the process of stopping.
	StoppingClusterPhase ClusterPhase = "Stopping"

	// StoppedClusterPhase represents all components are in `Stopped` phase, indicates that the cluster has stopped and
	// is not providing any functionality.
	StoppedClusterPhase ClusterPhase = "Stopped"

	// DeletingClusterPhase indicates the cluster is being deleted.
	DeletingClusterPhase ClusterPhase = "Deleting"

	// FailedClusterPhase represents all components are in `Failed` phase, indicates that the cluster is unavailable.
	FailedClusterPhase ClusterPhase = "Failed"

	// AbnormalClusterPhase represents some components are in `Failed` or `Abnormal` phase, indicates that the cluster
	// is in a fragile state and troubleshooting is required.
	AbnormalClusterPhase ClusterPhase = "Abnormal"
)
