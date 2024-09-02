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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
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

	// Specifies a BackupPolicyTemplate name if data protection functionality is supported.
	// Which will automatically create the corresponding backupPolicy.
	// +optional
	BackupPolicyTemplateName string `json:"backupPolicyTemplateName,omitempty"`

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
	// This field is immutable.
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

	// Defines the upper limit of the number of replicas supported by the Component.
	//
	// It defines the maximum number of replicas that can be created for the Component.
	// This field allows you to set a limit on the scalability of the Component, preventing it from exceeding a certain number of replicas.
	//
	// This field is immutable.
	//
	// +optional
	ReplicasLimit *ReplicasLimit `json:"replicasLimit,omitempty"`

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

	// Defines the built-in metrics exporter container.
	//
	// +optional
	Exporter *Exporter `json:"exporter,omitempty"`
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

// EnvVar represents a variable present in the env of Pod/Action or the template of config/script.
type EnvVar struct {
	// Name of the variable. Must be a C_IDENTIFIER.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Optional: no more than one of the following may be specified.

	// Variable references `$(VAR_NAME)` are expanded using the previously defined variables in the current context.
	//
	// If a variable cannot be resolved, the reference in the input string will be unchanged.
	// Double `$$` are reduced to a single `$`, which allows for escaping the `$(VAR_NAME)` syntax: i.e.
	//
	// - `$$(VAR_NAME)` will produce the string literal `$(VAR_NAME)`.
	//
	// Escaped references will never be expanded, regardless of whether the variable exists or not.
	// Defaults to "".
	//
	// +optional
	Value string `json:"value,omitempty"`

	// Source for the variable's value. Cannot be used if value is not empty.
	//
	// +optional
	ValueFrom *VarSource `json:"valueFrom,omitempty"`

	// A Go template expression that will be applied to the resolved value of the var.
	//
	// The expression will only be evaluated if the var is successfully resolved to a non-credential value.
	//
	// The resolved value can be accessed by its name within the expression, system vars and other user-defined
	// non-credential vars can be used within the expression in the same way.
	// Notice that, when accessing vars by its name, you should replace all the "-" in the name with "_", because of
	// that "-" is not a valid identifier in Go.
	//
	// All expressions are evaluated in the order the vars are defined. If a var depends on any vars that also
	// have expressions defined, be careful about the evaluation order as it may use intermediate values.
	//
	// The result of evaluation will be used as the final value of the var. If the expression fails to evaluate,
	// the resolving of var will also be considered failed.
	//
	// +optional
	Expression *string `json:"expression,omitempty"`
}

// VarSource represents a source for the value of an EnvVar.
type VarSource struct {
	// Selects a key of a ConfigMap.
	// +optional
	ConfigMapKeyRef *corev1.ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`

	// Selects a key of a Secret.
	// +optional
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`

	// Selects a defined var of host-network resources.
	// +optional
	HostNetworkVarRef *HostNetworkVarSelector `json:"hostNetworkVarRef,omitempty"`

	// Selects a defined var of a Service.
	// +optional
	ServiceVarRef *ServiceVarSelector `json:"serviceVarRef,omitempty"`

	// Selects a defined var of a Credential (SystemAccount).
	// +optional
	CredentialVarRef *CredentialVarSelector `json:"credentialVarRef,omitempty"`

	// Selects a defined var of a ServiceRef.
	// +optional
	ServiceRefVarRef *ServiceRefVarSelector `json:"serviceRefVarRef,omitempty"`

	// Selects a defined var of a Component.
	// +optional
	ComponentVarRef *ComponentVarSelector `json:"componentVarRef,omitempty"`

	// Selects a defined var of a Cluster.
	// +optional
	ClusterVarRef *ClusterVarSelector `json:"clusterVarRef,omitempty"`
}

// VarOption defines whether a variable is required or optional.
// +enum
// +kubebuilder:validation:Enum={Required,Optional}
type VarOption string

var (
	VarRequired VarOption = "Required"
	VarOptional VarOption = "Optional"
)

type NamedVar struct {
	// +optional
	Name string `json:"name,omitempty"`

	// +optional
	Option *VarOption `json:"option,omitempty"`
}

