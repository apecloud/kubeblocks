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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

// ClusterComponentDefinition defines a Component within a ClusterDefinition but is deprecated and
// has been replaced by ComponentDefinition.
//
// Deprecated: Use ComponentDefinition instead. This type is deprecated as of version 0.8.
//
// +kubebuilder:validation:XValidation:rule="has(self.workloadType) && self.workloadType == 'Consensus' ? (has(self.consensusSpec) || has(self.rsmSpec)) : !has(self.consensusSpec)",message="componentDefs.consensusSpec(deprecated) or componentDefs.rsmSpec(recommended) is required when componentDefs.workloadType is Consensus, and forbidden otherwise"
type ClusterComponentDefinition struct {
	// This name could be used as default name of `cluster.spec.componentSpecs.name`, and needs to conform with same
	// validation rules as `cluster.spec.componentSpecs.name`, currently complying with IANA Service Naming rule.
	// This name will apply to cluster objects as the value of label "apps.kubeblocks.io/component-name".
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Description of the component definition.
	//
	// +optional
	Description string `json:"description,omitempty"`

	// Defines the type of the workload.
	//
	// - `Stateless` describes stateless applications.
	// - `Stateful` describes common stateful applications.
	// - `Consensus` describes applications based on consensus protocols, such as raft and paxos.
	// - `Replication` describes applications based on the primary-secondary data replication protocol.
	//
	// +kubebuilder:validation:Required
	WorkloadType WorkloadType `json:"workloadType"`

	// Defines well-known database component name, such as mongos(mongodb), proxy(redis), mariadb(mysql).
	//
	// +optional
	CharacterType string `json:"characterType,omitempty"`

	// Defines the template of configurations.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ConfigSpecs []ComponentConfigSpec `json:"configSpecs,omitempty"`

	// Defines the template of scripts.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ScriptSpecs []ComponentTemplateSpec `json:"scriptSpecs,omitempty"`

	// Settings for health checks.
	//
	// +optional
	Probes *ClusterDefinitionProbes `json:"probes,omitempty"`

	// Specify the logging files which can be observed and configured by cluster users.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	LogConfigs []LogConfig `json:"logConfigs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Defines the pod spec template of component.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	PodSpec *corev1.PodSpec `json:"podSpec,omitempty"`

	// Defines the service spec.
	//
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// Defines spec for `Stateless` workloads.
	//
	// +kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.7.0, use RSMSpec instead."
	// +optional
	StatelessSpec *StatelessSetSpec `json:"statelessSpec,omitempty"`

	// Defines spec for `Stateful` workloads.
	//
	// +kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.7.0, use RSMSpec instead."
	// +optional
	StatefulSpec *StatefulSetSpec `json:"statefulSpec,omitempty"`

	// Defines spec for `Consensus` workloads. It's required if the workload type is `Consensus`.
	//
	// +kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.7.0, use RSMSpec instead."
	// +optional
	ConsensusSpec *ConsensusSetSpec `json:"consensusSpec,omitempty"`

	// Defines spec for `Replication` workloads.
	//
	// +kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.7.0, use RSMSpec instead."
	// +optional
	ReplicationSpec *ReplicationSetSpec `json:"replicationSpec,omitempty"`

	// Defines workload spec of this component.
	// From KB 0.7.0, RSM(InstanceSetSpec) will be the underlying CR which powers all kinds of workload in KB.
	// RSM is an enhanced stateful workload extension dedicated for heavy-state workloads like databases.
	//
	// +optional
	RSMSpec *RSMSpec `json:"rsmSpec,omitempty"`

	// Defines the behavior of horizontal scale.
	//
	// +optional
	HorizontalScalePolicy *HorizontalScalePolicy `json:"horizontalScalePolicy,omitempty"`

	// Defines system accounts needed to manage the component, and the statement to create them.
	//
	// +optional
	SystemAccounts *SystemAccountSpec `json:"systemAccounts,omitempty"`

	// Used to describe the purpose of the volumes mapping the name of the VolumeMounts in the PodSpec.Container field,
	// such as data volume, log volume, etc. When backing up the volume, the volume can be correctly backed up according
	// to the volumeType.
	//
	// For example:
	//
	// - `name: data, type: data` means that the volume named `data` is used to store `data`.
	// - `name: binlog, type: log` means that the volume named `binlog` is used to store `log`.
	//
	// NOTE: When volumeTypes is not defined, the backup function will not be supported, even if a persistent volume has
	// been specified.
	//
	// +listType=map
	// +listMapKey=name
	// +optional
	VolumeTypes []VolumeTypeSpec `json:"volumeTypes,omitempty"`

	// Used for custom label tags which you want to add to the component resources.
	//
	// +listType=map
	// +listMapKey=key
	// +optional
	CustomLabelSpecs []CustomLabelSpec `json:"customLabelSpecs,omitempty"`

	// Defines command to do switchover.
	// In particular, when workloadType=Replication, the command defined in switchoverSpec will only be executed under
	// the condition of cluster.componentSpecs[x].SwitchPolicy.type=Noop.
	//
	// +optional
	SwitchoverSpec *SwitchoverSpec `json:"switchoverSpec,omitempty"`

	// Defines the command to be executed when the component is ready, and the command will only be executed once after
	// the component becomes ready.
	//
	// +optional
	PostStartSpec *PostStartAction `json:"postStartSpec,omitempty"`

	// Defines settings to do volume protect.
	//
	// +optional
	VolumeProtectionSpec *VolumeProtectionSpec `json:"volumeProtectionSpec,omitempty"`

	// Used to inject values from other components into the current component. Values will be saved and updated in a
	// configmap and mounted to the current component.
	//
	// +patchMergeKey=componentDefName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentDefName
	// +optional
	ComponentDefRef []ComponentDefRef `json:"componentDefRef,omitempty" patchStrategy:"merge" patchMergeKey:"componentDefName"`

	// Used to declare the service reference of the current component.
	//
	// +optional
	ServiceRefDeclarations []ServiceRefDeclaration `json:"serviceRefDeclarations,omitempty"`

	// Defines the metrics exporter.
	//
	// +optional
	Exporter *Exporter `json:"exporter,omitempty"`

	// Deprecated since v0.9
	// monitor is monitoring config which provided by provider.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.10.0"
	// +optional
	Monitor *MonitorConfig `json:"monitor,omitempty"`
}

