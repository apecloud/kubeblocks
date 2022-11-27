/*
Copyright ApeCloud Inc.

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

// ClusterDefinitionSpec defines the desired state of ClusterDefinition
type ClusterDefinitionSpec struct {
	// Type define well known cluster types. Valid values are in-list of
	// [state.redis, mq.mqtt, mq.kafka, state.mysql-8, state.mysql-5.7, state.mysql-5.6, state-mongodb]
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=24
	Type string `json:"type"`

	// List of components belonging to the cluster
	// +kubebuilder:validation:MinItems=1
	// +optional
	Components []ClusterDefinitionComponent `json:"components,omitempty"`

	// Default termination policy if no termination policy defined in cluster
	// +kubebuilder:validation:Enum={DoNotTerminate,Halt,Delete,WipeOut}
	DefaultTerminationPolicy string `json:"defaultTerminationPolicy,omitempty"`

	// Credential used for connecting database
	// +optional
	ConnectionCredential ClusterDefinitionConnectionCredential `json:"connectionCredential,omitempty"`
}

// ClusterDefinitionStatus defines the observed state of ClusterDefinition
type ClusterDefinitionStatus struct {
	// phase - in list of [Available]
	// +kubebuilder:validation:Enum={Available}
	Phase Phase `json:"phase,omitempty"`
	// Extra message in current phase
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
	// Specify the name of the referenced configuration template, which is a configmap object
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	Name string `json:"name,omitempty"`

	// VolumeName is the volume name of PodTemplate, which the configuration file produced through the configuration template will be mounted to the corresponding volume.
	// The volume name must be defined in podSpec.containers[*].volumeMounts.
	// reference example: https://github.com/apecloud/kubeblocks/blob/main/examples/dbaas/mysql_clusterdefinition.yaml#L12
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	VolumeName string `json:"volumeName,omitempty"`
}

type ExporterConfig struct {
	// ScrapePort is exporter port for Time Series Database to scrape metrics
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Maximum=65536
	// +kubebuilder:validation:Minimum=1
	ScrapePort int `json:"scrapePort"`

	// ScrapePath is exporter url path for Time Series Database to scrape metrics
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:default="/metrics"
	ScrapePath string `json:"scrapePath"`
}

type MonitorConfig struct {
	// BuiltIn is a switch to enable DBaas builtIn monitoring.
	// If BuiltIn is true and CharacterType is wellknown, ExporterConfig and Sidecar container will generate automatically.
	// Otherwise, ISV should set BuiltIn to false and provide ExporterConfig and Sidecar container own.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=true
	BuiltIn bool `json:"builtIn"`

	// Exporter provided by ISV, which specify necessary information to Time Series Database.
	// ExporterConfig is valid when BuiltIn is false.
	// +optional
	Exporter *ExporterConfig `json:"exporterConfig,omitempty"`
}

type ConfigurationSpec struct {
	// The configTemplateRefs field provided by ISV, and
	// finally this configTemplateRefs will be rendered into the user's own configuration file against the user's cluster.
	// +optional
	ConfigTemplateRefs []ConfigTemplate `json:"configTemplateRefs,omitempty"`

	// TODO(zt) support multi scene, Different scenarios use different configuration templates.
	// User modify scene in cluster field or reconfigure ops.
	// ConfigTemplateRefs map[string][]ConfigTemplate `json:"configTemplateRefs,omitempty"`
	// DefaultScene string `json:"defaultScene,omitempty"`

	// ConfigRevisionHistoryLimit is number of prior versions of configuration variations submitted by users, By default, 6 versions are reserved.
	// +kubebuilder:default=6
	// +kubebuilder:validation:Minimum=0
	// +optional
	ConfigRevisionHistoryLimit int `json:"configRevisionHistoryLimit,omitempty"`

	// ConfigReload indicates whether the engine supports reload.
	// if false, the controller will restart the engine instance.
	// if true, the controller will determine the behavior of the engine instance based on the configuration templates,
	// restart or reload depending on whether any parameters in the StaticParameters have been modified.
	// +kubebuilder:default=false
	// +optional
	ConfigReload bool `json:"configReload,omitempty"`

	// ConfigReloadType describes the restart methods.
	// +kubebuilder:validation:Enum={signal,http,sql,exec}
	// +optional
	ConfigReloadType CfgReloadType `json:"configReloadType,omitempty"`

	// ConfigReloadTrigger describes the configuration against reload type.
	// +optional
	ConfigReloadTrigger *ConfigReloadTrigger `json:"configReloadTrigger,omitempty"`
}

type ConfigReloadTrigger struct {
	// Signal is valid for unix signal
	// e.g: SIGHUP
	// url: ../../internal/configuration/configmap/handler.go:allUnixSignals
	// +optional
	Signal string `json:"signal,omitempty"`

	// ProcessName is process name,sends unix signal to proc.
	// +optional
	ProcessName string `json:"processName,omitempty"`

	// TODO support reload way
}

// ClusterDefinitionComponent is a group of pods, pods in one component usually share the same data
type ClusterDefinitionComponent struct {
	// Type name of the component, it can be any valid string
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=12
	TypeName string `json:"typeName,omitempty"`

	// CharacterType defines well-known database component name, such as mongos(mongodb), proxy(redis), mariadb(mysql)
	// DBaas will generate proper monitor configs for well-known CharacterType when BuiltIn is true.
	// +optional
	CharacterType string `json:"characterType,omitempty"`

	// Minimum available pod count when updating
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	MinAvailable int `json:"minAvailable,omitempty"`

	// Maximum available pod count after scale
	// +kubebuilder:validation:Minimum=0
	MaxAvailable int `json:"maxAvailable,omitempty"`

	// Default replicas in this component when not specified.
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	DefaultReplicas int `json:"defaultReplicas,omitempty"`

	// ConfigSpec defines configuration related spec.
	// +optional
	ConfigSpec *ConfigurationSpec `json:"configSpec,omitempty"`

	// Monitor is monitoring config which provided by ISV
	// +optional
	Monitor *MonitorConfig `json:"monitor,omitempty"`

	// antiAffinity defines components should have anti-affinity constraint for pods with same component type.
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

	// Scripts executed before and after workload operation
	// script exec order：component.pre => component.exec => component.post
	// builtin ENV variables:
	// self: KB_SELF_{builtin_properties}
	// rule: KB_{conponent_name}[n]-{builtin_properties}
	// builtin_properties:
	// - ID # which shows in Cluster.status
	// - HOST # e.g. example-mongodb2-0.example-mongodb2-svc.default.svc.cluster.local
	// - PORT
	// - N # number of current components
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
	// Default scripts executed if the following scripts not defined
	Default ClusterDefinitionScript `json:"default,omitempty"`
	// Scripts executed before and after creation
	Create ClusterDefinitionScript `json:"create,omitempty"`
	// Scripts executed before and after upgrade
	Upgrade ClusterDefinitionScript `json:"upgrade,omitempty"`
	// Scripts executed before and after vertical scale
	VerticalScale ClusterDefinitionScript `json:"verticalScale,omitempty"`
	// Scripts executed before and after horizontal scale
	HorizontalScale ClusterDefinitionScript `json:"horizontalScale,omitempty"`
	// Scripts executed before and after deletion
	Delete ClusterDefinitionScript `json:"delete,omitempty"`
}

type ClusterDefinitionScript struct {
	// Pre hook before operation
	Pre []ClusterDefinitionContainerCMD `json:"pre,omitempty"`
	// Post hook after operation
	Post []ClusterDefinitionContainerCMD `json:"post,omitempty"`
}

// ClusterDefinitionContainerCMD defines content of a hook script
type ClusterDefinitionContainerCMD struct {
	// Container used to execute command
	Container string `json:"container,omitempty"`
	// Command executed in container
	Command []string `json:"command,omitempty"`
	// Args executed in container
	Args []string `json:"args,omitempty"`
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
	// sqls executed on db node, used to check db healthy
	Writes  []string `json:"writes,omitempty"`
	Queries []string `json:"queries,omitempty"`
}

type ClusterDefinitionProbe struct {
	// enable probe or not
	// +kubebuilder:default=true
	Enable bool `json:"enable,omitempty"`
	// How often (in seconds) to perform the probe.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`
	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	FailureThreshold int32 `json:"failureThreshold,omitempty"`
	// Minimum consecutive successes for the probe to be considered successful after having failed.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	SuccessThreshold int32                      `json:"successThreshold,omitempty"`
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