type RoledVar struct {
	// +optional
	Role string `json:"role,omitempty"`

	// +optional
	Option *VarOption `json:"option,omitempty"`
}

// ContainerVars defines the vars that can be referenced from a Container.
type ContainerVars struct {
	// The name of the container.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Container port to reference.
	//
	// +optional
	Port *NamedVar `json:"port,omitempty"`
}

// HostNetworkVars defines the vars that can be referenced from host-network resources.
type HostNetworkVars struct {
	// +optional
	Container *ContainerVars `json:"container,omitempty"`
}

// ServiceVars defines the vars that can be referenced from a Service.
type ServiceVars struct {
	// ServiceType references the type of the service.
	//
	// +optional
	ServiceType *VarOption `json:"serviceType,omitempty"`

	// +optional
	Host *VarOption `json:"host,omitempty"`

	// LoadBalancer represents the LoadBalancer ingress point of the service.
	//
	// If multiple ingress points are available, the first one will be used automatically, choosing between IP and Hostname.
	//
	// +optional
	LoadBalancer *VarOption `json:"loadBalancer,omitempty"`

	// Port references a port or node-port defined in the service.
	//
	// If the referenced service is a pod-service, there will be multiple service objects matched,
	// and the value will be presented in the following format: service1.name:port1,service2.name:port2...
	//
	// +optional
	Port *NamedVar `json:"port,omitempty"`
}

// CredentialVars defines the vars that can be referenced from a Credential (SystemAccount).
// !!!!! CredentialVars will only be used as environment variables for Pods & Actions, and will not be used to render the templates.
type CredentialVars struct {
	// +optional
	Username *VarOption `json:"username,omitempty"`

	// +optional
	Password *VarOption `json:"password,omitempty"`
}

// ServiceRefVars defines the vars that can be referenced from a ServiceRef.
type ServiceRefVars struct {
	// +optional
	Endpoint *VarOption `json:"endpoint,omitempty"`

	// +optional
	Host *VarOption `json:"host,omitempty"`

	// +optional
	Port *VarOption `json:"port,omitempty"`

	// +optional
	PodFQDNs *VarOption `json:"podFQDNs,omitempty"`

	CredentialVars `json:",inline"`
}

// HostNetworkVarSelector selects a var from host-network resources.
type HostNetworkVarSelector struct {
	// The component to select from.
	ClusterObjectReference `json:",inline"`

	HostNetworkVars `json:",inline"`
}

// ServiceVarSelector selects a var from a Service.
type ServiceVarSelector struct {
	// The Service to select from.
	// It can be referenced from the default headless service by setting the name to "headless".
	ClusterObjectReference `json:",inline"`

	ServiceVars `json:",inline"`
}

// CredentialVarSelector selects a var from a Credential (SystemAccount).
type CredentialVarSelector struct {
	// The Credential (SystemAccount) to select from.
	ClusterObjectReference `json:",inline"`

	CredentialVars `json:",inline"`
}

// ServiceRefVarSelector selects a var from a ServiceRefDeclaration.
type ServiceRefVarSelector struct {
	// The ServiceRefDeclaration to select from.
	ClusterObjectReference `json:",inline"`

	ServiceRefVars `json:",inline"`
}

// ComponentVarSelector selects a var from a Component.
type ComponentVarSelector struct {
	// The Component to select from.
	ClusterObjectReference `json:",inline"`

	ComponentVars `json:",inline"`
}

type ComponentVars struct {
	// Reference to the name of the Component object.
	//
	// +optional
	ComponentName *VarOption `json:"componentName,omitempty"`

	// Reference to the short name of the Component object.
	//
	// +optional
	ShortName *VarOption `json:"shortName,omitempty"`

	// Reference to the replicas of the component.
	//
	// +optional
	Replicas *VarOption `json:"replicas,omitempty"`

	// Reference to the pod name list of the component.
	// and the value will be presented in the following format: name1,name2,...
	//
	// +optional
	PodNames *VarOption `json:"podNames,omitempty"`

	// Reference to the pod FQDN list of the component.
	// The value will be presented in the following format: FQDN1,FQDN2,...
	//
	// +optional
	PodFQDNs *VarOption `json:"podFQDNs,omitempty"`

	// Reference to the pod name list of the component that have a specific role.
	// The value will be presented in the following format: name1,name2,...
	//
	// +optional
	PodNamesForRole *RoledVar `json:"podNamesForRole,omitempty"`

	// Reference to the pod FQDN list of the component that have a specific role.
	// The value will be presented in the following format: FQDN1,FQDN2,...
	//
	// +optional
	PodFQDNsForRole *RoledVar `json:"podFQDNsForRole,omitempty"`
}

