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

// Package v1alpha1 contains API Schema definitions for the apps v1alpha1 API group
package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	APIVersion            = "apps.kubeblocks.io/v1alpha1"
	ClusterVersionKind    = "ClusterVersion"
	ClusterDefinitionKind = "ClusterDefinition"
	ClusterKind           = "Cluster"
	OpsRequestKind        = "OpsRequestKind"
)

// Phase defines the CR .Status.Phase
// +enum
type Phase string

const (
	AvailablePhase         Phase = "Available"
	UnavailablePhase       Phase = "Unavailable"
	DeletingPhase          Phase = "Deleting"
	CreatingPhase          Phase = "Creating"
	PendingPhase           Phase = "Pending"
	RunningPhase           Phase = "Running"
	FailedPhase            Phase = "Failed"
	SpecUpdatingPhase      Phase = "SpecUpdating"
	VolumeExpandingPhase   Phase = "VolumeExpanding"
	HorizontalScalingPhase Phase = "HorizontalScaling"
	VerticalScalingPhase   Phase = "VerticalScaling"
	RebootingPhase         Phase = "Rebooting"
	VersionUpgradingPhase  Phase = "VersionUpgrading"
	SucceedPhase           Phase = "Succeed"
	AbnormalPhase          Phase = "Abnormal"
	ConditionsErrorPhase   Phase = "ConditionsError"
	ReconfiguringPhase     Phase = "Reconfiguring"
	StoppedPhase           Phase = "Stopped"
	StoppingPhase          Phase = "Stopping"
	StartingPhase          Phase = "Starting"
	ExposingPhase          Phase = "Exposing"
)

// OpsType defines operation types.
// +enum
// +kubebuilder:validation:Enum={Upgrade,VerticalScaling,VolumeExpansion,HorizontalScaling,Restart,Reconfiguring,Start,Stop,Expose}
type OpsType string

const (
	VerticalScalingType   OpsType = "VerticalScaling"
	HorizontalScalingType OpsType = "HorizontalScaling"
	VolumeExpansionType   OpsType = "VolumeExpansion"
	UpgradeType           OpsType = "Upgrade"
	ReconfiguringType     OpsType = "Reconfiguring"
	RestartType           OpsType = "Restart"
	StopType              OpsType = "Stop"
	StartType             OpsType = "Start"
	ExposeType            OpsType = "Expose"
)

// AccessMode define SVC access mode enums.
// +enum
// +kubebuilder:validation:Enum={None,Readonly,ReadWrite}
type AccessMode string

const (
	ReadWrite AccessMode = "ReadWrite"
	Readonly  AccessMode = "Readonly"
	None      AccessMode = "None"
)

// UpdateStrategy define Cluster Component update strategy.
// +enum
// +kubebuilder:validation:Enum={Serial,BestEffortParallel,Parallel}
type UpdateStrategy string

const (
	SerialStrategy             UpdateStrategy = "Serial"
	BestEffortParallelStrategy UpdateStrategy = "BestEffortParallel"
	ParallelStrategy           UpdateStrategy = "Parallel"
)

var DefaultLeader = ConsensusMember{
	Name:       "leader",
	AccessMode: ReadWrite,
}

// WorkloadType defines ClusterDefinition's component workload type.
// +enum
// +kubebuilder:validation:Enum={Stateless,Stateful,Consensus,Replication}
type WorkloadType string

const (
	Stateless   WorkloadType = "Stateless"
	Stateful    WorkloadType = "Stateful"
	Consensus   WorkloadType = "Consensus"
	Replication WorkloadType = "Replication"
)

var WorkloadTypes = []string{"Stateless", "Stateful", "Consensus", "Replication"}

// TerminationPolicyType define termination policy types.
// +enum
// +kubebuilder:validation:Enum={DoNotTerminate,Halt,Delete,WipeOut}
type TerminationPolicyType string

const (
	DoNotTerminate TerminationPolicyType = "DoNotTerminate"
	Halt           TerminationPolicyType = "Halt"
	Delete         TerminationPolicyType = "Delete"
	WipeOut        TerminationPolicyType = "WipeOut"
)

// HScaleDataClonePolicyType defines data clone policy when horizontal scaling.
// +enum
// +kubebuilder:validation:Enum={None,Snapshot}
type HScaleDataClonePolicyType string

const (
	HScaleDataClonePolicyNone         HScaleDataClonePolicyType = "None"
	HScaleDataClonePolicyFromSnapshot HScaleDataClonePolicyType = "Snapshot"
	HScaleDataClonePolicyFromBackup   HScaleDataClonePolicyType = "Backup"
)

// PodAntiAffinity define pod anti-affinity strategy.
// +enum
// +kubebuilder:validation:Enum={Preferred,Required}
type PodAntiAffinity string

const (
	Preferred PodAntiAffinity = "Preferred"
	Required  PodAntiAffinity = "Required"
)

