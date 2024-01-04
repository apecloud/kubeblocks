/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

// ComponentDefinition is the Schema for the componentdefinitions API
type ComponentDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentDefinitionSpec   `json:"spec,omitempty"`
	Status ComponentDefinitionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ComponentDefinitionList contains a list of ComponentDefinition
type ComponentDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentDefinition{}, &ComponentDefinitionList{})
}

// ComponentDefinitionSpec provides a workload component specification with attributes that strongly work with stateful workloads and day-2 operation behaviors.
type ComponentDefinitionSpec struct {
	// Provider is the name of the component provider.
	// +kubebuilder:validation:MaxLength=32
	// +optional
	Provider string `json:"provider,omitempty"`

	// Description is a brief description of the component.
	// +kubebuilder:validation:MaxLength=256
	// +optional
	Description string `json:"description,omitempty"`

	// ServiceKind defines what kind of well-known service that the component provides (e.g., MySQL, Redis, ETCD, case insensitive).
	// Cannot be updated.
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceKind string `json:"serviceKind,omitempty"`

	// ServiceVersion defines the version of the well-known service that the component provides.
	// Cannot be updated.
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceVersion string `json:"serviceVersion,omitempty"`

	// Runtime defines primarily runtime information for the component, including:
	//   - Init containers
	//   - Containers
	//       - Image
	//       - Commands
	//       - Args
	//       - Envs
	//       - Mounts
	//       - Ports
	//       - Security context
	//       - Probes
	//       - Lifecycle
	//   - Volumes
	// CPU and memory resource limits, as well as scheduling settings (affinity, toleration, priority), should not be configured within this structure.
	// Cannot be updated.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Required
	Runtime corev1.PodSpec `json:"runtime"`

	// Vars represents user-defined variables.
	// These variables can be utilized as environment variables for Pods and Actions, or to render the templates of config and script.
	// When used as environment variables, these variables are placed in front of the environment variables declared in the Pod.
	// Cannot be updated.
	// +optional
	Vars []EnvVar `json:"vars,omitempty"`

	// Volumes defines the persistent volumes needed by the component.
	// The users are responsible for providing these volumes when creating a component instance.
	// Cannot be updated.
	// +optional
	Volumes []ComponentVolume `json:"volumes"`

	// Services defines endpoints that can be used to access the component service to manage the component.
	// In addition, a reserved headless service will be created by default, with the name pattern {clusterName}-{componentName}-headless.
	// Cannot be updated.
	// +optional
	Services []ComponentService `json:"services,omitempty"`

	// The configs field provided by provider, and
	// finally this configTemplateRefs will be rendered into the user's own configuration file according to the user's cluster.
	// Cannot be updated.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	// TODO: support referencing configs from other components or clusters.
	Configs []ComponentConfigSpec `json:"configs,omitempty"`

	// LogConfigs is detail log file config which provided by provider.
	// Cannot be updated.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	LogConfigs []LogConfig `json:"logConfigs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Monitor is monitoring config which provided by provider.
	// Cannot be updated.
	// +optional
	Monitor *MonitorConfig `json:"monitor,omitempty"`

	// The scripts field provided by provider, and
	// finally this configTemplateRefs will be rendered into the user's own configuration file according to the user's cluster.
	// Cannot be updated.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	Scripts []ComponentTemplateSpec `json:"scripts,omitempty"`

	// PolicyRules defines the namespaced policy rules required by the component.
	// If any rule application fails (e.g., due to lack of permissions), the provisioning of the component instance will also fail.
	// Cannot be updated.
	// +optional
	PolicyRules []rbacv1.PolicyRule `json:"policyRules,omitempty"`

	// Labels defines static labels that will be patched to all k8s resources created for the component.
	// If a label key conflicts with any other system labels or user-specified labels, it will be silently ignored.
	// Cannot be updated.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// ReplicasLimit defines the limit of valid replicas supported.
	// Cannot be updated.
	// +optional
	ReplicasLimit *ReplicasLimit `json:"replicasLimit,omitempty"`

	// SystemAccounts defines the pre-defined system accounts required to manage the component.
	// TODO(component): accounts KB required
	// Cannot be updated.
	// +optional
	SystemAccounts []SystemAccount `json:"systemAccounts,omitempty"`

	// UpdateStrategy defines the strategy for updating the component instance.
	// Cannot be updated.
	// +kubebuilder:default=Serial
	// +optional
	UpdateStrategy *UpdateStrategy `json:"updateStrategy,omitempty"`

	// Roles defines all the roles that the component can assume.
	// Cannot be updated.
	// +optional
	Roles []ReplicaRole `json:"roles,omitempty"`

	// RoleArbitrator defines the strategy for electing the component's active role.
	// Cannot be updated.
	// +kubebuilder:default=External
	// +optional
	RoleArbitrator *RoleArbitrator `json:"roleArbitrator,omitempty"`

	// LifecycleActions defines the operational actions that needed to interoperate with the component
	// service and processes for lifecycle management.
	// Cannot be updated.
	// +optional
	LifecycleActions *ComponentLifecycleActions `json:"lifecycleActions,omitempty"`

	// ServiceRefDeclarations is used to declare the service reference of the current component.
	// Cannot be updated.
	// +optional
	ServiceRefDeclarations []ServiceRefDeclaration `json:"serviceRefDeclarations,omitempty"`
}

