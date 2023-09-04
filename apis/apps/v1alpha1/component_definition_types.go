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
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=cd
// +kubebuilder:printcolumn:name="MAIN-COMPONENT-NAME",type="string",JSONPath=".spec.componentDefs[0].name",description="main component names"
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

	// +kubebuilder:validation:MaxLength=32
	// +optional
	Provider string `json:"provider,omitempty"`

	// The description of component definition.
	// +kubebuilder:validation:MaxLength=512
	// +optional
	Description string `json:"description,omitempty"`

	// ServiceKind defines what kind of well-known service the component provided (e.g., MySQL, Redis, ETCD, case insensitive).
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceKind string `json:"serviceKind,omitempty"`

	// ServiceVersion defines version of the well-known service that the component provided.
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceVersion string `json:"serviceVersion,omitempty"`

	// Runtime defines the mainly runtime information, include:
	//  - init containers
	//  - containers
	//  	- image
	// 		- commands
	//      - args
	// 		- envs
	// 		- mounts
	// 		- ports
	// 		- security context
	//      - probes: startup, liveness, readiness
	// 		- lifecycle
	//  - volumes
	// Resources (cpu & mem) and scheduling setting (affinity, toleration, priority) should not been set here.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Required
	Runtime corev1.PodSpec `json:"runtime"`

	// Volumes defines the volumes needed by the component to persistent all meta and data.
	// The user will be responsible for providing these volumes when creating a component instance.
	// +optional
	Volumes []Volume `json:"volumes"`

	// Services defines endpoints that can be used to access the service provided by the component.
	// If specified, a headless service will be created with some attributes of Services[0] by default.
	// +optional
	Services []*corev1.ServiceSpec `json:"services,omitempty"`

	// The configSpec field provided by provider, and
	// finally this configTemplateRefs will be rendered into the user's own configuration file according to the user's cluster.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ConfigSpecs []ComponentConfigSpec `json:"configSpecs,omitempty"`

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

	// ConnectionCredential defines the template to create a default connection credential secret for a component instance.
	// Cannot be updated.
	// +optional
	ConnectionCredential ConnectionCredential `json:"connectionCredential,omitempty"`

	// The scriptSpec field provided by provider, and
	// finally this configTemplateRefs will be rendered into the user's own configuration file according to the user's cluster.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ScriptSpecs []ComponentTemplateSpec `json:"scriptSpecs,omitempty"`

	// Rules defines the namespaced policy rules needed by a component.
	// If any rule application fails (e.g., no permission), the provisioning of component instance will fail.
	// Cannot be updated.
	// +optional
	Rules []rbacv1.PolicyRule `json:"rules,omitempty"`

	// Labels defines the static labels which will be added to all component resources.
	// If the label key is conflict with any system labels, it will be ignored silently.
	// Cannot be updated.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Cannot be updated.
	// +optional
	Lifecycle *corev1.Lifecycle `json:"lifecycle,omitempty"`

	// MetaResources declares static meta resources to be created at the component ready (e.g., account, database, table, topic).
	// If any resources specify, the provisioning command for the kind of resource must be provider by @Actions.
	// +optional
	MetaResources map[MetaResourceKind]MetaResource `json:"metaResources,omitempty"`

	// +kubebuilder:default=Serial
	// +optional
	UpdateStrategy UpdateStrategy `json:"updateStrategy,omitempty"`

	// Roles defines the roles of the component.
	// +optional
	Roles []Role `json:"roles,omitempty"`

	// +kubebuilder:default=External
	// +optional
	RoleArbitrator RoleArbitrator `json:"roleArbitrator,omitempty"`

	// Actions defines all the internal operations that needed to interoperate with component's service (process).
	// +optional
	Actions ComponentActionSet `json:"actions,omitempty"`

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