// ClusterVarSelector selects a var from a Cluster.
type ClusterVarSelector struct {
	ClusterVars `json:",inline"`
}

type ClusterVars struct {
	// Reference to the namespace of the Cluster object.
	//
	// +optional
	Namespace *VarOption `json:"namespace,omitempty"`

	// Reference to the name of the Cluster object.
	//
	// +optional
	ClusterName *VarOption `json:"clusterName,omitempty"`

	// Reference to the UID of the Cluster object.
	//
	// +optional
	ClusterUID *VarOption `json:"clusterUID,omitempty"`
}

// ClusterObjectReference defines information to let you locate the referenced object inside the same Cluster.
type ClusterObjectReference struct {
	// Specifies the exact name, name prefix, or regular expression pattern for matching the name of the ComponentDefinition
	// custom resource (CR) used by the component that the referent object resident in.
	//
	// If not specified, the component itself will be used.
	//
	// +optional
	CompDef string `json:"compDef,omitempty"`

	// Name of the referent object.
	//
	// +optional
	Name string `json:"name,omitempty"`

	// Specify whether the object must be defined.
	//
	// +optional
	Optional *bool `json:"optional,omitempty"`

	// This option defines the behavior when multiple component objects match the specified @CompDef.
	// If not provided, an error will be raised when handling multiple matches.
	//
	// +optional
	MultipleClusterObjectOption *MultipleClusterObjectOption `json:"multipleClusterObjectOption,omitempty"`
}

// MultipleClusterObjectOption defines the options for handling multiple cluster objects matched.
type MultipleClusterObjectOption struct {
	// Define the strategy for handling multiple cluster objects.
	//
	// +kubebuilder:validation:Required
	Strategy MultipleClusterObjectStrategy `json:"strategy"`

	// Define the options for handling combined variables.
	// Valid only when the strategy is set to "combined".
	//
	// +optional
	CombinedOption *MultipleClusterObjectCombinedOption `json:"combinedOption,omitempty"`
}

// MultipleClusterObjectStrategy defines the strategy for handling multiple cluster objects.
// +enum
// +kubebuilder:validation:Enum={individual,combined}
type MultipleClusterObjectStrategy string

const (
	// MultipleClusterObjectStrategyIndividual - each matched component will have its individual variable with its name
	// as the suffix.
	// This is required when referencing credential variables that cannot be passed by values.
	MultipleClusterObjectStrategyIndividual MultipleClusterObjectStrategy = "individual"

	// MultipleClusterObjectStrategyCombined - the values from all matched components will be combined into a single
	// variable using the specified option.
	MultipleClusterObjectStrategyCombined MultipleClusterObjectStrategy = "combined"
)

// MultipleClusterObjectCombinedOption defines options for handling combined variables.
type MultipleClusterObjectCombinedOption struct {
	// If set, the existing variable will be kept, and a new variable will be defined with the specified suffix
	// in pattern: $(var.name)_$(suffix).
	// The new variable will be auto-created and placed behind the existing one.
	// If not set, the existing variable will be reused with the value format defined below.
	//
	// +optional
	NewVarSuffix *string `json:"newVarSuffix,omitempty"`

	// The format of the value that the operator will use to compose values from multiple components.
	//
	// +kubebuilder:default="Flatten"
	// +optional
	ValueFormat MultipleClusterObjectValueFormat `json:"valueFormat,omitempty"`

	// The flatten format, default is: $(comp-name-1):value,$(comp-name-2):value.
	//
	// +optional
	FlattenFormat *MultipleClusterObjectValueFormatFlatten `json:"flattenFormat,omitempty"`
}

// MultipleClusterObjectValueFormat defines the format details for the value.
type MultipleClusterObjectValueFormat string

const (
	FlattenFormat MultipleClusterObjectValueFormat = "Flatten"
)

