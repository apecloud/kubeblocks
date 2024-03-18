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

// Package v1alpha1 contains API Schema definitions for the apps v1alpha1 API group
package v1alpha1

import (
	"errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	APIVersion            = "apps.kubeblocks.io/v1alpha1"
	ClusterVersionKind    = "ClusterVersion"
	ClusterDefinitionKind = "ClusterDefinition"
	ClusterKind           = "Cluster"
	ComponentKind         = "Component"
	OpsRequestKind        = "OpsRequestKind"
)

type ComponentTemplateSpec struct {
	// Specifies the name of the configuration template.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Specifies the name of the referenced configuration template ConfigMap object.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	TemplateRef string `json:"templateRef"`

	// Specifies the namespace of the referenced configuration template ConfigMap object.
	// An empty namespace is equivalent to the "default" namespace.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
	// +kubebuilder:default="default"
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Refers to the volume name of PodTemplate. The configuration file produced through the configuration
	// template will be mounted to the corresponding volume. Must be a DNS_LABEL name.
	// The volume name must be defined in podSpec.containers[*].volumeMounts.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	VolumeName string `json:"volumeName"`

	// Refers to the mode bits used to set permissions on created files by default.
	//
	// Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511.
	// YAML accepts both octal and decimal values, JSON requires decimal values for mode bits.
	// Defaults to 0644.
	//
	// Directories within the path are not affected by this setting.
	// This might be in conflict with other options that affect the file
	// mode, like fsGroup, and the result can be other mode bits set.
	//
	// +optional
	DefaultMode *int32 `json:"defaultMode,omitempty" protobuf:"varint,3,opt,name=defaultMode"`
}

type ConfigTemplateExtension struct {
	// Specifies the name of the referenced configuration template ConfigMap object.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	TemplateRef string `json:"templateRef"`

	// Specifies the namespace of the referenced configuration template ConfigMap object.
	// An empty namespace is equivalent to the "default" namespace.
	//
	// +kubebuilder:default="default"
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Defines the strategy for merging externally imported templates into component templates.
	//
	// +kubebuilder:default="none"
	// +optional
	Policy MergedPolicy `json:"policy,omitempty"`
}

type LegacyRenderedTemplateSpec struct {
	// Extends the configuration template.
	ConfigTemplateExtension `json:",inline"`
}

type ComponentConfigSpec struct {
	ComponentTemplateSpec `json:",inline"`

	// Defines a list of keys.
	// If left empty, ConfigConstraint applies to all keys in the configmap.
	//
	// +listType=set
	// +optional
	Keys []string `json:"keys,omitempty"`

	// An optional field that defines the secondary rendered config spec.
	//
	// +optional
	LegacyRenderedConfigSpec *LegacyRenderedTemplateSpec `json:"legacyRenderedConfigSpec,omitempty"`

	// An optional field that defines the name of the referenced configuration constraints object.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	ConfigConstraintRef string `json:"constraintRef,omitempty"`

	// An optional field where the list of containers will be injected into EnvFrom.
	//
	// +listType=set
	// +optional
	AsEnvFrom []string `json:"asEnvFrom,omitempty"`

	// An optional field defines which resources change trigger re-render config.
	// +listType=set
	// +optional
	ReRenderResourceTypes []RerenderResourceType `json:"reRenderResourceTypes,omitempty"`
}

// RerenderResourceType defines the resource requirements for a component.
// +enum
// +kubebuilder:validation:Enum={resources,replcias,tls}
type RerenderResourceType string

const (
	ComponentResourceType RerenderResourceType = "resources"
	ComponentReplicasType RerenderResourceType = "replicas"
)

// MergedPolicy defines how to merge external imported templates into component templates.
// +enum
// +kubebuilder:validation:Enum={patch,replace,none}
type MergedPolicy string

const (
	PatchPolicy     MergedPolicy = "patch"
	ReplacePolicy   MergedPolicy = "replace"
	OnlyAddPolicy   MergedPolicy = "add"
	NoneMergePolicy MergedPolicy = "none"
)

// ClusterPhase defines the phase of the Cluster within the .status.phase field.
//
// +enum
// +kubebuilder:validation:Enum={Creating,Running,Updating,Stopping,Stopped,Deleting,Failed,Abnormal}
type ClusterPhase string

