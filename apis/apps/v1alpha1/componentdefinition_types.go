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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=cmpd
// +kubebuilder:printcolumn:name="SERVICE",type="string",JSONPath=".spec.serviceKind",description="service"
// +kubebuilder:printcolumn:name="SERVICE-VERSION",type="string",JSONPath=".spec.serviceVersion",description="service version"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ComponentDefinition serves as a reusable blueprint for creating Components,
// encapsulating essential static settings such as Component description,
// Pod templates, configuration file templates, scripts, parameter lists,
// injected environment variables and their sources, and event handlers.
// ComponentDefinition works in conjunction with dynamic settings from the ClusterComponentSpec,
// to instantiate Components during Cluster creation.
//
// Key aspects that can be defined in a ComponentDefinition include:
//
// - PodSpec template: Specifies the PodSpec template used by the Component.
// - Configuration templates: Specify the configuration file templates required by the Component.
// - Scripts: Provide the necessary scripts for Component management and operations.
// - Storage volumes: Specify the storage volumes and their configurations for the Component.
// - Pod roles: Outlines various roles of Pods within the Component along with their capabilities.
// - Exposed Kubernetes Services: Specify the Services that need to be exposed by the Component.
// - System accounts: Define the system accounts required for the Component.
// - Monitoring and logging: Configure the exporter and logging settings for the Component.
//
// ComponentDefinitions also enable defining reactive behaviors of the Component in response to events,
// such as member join/leave, Component addition/deletion, role changes, switch over, and more.
// This allows for automatic event handling, thus encapsulating complex behaviors within the Component.
//
// Referencing a ComponentDefinition when creating individual Components ensures inheritance of predefined configurations,
// promoting reusability and consistency across different deployments and cluster topologies.
type ComponentDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentDefinitionSpec   `json:"spec,omitempty"`
	Status ComponentDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ComponentDefinitionList contains a list of ComponentDefinition
type ComponentDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentDefinition{}, &ComponentDefinitionList{})
}

