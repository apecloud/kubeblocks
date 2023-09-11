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
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.version",description="component version"
// +kubebuilder:printcolumn:name="SERVICE",type="string",JSONPath=".spec.serviceKind",description="service"
// +kubebuilder:printcolumn:name="SERVICE-VERSION",type="string",JSONPath=".spec.serviceVersion",description="service version"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ComponentDefinition is the schema for the component definitions API.
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

// ComponentDefinitionSpec provides a workload component specification, with attributes that strongly work with
// stateful workloads and day-2 operations behaviors.
type ComponentDefinitionSpec struct {
	// Name of the component. It will apply to "apps.kubeblocks.io/component-name" object label value.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Version of the component.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	Version string `json:"version"`

	// Provider is the name of the component provider.
	// +kubebuilder:validation:MaxLength=32
	// +optional
	Provider string `json:"provider,omitempty"`

	// Description is a brief description of the component definition.
	// +kubebuilder:validation:MaxLength=512
	// +optional
	Description string `json:"description,omitempty"`

	// ServiceKind defines what kind of well-known service that the component provides (e.g., MySQL, Redis, ETCD, case insensitive).
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceKind string `json:"serviceKind,omitempty"`

	// ServiceVersion defines the version of the well-known service that the component provides.
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
	//       - Probes: readiness
	//       - Lifecycle
	//   - Volumes
	// CPU and memory resource limits, as well as scheduling settings (affinity, toleration, priority),
	// should not be configured within this structure.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Required
	Runtime corev1.PodSpec `json:"runtime"`

	// Volumes defines the persistent volumes needed by the component.
	// The users are responsible for providing these volumes when creating a component instance.
	// +optional
	Volumes []ComponentPersistentVolume `json:"volumes"`

	// Services defines endpoints that can be used to access the service provided by the component.
	// If specified, a headless service will be created with some attributes of Services[0] by default.
	// +optional
	Services []ComponentService `json:"services,omitempty"`

	// The configs field provided by provider, and
	// finally this configTemplateRefs will be rendered into the user's own configuration file according to the user's cluster.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Configs []ComponentConfigSpec `json:"configs,omitempty"`

	// LogConfigs is detail log file config which provided by provider.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	LogConfigs []LogConfig `json:"logConfigs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Monitor is monitoring config which provided by provider.
	// +optional
	Monitor *MonitorConfig `json:"monitor,omitempty"`

	// The scripts field provided by provider, and
	// finally this configTemplateRefs will be rendered into the user's own configuration file according to the user's cluster.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	Scripts []ComponentTemplateSpec `json:"scripts,omitempty"`

	// ConnectionCredentials defines the default connection credentials that can be used to access the component service.
	// Cannot be updated.
	// +optional
	ConnectionCredentials []ConnectionCredential `json:"connectionCredentials,omitempty"`

	// Rules defines the namespaced policy rules required by a component.
	// If any rule application fails (e.g., due to lack of permissions), the provisioning of the component instance will also fail.
	// Cannot be updated.
	// +optional
	Rules []rbacv1.PolicyRule `json:"rules,omitempty"`

	// Labels defines static labels that will be patched to all k8s resources created for the component.
	// If a label key conflicts with any other system labels or user-specified labels, it will be silently ignored.
	// Cannot be updated.
	// +optional
	Labels map[string]BuiltInString `json:"labels,omitempty"`

	// TODO: support other resources provisioning.
	// Statement to create system account.
	// Cannot be updated.
	// +optional
	SystemAccounts *SystemAccountSpec `json:"systemAccounts,omitempty"`

	// UpdateStrategy defines the strategy for updating the component instance.
	// Cannot be updated.
	// +kubebuilder:default=Serial
	// +optional
	UpdateStrategy UpdateStrategy `json:"updateStrategy,omitempty"`

	// Roles defines all the roles that the component can assume.
	// Cannot be updated.
	// +optional
	Roles []ComponentReplicaRole `json:"roles,omitempty"`

	// RoleArbitrator defines the strategy for electing the component's active role.
	// Cannot be updated.
	// +kubebuilder:default=External
	// +optional
	RoleArbitrator ComponentRoleArbitrator `json:"roleArbitrator,omitempty"`

	// LifecycleActions defines the operational actions that needed to interoperate with the component
	// service and processes for lifecycle management.
	// Cannot be updated.
	// +optional
	LifecycleActions ComponentLifecycleActions `json:"lifecycleActions,omitempty"`

	// TODO: introduce the event-based interoperability mechanism.

	// ComponentDefRef is used to inject values from other components into the current component.
	// values will be saved and updated in a configmap and mounted to the current component.
	// +patchMergeKey=componentDefName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentDefName
	// +optional
	ComponentDefRef []ComponentDefRef `json:"componentDefRef,omitempty" patchStrategy:"merge" patchMergeKey:"componentDefName"`
}

