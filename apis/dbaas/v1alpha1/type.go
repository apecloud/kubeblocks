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
	UpdatingPhase          Phase = "Updating"
	VolumeExpandingPhase   Phase = "VolumeExpanding"
	SucceedPhase           Phase = "Succeed"
	AbnormalPhase          Phase = "Abnormal"
	ConditionsErrorPhase   Phase = "ConditionsError"
	HorizontalScalingPhase Phase = "HorizontalScaling"
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

var webhookMgr *webhookManager

type webhookManager struct {
	client client.Client
}

func RegisterWebhookManager(mgr manager.Manager) {
	webhookMgr = &webhookManager{mgr.GetClient()}
}