// WorkloadType defines the type of workload for the components of the ClusterDefinition.
// It can be one of the following: `Stateless`, `Stateful`, `Consensus`, or `Replication`.
//
// Deprecated since v0.8.
//
// +enum
// +kubebuilder:validation:Enum={Stateless,Stateful,Consensus,Replication}
type WorkloadType string

const (
	// Stateless represents a workload type where components do not maintain state, and instances are interchangeable.
	Stateless WorkloadType = "Stateless"

	// Stateful represents a workload type where components maintain state, and each instance has a unique identity.
	Stateful WorkloadType = "Stateful"

	// Consensus represents a workload type involving distributed consensus algorithms for coordinated decision-making.
	Consensus WorkloadType = "Consensus"

	// Replication represents a workload type that involves replication, typically used for achieving high availability
	// and fault tolerance.
	Replication WorkloadType = "Replication"
)

// ClusterDefinitionProbes is deprecated since v0.8.
type ClusterDefinitionProbes struct {
	// Specifies the probe used for checking the running status of the component.
	//
	// +optional
	RunningProbe *ClusterDefinitionProbe `json:"runningProbe,omitempty"`

	// Specifies the probe used for checking the status of the component.
	//
	// +optional
	StatusProbe *ClusterDefinitionProbe `json:"statusProbe,omitempty"`

	// Specifies the probe used for checking the role of the component.
	//
	// +kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.7.0, use RSMSpec instead."
	// +optional
	RoleProbe *ClusterDefinitionProbe `json:"roleProbe,omitempty"`

	// Defines the timeout (in seconds) for the role probe after all pods of the component are ready.
	// The system will check if the application is available in the pod.
	// If pods exceed the InitializationTimeoutSeconds time without a role label, this component will enter the
	// Failed/Abnormal phase.
	//
	// Note that this configuration will only take effect if the component supports RoleProbe
	// and will not affect the life cycle of the pod. default values are 60 seconds.
	//
	// +kubebuilder:validation:Minimum=30
	// +optional
	RoleProbeTimeoutAfterPodsReady int32 `json:"roleProbeTimeoutAfterPodsReady,omitempty"`
}

