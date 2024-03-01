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

// ComponentDefinitionSpec provides a workload component specification with attributes that strongly work with stateful workloads and day-2 operation behaviors.
type ComponentDefinitionSpec struct {
	// Specifies the name of the component provider.
	//
	// +kubebuilder:validation:MaxLength=32
	// +optional
	Provider string `json:"provider,omitempty"`

	// Provides a brief description of the component.
	//
	// +kubebuilder:validation:MaxLength=256
	// +optional
	Description string `json:"description,omitempty"`

	// Defines the type of well-known service that the component provides (e.g., MySQL, Redis, ETCD, case insensitive).
	// This field is immutable.
	//
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceKind string `json:"serviceKind,omitempty"`

	// Specifies the version of the well-known service that the component provides.
	// This field is immutable.
	//
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceVersion string `json:"serviceVersion,omitempty"`

	// Primarily defines runtime information for the component, including:
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
	// CPU and memory resource limits, as well as scheduling settings (affinity, toleration, priority), should not be configured within this structure.
	// This field is immutable.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Required
	Runtime corev1.PodSpec `json:"runtime"`

	// Represents user-defined variables.
	//
	// These variables can be utilized as environment variables for Pods and Actions, or to render the templates of config and script.
	// When used as environment variables, these variables are placed in front of the environment variables declared in the Pod.
	// This field is immutable.
	//
	// +optional
	Vars []EnvVar `json:"vars,omitempty"`

	// Defines the persistent volumes needed by the component.
	// Users are responsible for providing these volumes when creating a component instance.
	// This field is immutable.
	//
	// +optional
	Volumes []ComponentVolume `json:"volumes"`

	// Defines the host-network capability and resources.
	//
	// +optional
	HostNetwork *HostNetwork `json:"hostNetwork,omitempty"`

	// Defines endpoints that can be used to access the component service to manage the component.
	//
	// In addition, a reserved headless service will be created by default, with the name pattern `{clusterName}-{componentName}-headless`.
	// This field is immutable.
	//
	// +optional
	Services []ComponentService `json:"services,omitempty"`

	// The configs field is provided by the provider, and
	// finally, these configTemplateRefs will be rendered into the user's own configuration file according to the user's cluster.
	// This field is immutable.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	// TODO: support referencing configs from other components or clusters.
	Configs []ComponentConfigSpec `json:"configs,omitempty"`

	// LogConfigs is a detailed log file config provided by the provider.
	// This field is immutable.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	LogConfigs []LogConfig `json:"logConfigs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Monitor is a monitoring config provided by the provider.
	// This field is immutable.
	//
	// +optional
	Monitor *MonitorConfig `json:"monitor,omitempty"`

	// The scripts field is provided by the provider, and
	// finally, these configTemplateRefs will be rendered into the user's own configuration file according to the user's cluster.
	// This field is immutable.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	Scripts []ComponentTemplateSpec `json:"scripts,omitempty"`

	// Defines the namespaced policy rules required by the component.
	// If any rule application fails (e.g., due to lack of permissions), the provisioning of the component instance will also fail.
	// This field is immutable.
	//
	// +optional
	PolicyRules []rbacv1.PolicyRule `json:"policyRules,omitempty"`

	// Defines static labels that will be patched to all k8s resources created for the component.
	// If a label key conflicts with any other system labels or user-specified labels, it will be silently ignored.
	// This field is immutable.
	//
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Defines static annotations that will be patched to all k8s resources created for the component.
	// If a annotation key conflicts with any other system annotations or user-specified annotations, it will be silently ignored.
	// This field is immutable.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Defines the limit of valid replicas supported.
	// This field is immutable.
	//
	// +optional
	ReplicasLimit *ReplicasLimit `json:"replicasLimit,omitempty"`

	// Defines the pre-defined system accounts required to manage the component.
	// TODO(component): accounts KB required
	// This field is immutable.
	//
	// +optional
	SystemAccounts []SystemAccount `json:"systemAccounts,omitempty"`

	// Defines the strategy for updating the component instance.
	// This field is immutable.
	//
	// +kubebuilder:default=Serial
	// +optional
	UpdateStrategy *UpdateStrategy `json:"updateStrategy,omitempty"`

	// Defines all the roles that the component can assume.
	// This field is immutable.
	//
	// +optional
	Roles []ReplicaRole `json:"roles,omitempty"`

	// Defines the strategy for electing the component's active role.
	// This field is immutable.
	//
	// +kubebuilder:default=External
	// +optional
	RoleArbitrator *RoleArbitrator `json:"roleArbitrator,omitempty"`

	// Defines the operational actions needed to interoperate with the component
	// service and processes for lifecycle management.
	// This field is immutable.
	//
	// +optional
	LifecycleActions *ComponentLifecycleActions `json:"lifecycleActions,omitempty"`

	// Used to declare the service reference of the current component.
	// This field is immutable.
	//
	// +optional
	ServiceRefDeclarations []ServiceRefDeclaration `json:"serviceRefDeclarations,omitempty"`

	// Specifies the minimum number of seconds for which a newly created pod should be ready
	// without any of its container crashing for it to be considered available.
	// Defaults to 0 (pod will be considered available as soon as it is ready)
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

	// Indicates whether a snapshot is required when creating a backup for the component.
	// Note: This field cannot be updated.
	//
	// +kubebuilder:default=false
	// +optional
	NeedSnapshot bool `json:"needSnapshot,omitempty"`

	// Defines the high watermark threshold for the volume space usage.
	//
	// If the space usage of any specified volume exceeds this threshold, a pre-defined "LOCK" action
	// will be triggered. This action degrades the service to protect the volume from space exhaustion,
	// for example, by setting the instance to read-only.
	//
	// If the space usage of all volumes drops below the threshold, a pre-defined "UNLOCK" action
	// will be performed to restore the service to normal operation.
	// Note: This field cannot be updated.
	//
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
	// This field is immutable once set.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Indicates if this account is the unique system initialization account (e.g., MySQL root).
	// Only one system initialization account is permitted.
	// This field is immutable once set.
	//
	// +kubebuilder:default=false
	// +optional
	InitAccount bool `json:"initAccount,omitempty"`

	// Defines the statement used to create the account with the necessary privileges.
	// This field is immutable once set.
	//
	// +optional
	Statement string `json:"statement,omitempty"`

	// Specifies the policy for generating the account's password.
	// This field is immutable once set.
	//
	// +optional
	PasswordGenerationPolicy PasswordConfig `json:"passwordGenerationPolicy"`

	// Refers to the secret from which data will be copied to create the new account.
	// This field is immutable once set.
	//
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
	// Defines the role's identifier. This will be applied to the "apps.kubeblocks.io/role" object label value.
	// This field is immutable once set.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Pattern=`^.*[^\s]+.*$`
	Name string `json:"name"`

	// Specifies if a replica assuming this role can provide services.
	// This field is immutable once set.
	//
	// +kubebuilder:default=false
	// +optional
	Serviceable bool `json:"serviceable,omitempty"`

	// Specifies if a replica assuming this role is permitted to write data.
	// This field is immutable once set.
	//
	// +kubebuilder:default=false
	// +optional
	Writable bool `json:"writable,omitempty"`

	// Specifies if a replica assuming this role is permitted to vote.
	// This field is immutable once set.
	//
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
	// Specifies the path to be accessed on the HTTP server.
	//
	// +optional
	Path string `json:"path,omitempty"`

	// Defines the name or number of the port to be accessed on the container.
	// The number must fall within the range of 1 to 65535.
	// The name must conform to the IANA_SVC_NAME standard.
	Port intstr.IntOrString `json:"port"`

	// Indicates the host name to connect to, which defaults to the pod IP.
	// It is recommended to set "Host" in httpHeaders instead.
	//
	// +optional
	Host string `json:"host,omitempty"`

	// Specifies the scheme to be used for connecting to the host.
	// The default scheme is HTTP.
	//
	// +optional
	Scheme corev1.URIScheme `json:"scheme,omitempty"`

	// Represents the HTTP request method, which can be one of the standard HTTP methods such as "GET," "POST," "PUT," etc.
	// The default method is Get.
	//
	// +optional
	Method string `json:"method,omitempty"`

	// Allows for the setting of custom headers in the request.
	// HTTP supports repeated headers.
	//
	// +optional
	HTTPHeaders []corev1.HTTPHeader `json:"httpHeaders,omitempty"`
}

