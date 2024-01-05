/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
	// +kubebuilder:default="default"
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// volumeName is the volume name of PodTemplate, which the configuration file produced through the configuration
	// template will be mounted to the corresponding volume. Must be a DNS_LABEL name.
	// The volume name must be defined in podSpec.containers[*].volumeMounts.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
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

type ConfigTemplateExtension struct {
	// Specify the name of the referenced the configuration template ConfigMap object.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	TemplateRef string `json:"templateRef"`

	// Specify the namespace of the referenced the configuration template ConfigMap object.
	// An empty namespace is equivalent to the "default" namespace.
	// +kubebuilder:default="default"
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// policy defines how to merge external imported templates into component templates.
	// +kubebuilder:default="none"
	// +optional
	Policy MergedPolicy `json:"policy,omitempty"`
}

type LegacyRenderedTemplateSpec struct {
	ConfigTemplateExtension `json:",inline"`
}

type ComponentConfigSpec struct {
	ComponentTemplateSpec `json:",inline"`

	// Specify a list of keys.
	// If empty, ConfigConstraint takes effect for all keys in configmap.
	// +listType=set
	// +optional
	Keys []string `json:"keys,omitempty"`

	// lazyRenderedConfigSpec is optional: specify the secondary rendered config spec.
	// +optional
	LegacyRenderedConfigSpec *LegacyRenderedTemplateSpec `json:"legacyRenderedConfigSpec,omitempty"`

	// Specify the name of the referenced the configuration constraints object.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	ConfigConstraintRef string `json:"constraintRef,omitempty"`

	// asEnvFrom is optional: the list of containers will be injected into EnvFrom.
	// +listType=set
	// +optional
	AsEnvFrom []string `json:"asEnvFrom,omitempty"`
}

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

// ClusterPhase defines the Cluster CR .status.phase
// +enum
// +kubebuilder:validation:Enum={Creating,Running,Updating,Stopping,Stopped,Deleting,Failed,Abnormal}
type ClusterPhase string

const (
	CreatingClusterPhase ClusterPhase = "Creating"
	RunningClusterPhase  ClusterPhase = "Running"
	UpdatingClusterPhase ClusterPhase = "Updating"
	StoppingClusterPhase ClusterPhase = "Stopping"
	StoppedClusterPhase  ClusterPhase = "Stopped"
	DeletingClusterPhase ClusterPhase = "Deleting"
	FailedClusterPhase   ClusterPhase = "Failed"
	AbnormalClusterPhase ClusterPhase = "Abnormal"
)

// ClusterComponentPhase defines the Cluster CR .status.components.phase
// +enum
// +kubebuilder:validation:Enum={Creating,Running,Updating,Stopping,Stopped,Deleting,Failed,Abnormal}
type ClusterComponentPhase string