type ComponentDefinitionSpec struct {
	// Specifies the name of the Component provider, typically the vendor or developer name.
	// It identifies the entity responsible for creating and maintaining the Component.
	//
	// When specifying the provider name, consider the following guidelines:
	//
	// - Keep the name concise and relevant to the Component.
	// - Use a consistent naming convention across Components from the same provider.
	// - Avoid using trademarked or copyrighted names without proper permission.
	//
	// +kubebuilder:validation:MaxLength=32
	// +optional
	Provider string `json:"provider,omitempty"`

	// Provides a brief and concise explanation of the Component's purpose, functionality, and any relevant details.
	// It serves as a quick reference for users to understand the Component's role and characteristics.
	//
	// +kubebuilder:validation:MaxLength=256
	// +optional
	Description string `json:"description,omitempty"`

	// Defines the type of well-known service protocol that the Component provides.
	// It specifies the standard or widely recognized protocol used by the Component to offer its Services.
	//
	// The `serviceKind` field allows users to quickly identify the type of Service provided by the Component
	// based on common protocols or service types. This information helps in understanding the compatibility,
	// interoperability, and usage of the Component within a system.
	//
	// Some examples of well-known service protocols include:
	//
	// - "MySQL": Indicates that the Component provides a MySQL database service.
	// - "PostgreSQL": Indicates that the Component offers a PostgreSQL database service.
	// - "Redis": Signifies that the Component functions as a Redis key-value store.
	// - "ETCD": Denotes that the Component serves as an ETCD distributed key-value store.
	//
	// The `serviceKind` value is case-insensitive, allowing for flexibility in specifying the protocol name.
	//
	// When specifying the `serviceKind`, consider the following guidelines:
	//
	// - Use well-established and widely recognized protocol names or service types.
	// - Ensure that the `serviceKind` accurately represents the primary service type offered by the Component.
	// - If the Component provides multiple services, choose the most prominent or commonly used protocol.
	// - Limit the `serviceKind` to a maximum of 32 characters for conciseness and readability.
	//
	// Note: The `serviceKind` field is optional and can be left empty if the Component does not fit into a well-known
	// service category or if the protocol is not widely recognized. It is primarily used to convey information about
	// the Component's service type to users and facilitate discovery and integration.
	//
	// The `serviceKind` field is immutable and cannot be updated.
	//
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceKind string `json:"serviceKind,omitempty"`

	// Specifies the version of the Service provided by the Component.
	// It follows the syntax and semantics of the "Semantic Versioning" specification (http://semver.org/).
	//
	// The Semantic Versioning specification defines a version number format of X.Y.Z (MAJOR.MINOR.PATCH), where:
	//
	// - X represents the major version and indicates incompatible API changes.
	// - Y represents the minor version and indicates added functionality in a backward-compatible manner.
	// - Z represents the patch version and indicates backward-compatible bug fixes.
	//
	// Additional labels for pre-release and build metadata are available as extensions to the X.Y.Z format:
	//
	// - Use pre-release labels (e.g., -alpha, -beta) for versions that are not yet stable or ready for production use.
	// - Use build metadata (e.g., +build.1) for additional version information if needed.
	//
	// Examples of valid ServiceVersion values:
	//
	// - "1.0.0"
	// - "2.3.1"
	// - "3.0.0-alpha.1"
	// - "4.5.2+build.1"
	//
	// The `serviceVersion` field is immutable and cannot be updated.
	//
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceVersion string `json:"serviceVersion,omitempty"`

	// Specifies the PodSpec template used in the Component.
	// It includes the following elements:
	//
	// - Init containers
	// - Containers
	//     - Image
	//     - Commands
	//     - Args
	//     - Envs
	//     - Mounts
	//     - Ports
	//     - Security context
	//     - Probes
	//     - Lifecycle
	// - Volumes
	//
	// This field is intended to define static settings that remain consistent across all instantiated Components.
	// Dynamic settings such as CPU and memory resource limits, as well as scheduling settings (affinity,
	// toleration, priority), may vary among different instantiated Components.
	// They should be specified in the `cluster.spec.componentSpecs` (ClusterComponentSpec).
	//
	// Specific instances of a Component may override settings defined here, such as using a different container image
	// or modifying environment variable values.
	// These instance-specific overrides can be specified in `cluster.spec.componentSpecs[*].instances`.
	//
	// This field is immutable and cannot be updated once set.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Required
	Runtime corev1.PodSpec `json:"runtime"`

	// Deprecated since v0.9
	// monitor is monitoring config which provided by provider.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.10.0"
	// +optional
	Monitor *MonitorConfig `json:"monitor,omitempty"`

	// Defines the built-in metrics exporter container.
	//
	// +optional
	Exporter *Exporter `json:"exporter,omitempty"`

	// Defines variables which are determined after Cluster instantiation and reflect
	// dynamic or runtime attributes of instantiated Clusters.
	// These variables serve as placeholders for setting environment variables in Pods and Actions,
	// or for rendering configuration and script templates before actual values are finalized.
	//
	// These variables are placed in front of the environment variables declared in the Pod if used as
	// environment variables.
	//
	// Variable values can be sourced from:
	//
	// - ConfigMap: Select and extract a value from a specific key within a ConfigMap.
	// - Secret: Select and extract a value from a specific key within a Secret.
	// - HostNetwork: Retrieves values (including ports) from host-network resources.
	// - Service: Retrieves values (including address, port, NodePort) from a selected Service.
	//   Intended to obtain the address of a ComponentService within the same Cluster.
	// - Credential: Retrieves account name and password from a SystemAccount variable.
	// - ServiceRef: Retrieves address, port, account name and password from a selected ServiceRefDeclaration.
	//   Designed to obtain the address bound to a ServiceRef, such as a ClusterService or
	//   ComponentService of another cluster or an external service.
	// - Component: Retrieves values from a selected Component, including replicas and instance name list.
	//
	// This field is immutable.
	//
	// +optional
	Vars []EnvVar `json:"vars,omitempty"`

	// Defines the volumes used by the Component and some static attributes of the volumes.
	// After defining the volumes here, user can reference them in the
	// `cluster.spec.componentSpecs[*].volumeClaimTemplates` field to configure dynamic properties such as
	// volume capacity and storage class.
	//
	// This field allows you to specify the following:
	//
	// - Snapshot behavior: Determines whether a snapshot of the volume should be taken when performing
	//   a snapshot backup of the Component.
	// - Disk high watermark: Sets the high watermark for the volume's disk usage.
	//   When the disk usage reaches the specified threshold, it triggers an alert or action.
	//
	// By configuring these volume behaviors, you can control how the volumes are managed and monitored within the Component.
	//
	// This field is immutable.
	//
	// +optional
	Volumes []ComponentVolume `json:"volumes"`

	// Specifies the host network configuration for the Component.
	//
	// When `hostNetwork` option is enabled, the Pods share the host's network namespace and can directly access
	// the host's network interfaces.
	// This means that if multiple Pods need to use the same port, they cannot run on the same host simultaneously
	// due to port conflicts.
	//
	// The DNSPolicy field in the Pod spec determines how containers within the Pod perform DNS resolution.
	// When using hostNetwork, the operator will set the DNSPolicy to 'ClusterFirstWithHostNet'.
	// With this policy, DNS queries will first go through the K8s cluster's DNS service.
	// If the query fails, it will fall back to the host's DNS settings.
	//
	// If set, the DNS policy will be automatically set to "ClusterFirstWithHostNet".
	//
	// +optional
	HostNetwork *HostNetwork `json:"hostNetwork,omitempty"`

	// Defines additional Services to expose the Component's endpoints.
	//
	// A default headless Service, named `{cluster.name}-{component.name}-headless`, is automatically created
	// for internal Cluster communication.
	//
	// This field enables customization of additional Services to expose the Component's endpoints to
	// other Components within the same or different Clusters, and to external applications.
	// Each Service entry in this list can include properties such as ports, type, and selectors.
	//
	// - For intra-Cluster access, Components can reference Services using variables declared in
	//   `componentDefinition.spec.vars[*].valueFrom.serviceVarRef`.
	// - For inter-Cluster access, reference Services use variables declared in
	//   `componentDefinition.spec.vars[*].valueFrom.serviceRefVarRef`,
	//   and bind Services at Cluster creation time with `clusterComponentSpec.ServiceRef[*].clusterServiceSelector`.
	//
	// This field is immutable.
	//
	// +optional
	Services []ComponentService `json:"services,omitempty"`

	// Specifies the configuration file templates and volume mount parameters used by the Component.
	// It also includes descriptions of the parameters in the ConfigMaps, such as value range limitations.
	//
	// This field specifies a list of templates that will be rendered into Component containers' configuration files.
	// Each template is represented as a ConfigMap and may contain multiple configuration files,
	// with each file being a key in the ConfigMap.
	//
	// The rendered configuration files will be mounted into the Component's containers
	//  according to the specified volume mount parameters.
	//
	// This field is immutable.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	// TODO: support referencing configs from other components or clusters.
	Configs []ComponentConfigSpec `json:"configs,omitempty"`

	// Defines the types of logs generated by instances of the Component and their corresponding file paths.
	// These logs can be collected for further analysis and monitoring.
	//
	// The `logConfigs` field is an optional list of LogConfig objects, where each object represents
	// a specific log type and its configuration.
	// It allows you to specify multiple log types and their respective file paths for the Component.
	//
	// Examples:
	//
	// ```yaml
	//  logConfigs:
	//  - filePathPattern: /data/mysql/log/mysqld-error.log
	//    name: error
	//  - filePathPattern: /data/mysql/log/mysqld.log
	//    name: general
	//  - filePathPattern: /data/mysql/log/mysqld-slowquery.log
	//    name: slow
	// ```
	//
	// This field is immutable.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	LogConfigs []LogConfig `json:"logConfigs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Specifies groups of scripts, each provided via a ConfigMap, to be mounted as volumes in the container.
	// These scripts can be executed during container startup or via specific actions.
	//
	// Each script group is encapsulated in a ComponentTemplateSpec that includes:
	//
	// - The ConfigMap containing the scripts.
	// - The mount point where the scripts will be mounted inside the container.
	//
	// This field is immutable.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	Scripts []ComponentTemplateSpec `json:"scripts,omitempty"`

	// Defines the namespaced policy rules required by the Component.
	//
	// The `policyRules` field is an array of `rbacv1.PolicyRule` objects that define the policy rules
	// needed by the Component to operate within a namespace.
	// These policy rules determine the permissions and verbs the Component is allowed to perform on
	// Kubernetes resources within the namespace.
	//
	// The purpose of this field is to automatically generate the necessary RBAC roles
	// for the Component based on the specified policy rules.
	// This ensures that the Pods in the Component has appropriate permissions to function.
	//
	// Note: This field is currently non-functional and is reserved for future implementation.
	//
	// This field is immutable.
	//
	// +optional
	PolicyRules []rbacv1.PolicyRule `json:"policyRules,omitempty"`

	// Specifies static labels that will be patched to all Kubernetes resources created for the Component.
	//
	// Note: If a label key in the `labels` field conflicts with any system labels or user-specified labels,
	// it will be silently ignored to avoid overriding higher-priority labels.
	//
	// This field is immutable.
	//
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Specifies static annotations that will be patched to all Kubernetes resources created for the Component.
	//
	// Note: If an annotation key in the `annotations` field conflicts with any system annotations
	// or user-specified annotations, it will be silently ignored to avoid overriding higher-priority annotations.
	//
	// This field is immutable.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Defines the upper limit of the number of replicas supported by the Component.
	//
	// It defines the maximum number of replicas that can be created for the Component.
	// This field allows you to set a limit on the scalability of the Component, preventing it from exceeding a certain number of replicas.
	//
	// This field is immutable.
	//
	// +optional
	ReplicasLimit *ReplicasLimit `json:"replicasLimit,omitempty"`

	// An array of `SystemAccount` objects that define the system accounts needed
	// for the management operations of the Component.
	//
	// Each `SystemAccount` includes:
	//
	// - Account name.
	// - The SQL statement template: Used to create the system account.
	// - Password Source: Either generated based on certain rules or retrieved from a Secret.
	//
	//  Use cases for system accounts typically involve tasks like system initialization, backups, monitoring,
	//  health checks, replication, and other system-level operations.
	//
	// System accounts are distinct from user accounts, although both are database accounts.
	//
	// - **System Accounts**: Created during Cluster setup by the KubeBlocks operator,
	//   these accounts have higher privileges for system management and are fully managed
	//   through a declarative API by the operator.
	// - **User Accounts**: Managed by users or administrator.
	//   User account permissions should follow the principle of least privilege,
	//   granting only the necessary access rights to complete their required tasks.
	//
	// This field is immutable.
	//
	// +optional
	SystemAccounts []SystemAccount `json:"systemAccounts,omitempty"`

	// Specifies the concurrency strategy for updating multiple instances of the Component.
	// Available strategies:
	//
	// - `Serial`: Updates replicas one at a time, ensuring minimal downtime by waiting for each replica to become ready
	//   before updating the next.
	// - `Parallel`: Updates all replicas simultaneously, optimizing for speed but potentially reducing availability
	//   during the update.
	// - `BestEffortParallel`: Updates replicas concurrently with a limit on simultaneous updates to ensure a minimum
	//   number of operational replicas for maintaining quorum.
	//	 For example, in a 5-replica component, updating a maximum of 2 replicas simultaneously keeps
	//	 at least 3 operational for quorum.
	//
	// This field is immutable and defaults to 'Serial'.
	//
	// +kubebuilder:default=Serial
	// +optional
	UpdateStrategy *UpdateStrategy `json:"updateStrategy,omitempty"`

	// InstanceSet controls the creation of pods during initial scale up, replacement of pods on nodes, and scaling down.
	//
	// - `OrderedReady`: Creates pods in increasing order (pod-0, then pod-1, etc). The controller waits until each pod
	// is ready before continuing. Pods are removed in reverse order when scaling down.
	// - `Parallel`: Creates pods in parallel to match the desired scale without waiting. All pods are deleted at once
	// when scaling down.
	//
	// +optional
	PodManagementPolicy *appsv1.PodManagementPolicyType `json:"podManagementPolicy,omitempty"`

	// Enumerate all possible roles assigned to each replica of the Component, influencing its behavior.
	//
	// A replica can have zero to multiple roles.
	// KubeBlocks operator determines the roles of each replica by invoking the `lifecycleActions.roleProbe` method.
	// This action returns a list of roles for each replica, and the returned roles must be predefined in the `roles` field.
	//
	// The roles assigned to a replica can influence various aspects of the Component's behavior, such as:
	//
	// - Service selection: The Component's exposed Services may target replicas based on their roles using `roleSelector`.
	// - Update order: The roles can determine the order in which replicas are updated during a Component update.
	//   For instance, replicas with a "follower" role can be updated first, while the replica with the "leader"
	//   role is updated last. This helps minimize the number of leader changes during the update process.
	//
	// This field is immutable.
	//
	// +optional
	Roles []ReplicaRole `json:"roles,omitempty"`

	// This field has been deprecated since v0.9.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// This field is immutable.
	//
	// +kubebuilder:default=External
	// +optional
	RoleArbitrator *RoleArbitrator `json:"roleArbitrator,omitempty"`

	// Defines a set of hooks and procedures that customize the behavior of a Component throughout its lifecycle.
	// Actions are triggered at specific lifecycle stages:
	//
	//   - `postProvision`: Defines the hook to be executed after the creation of a Component,
	//     with `preCondition` specifying when the action should be fired relative to the Component's lifecycle stages:
	//     `Immediately`, `RuntimeReady`, `ComponentReady`, and `ClusterReady`.
	//   - `preTerminate`: Defines the hook to be executed before terminating a Component.
	//   - `roleProbe`: Defines the procedure which is invoked regularly to assess the role of replicas.
	//   - `switchover`: Defines the procedure for a controlled transition of leadership from the current leader to a new replica.
	//     This approach aims to minimize downtime and maintain availability in systems with a leader-follower topology,
	//     such as before planned maintenance or upgrades on the current leader node.
	//   - `memberJoin`: Defines the procedure to add a new replica to the replication group.
	//   - `memberLeave`: Defines the method to remove a replica from the replication group.
	//   - `readOnly`: Defines the procedure to switch a replica into the read-only state.
	//   - `readWrite`: transition a replica from the read-only state back to the read-write state.
	//   - `dataDump`: Defines the procedure to export the data from a replica.
	//   - `dataLoad`: Defines the procedure to import data into a replica.
	//   - `reconfigure`: Defines the procedure that update a replica with new configuration file.
	//   - `accountProvision`: Defines the procedure to generate a new database account.
	//
	// This field is immutable.
	//
	// +optional
	LifecycleActions *ComponentLifecycleActions `json:"lifecycleActions,omitempty"`

	// Lists external service dependencies of the Component, including services from other Clusters or outside the K8s environment.
	//
	// This field is immutable.
	//
	// +optional
	ServiceRefDeclarations []ServiceRefDeclaration `json:"serviceRefDeclarations,omitempty"`

	// `minReadySeconds` is the minimum duration in seconds that a new Pod should remain in the ready
	// state without any of its containers crashing to be considered available.
	// This ensures the Pod's stability and readiness to serve requests.
	//
	// A default value of 0 seconds means the Pod is considered available as soon as it enters the ready state.
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	MinReadySeconds int32 `json:"minReadySeconds,omitempty"`
}

