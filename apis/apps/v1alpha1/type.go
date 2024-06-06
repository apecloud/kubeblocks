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

	defaultInstanceTemplateReplicas = 1
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

	// The operator attempts to set default file permissions for scripts (0555) and configurations (0444).
	// However, certain database engines may require different file permissions.
	// You can specify the desired file permissions here.
	//
	// Must be specified as an octal value between 0000 and 0777 (inclusive),
	// or as a decimal value between 0 and 511 (inclusive).
	// YAML supports both octal and decimal values for file permissions.
	//
	// Please note that this setting only affects the permissions of the files themselves.
	// Directories within the specified path are not impacted by this setting.
	// It's important to be aware that this setting might conflict with other options
	// that influence the file mode, such as fsGroup.
	// In such cases, the resulting file mode may have additional bits set.
	// Refers to documents of k8s.ConfigMapVolumeSource.defaultMode for more information.
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

// LegacyRenderedTemplateSpec describes the configuration extension for the lazy rendered template.
// Deprecated: LegacyRenderedTemplateSpec has been deprecated since 0.9.0 and will be removed in 0.10.0
type LegacyRenderedTemplateSpec struct {
	// Extends the configuration template.
	ConfigTemplateExtension `json:",inline"`
}

type ComponentConfigSpec struct {
	ComponentTemplateSpec `json:",inline"`

	// Specifies the configuration files within the ConfigMap that support dynamic updates.
	//
	// A configuration template (provided in the form of a ConfigMap) may contain templates for multiple
	// configuration files.
	// Each configuration file corresponds to a key in the ConfigMap.
	// Some of these configuration files may support dynamic modification and reloading without requiring
	// a pod restart.
	//
	// If empty or omitted, all configuration files in the ConfigMap are assumed to support dynamic updates,
	// and ConfigConstraint applies to all keys.
	//
	// +listType=set
	// +optional
	Keys []string `json:"keys,omitempty"`

	// Specifies the secondary rendered config spec for pod-specific customization.
	//
	// The template is rendered inside the pod (by the "config-manager" sidecar container) and merged with the main
	// template's render result to generate the final configuration file.
	//
	// This field is intended to handle scenarios where different pods within the same Component have
	// varying configurations. It allows for pod-specific customization of the configuration.
	//
	// Note: This field will be deprecated in future versions, and the functionality will be moved to
	// `cluster.spec.componentSpecs[*].instances[*]`.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0 and will be removed in 0.10.0"
	// +optional
	LegacyRenderedConfigSpec *LegacyRenderedTemplateSpec `json:"legacyRenderedConfigSpec,omitempty"`

	// Specifies the name of the referenced configuration constraints object.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	ConfigConstraintRef string `json:"constraintRef,omitempty"`

	// Specifies the containers to inject the ConfigMap parameters as environment variables.
	//
	// This is useful when application images accept parameters through environment variables and
	// generate the final configuration file in the startup script based on these variables.
	//
	// This field allows users to specify a list of container names, and KubeBlocks will inject the environment
	// variables converted from the ConfigMap into these designated containers. This provides a flexible way to
	// pass the configuration items from the ConfigMap to the container without modifying the image.
	//
	// Deprecated: `asEnvFrom` has been deprecated since 0.9.0 and will be removed in 0.10.0.
	// Use `injectEnvTo` instead.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0 and will be removed in 0.10.0"
	// +listType=set
	// +optional
	AsEnvFrom []string `json:"asEnvFrom,omitempty"`

	// Specifies the containers to inject the ConfigMap parameters as environment variables.
	//
	// This is useful when application images accept parameters through environment variables and
	// generate the final configuration file in the startup script based on these variables.
	//
	// This field allows users to specify a list of container names, and KubeBlocks will inject the environment
	// variables converted from the ConfigMap into these designated containers. This provides a flexible way to
	// pass the configuration items from the ConfigMap to the container without modifying the image.
	//
	//
	// +listType=set
	// +optional
	InjectEnvTo []string `json:"injectEnvTo,omitempty"`

	// Specifies whether the configuration needs to be re-rendered after v-scale or h-scale operations to reflect changes.
	//
	// In some scenarios, the configuration may need to be updated to reflect the changes in resource allocation
	// or cluster topology. Examples:
	//
	// - Redis: adjust maxmemory after v-scale operation.
	// - MySQL: increase max connections after v-scale operation.
	// - Zookeeper: update zoo.cfg with new node addresses after h-scale operation.
	//
	// +listType=set
	// +optional
	ReRenderResourceTypes []RerenderResourceType `json:"reRenderResourceTypes,omitempty"`
}