// TenancyType for cluster tenant resources.
// +enum
// +kubebuilder:validation:Enum={SharedNode,DedicatedNode}
type TenancyType string

const (
	SharedNode    TenancyType = "SharedNode"
	DedicatedNode TenancyType = "DedicatedNode"
)

// ProgressStatus defined
// +enum
// +kubebuilder:validation:Enum={Processing,Pending,Failed,Succeed}
type ProgressStatus string

const (
	PendingProgressStatus    ProgressStatus = "Pending"
	ProcessingProgressStatus ProgressStatus = "Processing"
	FailedProgressStatus     ProgressStatus = "Failed"
	SucceedProgressStatus    ProgressStatus = "Succeed"
)

// OpsRequestBehaviour record what cluster status that can trigger this OpsRequest
// and what status that the cluster enters after trigger OpsRequest.
type OpsRequestBehaviour struct {
	FromClusterPhases []Phase
	ToClusterPhase    Phase
}

// OpsRecorder recorder the running OpsRequest info in cluster annotation
type OpsRecorder struct {
	// Name OpsRequest name
	Name string `json:"name"`
	// ToClusterPhase the cluster phase when the OpsRequest is running
	ToClusterPhase Phase `json:"clusterPhase"`
}

// ProvisionPolicyType defines the policy for creating accounts.
// +enum
type ProvisionPolicyType string

const (
	// CreateByStmt will create account w.r.t. deleteion and creation statement given by provider.
	CreateByStmt ProvisionPolicyType = "CreateByStmt"
	// ReferToExisting will not create account, but create a secret by copying data from referred secret file.
	ReferToExisting ProvisionPolicyType = "ReferToExisting"
)

// ProvisionScope defines the scope (within component) of provision.
// +enum
type ProvisionScope string

const (
	// AllPods will create accounts for all pods belong to the component.
	AllPods ProvisionScope = "AllPods"
	// AndyPods will only create accounts on one pod.
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

// LetterCase defines cases to use in password generation.
// +enum
type LetterCase string

const (
	LowerCases LetterCase = "LowerCases"
	UpperCases LetterCase = "UpperCases"
	MixedCases LetterCase = "MixedCases"
)

var webhookMgr *webhookManager

type webhookManager struct {
	client client.Client
}

// CfgFileFormat defines formatter of configuration files.
// +enum
// +kubebuilder:validation:Enum={xml,ini,yaml,json,hcl,dotenv,toml,properties}
type CfgFileFormat string

const (
	Ini        CfgFileFormat = "ini"
	YAML       CfgFileFormat = "yaml"
	JSON       CfgFileFormat = "json"
	XML        CfgFileFormat = "xml"
	HCL        CfgFileFormat = "hcl"
	Dotenv     CfgFileFormat = "dotenv"
	TOML       CfgFileFormat = "toml"
	Properties CfgFileFormat = "properties"

	// RedisCfg support redis config.
	RedisCfg CfgFileFormat = "redis"
)

// UpgradePolicy defines the policy of reconfiguring.
// +enum
// +kubebuilder:validation:Enum={simple,parallel,rolling,autoReload}
type UpgradePolicy string

const (
	NormalPolicy  UpgradePolicy = "simple"
	RestartPolicy UpgradePolicy = "parallel"
	RollingPolicy UpgradePolicy = "rolling"
	AutoReload    UpgradePolicy = "autoReload"
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

// IssuerName defines Tls certs issuer name
// +enum
type IssuerName string

const (
	// IssuerKubeBlocks Certificates signed by KubeBlocks Operator.
	IssuerKubeBlocks IssuerName = "KubeBlocks"
	// IssuerUserProvided User provided own CA-signed certificates.
	IssuerUserProvided IssuerName = "UserProvided"
)

// SwitchPolicyType defines switchPolicy type.
// +enum
// +kubebuilder:validation:Enum={MaximumAvailability, MaximumDataProtection, Noop}
type SwitchPolicyType string

const (
	MaximumAvailability   SwitchPolicyType = "MaximumAvailability"
	MaximumDataProtection SwitchPolicyType = "MaximumDataProtection"
	Noop                  SwitchPolicyType = "Noop"
)

// SwitchStepRole defines the role to execute the switch command.
// +enum
// +kubebuilder:validation:Enum={NewPrimary, OldPrimary, Secondaries}
type SwitchStepRole string

const (
	NewPrimary  SwitchStepRole = "NewPrimary"
	OldPrimary  SwitchStepRole = "OldPrimary"
	Secondaries SwitchStepRole = "Secondaries"
)

// VolumeType defines volume type for backup data or log.
// +enum
// +kubebuilder:validation:Enum={data,log}
type VolumeType string

const (
	VolumeTypeData VolumeType = "data"
	VolumeTypeLog  VolumeType = "log"
)

func RegisterWebhookManager(mgr manager.Manager) {
	webhookMgr = &webhookManager{mgr.GetClient()}
}