// ComponentDefinitionStatus defines the observed state of ComponentDefinition.
type ComponentDefinitionStatus struct {
	// Refers to the most recent generation that has been observed for the ComponentDefinition.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the current status of the ComponentDefinition. Valid values include ``, `Available`, and `Unavailable`.
	// When the status is `Available`, the ComponentDefinition is ready and can be utilized by related objects.
	//
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Provides additional information about the current phase.
	//
	// +optional
	Message string `json:"message,omitempty"`
}

type ComponentVolume struct {
	// Specifies the name of the volume.
	// It must be a DNS_LABEL and unique within the pod.
	// More info can be found at: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	// Note: This field cannot be updated.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies whether the creation of a snapshot of this volume is necessary when performing a backup of the Component.
	//
	// Note: This field cannot be updated.
	//
	// +kubebuilder:default=false
	// +optional
	NeedSnapshot bool `json:"needSnapshot,omitempty"`

	// Sets the critical threshold for volume space utilization as a percentage (0-100).
	//
	// Exceeding this percentage triggers the system to switch the volume to read-only mode as specified in
	// `componentDefinition.spec.lifecycleActions.readOnly`.
	// This precaution helps prevent space depletion while maintaining read-only access.
	// If the space utilization later falls below this threshold, the system reverts the volume to read-write mode
	// as defined in `componentDefinition.spec.lifecycleActions.readWrite`, restoring full functionality.
	//
	// Note: This field cannot be updated.
	//
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	HighWatermark int `json:"highWatermark,omitempty"`
}