const (
	// CreatingClusterPhase represents all components are in `Creating` phase.
	CreatingClusterPhase ClusterPhase = "Creating"

	// RunningClusterPhase represents all components are in `Running` phase, indicates that the cluster is functioning properly.
	RunningClusterPhase ClusterPhase = "Running"

	// UpdatingClusterPhase represents all components are in `Creating`, `Running` or `Updating` phase, and at least one
	// component is in `Creating` or `Updating` phase, indicates that the cluster is undergoing an update.
	UpdatingClusterPhase ClusterPhase = "Updating"

	// StoppingClusterPhase represents at least one component is in `Stopping` phase, indicates that the cluster is in
	// the process of stopping.
	StoppingClusterPhase ClusterPhase = "Stopping"

	// StoppedClusterPhase represents all components are in `Stopped` phase, indicates that the cluster has stopped and
	// is not providing any functionality.
	StoppedClusterPhase ClusterPhase = "Stopped"

	// DeletingClusterPhase indicates the cluster is being deleted.
	DeletingClusterPhase ClusterPhase = "Deleting"

	// FailedClusterPhase represents all components are in `Failed` phase, indicates that the cluster is unavailable.
	FailedClusterPhase ClusterPhase = "Failed"

	// AbnormalClusterPhase represents some components are in `Failed` or `Abnormal` phase, indicates that the cluster
	// is in a fragile state and troubleshooting is required.
	AbnormalClusterPhase ClusterPhase = "Abnormal"
)

// ClusterComponentPhase defines the phase of a cluster component as represented in cluster.status.components.phase field.
//
// +enum
// +kubebuilder:validation:Enum={Creating,Running,Updating,Stopping,Stopped,Deleting,Failed,Abnormal}
type ClusterComponentPhase string

const (
	// CreatingClusterCompPhase indicates the component is being created.
	CreatingClusterCompPhase ClusterComponentPhase = "Creating"

	// RunningClusterCompPhase indicates the component has more than zero replicas, and all pods are up-to-date and
	// in a 'Running' state.
	RunningClusterCompPhase ClusterComponentPhase = "Running"

	// UpdatingClusterCompPhase indicates the component has more than zero replicas, and there are no failed pods,
	// it is currently being updated.
	UpdatingClusterCompPhase ClusterComponentPhase = "Updating"

	// StoppingClusterCompPhase indicates the component has zero replicas, and there are pods that are terminating.
	StoppingClusterCompPhase ClusterComponentPhase = "Stopping"

	// StoppedClusterCompPhase indicates the component has zero replicas, and all pods have been deleted.
	StoppedClusterCompPhase ClusterComponentPhase = "Stopped"

	// DeletingClusterCompPhase indicates the component is currently being deleted.
	DeletingClusterCompPhase ClusterComponentPhase = "Deleting"

	// FailedClusterCompPhase indicates the component has more than zero replicas, but there are some failed pods.
	// The component is not functioning.
	FailedClusterCompPhase ClusterComponentPhase = "Failed"

	// AbnormalClusterCompPhase indicates the component has more than zero replicas, but there are some failed pods.
	// The component is functioning, but it is in a fragile state.
	AbnormalClusterCompPhase ClusterComponentPhase = "Abnormal"
)

const (
	// define the cluster condition type
	ConditionTypeHaltRecovery        = "HaltRecovery"        // ConditionTypeHaltRecovery describe Halt recovery processing stage
	ConditionTypeProvisioningStarted = "ProvisioningStarted" // ConditionTypeProvisioningStarted the operator starts resource provisioning to create or change the cluster
	ConditionTypeApplyResources      = "ApplyResources"      // ConditionTypeApplyResources the operator start to apply resources to create or change the cluster
	ConditionTypeReplicasReady       = "ReplicasReady"       // ConditionTypeReplicasReady all pods of components are ready
	ConditionTypeReady               = "Ready"               // ConditionTypeReady all components are running
	ConditionTypeSwitchoverPrefix    = "Switchover-"         // ConditionTypeSwitchoverPrefix component status condition of switchover
)

// Phase represents the current status of the ClusterDefinition and ClusterVersion CR.
//
// +enum
// +kubebuilder:validation:Enum={Available,Unavailable}
type Phase string

const (
	// AvailablePhase indicates that the object is in an available state.
	AvailablePhase Phase = "Available"

	// UnavailablePhase indicates that the object is in an unavailable state.
	UnavailablePhase Phase = "Unavailable"
)

// ConfigConstraintPhase defines the ConfigConstraint  CR .status.phase
// +enum
// +kubebuilder:validation:Enum={Available,Unavailable, Deleting}
type ConfigConstraintPhase string

const (
	CCAvailablePhase   ConfigConstraintPhase = "Available"
	CCUnavailablePhase ConfigConstraintPhase = "Unavailable"
	CCDeletingPhase    ConfigConstraintPhase = "Deleting"
)

// DynamicParameterSelectedPolicy determines how to select the parameters of dynamic reload actions
//
// +enum
// +kubebuilder:validation:Enum={all,dynamic}
type DynamicParameterSelectedPolicy string

const (
	SelectedAllParameters     DynamicParameterSelectedPolicy = "all"
	SelectedDynamicParameters DynamicParameterSelectedPolicy = "dynamic"
)

// OpsPhase defines opsRequest phase.
// +enum
// +kubebuilder:validation:Enum={Pending,Creating,Running,Cancelling,Cancelled,Failed,Succeed}
type OpsPhase string