const (
	CreatingClusterCompPhase ClusterComponentPhase = "Creating"
	RunningClusterCompPhase  ClusterComponentPhase = "Running"
	UpdatingClusterCompPhase ClusterComponentPhase = "Updating"
	StoppingClusterCompPhase ClusterComponentPhase = "Stopping"
	StoppedClusterCompPhase  ClusterComponentPhase = "Stopped"
	DeletingClusterCompPhase ClusterComponentPhase = "Deleting"
	FailedClusterCompPhase   ClusterComponentPhase = "Failed"
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

// PodSelectionStrategy pod selection strategy.
// +enum
// +kubebuilder:validation:Enum={Available,PreferredAvailable}
type PodSelectionStrategy string

const (
	Available          PodSelectionStrategy = "Available"
	PreferredAvailable PodSelectionStrategy = "PreferredAvailable"
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
	ParallelStrategy           UpdateStrategy = "Parallel"
	BestEffortParallelStrategy UpdateStrategy = "BestEffortParallel"
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
// +kubebuilder:validation:Enum={None,CloneVolume,Snapshot}
type HScaleDataClonePolicyType string

const (
	HScaleDataClonePolicyNone         HScaleDataClonePolicyType = "None"
	HScaleDataClonePolicyCloneVolume  HScaleDataClonePolicyType = "CloneVolume"
	HScaleDataClonePolicyFromSnapshot HScaleDataClonePolicyType = "Snapshot"
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

// AvailabilityPolicyType for cluster affinity policy.
// +enum
// +kubebuilder:validation:Enum={zone,node,none}
type AvailabilityPolicyType string

const (
	AvailabilityPolicyZone AvailabilityPolicyType = "zone"
	AvailabilityPolicyNode AvailabilityPolicyType = "node"
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
	// AnyPods will only create accounts on one pod.
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
// Currently, only Noop is supported. MaximumAvailability and MaximumDataProtection will be supported in the future.
// +enum
// +kubebuilder:validation:Enum={Noop}
type SwitchPolicyType string

const (
	MaximumAvailability   SwitchPolicyType = "MaximumAvailability"
	MaximumDataProtection SwitchPolicyType = "MaximumDataProtection"
	Noop                  SwitchPolicyType = "Noop"
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

type ClusterService struct {
	Service `json:",inline"`

	// ComponentSelector extends the ServiceSpec.Selector by allowing you to specify a component as selectors for the service.
	// +optional
	ComponentSelector string `json:"componentSelector,omitempty"`
}

type ComponentService struct {
	Service `json:",inline"`

	// GeneratePodOrdinalService indicates whether to create a corresponding Service for each Pod of the selected Component.
	// If sets to true, a set of Service will be automatically generated for each Pod. And Service.RoleSelector will be ignored.
	// They can be referred to by adding the PodOrdinal to the defined ServiceName with named pattern <Service.ServiceName>-<PodOrdinal>.
	// And the Service.Name will also be generated with named pattern <Service.Name>-<PodOrdinal>.
	// The PodOrdinal is zero-based, and the number of generated Services is equal to the number of replicas of the Component.
	// For example, a Service might be defined as follows:
	// - name: my-service
	//   serviceName: my-service
	//   generatePodOrdinalService: true
	//   spec:
	//     type: NodePort
	//     ports:
	//     - name: http
	//       port: 80
	//       targetPort: 8080
	// Assuming that the Component has 3 replicas, then three services would be generated: my-service-0, my-service-1, and my-service-2, each pointing to its respective Pod.
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
	//  - <CLUSTER_NAME>: for cluster-level services
	//  - <CLUSTER_NAME>-<COMPONENT_NAME>: for component-level services
	// Only one default service name is allowed.
	// Cannot be updated.
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
// ---------------------------------------------------------------------------------------
// | Object    | Attribute | Variable             | Template | Env  | Dynamic | Provided |
// ---------------------------------------------------------------------------------------
// | Namespace |           | KB_NAMESPACE         |          |      |         |          |
// | Cluster   | Name      | KB_CLUSTER_NAME      |          |      |         |          |
// |           | UID       | KB_CLUSTER_UID       |          |      |         |          |
// |           | Component | KB_CLUSTER_COMP_NAME |          |      |         |          |
// | Component | Name      | KB_COMP_NAME         |          |      |         |          |
// |           | Replicas  | KB_COMP_REPLICAS     |          |      |    ✓    |          |
// | Pod       | Name      | KB_POD_NAME          |     x    |      |    ✓    |          |
// |           | UID       | KB_POD_UID           |     x    |      |    ✓    |          |
// |           | IP        | KB_POD_IP            |     x    |      |    ✓    |          |
// |           | IPs       | KB_POD_IPS           |     x    |      |    ✓    |          |
// |           | FQDN      | KB_POD_FQDN          |     x    |      |         |          |
// |           | Ordinal   | KB_POD_ORDINAL       |     x    |      |         |          |
// | Host      | Name      | KB_NODENAME          |     x    |      |    ✓    |          |
// |           | IP        | KB_HOST_IP           |     x    |      |    ✓    |          |
// | SA        | Name      | KB_SA_NAME           |     x    |      |         |          |
// ---------------------------------------------------------------------------------------

// EnvVar represents a variable present in the env of Pod/Action or the template of config/script.
type EnvVar struct {
	// Name of the variable. Must be a C_IDENTIFIER.
	// +required
	Name string `json:"name"`

	// Optional: no more than one of the following may be specified.

	// Variable references $(VAR_NAME) are expanded using the previously defined variables in the current context.
	// If a variable cannot be resolved, the reference in the input string will be unchanged.
	// Double $$ are reduced to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e.
	// "$$(VAR_NAME)" will produce the string literal "$(VAR_NAME)".
	// Escaped references will never be expanded, regardless of whether the variable exists or not.
	// Defaults to "".
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

// ServiceVarSelector selects a var from a Service.
type ServiceVarSelector struct {
	// The Service to select from.
	// It can be referenced from the default headless service by setting the name to "headless".
	ClusterObjectReference `json:",inline"`

	ServiceVars `json:",inline"`

	// GeneratePodOrdinalServiceVar indicates whether to create a corresponding ServiceVars reference variable for each Pod.
	// If set to true, a set of ServiceVars that can be referenced will be automatically generated for each Pod Ordinal.
	// They can be referred to by adding the PodOrdinal to the defined name template with named pattern $<Vars[x].Name>_<PodOrdinal>.
	// For example, a ServiceVarRef might be defined as follows:
	// - name: MY_SERVICE_PORT
	//   valueFrom:
	//     serviceVarRef:
	//       compDef: my-component-definition
	//       name: my-service
	//       optional: true
	//       generatePodOrdinalServiceVar: true
	//       port:
	//         name: redis-sentinel
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
