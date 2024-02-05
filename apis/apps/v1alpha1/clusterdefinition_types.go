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
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

// ClusterDefinitionSpec defines the desired state of ClusterDefinition
type ClusterDefinitionSpec struct {

	// Cluster definition type defines well known application cluster type, e.g. mysql/redis/mongodb
	// +kubebuilder:validation:MaxLength=24
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
	// +optional
	Type string `json:"type,omitempty"`

	// componentDefs provides cluster components definitions.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ComponentDefs []ClusterComponentDefinition `json:"componentDefs" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Connection credential template used for creating a connection credential
	// secret for cluster.apps.kubeblocks.io object.
	//
	// Built-in objects are:
	// - `$(RANDOM_PASSWD)` - random 8 characters.
	// - `$(STRONG_RANDOM_PASSWD)` - random 16 characters, with mixed cases, digits and symbols.
	// - `$(UUID)` - generate a random UUID v4 string.
	// - `$(UUID_B64)` - generate a random UUID v4 BASE64 encoded string.
	// - `$(UUID_STR_B64)` - generate a random UUID v4 string then BASE64 encoded.
	// - `$(UUID_HEX)` - generate a random UUID v4 HEX representation.
	// - `$(HEADLESS_SVC_FQDN)` - headless service FQDN placeholder, value pattern - $(CLUSTER_NAME)-$(1ST_COMP_NAME)-headless.$(NAMESPACE).svc,
	//    where 1ST_COMP_NAME is the 1st component that provide `ClusterDefinition.spec.componentDefs[].service` attribute;
	// - `$(SVC_FQDN)` - service FQDN  placeholder, value pattern - $(CLUSTER_NAME)-$(1ST_COMP_NAME).$(NAMESPACE).svc,
	//    where 1ST_COMP_NAME is the 1st component that provide `ClusterDefinition.spec.componentDefs[].service` attribute;
	// - `$(SVC_PORT_{PORT-NAME})` - a ServicePort's port value with specified port name, i.e, a servicePort JSON struct:
	//    `{"name": "mysql", "targetPort": "mysqlContainerPort", "port": 3306}`, and "$(SVC_PORT_mysql)" in the
	//    connection credential value is 3306.
	// +optional
	ConnectionCredential map[string]string `json:"connectionCredential,omitempty"`
}

// SystemAccountSpec specifies information to create system accounts.
type SystemAccountSpec struct {
	// cmdExecutorConfig configs how to get client SDK and perform statements.
	// +kubebuilder:validation:Required
	CmdExecutorConfig *CmdExecutorConfig `json:"cmdExecutorConfig"`
	// passwordConfig defines the pattern to generate password.
	// +kubebuilder:validation:Required
	PasswordConfig PasswordConfig `json:"passwordConfig"`
	// accounts defines system account config settings.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Accounts []SystemAccountConfig `json:"accounts" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`
}

// CmdExecutorConfig specifies how to perform creation and deletion statements.
type CmdExecutorConfig struct {
	CommandExecutorEnvItem `json:",inline"`
	CommandExecutorItem    `json:",inline"`
}

// PasswordConfig helps provide to customize complexity of password generation pattern.
type PasswordConfig struct {
	// length defines the length of password.
	// +kubebuilder:validation:Maximum=32
	// +kubebuilder:validation:Minimum=8
	// +kubebuilder:default=16
	// +optional
	Length int32 `json:"length,omitempty"`
	//  numDigits defines number of digits.
	// +kubebuilder:validation:Maximum=8
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=4
	// +optional
	NumDigits int32 `json:"numDigits,omitempty"`
	// numSymbols defines number of symbols.
	// +kubebuilder:validation:Maximum=8
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	NumSymbols int32 `json:"numSymbols,omitempty"`
	// letterCase defines to use lower-cases, upper-cases or mixed-cases of letters.
	// +kubebuilder:default=MixedCases
	// +optional
	LetterCase LetterCase `json:"letterCase,omitempty"`
	// seed specifies the seed used to generate the account's password.
	// Cannot be updated.
	// +optional
	Seed string `json:"seed,omitempty"`
}

// SystemAccountConfig specifies how to create and delete system accounts.
type SystemAccountConfig struct {
	// name is the name of a system account.
	// +kubebuilder:validation:Required
	Name AccountName `json:"name"`
	// provisionPolicy defines how to create account.
	// +kubebuilder:validation:Required
	ProvisionPolicy ProvisionPolicy `json:"provisionPolicy"`
}

// ProvisionPolicy defines the policy details for creating accounts.
type ProvisionPolicy struct {
	// type defines the way to provision an account, either `CreateByStmt` or `ReferToExisting`.
	// +kubebuilder:validation:Required
	Type ProvisionPolicyType `json:"type"`
	// scope is the scope to provision account, and the scope could be `AnyPods` or `AllPods`.
	// +kubebuilder:default=AnyPods
	Scope ProvisionScope `json:"scope"`
	// statements will be used when Type is CreateByStmt.
	// +optional
	Statements *ProvisionStatements `json:"statements,omitempty"`
	// secretRef will be used when Type is ReferToExisting.
	// +optional
	SecretRef *ProvisionSecretRef `json:"secretRef,omitempty"`
}