// RerenderResourceType defines the resource requirements for a component.
// +enum
// +kubebuilder:validation:Enum={vscale,hscale,tls}
type RerenderResourceType string

const (
	ComponentVScaleType RerenderResourceType = "vscale"
	ComponentHScaleType RerenderResourceType = "hscale"
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

// OpsPhase defines opsRequest phase.
// +enum
// +kubebuilder:validation:Enum={Pending,Creating,Running,Cancelling,Cancelled,Aborted,Failed,Succeed}
type OpsPhase string

const (
	OpsPendingPhase    OpsPhase = "Pending"
	OpsCreatingPhase   OpsPhase = "Creating"
	OpsRunningPhase    OpsPhase = "Running"
	OpsCancellingPhase OpsPhase = "Cancelling"
	OpsSucceedPhase    OpsPhase = "Succeed"
	OpsCancelledPhase  OpsPhase = "Cancelled"
	OpsFailedPhase     OpsPhase = "Failed"
	OpsAbortedPhase    OpsPhase = "Aborted"
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
// +kubebuilder:validation:Enum={Upgrade,VerticalScaling,VolumeExpansion,HorizontalScaling,Restart,Reconfiguring,Start,Stop,Expose,Switchover,DataScript,Backup,Restore,RebuildInstance,Custom}
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
	RebuildInstanceType   OpsType = "RebuildInstance" // RebuildInstance rebuilding an instance is very useful when a node is offline or an instance is unrecoverable.
	CustomType            OpsType = "Custom"          // use opsDefinition
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
	// The operator waits for each replica to be updated and ready before proceeding to the next one.
	// This ensures that only one replica is unavailable at a time during the update process.
	SerialStrategy UpdateStrategy = "Serial"

	// ParallelStrategy indicates that updates are applied simultaneously to all Pods of a Component.
	// The replicas are updated in parallel, with the operator updating all replicas concurrently.
	// This strategy provides the fastest update time but may lead to a period of reduced availability or
	// capacity during the update process.
	ParallelStrategy UpdateStrategy = "Parallel"

	// BestEffortParallelStrategy indicates that the replicas are updated in parallel, with the operator making
	// a best-effort attempt to update as many replicas as possible concurrently
	// while maintaining the component's availability.
	// Unlike the `Parallel` strategy, the `BestEffortParallel` strategy aims to ensure that a minimum number
	// of replicas remain available during the update process to maintain the component's quorum and functionality.
	//
	// For example, consider a component with 5 replicas. To maintain the component's availability and quorum,
	// the operator may allow a maximum of 2 replicas to be simultaneously updated. This ensures that at least
	// 3 replicas (a quorum) remain available and functional during the update process.
	//
	// The `BestEffortParallel` strategy strikes a balance between update speed and component availability.
	BestEffortParallelStrategy UpdateStrategy = "BestEffortParallel"
)

var DefaultLeader = ConsensusMember{
	Name:       "leader",
	AccessMode: ReadWrite,
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
}

type HostNetworkContainerPort struct {
	// Container specifies the target container within the Pod.
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

// ClusterService defines a service that is exposed externally, allowing entities outside the cluster to access it.
// For example, external applications, or other Clusters.
// And another Cluster managed by the same KubeBlocks operator can resolve the address exposed by a ClusterService
// using the `serviceRef` field.
//
// When a Component needs to access another Cluster's ClusterService using the `serviceRef` field,
// it must also define the service type and version information in the `componentDefinition.spec.serviceRefDeclarations`
// section.
type ClusterService struct {
	Service `json:",inline"`

	// Extends the ServiceSpec.Selector by allowing the specification of a sharding name, which is defined in
	// `cluster.spec.shardingSpecs[*].name`, to be used as a selector for the service.
	// Note that this and the `componentSelector` are mutually exclusive and cannot be set simultaneously.
	//
	// +optional
	ShardingSelector string `json:"shardingSelector,omitempty"`

	// Extends the ServiceSpec.Selector by allowing the specification of a component, to be used as a selector for the service.
	// Note that this and the `shardingSelector` are mutually exclusive and cannot be set simultaneously.
	//
	// +optional
	ComponentSelector string `json:"componentSelector,omitempty"`
}

// ComponentService defines a service that would be exposed as an inter-component service within a Cluster.
// A Service defined in the ComponentService is expected to be accessed by other Components within the same Cluster.
//
// When a Component needs to use a ComponentService provided by another Component within the same Cluster,
// it can declare a variable in the `componentDefinition.spec.vars` section and bind it to the specific exposed address
// of the ComponentService using the `serviceVarRef` field.
type ComponentService struct {
	Service `json:",inline"`

	// Indicates whether to create a corresponding Service for each Pod of the selected Component.
	// When set to true, a set of Services will be automatically generated for each Pod,
	// and the `roleSelector` field will be ignored.
	//
	// The names of the generated Services will follow the same suffix naming pattern: `$(serviceName)-$(podOrdinal)`.
	// The total number of generated Services will be equal to the number of replicas specified for the Component.
	//
	// Example usage:
	//
	// ```yaml
	// name: my-service
	// serviceName: my-service
	// podService: true
	// disableAutoProvision: true
	// spec:
	//   type: NodePort
	//   ports:
	//   - name: http
	//     port: 80
	//     targetPort: 8080
	// ```
	//
	// In this example, if the Component has 3 replicas, three Services will be generated:
	// - my-service-0: Points to the first Pod (podOrdinal: 0)
	// - my-service-1: Points to the second Pod (podOrdinal: 1)
	// - my-service-2: Points to the third Pod (podOrdinal: 2)
	//
	// Each generated Service will have the specified spec configuration and will target its respective Pod.
	//
	// This feature is useful when you need to expose each Pod of a Component individually, allowing external access
	// to specific instances of the Component.
	//
	// +kubebuilder:default=false
	// +optional
	PodService *bool `json:"podService,omitempty"`

	// Indicates whether the automatic provisioning of the service should be disabled.
	//
	// If set to true, the service will not be automatically created at the component provisioning.
	// Instead, you can enable the creation of this service by specifying it explicitly in the cluster API.
	//
	// +optional
	DisableAutoProvision *bool `json:"disableAutoProvision,omitempty"`
}

type Service struct {
	// Name defines the name of the service.
	// otherwise, it indicates the name of the service.
	// Others can refer to this service by its name. (e.g., connection credential)
	// Cannot be updated.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=25
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
	// +kubebuilder:validation:MaxLength=25
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	//
	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	// If ServiceType is LoadBalancer, cloud provider related parameters can be put here
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Spec defines the behavior of a service.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	//
	// +optional
	Spec corev1.ServiceSpec `json:"spec,omitempty"`

	// Extends the above `serviceSpec.selector` by allowing you to specify defined role as selector for the service.
	// When `roleSelector` is set, it adds a label selector "kubeblocks.io/role: {roleSelector}"
	// to the `serviceSpec.selector`.
	// Example usage:
	//
	//	  roleSelector: "leader"
	//
	// In this example, setting `roleSelector` to "leader" will add a label selector
	// "kubeblocks.io/role: leader" to the `serviceSpec.selector`.
	// This means that the service will select and route traffic to Pods with the label
	// "kubeblocks.io/role" set to "leader".
	//
	// Note that if `podService` sets to true, RoleSelector will be ignored.
	// The `podService` flag takes precedence over `roleSelector` and generates a service for each Pod.
	//
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
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Optional: no more than one of the following may be specified.

	// Variable references `$(VAR_NAME)` are expanded using the previously defined variables in the current context.
	//
	// If a variable cannot be resolved, the reference in the input string will be unchanged.
	// Double `$$` are reduced to a single `$`, which allows for escaping the `$(VAR_NAME)` syntax: i.e.
	//
	// - `$$(VAR_NAME)` will produce the string literal `$(VAR_NAME)`.
	//
	// Escaped references will never be expanded, regardless of whether the variable exists or not.
	// Defaults to "".
	//
	// +optional
	Value string `json:"value,omitempty"`

	// Source for the variable's value. Cannot be used if value is not empty.
	//
	// +optional
	ValueFrom *VarSource `json:"valueFrom,omitempty"`

	// A Go template expression that will be applied to the resolved value of the var.
	//
	// The expression will only be evaluated if the var is successfully resolved to a non-credential value.
	//
	// The resolved value can be accessed by its name within the expression, system vars and other user-defined
	// non-credential vars can be used within the expression in the same way.
	// Notice that, when accessing vars by its name, you should replace all the "-" in the name with "_", because of
	// that "-" is not a valid identifier in Go.
	//
	// All expressions are evaluated in the order the vars are defined. If a var depends on any vars that also
	// have expressions defined, be careful about the evaluation order as it may use intermediate values.
	//
	// The result of evaluation will be used as the final value of the var. If the expression fails to evaluate,
	// the resolving of var will also be considered failed.
	//
	// +optional
	Expression *string `json:"expression,omitempty"`
}

// VarSource represents a source for the value of an EnvVar.
type VarSource struct {
	// Selects a key of a ConfigMap.
	// +optional
	ConfigMapKeyRef *corev1.ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`

	// Selects a key of a Secret.
	// +optional
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`

	// Selects a defined var of host-network resources.
	// +optional
	HostNetworkVarRef *HostNetworkVarSelector `json:"hostNetworkVarRef,omitempty"`

	// Selects a defined var of a Service.
	// +optional
	ServiceVarRef *ServiceVarSelector `json:"serviceVarRef,omitempty"`

	// Selects a defined var of a Credential (SystemAccount).
	// +optional
	CredentialVarRef *CredentialVarSelector `json:"credentialVarRef,omitempty"`

	// Selects a defined var of a ServiceRef.
	// +optional
	ServiceRefVarRef *ServiceRefVarSelector `json:"serviceRefVarRef,omitempty"`

	// Selects a defined var of a Component.
	// +optional
	ComponentVarRef *ComponentVarSelector `json:"componentVarRef,omitempty"`
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

// ContainerVars defines the vars that can be referenced from a Container.
type ContainerVars struct {
	// The name of the container.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Container port to reference.
	//
	// +optional
	Port *NamedVar `json:"port,omitempty"`
}

// HostNetworkVars defines the vars that can be referenced from host-network resources.
type HostNetworkVars struct {
	// +optional
	Container *ContainerVars `json:"container,omitempty"`
}

// ServiceVars defines the vars that can be referenced from a Service.
type ServiceVars struct {
	// +optional
	Host *VarOption `json:"host,omitempty"`

	// LoadBalancer represents the LoadBalancer ingress point of the service.
	//
	// If multiple ingress points are available, the first one will be used automatically, choosing between IP and Hostname.
	//
	// +optional
	LoadBalancer *VarOption `json:"loadBalancer,omitempty"`

	// Port references a port or node-port defined in the service.
	//
	// If the referenced service is a pod-service, there will be multiple service objects matched,
	// and the value will be presented in the following format: service1.name:port1,service2.name:port2...
	//
	// +optional
	Port *NamedVar `json:"port,omitempty"`
}

// CredentialVars defines the vars that can be referenced from a Credential (SystemAccount).
// !!!!! CredentialVars will only be used as environment variables for Pods & Actions, and will not be used to render the templates.
type CredentialVars struct {
	// +optional
	Username *VarOption `json:"username,omitempty"`

	// +optional
	Password *VarOption `json:"password,omitempty"`
}

// ServiceRefVars defines the vars that can be referenced from a ServiceRef.
type ServiceRefVars struct {
	// +optional
	Endpoint *VarOption `json:"endpoint,omitempty"`

	// +optional
	Host *VarOption `json:"host,omitempty"`

	// +optional
	Port *VarOption `json:"port,omitempty"`

	CredentialVars `json:",inline"`
}

// HostNetworkVarSelector selects a var from host-network resources.
type HostNetworkVarSelector struct {
	// The component to select from.
	ClusterObjectReference `json:",inline"`

	HostNetworkVars `json:",inline"`
}

// ServiceVarSelector selects a var from a Service.
type ServiceVarSelector struct {
	// The Service to select from.
	// It can be referenced from the default headless service by setting the name to "headless".
	ClusterObjectReference `json:",inline"`

	ServiceVars `json:",inline"`
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

// ComponentVarSelector selects a var from a Component.
type ComponentVarSelector struct {
	// The Component to select from.
	ClusterObjectReference `json:",inline"`

	ComponentVars `json:",inline"`
}

type ComponentVars struct {
	// Reference to the name of the Component object.
	//
	// +optional
	ComponentName *VarOption `json:"componentName,omitempty"`

	// Reference to the replicas of the component.
	//
	// +optional
	Replicas *VarOption `json:"replicas,omitempty"`

	// Reference to the instanceName list of the component.
	// and the value will be presented in the following format: instanceName1,instanceName2,...
	//
	// +optional
	InstanceNames *VarOption `json:"instanceNames,omitempty"`

	// Reference to the pod FQDN list of the component.
	// The value will be presented in the following format: FQDN1,FQDN2,...
	//
	// +optional
	PodFQDNs *VarOption `json:"podFQDNs,omitempty"`
}

// ClusterObjectReference defines information to let you locate the referenced object inside the same Cluster.
type ClusterObjectReference struct {
	// CompDef specifies the definition used by the component that the referent object resident in.
	// If not specified, the component itself will be used.
	//
	// +optional
	CompDef string `json:"compDef,omitempty"`

	// Name of the referent object.
	//
	// +optional
	Name string `json:"name,omitempty"`

	// Specify whether the object must be defined.
	//
	// +optional
	Optional *bool `json:"optional,omitempty"`

	// This option defines the behavior when multiple component objects match the specified @CompDef.
	// If not provided, an error will be raised when handling multiple matches.
	//
	// +optional
	MultipleClusterObjectOption *MultipleClusterObjectOption `json:"multipleClusterObjectOption,omitempty"`
}

// MultipleClusterObjectOption defines the options for handling multiple cluster objects matched.
type MultipleClusterObjectOption struct {
	// Define the strategy for handling multiple cluster objects.
	//
	// +kubebuilder:validation:Required
	Strategy MultipleClusterObjectStrategy `json:"strategy"`

	// Define the options for handling combined variables.
	// Valid only when the strategy is set to "combined".
	//
	// +optional
	CombinedOption *MultipleClusterObjectCombinedOption `json:"combinedOption,omitempty"`
}

// MultipleClusterObjectStrategy defines the strategy for handling multiple cluster objects.
// +enum
// +kubebuilder:validation:Enum={individual,combined}
type MultipleClusterObjectStrategy string

const (
	// MultipleClusterObjectStrategyIndividual - each matched component will have its individual variable with its name
	// as the suffix.
	// This is required when referencing credential variables that cannot be passed by values.
	MultipleClusterObjectStrategyIndividual MultipleClusterObjectStrategy = "individual"

	// MultipleClusterObjectStrategyCombined - the values from all matched components will be combined into a single
	// variable using the specified option.
	MultipleClusterObjectStrategyCombined MultipleClusterObjectStrategy = "combined"
)

// MultipleClusterObjectCombinedOption defines options for handling combined variables.
type MultipleClusterObjectCombinedOption struct {
	// If set, the existing variable will be kept, and a new variable will be defined with the specified suffix
	// in pattern: $(var.name)_$(suffix).
	// The new variable will be auto-created and placed behind the existing one.
	// If not set, the existing variable will be reused with the value format defined below.
	//
	// +optional
	NewVarSuffix *string `json:"newVarSuffix,omitempty"`

	// The format of the value that the operator will use to compose values from multiple components.
	//
	// +kubebuilder:default="Flatten"
	// +optional
	ValueFormat MultipleClusterObjectValueFormat `json:"valueFormat,omitempty"`

	// The flatten format, default is: $(comp-name-1):value,$(comp-name-2):value.
	//
	// +optional
	FlattenFormat *MultipleClusterObjectValueFormatFlatten `json:"flattenFormat,omitempty"`
}

// MultipleClusterObjectValueFormat defines the format details for the value.
type MultipleClusterObjectValueFormat string

const (
	FlattenFormat MultipleClusterObjectValueFormat = "Flatten"
)

// MultipleClusterObjectValueFormatFlatten defines the flatten format for the value.
type MultipleClusterObjectValueFormatFlatten struct {
	// Pair delimiter.
	//
	// +kubebuilder:default=","
	// +kubebuilder:validation:Required
	Delimiter string `json:"delimiter"`

	// Key-value delimiter.
	//
	// +kubebuilder:default=":"
	// +kubebuilder:validation:Required
	KeyValueDelimiter string `json:"keyValueDelimiter"`
}

// PrometheusScheme defines the protocol of prometheus scrape metrics.
//
// +enum
// +kubebuilder:validation:Enum={http,https}
type PrometheusScheme string

const (
	HTTPProtocol  PrometheusScheme = "http"
	HTTPSProtocol PrometheusScheme = "https"
)