// ComponentDefinitionStatus defines the observed state of ComponentDefinition.
type ComponentDefinitionStatus struct {
	// ObservedGeneration is the most recent generation observed for this ComponentDefinition.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase valid values are ``, `Available`, 'Unavailable`.
	// Available is ComponentDefinition become available, and can be used for co-related objects.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Extra message for current phase.
	// +optional
	Message string `json:"message,omitempty"`
}

type ComponentVolume struct {
	// The Name of the volume.
	// Must be a DNS_LABEL and unique within the pod.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	// Cannot be updated.
	// +required
	Name string `json:"name"`

	// NeedSnapshot indicates whether the volume need to snapshot when making a backup for the component.
	// Cannot be updated.
	// +kubebuilder:default=false
	// +optional
	NeedSnapshot bool `json:"needSnapshot,omitempty"`

	// HighWatermark defines the high watermark threshold for the volume space usage.
	// If there is any specified volumes who's space usage is over the threshold, the pre-defined "LOCK" action
	// will be triggered to degrade the service to protect volume from space exhaustion, such as to set the instance
	// as read-only. And after that, if all volumes' space usage drops under the threshold later, the pre-defined
	// "UNLOCK" action will be performed to recover the service normally.
	// Cannot be updated.
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	HighWatermark int `json:"highWatermark,omitempty"`
}

// ReplicasLimit defines the limit of valid replicas supported.
// +kubebuilder:validation:XValidation:rule="self.minReplicas >= 0 && self.maxReplicas <= 128",message="the minimum and maximum limit of replicas should be in the range of [0, 128]"
// +kubebuilder:validation:XValidation:rule="self.minReplicas <= self.maxReplicas",message="the minimum replicas limit should be no greater than the maximum"
type ReplicasLimit struct {
	// The minimum limit of replicas.
	// +required
	MinReplicas int32 `json:"minReplicas"`

	// The maximum limit of replicas.
	// +required
	MaxReplicas int32 `json:"maxReplicas"`
}

type SystemAccount struct {
	// The name of the account.
	// Others can refer to this account by the name.
	// Cannot be updated.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// InitAccount indicates whether this is the unique system initialization account (e.g., MySQL root).
	// Only one system init account is allowed.
	// Cannot be updated.
	// +kubebuilder:default=false
	// +optional
	InitAccount bool `json:"initAccount,omitempty"`

	// Statement specifies the statement used to create the account with required privileges.
	// Cannot be updated.
	// +optional
	Statement string `json:"statement,omitempty"`

	// PasswordGenerationPolicy defines the policy for generating the account's password.
	// Cannot be updated.
	// +optional
	PasswordGenerationPolicy PasswordConfig `json:"passwordGenerationPolicy"`

	// SecretRef specifies the secret from which data will be copied to create the new account.
	// Cannot be updated.
	// +optional
	SecretRef *ProvisionSecretRef `json:"secretRef,omitempty"`
}