// ProvisionSecretRef defines the information of secret referred to.
type ProvisionSecretRef struct {
	// name refers to the name of the secret.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// namespace refers to the namespace of the secret.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

// ProvisionStatements defines the statements used to create accounts.
type ProvisionStatements struct {
	// creation specifies statement how to create this account with required privileges.
	// +kubebuilder:validation:Required
	CreationStatement string `json:"creation"`
	// update specifies statement how to update account's password.
	// +optional
	UpdateStatement string `json:"update,omitempty"`
	// deletion specifies statement how to delete this account.
	// Used in combination with `CreateionStatement` to delete the account before create it.
	// For instance, one usually uses `drop user if exists` statement followed by `create user` statement to create an account.
	// Deprecated: this field is deprecated, use `update` instead.
	// +optional
	DeletionStatement string `json:"deletion,omitempty"`
}

// ClusterDefinitionStatus defines the observed state of ClusterDefinition
type ClusterDefinitionStatus struct {
	// ClusterDefinition phase, valid values are `empty`, `Available`, 'Unavailable`.
	// Available is ClusterDefinition become available, and can be referenced for co-related objects.
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

func (r ClusterDefinitionStatus) GetTerminalPhases() []Phase {
	return []Phase{AvailablePhase}
}

type ExporterConfig struct {
	// scrapePort is exporter port for Time Series Database to scrape metrics.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XIntOrString
	ScrapePort intstr.IntOrString `json:"scrapePort"`

	// scrapePath is exporter url path for Time Series Database to scrape metrics.
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:default="/metrics"
	// +optional
	ScrapePath string `json:"scrapePath,omitempty"`
}

type MonitorConfig struct {
	// builtIn is a switch to enable KubeBlocks builtIn monitoring.
	// If BuiltIn is set to true, monitor metrics will be scraped automatically.
	// If BuiltIn is set to false, the provider should set ExporterConfig and Sidecar container own.
	// +kubebuilder:default=false
	// +optional
	BuiltIn bool `json:"builtIn,omitempty"`

	// exporterConfig provided by provider, which specify necessary information to Time Series Database.
	// exporterConfig is valid when builtIn is false.
	// +optional
	Exporter *ExporterConfig `json:"exporterConfig,omitempty"`
}

type LogConfig struct {
	// name log type name, such as slow for MySQL slow log file.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	Name string `json:"name"`

	// filePathPattern log file path pattern which indicate how to find this file
	// corresponding to variable (log path) in database kernel. please don't set this casually.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=4096
	FilePathPattern string `json:"filePathPattern"`
}

type VolumeTypeSpec struct {
	// name definition is the same as the name of the VolumeMounts field in PodSpec.Container,
	// similar to the relations of Volumes[*].name and VolumesMounts[*].name in Pod.Spec.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// type is in enum of {data, log}.
	// VolumeTypeData: the volume is for the persistent data storage.
	// VolumeTypeLog: the volume is for the persistent log storage.
	// +optional
	Type VolumeType `json:"type,omitempty"`
}

type VolumeProtectionSpec struct {
	// The high watermark threshold for volume space usage.
	// If there is any specified volumes who's space usage is over the threshold, the pre-defined "LOCK" action
	// will be triggered to degrade the service to protect volume from space exhaustion, such as to set the instance
	// as read-only. And after that, if all volumes' space usage drops under the threshold later, the pre-defined
	// "UNLOCK" action will be performed to recover the service normally.
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=90
	// +optional
	HighWatermark int `json:"highWatermark,omitempty"`

	// Volumes to protect.
	// +optional
	Volumes []ProtectedVolume `json:"volumes,omitempty"`
}

type ProtectedVolume struct {
	// Name of volume to protect.
	// +optional
	Name string `json:"name,omitempty"`

	// Volume specified high watermark threshold, it will override the component level threshold.
	// If the value is invalid, it will be ignored and the component level threshold will be used.
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	// +optional
	HighWatermark *int `json:"highWatermark,omitempty"`
}

type ServiceRefDeclaration struct {
	// The name of the service reference declaration.
	// The service reference can come from an external service that is not part of KubeBlocks, or services provided by other KubeBlocks Cluster objects.
	// The specific type of service reference depends on the binding declaration when creates a Cluster.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// serviceRefDeclarationSpecs is a collection of service descriptions for a service reference declaration.
	// Each ServiceRefDeclarationSpec defines a service Kind and Version. When multiple ServiceRefDeclarationSpecs are defined,
	// it indicates that the ServiceRefDeclaration can be any one of the specified ServiceRefDeclarationSpecs.
	// For example, when the ServiceRefDeclaration is declared to require an OLTP database, which can be either MySQL or PostgreSQL,
	// you can define a ServiceRefDeclarationSpec for MySQL and another ServiceRefDeclarationSpec for PostgreSQL,
	// when referencing the service within the cluster, as long as the serviceKind and serviceVersion match either MySQL or PostgreSQL, it can be used.
	// +kubebuilder:validation:Required
	ServiceRefDeclarationSpecs []ServiceRefDeclarationSpec `json:"serviceRefDeclarationSpecs"`
}

type ServiceRefDeclarationSpec struct {
	// service kind, indicating the type or nature of the service. It should be well-known application cluster type, e.g. {mysql, redis, mongodb}.
	// The serviceKind is case-insensitive and supports abbreviations for some well-known databases.
	// For example, both 'zk' and 'zookeeper' will be considered as a ZooKeeper cluster, and 'pg', 'postgres', 'postgresql' will all be considered as a PostgreSQL cluster.
	// +kubebuilder:validation:Required
	ServiceKind string `json:"serviceKind"`

	// The service version of the service reference. It is a regular expression that matches a version number pattern.
	// For example, `^8.0.8$`, `8.0.\d{1,2}$`, `^[v\-]*?(\d{1,2}\.){0,3}\d{1,2}$`
	// +kubebuilder:validation:Required
	ServiceVersion string `json:"serviceVersion"`
}

// ClusterComponentDefinition provides a workload component specification template,
// with attributes that strongly work with stateful workloads and day-2 operations
// behaviors.
// +kubebuilder:validation:XValidation:rule="has(self.workloadType) && self.workloadType == 'Consensus' ? (has(self.consensusSpec) || has(self.rsmSpec)) : !has(self.consensusSpec)",message="componentDefs.consensusSpec(deprecated) or componentDefs.rsmSpec(recommended) is required when componentDefs.workloadType is Consensus, and forbidden otherwise"
type ClusterComponentDefinition struct {
	// A component definition name, this name could be used as default name of `Cluster.spec.componentSpecs.name`,
	// and so this name is need to conform with same validation rules as `Cluster.spec.componentSpecs.name`, that
	// is currently comply with IANA Service Naming rule. This name will apply to "apps.kubeblocks.io/component-name"
	// object label value.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// The description of component definition.
	// +optional
	Description string `json:"description,omitempty"`

	// workloadType defines type of the workload.
	// Stateless is a stateless workload type used to describe stateless applications.
	// Stateful is a stateful workload type used to describe common stateful applications.
	// Consensus is a stateful workload type used to describe applications based on consensus protocols, common consensus protocols such as raft and paxos.
	// Replication is a stateful workload type used to describe applications based on the primary-secondary data replication protocol.
	// +kubebuilder:validation:Required
	WorkloadType WorkloadType `json:"workloadType"`

	// characterType defines well-known database component name, such as mongos(mongodb), proxy(redis), mariadb(mysql)
	// KubeBlocks will generate proper monitor configs for well-known characterType when builtIn is true.
	//
	// CharacterType will also be used in role probe to decide which probe engine to use.
	// current available candidates are: mysql, postgres, mongodb, redis, etcd, kafka.
	// +optional
	CharacterType string `json:"characterType,omitempty"`

	// The configSpec field provided by provider, and
	// finally this configTemplateRefs will be rendered into the user's own configuration file according to the user's cluster.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ConfigSpecs []ComponentConfigSpec `json:"configSpecs,omitempty"`

	// The scriptSpec field provided by provider, and
	// finally this configTemplateRefs will be rendered into the user's own configuration file according to the user's cluster.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ScriptSpecs []ComponentTemplateSpec `json:"scriptSpecs,omitempty"`

	// probes setting for healthy checks.
	// +optional
	Probes *ClusterDefinitionProbes `json:"probes,omitempty"`

	// monitor is monitoring config which provided by provider.
	// +optional
	Monitor *MonitorConfig `json:"monitor,omitempty"`

	// logConfigs is detail log file config which provided by provider.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	LogConfigs []LogConfig `json:"logConfigs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// podSpec define pod spec template of the cluster component.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	PodSpec *corev1.PodSpec `json:"podSpec,omitempty"`

	// service defines the behavior of a service spec.
	// provide read-write service when WorkloadType is Consensus.
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// statelessSpec defines stateless related spec if workloadType is Stateless.
	// +optional
	//+kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.7.0, use RSMSpec instead."
	StatelessSpec *StatelessSetSpec `json:"statelessSpec,omitempty"`

	// statefulSpec defines stateful related spec if workloadType is Stateful.
	// +optional
	//+kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.7.0, use RSMSpec instead."
	StatefulSpec *StatefulSetSpec `json:"statefulSpec,omitempty"`

	// consensusSpec defines consensus related spec if workloadType is Consensus, required if workloadType is Consensus.
	// +optional
	//+kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.7.0, use RSMSpec instead."
	ConsensusSpec *ConsensusSetSpec `json:"consensusSpec,omitempty"`

	// replicationSpec defines replication related spec if workloadType is Replication.
	// +optional
	//+kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.7.0, use RSMSpec instead."
	ReplicationSpec *ReplicationSetSpec `json:"replicationSpec,omitempty"`

	// RSMSpec defines workload related spec of this component.
	// start from KB 0.7.0, RSM(ReplicatedStateMachineSpec) will be the underlying CR which powers all kinds of workload in KB.
	// RSM is an enhanced stateful workload extension dedicated for heavy-state workloads like databases.
	// +optional
	RSMSpec *RSMSpec `json:"rsmSpec,omitempty"`

	// horizontalScalePolicy controls the behavior of horizontal scale.
	// +optional
	HorizontalScalePolicy *HorizontalScalePolicy `json:"horizontalScalePolicy,omitempty"`

	// Statement to create system account.
	// +optional
	SystemAccounts *SystemAccountSpec `json:"systemAccounts,omitempty"`

	// volumeTypes is used to describe the purpose of the volumes
	// mapping the name of the VolumeMounts in the PodSpec.Container field,
	// such as data volume, log volume, etc.
	// When backing up the volume, the volume can be correctly backed up
	// according to the volumeType.
	//
	// For example:
	//  `name: data, type: data` means that the volume named `data` is used to store `data`.
	//  `name: binlog, type: log` means that the volume named `binlog` is used to store `log`.
	//
	// NOTE:
	//   When volumeTypes is not defined, the backup function will not be supported,
	// even if a persistent volume has been specified.
	// +listType=map
	// +listMapKey=name
	// +optional
	VolumeTypes []VolumeTypeSpec `json:"volumeTypes,omitempty"`

	// customLabelSpecs is used for custom label tags which you want to add to the component resources.
	// +listType=map
	// +listMapKey=key
	// +optional
	CustomLabelSpecs []CustomLabelSpec `json:"customLabelSpecs,omitempty"`

	// switchoverSpec defines command to do switchover.
	// in particular, when workloadType=Replication, the command defined in switchoverSpec will only be executed under the condition of cluster.componentSpecs[x].SwitchPolicy.type=Noop.
	// +optional
	SwitchoverSpec *SwitchoverSpec `json:"switchoverSpec,omitempty"`

	// postStartSpec defines the command to be executed when the component is ready, and the command will only be executed once after the component becomes ready.
	// +optional
	PostStartSpec *PostStartAction `json:"postStartSpec,omitempty"`

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

	// serviceRefDeclarations is used to declare the service reference of the current component.
	// +optional
	ServiceRefDeclarations []ServiceRefDeclaration `json:"serviceRefDeclarations,omitempty"`
}

func (r *ClusterComponentDefinition) GetStatefulSetWorkload() StatefulSetWorkload {
	switch r.WorkloadType {
	case Stateless:
		return nil
	case Stateful:
		return r.StatefulSpec
	case Consensus:
		return r.ConsensusSpec
	case Replication:
		return r.ReplicationSpec
	}
	panic("unreachable")
}

func (r *ClusterComponentDefinition) IsStatelessWorkload() bool {
	return r.WorkloadType == Stateless
}

func (r *ClusterComponentDefinition) GetCommonStatefulSpec() (*StatefulSetSpec, error) {
	if r.IsStatelessWorkload() {
		return nil, ErrWorkloadTypeIsStateless
	}
	switch r.WorkloadType {
	case Stateful:
		return r.StatefulSpec, nil
	case Consensus:
		if r.ConsensusSpec != nil {
			return &r.ConsensusSpec.StatefulSetSpec, nil
		}
	case Replication:
		if r.ReplicationSpec != nil {
			return &r.ReplicationSpec.StatefulSetSpec, nil
		}
	default:
		panic("unreachable")
		// return nil, ErrWorkloadTypeIsUnknown
	}
	return nil, nil
}

type ServiceSpec struct {
	// The list of ports that are exposed by this service.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies
	// +patchMergeKey=port
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=port
	// +listMapKey=protocol
	// +optional
	Ports []ServicePort `json:"ports,omitempty" patchStrategy:"merge" patchMergeKey:"port" protobuf:"bytes,1,rep,name=ports"`

	// NOTES: name also need to be key
}

func (r *ServiceSpec) ToSVCPorts() []corev1.ServicePort {
	ports := make([]corev1.ServicePort, 0, len(r.Ports))
	for _, p := range r.Ports {
		ports = append(ports, p.toSVCPort())
	}
	return ports
}

func (r ServiceSpec) ToSVCSpec() corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Ports: r.ToSVCPorts(),
	}
}

