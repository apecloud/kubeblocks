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

// Package v1alpha1 contains API Schema definitions for the dbaas v1alpha1 API group
package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	APIVersion            = "dbaas.kubeblocks.io/v1alpha1"
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
)

// Status define CR .Status.ClusterDefSyncStatus
// +enum
type Status string

const (
	OutOfSyncStatus Status = "OutOfSync"
	InSyncStatus    Status = "InSync"
)

// OpsType defines operation types.
// +enum
type OpsType string

const (
	VerticalScalingType   OpsType = "VerticalScaling"
	HorizontalScalingType OpsType = "HorizontalScaling"
	VolumeExpansionType   OpsType = "VolumeExpansion"
	UpgradeType           OpsType = "Upgrade"
	ReconfiguringType     OpsType = "Reconfiguring"
	RestartType           OpsType = "Restart"
)

// AccessMode define SVC access mode enums.
// +enum
type AccessMode string

const (
	ReadWrite AccessMode = "ReadWrite"
	Readonly  AccessMode = "Readonly"
	None      AccessMode = "None"
)

// UpdateStrategy define Cluster Component update strategy.
// +enum
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

// ComponentType defines ClusterDefinition's component type.
// +enum
type ComponentType string

const (
	Stateless ComponentType = "Stateless"
	Stateful  ComponentType = "Stateful"
	Consensus ComponentType = "Consensus"
)

// TerminationPolicyType define termination policy types.
// +enum
type TerminationPolicyType string

const (
	DoNotTerminate TerminationPolicyType = "DoNotTerminate"
	Halt           TerminationPolicyType = "Halt"
	Delete         TerminationPolicyType = "Delete"
	WipeOut        TerminationPolicyType = "WipeOut"
)

// HScaleDataClonePolicyType defines data clone policy when horizontal scaling.
// +enum
type HScaleDataClonePolicyType string

const (
	HScaleDataClonePolicyNone         HScaleDataClonePolicyType = "None"
	HScaleDataClonePolicyFromSnapshot HScaleDataClonePolicyType = "Snapshot"
	HScaleDataClonePolicyFromBackup   HScaleDataClonePolicyType = "Backup"
)

// PodAntiAffinity define pod anti-affinity strategy.
// +enum
type PodAntiAffinity string

const (
	Preferred PodAntiAffinity = "Preferred"
	Required  PodAntiAffinity = "Required"
)

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

type ScopeType string

const (
	ScopeBothType   ScopeType = "ScopeBoth"
	ScopeFileType   ScopeType = "ScopeFile"
	ScopeMemoryType ScopeType = "ScopeMemory"
)

type ConfigurationFormatter string

const (
	INI    ConfigurationFormatter = "ini"
	YAML   ConfigurationFormatter = "yaml"
	JSON   ConfigurationFormatter = "json"
	XML    ConfigurationFormatter = "xml"
	HCL    ConfigurationFormatter = "hcl"
	DOTENV ConfigurationFormatter = "dotenv"
)

type UpgradePolicy string

const (
	NormalPolicy  UpgradePolicy = "simple"
	RestartPolicy UpgradePolicy = "parallel"
	RollingPolicy UpgradePolicy = "rolling"
	AutoReload    UpgradePolicy = "autoReload"
)

type CfgReloadType string

const (
	UnixSignalType CfgReloadType = "signal"
	SQLType        CfgReloadType = "sql"
	ShellType      CfgReloadType = "exec"
	HTTPType       CfgReloadType = "http"
)

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

// ConfigTplType defines the purpose of the configuration template.
// +enum
type ConfigTplType string

const (
	ConfigurationType ConfigTplType = "config"
	ScriptType        ConfigTplType = "script"
)

func RegisterWebhookManager(mgr manager.Manager) {
	webhookMgr = &webhookManager{mgr.GetClient()}
}