// RoleArbitrator defines how to arbitrate the role of replicas.
// +enum
// +kubebuilder:validation:Enum={External,Lorry}
type RoleArbitrator string

const (
	ExternalRoleArbitrator RoleArbitrator = "External"
	LorryRoleArbitrator    RoleArbitrator = "Lorry"
)

// ReplicaRole represents a role that can be assumed by a component instance.
type ReplicaRole struct {
	// Name of the role. It will apply to "apps.kubeblocks.io/role" object label value.
	// Cannot be updated.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Pattern=`^.*[^\s]+.*$`
	Name string `json:"name"`

	// Serviceable indicates whether a replica with this role can provide services.
	// Cannot be updated.
	// +kubebuilder:default=false
	// +optional
	Serviceable bool `json:"serviceable,omitempty"`

	// Writable indicates whether a replica with this role is allowed to write data.
	// Cannot be updated.
	// +kubebuilder:default=false
	// +optional
	Writable bool `json:"writable,omitempty"`

	// Votable indicates whether a replica with this role is allowed to vote.
	// Cannot be updated.
	// +kubebuilder:default=false
	// +optional
	Votable bool `json:"votable,omitempty"`
}

// TargetPodSelector defines how to select pod(s) to execute a action.
// +enum
// +kubebuilder:validation:Enum={Any,All,Role,Ordinal}
type TargetPodSelector string

const (
	AnyReplica      TargetPodSelector = "Any"
	AllReplicas     TargetPodSelector = "All"
	RoleSelector    TargetPodSelector = "Role"
	OrdinalSelector TargetPodSelector = "Ordinal"
)

// HTTPAction describes an action based on HTTP requests.
type HTTPAction struct {
	// Path to access on the HTTP server.
	// +optional
	Path string `json:"path,omitempty"`

	// Name or number of the port to access on the container.
	// Number must be in the range 1 to 65535.
	// Name must be an IANA_SVC_NAME.
	Port intstr.IntOrString `json:"port"`

	// Host name to connect to, defaults to the pod IP. You probably want to set
	// "Host" in httpHeaders instead.
	// +optional
	Host string `json:"host,omitempty"`

	// Scheme to use for connecting to the host.
	// Defaults to HTTP.
	// +optional
	Scheme corev1.URIScheme `json:"scheme,omitempty"`

	// Method represents the HTTP request method, which can be one of the standard HTTP methods like "GET," "POST," "PUT," etc.
	// Defaults to Get.
	// +optional
	Method string `json:"method,omitempty"`

	// Custom headers to set in the request. HTTP allows repeated headers.
	// +optional
	HTTPHeaders []corev1.HTTPHeader `json:"httpHeaders,omitempty"`
}

type ExecAction struct {
	// Command is the command line to execute inside the container, the working directory for the
	// command  is root ('/') in the container's filesystem. The command is simply exec'd, it is
	// not run inside a shell, so traditional shell instructions ('|', etc) won't work. To use
	// a shell, you need to explicitly call out to that shell.
	// Exit status of 0 is treated as live/healthy and non-zero is unhealthy.
	// +optional
	Command []string `json:"command,omitempty" protobuf:"bytes,1,rep,name=command"`

	// args is used to perform statements.
	// +optional
	Args []string `json:"args,omitempty"`
}