type ServicePort struct {
	// The name of this port within the service. This must be a DNS_LABEL.
	// All ports within a ServiceSpec must have unique names. When considering
	// the endpoints for a Service, this must match the 'name' field in the
	// EndpointPort.
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// The IP protocol for this port. Supports "TCP", "UDP", and "SCTP".
	// Default is TCP.
	// +kubebuilder:validation:Enum={TCP,UDP,SCTP}
	// +default="TCP"
	// +optional
	Protocol corev1.Protocol `json:"protocol,omitempty" protobuf:"bytes,2,opt,name=protocol,casttype=Protocol"`

	// The application protocol for this port.
	// This field follows standard Kubernetes label syntax.
	// Un-prefixed names are reserved for IANA standard service names (as per
	// RFC-6335 and https://www.iana.org/assignments/service-names).
	// Non-standard protocols should use prefixed names such as
	// mycompany.com/my-custom-protocol.
	// +optional
	AppProtocol *string `json:"appProtocol,omitempty" protobuf:"bytes,6,opt,name=appProtocol"`

	// The port that will be exposed by this service.
	Port int32 `json:"port" protobuf:"varint,3,opt,name=port"`

	// Number or name of the port to access on the pods targeted by the service.
	// Number must be in the range 1 to 65535. Name must be an IANA_SVC_NAME.
	// If this is a string, it will be looked up as a named port in the
	// target Pod's container ports. If this is not specified, the value
	// of the 'port' field is used (an identity map).
	// This field is ignored for services with clusterIP=None, and should be
	// omitted or set equal to the 'port' field.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#defining-a-service
	// +kubebuilder:validation:XIntOrString
	// +optional
	TargetPort intstr.IntOrString `json:"targetPort,omitempty" protobuf:"bytes,4,opt,name=targetPort"`
}