const (
	OpsPendingPhase    OpsPhase = "Pending"
	OpsCreatingPhase   OpsPhase = "Creating"
	OpsRunningPhase    OpsPhase = "Running"
	OpsCancellingPhase OpsPhase = "Cancelling"
	OpsSucceedPhase    OpsPhase = "Succeed"
	OpsCancelledPhase  OpsPhase = "Cancelled"
	OpsFailedPhase     OpsPhase = "Failed"
)

// PodSelectionPolicy pod selection strategy.
// +enum
// +kubebuilder:validation:Enum={All,Any}
type PodSelectionPolicy string

const (
	All PodSelectionPolicy = "All"
	Any PodSelectionPolicy = "Any"
)

// PodAvailabilityPolicy pod availability strategy.
// +enum
// +kubebuilder:validation:Enum={Available,PreferredAvailable,None}
type PodAvailabilityPolicy string

const (
	AvailablePolicy        PodAvailabilityPolicy = "Available"
	UnAvailablePolicy      PodAvailabilityPolicy = "UnAvailable"
	NoneAvailabilityPolicy PodAvailabilityPolicy = "None"
)

// OpsWorkloadType policy after action failure.
// +enum
// +kubebuilder:validation:Enum={Job,Pod}
type OpsWorkloadType string

const (
	PodWorkload OpsWorkloadType = "Pod"
	JobWorkload OpsWorkloadType = "Job"
)

// OpsType defines operation types.
// +enum
// +kubebuilder:validation:Enum={Upgrade,VerticalScaling,VolumeExpansion,HorizontalScaling,Restart,Reconfiguring,Start,Stop,Expose,Switchover,DataScript,Backup,Restore,Custom}
type OpsType string

const (
	VerticalScalingType   OpsType = "VerticalScaling"
	HorizontalScalingType OpsType = "HorizontalScaling"
	VolumeExpansionType   OpsType = "VolumeExpansion"
	UpgradeType           OpsType = "Upgrade"
	ReconfiguringType     OpsType = "Reconfiguring"
	SwitchoverType        OpsType = "Switchover"
	RestartType           OpsType = "Restart" // RestartType the restart operation is a special case of the rolling update operation.
	StopType              OpsType = "Stop"    // StopType the stop operation will delete all pods in a cluster concurrently.
	StartType             OpsType = "Start"   // StartType the start operation will start the pods which is deleted in stop operation.
	ExposeType            OpsType = "Expose"
	DataScriptType        OpsType = "DataScript" // DataScriptType the data script operation will execute the data script against the cluster.
	BackupType            OpsType = "Backup"
	RestoreType           OpsType = "Restore"
	CustomType            OpsType = "Custom" // use opsDefinition
)

// ComponentResourceKey defines the resource key of component, such as pod/pvc.
// +enum
// +kubebuilder:validation:Enum={pods}
type ComponentResourceKey string

const PodsCompResourceKey ComponentResourceKey = "pods"

// AccessMode defines the modes of access granted to the SVC.
// The modes can be `None`, `Readonly`, or `ReadWrite`.
//
// +enum
// +kubebuilder:validation:Enum={None,Readonly,ReadWrite}
type AccessMode string

const (
	// ReadWrite permits both read and write operations.
	ReadWrite AccessMode = "ReadWrite"

	// Readonly allows only read operations.
	Readonly AccessMode = "Readonly"

	// None implies no access.
	None AccessMode = "None"
)

// UpdateStrategy defines the update strategy for cluster components. This strategy determines how updates are applied
// across the cluster.
// The available strategies are `Serial`, `BestEffortParallel`, and `Parallel`.
//
// +enum
// +kubebuilder:validation:Enum={Serial,BestEffortParallel,Parallel}
type UpdateStrategy string

const (
	// SerialStrategy indicates that updates are applied one at a time in a sequential manner.
	SerialStrategy UpdateStrategy = "Serial"

	// ParallelStrategy indicates that updates are applied simultaneously across all components.
	ParallelStrategy UpdateStrategy = "Parallel"

	// BestEffortParallelStrategy indicates that updates are applied as quickly as possible, but not necessarily all at once.
	// This strategy attempts to strike a balance between speed and stability.
	BestEffortParallelStrategy UpdateStrategy = "BestEffortParallel"
)

var DefaultLeader = ConsensusMember{
	Name:       "leader",
	AccessMode: ReadWrite,
}

// WorkloadType defines the type of workload for the components of the ClusterDefinition.
// It can be one of the following: `Stateless`, `Stateful`, `Consensus`, or `Replication`.
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

var WorkloadTypes = []string{"Stateless", "Stateful", "Consensus", "Replication"}

// TerminationPolicyType defines termination policy types.
//
// +enum
// +kubebuilder:validation:Enum={DoNotTerminate,Halt,Delete,WipeOut}
type TerminationPolicyType string