// MultipleClusterObjectValueFormatFlatten defines the flatten format for the value.
type MultipleClusterObjectValueFormatFlatten struct {
	// Pair delimiter.
	//
	// +kubebuilder:default=","
	// +kubebuilder:validation:Required
	Delimiter string `json:"delimiter"`

	// Key-value delimiter.
	//
	// +kubebuilder:default=":"
	// +kubebuilder:validation:Required
	KeyValueDelimiter string `json:"keyValueDelimiter"`
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

type HostNetwork struct {
	// The list of container ports that are required by the component.
	//
	// +optional
	ContainerPorts []HostNetworkContainerPort `json:"containerPorts,omitempty"`
}

type HostNetworkContainerPort struct {
	// Container specifies the target container within the Pod.
	//
	// +required
	Container string `json:"container"`

	// Ports are named container ports within the specified container.
	// These container ports must be defined in the container for proper port allocation.
	//
	// +kubebuilder:validation:MinItems=1
	// +required
	Ports []string `json:"ports"`
}

type ComponentTemplateSpec struct {
	// Specifies the name of the configuration template.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Specifies the name of the referenced configuration template ConfigMap object.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	TemplateRef string `json:"templateRef"`

	// Specifies the namespace of the referenced configuration template ConfigMap object.
	// An empty namespace is equivalent to the "default" namespace.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
	// +kubebuilder:default="default"
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Refers to the volume name of PodTemplate. The configuration file produced through the configuration
	// template will be mounted to the corresponding volume. Must be a DNS_LABEL name.
	// The volume name must be defined in podSpec.containers[*].volumeMounts.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	// +optional
	VolumeName string `json:"volumeName"`

	// The operator attempts to set default file permissions for scripts (0555) and configurations (0444).
	// However, certain database engines may require different file permissions.
	// You can specify the desired file permissions here.
	//
	// Must be specified as an octal value between 0000 and 0777 (inclusive),
	// or as a decimal value between 0 and 511 (inclusive).
	// YAML supports both octal and decimal values for file permissions.
	//
	// Please note that this setting only affects the permissions of the files themselves.
	// Directories within the specified path are not impacted by this setting.
	// It's important to be aware that this setting might conflict with other options
	// that influence the file mode, such as fsGroup.
	// In such cases, the resulting file mode may have additional bits set.
	// Refers to documents of k8s.ConfigMapVolumeSource.defaultMode for more information.
	//
	// +optional
	DefaultMode *int32 `json:"defaultMode,omitempty" protobuf:"varint,3,opt,name=defaultMode"`
}

type ComponentConfigSpec struct {
	ComponentTemplateSpec `json:",inline"`

	// Specifies the configuration files within the ConfigMap that support dynamic updates.
	//
	// A configuration template (provided in the form of a ConfigMap) may contain templates for multiple
	// configuration files.
	// Each configuration file corresponds to a key in the ConfigMap.
	// Some of these configuration files may support dynamic modification and reloading without requiring
	// a pod restart.
	//
	// If empty or omitted, all configuration files in the ConfigMap are assumed to support dynamic updates,
	// and ConfigConstraint applies to all keys.
	//
	// +listType=set
	// +optional
	Keys []string `json:"keys,omitempty"`

	// Specifies the secondary rendered config spec for pod-specific customization.
	//
	// The template is rendered inside the pod (by the "config-manager" sidecar container) and merged with the main
	// template's render result to generate the final configuration file.
	//
	// This field is intended to handle scenarios where different pods within the same Component have
	// varying configurations. It allows for pod-specific customization of the configuration.
	//
	// Note: This field will be deprecated in future versions, and the functionality will be moved to
	// `cluster.spec.componentSpecs[*].instances[*]`.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0 and will be removed in 0.10.0"
	// +optional
	LegacyRenderedConfigSpec *LegacyRenderedTemplateSpec `json:"legacyRenderedConfigSpec,omitempty"`

	// Specifies the name of the referenced configuration constraints object.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	ConfigConstraintRef string `json:"constraintRef,omitempty"`

	// Specifies the containers to inject the ConfigMap parameters as environment variables.
	//
	// This is useful when application images accept parameters through environment variables and
	// generate the final configuration file in the startup script based on these variables.
	//
	// This field allows users to specify a list of container names, and KubeBlocks will inject the environment
	// variables converted from the ConfigMap into these designated containers. This provides a flexible way to
	// pass the configuration items from the ConfigMap to the container without modifying the image.
	//
	// Deprecated: `asEnvFrom` has been deprecated since 0.9.0 and will be removed in 0.10.0.
	// Use `injectEnvTo` instead.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0 and will be removed in 0.10.0"
	// +listType=set
	// +optional
	AsEnvFrom []string `json:"asEnvFrom,omitempty"`

	// Specifies the containers to inject the ConfigMap parameters as environment variables.
	//
	// This is useful when application images accept parameters through environment variables and
	// generate the final configuration file in the startup script based on these variables.
	//
	// This field allows users to specify a list of container names, and KubeBlocks will inject the environment
	// variables converted from the ConfigMap into these designated containers. This provides a flexible way to
	// pass the configuration items from the ConfigMap to the container without modifying the image.
	//
	//
	// +listType=set
	// +optional
	InjectEnvTo []string `json:"injectEnvTo,omitempty"`

	// Specifies whether the configuration needs to be re-rendered after v-scale or h-scale operations to reflect changes.
	//
	// In some scenarios, the configuration may need to be updated to reflect the changes in resource allocation
	// or cluster topology. Examples:
	//
	// - Redis: adjust maxmemory after v-scale operation.
	// - MySQL: increase max connections after v-scale operation.
	// - Zookeeper: update zoo.cfg with new node addresses after h-scale operation.
	//
	// +listType=set
	// +optional
	ReRenderResourceTypes []RerenderResourceType `json:"reRenderResourceTypes,omitempty"`

	// Whether to store the final rendered parameters as a secret.
	//
	// +optional
	AsSecret *bool `json:"asSecret,omitempty"`
}

// LegacyRenderedTemplateSpec describes the configuration extension for the lazy rendered template.
// Deprecated: LegacyRenderedTemplateSpec has been deprecated since 0.9.0 and will be removed in 0.10.0
type LegacyRenderedTemplateSpec struct {
	// Extends the configuration template.
	ConfigTemplateExtension `json:",inline"`
}

type ConfigTemplateExtension struct {
	// Specifies the name of the referenced configuration template ConfigMap object.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	TemplateRef string `json:"templateRef"`

	// Specifies the namespace of the referenced configuration template ConfigMap object.
	// An empty namespace is equivalent to the "default" namespace.
	//
	// +kubebuilder:default="default"
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Defines the strategy for merging externally imported templates into component templates.
	//
	// +kubebuilder:default="none"
	// +optional
	Policy MergedPolicy `json:"policy,omitempty"`
}

// MergedPolicy defines how to merge external imported templates into component templates.
// +enum
// +kubebuilder:validation:Enum={patch,replace,none}
type MergedPolicy string

const (
	PatchPolicy     MergedPolicy = "patch"
	ReplacePolicy   MergedPolicy = "replace"
	OnlyAddPolicy   MergedPolicy = "add"
	NoneMergePolicy MergedPolicy = "none"
)

// RerenderResourceType defines the resource requirements for a component.
// +enum
// +kubebuilder:validation:Enum={vscale,hscale,tls}
type RerenderResourceType string

const (
	ComponentVScaleType RerenderResourceType = "vscale"
	ComponentHScaleType RerenderResourceType = "hscale"
)

type LogConfig struct {
	// Specifies a descriptive label for the log type, such as 'slow' for a MySQL slow log file.
	// It provides a clear identification of the log's purpose and content.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	Name string `json:"name"`

	// Specifies the paths or patterns identifying where the log files are stored.
	// This field allows the system to locate and manage log files effectively.
	//
	// Examples:
	//
	// - /home/postgres/pgdata/pgroot/data/log/postgresql-*
	// - /data/mysql/log/mysqld-error.log
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=4096
	FilePathPattern string `json:"filePathPattern"`
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

// ReplicasLimit defines the valid range of number of replicas supported.
//
// +kubebuilder:validation:XValidation:rule="self.minReplicas >= 0 && self.maxReplicas <= 16384",message="the minimum and maximum limit of replicas should be in the range of [0, 16384]"
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

// UpdateStrategy defines the update strategy for cluster components. This strategy determines how updates are applied
// across the cluster.
// The available strategies are `Serial`, `BestEffortParallel`, and `Parallel`.
//
// +enum
// +kubebuilder:validation:Enum={Serial,BestEffortParallel,Parallel}
type UpdateStrategy string

const (
	// SerialStrategy indicates that updates are applied one at a time in a sequential manner.
	// The operator waits for each replica to be updated and ready before proceeding to the next one.
	// This ensures that only one replica is unavailable at a time during the update process.
	SerialStrategy UpdateStrategy = "Serial"

	// ParallelStrategy indicates that updates are applied simultaneously to all Pods of a Component.
	// The replicas are updated in parallel, with the operator updating all replicas concurrently.
	// This strategy provides the fastest update time but may lead to a period of reduced availability or
	// capacity during the update process.
	ParallelStrategy UpdateStrategy = "Parallel"

	// BestEffortParallelStrategy indicates that the replicas are updated in parallel, with the operator making
	// a best-effort attempt to update as many replicas as possible concurrently
	// while maintaining the component's availability.
	// Unlike the `Parallel` strategy, the `BestEffortParallel` strategy aims to ensure that a minimum number
	// of replicas remain available during the update process to maintain the component's quorum and functionality.
	//
	// For example, consider a component with 5 replicas. To maintain the component's availability and quorum,
	// the operator may allow a maximum of 2 replicas to be simultaneously updated. This ensures that at least
	// 3 replicas (a quorum) remain available and functional during the update process.
	//
	// The `BestEffortParallel` strategy strikes a balance between update speed and component availability.
	BestEffortParallelStrategy UpdateStrategy = "BestEffortParallel"
)

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
	// Note: This field is immutable once it has been set.
	//
	// +optional
	PostProvision *Action `json:"postProvision,omitempty"`

	// Specifies the hook to be executed prior to terminating a component.
	//
	// The PreTerminate Action is intended to run only once.
	//
	// This action is executed immediately when a scale-down operation for the Component is initiated.
	// The actual termination and cleanup of the Component and its associated resources will not proceed
	// until the PreTerminate action has completed successfully.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	PreTerminate *Action `json:"preTerminate,omitempty"`

	// Defines the procedure which is invoked regularly to assess the role of replicas.
	//
	// This action is periodically triggered at the specified interval to determine the role of each replica.
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
	// The container executing this action has access to following variables:
	//
	// - KB_POD_FQDN: The FQDN of the Pod whose role is being assessed.
	//
	// Expected output of this action:
	// - On Success: The determined role of the replica, which must align with one of the roles specified
	//   in the component definition.
	// - On Failure: An error message, if applicable, indicating why the action failed.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	RoleProbe *Probe `json:"roleProbe,omitempty"`

	// Defines the procedure for a controlled transition of leadership from the current leader to a new replica.
	// This approach aims to minimize downtime and maintain availability in systems with a leader-follower topology,
	// during events such as planned maintenance or when performing stop, shutdown, restart, or upgrade operations
	// involving the current leader node.
	//
	// The container executing this action has access to following variables:
	//
	// - KB_SWITCHOVER_CANDIDATE_NAME: The name of the pod for the new leader candidate, which may not be specified (empty).
	// - KB_SWITCHOVER_CANDIDATE_FQDN: The FQDN of the new leader candidate's pod, which may not be specified (empty).
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	Switchover *Action `json:"switchover,omitempty"`

	// Defines the procedure to add a new replica to the replication group.
	//
	// This action is initiated after a replica pod becomes ready.
	//
	// The role of the replica (e.g., primary, secondary) will be determined and assigned as part of the action command
	// implementation, or automatically by the database kernel or a sidecar utility like Patroni that implements
	// a consensus algorithm.
	//
	// The container executing this action has access to following variables:
	//
	// - KB_JOIN_MEMBER_POD_FQDN: The pod FQDN of the replica being added to the group.
	// - KB_JOIN_MEMBER_POD_NAME: The pod name of the replica being added to the group.
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
	//    CLIENT="mysql -u $SERVICE_USER -p$SERVICE_PASSWORD -P $SERVICE_PORT -h $SERVICE_HOST -e"
	// 	  $CLIENT "ALTER SYSTEM ADD SERVER '$KB_POD_FQDN:$SERVICE_PORT' ZONE 'zone1'"
	// ```
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	MemberJoin *Action `json:"memberJoin,omitempty"`

	// Defines the procedure to remove a replica from the replication group.
	//
	// This action is initiated before remove a replica from the group.
	// The operator will wait for MemberLeave to complete successfully before releasing the replica and cleaning up
	// related Kubernetes resources.
	//
	// The process typically includes updating configurations and informing other group members about the removal.
	// Data migration is generally not part of this action and should be handled separately if needed.
	//
	// The container executing this action has access to following variables:
	//
	// - KB_LEAVE_MEMBER_POD_FQDN: The pod name of the replica being removed from the group.
	// - KB_LEAVE_MEMBER_POD_NAME: The pod name of the replica being removed from the group.
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
	//    CLIENT="mysql -u $SERVICE_USER -p$SERVICE_PASSWORD -P $SERVICE_PORT -h $SERVICE_HOST -e"
	// 	  $CLIENT "ALTER SYSTEM DELETE SERVER '$KB_POD_FQDN:$SERVICE_PORT' ZONE 'zone1'"
	// ```
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	MemberLeave *Action `json:"memberLeave,omitempty"`

	// Defines the procedure to switch a replica into the read-only state.
	//
	// Use Case:
	// This action is invoked when the database's volume capacity nears its upper limit and space is about to be exhausted.
	//
	// The container executing this action has access to following environment variables:
	//
	// - KB_POD_FQDN: The FQDN of the replica pod whose role is being checked.
	//
	// Expected action output:
	// - On Failure: An error message, if applicable, indicating why the action failed.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	Readonly *Action `json:"readonly,omitempty"`

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
	//
	// Expected action output:
	// - On Failure: An error message, if applicable, indicating why the action failed.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	Readwrite *Action `json:"readwrite,omitempty"`

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
	DataDump *Action `json:"dataDump,omitempty"`

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
	DataLoad *Action `json:"dataLoad,omitempty"`

	// Defines the procedure that update a replica with new configuration.
	//
	// Note: This field is immutable once it has been set.
	//
	// This Action is reserved for future versions.
	//
	// +optional
	Reconfigure *Action `json:"reconfigure,omitempty"`

	// Defines the procedure to generate a new database account.
	//
	// Use Case:
	// This action is designed to create system accounts that are utilized for replication, monitoring, backup,
	// and other administrative tasks.
	//
	// The container executing this action has access to following variables:
	//
	// - KB_ACCOUNT_NAME: The name of the system account to be created.
	// - KB_ACCOUNT_PASSWORD: The password for the system account.  // TODO: how to pass the password securely?
	// - KB_ACCOUNT_STATEMENT: The statement used to create the system account.
	//
	// Note: This field is immutable once it has been set.
	//
	// +optional
	AccountProvision *Action `json:"accountProvision,omitempty"`
}

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
	// Defines the command to run.
	//
	// This field cannot be updated.
	//
	// +optional
	Exec *ExecAction `json:"exec,omitempty"`

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

// ExecAction describes an Action that executes a command inside a container.
type ExecAction struct {
	// Specifies the container image to be used for running the Action.
	//
	// When specified, a dedicated container will be created using this image to execute the Action.
	// All actions with same image will share the same container.
	//
	// This field cannot be updated.
	//
	// +optional
	Image string `json:"image,omitempty"`

	// Represents a list of environment variables that will be injected into the container.
	// These variables enable the container to adapt its behavior based on the environment it's running in.
	//
	// This field cannot be updated.
	//
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// Specifies the command to be executed inside the container.
	// The working directory for this command is the container's root directory('/').
	// Commands are executed directly without a shell environment, meaning shell-specific syntax ('|', etc.) is not supported.
	// If the shell is required, it must be explicitly invoked in the command.
	//
	// A successful execution is indicated by an exit status of 0; any non-zero status signifies a failure.
	//
	// +optional
	Command []string `json:"command,omitempty"`

	// Args represents the arguments that are passed to the `command` for execution.
	//
	// +optional
	Args []string `json:"args,omitempty"`

	// Defines the criteria used to select the target Pod(s) for executing the Action.
	// This is useful when there is no default target replica identified.
	// It allows for precise control over which Pod(s) the Action should run in.
	//
	// If not specified, the Action will be executed in the pod where the Action is triggered, such as the pod
	// to be removed or added; or a random pod if the Action is triggered at the component level, such as
	// post-provision or pre-terminate of the component.
	//
	// This field cannot be updated.
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
	// +optional
	MatchingKey string `json:"matchingKey,omitempty"`

	// Specifies the name of the container within the same pod whose resources will be shared with the action.
	// This allows the action to utilize the specified container's resources without executing within it.
	//
	// The name must match one of the containers defined in `componentDefinition.spec.runtime`.
	//
	// The resources that can be shared are included:
	//
	// - volume mounts
	//
	// This field cannot be updated.
	//
	// +optional
	Container string `json:"container,omitempty"`
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

type Probe struct {
	Action `json:",inline"`

	// Specifies the number of seconds to wait after the container has started before the RoleProbe
	// begins to detect the container's role.
	//
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`

	// Specifies the frequency at which the probe is conducted. This value is expressed in seconds.
	// Default to 10 seconds. Minimum value is 1.
	//
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`

	// Minimum consecutive successes for the probe to be considered successful after having failed.
	// Defaults to 1. Minimum value is 1.
	//
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty"`

	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	// Defaults to 3. Minimum value is 1.
	//
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`
}

// ServiceRefDeclaration represents a reference to a service that can be either provided by a KubeBlocks Cluster
// or an external service.
// It acts as a placeholder for the actual service reference, which is determined later when a Cluster is created.
//
// The purpose of ServiceRefDeclaration is to declare a service dependency without specifying the concrete details
// of the service.
// It allows for flexibility and abstraction in defining service references within a Component.
// By using ServiceRefDeclaration, you can define service dependencies in a declarative manner, enabling loose coupling
// and easier management of service references across different components and clusters.
//
// Upon Cluster creation, the ServiceRefDeclaration is bound to an actual service through the ServiceRef field,
// effectively resolving and connecting to the specified service.
type ServiceRefDeclaration struct {
	// Specifies the name of the ServiceRefDeclaration.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Defines a list of constraints and requirements for services that can be bound to this ServiceRefDeclaration
	// upon Cluster creation.
	// Each ServiceRefDeclarationSpec defines a ServiceKind and ServiceVersion,
	// outlining the acceptable service types and versions that are compatible.
	//
	// This flexibility allows a ServiceRefDeclaration to be fulfilled by any one of the provided specs.
	// For example, if it requires an OLTP database, specs for both MySQL and PostgreSQL are listed,
	// either MySQL or PostgreSQL services can be used when binding.
	//
	// +kubebuilder:validation:Required
	ServiceRefDeclarationSpecs []ServiceRefDeclarationSpec `json:"serviceRefDeclarationSpecs"`

	// Specifies whether the service reference can be optional.
	//
	// For an optional service-ref, the component can still be created even if the service-ref is not provided.
	//
	// +optional
	Optional *bool `json:"optional,omitempty"`
}

type ServiceRefDeclarationSpec struct {
	// Specifies the type or nature of the service. This should be a well-known application cluster type, such as
	// {mysql, redis, mongodb}.
	// The field is case-insensitive and supports abbreviations for some well-known databases.
	// For instance, both `zk` and `zookeeper` are considered as a ZooKeeper cluster, while `pg`, `postgres`, `postgresql`
	// are all recognized as a PostgreSQL cluster.
	//
	// +kubebuilder:validation:Required
	ServiceKind string `json:"serviceKind"`

	// Defines the service version of the service reference. This is a regular expression that matches a version number pattern.
	// For instance, `^8.0.8$`, `8.0.\d{1,2}$`, `^[v\-]*?(\d{1,2}\.){0,3}\d{1,2}$` are all valid patterns.
	//
	// +kubebuilder:validation:Required
	ServiceVersion string `json:"serviceVersion"`
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

// PrometheusScheme defines the protocol of prometheus scrape metrics.
//
// +enum
// +kubebuilder:validation:Enum={http,https}
type PrometheusScheme string

const (
	HTTPProtocol  PrometheusScheme = "http"
	HTTPSProtocol PrometheusScheme = "https"
)