func (r *ServicePort) toSVCPort() corev1.ServicePort {
	return corev1.ServicePort{
		Name:        r.Name,
		Protocol:    r.Protocol,
		AppProtocol: r.AppProtocol,
		Port:        r.Port,
		TargetPort:  r.TargetPort,
	}
}

type HorizontalScalePolicy struct {
	// type controls what kind of data synchronization do when component scale out.
	// Policy is in enum of {None, CloneVolume}. The default policy is `None`.
	// None: Default policy, create empty volume and no data clone.
	// CloneVolume: Do data clone to newly scaled pods. Prefer to use volume snapshot first,
	//         and will try backup tool if volume snapshot is not enabled, finally
	// 	       report error if both above cannot work.
	// Snapshot: Deprecated, alias for CloneVolume.
	// +kubebuilder:default=None
	// +optional
	Type HScaleDataClonePolicyType `json:"type,omitempty"`

	// BackupPolicyTemplateName reference the backup policy template.
	// +optional
	BackupPolicyTemplateName string `json:"backupPolicyTemplateName,omitempty"`

	// volumeMountsName defines which volumeMount of the container to do backup,
	// only work if Type is not None
	// if not specified, the 1st volumeMount will be chosen
	// +optional
	VolumeMountsName string `json:"volumeMountsName,omitempty"`
}