type Volume struct {
	// Name of the volume.
	// Must be a DNS_LABEL and unique within the pod.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	// +required
	Name string `json:"name"`

	// Optional indicates whether this volume is optional to provide when creating a component instance.
	// If the values is true, but there is somewhere reference it (e.g., a mount in @Runtime),
	// the default volume (or first volume) will be used, or the provisioning will fail.
	// +kubebuilder:default=false
	// +optional
	Optional bool `json:"optional,omitempty"`

	// Synchronization indicates whether the data on this volume needs to be synchronized to destination when making a backup or building a new replica.
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

type MetaResourceKind string

type MetaResource struct {
	// +optional
	Resources []BuiltInString `json:"resources,omitempty"`

	// +optional
	SecretRef *corev1.SecretReference `json:"secretRef,omitempty"`
}

// RoleArbitrator defines how to arbitrate the role of replicas.
// +enum
// +kubebuilder:validation:Enum={External,Lorry}
type RoleArbitrator string

const (
	External RoleArbitrator = "External"
	Lorry    RoleArbitrator = "Lorry"
)

type Role struct {
	// Name of the role. It will apply to "apps.kubeblocks.io/role" object label value.
	// It can not be empty.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Pattern=`^.*[^\s]+.*$`
	Name string `json:"name"`

	// Writable indicates whether a replica with this role can be written.
	// +kubebuilder:default=true
	// +optional
	Writable bool `json:"writable,omitempty"`

	// // Votable indicates whether a replica with this role can vote at leader election.
	// // +kubebuilder:default=true
	// // +optional
	// Votable bool `json:"votable,omitempty"`
	//
	// // Leaderable indicates whether a replica with this role can be elected as a leader.
	// // +kubebuilder:default=true
	// // +optional
	// Leaderable bool `json:"leaderable,omitempty"`
}

type ActionHandler struct {
	corev1.ProbeHandler `json:"inline"`

	// +optional
	Image string `json:"image,omitempty"`
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

// There are some pre-defined env vars (BuiltInVars) can be used in the action, check @BuiltInVars for reference.
type Action struct {
	ActionHandler `json:"inline"`

	// PodSelector specifies the way that how to select pod(s) to execute the action if there may not have a target replica.
	// +optional
	PodSelector TargetPodSelector `json:"podSelector,omitempty"`

	// MatchingKey uses to select pod(s) actually.
	// If the selector is AnyReplica or AllReplicas, the matchingKey will be ignored.
	// If the selector is RoleSelector
	// If the selector is OrdinalSelector
	// +optional
	MatchingKey string `json:"matchingKey,omitempty"`

	// Container specifies which container the action will be executed in.
	// If specified, it must be one of container declared in @Runtime.
	// +optional
	Container string `json:"container,omitempty"`

	// Timeout
	// +kubebuilder:default=0
	// +optional
	Timeout int32 `json:"timeout:omitempty"`

	//// +optional
	// Preconditions Preconditions `json:"preconditions:omitempty"`
}

// ComponentActionSet defines all actions that should be provided to interoperate with component services and processes.
type ComponentActionSet struct {
	// RoleProbe defines how to probe the role of a replica.
	// +optional
	RoleProbe *corev1.Probe `json:roleProbe,omitempty`

	// Switchover defines how to actively switch the current leader to a new replica to minimize the affect to availability.
	// It will be called when the leader is about to become unavailable, such as:
	//  - switchover
	//  - stop
	//  - restart
	//  - scale-in
	// Action dedicated env vars:
	//  - KB_SWITCHOVER_CANDIDATE_NAME: the pod name of new candidate replica, it may be empty.
	//  - KB_SWITCHOVER_CANDIDATE_FQDN: the FQDN of new candidate replica, it may be empty.
	// +optional
	Switchover *Action `json:"switchover,omitempty"`

	// MemberJoin defines how to add a new replica into the replication membership.
	// It's typically called when a new replica to be added (e.g., scale-out).
	// +optional
	MemberJoin *Action `json:"memberJoin,omitempty"`

	// MemberLeave defines how to remove a new replica from the replication membership.
	// It's typically called when a replica to be removed (e.g., scale-in).
	// +optional
	MemberLeave *Action `json:"memberLeave,omitempty"`

	// Readonly defines how to set the replica service as read-only.
	// It will be used to protect the replica in case of volume space exhaustion or too heavy traffic.
	// +optional
	Readonly *Action `json:"readonly,omitempty"`

	// Readwrite defines how to set the replica service back to read-write, the opposite operation of @Readonly.
	// +optional
	Readwrite *Action `json:"readwrite,omitempty"`

	// Backup defines how to set up an endpoint from which data can be replicated out an existed replica.
	// It's typically used when a new replica to be built, such as:
	//  - scale-out
	//  - rebuild
	//  - clone
	// In:
	//  - xxx
	// Out:
	//  - xxx
	// +optional
	Backup *Action `json:"backup,omitempty"`

	// Restore defines how to set up an endpoint from which data can be replicated in a new replica.
	// It's typically used when a new replica to be built, such as:
	//  - scale-out
	//  - rebuild
	//  - clone
	// In:
	//  - xxx
	// Out:
	//  - xxx
	// +optional
	Restore *Action `json:"restore,omitempty"`

	// Reload defines how to notify the replica service that there is a configuration update.
	// +optional
	Reload *Action `json:"reload,omitempty"`

	// +optional
	MetaResourceProvision map[MetaResourceKind]*Action `json:"metaResourceProvision,omitempty"`
}