type RetryPolicy struct {
	// MaxRetries specifies the maximum number of times the action should be retried.
	// +kubebuilder:default=0
	// +optional
	MaxRetries int `json:"maxRetries,omitempty"`

	// RetryInterval specifies the interval between retry attempts.
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

// Action defines an operational action that can be performed by a component instance.
// There are some pre-defined environment variables that can be used when writing action commands, check @BuiltInVars for reference.
//
// An action is considered successful if it returns 0 (or HTTP 200 for HTTP(s) actions). Any other return value or
// HTTP status code is considered as a failure, and the action may be retried based on the configured retry policy.
//
// If an action exceeds the specified timeout duration, it will be terminated, and the action is considered failed.
// If an action produces any data as output, it should be written to stdout (or included in the HTTP response payload for HTTP(s) actions).
// If an action encounters any errors, error messages should be written to stderr (or included in the HTTP response payload with a non-200 HTTP status code).
type Action struct {
	// Image defines the container image to run the action.
	// Cannot be updated.
	// +optional
	Image string `json:"image,omitempty"`

	// Exec specifies the action to take.
	// Cannot be updated.
	// +optional
	Exec *ExecAction `json:"exec,omitempty"`

	// HTTP specifies the http request to perform.
	// Cannot be updated.
	// +optional
	HTTP *HTTPAction `json:"http,omitempty"`

	// List of environment variables to set in the container.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// TargetPodSelector defines the way that how to select the target Pod where the action will be performed,
	// if there may not have a target replica by default.
	// Cannot be updated.
	// +optional
	TargetPodSelector TargetPodSelector `json:"targetPodSelector,omitempty"`

	// MatchingKey uses to select the target pod(s) actually.
	// If the selector is AnyReplica or AllReplicas, the matchingKey will be ignored.
	// If the selector is RoleSelector, any replica which has the same role with matchingKey will be chosen.
	// Cannot be updated.
	// +optional
	MatchingKey string `json:"matchingKey,omitempty"`

	// Container defines the name of the container within the target Pod where the action will be executed.
	// If specified, it must be one of container declared in @Runtime.
	// If not specified, the first container declared in @Runtime will be used.
	// Cannot be updated.
	// +optional
	Container string `json:"container,omitempty"`

	// TimeoutSeconds defines the timeout duration for the action in seconds.
	// Cannot be updated.
	// +kubebuilder:default=0
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds:omitempty"`

	// RetryPolicy defines the strategy for retrying the action in case of failure.
	// Cannot be updated.
	// +optional
	RetryPolicy *RetryPolicy `json:"retryPolicy,omitempty"`

	// PreCondition defines the condition when the action will be executed.
	// - Immediately: The Action is executed immediately after the Component object is created,
	// without guaranteeing the availability of the Component and its underlying resources. only after the action is successfully executed will the Component's state turn to ready.
	// - RuntimeReady: The Action is executed after the Component object is created and once all underlying Runtimes are ready.
	// only after the action is successfully executed will the Component's state turn to ready.
	// - ComponentReady: The Action is executed after the Component object is created and once the Component is ready.
	// the execution process does not impact the state of the Component and the Cluster.
	// - ClusterReady: The Action is executed after the Cluster object is created and once the Cluster is ready.
	// the execution process does not impact the state of the Component and the Cluster.
	// Cannot be updated.
	// +optional
	PreCondition *PreConditionType `json:"preCondition,omitempty"`
}

// BuiltinActionHandlerType defines build-in action handlers provided by Lorry.
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
	UnknownBuiltinActionHandler            BuiltinActionHandlerType = "unknown"
)

type LifecycleActionHandler struct {
	// builtinHandler specifies the builtin action handler name to do the action.
	// the BuiltinHandler within the same ComponentLifecycleActions should be consistent. Details can be queried through official documentation in the future.
	// use CustomHandler to define your own actions if none of them satisfies the requirement.
	// +optional
	BuiltinHandler *BuiltinActionHandlerType `json:"builtinHandler,omitempty"`

	// customHandler defines the custom way to do action.
	// +optional
	CustomHandler *Action `json:"customHandler,omitempty"`
}