type ClusterDefinitionProbeCMDs struct {
	// Write check executed on probe sidecar, used to check workload's allow write access.
	// +optional
	Writes []string `json:"writes,omitempty"`

	// Read check executed on probe sidecar, used to check workload's readonly access.
	// +optional
	Queries []string `json:"queries,omitempty"`
}

type ClusterDefinitionProbe struct {
	// How often (in seconds) to perform the probe.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`

	// Number of seconds after which the probe times out. Defaults to 1 second.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`

	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=2
	FailureThreshold int32 `json:"failureThreshold,omitempty"`

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
	//+kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.7.0, use RSMSpec instead."
	RoleProbe *ClusterDefinitionProbe `json:"roleProbe,omitempty"`

	// roleProbeTimeoutAfterPodsReady(in seconds), when all pods of the component are ready,
	// it will detect whether the application is available in the pod.
	// if pods exceed the InitializationTimeoutSeconds time without a role label,
	// this component will enter the Failed/Abnormal phase.
	// Note that this configuration will only take effect if the component supports RoleProbe
	// and will not affect the life cycle of the pod. default values are 60 seconds.
	// +optional
	// +kubebuilder:validation:Minimum=30
	RoleProbeTimeoutAfterPodsReady int32 `json:"roleProbeTimeoutAfterPodsReady,omitempty"`
}

type StatelessSetSpec struct {
	// updateStrategy defines the underlying deployment strategy to use to replace existing pods with new ones.
	// +optional
	// +patchStrategy=retainKeys
	UpdateStrategy appsv1.DeploymentStrategy `json:"updateStrategy,omitempty"`
}

