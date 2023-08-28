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

// commands：
//  1. probe：role、healthy
//  2. 磁盘满锁 - lock/unlock instance
//  3. HA - failover/switchover
//  4. scale in/out
//  5. account
//  6. backup/resotre
//  7. restart
//  8. reload
//
// scripts：
//  1. 容器入口，包括 init container

// lifecycle：
// 1. provisioning
//   a. runtime:
//     - image/command/env
//     - cpu/mem *******
//     - scheduling: affinity, priority... *******
//     - mounts
//     - ports
//     - security
//     - lifecycle
//   b. storage *******
//   c. network
//   d. config
//   e. logging
//   f. monitor
//   g. rbac
//   h. tag/label
//   i. lifecycle
// 2. database resources
//   a. account
//   b. role
//   c.database/table/topic 等
// 3. day-2 operations
//   a. general policy: update strategy
//   b. HA: membership
//   c. scale in/out & up/down
//   d. start/stop/restart
//   e. reconfigure
//   f. backup/restore/clone
//   g. upgrade

// ComponentDefinitionSpec provides a workload component specification template, with attributes that strongly work with
// stateful workloads and day-2 operations behaviors.
type ComponentDefinitionSpec struct {
	// A component definition name, this name could be used as default name of `Cluster.spec.componentSpecs.name`,
	// and so this name is need to conform with same validation rules as `Cluster.spec.componentSpecs.name`, that
	// is currently comply with IANA Service Naming rule. This name will apply to "apps.kubeblocks.io/component-name"
	// object label value.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	Name string `json:"name"`

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

	// runtime defines runtime template of the component.
	////  - container
	////    - image/command/env
	////    - cpu/mem *******
	////    - scheduling: affinity, priority... *******
	////    - mounts
	////    - ports
	////    - security
	////    - lifecycle
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Required
	Runtime corev1.PodSpec `json:"runtime"`

	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// The configSpec field provided by provider, and
	// finally this configTemplateRefs will be rendered into the user's own configuration file according to the user's cluster.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ConfigSpecs []ComponentConfigSpec `json:"configSpecs,omitempty"`

	// logConfigs is detail log file config which provided by provider.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	LogConfigs []LogConfig `json:"logConfigs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// monitor is monitoring config which provided by provider.
	// +optional
	Monitor *MonitorConfig `json:"monitor,omitempty"`

	// +optional
	Roles []rbacv1.PolicyRule `json:"roles,omitempty"`

	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	Lifecycle *corev1.Lifecycle `json:"lifecycle,omitempty"`

	// TODO: data resources
	////   a. account
	////   b. role
	////   c. database/table/topic/collection/directory...
	// +optional
	DataResources map[ResourceKind]BuiltInResource `json:"dataResources,omitempty"`

	// TODO: day-2 operations
	//   a. general - update strategy, membership
	//   c. scale in/out & up/down
	//   d. start/stop/restart
	//   e. reconfigure
	//   f. backup/restore/clone
	//   g. upgrade

	// updateStrategy, Pods update strategy.
	// In case of workloadType=Consensus the update strategy will be following:
	//
	// serial: update Pods one by one that guarantee minimum component unavailable time.
	// 		Learner -> Follower(with AccessMode=none) -> Follower(with AccessMode=readonly) -> Follower(with AccessMode=readWrite) -> Leader
	// bestEffortParallel: update Pods in parallel that guarantee minimum component un-writable time.
	//		Learner, Follower(minority) in parallel -> Follower(majority) -> Leader, keep majority online all the time.
	// parallel: force parallel
	// +kubebuilder:default=Serial
	// +optional
	UpdateStrategy UpdateStrategy `json:"updateStrategy,omitempty"`

	// +optional
	Replication *ReplicationSpec `json:"replication,omitempty"`

	// +optional
	VolumeProtectionSpec *VolumeProtectionSpec `json:"volumeProtectionSpec,omitempty"`

	// componentDefRef is used to inject values from other components into the current component.
	// values will be saved and updated in a configmap and mounted to the current component.
	// +patchMergeKey=componentDefName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentDefName
	// +optional
	ComponentDefRef []ComponentDefRef `json:"componentDefRef,omitempty" patchStrategy:"merge" patchMergeKey:"componentDefName"`

	//// +optional
	// Manipulator *ComponentManipulator `json:"manipulator,omitempty"`
	//
	//
	//// +optional
	//
	////   c. scale in/out & up/down
	////   d. start/stop/restart
	////   e. reconfigure
	//
	////   f. backup/restore/clone
	//// +optional
	// Backup *BackupSpec `json:"replication,omitempty"`
	//
	//// +optional
	// Restore *RestoreSpec `json:"replication,omitempty"`
	////   g. upgrade

	// +optional
	Events map[EventKind]EventDefinition `json:"events,omitempty"`

	// +optional
	UserDefinedEvents map[EventKind]UserDefinedEventDefinition `json:"userDefinedEvents,omitempty"`

	// The scriptSpec field provided by provider, and
	// finally this configTemplateRefs will be rendered into the user's own configuration file according to the user's cluster.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ScriptSpecs []ComponentTemplateSpec `json:"scriptSpecs,omitempty"`

	//// probes setting for healthy checks.
	//// +optional
	// Probes *ClusterDefinitionProbes `json:"probes,omitempty"`
	//
	//// statelessSpec defines stateless related spec if workloadType is Stateless.
	//// +optional
	// StatelessSpec *StatelessSetSpec `json:"statelessSpec,omitempty"`
	//
	//// statefulSpec defines stateful related spec if workloadType is Stateful.
	//// +optional
	// StatefulSpec *StatefulSetSpec `json:"statefulSpec,omitempty"`
	//
	//// consensusSpec defines consensus related spec if workloadType is Consensus, required if workloadType is Consensus.
	//// +optional
	// ConsensusSpec *ConsensusSetSpec `json:"consensusSpec,omitempty"`
	//
	//// replicationSpec defines replication related spec if workloadType is Replication.
	//// +optional
	// ReplicationSpec *ReplicationSetSpec `json:"replicationSpec,omitempty"`
	//
	//// horizontalScalePolicy controls the behavior of horizontal scale.
	//// +optional
	// HorizontalScalePolicy *HorizontalScalePolicy `json:"horizontalScalePolicy,omitempty"`
	//
	//// Statement to create system account.
	//// +optional
	// SystemAccounts *SystemAccountSpec `json:"systemAccounts,omitempty"`
	//
	//// volumeTypes is used to describe the purpose of the volumes
	//// mapping the name of the VolumeMounts in the PodSpec.Container field,
	//// such as data volume, log volume, etc.
	//// When backing up the volume, the volume can be correctly backed up
	//// according to the volumeType.
	////
	//// For example:
	////  `name: data, type: data` means that the volume named `data` is used to store `data`.
	////  `name: binlog, type: log` means that the volume named `binlog` is used to store `log`.
	////
	//// NOTE:
	////   When volumeTypes is not defined, the backup function will not be supported,
	//// even if a persistent volume has been specified.
	//// +listType=map
	//// +listMapKey=name
	//// +optional
	// VolumeTypes []VolumeTypeSpec `json:"volumeTypes,omitempty"`
	//
	//// customLabelSpecs is used for custom label tags which you want to add to the component resources.
	//// +listType=map
	//// +listMapKey=key
	//// +optional
	// CustomLabelSpecs []CustomLabelSpec `json:"customLabelSpecs,omitempty"`
	//
	//// switchoverSpec defines command to do switchover.
	//// in particular, when workloadType=Replication, the command defined in switchoverSpec will only be executed under the condition of cluster.componentSpecs[x].SwitchPolicy.type=Noop.
	//// +optional
	// SwitchoverSpec *SwitchoverSpec `json:"switchoverSpec,omitempty"`
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

type ResourceKind string

type BuiltInResource struct {
	Resources []string
	// TODO: the way to manipulate resources

}

// RoleArbitrator defines how to arbitrate roles of cluster replicas.
// +enum
// +kubebuilder:validation:Enum={Self,Lorry}
type RoleArbitrator string

const (
	Self  RoleArbitrator = "Self"
	Lorry RoleArbitrator = "Lorry"
)

type Role struct {
	// name, role name.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=leader
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default=true
	Writable bool `json:"writable"`

	// +kubebuilder:default=true
	// +optional
	Votable bool `json:"votable"`

	// +kubebuilder:default=true
	// +optional
	Leaderable bool `json:"leaderable"`
}

type ReplicationSpec struct {
	// +optional
	Roles []Role `json:"roles,omitempty"`

	// +kubebuilder:default=Self
	// +optional
	RoleArbitrator RoleArbitrator `json:"roleArbitrator,omitempty"`

	// TODO: way to ref a defined action
	// +optional
	RoleProbe ManagedActionRef `json:roleProbe,omitempty`
}

type EventKind string

const (
	NewVersionKind          EventKind = "NewVersion"          // upgrade
	SwitchoverKind          EventKind = "Switchover"          // switchover
	NewReplicasKind         EventKind = "NewReplicas"         // backup
	DataReadyKind           EventKind = "DataReady"           // restore
	ReplicaReadyKind        EventKind = "ReplicaReady"        // join
	ReplicaRemovalKind      EventKind = "ReplicaRemoval"      // switchover + leave
	StopReplicaKind         EventKind = "StopReplica"         // switchover
	RestartReplicaKind      EventKind = "RestartReplica"      // switchover
	VolumeHighWatermarkKind EventKind = "VolumeHighWatermark" // lock
	VolumeLowWatermarkKind  EventKind = "VolumeLowWatermark"  // unlock
	ConfigurationUpdateKind EventKind = "ConfigurationUpdate" // reload
	TimerKind               EventKind = "Timer"               // timer?
	UserDefinedKind         EventKind = "UserDefined"
)

type EventScope string

const (
	EventScopeComponent EventScope = "Component"
	EventScopeReplica   EventScope = "Replica"
)

type TargetReplicaSelector string

const (
	AnyReplica   TargetReplicaSelector = "*"
	AllReplicas  TargetReplicaSelector = "All"
	RoleSelector TargetReplicaSelector = "Role"
	PodSelector  TargetReplicaSelector = "Pod"
)

type EventDefinition struct {
	Kind            EventKind               `json:"kind"`
	Scope           EventScope              `json:"scope"`
	TargetContainer string                  `json:"targetContainer,omitempty"`
	Handler         corev1.LifecycleHandler `json:"handler"`
	Selector        TargetReplicaSelector   `json:"selector,omitempty"`
	MatchingKey     string                  `json:"matchingKey,omitempty"`
}

type EventPreconditions struct {
	Role  string
	Phase string
	// TODO: ...
}

type UserDefinedEventDefinition struct {
	EventDefinition
	SubKind       EventKind
	Preconditions EventPreconditions
}

type ComponentManipulator struct {
	// commands：
	//  1. probe：role、healthy
	//  2. 磁盘满锁 - lock/unlock instance
	//  3. HA - failover/switchover
	//  4. scale in/out
	//  5. account
	//  6. backup/resotre
	//  7. restart
	//  8. reload

	// +optional
	Healthy *corev1.Probe `json:"healthy,omitempty"`

	// +optional
	Readiness *corev1.Probe `json:"readiness,omitempty"`

	// +optional
	RoleProbe *corev1.Probe `json:"roleProbe,omitempty"`

	// +optional
	Promote *Action `json:"promote,omitempty"`

	// +optional
	Demote *Action `json:"demote,omitempty"`

	// +optional
	MemberJoin *Action `json:"memberJoin,omitempty"`

	// +optional
	MemberLeave *Action `json:"memberLeave,omitempty"`

	//// +optional
	Switchover *Action `json:"switchover,omitempty"`

	// VolumeXxxxxEvent:
	// +optional
	Lock *Action `json:"lock,omitempty"`

	// +optional
	Unlock *Action `json:"unlock,omitempty"`

	//
	// +optional
	AddReplica *Action `json:"addReplica,omitempty"`

	// +optional
	RemoveReplica *Action `json:"removeReplica,omitempty"`

	// +optional
	Backup *Action `json:"backup,omitempty"`

	// +optional
	Restore *Action `json:"restore,omitempty"`

	// +optional
	Shutdown *Action `json:"shutdown,omitempty"`

	// +optional
	Restart *Action `json:"restart,omitempty"`

	// +optional
	Reload *Action `json:"reload,omitempty"`

	// +optional
	UnmanagedActions map[string]Action `json:"unmanagedActions:omitempty"`
}

type Action struct {
	ActionHandler `json:",inline"`

	// +optional
	Preconditions Preconditions `json:"preconditions:omitempty"`

	// +optional
	PostHook string `json:"postHook,omitempty"`
}

type ManagedActionRef string

type UnmanagedActionRef string

type ActionHandler corev1.LifecycleHandler

// Preconditions must be fulfilled before an action is carried out.
type Preconditions struct {
	Role string
}

// in:
//   envs:
//   	name: cluster, component, pod
// 		endpoint: cluster, component, pod
// 		replicas
//		role
// 		credential: user & password
//    args: json, yaml, text
// out:
//    return: 0 - ok,
//            1 - in-progress
//            others - fail
//    output: json
//
// actions:
// 	healthProbe:
// 		read
//  	write
// 	readinessProbe:
//  membership:
//		probe
// 		promote
//		demote
//  	join
// 		leave
//  switchover
//	writablility:
//		disable:
//		enable:
// 	addReplica:
//	removeReplica:
//  backup
//  restore
//  shutdown
//  restart
//  reload
//