// ComponentLifecycleActions defines a set of operational actions for interacting with component services and processes.
type ComponentLifecycleActions struct {
	// PostProvision defines the actions to be executed and the corresponding policy when a component is created.
	// You can define the preCondition for executing PostProvision using Action.PreCondition. The default PostProvision action preCondition is ComponentReady.
	// The PostProvision Action will be executed only once.
	// Dedicated env vars for the action:
	// - KB_CLUSTER_COMPONENT_LIST: The list of all components in the cluster, joined by ',' (e.g., "comp1,comp2").
	// - KB_CLUSTER_COMPONENT_POD_NAME_LIST: The list of all pods name in this component, joined by ',' (e.g., "pod1,pod2").
	// - KB_CLUSTER_COMPONENT_POD_IP_LIST: The list of pod IPs where each pod resides in this component, corresponding one-to-one with each pod in the KB_CLUSTER_COMPONENT_POD_NAME_LIST. joined by ',' (e.g., "podIp1,podIp2").
	// - KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST: The list of hostName where each pod resides in this component, corresponding one-to-one with each pod in the KB_CLUSTER_COMPONENT_POD_NAME_LIST. joined by ',' (e.g., "hostName1,hostName2").
	// - KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST: The list of host IPs where each pod resides in this component, corresponding one-to-one with each pod in the KB_CLUSTER_COMPONENT_POD_NAME_LIST. joined by ',' (e.g., "hostIp1,hostIp2").
	// Cannot be updated.
	// +optional
	PostProvision *LifecycleActionHandler `json:"postProvision,omitempty"`

	// PreTerminate defines the actions to be executed when a component is terminated due to an API request.
	// The PreTerminate Action will be executed only once. Upon receiving a scale-down command for the Component, it is executed immediately.
	// Only after the preTerminate action is successfully executed, the destruction of the Component and its underlying resources proceeds.
	// Cannot be updated.
	// +optional
	PreTerminate *LifecycleActionHandler `json:"preTerminate,omitempty"`

	// RoleProbe defines how to probe the role of replicas.
	// Cannot be updated.
	// +optional
	RoleProbe *RoleProbe `json:"roleProbe,omitempty"`

	// Switchover defines how to proactively switch the current leader to a new replica to minimize the impact on availability.
	// This action is typically invoked when the leader is about to become unavailable due to events, such as:
	// - switchover
	// - stop
	// - restart
	// - scale-in
	// Dedicated env vars for the action:
	// - KB_SWITCHOVER_CANDIDATE_NAME: The name of the new candidate replica's Pod. It may be empty.
	// - KB_SWITCHOVER_CANDIDATE_FQDN: The FQDN of the new candidate replica. It may be empty.
	// - KB_LEADER_POD_IP: The IP address of the original leader's Pod before switchover.
	// - KB_LEADER_POD_NAME: The name of the original leader's Pod before switchover.
	// - KB_LEADER_POD_FQDN: The FQDN of the original leader's Pod before switchover.
	// The env vars with following prefix are deprecated and will be removed in the future:
	// - KB_REPLICATION_PRIMARY_POD_: The prefix of the environment variables of the original primary's Pod before switchover.
	// - KB_CONSENSUS_LEADER_POD_: The prefix of the environment variables of the original leader's Pod before switchover.
	// Cannot be updated.
	// +optional
	Switchover *ComponentSwitchover `json:"switchover,omitempty"`

	// MemberJoin defines how to add a new replica to the replication group.
	// This action is typically invoked when a new replica needs to be added, such as during scale-out.
	// It may involve updating configuration, notifying other members, and ensuring data consistency.
	// Cannot be updated.
	// +optional
	MemberJoin *LifecycleActionHandler `json:"memberJoin,omitempty"`

	// MemberLeave defines how to remove a replica from the replication group.
	// This action is typically invoked when a replica needs to be removed, such as during scale-in.
	// It may involve configuration updates and notifying other members about the departure,
	// but it is advisable to avoid performing data migration within this action.
	// Cannot be updated.
	// +optional
	MemberLeave *LifecycleActionHandler `json:"memberLeave,omitempty"`

	// Readonly defines how to set a replica service as read-only.
	// This action is used to protect a replica in case of volume space exhaustion or excessive traffic.
	// Cannot be updated.
	// +optional
	Readonly *LifecycleActionHandler `json:"readonly,omitempty"`

	// Readwrite defines how to set a replica service as read-write.
	// Cannot be updated.
	// +optional
	Readwrite *LifecycleActionHandler `json:"readwrite,omitempty"`

	// DataPopulate defines how to populate the data to create new replicas.
	// This action is typically used when a new replica needs to be constructed, such as:
	// - scale-out
	// - rebuild
	// - clone
	// It should write the valid data to stdout without including any extraneous information.
	// Cannot be updated.
	// +optional
	DataPopulate *LifecycleActionHandler `json:"dataPopulate,omitempty"`

	// DataAssemble defines how to assemble data synchronized from external before starting the service for a new replica.
	// This action is typically used when creating a new replica, such as:
	//  - scale-out
	//  - rebuild
	//  - clone
	// The data will be streamed in via stdin. If any error occurs during the assembly process,
	// the action must be able to guarantee idempotence to allow for retries from the beginning.
	// Cannot be updated.
	// +optional
	DataAssemble *LifecycleActionHandler `json:"dataAssemble,omitempty"`

	// Reconfigure defines how to notify the replica service that there is a configuration update.
	// Cannot be updated.
	// +optional
	Reconfigure *LifecycleActionHandler `json:"reconfigure,omitempty"`

	// AccountProvision defines how to provision accounts.
	// Cannot be updated.
	// +optional
	AccountProvision *LifecycleActionHandler `json:"accountProvision,omitempty"`
}