// ClusterDefinitionProbe is deprecated since v0.8.
type ClusterDefinitionProbe struct {
	// How often (in seconds) to perform the probe.
	//
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`

	// Number of seconds after which the probe times out. Defaults to 1 second.
	//
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`

	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	//
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=2
	FailureThreshold int32 `json:"failureThreshold,omitempty"`

	// Commands used to execute for probe.
	//
	// +optional
	Commands *ClusterDefinitionProbeCMDs `json:"commands,omitempty"`
}

// ClusterDefinitionProbeCMDs is deprecated since v0.8.
type ClusterDefinitionProbeCMDs struct {
	// Defines write checks that are executed on the probe sidecar.
	//
	// +optional
	Writes []string `json:"writes,omitempty"`

	// Defines read checks that are executed on the probe sidecar.
	//
	// +optional
	Queries []string `json:"queries,omitempty"`
}

// ServiceSpec is deprecated since v0.8.
type ServiceSpec struct {
	// The list of ports that are exposed by this service.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies
	//
	// +patchMergeKey=port
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=port
	// +listMapKey=protocol
	// +optional
	Ports []ServicePort `json:"ports,omitempty" patchStrategy:"merge" patchMergeKey:"port" protobuf:"bytes,1,rep,name=ports"`

	// NOTES: name also need to be key
}

// ServicePort is deprecated since v0.8.
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
	//
	// Number must be in the range 1 to 65535. Name must be an IANA_SVC_NAME.
	//
	// - If this is a string, it will be looked up as a named port in the target Pod's container ports.
	// - If this is not specified, the value of the `port` field is used (an identity map).
	//
	// This field is ignored for services with clusterIP=None, and should be
	// omitted or set equal to the `port` field.
	//
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#defining-a-service
	//
	// +kubebuilder:validation:XIntOrString
	// +optional
	TargetPort intstr.IntOrString `json:"targetPort,omitempty" protobuf:"bytes,4,opt,name=targetPort"`
}

// StatelessSetSpec is deprecated since v0.7.
type StatelessSetSpec struct {
	// Specifies the deployment strategy that will be used to replace existing pods with new ones.
	//
	// +patchStrategy=retainKeys
	// +optional
	UpdateStrategy appsv1.DeploymentStrategy `json:"updateStrategy,omitempty"`
}

// StatefulSetSpec is deprecated since v0.7.
type StatefulSetSpec struct {
	// Specifies the strategy for updating Pods.
	// For workloadType=`Consensus`, the update strategy can be one of the following:
	//
	// - `Serial`: Updates Members sequentially to minimize component downtime.
	// - `BestEffortParallel`: Updates Members in parallel to minimize component write downtime. Majority remains online
	// at all times.
	// - `Parallel`: Forces parallel updates.
	//
	// +kubebuilder:default=Serial
	// +optional
	UpdateStrategy UpdateStrategy `json:"updateStrategy,omitempty"`

	// Controls the creation of pods during initial scale up, replacement of pods on nodes, and scaling down.
	//
	// - `OrderedReady`: Creates pods in increasing order (pod-0, then pod-1, etc). The controller waits until each pod
	// is ready before continuing. Pods are removed in reverse order when scaling down.
	// - `Parallel`: Creates pods in parallel to match the desired scale without waiting. All pods are deleted at once
	// when scaling down.
	//
	// +optional
	LLPodManagementPolicy appsv1.PodManagementPolicyType `json:"llPodManagementPolicy,omitempty"`

	// Specifies the low-level StatefulSetUpdateStrategy to be used when updating Pods in the StatefulSet upon a
	// revision to the Template.
	// `UpdateStrategy` will be ignored if this is provided.
	//
	// +optional
	LLUpdateStrategy *appsv1.StatefulSetUpdateStrategy `json:"llUpdateStrategy,omitempty"`
}

// ConsensusSetSpec is deprecated since v0.7.
type ConsensusSetSpec struct {
	StatefulSetSpec `json:",inline"`

	// Represents a single leader in the consensus set.
	//
	// +kubebuilder:validation:Required
	Leader ConsensusMember `json:"leader"`

	// Members of the consensus set that have voting rights but are not the leader.
	//
	// +optional
	Followers []ConsensusMember `json:"followers,omitempty"`

	// Represents a member of the consensus set that does not have voting rights.
	//
	// +optional
	Learner *ConsensusMember `json:"learner,omitempty"`
}