// ReplicasLimit defines the valid range of number of replicas supported.
//
// +kubebuilder:validation:XValidation:rule="self.minReplicas >= 0 && self.maxReplicas <= 128",message="the minimum and maximum limit of replicas should be in the range of [0, 128]"
// +kubebuilder:validation:XValidation:rule="self.minReplicas <= self.maxReplicas",message="the minimum replicas limit should be no greater than the maximum"
type ReplicasLimit struct {
	// The minimum limit of replicas.
	//
	// +kubebuilder:validation:Required
	MinReplicas int32 `json:"minReplicas"`

	// The maximum limit of replicas.
	//
	// +kubebuilder:validation:Required
	MaxReplicas int32 `json:"maxReplicas"`
}

type SystemAccount struct {
	// Specifies the unique identifier for the account. This name is used by other entities to reference the account.
	//
	// This field is immutable once set.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Indicates if this account is a system initialization account (e.g., MySQL root).
	//
	// This field is immutable once set.
	//
	// +kubebuilder:default=false
	// +optional
	InitAccount bool `json:"initAccount,omitempty"`

	// Defines the statement used to create the account with the necessary privileges.
	//
	// This field is immutable once set.
	//
	// +optional
	Statement string `json:"statement,omitempty"`

	// Specifies the policy for generating the account's password.
	//
	// This field is immutable once set.
	//
	// +optional
	PasswordGenerationPolicy PasswordConfig `json:"passwordGenerationPolicy"`

	// Refers to the secret from which data will be copied to create the new account.
	//
	// This field is immutable once set.
	//
	// +optional
	SecretRef *ProvisionSecretRef `json:"secretRef,omitempty"`
}

type Exporter struct {
	// Specifies the name of the built-in metrics exporter container.
	//
	// +optional
	ContainerName string `json:"containerName,omitempty"`

	// Specifies the http/https url path to scrape for metrics.
	// If empty, Prometheus uses the default value (e.g. `/metrics`).
	//
	// +kubebuilder:validation:default="/metrics"
	// +optional
	ScrapePath string `json:"scrapePath,omitempty"`

	// Specifies the port name to scrape for metrics.
	//
	// +optional
	ScrapePort string `json:"scrapePort,omitempty"`

	// Specifies the schema to use for scraping.
	// `http` and `https` are the expected values unless you rewrite the `__scheme__` label via relabeling.
	// If empty, Prometheus uses the default value `http`.
	//
	// +kubebuilder:validation:default="http"
	// +optional
	ScrapeScheme PrometheusScheme `json:"scrapeScheme,omitempty"`
}

// RoleArbitrator defines how to arbitrate the role of replicas.
//
// Deprecated since v0.9
// +enum
// +kubebuilder:validation:Enum={External,Lorry}
type RoleArbitrator string

const (
	ExternalRoleArbitrator RoleArbitrator = "External"
	LorryRoleArbitrator    RoleArbitrator = "Lorry"
)

// ReplicaRole represents a role that can be assumed by a component instance.
type ReplicaRole struct {
	// Defines the role's identifier. It is used to set the "apps.kubeblocks.io/role" label value
	// on the corresponding object.
	//
	// This field is immutable once set.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Pattern=`^.*[^\s]+.*$`
	Name string `json:"name"`

	// Indicates whether a replica assigned this role is capable of providing services.
	//
	// This field is immutable once set.
	//
	// +kubebuilder:default=false
	// +optional
	Serviceable bool `json:"serviceable,omitempty"`

	// Determines if a replica in this role has the authority to perform write operations.
	// A writable replica can modify data, handle update operations.
	//
	// This field is immutable once set.
	//
	// +kubebuilder:default=false
	// +optional
	Writable bool `json:"writable,omitempty"`

	// Specifies whether a replica with this role has voting rights.
	// In distributed systems, this typically means the replica can participate in consensus decisions,
	// configuration changes, or other processes that require a quorum.
	//
	// This field is immutable once set.
	//
	// +kubebuilder:default=false
	// +optional
	Votable bool `json:"votable,omitempty"`
}

// TargetPodSelector defines how to select pod(s) to execute an Action.
// +enum
// +kubebuilder:validation:Enum={Any,All,Role,Ordinal}
type TargetPodSelector string

const (
	AnyReplica      TargetPodSelector = "Any"
	AllReplicas     TargetPodSelector = "All"
	RoleSelector    TargetPodSelector = "Role"
	OrdinalSelector TargetPodSelector = "Ordinal"
)

// HTTPAction describes an Action that triggers HTTP requests.
// HTTPAction is to be implemented in future version.
type HTTPAction struct {
	// Specifies the endpoint to be requested on the HTTP server.
	//
	// +optional
	Path string `json:"path,omitempty"`

	// Specifies the target port for the HTTP request.
	// It can be specified either as a numeric value in the range of 1 to 65535,
	// or as a named port that meets the IANA_SVC_NAME specification.
	Port intstr.IntOrString `json:"port"`

	// Indicates the server's domain name or IP address. Defaults to the Pod's IP.
	// Prefer setting the "Host" header in httpHeaders when needed.
	//
	// +optional
	Host string `json:"host,omitempty"`

	// Designates the protocol used to make the request, such as HTTP or HTTPS.
	// If not specified, HTTP is used by default.
	//
	// +optional
	Scheme corev1.URIScheme `json:"scheme,omitempty"`

	// Represents the type of HTTP request to be made, such as "GET," "POST," "PUT," etc.
	// If not specified, "GET" is the default method.
	//
	// +optional
	Method string `json:"method,omitempty"`

	// Allows for the inclusion of custom headers in the request.
	// HTTP permits the use of repeated headers.
	//
	// +optional
	HTTPHeaders []corev1.HTTPHeader `json:"httpHeaders,omitempty"`
}

// ExecAction describes an Action that executes a command inside a container.
// Which may run as a K8s job or be executed inside the Lorry sidecar container, depending on the implementation.
// Future implementations will standardize execution within Lorry.
type ExecAction struct {
	// Specifies the command to be executed inside the container.
	// The working directory for this command is the container's root directory('/').
	// Commands are executed directly without a shell environment, meaning shell-specific syntax ('|', etc.) is not supported.
	// If the shell is required, it must be explicitly invoked in the command.
	//
	// A successful execution is indicated by an exit status of 0; any non-zero status signifies a failure.
	//
	// +optional
	Command []string `json:"command,omitempty" protobuf:"bytes,1,rep,name=command"`

	// Args represents the arguments that are passed to the `command` for execution.
	//
	// +optional
	Args []string `json:"args,omitempty"`
}

type RetryPolicy struct {
	// Defines the maximum number of retry attempts that should be made for a given Action.
	// This value is set to 0 by default, indicating that no retries will be made.
	//
	// +kubebuilder:default=0
	// +optional
	MaxRetries int `json:"maxRetries,omitempty"`

	// Indicates the duration of time to wait between each retry attempt.
	// This value is set to 0 by default, indicating that there will be no delay between retry attempts.
	//
	// +kubebuilder:default=0
	// +optional
	RetryInterval time.Duration `json:"retryInterval,omitempty"`
}

