/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	LifecycleActionIdentityVersionV1      = "v1"
	LifecycleActionExecutionNameVersionV1 = "v1"
	LifecycleActionExecutionNamePrefix    = "lae-"
	MultiClusterPlacementAnnotationKey    = "apps.kubeblocks.io/multi-cluster-placement"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},shortName=lae
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="REASON",type="string",JSONPath=".status.reason"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// LifecycleActionExecution records one immutable lifecycle-action attempt.
type LifecycleActionExecution struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec LifecycleActionExecutionSpec `json:"spec"`
	// +kubebuilder:default={phase:Unobserved}
	Status LifecycleActionExecutionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type LifecycleActionExecutionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LifecycleActionExecution `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LifecycleActionExecution{}, &LifecycleActionExecutionList{})
}

// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable"
// +kubebuilder:validation:XValidation:rule="self.sourceRef.__namespace__ == self.workloadRef.__namespace__ && self.sourceRef.__namespace__ == self.target.pod.__namespace__",message="source, workload, and target namespaces must match"
type LifecycleActionExecutionSpec struct {
	// +kubebuilder:validation:Enum=v1
	IdentityVersion string `json:"identityVersion"`

	// +kubebuilder:validation:Pattern=`^[a-z2-7]{52}$`
	InvocationKey string `json:"invocationKey"`

	SourceRef   ObjectIdentityRef `json:"sourceRef"`
	WorkloadRef ObjectIdentityRef `json:"workloadRef"`

	// +kubebuilder:validation:MinLength=1
	ActionName string `json:"actionName"`

	Target LifecycleActionTarget `json:"target"`

	// +kubebuilder:validation:Minimum=1
	Attempt int32 `json:"attempt"`

	Context LifecycleActionContext `json:"context"`
}

// +kubebuilder:validation:XValidation:rule="self.uid.size() > 0",message="uid must not be empty"
type ObjectIdentityRef struct {
	// +kubebuilder:validation:MinLength=1
	APIGroup string `json:"apiGroup"`
	// +kubebuilder:validation:MinLength=1
	Kind string `json:"kind"`
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`
	// +kubebuilder:validation:MinLength=1
	Name string    `json:"name"`
	UID  types.UID `json:"uid"`
}

type LifecycleActionTargetType string

const LifecycleActionTargetTypePod LifecycleActionTargetType = "Pod"

// +kubebuilder:validation:XValidation:rule="self.type == 'Pod' && has(self.pod)",message="pod is required for Pod targets"
type LifecycleActionTarget struct {
	// +kubebuilder:validation:Enum=Pod
	Type LifecycleActionTargetType `json:"type"`
	Pod  *PodLifecycleActionTarget `json:"pod,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="self.podUID.size() > 0",message="podUID must not be empty"
type PodLifecycleActionTarget struct {
	ClusterContext LifecycleActionClusterContext `json:"clusterContext"`
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`
	// +kubebuilder:validation:MinLength=1
	ComponentName string `json:"componentName"`
	// +kubebuilder:validation:MinLength=1
	InstanceName string `json:"instanceName"`
	// +kubebuilder:validation:MinLength=1
	PodName string    `json:"podName"`
	PodUID  types.UID `json:"podUID"`
}

type LifecycleActionClusterContextType string

const (
	LifecycleActionClusterContextLocal     LifecycleActionClusterContextType = "Local"
	LifecycleActionClusterContextPlacement LifecycleActionClusterContextType = "Placement"
)

// +kubebuilder:validation:XValidation:rule="self.type == 'Local' ? !has(self.placement) : has(self.placement) && self.placement.size() > 0",message="placement must be absent for Local and non-empty for Placement"
type LifecycleActionClusterContext struct {
	// +kubebuilder:validation:Enum=Local;Placement
	Type      LifecycleActionClusterContextType `json:"type"`
	Placement *string                           `json:"placement,omitempty"`
}

type LifecycleActionContextType string

const LifecycleActionContextTypeReconfigure LifecycleActionContextType = "Reconfigure"

// +kubebuilder:validation:XValidation:rule="self.type == 'Reconfigure' && has(self.reconfigure)",message="reconfigure is required for Reconfigure context"
type LifecycleActionContext struct {
	// +kubebuilder:validation:Enum=Reconfigure
	Type        LifecycleActionContextType         `json:"type"`
	Reconfigure *ReconfigureLifecycleActionContext `json:"reconfigure,omitempty"`
}

