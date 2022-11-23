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
	Type string `json:"type"`

	// List of components belonging to the cluster.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Components []ClusterDefinitionComponent `json:"components"`

	// Default termination policy if no termination policy defined in cluster.
	// +kubebuilder:validation:Enum={DoNotTerminate,Halt,Delete,WipeOut}
	// +optional
	DefaultTerminationPolicy string `json:"defaultTerminationPolicy,omitempty"`

	// Credential used for connecting database.
	// +optional
	ConnectionCredential *ClusterDefinitionConnectionCredential `json:"connectionCredential,omitempty"`
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
	// Specify the name of the referenced configuration template, which is a configmap object.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	Name string `json:"name"`

	// VolumeName is the volume name of PodTemplate, which the configuration file produced through the configuration template will be mounted to the corresponding volume.
	// The volume name must be defined in podSpec.containers[*].volumeMounts.
	// reference example: https://github.com/apecloud/kubeblocks/blob/main/examples/dbaas/mysql_clusterdefinition.yaml#L12
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	VolumeName string `json:"volumeName"`
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
	// Otherwise, ISV should set BuiltIn to false and provide ExporterConfig and Sidecar container own.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=true
	BuiltIn bool `json:"builtIn"`

	// Exporter provided by ISV, which specify necessary information to Time Series Database.
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

// ClusterDefinitionComponent is a group of pods, pods in one component usually share the same data
type ClusterDefinitionComponent struct {
	// Type name of the component, it can be any valid string.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=12
	TypeName string `json:"typeName"`

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

	// The configTemplateRefs field provided by ISV, and
	// finally this configTemplateRefs will be rendered into the user's own configuration file according to the user's cluster.
	// +optional
	ConfigTemplateRefs []ConfigTemplate `json:"configTemplateRefs,omitempty"`

	// Monitor is monitoring config which provided by ISV.
	// +optional
	Monitor *MonitorConfig `json:"monitor,omitempty"`

	// LogConfigs is detail log file config which provided by ISV.
	// +optional
	LogConfigs []LogConfig `json:"logConfigs,omitempty"`

	// antiAffinity defines components should have anti-affinity constraint to same component type.
	// +kubebuilder:default=false
	// +optional
	AntiAffinity bool `json:"antiAffinity,omitempty"`

	// podSpec of final workload
	// +optional
	PodSpec *corev1.PodSpec `json:"podSpec,omitempty"`

	// Service defines the behavior of a service spec.
	// provide read-write service when ComponentType is Consensus.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Service corev1.ServiceSpec `json:"service,omitempty"`

	// Probes setting for db healthy checks.
	// +optional
	Probes *ClusterDefinitionProbes `json:"probes,omitempty"`

	// ComponentType defines type of the component.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Stateless
	// +kubebuilder:validation:Enum={Stateless,Stateful,Consensus}
	ComponentType ComponentType `json:"componentType"`

	// ConsensusSpec defines consensus related spec if componentType is Consensus.
	// CAN'T be empty if componentType is Consensus.
	// +optional
	ConsensusSpec *ConsensusSetSpec `json:"consensusSpec,omitempty"`
}

type ClusterDefinitionConnectionCredential struct {
	// User defines system credential username.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=root
	User string `json:"user"`

	// Password defines system credential password.
	// +optional
	Password string `json:"password,omitempty"`
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

	// Cmds used to execute for probe.
	// +optional
	Commands *ClusterDefinitionProbeCMDs `json:"commands,omitempty"`
}

type ClusterDefinitionProbes struct {
	// Probe for db running check.
	// +optional
	RunningProbe *ClusterDefinitionProbe `json:"runningProbe,omitempty"`
	// Probe for db status check.
	// +optional
	StatusProbe *ClusterDefinitionProbe `json:"statusProbe,omitempty"`
	// Probe for db role changed check.
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

func init() {
	SchemeBuilder.Register(&ClusterDefinition{}, &ClusterDefinitionList{})
}

// ValidateEnabledLogConfigs validate enabledLogs according to component typeName, and return the invalid logNames which aren't defined in ClusterDefinition.
func (r *ClusterDefinition) ValidateEnabledLogConfigs(typeName string, enabledLogs []string) []string {
	invalidLogNames := make([]string, 0, len(enabledLogs))
	logTypes := make(map[string]bool)
	for _, comp := range r.Spec.Components {
		if !strings.EqualFold(typeName, comp.TypeName) {
			continue
		}
		for _, logConfig := range comp.LogConfigs {
			logTypes[logConfig.Name] = true
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