// ComponentDefinitionStatus defines the observed state of ComponentDefinition
type ComponentDefinitionStatus struct {
	// observedGeneration is the most recent generation observed for this ComponentDefinition.
	// It corresponds to the ComponentDefinition's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// phase valid values are ``, `Available`, 'Unavailable`.
	// Available is ComponentDefinition become available, and can be referenced for co-related objects.
	Phase Phase `json:"phase,omitempty"`
}

type ComponentPersistentVolume struct {
	// Name of the volume.
	// Must be a DNS_LABEL and unique within the pod.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	// +required
	Name string `json:"name"`

	// Synchronization indicates whether the data on this volume needs to be synchronized when making a backup or building a new replica.
	// +kubebuilder:default=false
	// +optional
	Synchronization bool `json:"synchronization,omitempty"`

	// The high watermark threshold for the volume space usage.
	// If there is any specified volumes who's space usage is over the threshold, the pre-defined "LOCK" action
	// will be triggered to degrade the service to protect volume from space exhaustion, such as to set the instance
	// as read-only. And after that, if all volumes' space usage drops under the threshold later, the pre-defined
	// "UNLOCK" action will be performed to recover the service normally.
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	HighWatermark int `json:"highWatermark,omitempty"`
}

type ComponentService struct {
	// The name of the component service.
	// +required
	Name string `json:"name"`

	// ServiceName defines the name of the service object.
	// If not specified, the default service name with pattern <CLUSTER_NAME>-<COMPONENT_NAME> will be used.
	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	corev1.ServiceSpec `json:",inline"`
}

type ConnectionCredential struct {
	// The name of the ConnectionCredential.
	// +required
	Name string `json:"name"`

	// ServiceName specifies the service spec to use for accessing the component service.
	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	// PortName specifies the name of the port to access the component service.
	// If the service has multiple ports, you can specify a specific port to use here.
	// Otherwise, the unique port of the service will be used.
	// +optional
	PortName string `json:"portName,omitempty"`

	// CredentialSecret specifies the secret required to access the component service.
	// +optional
	CredentialSecret string `json:"credentialSecret,omitempty"`
}

// ComponentRoleArbitrator defines how to arbitrate the role of replicas.
// +enum
// +kubebuilder:validation:Enum={External,Lorry}
type ComponentRoleArbitrator string

const (
	External ComponentRoleArbitrator = "External"
	Lorry    ComponentRoleArbitrator = "Lorry"
)

// ComponentReplicaRole represents a role that can be assumed by a component instance.
type ComponentReplicaRole struct {
	// Name of the role. It will apply to "apps.kubeblocks.io/role" object label value.
	// It cannot be empty.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Pattern=`^.*[^\s]+.*$`
	Name string `json:"name"`

	// Serviceable indicates whether a replica with this role can provide services.
	// +kubebuilder:default=true
	// +optional
	Serviceable bool `json:"serviceable,omitempty"`

	// Writable indicates whether a replica with this role is allowed to write data.
	// +kubebuilder:default=false
	// +optional
	Writable bool `json:"writable,omitempty"`

	//// Votable indicates whether a replica with this role can participate in leader election voting.
	//// +kubebuilder:default=true
	//// +optional
	// Votable bool `json:"votable,omitempty"`
	//
	//// Leaderable indicates whether a replica with this role can be elected as a leader.
	//// +kubebuilder:default=true
	//// +optional
	// Leaderable bool `json:"leaderable,omitempty"`
}

// TargetPodSelector defines how to select pod(s) to execute a action.
// +enum
// +kubebuilder:validation:Enum={Any,All,Pod,Role,Ordinal}
type TargetPodSelector string