const (
	// DoNotTerminate will block delete operation.
	DoNotTerminate TerminationPolicyType = "DoNotTerminate"

	// Halt will delete workload resources such as statefulset, deployment workloads but keep PVCs.
	Halt TerminationPolicyType = "Halt"

	// Delete is based on Halt and deletes PVCs.
	Delete TerminationPolicyType = "Delete"

	// WipeOut is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.
	WipeOut TerminationPolicyType = "WipeOut"
)

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

// PodAntiAffinity defines the pod anti-affinity strategy.
//
// This strategy determines how pods are scheduled in relation to other pods, with the aim of either spreading pods
// across nodes (Preferred) or ensuring that certain pods do not share a node (Required).
//
// +enum
// +kubebuilder:validation:Enum={Preferred,Required}
type PodAntiAffinity string

const (
	// Preferred indicates that the scheduler will try to enforce the anti-affinity rules, but it will not guarantee it.
	Preferred PodAntiAffinity = "Preferred"

	// Required indicates that the scheduler must enforce the anti-affinity rules and will not schedule the pods unless
	// the rules are met.
	Required PodAntiAffinity = "Required"
)

// TenancyType defines the type of tenancy for cluster tenant resources.
//
// +enum
// +kubebuilder:validation:Enum={SharedNode,DedicatedNode}
type TenancyType string

const (
	// SharedNode means multiple pods may share the same node.
	SharedNode TenancyType = "SharedNode"

	// DedicatedNode means each pod runs on their own dedicated node.
	DedicatedNode TenancyType = "DedicatedNode"
)

// AvailabilityPolicyType defines the type of availability policy to be applied for cluster affinity, influencing how
// resources are distributed across zones or nodes for high availability and resilience.
//
// +enum
// +kubebuilder:validation:Enum={zone,node,none}
type AvailabilityPolicyType string

const (
	// AvailabilityPolicyZone specifies that resources should be distributed across different availability zones.
	// This policy aims to ensure high availability and protect against zone failures, spreading the resources to reduce
	// the risk of simultaneous downtime.
	AvailabilityPolicyZone AvailabilityPolicyType = "zone"

	// AvailabilityPolicyNode specifies that resources should be distributed across different nodes within the same zone.
	// This policy aims to provide resilience against node failures, ensuring that the failure of a single node does not
	// impact the overall service availability.
	AvailabilityPolicyNode AvailabilityPolicyType = "node"

	// AvailabilityPolicyNone specifies that no specific availability policy is applied.
	// Resources may not be explicitly distributed for high availability, potentially concentrating them in a single
	// zone or node based on other scheduling decisions.
	AvailabilityPolicyNone AvailabilityPolicyType = "none"
)

// ProgressStatus defines the status of the opsRequest progress.
// +enum
// +kubebuilder:validation:Enum={Processing,Pending,Failed,Succeed}
type ProgressStatus string

const (
	PendingProgressStatus    ProgressStatus = "Pending"
	ProcessingProgressStatus ProgressStatus = "Processing"
	FailedProgressStatus     ProgressStatus = "Failed"
	SucceedProgressStatus    ProgressStatus = "Succeed"
)

// ActionTaskStatus defines the status of the task.
// +enum
// +kubebuilder:validation:Enum={Processing,Failed,Succeed}
type ActionTaskStatus string

const (
	ProcessingActionTaskStatus ActionTaskStatus = "Processing"
	FailedActionTaskStatus     ActionTaskStatus = "Failed"
	SucceedActionTaskStatus    ActionTaskStatus = "Succeed"
)

type OpsRequestBehaviour struct {
	FromClusterPhases []ClusterPhase
	ToClusterPhase    ClusterPhase
}

