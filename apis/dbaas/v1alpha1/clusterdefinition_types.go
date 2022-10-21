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

	// Strategies such as order of every component in operation
	// +optional
	Cluster *ClusterDefinitionCluster `json:"cluster,omitempty"`

	// List of components belonging to the cluster
	// +kubebuilder:validation:MinItems=1
	// +optional
	Components []ClusterDefinitionComponent `json:"components,omitempty"`

	// +kubebuilder:validation:MinItems=1
	// +optional
	RoleGroupTemplates []RoleGroupTemplate `json:"roleGroupTemplates,omitempty"`

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

type ClusterDefinitionCluster struct {

	// +optional
	Strategies ClusterDefinitionStrategies `json:"strategies,omitempty"`
}

type ConfigTemplate struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	Name string `json:"name,omitempty"`

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

// ClusterDefinitionComponent is a group of pods, pods in one component usually share the same data
type ClusterDefinitionComponent struct {
	// Type name of the component, it can be any valid string
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=12
	TypeName string `json:"typeName,omitempty"`

	// CharacterType defines well-known database component name, such as mongos(mongodb), proxy(redis), wesql(mysql)
	// DBaas will generate proper monitor configs for wellknown CharacterType when BuiltIn is true.
	// +optional
	CharacterType string `json:"characterType,omitempty"`

	// roleGroups specify roleGroupTemplate name
	RoleGroups []string `json:"roleGroups,omitempty"`

	// Minimum available pod count when updating
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	MinAvailable int `json:"minAvailable,omitempty"`

	// Maximum available pod count after scale
	// +kubebuilder:validation:Minimum=0
	MaxAvailable int `json:"maxAvailable,omitempty"`

	// Default replicas in this component if user not specify
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	DefaultReplicas int `json:"defaultReplicas,omitempty"`

	// Define if this component is stateless or not
	// +kubebuilder:default=false
	IsStateless bool `json:"isStateless,omitempty"`

	// The configTemplateRefs field provided by ISV, and
	// finally this configTemplateRefs will be rendered into the user's own configuration file according to the user's cluster
	// +optional
	ConfigTemplateRefs []ConfigTemplate `json:"configTemplateRefs,omitempty"`

	// Monitor is monitoring config which provided by ISV
	// +optional
	Monitor *MonitorConfig `json:"monitor,omitempty"`

	// IsQuorum defines odd number of pods & N/2+1 pods
	// +kubebuilder:default=false
	IsQuorum bool `json:"isQuorum,omitempty"`

	// +optional
	Strategies ClusterDefinitionStrategies `json:"strategies,omitempty"`

	// podSpec of final workload
	// +optional
	PodSpec *corev1.PodSpec `json:"podSpec,omitempty"`

	// Service defines the behavior of a service spec.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Service corev1.ServiceSpec `json:"service,omitempty"`

	// Scripts executed before and after workload operation
	// script exec order：component.pre => roleGroup.pre => component.exec => roleGroup.exec => roleGroup.post => component.post
	// builtin ENV variables:
	// self: OPENDBAAS_SELF_{builtin_properties}
	// rule: OPENDBAAS_{conponent_name}[n]-{roleGroup_name}[n]-{builtin_properties}
	// builtin_properties:
	// - ID # which shows in Cluster.status
	// - HOST # e.g. example-mongodb2-0.example-mongodb2-svc.default.svc.cluster.local
	// - PORT
	// - N # number of current component/roleGroup
	// +optional
	Scripts ClusterDefinitionScripts `json:"scripts,omitempty"`
}

type ClusterDefinitionStrategies struct {
	Default         ClusterDefinitionStrategy `json:"default,omitempty"`
	Create          ClusterDefinitionStrategy `json:"create,omitempty"`
	Upgrade         ClusterDefinitionStrategy `json:"upgrade,omitempty"`
	VerticalScale   ClusterDefinitionStrategy `json:"verticalScale,omitempty"`
	HorizontalScale ClusterDefinitionStrategy `json:"horizontalScale,omitempty"`
	Delete          ClusterDefinitionStrategy `json:"delete,omitempty"`
}

type ClusterDefinitionStrategy struct {
	Order []string `json:"order,omitempty"`
}

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

type RoleGroupTemplate struct {
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
	DefaultReplicas int                             `json:"defaultReplicas,omitempty"`
	UpdateStrategy  ClusterDefinitionUpdateStrategy `json:"updateStrategy,omitempty"`
	// script exec order：component.pre => roleGroup.pre => component.exec => roleGroup.exec => roleGroup.post => component.post
	// builtin ENV variables:
	// self: OPENDBAAS_SELF_{builtin_properties}
	// rule: OPENDBAAS_{conponent_name}[n]-{roleGroup_name}[n]-{builtin_properties}
	// builtin_properties:
	// - ID # which shows in Cluster.status
	// - HOST # e.g. example-mongodb2-0.example-mongodb2-svc.default.svc.cluster.local
	// - PORT
	// - N # number of current component/roleGroup
	Scripts ClusterDefinitionScripts `json:"scripts,omitempty"`
}

type ClusterDefinitionUpdateStrategy struct {
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	MaxUnavailable int `json:"maxUnavailable,omitempty"`
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	MaxSurge int `json:"maxSurge,omitempty"`
}

type ClusterDefinitionRoleGroupScript struct {
	Pre  []ClusterDefinitionContainerCMD `json:"pre,omitempty"`
	Exec []ClusterDefinitionContainerCMD `json:"exec,omitempty"`
	Post []ClusterDefinitionContainerCMD `json:"post,omitempty"`
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

func init() {
	SchemeBuilder.Register(&ClusterDefinition{}, &ClusterDefinitionList{})
}