// ReplicationSetSpec is deprecated since v0.7.
type ReplicationSetSpec struct {
	StatefulSetSpec `json:",inline"`
}

// ConsensusMember is deprecated since v0.7.
type ConsensusMember struct {
	// Specifies the name of the consensus member.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:default=leader
	Name string `json:"name"`

	// Specifies the services that this member is capable of providing.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:default=ReadWrite
	AccessMode AccessMode `json:"accessMode"`

	// Indicates the number of Pods that perform this role.
	// The default is 1 for `Leader`, 0 for `Learner`, others for `Followers`.
	//
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
}

// RSMSpec is deprecated since v0.8.
type RSMSpec struct {
	// Specifies a list of roles defined within the system.
	//
	// +optional
	Roles []workloads.ReplicaRole `json:"roles,omitempty"`

	// Defines the method used to probe a role.
	//
	// +optional
	RoleProbe *workloads.RoleProbe `json:"roleProbe,omitempty"`

	// Indicates the actions required for dynamic membership reconfiguration.
	//
	// +optional
	MembershipReconfiguration *workloads.MembershipReconfiguration `json:"membershipReconfiguration,omitempty"`

	// Describes the strategy for updating Members (Pods).
	//
	// - `Serial`: Updates Members sequentially to ensure minimum component downtime.
	// - `BestEffortParallel`: Updates Members in parallel to ensure minimum component write downtime.
	// - `Parallel`: Forces parallel updates.
	//
	// +kubebuilder:validation:Enum={Serial,BestEffortParallel,Parallel}
	// +optional
	MemberUpdateStrategy *workloads.MemberUpdateStrategy `json:"memberUpdateStrategy,omitempty"`
}

// HorizontalScalePolicy is deprecated since v0.8.
type HorizontalScalePolicy struct {
	// Determines the data synchronization method when a component scales out.
	// The policy can be one of the following: {None, CloneVolume}. The default policy is `None`.
	//
	// - `None`: This is the default policy. It creates an empty volume without data cloning.
	// - `CloneVolume`: This policy clones data to newly scaled pods. It first tries to use a volume snapshot.
	//   If volume snapshot is not enabled, it will attempt to use a backup tool. If neither method works, it will report an error.
	// - `Snapshot`: This policy is deprecated and is an alias for CloneVolume.
	//
	// +kubebuilder:default=None
	// +optional
	Type HScaleDataClonePolicyType `json:"type,omitempty"`

	// Refers to the backup policy template.
	//
	// +optional
	BackupPolicyTemplateName string `json:"backupPolicyTemplateName,omitempty"`

	// Specifies the volumeMount of the container to backup.
	// This only works if Type is not None. If not specified, the first volumeMount will be selected.
	//
	// +optional
	VolumeMountsName string `json:"volumeMountsName,omitempty"`
}

// HScaleDataClonePolicyType defines the data clone policy to be used during horizontal scaling.
// This policy determines how data is handled when new nodes are added to the cluster.
// The policy can be set to `None`, `CloneVolume`, or `Snapshot`.
//
// +enum
// +kubebuilder:validation:Enum={None,CloneVolume,Snapshot}
type HScaleDataClonePolicyType string

const (
	// HScaleDataClonePolicyNone indicates that no data cloning will occur during horizontal scaling.
	HScaleDataClonePolicyNone HScaleDataClonePolicyType = "None"

	// HScaleDataClonePolicyCloneVolume indicates that data will be cloned from existing volumes during horizontal scaling.
	HScaleDataClonePolicyCloneVolume HScaleDataClonePolicyType = "CloneVolume"

	// HScaleDataClonePolicyFromSnapshot indicates that data will be cloned from a snapshot during horizontal scaling.
	HScaleDataClonePolicyFromSnapshot HScaleDataClonePolicyType = "Snapshot"
)