type ReconfigureLifecycleActionContext struct {
	// +kubebuilder:validation:MinLength=1
	ConfigName string `json:"configName"`
	// +kubebuilder:validation:MinLength=1
	TargetConfigHash string `json:"targetConfigHash"`
	// +kubebuilder:validation:Minimum=1
	ComponentParameterGeneration int64     `json:"componentParameterGeneration"`
	OperationUID                 types.UID `json:"operationUID,omitempty"`
}

type LifecycleActionExecutionPhase string

const (
	LifecycleActionExecutionPhaseUnobserved LifecycleActionExecutionPhase = "Unobserved"
	LifecycleActionExecutionPhasePending    LifecycleActionExecutionPhase = "Pending"
	LifecycleActionExecutionPhaseRunning    LifecycleActionExecutionPhase = "Running"
	LifecycleActionExecutionPhaseSucceeded  LifecycleActionExecutionPhase = "Succeeded"
	LifecycleActionExecutionPhaseFailed     LifecycleActionExecutionPhase = "Failed"
	LifecycleActionExecutionPhaseCancelled  LifecycleActionExecutionPhase = "Cancelled"
)

type LifecycleActionFailureClass string

const (
	LifecycleActionFailureClassPermanent LifecycleActionFailureClass = "Permanent"
	LifecycleActionFailureClassRetryable LifecycleActionFailureClass = "Retryable"
	LifecycleActionFailureClassUnknown   LifecycleActionFailureClass = "Unknown"
)

type LifecycleActionReason string

const (
	LifecycleActionReasonActionRejected           LifecycleActionReason = "ActionRejected"
	LifecycleActionReasonActionFailed             LifecycleActionReason = "ActionFailed"
	LifecycleActionReasonTransportError           LifecycleActionReason = "TransportError"
	LifecycleActionReasonConfirmationLost         LifecycleActionReason = "ConfirmationLost"
	LifecycleActionReasonTargetUnavailable        LifecycleActionReason = "TargetUnavailable"
	LifecycleActionReasonTargetIdentityChanged    LifecycleActionReason = "TargetIdentityChanged"
	LifecycleActionReasonInvalidExecutionIdentity LifecycleActionReason = "InvalidExecutionIdentity"
	LifecycleActionReasonCancelledBySource        LifecycleActionReason = "CancelledBySource"
)