// PreConditionType defines the preCondition type of the action execution.
type PreConditionType string

const (
	ImmediatelyPreConditionType    PreConditionType = "Immediately"
	RuntimeReadyPreConditionType   PreConditionType = "RuntimeReady"
	ComponentReadyPreConditionType PreConditionType = "ComponentReady"
	ClusterReadyPreConditionType   PreConditionType = "ClusterReady"
)

// Action defines a customizable hook or procedure tailored for different database engines,
// designed to be invoked at predetermined points within the lifecycle of a Component instance.
// It provides a modular and extensible way to customize a Component's behavior through the execution of defined actions.
//
// Available Action triggers include:
//
//   - `postProvision`: Defines the hook to be executed after the creation of a Component,
//     with `preCondition` specifying when the action should be fired relative to the Component's lifecycle stages:
//     `Immediately`, `RuntimeReady`, `ComponentReady`, and `ClusterReady`.
//   - `preTerminate`: Defines the hook to be executed before terminating a Component.
//   - `roleProbe`: Defines the procedure which is invoked regularly to assess the role of replicas.
//   - `switchover`: Defines the procedure for a controlled transition of leadership from the current leader to a new replica.
//     This approach aims to minimize downtime and maintain availability in systems with a leader-follower topology,
//     such as during planned maintenance or upgrades on the current leader node.
//   - `memberJoin`: Defines the procedure to add a new replica to the replication group.
//   - `memberLeave`: Defines the method to remove a replica from the replication group.
//   - `readOnly`: Defines the procedure to switch a replica into the read-only state.
//   - `readWrite`: Defines the procedure to transition a replica from the read-only state back to the read-write state.
//   - `dataDump`: Defines the procedure to export the data from a replica.
//   - `dataLoad`: Defines the procedure to import data into a replica.
//   - `reconfigure`: Defines the procedure that update a replica with new configuration.
//   - `accountProvision`: Defines the procedure to generate a new database account.
//
// Actions can be executed in different ways:
//
//   - ExecAction: Executes a command inside a container.
//     which may run as a K8s job or be executed inside the Lorry sidecar container, depending on the implementation.
//     Future implementations will standardize execution within Lorry.
//     A set of predefined environment variables are available and can be leveraged within the `exec.command`
//     to access context information such as details about pods, components, the overall cluster state,
//     or database connection credentials.
//     These variables provide a dynamic and context-aware mechanism for script execution.
//   - HTTPAction: Performs an HTTP request.
//     HTTPAction is to be implemented in future version.
//   - GRPCAction: In future version, Actions will support initiating gRPC calls.
//     This allows developers to implement Actions using plugins written in programming language like Go,
//     providing greater flexibility and extensibility.
//
// An action is considered successful on returning 0, or HTTP 200 for status HTTP(s) Actions.
// Any other return value or HTTP status codes indicate failure,
// and the action may be retried based on the configured retry policy.
//
//   - If an action exceeds the specified timeout duration, it will be terminated, and the action is considered failed.
//   - If an action produces any data as output, it should be written to stdout,
//     or included in the HTTP response payload for HTTP(s) actions.
//   - If an action encounters any errors, error messages should be written to stderr,
//     or detailed in the HTTP response with the appropriate non-200 status code.
type Action struct {
	// Specifies the container image to be used for running the Action.
	//
	// When specified, a dedicated container will be created using this image to execute the Action.
	// This field is mutually exclusive with the `container` field; only one of them should be provided.
	//
	// This field cannot be updated.
	//
	// +optional
	Image string `json:"image,omitempty"`

	// Defines the command to run.
	//
	// This field cannot be updated.
	//
	// +optional
	Exec *ExecAction `json:"exec,omitempty"`

	// Specifies the HTTP request to perform.
	//
	// This field cannot be updated.
	//
	// Note: HTTPAction is to be implemented in future version.
	//
	// +optional
	HTTP *HTTPAction `json:"http,omitempty"`

	// Represents a list of environment variables that will be injected into the container.
	// These variables enable the container to adapt its behavior based on the environment it's running in.
	//
	// This field cannot be updated.
	//
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// Defines the criteria used to select the target Pod(s) for executing the Action.
	// This is useful when there is no default target replica identified.
	// It allows for precise control over which Pod(s) the Action should run in.
	//
	// This field cannot be updated.
	//
	// Note: This field is reserved for future use and is not currently active.
	//
	// +optional
	TargetPodSelector TargetPodSelector `json:"targetPodSelector,omitempty"`

	// Used in conjunction with the `targetPodSelector` field to refine the selection of target pod(s) for Action execution.
	// The impact of this field depends on the `targetPodSelector` value:
	//
	// - When `targetPodSelector` is set to `Any` or `All`, this field will be ignored.
	// - When `targetPodSelector` is set to `Role`, only those replicas whose role matches the `matchingKey`
	//   will be selected for the Action.
	//
	// This field cannot be updated.
	//
	// Note: This field is reserved for future use and is not currently active.
	//
	// +optional
	MatchingKey string `json:"matchingKey,omitempty"`

	// Defines the name of the container within the target Pod where the action will be executed.
	//
	// This name must correspond to one of the containers defined in `componentDefinition.spec.runtime`.
	// If this field is not specified, the default behavior is to use the first container listed in
	// `componentDefinition.spec.runtime`.
	//
	// This field cannot be updated.
	//
	// Note: This field is reserved for future use and is not currently active.
	//
	// +optional
	Container string `json:"container,omitempty"`

	// Specifies the maximum duration in seconds that the Action is allowed to run.
	//
	// If the Action does not complete within this time frame, it will be terminated.
	//
	// This field cannot be updated.
	//
	// +kubebuilder:default=0
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`

	// Defines the strategy to be taken when retrying the Action after a failure.
	//
	// It specifies the conditions under which the Action should be retried and the limits to apply,
	// such as the maximum number of retries and backoff strategy.
	//
	// This field cannot be updated.
	//
	// +optional
	RetryPolicy *RetryPolicy `json:"retryPolicy,omitempty"`

	// Specifies the state that the cluster must reach before the Action is executed.
	// Currently, this is only applicable to the `postProvision` action.
	//
	// The conditions are as follows:
	//
	// - `Immediately`: Executed right after the Component object is created.
	//   The readiness of the Component and its resources is not guaranteed at this stage.
	// - `RuntimeReady`: The Action is triggered after the Component object has been created and all associated
	//   runtime resources (e.g. Pods) are in a ready state.
	// - `ComponentReady`: The Action is triggered after the Component itself is in a ready state.
	//   This process does not affect the readiness state of the Component or the Cluster.
	// - `ClusterReady`: The Action is executed after the Cluster is in a ready state.
	//   This execution does not alter the Component or the Cluster's state of readiness.
	//
	// This field cannot be updated.
	//
	// +optional
	PreCondition *PreConditionType `json:"preCondition,omitempty"`
}

// BuiltinActionHandlerType defines build-in action handlers provided by Lorry, including:
//
// - `mysql`
// - `wesql`
// - `oceanbase`
// - `redis`
// - `mongodb`
// - `etcd`
// - `postgresql`
// - `official-postgresql`
// - `apecloud-postgresql`
// - `polardbx`
// - `custom`
// - `unknown`
type BuiltinActionHandlerType string

const (
	MySQLBuiltinActionHandler              BuiltinActionHandlerType = "mysql"
	WeSQLBuiltinActionHandler              BuiltinActionHandlerType = "wesql"
	OceanbaseBuiltinActionHandler          BuiltinActionHandlerType = "oceanbase"
	RedisBuiltinActionHandler              BuiltinActionHandlerType = "redis"
	MongoDBBuiltinActionHandler            BuiltinActionHandlerType = "mongodb"
	ETCDBuiltinActionHandler               BuiltinActionHandlerType = "etcd"
	PostgresqlBuiltinActionHandler         BuiltinActionHandlerType = "postgresql"
	OfficialPostgresqlBuiltinActionHandler BuiltinActionHandlerType = "official-postgresql"
	ApeCloudPostgresqlBuiltinActionHandler BuiltinActionHandlerType = "apecloud-postgresql"
	PolarDBXBuiltinActionHandler           BuiltinActionHandlerType = "polardbx"
	CustomActionHandler                    BuiltinActionHandlerType = "custom"
	UnknownBuiltinActionHandler            BuiltinActionHandlerType = "unknown"
)

// LifecycleActionHandler describes the implementation of a specific lifecycle action.
//
// Each action is deemed successful if it returns an exit code of 0 for command executions,
// or an HTTP 200 status for HTTP(s) actions.
// Any other exit code or HTTP status is considered an indication of failure.
type LifecycleActionHandler struct {
	// Specifies the name of the predefined action handler to be invoked for lifecycle actions.
	//
	// Lorry, as a sidecar agent co-located with the database container in the same Pod,
	// includes a suite of built-in action implementations that are tailored to different database engines.
	// These are known as "builtin" handlers, includes: `mysql`, `redis`, `mongodb`, `etcd`,
	// `postgresql`, `official-postgresql`, `apecloud-postgresql`, `wesql`, `oceanbase`, `polardbx`.
	//
	// If the `builtinHandler` field is specified, it instructs Lorry to utilize its internal built-in action handler
	// to execute the specified lifecycle actions.
	//
	// The `builtinHandler` field is of type `BuiltinActionHandlerType`,
	// which represents the name of the built-in handler.
	// The `builtinHandler` specified within the same `ComponentLifecycleActions` should be consistent across all
	// actions.
	// This means that if you specify a built-in handler for one action, you should use the same handler
	// for all other actions throughout the entire `ComponentLifecycleActions` collection.
	//
	// If you need to define lifecycle actions for database engines not covered by the existing built-in support,
	// or when the pre-existing built-in handlers do not meet your specific needs,
	// you can use the `customHandler` field to define your own action implementation.
	//
	// Deprecation Notice:
	//
	// - In the future, the `builtinHandler` field will be deprecated in favor of using the `customHandler` field
	//   for configuring all lifecycle actions.
	// - Instead of using a name to indicate the built-in action implementations in Lorry,
	//   the recommended approach will be to explicitly invoke the desired action implementation through
	//   a gRPC interface exposed by the sidecar agent.
	// - Developers will have the flexibility to either use the built-in action implementations provided by Lorry
	//   or develop their own sidecar agent to implement custom actions and expose them via gRPC interfaces.
	// - This change will allow for greater customization and extensibility of lifecycle actions,
	//   as developers can create their own "builtin" implementations tailored to their specific requirements.
	//
	// +optional
	BuiltinHandler *BuiltinActionHandlerType `json:"builtinHandler,omitempty"`

	// Specifies a user-defined hook or procedure that is called to perform the specific lifecycle action.
	// It offers a flexible and expandable approach for customizing the behavior of a Component by leveraging
	// tailored actions.
	//
	// An Action can be implemented as either an ExecAction or an HTTPAction, with future versions planning
	// to support GRPCAction,
	// thereby accommodating unique logic for different database systems within the Action's framework.
	//
	// In future iterations, all built-in handlers are expected to transition to GRPCAction.
	// This change means that Lorry or other sidecar agents will expose the implementation of actions
	// through a GRPC interface for external invocation.
	// Then the controller will interact with these actions via GRPCAction calls.
	//
	// +optional
	CustomHandler *Action `json:"customHandler,omitempty"`
}

// ComponentLifecycleActions defines a collection of Actions for customizing the behavior of a Component.
type ComponentLifecycleActions struct {
	// Specifies the hook to be executed after a component's creation.
	//
	// By setting `postProvision.customHandler.preCondition`, you can determine the specific lifecycle stage
	// at which the action should trigger: `Immediately`, `RuntimeReady`, `ComponentReady`, and `ClusterReady`.
	// with `ComponentReady` being the default.
	//
	// The PostProvision Action is intended to run only once.
	//
	// The container executing this action has access to following environment variables:
	//
	// - KB_CLUSTER_POD_IP_LIST: Comma-separated list of the cluster's pod IP addresses (e.g., "podIp1,podIp2").
	// - KB_CLUSTER_POD_NAME_LIST: Comma-separated list of the cluster's pod names (e.g., "pod1,pod2").
	// - KB_CLUSTER_POD_HOST_NAME_LIST: Comma-separated list of host names, each corresponding to a pod in
	//   KB_CLUSTER_POD_NAME_LIST (e.g., "hostName1,hostName2").
	// - KB_CLUSTER_POD_HOST_IP_LIST: Comma-separated list of host IP addresses, each corresponding to a pod in
	//   KB_CLUSTER_POD_NAME_LIST (e.g., "hostIp1,hostIp2").
	//
	// - KB_CLUSTER_COMPONENT_POD_NAME_LIST: Comma-separated list of all pod names within the component
	//   (e.g., "pod1,pod2").
	// - KB_CLUSTER_COMPONENT_POD_IP_LIST: Comma-separated list of pod IP addresses,
	//   matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., "podIp1,podIp2").
	// - KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST: Comma-separated list of host names for each pod,
	//   matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., "hostName1,hostName2").
	// - KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST: Comma-separated list of host IP addresses for each pod,
	//   matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., "hostIp1,hostIp2").
	//
	// - KB_CLUSTER_COMPONENT_LIST: Comma-separated list of all cluster components (e.g., "comp1,comp2").
	// - KB_CLUSTER_COMPONENT_DELETING_LIST: Comma-separated list of components that are currently being deleted
	//   (e.g., "comp1,comp2").
	// - KB_CLUSTER_COMPONENT_UNDELETED_LIST: Comma-separated list of components that are not being deleted
	//   (e.g., "comp1,comp2").
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	PostProvision *LifecycleActionHandler `json:"postProvision,omitempty"`

	// Specifies the hook to be executed prior to terminating a component.
	//
	// The PreTerminate Action is intended to run only once.
	//
	// This action is executed immediately when a scale-down operation for the Component is initiated.
	// The actual termination and cleanup of the Component and its associated resources will not proceed
	// until the PreTerminate action has completed successfully.
	//
	// The container executing this action has access to following environment variables:
	//
	// - KB_CLUSTER_POD_IP_LIST: Comma-separated list of the cluster's pod IP addresses (e.g., "podIp1,podIp2").
	// - KB_CLUSTER_POD_NAME_LIST: Comma-separated list of the cluster's pod names (e.g., "pod1,pod2").
	// - KB_CLUSTER_POD_HOST_NAME_LIST: Comma-separated list of host names, each corresponding to a pod in
	//   KB_CLUSTER_POD_NAME_LIST (e.g., "hostName1,hostName2").
	// - KB_CLUSTER_POD_HOST_IP_LIST: Comma-separated list of host IP addresses, each corresponding to a pod in
	//   KB_CLUSTER_POD_NAME_LIST (e.g., "hostIp1,hostIp2").
	//
	// - KB_CLUSTER_COMPONENT_POD_NAME_LIST: Comma-separated list of all pod names within the component
	//   (e.g., "pod1,pod2").
	// - KB_CLUSTER_COMPONENT_POD_IP_LIST: Comma-separated list of pod IP addresses,
	//   matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., "podIp1,podIp2").
	// - KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST: Comma-separated list of host names for each pod,
	//   matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., "hostName1,hostName2").
	// - KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST: Comma-separated list of host IP addresses for each pod,
	//   matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., "hostIp1,hostIp2").
	//
	// - KB_CLUSTER_COMPONENT_LIST: Comma-separated list of all cluster components (e.g., "comp1,comp2").
	// - KB_CLUSTER_COMPONENT_DELETING_LIST: Comma-separated list of components that are currently being deleted
	//   (e.g., "comp1,comp2").
	// - KB_CLUSTER_COMPONENT_UNDELETED_LIST: Comma-separated list of components that are not being deleted
	//   (e.g., "comp1,comp2").
	//
	// - KB_CLUSTER_COMPONENT_IS_SCALING_IN: Indicates whether the component is currently scaling in.
	//   If this variable is present and set to "true", it denotes that the component is undergoing a scale-in operation.
	//   During scale-in, data rebalancing is necessary to maintain cluster integrity.
	//   Contrast this with a cluster deletion scenario where data rebalancing is not required as the entire cluster
	//   is being cleaned up.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	PreTerminate *LifecycleActionHandler `json:"preTerminate,omitempty"`

	// Defines the procedure which is invoked regularly to assess the role of replicas.
	//
	// This action is periodically triggered by Lorry at the specified interval to determine the role of each replica.
	// Upon successful execution, the action's output designates the role of the replica,
	// which should match one of the predefined role names within `componentDefinition.spec.roles`.
	// The output is then compared with the previous successful execution result.
	// If a role change is detected, an event is generated to inform the controller,
	// which initiates an update of the replica's role.
	//
	// Defining a RoleProbe Action for a Component is required if roles are defined for the Component.
	// It ensures replicas are correctly labeled with their respective roles.
	// Without this, services that rely on roleSelectors might improperly direct traffic to wrong replicas.
	//
	// The container executing this action has access to following environment variables:
	//
	// - KB_POD_FQDN: The FQDN of the Pod whose role is being assessed.
	// - KB_SERVICE_PORT: The port used by the database service.
	// - KB_SERVICE_USER: The username with the necessary permissions to interact with the database service.
	// - KB_SERVICE_PASSWORD: The corresponding password for KB_SERVICE_USER to authenticate with the database service.
	//
	// Expected output of this action:
	// - On Success: The determined role of the replica, which must align with one of the roles specified
	//   in the component definition.
	// - On Failure: An error message, if applicable, indicating why the action failed.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	RoleProbe *RoleProbe `json:"roleProbe,omitempty"`

	// Defines the procedure for a controlled transition of leadership from the current leader to a new replica.
	// This approach aims to minimize downtime and maintain availability in systems with a leader-follower topology,
	// during events such as planned maintenance or when performing stop, shutdown, restart, or upgrade operations
	// involving the current leader node.
	//
	// The container executing this action has access to following environment variables:
	//
	// - KB_SWITCHOVER_CANDIDATE_NAME: The name of the pod for the new leader candidate, which may not be specified (empty).
	// - KB_SWITCHOVER_CANDIDATE_FQDN: The FQDN of the new leader candidate's pod, which may not be specified (empty).
	// - KB_LEADER_POD_IP: The IP address of the current leader's pod prior to the switchover.
	// - KB_LEADER_POD_NAME: The name of the current leader's pod prior to the switchover.
	// - KB_LEADER_POD_FQDN: The FQDN of the current leader's pod prior to the switchover.
	//
	// The environment variables with the following prefixes are deprecated and will be removed in future releases:
	//
	// - KB_REPLICATION_PRIMARY_POD_
	// - KB_CONSENSUS_LEADER_POD_
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	Switchover *ComponentSwitchover `json:"switchover,omitempty"`

	// Defines the procedure to add a new replica to the replication group.
	//
	// This action is initiated after a replica pod becomes ready.
	//
	// The role of the replica (e.g., primary, secondary) will be determined and assigned as part of the action command
	// implementation, or automatically by the database kernel or a sidecar utility like Patroni that implements
	// a consensus algorithm.
	//
	// The container executing this action has access to following environment variables:
	//
	// - KB_SERVICE_PORT: The port used by the database service.
	// - KB_SERVICE_USER: The username with the necessary permissions to interact with the database service.
	// - KB_SERVICE_PASSWORD: The corresponding password for KB_SERVICE_USER to authenticate with the database service.
	// - KB_PRIMARY_POD_FQDN: The FQDN of the primary Pod within the replication group.
	// - KB_MEMBER_ADDRESSES: A comma-separated list of Pod addresses for all replicas in the group.
	// - KB_NEW_MEMBER_POD_NAME: The pod name of the replica being added to the group.
	// - KB_NEW_MEMBER_POD_IP: The IP address of the replica being added to the group.
	//
	// Expected action output:
	// - On Failure: An error message detailing the reason for any failure encountered
	//   during the addition of the new member.
	//
	// For example, to add a new OBServer to an OceanBase Cluster in 'zone1', the following command may be used:
	//
	// ```yaml
	// command:
	// - bash
	// - -c
	// - |
	//    ADDRESS=$(KB_MEMBER_ADDRESSES%%,*)
	//    HOST=$(echo $ADDRESS | cut -d ':' -f 1)
	//    PORT=$(echo $ADDRESS | cut -d ':' -f 2)
	//    CLIENT="mysql -u $KB_SERVICE_USER -p$KB_SERVICE_PASSWORD -P $PORT -h $HOST -e"
	// 	  $CLIENT "ALTER SYSTEM ADD SERVER '$KB_NEW_MEMBER_POD_IP:$KB_SERVICE_PORT' ZONE 'zone1'"
	// ```
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	MemberJoin *LifecycleActionHandler `json:"memberJoin,omitempty"`

	// Defines the procedure to remove a replica from the replication group.
	//
	// This action is initiated before remove a replica from the group.
	// The operator will wait for MemberLeave to complete successfully before releasing the replica and cleaning up
	// related Kubernetes resources.
	//
	// The process typically includes updating configurations and informing other group members about the removal.
	// Data migration is generally not part of this action and should be handled separately if needed.
	//
	// The container executing this action has access to following environment variables:
	//
	// - KB_SERVICE_PORT: The port used by the database service.
	// - KB_SERVICE_USER: The username with the necessary permissions to interact with the database service.
	// - KB_SERVICE_PASSWORD: The corresponding password for KB_SERVICE_USER to authenticate with the database service.
	// - KB_PRIMARY_POD_FQDN: The FQDN of the primary Pod within the replication group.
	// - KB_MEMBER_ADDRESSES: A comma-separated list of Pod addresses for all replicas in the group.
	// - KB_LEAVE_MEMBER_POD_NAME: The pod name of the replica being removed from the group.
	// - KB_LEAVE_MEMBER_POD_IP: The IP address of the replica being removed from the group.
	//
	// Expected action output:
	// - On Failure: An error message, if applicable, indicating why the action failed.
	//
	// For example, to remove an OBServer from an OceanBase Cluster in 'zone1', the following command can be executed:
	//
	// ```yaml
	// command:
	// - bash
	// - -c
	// - |
	//    ADDRESS=$(KB_MEMBER_ADDRESSES%%,*)
	//    HOST=$(echo $ADDRESS | cut -d ':' -f 1)
	//    PORT=$(echo $ADDRESS | cut -d ':' -f 2)
	//    CLIENT="mysql -u $KB_SERVICE_USER  -p$KB_SERVICE_PASSWORD -P $PORT -h $HOST -e"
	// 	  $CLIENT "ALTER SYSTEM DELETE SERVER '$KB_LEAVE_MEMBER_POD_IP:$KB_SERVICE_PORT' ZONE 'zone1'"
	// ```
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	MemberLeave *LifecycleActionHandler `json:"memberLeave,omitempty"`

	// Defines the procedure to switch a replica into the read-only state.
	//
	// Use Case:
	// This action is invoked when the database's volume capacity nears its upper limit and space is about to be exhausted.
	//
	// The container executing this action has access to following environment variables:
	//
	// - KB_POD_FQDN: The FQDN of the replica pod whose role is being checked.
	// - KB_SERVICE_PORT: The port used by the database service.
	// - KB_SERVICE_USER: The username with the necessary permissions to interact with the database service.
	// - KB_SERVICE_PASSWORD: The corresponding password for KB_SERVICE_USER to authenticate with the database service.
	//
	// Expected action output:
	// - On Failure: An error message, if applicable, indicating why the action failed.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	Readonly *LifecycleActionHandler `json:"readonly,omitempty"`

	// Defines the procedure to transition a replica from the read-only state back to the read-write state.
	//
	// Use Case:
	// This action is used to bring back a replica that was previously in a read-only state,
	// which restricted write operations, to its normal operational state where it can handle
	// both read and write operations.
	//
	// The container executing this action has access to following environment variables:
	//
	// - KB_POD_FQDN: The FQDN of the replica pod whose role is being checked.
	// - KB_SERVICE_PORT: The port used by the database service.
	// - KB_SERVICE_USER: The username with the necessary permissions to interact with the database service.
	// - KB_SERVICE_PASSWORD: The corresponding password for KB_SERVICE_USER to authenticate with the database service.
	//
	// Expected action output:
	// - On Failure: An error message, if applicable, indicating why the action failed.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	Readwrite *LifecycleActionHandler `json:"readwrite,omitempty"`

	// Defines the procedure for exporting the data from a replica.
	//
	// Use Case:
	// This action is intended for initializing a newly created replica with data. It involves exporting data
	// from an existing replica and importing it into the new, empty replica. This is essential for synchronizing
	// the state of replicas across the system.
	//
	// Applicability:
	// Some database engines or associated sidecar applications (e.g., Patroni) may already provide this functionality.
	// In such cases, this action may not be required.
	//
	// The output should be a valid data dump streamed to stdout. It must exclude any irrelevant information to ensure
	// that only the necessary data is exported for import into the new replica.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	DataDump *LifecycleActionHandler `json:"dataDump,omitempty"`

	// Defines the procedure for importing data into a replica.
	//
	// Use Case:
	// This action is intended for initializing a newly created replica with data. It involves exporting data
	// from an existing replica and importing it into the new, empty replica. This is essential for synchronizing
	// the state of replicas across the system.
	//
	// Some database engines or associated sidecar applications (e.g., Patroni) may already provide this functionality.
	// In such cases, this action may not be required.
	//
	// Data should be received through stdin. If any error occurs during the process,
	// the action must be able to guarantee idempotence to allow for retries from the beginning.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	DataLoad *LifecycleActionHandler `json:"dataLoad,omitempty"`

	// Defines the procedure that update a replica with new configuration.
	//
	// Note: This field is immutable once it has been set.
	//
	// This Action is reserved for future versions.
	//
	// +optional
	Reconfigure *LifecycleActionHandler `json:"reconfigure,omitempty"`

	// Defines the procedure to generate a new database account.
	//
	// Use Case:
	// This action is designed to create system accounts that are utilized for replication, monitoring, backup,
	// and other administrative tasks.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	AccountProvision *LifecycleActionHandler `json:"accountProvision,omitempty"`
}

