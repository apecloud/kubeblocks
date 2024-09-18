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
	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// PodSelectionPolicy pod selection strategy.
// +enum
// +kubebuilder:validation:Enum={All,Any}
type PodSelectionPolicy string

const (
	All PodSelectionPolicy = "All"
	Any PodSelectionPolicy = "Any"
)

// OpsWorkloadType policy after action failure.
// +enum
// +kubebuilder:validation:Enum={Job,Pod}
type OpsWorkloadType string

const (
	PodWorkload OpsWorkloadType = "Pod"
	JobWorkload OpsWorkloadType = "Job"
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

// Phase represents the current status of the ClusterDefinition CR.
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

// OpsType defines operation types.
// +enum
// +kubebuilder:validation:Enum={Upgrade,VerticalScaling,VolumeExpansion,HorizontalScaling,Restart,Reconfiguring,Start,Stop,Expose,Switchover,Backup,Restore,RebuildInstance,Custom}
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
	BackupType            OpsType = "Backup"
	RestoreType           OpsType = "Restore"
	RebuildInstanceType   OpsType = "RebuildInstance" // RebuildInstance rebuilding an instance is very useful when a node is offline or an instance is unrecoverable.
	CustomType            OpsType = "Custom"          // use opsDefinition
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
	FromClusterPhases []kbappsv1.ClusterPhase
	ToClusterPhase    kbappsv1.ClusterPhase
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
