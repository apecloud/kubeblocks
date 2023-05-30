/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

// Package v1alpha1 contains API Schema definitions for the apps v1alpha1 API group
package v1alpha1

import (
	"errors"

	appsv1 "k8s.io/api/apps/v1"
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
	// +listType=set
	// +optional
	Keys []string `json:"keys,omitempty"`

	// Specify the name of the referenced the configuration constraints object.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	ConfigConstraintRef string `json:"constraintRef,omitempty"`
}

// ClusterPhase defines the Cluster CR .status.phase
// +enum
// +kubebuilder:validation:Enum={Running,Stopped,Failed,Abnormal,Creating,Updating}
type ClusterPhase string

const (
	// REVIEW/TODO: AbnormalClusterPhase provides hybrid, consider remove it if possible
	RunningClusterPhase         ClusterPhase = "Running"
	StoppedClusterPhase         ClusterPhase = "Stopped"
	FailedClusterPhase          ClusterPhase = "Failed"
	AbnormalClusterPhase        ClusterPhase = "Abnormal" // Abnormal is a sub-state of failed, where one of the cluster components has "Failed" or "Abnormal" status phase.
	CreatingClusterPhase        ClusterPhase = "Creating"
	SpecReconcilingClusterPhase ClusterPhase = "Updating"
	// DeletingClusterPhase        ClusterPhase = "Deleting" // DO REVIEW: may merged with  Stopping
)

// ClusterComponentPhase defines the Cluster CR .status.components.phase
// +enum
// +kubebuilder:validation:Enum={Running,Stopped,Failed,Abnormal,Creating,Updating}
type ClusterComponentPhase string

const (
	RunningClusterCompPhase         ClusterComponentPhase = "Running"
	StoppedClusterCompPhase         ClusterComponentPhase = "Stopped"
	FailedClusterCompPhase          ClusterComponentPhase = "Failed"
	AbnormalClusterCompPhase        ClusterComponentPhase = "Abnormal" // Abnormal is a sub-state of failed, where one or more workload pods is not in "Running" phase.
	SpecReconcilingClusterCompPhase ClusterComponentPhase = "Updating"
	CreatingClusterCompPhase        ClusterComponentPhase = "Creating"
	// DeletingClusterCompPhase        ClusterComponentPhase = "Deleting" // DO REVIEW: may merged with  Stopping
)

const (
	// define the cluster condition type
	ConditionTypeLatestOpsRequestProcessed = "LatestOpsRequestProcessed" // ConditionTypeLatestOpsRequestProcessed describes whether the latest OpsRequest that affect the cluster lifecycle has been processed.
	ConditionTypeHaltRecovery              = "HaltRecovery"              // ConditionTypeHaltRecovery describe Halt recovery processing stage
	ConditionTypeProvisioningStarted       = "ProvisioningStarted"       // ConditionTypeProvisioningStarted the operator starts resource provisioning to create or change the cluster
	ConditionTypeApplyResources            = "ApplyResources"            // ConditionTypeApplyResources the operator start to apply resources to create or change the cluster
	ConditionTypeReplicasReady             = "ReplicasReady"             // ConditionTypeReplicasReady all pods of components are ready
	ConditionTypeReady                     = "Ready"                     // ConditionTypeReady all components are running

)

// Phase defines the ClusterDefinition and ClusterVersion  CR .status.phase
// +enum
// +kubebuilder:validation:Enum={Available,Unavailable}
type Phase string

const (
	AvailablePhase   Phase = "Available"
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

// OpsPhase defines opsRequest phase.
// +enum
// +kubebuilder:validation:Enum={Pending,Creating,Running,Failed,Succeed}
type OpsPhase string

const (
	OpsPendingPhase  OpsPhase = "Pending"
	OpsCreatingPhase OpsPhase = "Creating"
	OpsRunningPhase  OpsPhase = "Running"
	OpsFailedPhase   OpsPhase = "Failed"
	OpsSucceedPhase  OpsPhase = "Succeed"
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
	RestartType           OpsType = "Restart" // RestartType the restart operation is a special case of the rolling update operation.
	StopType              OpsType = "Stop"    // StopType the stop operation will delete all pods in a cluster concurrently.
	StartType             OpsType = "Start"   // StartType the start operation will start the pods which is deleted in stop operation.
	ExposeType            OpsType = "Expose"
)

// ComponentResourceKey defines the resource key of component, such as pod/pvc.
// +enum
// +kubebuilder:validation:Enum={pods}
type ComponentResourceKey string

const PodsCompResourceKey ComponentResourceKey = "pods"

// AccessMode defines SVC access mode enums.
// +enum
// +kubebuilder:validation:Enum={None,Readonly,ReadWrite}
type AccessMode string

const (
	ReadWrite AccessMode = "ReadWrite"
	Readonly  AccessMode = "Readonly"
	None      AccessMode = "None"
)

// UpdateStrategy defines Cluster Component update strategy.
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

// TerminationPolicyType defines termination policy types.
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

// PodAntiAffinity defines pod anti-affinity strategy.
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

type OpsRequestBehaviour struct {
	FromClusterPhases                  []ClusterPhase
	ToClusterPhase                     ClusterPhase
	ProcessingReasonInClusterCondition string
}

type OpsRecorder struct {
	// name OpsRequest name
	Name string `json:"name"`
	// clusterPhase the cluster phase when the OpsRequest is running
	Type OpsType `json:"type"`
}

// ProvisionPolicyType defines the policy for creating accounts.
// +enum
type ProvisionPolicyType string

const (
	// CreateByStmt will create account w.r.t. deletion and creation statement given by provider.
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
// +kubebuilder:validation:Enum={xml,ini,yaml,json,hcl,dotenv,toml,properties,redis}
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
	RedisCfg   CfgFileFormat = "redis"
)

// UpgradePolicy defines the policy of reconfiguring.
// +enum
// +kubebuilder:validation:Enum={simple,parallel,rolling,autoReload,operatorSyncUpdate}
type UpgradePolicy string

const (
	NonePolicy         UpgradePolicy = "none"
	NormalPolicy       UpgradePolicy = "simple"
	RestartPolicy      UpgradePolicy = "parallel"
	RollingPolicy      UpgradePolicy = "rolling"
	AutoReload         UpgradePolicy = "autoReload"
	OperatorSyncUpdate UpgradePolicy = "operatorSyncUpdate"
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

// BaseBackupType the base backup type, keep synchronized with the BaseBackupType of the data protection API.
// +enum
// +kubebuilder:validation:Enum={full,snapshot}
type BaseBackupType string

// BackupStatusUpdateStage defines the stage of backup status update.
// +enum
// +kubebuilder:validation:Enum={pre,post}
type BackupStatusUpdateStage string

// CandidateOperator is the set of operators that can be used in a candidateInstance spec.
// +enum
// +kubebuilder:validation:Enum={Equal,NotEqual}
type CandidateOperator string

const (
	CandidateOpEqual    CandidateOperator = "Equal"
	CandidateOpNotEqual CandidateOperator = "NotEqual"
)

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
