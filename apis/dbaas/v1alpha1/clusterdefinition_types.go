/*
Copyright 2022 The KubeBlocks Authors

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterDefinitionSpec defines the desired state of ClusterDefinition
type ClusterDefinitionSpec struct {
	// Type define well known cluster types. Valid values are in-list of
	// [state.redis, mq.mqtt, mq.kafka, state.mysql-8, state.mysql-5.7, state.mysql-5.6, state-mongodb]
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=24
	Type string `json:"type"`

	// +kubebuilder:validation:MinItems=1
	// +optional
	Components []ClusterDefinitionComponent `json:"components,omitempty"`

	// +kubebuilder:validation:Enum={DoNotTerminate,Halt,Delete,WipeOut}
	DefaultTerminatingPolicy string `json:"defaultTerminationPolicy,omitempty"`

	// +optional
	ConnectionCredential ClusterDefinitionConnectionCredential `json:"connectionCredential,omitempty"`
}

// ClusterDefinitionStatus defines the observed state of ClusterDefinition
type ClusterDefinitionStatus struct {
	// phase - in list of [Available,Deleting]
	// +kubebuilder:validation:Enum={Available,Deleting}
	Phase Phase `json:"phase,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
	// observedGeneration is the most recent generation observed for this
	// ClusterDefinition. It corresponds to the ClusterDefinition's generation, which is
	// updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories={dbaas},scope=Cluster,shortName=cd
//+kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="status phase"
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ClusterDefinition is the Schema for the clusterdefinitions API
type ClusterDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterDefinitionSpec   `json:"spec,omitempty"`
	Status ClusterDefinitionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterDefinitionList contains a list of ClusterDefinition
type ClusterDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterDefinition `json:"items"`
}

type ConfigTemplate struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	Name string `json:"name,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	VolumeName string `json:"volumeName,omitempty"`
}

type ClusterDefinitionComponent struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=12
	TypeName string `json:"typeName,omitempty"`

	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	MinAvailable int `json:"minAvailable,omitempty"`

	// +kubebuilder:validation:Minimum=0
	MaxAvailable int `json:"maxAvailable,omitempty"`

	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	DefaultReplicas int `json:"defaultReplicas,omitempty"`

	// The configTemplateRefs field provided by ISV, and
	// finally this configTemplateRefs will be rendered into the user's own configuration file according to the user's cluster
	// +optional
	ConfigTemplateRefs []ConfigTemplate `json:"configTemplateRefs,omitempty"`

	// antiAffinity defines components should have anti-affinity constraint to same component type
	// +kubebuilder:default=false
	AntiAffinity bool `json:"antiAffinity,omitempty"`

	// podSpec of final workload
	// +optional
	PodSpec *corev1.PodSpec `json:"podSpec,omitempty"`

	// Service defines the behavior of a service spec.
	// provide read-write service when ComponentType is Consensus
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Service corev1.ServiceSpec `json:"service,omitempty"`

	// script exec order：component.pre => component.exec => component.post
	// builtin ENV variables:
	// self: OPENDBAAS_SELF_{builtin_properties}
	// rule: OPENDBAAS_{conponent_name}[n]-{builtin_properties}
	// builtin_properties:
	// - ID # which shows in Cluster.status
	// - HOST # e.g. example-mongodb2-0.example-mongodb2-svc.default.svc.cluster.local
	// - PORT
	// - N # number of current component
	// +optional
	Scripts ClusterDefinitionScripts `json:"scripts,omitempty"`

	Probes ClusterDefinitionProbes `json:"probes,omitempty"`

	// ComponentType defines type of the component
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Stateless
	// +kubebuilder:validation:Enum={Stateless,Stateful,Consensus}
	ComponentType ComponentType `json:"componentType"`

	// ConsensusSpec defines consensus related spec if componentType is Consensus
	// CAN'T be empty if componentType is Consensus
	// +optional
	ConsensusSpec *ConsensusSetSpec `json:"consensusSpec,omitempty"`
}

type ComponentType string

const (
	Stateless ComponentType = "Stateless"
	Stateful  ComponentType = "Stateful"
	Consensus ComponentType = "Consensus"
)

type ClusterDefinitionScripts struct {
	Default         ClusterDefinitionScript `json:"default,omitempty"`
	Create          ClusterDefinitionScript `json:"create,omitempty"`
	Upgrade         ClusterDefinitionScript `json:"upgrade,omitempty"`
	VerticalScale   ClusterDefinitionScript `json:"verticalScale,omitempty"`
	HorizontalScale ClusterDefinitionScript `json:"horizontalScale,omitempty"`
	Delete          ClusterDefinitionScript `json:"delete,omitempty"`
}

type ClusterDefinitionScript struct {
	Pre  []ClusterDefinitionContainerCMD `json:"pre,omitempty"`
	Post []ClusterDefinitionContainerCMD `json:"post,omitempty"`
}

type ClusterDefinitionContainerCMD struct {
	Container string   `json:"container,omitempty"`
	Command   []string `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
}

type ClusterDefinitionUpdateStrategy struct {
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	MaxUnavailable int `json:"maxUnavailable,omitempty"`
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	MaxSurge int `json:"maxSurge,omitempty"`
}

type ClusterDefinitionConnectionCredential struct {
	// +kubebuilder:default=root
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
}

type ClusterDefinitionStatusGeneration struct {

	// ClusterDefinition generation number
	// +optional
	ClusterDefGeneration int64 `json:"clusterDefGeneration,omitempty"`

	// ClusterDefinition sync. status
	// +kubebuilder:validation:Enum={InSync,OutOfSync}
	// +optional
	ClusterDefSyncStatus Status `json:"clusterDefSyncStatus,omitempty"`
}

type ClusterDefinitionProbeCMDs struct {
	Writes  []string `json:"writes,omitempty"`
	Queries []string `json:"queries,omitempty"`
}

type ClusterDefinitionProbe struct {
	// +kubebuilder:default=true
	Enable bool `json:"enable,omitempty"`
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	PeriodSeconds int `json:"periodSeconds,omitempty"`
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	FailureThreshold int                        `json:"failureThreshold,omitempty"`
	Commands         ClusterDefinitionProbeCMDs `json:"commands,omitempty"`
}

type ClusterDefinitionProbes struct {
	RunningProbe     ClusterDefinitionProbe `json:"runningProbe,omitempty"`
	StatusProbe      ClusterDefinitionProbe `json:"statusProbe,omitempty"`
	RoleChangedProbe ClusterDefinitionProbe `json:"roleChangedProbe,omitempty"`
}

type ConsensusSetSpec struct {
	// Leader, one single leader
	// +kubebuilder:validation:Required
	Leader ConsensusMember `json:"leader"`

	// Followers, has voting right but not Leader
	// +optional
	Followers []ConsensusMember `json:"followers,omitempty"`

	// Learner, no voting right
	// +optional
	Learner *ConsensusMember `json:"learner,omitempty"`

	// UpdateStrategy, Pods update strategy
	// options: serial, bestEffortParallel, parallel
	// serial: update Pods one by one that guarantee minimum component unavailable time
	// 		Learner -> Follower(with AccessMode=none) -> Follower(with AccessMode=readonly) -> Follower(with AccessMode=readWrite) -> Leader
	// bestEffortParallel: update Pods in parallel that guarantee minimum component un-writable time
	//		Learner, Follower(minority) in parallel -> Follower(majority) -> Leader, keep majority online all the time
	// parallel: force parallel
	// +kubebuilder:default=Serial
	// +kubebuilder:validation:Enum={Serial,BestEffortParallel,Parallel}
	// +optional
	UpdateStrategy UpdateStrategy `json:"updateStrategy,omitempty"`
}

type ConsensusMember struct {
	// Name, role name
	// +kubebuilder:validation:Required
	// +kubebuilder:default=leader
	Name string `json:"name"`

	// AccessMode, what service this member capable for
	// +kubebuilder:validation:Required
	// +kubebuilder:default=ReadWrite
	// +kubebuilder:validation:Enum={None, Readonly, ReadWrite}
	AccessMode AccessMode `json:"accessMode"`

	// Replicas, number of Pods of this role
	// default 1 for Leader
	// default 0 for Learner
	// default Components[*].Replicas - Leader.Replicas - Learner.Replicas for Followers
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
}

type AccessMode string

const (
	ReadWrite AccessMode = "ReadWrite"
	Readonly  AccessMode = "Readonly"
	None      AccessMode = "None"
)

type UpdateStrategy string

const (
	Serial             UpdateStrategy = "Serial"
	BestEffortParallel UpdateStrategy = "BestEffortParallel"
	Parallel           UpdateStrategy = "Parallel"
)

func init() {
	SchemeBuilder.Register(&ClusterDefinition{}, &ClusterDefinitionList{})
}
