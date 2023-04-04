/*
Copyright ApeCloud, Inc.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ClusterDefinitionSpec defines the desired state of ClusterDefinition
type ClusterDefinitionSpec struct {

	// componentDefs provides cluster components definitions.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ComponentDefs []ClusterComponentDefinition `json:"componentDefs" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Connection credential template used for creating a connection credential
	// secret for cluster.apps.kubeblock.io object. Built-in objects are:
	// `$(RANDOM_PASSWD)` - random 8 characters.
	// `$(UUID)` - generate a random UUID v4 string.
	// `$(UUID_B64)` - generate a random UUID v4 BASE64 encoded string``.
	// `$(UUID_STR_B64)` - generate a random UUID v4 string then BASE64 encoded``.
	// `$(UUID_HEX)` - generate a random UUID v4 wth HEX representation``.
	// `$(SVC_FQDN)` - service FQDN  placeholder, value pattern - $(CLUSTER_NAME)-$(1ST_COMP_NAME).$(NAMESPACE).svc,
	//    where 1ST_COMP_NAME is the 1st component that provide `ClusterDefinition.spec.componentDefs[].service` attribute;
	// `$(SVC_PORT_<PORT-NAME>)` - a ServicePort's port value with specified port name, i.e, a servicePort JSON struct:
	//    { "name": "mysql", "targetPort": "mysqlContainerPort", "port": 3306 }, and "$(SVC_PORT_mysql)" in the
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

	CommandExecutorItem `json:",inline"`
}

// PasswordConfig helps provide to customize complexity of password generation pattern.
type PasswordConfig struct {
	// length defines the length of password.
	// +kubebuilder:validation:Maximum=32
	// +kubebuilder:validation:Minimum=8
	// +kubebuilder:default=10
	// +optional
	Length int32 `json:"length,omitempty"`
	//  numDigits defines number of digits.
	// +kubebuilder:validation:Maximum=20
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=2
	// +optional
	NumDigits int32 `json:"numDigits,omitempty"`
	// numSymbols defines number of symbols.
	// +kubebuilder:validation:Maximum=20
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	NumSymbols int32 `json:"numSymbols,omitempty"`
	// letterCase defines to use lower-cases, upper-cases or mixed-cases of letters.
	// +kubebuilder:default=MixedCases
	// +optional
	LetterCase LetterCase `json:"letterCase,omitempty"`
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
	// scope is the scope to provision account, and the scope could be `anyPod` or `allPods`.
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
	// deletion specifies statement how to delete this account.
	// +optional
	DeletionStatement string `json:"deletion,omitempty"`
}

// ClusterDefinitionStatus defines the observed state of ClusterDefinition
type ClusterDefinitionStatus struct {
	// ClusterDefinition phase, valid values are <empty>, Available.
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

type ComponentTemplateSpec struct {
	// Specify the name of configuration template.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Specify the name of the referenced the configuration template ConfigMap object.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	TemplateRef string `json:"templateRef"`

	// Specify the namespace of the referenced the configuration template ConfigMap object.
	// An empty namespace is equivalent to the "default" namespace.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:default="default"
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// volumeName is the volume name of PodTemplate, which the configuration file produced through the configuration template will be mounted to the corresponding volume.
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

type ComponentConfigSpec struct {
	ComponentTemplateSpec `json:",inline"`

	// Specify a list of keys.
	// If empty, ConfigConstraint takes effect for all keys in configmap.
	// +optional
	Keys []string `json:"keys,omitempty"`

	// Specify the name of the referenced the configuration constraints object.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	ConfigConstraintRef string `json:"constraintRef,omitempty"`
}

type ExporterConfig struct {
	// scrapePort is exporter port for Time Series Database to scrape metrics.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:Minimum=0
	ScrapePort int32 `json:"scrapePort"`

	// scrapePath is exporter url path for Time Series Database to scrape metrics.
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:default="/metrics"
	// +optional
	ScrapePath string `json:"scrapePath,omitempty"`
}

type MonitorConfig struct {
	// builtIn is a switch to enable KubeBlocks builtIn monitoring.
	// If BuiltIn is set to false, the provider should set ExporterConfig and Sidecar container own.
	// BuiltIn set to true is not currently supported but will be soon.
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
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	Name string `json:"name,omitempty"`

	// type is in enum of {data, log}.
	// VolumeTypeData: the volume is for the persistent data storage.
	// VolumeTypeLog: the volume is for the persistent log storage.
	// +optional
	Type VolumeType `json:"type,omitempty"`
}

// ClusterComponentDefinition provides a workload component specification template,
// with attributes that strongly work with stateful workloads and day-2 operations
// behaviors.
type ClusterComponentDefinition struct {
	// name of the component, it can be any valid string.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=18
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
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
	// +optional
	CharacterType string `json:"characterType,omitempty"`

	// The maximum number of pods that can be unavailable during scaling.
	// Value can be an absolute number (ex: 5) or a percentage of desired pods (ex: 10%).
	// Absolute number is calculated from percentage by rounding down. This value is ignored
	// if workloadType is Consensus.
	// +kubebuilder:validation:XIntOrString
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

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

	// consensusSpec defines consensus related spec if workloadType is Consensus, required if workloadType is Consensus.
	// +optional
	ConsensusSpec *ConsensusSetSpec `json:"consensusSpec,omitempty"`

	// replicationSpec defines replication related spec if workloadType is Replication, required if workloadType is Replication.
	// +optional
	ReplicationSpec *ReplicationSpec `json:"replicationSpec,omitempty"`

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
	//  `{name: data, type: data}` means that the volume named `data` is used to store `data`.
	//  `{name: binlog, type: log}` means that the volume named `binlog` is used to store `log`.
	//
	// NOTE:
	//   When volumeTypes is not defined, the backup function will not be supported,
	// even if a persistent volume has been specified.
	// +optional
	VolumeTypes []VolumeTypeSpec `json:"volumeTypes,omitempty"`

	// customLabelSpecs is used for custom label tags which you want to add to the component resources.
	// +listType=map
	// +listMapKey=key
	// +optional
	CustomLabelSpecs []CustomLabelSpec `json:"customLabelSpecs,omitempty"`
}

type ServiceSpec struct {
	// The list of ports that are exposed by this service.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies
	// +patchMergeKey=port
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=port
	// +listMapKey=protocol
	Ports []ServicePort `json:"ports,omitempty" patchStrategy:"merge" patchMergeKey:"port" protobuf:"bytes,1,rep,name=ports"`
}

func (r *ServiceSpec) toSVCPorts() []corev1.ServicePort {
	ports := make([]corev1.ServicePort, 0, len(r.Ports))
	for _, p := range r.Ports {
		ports = append(ports, p.toSVCPort())
	}
	return ports
}

func (r ServiceSpec) ToSVCSpec() corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Ports: r.toSVCPorts(),
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
	// Policy is in enum of {None, Snapshot}. The default policy is `None`.
	// None: Default policy, do nothing.
	// Snapshot: Do native volume snapshot before scaling and restore to newly scaled pods.
	//           Prefer backup job to create snapshot if `BackupTemplateSelector` can find a template.
	//           Notice that 'Snapshot' policy will only take snapshot on one volumeMount, default is
	//           the first volumeMount of first container (i.e. clusterdefinition.spec.components.podSpec.containers[0].volumeMounts[0]),
	//           since take multiple snapshots at one time might cause consistency problem.
	// +kubebuilder:default=None
	// +optional
	Type HScaleDataClonePolicyType `json:"type,omitempty"`

	// backupTemplateSelector defines the label selector for finding associated BackupTemplate API object.
	// +optional
	BackupTemplateSelector map[string]string `json:"backupTemplateSelector,omitempty"`

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

type ConsensusSetSpec struct {
	// leader, one single leader.
	// +kubebuilder:validation:Required
	Leader ConsensusMember `json:"leader"`

	// followers, has voting right but not Leader.
	// +optional
	Followers []ConsensusMember `json:"followers,omitempty"`

	// learner, no voting right.
	// +optional
	Learner *ConsensusMember `json:"learner,omitempty"`

	// updateStrategy, Pods update strategy.
	// serial: update Pods one by one that guarantee minimum component unavailable time.
	// 		Learner -> Follower(with AccessMode=none) -> Follower(with AccessMode=readonly) -> Follower(with AccessMode=readWrite) -> Leader
	// bestEffortParallel: update Pods in parallel that guarantee minimum component un-writable time.
	//		Learner, Follower(minority) in parallel -> Follower(majority) -> Leader, keep majority online all the time.
	// parallel: force parallel
	// +kubebuilder:default=Serial
	// +optional
	UpdateStrategy UpdateStrategy `json:"updateStrategy,omitempty"`
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

type ReplicationSpec struct {
	// switchPolicies defines a collection of different types of switchPolicy, and each type of switchPolicy is limited to one.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	SwitchPolicies []SwitchPolicy `json:"switchPolicies,omitempty"`

	// switchCmdExecutorConfig configs how to get client SDK and perform switch statements.
	// +kubebuilder:validation:Required
	SwitchCmdExecutorConfig *SwitchCmdExecutorConfig `json:"switchCmdExecutorConfig"`
}

type SwitchPolicy struct {
	// switchPolicyType defines type of the switchPolicy.
	// MaximumAvailability: when the primary is active, do switch if the synchronization delay = 0 in the user-defined lagProbe data delay detection logic, otherwise do not switch. The primary is down, switch immediately.
	// MaximumDataProtection: when the primary is active, do switch if synchronization delay = 0 in the user-defined lagProbe data lag detection logic, otherwise do not switch. If the primary is down, if it can be judged that the primary and secondary data are consistent, then do the switch, otherwise do not switch.
	// Noop: KubeBlocks will not perform high-availability switching on components. Users need to implement HA by themselves.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=MaximumAvailability
	Type SwitchPolicyType `json:"type"`

	// switchStatements defines switching actions according to their respective roles, We divide all pods into three switchStatement role={Promote,Demote,Follow}.
	// Promote: candidate primary after elected, which to be promoted
	// Demote: primary before switch, which to be demoted
	// Follow: the other secondaries that are not selected as the primary, which to follow the new primary
	// if switchStatements is not set，we will try to use the built-in switchStatements for the database engine with built-in support.
	// +optional
	SwitchStatements *SwitchStatements `json:"switchStatements,omitempty"`
}

type SwitchStatements struct {
	// promote defines the switching actions for the candidate primary which to be promoted.
	// +optional
	Promote []string `json:"promote,omitempty"`

	// demote defines the switching actions for the old primary which to be demoted.
	// +optional
	Demote []string `json:"demote,omitempty"`

	// follow defines the switching actions for the other secondaries which are not selected as the primary.
	// +optional
	Follow []string `json:"follow,omitempty"`
}

type SwitchCmdExecutorConfig struct {
	CommandExecutorEnvItem `json:",inline"`

	// switchSteps definition, users can customize the switching steps on the provided three roles - NewPrimary, OldPrimary, and Secondaries.
	// the same role can customize multiple steps in the order of the list, and KubeBlocks will perform switching operations in the defined order.
	// if switchStep is not set, we will try to use the built-in switchStep for the database engine with built-in support.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +optional
	SwitchSteps []SwitchStep `json:"switchSteps"`
}

type SwitchStep struct {
	CommandExecutorItem `json:",inline"`

	// role determines which role to execute the command on, role is divided into three roles NewPrimary, OldPrimary, and Secondaries.
	Role SwitchStepRole `json:"role"`
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