type ComponentSwitchover struct {
	// withCandidate corresponds to the switchover of the specified candidate primary or leader instance.
	// Currently, only Action.Exec is supported, Action.HTTP is not supported.
	// +optional
	WithCandidate *Action `json:"withCandidate,omitempty"`

	// withoutCandidate corresponds to a switchover that does not specify a candidate primary or leader instance.
	// Currently, only Action.Exec is supported, Action.HTTP is not supported.
	// +optional
	WithoutCandidate *Action `json:"withoutCandidate,omitempty"`

	// scriptSpecSelectors defines the selector of the scriptSpecs that need to be referenced.
	// Once ScriptSpecSelectors is defined, the scripts defined in scripts can be referenced in the Action.
	// +optional
	ScriptSpecSelectors []ScriptSpecSelector `json:"scriptSpecSelectors,omitempty"`
}

type RoleProbe struct {
	LifecycleActionHandler `json:",inline"`

	// Number of seconds after the container has started before liveness probes are initiated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty" protobuf:"varint,2,opt,name=initialDelaySeconds"`
	// Number of seconds after which the probe times out.
	// Defaults to 1 second. Minimum value is 1.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty" protobuf:"varint,3,opt,name=timeoutSeconds"`
	// How often (in seconds) to perform the probe.
	// Default to 10 seconds. Minimum value is 1.
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty" protobuf:"varint,4,opt,name=periodSeconds"`
	// Minimum consecutive successes for the probe to be considered successful after having failed.
	// Defaults to 1. Must be 1 for liveness and startup. Minimum value is 1.
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty" protobuf:"varint,5,opt,name=successThreshold"`
	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	// Defaults to 3. Minimum value is 1.
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty" protobuf:"varint,6,opt,name=failureThreshold"`
	// Optional duration in seconds the pod needs to terminate gracefully upon probe failure.
	// The grace period is the duration in seconds after the processes running in the pod are sent
	// a termination signal and the time when the processes are forcibly halted with a kill signal.
	// Set this value longer than the expected cleanup time for your process.
	// If this value is nil, the pod's terminationGracePeriodSeconds will be used. Otherwise, this
	// value overrides the value provided by the pod spec.
	// Value must be non-negative integer. The value zero indicates stop immediately via
	// the kill signal (no opportunity to shut down).
	// This is a beta field and requires enabling ProbeTerminationGracePeriod feature gate.
	// Minimum value is 1. spec.terminationGracePeriodSeconds is used if unset.
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,7,opt,name=terminationGracePeriodSeconds"`
}