type OpsRecorder struct {
	// name OpsRequest name
	Name string `json:"name"`
	// opsRequest type
	Type OpsType `json:"type"`
	// indicates whether the current opsRequest is in the queue
	InQueue bool `json:"inQueue,omitempty"`
	// indicates that the operation is queued for execution within its own-type scope.
	QueueBySelf bool `json:"queueBySelf,omitempty"`
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

// KBAccountType is used for bitwise operation.
type KBAccountType uint8

// System accounts represented in bit.
const (
	KBAccountInvalid        KBAccountType = 0
	KBAccountAdmin                        = 1
	KBAccountDataprotection               = 1 << 1
	KBAccountProbe                        = 1 << 2
	KBAccountMonitor                      = 1 << 3
	KBAccountReplicator                   = 1 << 4
	KBAccountMAX                          = KBAccountReplicator // KBAccountMAX indicates the max value of KBAccountType, used for validation.
)

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

func (r AccountName) GetAccountID() KBAccountType {
	switch r {
	case AdminAccount:
		return KBAccountAdmin
	case DataprotectionAccount:
		return KBAccountDataprotection
	case ProbeAccount:
		return KBAccountProbe
	case MonitorAccount:
		return KBAccountMonitor
	case ReplicatorAccount:
		return KBAccountReplicator
	}
	return KBAccountInvalid
}

// LetterCase defines the available cases to be used in password generation.
//
// +enum
// +kubebuilder:validation:Enum={LowerCases,UpperCases,MixedCases}
type LetterCase string

const (
	// LowerCases represents the use of lower case letters only.
	LowerCases LetterCase = "LowerCases"

	// UpperCases represents the use of upper case letters only.
	UpperCases LetterCase = "UpperCases"

	// MixedCases represents the use of a mix of both lower and upper case letters.
	MixedCases LetterCase = "MixedCases"
)

var webhookMgr *webhookManager

type webhookManager struct {
	client client.Client
}

// CfgFileFormat defines formatter of configuration files.
// +enum
// +kubebuilder:validation:Enum={xml,ini,yaml,json,hcl,dotenv,toml,properties,redis,props-plus}
type CfgFileFormat string

const (
	Ini            CfgFileFormat = "ini"
	YAML           CfgFileFormat = "yaml"
	JSON           CfgFileFormat = "json"
	XML            CfgFileFormat = "xml"
	HCL            CfgFileFormat = "hcl"
	Dotenv         CfgFileFormat = "dotenv"
	TOML           CfgFileFormat = "toml"
	Properties     CfgFileFormat = "properties"
	RedisCfg       CfgFileFormat = "redis"
	PropertiesPlus CfgFileFormat = "props-plus"
)

// UpgradePolicy defines the policy of reconfiguring.
// +enum
// +kubebuilder:validation:Enum={simple,parallel,rolling,autoReload,operatorSyncUpdate,dynamicReloadBeginRestart}
type UpgradePolicy string

const (
	NonePolicy                    UpgradePolicy = "none"
	NormalPolicy                  UpgradePolicy = "simple"
	RestartPolicy                 UpgradePolicy = "parallel"
	RollingPolicy                 UpgradePolicy = "rolling"
	AsyncDynamicReloadPolicy      UpgradePolicy = "autoReload"
	SyncDynamicReloadPolicy       UpgradePolicy = "operatorSyncUpdate"
	DynamicReloadAndRestartPolicy UpgradePolicy = "dynamicReloadBeginRestart"
)

// CfgReloadType defines reload method.
// +enum
type CfgReloadType string

const (
	UnixSignalType CfgReloadType = "signal"
	SQLType        CfgReloadType = "sql"
	ShellType      CfgReloadType = "exec"
	HTTPType       CfgReloadType = "http"
	TPLScriptType  CfgReloadType = "tpl"
	AutoType       CfgReloadType = "auto"
)

// SignalType defines which signals are valid.
// +enum
// +kubebuilder:validation:Enum={SIGHUP,SIGINT,SIGQUIT,SIGILL,SIGTRAP,SIGABRT,SIGBUS,SIGFPE,SIGKILL,SIGUSR1,SIGSEGV,SIGUSR2,SIGPIPE,SIGALRM,SIGTERM,SIGSTKFLT,SIGCHLD,SIGCONT,SIGSTOP,SIGTSTP,SIGTTIN,SIGTTOU,SIGURG,SIGXCPU,SIGXFSZ,SIGVTALRM,SIGPROF,SIGWINCH,SIGIO,SIGPWR,SIGSYS}
type SignalType string

const (
	SIGHUP    SignalType = "SIGHUP"
	SIGINT    SignalType = "SIGINT"
	SIGQUIT   SignalType = "SIGQUIT"
	SIGILL    SignalType = "SIGILL"
	SIGTRAP   SignalType = "SIGTRAP"
	SIGABRT   SignalType = "SIGABRT"
	SIGBUS    SignalType = "SIGBUS"
	SIGFPE    SignalType = "SIGFPE"
	SIGKILL   SignalType = "SIGKILL"
	SIGUSR1   SignalType = "SIGUSR1"
	SIGSEGV   SignalType = "SIGSEGV"
	SIGUSR2   SignalType = "SIGUSR2"
	SIGPIPE   SignalType = "SIGPIPE"
	SIGALRM   SignalType = "SIGALRM"
	SIGTERM   SignalType = "SIGTERM"
	SIGSTKFLT SignalType = "SIGSTKFLT"
	SIGCHLD   SignalType = "SIGCHLD"
	SIGCONT   SignalType = "SIGCONT"
	SIGSTOP   SignalType = "SIGSTOP"
	SIGTSTP   SignalType = "SIGTSTP"
	SIGTTIN   SignalType = "SIGTTIN"
	SIGTTOU   SignalType = "SIGTTOU"
	SIGURG    SignalType = "SIGURG"
	SIGXCPU   SignalType = "SIGXCPU"
	SIGXFSZ   SignalType = "SIGXFSZ"
	SIGVTALRM SignalType = "SIGVTALRM"
	SIGPROF   SignalType = "SIGPROF"
	SIGWINCH  SignalType = "SIGWINCH"
	SIGIO     SignalType = "SIGIO"
	SIGPWR    SignalType = "SIGPWR"
	SIGSYS    SignalType = "SIGSYS"
)

// IssuerName defines the name of the TLS certificates issuer.
// +enum
// +kubebuilder:validation:Enum={KubeBlocks,UserProvided}
type IssuerName string

const (
	// IssuerKubeBlocks represents certificates that are signed by the KubeBlocks Operator.
	IssuerKubeBlocks IssuerName = "KubeBlocks"

	// IssuerUserProvided indicates that the user has provided their own CA-signed certificates.
	IssuerUserProvided IssuerName = "UserProvided"
)

// SwitchPolicyType defines the types of switch policies that can be applied to a cluster.
//
// Currently, only the Noop policy is supported. Support for MaximumAvailability and MaximumDataProtection policies is
// planned for future releases.
//
// +enum
// +kubebuilder:validation:Enum={Noop}
type SwitchPolicyType string

const (
	// MaximumAvailability represents a switch policy that aims for maximum availability. This policy will switch if the
	// primary is active and the synchronization delay is 0 according to the user-defined lagProbe data delay detection
	// logic. If the primary is down, it will switch immediately.
	// This policy is intended for future support.
	MaximumAvailability SwitchPolicyType = "MaximumAvailability"

	// MaximumDataProtection represents a switch policy focused on maximum data protection. This policy will only switch
	// if the primary is active and the synchronization delay is 0, based on the user-defined lagProbe data lag detection
	// logic. If the primary is down, it will switch only if it can be confirmed that the primary and secondary data are
	// consistent. Otherwise, it will not switch.
	// This policy is planned for future implementation.
	MaximumDataProtection SwitchPolicyType = "MaximumDataProtection"

	// Noop indicates that KubeBlocks will not perform any high-availability switching for the components. Users are
	// required to implement their own HA solution or integrate an existing open-source HA solution.
	Noop SwitchPolicyType = "Noop"
)

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

// BaseBackupType the base backup type, keep synchronized with the BaseBackupType of the data protection API.
//
// +enum
// +kubebuilder:validation:Enum={full,snapshot}
type BaseBackupType string

// BackupStatusUpdateStage defines the stage of backup status update.
//
// +enum
// +kubebuilder:validation:Enum={pre,post}
type BackupStatusUpdateStage string

func RegisterWebhookManager(mgr manager.Manager) {
	webhookMgr = &webhookManager{mgr.GetClient()}
}

type ComponentNameSet map[string]struct{}

var (
	ErrWorkloadTypeIsUnknown   = errors.New("workloadType is unknown")
	ErrWorkloadTypeIsStateless = errors.New("workloadType should not be stateless")
	ErrNotMatchingCompDef      = errors.New("not matching componentDefRef")
)

// StatefulSetWorkload interface
// +kubebuilder:object:generate=false
type StatefulSetWorkload interface {
	FinalStsUpdateStrategy() (appsv1.PodManagementPolicyType, appsv1.StatefulSetUpdateStrategy)
	GetUpdateStrategy() UpdateStrategy
}

type HostNetwork struct {
	// The list of container ports that are required by the component.
	//
	// +optional
	ContainerPorts []HostNetworkContainerPort `json:"containerPorts,omitempty"`

	// Set DNS policy for the component.
	// Defaults to "ClusterFirst".
	// Valid values are 'ClusterFirstWithHostNet', 'ClusterFirst', 'Default' or 'None'.
	// DNS parameters given in DNSConfig will be merged with the policy selected with DNSPolicy.
	// To have DNS options set along with hostNetwork, you have to specify DNS policy explicitly to 'ClusterFirstWithHostNet'.
	//
	// +optional
	DNSPolicy *corev1.DNSPolicy `json:"dnsPolicy,omitempty"`
}

type HostNetworkContainerPort struct {
	// Container specifies the target container within the pod.
	//
	// +required
	Container string `json:"container"`

	// Ports are named container ports within the specified container.
	// These container ports must be defined in the container for proper port allocation.
	//
	// +kubebuilder:validation:MinItems=1
	// +required
	Ports []string `json:"ports"`
}

// ClusterService defines the service of a cluster.
type ClusterService struct {
	Service `json:",inline"`

	// Extends the ServiceSpec.Selector by allowing the specification of a sharding name, which is defined in
	// cluster.spec.shardingSpecs[x].name, to be used as a selector for the service.
	// Note that this and the ComponentSelector are mutually exclusive and cannot be set simultaneously.
	//
	// +optional
	ShardingSelector string `json:"shardingSelector,omitempty"`

	// Extends the ServiceSpec.Selector by allowing the specification of a component, to be used as a selector for the service.
	// Note that this and the ShardingSelector are mutually exclusive and cannot be set simultaneously.
	//
	// +optional
	ComponentSelector string `json:"componentSelector,omitempty"`
}

type ComponentService struct {
	Service `json:",inline"`

	// GeneratePodOrdinalService indicates whether to create a corresponding Service for each Pod of the selected Component.
	// If sets to true, a set of Service will be automatically generated for each Pod. And Service.RoleSelector will be ignored.
	// They can be referred to by adding the PodOrdinal to the defined ServiceName with named pattern `$(Service.ServiceName)-$(PodOrdinal)`.
	// And the Service.Name will also be generated with named pattern `$(Service.Name)-$(PodOrdinal)`.
	// The PodOrdinal is zero-based, and the number of generated Services is equal to the number of replicas of the Component.
	// For example, a Service might be defined as follows:
	//
	// ```yaml
	// name: my-service
	// serviceName: my-service
	// generatePodOrdinalService: true
	// spec:
	//   type: NodePort
	//   ports:
	//   - name: http
	//     port: 80
	//     targetPort: 8080
	// ```
	//
	// Assuming that the Component has 3 replicas, then three services would be generated: my-service-0, my-service-1, and my-service-2, each pointing to its respective Pod.
	//
	// +kubebuilder:default=false
	// +optional
	GeneratePodOrdinalService bool `json:"generatePodOrdinalService,omitempty"`
}

type Service struct {
	// Name defines the name of the service.
	// otherwise, it indicates the name of the service.
	// Others can refer to this service by its name. (e.g., connection credential)
	// Cannot be updated.
	// +required
	Name string `json:"name"`

	// ServiceName defines the name of the underlying service object.
	// If not specified, the default service name with different patterns will be used:
	//
	// - CLUSTER_NAME: for cluster-level services
	// - CLUSTER_NAME-COMPONENT_NAME: for component-level services
	//
	// Only one default service name is allowed.
	// Cannot be updated.
	//
	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	// If ServiceType is LoadBalancer, cloud provider related parameters can be put here
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Spec defines the behavior of a service.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec corev1.ServiceSpec `json:"spec,omitempty"`

	// RoleSelector extends the ServiceSpec.Selector by allowing you to specify defined role as selector for the service.
	// if GeneratePodOrdinalService sets to true, RoleSelector will be ignored.
	// +optional
	RoleSelector string `json:"roleSelector,omitempty"`
}

// List of all the built-in variables provided by KubeBlocks.
// These variables are automatically available when building environment variables for Pods and Actions, as well as
// rendering templates for config and script. Users can directly use these variables without explicit declaration.
//
// Note: Dynamic variables have values that may change at runtime, so exercise caution when using them.
//
// TODO: resources.
// ----------------------------------------------------------------------------
// | Object    | Attribute | Variable             | Template | Env  | Dynamic |
// ----------------------------------------------------------------------------
// | Namespace |           | KB_NAMESPACE         |          |      |         |
// | Cluster   | Name      | KB_CLUSTER_NAME      |          |      |         |
// |           | UID       | KB_CLUSTER_UID       |          |      |         |
// |           | Component | KB_CLUSTER_COMP_NAME |          |      |         |
// | Component | Name      | KB_COMP_NAME         |          |      |         |
// |           | Replicas  | KB_COMP_REPLICAS     |          |      |    ✓    |
// ----------------------------------------------------------------------------

// EnvVar represents a variable present in the env of Pod/Action or the template of config/script.
type EnvVar struct {
	// Name of the variable. Must be a C_IDENTIFIER.
	// +required
	Name string `json:"name"`

	// Optional: no more than one of the following may be specified.

	// Variable references $(VAR_NAME) are expanded using the previously defined variables in the current context.
	//
	// If a variable cannot be resolved, the reference in the input string will be unchanged.
	// Double $$ are reduced to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e.
	//
	// - "$$(VAR_NAME)" will produce the string literal "$(VAR_NAME)".
	//
	// Escaped references will never be expanded, regardless of whether the variable exists or not.
	// Defaults to "".
	//
	// +optional
	Value string `json:"value,omitempty"`

	// Source for the variable's value. Cannot be used if value is not empty.
	// +optional
	ValueFrom *VarSource `json:"valueFrom,omitempty"`
}

// VarSource represents a source for the value of an EnvVar.
type VarSource struct {
	// Selects a key of a ConfigMap.
	// +optional
	ConfigMapKeyRef *corev1.ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`

	// Selects a key of a Secret.
	// +optional
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`

	// Selects a defined var of a Pod.
	// +optional
	PodVarRef *PodVarSelector `json:"podVarRef,omitempty"`

	// Selects a defined var of a Service.
	// +optional
	ServiceVarRef *ServiceVarSelector `json:"serviceVarRef,omitempty"`

	// Selects a defined var of a Credential (SystemAccount).
	// +optional
	CredentialVarRef *CredentialVarSelector `json:"credentialVarRef,omitempty"`

	// Selects a defined var of a ServiceRef.
	// +optional
	ServiceRefVarRef *ServiceRefVarSelector `json:"serviceRefVarRef,omitempty"`
}

// VarOption defines whether a variable is required or optional.
// +enum
// +kubebuilder:validation:Enum={Required,Optional}
type VarOption string

var (
	VarRequired VarOption = "Required"
	VarOptional VarOption = "Optional"
)

type NamedVar struct {
	// +optional
	Name string `json:"name,omitempty"`

	// +optional
	Option *VarOption `json:"option,omitempty"`
}

// PodVars defines the vars can be referenced from a Pod.
type PodVars struct {
	// +optional
	Container *ContainerVars `json:"container,omitempty"`
}

// ContainerVars defines the vars can be referenced from a Container.
type ContainerVars struct {
	// The name of the container.
	// +required
	Name string `json:"name"`

	// Container port to reference.
	// +optional
	Port *NamedVar `json:"port,omitempty"`
}

// ServiceVars defines the vars can be referenced from a Service.
type ServiceVars struct {
	// +optional
	Host *VarOption `json:"host,omitempty"`

	// +optional
	Port *NamedVar `json:"port,omitempty"`

	// +optional
	NodePort *NamedVar `json:"nodePort,omitempty"`
}

// CredentialVars defines the vars can be referenced from a Credential (SystemAccount).
// !!!!! CredentialVars will only be used as environment variables for Pods & Actions, and will not be used to render the templates.
type CredentialVars struct {
	// +optional
	Username *VarOption `json:"username,omitempty"`

	// +optional
	Password *VarOption `json:"password,omitempty"`
}

// ServiceRefVars defines the vars can be referenced from a ServiceRef.
type ServiceRefVars struct {
	// +optional
	Endpoint *VarOption `json:"endpoint,omitempty"`

	// +optional
	Port *VarOption `json:"port,omitempty"`

	CredentialVars `json:",inline"`
}

// PodVarSelector selects a var from a Pod.
type PodVarSelector struct {
	// The pod to select from.
	ClusterObjectReference `json:",inline"`

	PodVars `json:",inline"`
}

// ServiceVarSelector selects a var from a Service.
type ServiceVarSelector struct {
	// The Service to select from.
	// It can be referenced from the default headless service by setting the name to "headless".
	ClusterObjectReference `json:",inline"`

	ServiceVars `json:",inline"`

	// GeneratePodOrdinalServiceVar indicates whether to create a corresponding ServiceVars reference variable for each Pod.
	// If set to true, a set of ServiceVars that can be referenced will be automatically generated for each Pod Ordinal.
	// They can be referred to by adding the PodOrdinal to the defined name template with named pattern `$(Vars[x].Name)_$(PodOrdinal)`.
	// For example, a ServiceVarRef might be defined as follows:
	//
	// ```yaml
	//
	// name: MY_SERVICE_PORT
	// valueFrom:
	//   serviceVarRef:
	//     compDef: my-component-definition
	//     name: my-service
	//     optional: true
	//     generatePodOrdinalServiceVar: true
	//     port:
	//       name: redis-sentinel
	//
	// ```
	//
	// Assuming that the Component has 3 replicas, then you can reference the port of existing services named my-service-0, my-service-1,
	// and my-service-2 with $MY_SERVICE_PORT_0, $MY_SERVICE_PORT_1, and $MY_SERVICE_PORT_2, respectively.
	// It should be used in conjunction with Service.GeneratePodOrdinalService.
	// +kubebuilder:default=false
	// +optional
	GeneratePodOrdinalServiceVar bool `json:"generatePodOrdinalServiceVar,omitempty"`
}

// CredentialVarSelector selects a var from a Credential (SystemAccount).
type CredentialVarSelector struct {
	// The Credential (SystemAccount) to select from.
	ClusterObjectReference `json:",inline"`

	CredentialVars `json:",inline"`
}

// ServiceRefVarSelector selects a var from a ServiceRefDeclaration.
type ServiceRefVarSelector struct {
	// The ServiceRefDeclaration to select from.
	ClusterObjectReference `json:",inline"`

	ServiceRefVars `json:",inline"`
}

// ClusterObjectReference contains information to let you locate the referenced object inside the same cluster.
type ClusterObjectReference struct {
	// CompDef specifies the definition used by the component that the referent object resident in.
	// +optional
	CompDef string `json:"compDef,omitempty"`

	// Name of the referent object.
	// +optional
	Name string `json:"name,omitempty"`

	// Specify whether the object must be defined.
	// +optional
	Optional *bool `json:"optional,omitempty"`
}