type StatefulSetSpec struct {
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

	// llPodManagementPolicy is the low-level controls how pods are created during initial scale up,
	// when replacing pods on nodes, or when scaling down.
	// `OrderedReady` policy specify where pods are created in increasing order (pod-0, then
	// pod-1, etc) and the controller will wait until each pod is ready before
	// continuing. When scaling down, the pods are removed in the opposite order.
	// `Parallel` policy specify create pods in parallel
	// to match the desired scale without waiting, and on scale down will delete
	// all pods at once.
	// +optional
	LLPodManagementPolicy appsv1.PodManagementPolicyType `json:"llPodManagementPolicy,omitempty"`

	// llUpdateStrategy indicates the low-level StatefulSetUpdateStrategy that will be
	// employed to update Pods in the StatefulSet when a revision is made to
	// Template. Will ignore `updateStrategy` attribute if provided.
	// +optional
	LLUpdateStrategy *appsv1.StatefulSetUpdateStrategy `json:"llUpdateStrategy,omitempty"`
}

var _ StatefulSetWorkload = &StatefulSetSpec{}

func (r *StatefulSetSpec) GetUpdateStrategy() UpdateStrategy {
	if r == nil {
		return SerialStrategy
	}
	return r.UpdateStrategy
}

func (r *StatefulSetSpec) FinalStsUpdateStrategy() (appsv1.PodManagementPolicyType, appsv1.StatefulSetUpdateStrategy) {
	if r == nil {
		r = &StatefulSetSpec{
			UpdateStrategy: SerialStrategy,
		}
	}
	return r.finalStsUpdateStrategy()
}

func (r *StatefulSetSpec) finalStsUpdateStrategy() (appsv1.PodManagementPolicyType, appsv1.StatefulSetUpdateStrategy) {
	if r.LLUpdateStrategy != nil {
		return r.LLPodManagementPolicy, *r.LLUpdateStrategy
	}

	zeroPartition := int32(0)
	switch r.UpdateStrategy {
	case BestEffortParallelStrategy:
		m := intstr.FromString("49%")
		return appsv1.ParallelPodManagement, appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.RollingUpdateStatefulSetStrategyType,
			RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
				// explicitly set the partition as 0 to avoid update workload unexpectedly.
				Partition: &zeroPartition,
				// alpha feature since v1.24
				// ref: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#maximum-unavailable-pods
				MaxUnavailable: &m,
			},
		}
	case ParallelStrategy:
		return appsv1.ParallelPodManagement, appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.RollingUpdateStatefulSetStrategyType,
		}
	case SerialStrategy:
		fallthrough
	default:
		m := intstr.FromInt(1)
		return appsv1.OrderedReadyPodManagement, appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.RollingUpdateStatefulSetStrategyType,
			RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
				// explicitly set the partition as 0 to avoid update workload unexpectedly.
				Partition: &zeroPartition,
				// alpha feature since v1.24
				// ref: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#maximum-unavailable-pods
				MaxUnavailable: &m,
			},
		}
	}
}

type ConsensusSetSpec struct {
	StatefulSetSpec `json:",inline"`

	// leader, one single leader.
	// +kubebuilder:validation:Required
	Leader ConsensusMember `json:"leader"`

	// followers, has voting right but not Leader.
	// +optional
	Followers []ConsensusMember `json:"followers,omitempty"`

	// learner, no voting right.
	// +optional
	Learner *ConsensusMember `json:"learner,omitempty"`
}

var _ StatefulSetWorkload = &ConsensusSetSpec{}

func (r *ConsensusSetSpec) GetUpdateStrategy() UpdateStrategy {
	if r == nil {
		return SerialStrategy
	}
	return r.UpdateStrategy
}

func (r *ConsensusSetSpec) FinalStsUpdateStrategy() (appsv1.PodManagementPolicyType, appsv1.StatefulSetUpdateStrategy) {
	if r == nil {
		r = NewConsensusSetSpec()
	}
	if r.LLUpdateStrategy != nil {
		return r.LLPodManagementPolicy, *r.LLUpdateStrategy
	}
	_, s := r.StatefulSetSpec.finalStsUpdateStrategy()
	// switch r.UpdateStrategy {
	// case SerialStrategy, BestEffortParallelStrategy:
	s.Type = appsv1.OnDeleteStatefulSetStrategyType
	s.RollingUpdate = nil
	// }
	return appsv1.ParallelPodManagement, s
}

func NewConsensusSetSpec() *ConsensusSetSpec {
	return &ConsensusSetSpec{
		Leader: DefaultLeader,
		StatefulSetSpec: StatefulSetSpec{
			UpdateStrategy: SerialStrategy,
		},
	}
}

type ConsensusMember struct {
	// name, role name.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=leader
	Name string `json:"name"`

	// accessMode, what service this member capable.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=ReadWrite
	AccessMode AccessMode `json:"accessMode"`

	// replicas, number of Pods of this role.
	// default 1 for Leader
	// default 0 for Learner
	// default Cluster.spec.componentSpec[*].Replicas - Leader.Replicas - Learner.Replicas for Followers
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
}