type ComponentSwitchover struct {
	// Represents the switchover process for a specified candidate primary or leader instance.
	// Note that only Action.Exec is currently supported, while Action.HTTP is not.
	//
	// +optional
	WithCandidate *Action `json:"withCandidate,omitempty"`

	// Represents a switchover process that does not involve a specific candidate primary or leader instance.
	// As with the previous field, only Action.Exec is currently supported, not Action.HTTP.
	//
	// +optional
	WithoutCandidate *Action `json:"withoutCandidate,omitempty"`

	// Used to define the selectors for the scriptSpecs that need to be referenced.
	// If this field is set, the scripts defined under the 'scripts' field can be invoked or referenced within an Action.
	//
	// This field is deprecated from v0.9.
	// This field is maintained for backward compatibility and its use is discouraged.
	// Existing usage should be updated to the current preferred approach to avoid compatibility issues in future releases.
	//
	// +kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.9.0"
	// +optional
	ScriptSpecSelectors []ScriptSpecSelector `json:"scriptSpecSelectors,omitempty"`
}

type RoleProbe struct {
	LifecycleActionHandler `json:",inline"`

	// Specifies the number of seconds to wait after the container has started before the RoleProbe
	// begins to detect the container's role.
	//
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty" protobuf:"varint,2,opt,name=initialDelaySeconds"`

	// Specifies the number of seconds after which the probe times out.
	// Defaults to 1 second. Minimum value is 1.
	//
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty" protobuf:"varint,3,opt,name=timeoutSeconds"`

	// Specifies the frequency at which the probe is conducted. This value is expressed in seconds.
	// Default to 10 seconds. Minimum value is 1.
	//
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty" protobuf:"varint,4,opt,name=periodSeconds"`
}