// SystemAccountSpec specifies information to create system accounts.
//
// Deprecated since v0.8, be replaced by `componentDefinition.spec.systemAccounts` and
// `componentDefinition.spec.lifecycleActions.accountProvision`.
type SystemAccountSpec struct {
	// Configures how to obtain the client SDK and execute statements.
	//
	// +kubebuilder:validation:Required
	CmdExecutorConfig *CmdExecutorConfig `json:"cmdExecutorConfig"`

	// Defines the pattern used to generate passwords for system accounts.
	//
	// +kubebuilder:validation:Required
	PasswordConfig PasswordConfig `json:"passwordConfig"`

	// Defines the configuration settings for system accounts.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Accounts []SystemAccountConfig `json:"accounts" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`
}

// CmdExecutorConfig specifies how to perform creation and deletion statements.
//
// Deprecated since v0.8.
type CmdExecutorConfig struct {
	CommandExecutorEnvItem `json:",inline"`
	CommandExecutorItem    `json:",inline"`
}

// CommandExecutorEnvItem is deprecated since v0.8.
type CommandExecutorEnvItem struct {
	// Specifies the image used to execute the command.
	//
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// A list of environment variables that will be injected into the command execution context.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

// CommandExecutorItem is deprecated since v0.8.
type CommandExecutorItem struct {
	// The command to be executed.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Command []string `json:"command"`

	// Additional parameters used in the execution of the command.
	//
	// +optional
	Args []string `json:"args,omitempty"`
}

// SystemAccountConfig specifies how to create and delete system accounts.
//
// Deprecated since v0.9.
type SystemAccountConfig struct {
	// The unique identifier of a system account.
	//
	// +kubebuilder:validation:Required
	Name AccountName `json:"name"`

	// Outlines the strategy for creating the account.
	//
	// +kubebuilder:validation:Required
	ProvisionPolicy ProvisionPolicy `json:"provisionPolicy"`
}

// AccountName defines system account names.
// +enum
// +kubebuilder:validation:Enum={kbadmin,kbdataprotection,kbprobe,kbmonitoring,kbreplicator}
type AccountName string

const (
	AdminAccount          AccountName = "kbadmin"
	DataprotectionAccount AccountName = "kbdataprotection"
	ProbeAccount          AccountName = "kbprobe"
	MonitorAccount        AccountName = "kbmonitoring"
	ReplicatorAccount     AccountName = "kbreplicator"
)

// ProvisionPolicy defines the policy details for creating accounts.
//
// Deprecated since v0.9.
type ProvisionPolicy struct {
	// Specifies the method to provision an account.
	//
	// +kubebuilder:validation:Required
	Type ProvisionPolicyType `json:"type"`

	// Defines the scope within which the account is provisioned.
	//
	// +kubebuilder:default=AnyPods
	Scope ProvisionScope `json:"scope"`

	// The statement to provision an account.
	//
	// +optional
	Statements *ProvisionStatements `json:"statements,omitempty"`

	// The external secret to refer.
	//
	// +optional
	SecretRef *ProvisionSecretRef `json:"secretRef,omitempty"`
}

// ProvisionPolicyType defines the policy for creating accounts.
//
// +enum
// +kubebuilder:validation:Enum={CreateByStmt,ReferToExisting}
type ProvisionPolicyType string

const (
	// CreateByStmt will create account w.r.t. deletion and creation statement given by provider.
	CreateByStmt ProvisionPolicyType = "CreateByStmt"

	// ReferToExisting will not create account, but create a secret by copying data from referred secret file.
	ReferToExisting ProvisionPolicyType = "ReferToExisting"
)

// ProvisionScope defines the scope of provision within a component.
//
// +enum
type ProvisionScope string

const (
	// AllPods indicates that accounts will be created for all pods within the component.
	AllPods ProvisionScope = "AllPods"

	// AnyPods indicates that accounts will be created only on a single pod within the component.
	AnyPods ProvisionScope = "AnyPods"
)

// ProvisionStatements defines the statements used to create accounts.
//
// Deprecated since v0.9.
type ProvisionStatements struct {
	// Specifies the statement required to create a new account with the necessary privileges.
	//
	// +kubebuilder:validation:Required
	CreationStatement string `json:"creation"`

	// Defines the statement required to update the password of an existing account.
	//
	// +optional
	UpdateStatement string `json:"update,omitempty"`

	// Defines the statement required to delete an existing account.
	// Typically used in conjunction with the creation statement to delete an account before recreating it.
	// For example, one might use a `drop user if exists` statement followed by a `create user` statement to ensure a fresh account.
	//
	// Deprecated: This field is deprecated and the update statement should be used instead.
	//
	// +optional
	DeletionStatement string `json:"deletion,omitempty"`
}

// VolumeTypeSpec is deprecated since v0.9, replaced with ComponentVolume.
type VolumeTypeSpec struct {
	// Corresponds to the name of the VolumeMounts field in PodSpec.Container.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Type of data the volume will persistent.
	//
	// +optional
	Type VolumeType `json:"type,omitempty"`
}

// VolumeType defines the type of volume, specifically distinguishing between volumes used for backup data and those used for logs.
//
// +enum
// +kubebuilder:validation:Enum={data,log}
type VolumeType string

const (
	// VolumeTypeData indicates a volume designated for storing backup data. This type of volume is optimized for the
	// storage and retrieval of data backups, ensuring data persistence and reliability.
	VolumeTypeData VolumeType = "data"

	// VolumeTypeLog indicates a volume designated for storing logs. This type of volume is optimized for log data,
	// facilitating efficient log storage, retrieval, and management.
	VolumeTypeLog VolumeType = "log"
)

// CustomLabelSpec is deprecated since v0.8.
type CustomLabelSpec struct {
	// The key of the label.
	//
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// The value of the label.
	//
	// +kubebuilder:validation:Required
	Value string `json:"value"`

	// The resources that will be patched with the label.
	//
	// +kubebuilder:validation:Required
	Resources []GVKResource `json:"resources,omitempty"`
}

// GVKResource is deprecated since v0.8.
type GVKResource struct {
	// Represents the GVK of a resource, such as "v1/Pod", "apps/v1/StatefulSet", etc.
	// When a resource matching this is found by the selector, a custom label will be added if it doesn't already exist,
	// or updated if it does.
	//
	// +kubebuilder:validation:Required
	GVK string `json:"gvk"`

	// A label query used to filter a set of resources.
	//
	// +optional
	Selector map[string]string `json:"selector,omitempty"`
}

// SwitchoverSpec is deprecated since v0.8.
type SwitchoverSpec struct {
	// Represents the action of switching over to a specified candidate primary or leader instance.
	//
	// +optional
	WithCandidate *SwitchoverAction `json:"withCandidate,omitempty"`

	// Represents the action of switching over without specifying a candidate primary or leader instance.
	//
	// +optional
	WithoutCandidate *SwitchoverAction `json:"withoutCandidate,omitempty"`
}

// SwitchoverAction is deprecated since v0.8.
type SwitchoverAction struct {
	// Specifies the switchover command.
	//
	// +kubebuilder:validation:Required
	CmdExecutorConfig *CmdExecutorConfig `json:"cmdExecutorConfig"`

	// Used to select the script that need to be referenced.
	// When defined, the scripts defined in scriptSpecs can be referenced within the SwitchoverAction.CmdExecutorConfig.
	//
	// +kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.9.0"
	// +optional
	ScriptSpecSelectors []ScriptSpecSelector `json:"scriptSpecSelectors,omitempty"`
}

type ScriptSpecSelector struct {
	// Represents the name of the ScriptSpec referent.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`
}

// PostStartAction is deprecated since v0.8.
type PostStartAction struct {
	// Specifies the  post-start command to be executed.
	//
	// +kubebuilder:validation:Required
	CmdExecutorConfig CmdExecutorConfig `json:"cmdExecutorConfig"`

	// Used to select the script that need to be referenced.
	// When defined, the scripts defined in scriptSpecs can be referenced within the CmdExecutorConfig.
	//
	// +optional
	ScriptSpecSelectors []ScriptSpecSelector `json:"scriptSpecSelectors,omitempty"`
}

// VolumeProtectionSpec is deprecated since v0.9, replaced with ComponentVolume.HighWatermark.
type VolumeProtectionSpec struct {
	// The high watermark threshold for volume space usage.
	// If there is any specified volumes who's space usage is over the threshold, the pre-defined "LOCK" action
	// will be triggered to degrade the service to protect volume from space exhaustion, such as to set the instance
	// as read-only. And after that, if all volumes' space usage drops under the threshold later, the pre-defined
	// "UNLOCK" action will be performed to recover the service normally.
	//
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=90
	// +optional
	HighWatermark int `json:"highWatermark,omitempty"`

	// The Volumes to be protected.
	//
	// +optional
	Volumes []ProtectedVolume `json:"volumes,omitempty"`
}

type ProtectedVolume struct {
	// The Name of the volume to protect.
	//
	// +optional
	Name string `json:"name,omitempty"`

	// Defines the high watermark threshold for the volume, it will override the component level threshold.
	// If the value is invalid, it will be ignored and the component level threshold will be used.
	//
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	// +optional
	HighWatermark *int `json:"highWatermark,omitempty"`
}

// ComponentDefRef is used to select the component and its fields to be referenced.
//
// Deprecated since v0.8.
type ComponentDefRef struct {
	// The name of the componentDef to be selected.
	//
	// +kubebuilder:validation:Required
	ComponentDefName string `json:"componentDefName"`

	// Defines the policy to be followed in case of a failure in finding the component.
	//
	// +kubebuilder:validation:Enum={Ignore,Fail}
	// +default="Ignore"
	// +optional
	FailurePolicy FailurePolicyType `json:"failurePolicy,omitempty"`

	// The values that are to be injected as environment variables into each component.
	//
	// +kbubebuilder:validation:Required
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ComponentRefEnvs []ComponentRefEnv `json:"componentRefEnv" patchStrategy:"merge" patchMergeKey:"name"`
}

// ComponentRefEnv specifies name and value of an env.
//
// Deprecated since v0.8.
type ComponentRefEnv struct {
	// The name of the env, it must be a C identifier.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[A-Za-z_][A-Za-z0-9_]*$`
	Name string `json:"name"`

	// The value of the env.
	//
	// +optional
	Value string `json:"value,omitempty"`

	// The source from which the value of the env.
	//
	// +optional
	ValueFrom *ComponentValueFrom `json:"valueFrom,omitempty"`
}

// ComponentValueFrom is deprecated since v0.8.
type ComponentValueFrom struct {
	// Specifies the source to select. It can be one of three types: `FieldRef`, `ServiceRef`, `HeadlessServiceRef`.
	//
	// +kubebuilder:validation:Enum={FieldRef,ServiceRef,HeadlessServiceRef}
	// +kubebuilder:validation:Required
	Type ComponentValueFromType `json:"type"`

	// The jsonpath of the source to select when the Type is `FieldRef`.
	// Two objects are registered in the jsonpath: `componentDef` and `components`:
	//
	// - `componentDef` is the component definition object specified in `componentRef.componentDefName`.
	// - `components` are the component list objects referring to the component definition object.
	//
	// +optional
	FieldPath string `json:"fieldPath,omitempty"`

	// Defines the format of each headless service address.
	// Three builtin variables can be used as placeholders: `$POD_ORDINAL`, `$POD_FQDN`, `$POD_NAME`
	//
	// - `$POD_ORDINAL` represents the ordinal of the pod.
	// - `$POD_FQDN` represents the fully qualified domain name of the pod.
	// - `$POD_NAME` represents the name of the pod.
	//
	// +kubebuilder:default=="$POD_FQDN"
	// +optional
	Format string `json:"format,omitempty"`

	// The string used to join the values of headless service addresses.
	//
	// +kubebuilder:default=","
	// +optional
	JoinWith string `json:"joinWith,omitempty"`
}

// ComponentValueFromType specifies the type of component value from which the data is derived.
//
// Deprecated since v0.8.
//
// +enum
// +kubebuilder:validation:Enum={FieldRef,ServiceRef,HeadlessServiceRef}
type ComponentValueFromType string

const (
	// FromFieldRef refers to the value of a specific field in the object.
	FromFieldRef ComponentValueFromType = "FieldRef"
	// FromServiceRef refers to a service within the same namespace as the object.
	FromServiceRef ComponentValueFromType = "ServiceRef"
	// FromHeadlessServiceRef refers to a headless service within the same namespace as the object.
	FromHeadlessServiceRef ComponentValueFromType = "HeadlessServiceRef"
)

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