const (
	AnyReplica      TargetPodSelector = "Any"
	AllReplicas     TargetPodSelector = "All"
	PodSelector     TargetPodSelector = "Pod"
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

// Preconditions must be fulfilled before an action is executed.
type Preconditions struct {
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
	Exec *corev1.ExecAction `json:"exec,omitempty"`

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
	// Cannot be updated.
	// +optional
	Container string `json:"container,omitempty"`

	// Preconditions represent conditions that must be met before executing the action.
	// If any precondition is not met, the action will not be executed.
	// Cannot be updated.
	// +optional
	Preconditions *Preconditions `json:"preconditions:omitempty"`

	// TimeoutSeconds defines the timeout duration for the action in seconds.
	// Cannot be updated.
	// +kubebuilder:default=0
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds:omitempty"`

	// RetryPolicy defines the strategy for retrying the action in case of failure.
	// Cannot be updated.
	// +optional
	RetryPolicy *RetryPolicy `json:"retryPolicy,omitempty"`
}

// ComponentLifecycleActions defines a set of operational actions for interacting with component services and processes.
type ComponentLifecycleActions struct {
	// PostStart is called immediately after a component is created.
	// Cannot be updated.
	// +optional
	PostStart *Action `json:"postStart,omitempty"`

	// PreStop is called immediately before a component is terminated due to an API request.
	// Cannot be updated.
	// +optional
	PreStop *Action `json:"preStop,omitempty"`

	// LivenessProbe defines how to probe the liveness of replicas periodically.
	// Cannot be updated.
	// +optional
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`

	// RoleProbe defines how to probe the role of replicas.
	// Cannot be updated.
	// +optional
	RoleProbe *corev1.Probe `json:"roleProbe,omitempty"`

	// Switchover defines how to proactively switch the current leader to a new replica to minimize the impact on availability.
	// This action is typically invoked when the leader is about to become unavailable due to events, such as:
	// - switchover
	// - stop
	// - restart
	// - scale-in
	// - liveness probe
	// Dedicated env vars for the action:
	// - KB_SWITCHOVER_CANDIDATE_NAME: The name of the new candidate replica's Pod. It may be empty.
	// - KB_SWITCHOVER_CANDIDATE_FQDN: The FQDN of the new candidate replica. It may be empty.
	// Cannot be updated.
	// +optional
	Switchover *Action `json:"switchover,omitempty"`

	// MemberJoin defines how to add a new replica to the replication group.
	// This action is typically invoked when a new replica needs to be added, such as during scale-out.
	// It may involve updating configuration, notifying other members, and ensuring data consistency.
	// Cannot be updated.
	// +optional
	MemberJoin *Action `json:"memberJoin,omitempty"`

	// MemberLeave defines how to remove a replica from the replication group.
	// This action is typically invoked when a replica needs to be removed, such as during scale-in.
	// It may involve configuration updates and notifying other members about the departure,
	// but it is advisable to avoid performing data migration within this action.
	// Cannot be updated.
	// +optional
	MemberLeave *Action `json:"memberLeave,omitempty"`

	// Readonly defines how to set a replica service as read-only.
	// This action is used to protect a replica in case of volume space exhaustion or excessive traffic.
	// Cannot be updated.
	// +optional
	Readonly *Action `json:"readonly,omitempty"`

	// Readwrite defines how to set a replica service as read-write.
	// Cannot be updated.
	// +optional
	Readwrite *Action `json:"readwrite,omitempty"`

	// DataPopulate defines how to populate the data to create new replicas.
	// This action is typically used when a new replica needs to be constructed, such as:
	// - scale-out
	// - rebuild
	// - clone
	// It should write the valid data to stdout without including any extraneous information.
	// Cannot be updated.
	// +optional
	DataPopulate *Action `json:"dataPopulate,omitempty"`

	// DataAssemble defines how to assemble data synchronized from external before starting the service for a new replica.
	// This action is typically used when creating a new replica, such as:
	//  - scale-out
	//  - rebuild
	//  - clone
	// The data will be streamed in via stdin. If any error occurs during the assembly process,
	// the action must be able to guarantee idempotence to allow for retries from the beginning.
	// Cannot be updated.
	// +optional
	DataAssemble *Action `json:"dataAssemble,omitempty"`

	// Reload defines how to notify the replica service that there is a configuration update.
	// Cannot be updated.
	// +optional
	Reload *Action `json:"reload,omitempty"`
}