// The first rule freezes equal-phase and terminal statuses. Later rules constrain every legal edge and shape.
// +kubebuilder:validation:XValidation:rule="self == oldSelf || (oldSelf.phase == 'Unobserved' && self.phase in ['Pending', 'Failed', 'Cancelled']) || (oldSelf.phase == 'Pending' && self.phase in ['Running', 'Failed', 'Cancelled']) || (oldSelf.phase == 'Running' && self.phase in ['Succeeded', 'Failed'])",message="illegal lifecycle action execution phase transition"
// +kubebuilder:validation:XValidation:rule="self.phase == 'Unobserved' || self.phase == 'Pending' ? !has(self.failureClass) && !has(self.reason) && !has(self.detail) && !has(self.startTime) && !has(self.finishedTime) : true",message="unobserved and pending statuses cannot contain terminal data"
// +kubebuilder:validation:XValidation:rule="self.phase == 'Running' ? has(self.startTime) && !has(self.failureClass) && !has(self.reason) && !has(self.detail) && !has(self.finishedTime) : true",message="running status requires only startTime"
// +kubebuilder:validation:XValidation:rule="self.phase == 'Succeeded' ? has(self.startTime) && has(self.finishedTime) && !has(self.failureClass) && !has(self.reason) && !has(self.detail) : true",message="succeeded status requires startTime and finishedTime"
// +kubebuilder:validation:XValidation:rule="self.phase == 'Failed' ? has(self.failureClass) && has(self.reason) && has(self.finishedTime) : true",message="failed status requires failureClass, reason, and finishedTime"
// +kubebuilder:validation:XValidation:rule="self.phase == 'Cancelled' ? self.reason == 'CancelledBySource' && has(self.finishedTime) && !has(self.startTime) && !has(self.failureClass) && !has(self.detail) : true",message="cancelled status has a fixed reason and no execution or failure data"
// +kubebuilder:validation:XValidation:rule="!has(self.startTime) || !has(self.finishedTime) || self.finishedTime >= self.startTime",message="finishedTime cannot precede startTime"
// +kubebuilder:validation:XValidation:rule="self.phase != 'Failed' || (self.failureClass == 'Permanent' && self.reason in ['ActionRejected', 'ActionFailed', 'TargetIdentityChanged', 'InvalidExecutionIdentity']) || (self.failureClass == 'Retryable' && self.reason in ['ActionFailed', 'TransportError', 'TargetUnavailable']) || (self.failureClass == 'Unknown' && self.reason in ['ActionFailed', 'TransportError', 'ConfirmationLost'])",message="failureClass and reason combination is invalid"
// +kubebuilder:validation:XValidation:rule="!has(self.detail) || (self.phase == 'Failed' && self.failureClass == 'Permanent' && self.reason == 'ActionRejected')",message="typed detail is allowed only for permanent action rejection"
// +kubebuilder:validation:XValidation:rule="self.phase == 'Failed' && self.reason in ['InvalidExecutionIdentity', 'TargetIdentityChanged'] ? oldSelf.phase in ['Unobserved', 'Pending'] : true",message="identity failures are pre-dispatch only"
// +kubebuilder:validation:XValidation:rule="self.phase != 'Failed' || self.reason != 'TargetUnavailable' || oldSelf.phase == 'Pending'",message="target unavailable is a pending failure"
// +kubebuilder:validation:XValidation:rule="self.phase != 'Failed' || self.reason in ['InvalidExecutionIdentity', 'TargetIdentityChanged', 'TargetUnavailable'] || (self.reason == 'ActionRejected' && oldSelf.phase in ['Pending', 'Running']) || (self.reason in ['ActionFailed', 'TransportError', 'ConfirmationLost'] && oldSelf.phase == 'Running')",message="failure reason is invalid for the source phase"
// +kubebuilder:validation:XValidation:rule="oldSelf.phase != 'Running' || self == oldSelf || (self.phase in ['Succeeded', 'Failed'] && has(self.startTime) && self.startTime == oldSelf.startTime)",message="terminal status must preserve running startTime"
// +kubebuilder:validation:XValidation:rule="self.phase != 'Failed' || oldSelf.phase == 'Running' || !has(self.startTime)",message="pre-dispatch failure cannot set startTime"
type LifecycleActionExecutionStatus struct {
	// +kubebuilder:default=Unobserved
	// +kubebuilder:validation:Enum=Unobserved;Pending;Running;Succeeded;Failed;Cancelled
	Phase LifecycleActionExecutionPhase `json:"phase,omitempty"`

	// +kubebuilder:validation:Enum=Permanent;Retryable;Unknown
	FailureClass LifecycleActionFailureClass `json:"failureClass,omitempty"`

	// +kubebuilder:validation:Enum=ActionRejected;ActionFailed;TransportError;ConfirmationLost;TargetUnavailable;TargetIdentityChanged;InvalidExecutionIdentity;CancelledBySource
	Reason LifecycleActionReason `json:"reason,omitempty"`

	Detail *LifecycleActionFailureDetail `json:"detail,omitempty"`

	StartTime    *metav1.Time `json:"startTime,omitempty"`
	FinishedTime *metav1.Time `json:"finishedTime,omitempty"`
}

type LifecycleActionFailureDetailType string

const LifecycleActionFailureDetailTypeReconfigure LifecycleActionFailureDetailType = "Reconfigure"

// +kubebuilder:validation:XValidation:rule="self.type == 'Reconfigure' && has(self.reconfigure)",message="reconfigure is required for Reconfigure detail"
type LifecycleActionFailureDetail struct {
	// +kubebuilder:validation:Enum=Reconfigure
	Type        LifecycleActionFailureDetailType         `json:"type"`
	Reconfigure *ReconfigureLifecycleActionFailureDetail `json:"reconfigure,omitempty"`
}

type LifecycleActionReconfigureFailureReason string

const (
	LifecycleActionReconfigureFailureInvalidParameter     LifecycleActionReconfigureFailureReason = "InvalidParameter"
	LifecycleActionReconfigureFailureUnsupportedParameter LifecycleActionReconfigureFailureReason = "UnsupportedParameter"
)

type ReconfigureLifecycleActionFailureDetail struct {
	// +kubebuilder:validation:Enum=InvalidParameter;UnsupportedParameter
	Reason LifecycleActionReconfigureFailureReason `json:"reason"`
}