type RSMSpec struct {
	// Roles, a list of roles defined in the system.
	// +optional
	Roles []workloads.ReplicaRole `json:"roles,omitempty"`

	// RoleProbe provides method to probe role.
	// +optional
	RoleProbe *workloads.RoleProbe `json:"roleProbe,omitempty"`

	// MembershipReconfiguration provides actions to do membership dynamic reconfiguration.
	// +optional
	MembershipReconfiguration *workloads.MembershipReconfiguration `json:"membershipReconfiguration,omitempty"`

	// MemberUpdateStrategy, Members(Pods) update strategy.
	// serial: update Members one by one that guarantee minimum component unavailable time.
	// 		Learner -> Follower(with AccessMode=none) -> Follower(with AccessMode=readonly) -> Follower(with AccessMode=readWrite) -> Leader
	// bestEffortParallel: update Members in parallel that guarantee minimum component un-writable time.
	//		Learner, Follower(minority) in parallel -> Follower(majority) -> Leader, keep majority online all the time.
	// parallel: force parallel
	// +kubebuilder:validation:Enum={Serial,BestEffortParallel,Parallel}
	// +optional
	MemberUpdateStrategy *workloads.MemberUpdateStrategy `json:"memberUpdateStrategy,omitempty"`
}

type ReplicationSetSpec struct {
	StatefulSetSpec `json:",inline"`
}

var _ StatefulSetWorkload = &ReplicationSetSpec{}

func (r *ReplicationSetSpec) GetUpdateStrategy() UpdateStrategy {
	if r == nil {
		return SerialStrategy
	}
	return r.UpdateStrategy
}

func (r *ReplicationSetSpec) FinalStsUpdateStrategy() (appsv1.PodManagementPolicyType, appsv1.StatefulSetUpdateStrategy) {
	if r == nil {
		r = &ReplicationSetSpec{}
	}
	if r.LLUpdateStrategy != nil {
		return r.LLPodManagementPolicy, *r.LLUpdateStrategy
	}
	_, s := r.StatefulSetSpec.finalStsUpdateStrategy()
	s.Type = appsv1.OnDeleteStatefulSetStrategyType
	s.RollingUpdate = nil
	return appsv1.ParallelPodManagement, s
}

type PostStartAction struct {
	// cmdExecutorConfig is the executor configuration of the post-start command.
	// +kubebuilder:validation:Required
	CmdExecutorConfig CmdExecutorConfig `json:"cmdExecutorConfig"`

	// scriptSpecSelectors defines the selector of the scriptSpecs that need to be referenced.
	// Once ScriptSpecSelectors is defined, the scripts defined in scriptSpecs can be referenced in the PostStartAction.CmdExecutorConfig.
	// +optional
	ScriptSpecSelectors []ScriptSpecSelector `json:"scriptSpecSelectors,omitempty"`
}

type SwitchoverSpec struct {
	// withCandidate corresponds to the switchover of the specified candidate primary or leader instance.
	// +optional
	WithCandidate *SwitchoverAction `json:"withCandidate,omitempty"`

	// withoutCandidate corresponds to a switchover that does not specify a candidate primary or leader instance.
	// +optional
	WithoutCandidate *SwitchoverAction `json:"withoutCandidate,omitempty"`
}

type SwitchoverAction struct {
	// cmdExecutorConfig is the executor configuration of the switchover command.
	// +kubebuilder:validation:Required
	CmdExecutorConfig *CmdExecutorConfig `json:"cmdExecutorConfig"`

	// scriptSpecSelectors defines the selector of the scriptSpecs that need to be referenced.
	// Once ScriptSpecSelectors is defined, the scripts defined in scriptSpecs can be referenced in the SwitchoverAction.CmdExecutorConfig.
	// +optional
	ScriptSpecSelectors []ScriptSpecSelector `json:"scriptSpecSelectors,omitempty"`
}

type ScriptSpecSelector struct {
	// ScriptSpec name of the referent, refer to componentDefs[x].scriptSpecs[y].Name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`
}

