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
	"strings"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterDefinitionSpec defines the desired state of ClusterDefinition
type ClusterDefinitionSpec struct {
	// Type define well known cluster types. Valid values are in-list of
	// [state.redis, mq.mqtt, mq.kafka, state.mysql-8, state.mysql-5.7, state.mysql-5.6, state-mongodb].
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=24
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Type string `json:"type"`

	// List of components belonging to the cluster.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=typeName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=typeName
	Components []ClusterDefinitionComponent `json:"components" patchStrategy:"merge,retainKeys" patchMergeKey:"typeName"`

	// Default connection credential used for connecting to cluster service.
	// +optional
	ConnectionCredential map[string]string `json:"connectionCredential,omitempty"`
}

// ClusterDefinitionStatus defines the observed state of ClusterDefinition
type ClusterDefinitionStatus struct {
	// ClusterDefinition phase -
	// Available is ClusterDefinition become available, and can be referenced for co-related objects.
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

type ConfigTemplate struct {
	// Specify the name of the referenced the configuration template ConfigMap object.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Specify the namespace of the referenced the configuration template ConfigMap object.
	// An empty namespace is equivalent to the "default" namespace.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:default="default"
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// VolumeName is the volume name of PodTemplate, which the configuration file produced through the configuration template will be mounted to the corresponding volume.
	// The volume name must be defined in podSpec.containers[*].volumeMounts.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	VolumeName string `json:"volumeName"`

	// defaultMode is optional: mode bits used to set permissions on created files by default.
	// Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511.
	// YAML accepts both octal and decimal values, JSON requires decimal values for mode bits.
	// Defaults to 0644.
	// Directories within the path are not affected by this setting.
	// This might be in conflict with other options that affect the file
	// mode, like fsGroup, and the result can be other mode bits set.
	// +optional
	DefaultMode *int32 `json:"defaultMode,omitempty" protobuf:"varint,3,opt,name=defaultMode"`
}

type ExporterConfig struct {
	// ScrapePort is exporter port for Time Series Database to scrape metrics.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:Minimum=0
	ScrapePort int32 `json:"scrapePort"`

	// ScrapePath is exporter url path for Time Series Database to scrape metrics.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:default="/metrics"
	ScrapePath string `json:"scrapePath"`
}

type MonitorConfig struct {
	// BuiltIn is a switch to enable DBaas builtIn monitoring.
	// If BuiltIn is true and CharacterType is wellknown, ExporterConfig and Sidecar container will generate automatically.
	// Otherwise, provider should set BuiltIn to false and provide ExporterConfig and Sidecar container own.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=false
	BuiltIn bool `json:"builtIn"`

	// Exporter provided by provider, which specify necessary information to Time Series Database.
	// ExporterConfig is valid when BuiltIn is false.
	// +optional
	Exporter *ExporterConfig `json:"exporterConfig,omitempty"`
}

type LogConfig struct {
	// Name log type name, such as slow for MySQL slow log file.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	Name string `json:"name"`

	// FilePathPattern log file path pattern which indicate how to find this file
	// corresponding to variable (log path) in database kernel. please don't set this casually.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=4096
	FilePathPattern string `json:"filePathPattern"`
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
	// Type name of the component, it can be any valid string.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=12
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	TypeName string `json:"typeName"`

	// componentType defines type of the component. On of Stateful, Stateless, Consensus.
	// Default to Stateless.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum={Stateless,Stateful,Consensus}
	// +kubebuilder:default=Stateless
	ComponentType ComponentType `json:"componentType"`

	// CharacterType defines well-known database component name, such as mongos(mongodb), proxy(redis), wesql(mysql)
	// DBaas will generate proper monitor configs for wellknown CharacterType when BuiltIn is true.
	// +optional
	CharacterType string `json:"characterType,omitempty"`

	// MinReplicas minimum replicas for component pod count.
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	MinReplicas int32 `json:"minReplicas,omitempty"`

	// MaxReplicas maximum replicas pod for component pod count.
	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxReplicas int32 `json:"maxReplicas,omitempty"`

	// DefaultReplicas default replicas in this component if user not specify.
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	DefaultReplicas int32 `json:"defaultReplicas,omitempty"`

	// PDBSpec pod disruption budget spec. This is mutually exclusive with the component type of Consensus.
	// +optional
	PDBSpec *policyv1.PodDisruptionBudgetSpec `json:"pdbSpec,omitempty"`

	// ConfigSpec defines configuration related spec.
	// +optional
	ConfigSpec *ConfigurationSpec `json:"configSpec,omitempty"`

	// Monitor is monitoring config which provided by provider.
	// +optional
	Monitor *MonitorConfig `json:"monitor,omitempty"`

	// LogConfigs is detail log file config which provided by provider.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	LogConfigs []LogConfig `json:"logConfigs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// antiAffinity defines components should have anti-affinity constraint to same component type.
	// +kubebuilder:default=false
	// +optional
	AntiAffinity bool `json:"antiAffinity,omitempty"`

	// podSpec of final workload
	// +optional
	PodSpec *corev1.PodSpec `json:"podSpec,omitempty"`

	// service defines the behavior of a service spec.
	// provide read-write service when ComponentType is Consensus.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Service *corev1.ServiceSpec `json:"service,omitempty"`

	// Probes setting for db healthy checks.
	// +optional
	Probes *ClusterDefinitionProbes `json:"probes,omitempty"`

	// consensusSpec defines consensus related spec if componentType is Consensus, required if componentType is Consensus.
	// +optional
	ConsensusSpec *ConsensusSetSpec `json:"consensusSpec,omitempty"`
}

type ClusterDefinitionStatusGeneration struct {
	// ClusterDefinition generation number.
	// +optional
	ClusterDefGeneration int64 `json:"clusterDefGeneration,omitempty"`

	// ClusterDefinition sync. status.
	// +kubebuilder:validation:Enum={InSync,OutOfSync}
	// +optional
	ClusterDefSyncStatus Status `json:"clusterDefSyncStatus,omitempty"`
}

type ClusterDefinitionProbeCMDs struct {
	// Write check executed on probe sidecar, used to check workload's allow write access.
	// +optional
	Writes []string `json:"writes,omitempty"`

	// Read check executed on probe sidecar, used to check workload's reaonly access .
	// +optional
	Queries []string `json:"queries,omitempty"`
}

type ClusterDefinitionProbe struct {
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
	SuccessThreshold int32 `json:"successThreshold,omitempty"`

	// commands used to execute for probe.
	// +optional
	Commands *ClusterDefinitionProbeCMDs `json:"commands,omitempty"`
}

type ClusterDefinitionProbes struct {
	// Probe for DB running check.
	// +optional
	RunningProbe *ClusterDefinitionProbe `json:"runningProbe,omitempty"`

	// Probe for DB status check.
	// +optional
	StatusProbe *ClusterDefinitionProbe `json:"statusProbe,omitempty"`

	// Probe for DB role changed check.
	// +optional
	RoleChangedProbe *ClusterDefinitionProbe `json:"roleChangedProbe,omitempty"`
}

type ConsensusSetSpec struct {
	// Leader, one single leader.
	// +kubebuilder:validation:Required
	Leader ConsensusMember `json:"leader"`

	// Followers, has voting right but not Leader.
	// +optional
	Followers []ConsensusMember `json:"followers,omitempty"`

	// Learner, no voting right.
	// +optional
	Learner *ConsensusMember `json:"learner,omitempty"`

	// UpdateStrategy, Pods update strategy.
	// serial: update Pods one by one that guarantee minimum component unavailable time.
	// 		Learner -> Follower(with AccessMode=none) -> Follower(with AccessMode=readonly) -> Follower(with AccessMode=readWrite) -> Leader
	// bestEffortParallel: update Pods in parallel that guarantee minimum component un-writable time.
	//		Learner, Follower(minority) in parallel -> Follower(majority) -> Leader, keep majority online all the time.
	// parallel: force parallel
	// +kubebuilder:default=Serial
	// +kubebuilder:validation:Enum={Serial,BestEffortParallel,Parallel}
	// +optional
	UpdateStrategy UpdateStrategy `json:"updateStrategy,omitempty"`
}

type ConsensusMember struct {
	// Name, role name.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=leader
	Name string `json:"name"`

	// AccessMode, what service this member capable.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=ReadWrite
	// +kubebuilder:validation:Enum={None, Readonly, ReadWrite}
	AccessMode AccessMode `json:"accessMode"`

	// Replicas, number of Pods of this role.
	// default 1 for Leader
	// default 0 for Learner
	// default Components[*].Replicas - Leader.Replicas - Learner.Replicas for Followers
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories={dbaas},scope=Cluster,shortName=cd
//+kubebuilder:printcolumn:name="MAIN-COMPONENT-TYPE",type="string",JSONPath=".spec.components[0].typeName",description="main component types"
//+kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
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

func init() {
	SchemeBuilder.Register(&ClusterDefinition{}, &ClusterDefinitionList{})
}

// ValidateEnabledLogConfigs validates enabledLogs against component typeName, and returns the invalid logNames undefined in ClusterDefinition.
func (r *ClusterDefinition) ValidateEnabledLogConfigs(typeName string, enabledLogs []string) []string {
	invalidLogNames := make([]string, 0, len(enabledLogs))
	logTypes := make(map[string]struct{})
	for _, comp := range r.Spec.Components {
		if !strings.EqualFold(typeName, comp.TypeName) {
			continue
		}
		for _, logConfig := range comp.LogConfigs {
			logTypes[logConfig.Name] = struct{}{}
		}
	}
	// imply that all values in enabledLogs config are invalid.
	if len(logTypes) == 0 {
		return enabledLogs
	}
	for _, name := range enabledLogs {
		if _, ok := logTypes[name]; !ok {
			invalidLogNames = append(invalidLogNames, name)
		}
	}
	return invalidLogNames
}