type ExecAction struct {
	// Specifies the command line to be executed inside the container. The working directory for this command
	// is the root ('/') of the container's filesystem. The command is directly executed and not run inside a shell,
	// hence traditional shell instructions ('|', etc) are not applicable. To use a shell, it needs to be explicitly invoked.
	//
	// An exit status of 0 is interpreted as live/healthy, while a non-zero status indicates unhealthy.
	//
	// +optional
	Command []string `json:"command,omitempty" protobuf:"bytes,1,rep,name=command"`

	// Args are used to perform statements.
	//
	// +optional
	Args []string `json:"args,omitempty"`
}

type RetryPolicy struct {
	// Defines the maximum number of retry attempts that should be made for a given action.
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

// Action defines an operational action that can be performed by a component instance.
// There are some pre-defined environment variables that can be used when writing action commands, check @BuiltInVars for reference.
//
// An action is considered successful if it returns 0 (or HTTP 200 for HTTP(s) actions). Any other return value or
// HTTP status code is considered as a failure, and the action may be retried based on the configured retry policy.
//
// - If an action exceeds the specified timeout duration, it will be terminated, and the action is considered failed.
// - If an action produces any data as output, it should be written to stdout (or included in the HTTP response payload for HTTP(s) actions).
// - If an action encounters any errors, error messages should be written to stderr (or included in the HTTP response payload with a non-200 HTTP status code).
type Action struct {
	// Specifies the container image to run the action.
	// This field cannot be updated.
	//
	// +optional
	Image string `json:"image,omitempty"`

	// Defines the action to take.
	// This field cannot be updated.
	//
	// +optional
	Exec *ExecAction `json:"exec,omitempty"`

	// Specifies the HTTP request to perform.
	// This field cannot be updated.
	//
	// +optional
	HTTP *HTTPAction `json:"http,omitempty"`

	// Represents a list of environment variables to set in the container.
	// This field cannot be updated.
	//
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// Defines how to select the target Pod where the action will be performed,
	// if there may not have a target replica by default.
	// This field cannot be updated.
	//
	// +optional
	TargetPodSelector TargetPodSelector `json:"targetPodSelector,omitempty"`

	// Used to select the target pod(s) actually.
	// If the selector is AnyReplica or AllReplicas, this field will be ignored.
	// If the selector is RoleSelector, any replica which has the same role with this field will be chosen.
	// This field cannot be updated.
	//
	// +optional
	MatchingKey string `json:"matchingKey,omitempty"`

	// Defines the name of the container within the target Pod where the action will be executed.
	// If specified, it must be one of container declared in @Runtime.
	// If not specified, the first container declared in @Runtime will be used.
	// This field cannot be updated.
	//
	// +optional
	Container string `json:"container,omitempty"`

	// Defines the timeout duration for the action in seconds.
	// This field cannot be updated.
	//
	// +kubebuilder:default=0
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`

	// Defines the strategy for retrying the action in case of failure.
	// This field cannot be updated.
	//
	// +optional
	RetryPolicy *RetryPolicy `json:"retryPolicy,omitempty"`

	// Defines the condition when the action will be executed.
	//
	// - Immediately: The Action is executed immediately after the Component object is created,
	//   without guaranteeing the availability of the Component and its underlying resources. Only after the action is successfully executed will the Component's state turn to ready.
	// - RuntimeReady: The Action is executed after the Component object is created and once all underlying Runtimes are ready.
	//   Only after the action is successfully executed will the Component's state turn to ready.
	// - ComponentReady: The Action is executed after the Component object is created and once the Component is ready.
	//   The execution process does not impact the state of the Component and the Cluster.
	// - ClusterReady: The Action is executed after the Cluster object is created and once the Cluster is ready.
	//
	// The execution process does not impact the state of the Component and the Cluster.
	// This field cannot be updated.
	//
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
	CustomActionHandler                    BuiltinActionHandlerType = "custom"
	UnknownBuiltinActionHandler            BuiltinActionHandlerType = "unknown"
)

type LifecycleActionHandler struct {
	// BuiltinHandler specifies the builtin action handler name to do the action.
	// the BuiltinHandler within the same ComponentLifecycleActions should be consistent. Details can be queried through official documentation in the future.
	// use CustomHandler to define your own actions if none of them satisfies the requirement.
	//
	// +optional
	BuiltinHandler *BuiltinActionHandlerType `json:"builtinHandler,omitempty"`

	// CustomHandler defines the custom way to do action.
	//
	// +optional
	CustomHandler *Action `json:"customHandler,omitempty"`
}

// ComponentLifecycleActions defines a set of operational actions for interacting with component services and processes.
type ComponentLifecycleActions struct {
	// Specifies the actions and corresponding policy to be executed when a component is created.
	// The precondition for executing PostProvision can be defined using Action.PreCondition. The default precondition for PostProvision action is ComponentReady.
	// The PostProvision Action will be executed only once.
	// The following dedicated environment variables are available for the action:
	//
	// - KB_CLUSTER_COMPONENT_LIST: Lists all components in the cluster, joined by ',' (e.g., "comp1,comp2").
	// - KB_CLUSTER_COMPONENT_POD_NAME_LIST: Lists all pod names in this component, joined by ',' (e.g., "pod1,pod2").
	// - KB_CLUSTER_COMPONENT_POD_IP_LIST: Lists the IP addresses of each pod in this component, corresponding one-to-one with each pod in the KB_CLUSTER_COMPONENT_POD_NAME_LIST. Joined by ',' (e.g., "podIp1,podIp2").
	// - KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST: Lists the host names where each pod resides in this component, corresponding one-to-one with each pod in the KB_CLUSTER_COMPONENT_POD_NAME_LIST. Joined by ',' (e.g., "hostName1,hostName2").
	// - KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST: Lists the host IP addresses where each pod resides in this component, corresponding one-to-one with each pod in the KB_CLUSTER_COMPONENT_POD_NAME_LIST. Joined by ',' (e.g., "hostIp1,hostIp2").
	//
	// This field cannot be updated.
	//
	// +optional
	PostProvision *LifecycleActionHandler `json:"postProvision,omitempty"`

	// Defines the actions to be executed when a component is terminated due to an API request.
	// The PreTerminate Action will be executed only once. Upon receiving a scale-down command for the Component, it is executed immediately.
	// The destruction of the Component and its underlying resources proceeds only after the preTerminate action is successfully executed.
	// This field cannot be updated.
	//
	// +optional
	PreTerminate *LifecycleActionHandler `json:"preTerminate,omitempty"`

	// RoleProbe defines the mechanism to probe the role of replicas periodically. The specified action will be
	// executed by Lorry at the configured interval. If the execution is successful, the output will be used as
	// the replica's assigned role, and the role must be one of the names defined in the ComponentDefinition roles.
	// The output will be compared with the last successful result.  If there is a change, a role change event will
	// be created to notify the controller and trigger updating the replica's role.
	// Defining a RoleProbe is required if roles are configured for the component. Otherwise, the replicas' pods will
	// lack role information after the cluster is created, and services will not route to the replica correctly.
	//
	// The following dedicated environment variables are available for the action:
	//
	// - KB_POD_FQDN: The pod FQDN of the replica to check the role.
	// - KB_SERVICE_PORT: The port on which the DB service listens.
	// - KB_SERVICE_USER: The username used to access the DB service and retrieve the role information with sufficient privileges.
	// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service and retrieve the role information.
	//
	// Output of the action:
	// - ROLE: the role of the replica. It must be one of the names defined in the roles.
	// - ERROR: Any error message if the action fails.
	//
	// This field cannot be updated.
	//
	// +optional
	RoleProbe *RoleProbe `json:"roleProbe,omitempty"`

	// Defines the method to proactively switch the current leader to a new replica to minimize the impact on availability.
	// This action is typically invoked when the leader is about to become unavailable due to events, such as:
	//
	// - switchover
	// - stop
	// - restart
	// - scale-in
	//
	// The following dedicated environment variables are available for the action:
	//
	// - KB_SWITCHOVER_CANDIDATE_NAME: The name of the new candidate replica's Pod. It may be empty.
	// - KB_SWITCHOVER_CANDIDATE_FQDN: The FQDN of the new candidate replica. It may be empty.
	// - KB_LEADER_POD_IP: The IP address of the original leader's Pod before switchover.
	// - KB_LEADER_POD_NAME: The name of the original leader's Pod before switchover.
	// - KB_LEADER_POD_FQDN: The FQDN of the original leader's Pod before switchover.
	//
	// The environment variables with the following prefixes are deprecated and will be removed in the future:
	//
	// - KB_REPLICATION_PRIMARY_POD_: The prefix of the environment variables of the original primary's Pod before switchover.
	// - KB_CONSENSUS_LEADER_POD_: The prefix of the environment variables of the original leader's Pod before switchover.
	//
	// This field cannot be updated.
	//
	// +optional
	Switchover *ComponentSwitchover `json:"switchover,omitempty"`

	// Defines the method to add a new replica to the replication group.
	// This action is typically invoked when a new replica needs to be added, such as during scale-out.
	// It may involve updating configuration, notifying other members, and ensuring data consistency.
	//
	// The following dedicated environment variables are available for the action:
	//
	// - KB_SERVICE_PORT: The port on which the DB service listens.
	// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
	// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service .
	// - KB_PRIMARY_POD_FQDN: The FQDN of the original primary Pod before switchover.
	// - KB_NEW_MEMBER_POD_NAME: The name of the new member's Pod.
	//
	// Output of the action:
	// - ERROR: Any error message if the action fails.
	//
	// This field cannot be updated.
	//
	// +optional
	MemberJoin *LifecycleActionHandler `json:"memberJoin,omitempty"`

	// Defines the method to remove a replica from the replication group.
	// This action is typically invoked when a replica needs to be removed, such as during scale-in.
	// It may involve configuration updates and notifying other members about the departure,
	// but it is advisable to avoid performing data migration within this action.
	//
	// The following dedicated environment variables are available for the action:
	//
	// - KB_SERVICE_PORT: The port on which the DB service listens.
	// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
	// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service.
	// - KB_PRIMARY_POD_FQDN: The FQDN of the original primary Pod before switchover.
	// - KB_LEAVE_MEMBER_POD_NAME: The name of the leave member's Pod.
	//
	// Output of the action:
	// - ERROR: Any error message if the action fails.
	//
	// This field cannot be updated.
	//
	// +optional
	MemberLeave *LifecycleActionHandler `json:"memberLeave,omitempty"`

	// Defines the method to set a replica service as read-only.
	// This action is used to protect a replica in case of volume space exhaustion or excessive traffic.
	//
	// The following dedicated environment variables are available for the action:
	//
	// - KB_POD_FQDN: The FQDN of the replica pod to check the role.
	// - KB_SERVICE_PORT: The port on which the DB service listens.
	// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
	// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service.
	//
	// Output of the action:
	// - ERROR: Any error message if the action fails.
	//
	// This field cannot be updated.
	//
	// +optional
	Readonly *LifecycleActionHandler `json:"readonly,omitempty"`

	// Readwrite defines how to set a replica service as read-write.
	//
	// The following dedicated environment variables are available for the action:
	//
	// - KB_POD_FQDN: The FQDN of the replica pod to check the role.
	// - KB_SERVICE_PORT: The port on which the DB service listens.
	// - KB_SERVICE_USER: The username used to access the DB service with sufficient privileges.
	// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service.
	//
	// Output of the action:
	// - ERROR: Any error message if the action fails.
	//
	// This field cannot be updated.
	//
	// +optional
	Readwrite *LifecycleActionHandler `json:"readwrite,omitempty"`

	// Defines the method to populate the data to create new replicas.
	// This action is typically used when a new replica needs to be constructed, such as:
	//
	// - scale-out
	// - rebuild
	// - clone
	//
	// It should write the valid data to stdout without including any extraneous information.
	// This field cannot be updated.
	//
	// +optional
	DataPopulate *LifecycleActionHandler `json:"dataPopulate,omitempty"`

	// Defines the method to assemble data synchronized from external before starting the service for a new replica.
	// This action is typically used when creating a new replica, such as:
	//
	// - scale-out
	// - rebuild
	// - clone
	//
	// The data will be streamed in via stdin. If any error occurs during the assembly process,
	// the action must be able to guarantee idempotence to allow for retries from the beginning.
	// This field cannot be updated.
	//
	// +optional
	DataAssemble *LifecycleActionHandler `json:"dataAssemble,omitempty"`

	// Defines the method to notify the replica service that there is a configuration update.
	// This field cannot be updated.
	//
	// +optional
	Reconfigure *LifecycleActionHandler `json:"reconfigure,omitempty"`

	// Defines the method to provision accounts.
	// This field cannot be updated.
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
	// When this field is defined, the scripts specified in the scripts field can be referenced in the Action.
	//
	// +kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.9.0"
	// +optional
	ScriptSpecSelectors []ScriptSpecSelector `json:"scriptSpecSelectors,omitempty"`
}

type RoleProbe struct {
	LifecycleActionHandler `json:",inline"`

	// Number of seconds after the container has started before liveness probes are initiated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	//
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty" protobuf:"varint,2,opt,name=initialDelaySeconds"`

	// Number of seconds after which the probe times out.
	// Defaults to 1 second. Minimum value is 1.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	//
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty" protobuf:"varint,3,opt,name=timeoutSeconds"`

	// How often (in seconds) to perform the probe.
	// Default to 10 seconds. Minimum value is 1.
	//
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty" protobuf:"varint,4,opt,name=periodSeconds"`
}