type CommandExecutorEnvItem struct {
	// image for Connector when executing the command.
	// +kubebuilder:validation:Required
	Image string `json:"image"`
	// envs is a list of environment variables.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

type CommandExecutorItem struct {
	// command to perform statements.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Command []string `json:"command"`
	// args is used to perform statements.
	// +optional
	Args []string `json:"args,omitempty"`
}

type CustomLabelSpec struct {
	// key name of label
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// value of label
	// +kubebuilder:validation:Required
	Value string `json:"value"`

	// resources defines the resources to be labeled.
	// +kubebuilder:validation:Required
	Resources []GVKResource `json:"resources,omitempty"`
}

type GVKResource struct {
	// gvk is Group/Version/Kind, for example "v1/Pod", "apps/v1/StatefulSet", etc.
	// when the gvk resource filtered by the selector already exists, if there is no corresponding custom label, it will be added, and if label already exists, it will be updated.
	// +kubebuilder:validation:Required
	GVK string `json:"gvk"`

	// selector is a label query over a set of resources.
	// +optional
	Selector map[string]string `json:"selector,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=cd
// +kubebuilder:printcolumn:name="MAIN-COMPONENT-NAME",type="string",JSONPath=".spec.componentDefs[0].name",description="main component names"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ClusterDefinition is the Schema for the clusterdefinitions API
type ClusterDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterDefinitionSpec   `json:"spec,omitempty"`
	Status ClusterDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterDefinitionList contains a list of ClusterDefinition
type ClusterDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterDefinition{}, &ClusterDefinitionList{})
}

// ValidateEnabledLogConfigs validates enabledLogs against component compDefName, and returns the invalid logNames undefined in ClusterDefinition.
func (r *ClusterDefinition) ValidateEnabledLogConfigs(compDefName string, enabledLogs []string) []string {
	invalidLogNames := make([]string, 0, len(enabledLogs))
	logTypes := make(map[string]struct{})
	for _, comp := range r.Spec.ComponentDefs {
		if !strings.EqualFold(compDefName, comp.Name) {
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

// GetComponentDefByName gets component definition from ClusterDefinition with compDefName
func (r *ClusterDefinition) GetComponentDefByName(compDefName string) *ClusterComponentDefinition {
	for _, component := range r.Spec.ComponentDefs {
		if component.Name == compDefName {
			return &component
		}
	}
	return nil
}

// FailurePolicyType specifies the type of failure policy
// +enum
// +kubebuilder:validation:Enum={Ignore,Fail}
type FailurePolicyType string

const (
	// Ignore means that an error will be ignored but logged.
	FailurePolicyIgnore FailurePolicyType = "Ignore"
	// ReportError means that an error will be reported.
	FailurePolicyFail FailurePolicyType = "Fail"
)

// ComponentValueFromType specifies the type of component value from.
// +enum
// +kubebuilder:validation:Enum={FieldRef,ServiceRef,HeadlessServiceRef}
type ComponentValueFromType string

const (
	FromFieldRef           ComponentValueFromType = "FieldRef"
	FromServiceRef         ComponentValueFromType = "ServiceRef"
	FromHeadlessServiceRef ComponentValueFromType = "HeadlessServiceRef"
)

// ComponentDefRef is used to select the component and its fields to be referenced.
type ComponentDefRef struct {
	// componentDefName is the name of the componentDef to select.
	// +kubebuilder:validation:Required
	ComponentDefName string `json:"componentDefName"`
	// failurePolicy is the failure policy of the component.
	// If failed to find the component, the failure policy will be used.
	// +kubebuilder:validation:Enum={Ignore,Fail}
	// +default="Ignore"
	// +optional
	FailurePolicy FailurePolicyType `json:"failurePolicy,omitempty"`
	// componentRefEnv specifies a list of values to be injected as env variables to each component.
	// +kbubebuilder:validation:Required
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ComponentRefEnvs []ComponentRefEnv `json:"componentRefEnv" patchStrategy:"merge" patchMergeKey:"name"`
}

// ComponentRefEnv specifies name and value of an env.
type ComponentRefEnv struct {
	// name is the name of the env to be injected, and it must be a C identifier.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[A-Za-z_][A-Za-z0-9_]*$`
	Name string `json:"name"`
	// value is the value of the env to be injected.
	// +optional
	Value string `json:"value,omitempty"`
	// valueFrom specifies the source of the env to be injected.
	// +optional
	ValueFrom *ComponentValueFrom `json:"valueFrom,omitempty"`
}

type ComponentValueFrom struct {
	// type is the type of the source to select. There are three types: `FieldRef`, `ServiceRef`, `HeadlessServiceRef`.
	// +kubebuilder:validation:Enum={FieldRef,ServiceRef,HeadlessServiceRef}
	// +kubebuilder:validation:Required
	Type ComponentValueFromType `json:"type"`
	// fieldRef is the jsonpath of the source to select when type is `FieldRef`.
	// there are two objects registered in the jsonpath: `componentDef` and `components`.
	// componentDef is the component definition object specified in `componentRef.componentDefName`.
	// components is the component list objects referring to the component definition object.
	// +optional
	FieldPath string `json:"fieldPath,omitempty"`
	// format is the format of each headless service address.
	// there are three builtin variables can be used as placeholder: $POD_ORDINAL, $POD_FQDN, $POD_NAME
	// $POD_ORDINAL is the ordinal of the pod.
	// $POD_FQDN is the fully qualified domain name of the pod.
	// $POD_NAME is the name of the pod
	// +optional
	// +kubebuilder:default=="$POD_FQDN"
	Format string `json:"format,omitempty"`
	// joinWith is the string to join the values of headless service addresses.
	// +optional
	// +kubebuilder:default=","
	JoinWith string `json:"joinWith,omitempty"`
}
