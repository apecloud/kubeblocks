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
	"fmt"
	"reflect"
)

func ValidateLifecycleActionExecutionIdentity(execution *LifecycleActionExecution) error {
	if execution == nil {
		return fmt.Errorf("execution is nil")
	}
	if len(execution.OwnerReferences) != 0 {
		return fmt.Errorf("ownerReferences must be empty")
	}
	if _, ok := execution.Annotations[MultiClusterPlacementAnnotationKey]; ok {
		return fmt.Errorf("placement annotation is forbidden on the control-plane execution")
	}
	key, _, err := ComputeLifecycleActionInvocationKey(execution.Spec)
	if err != nil {
		return err
	}
	if execution.Spec.InvocationKey != key {
		return fmt.Errorf("invocation key mismatch")
	}
	name, _, err := ComputeLifecycleActionExecutionName(execution.Spec)
	if err != nil {
		return err
	}
	if execution.Name != name {
		return fmt.Errorf("execution name mismatch")
	}
	pod := execution.Spec.Target.Pod
	namespace := execution.Namespace
	if namespace == "" || execution.Spec.SourceRef.Namespace != namespace || execution.Spec.WorkloadRef.Namespace != namespace || pod.Namespace != namespace {
		return fmt.Errorf("execution, source, workload, and target namespaces must match")
	}
	return nil
}

func ValidateLifecycleActionExecutionStatusTransition(oldStatus, newStatus LifecycleActionExecutionStatus) error {
	if err := validateStatusShape(newStatus); err != nil {
		return err
	}
	if reflect.DeepEqual(oldStatus, newStatus) {
		return nil
	}
	if !validPhaseEdge(oldStatus.Phase, newStatus.Phase) {
		return fmt.Errorf("illegal phase transition %s -> %s", oldStatus.Phase, newStatus.Phase)
	}
	if oldStatus.Phase == LifecycleActionExecutionPhaseRunning && (newStatus.Phase == LifecycleActionExecutionPhaseSucceeded || newStatus.Phase == LifecycleActionExecutionPhaseFailed) && !reflect.DeepEqual(oldStatus.StartTime, newStatus.StartTime) {
		return fmt.Errorf("terminal status must preserve running startTime")
	}
	if newStatus.Phase == LifecycleActionExecutionPhaseFailed && oldStatus.Phase != LifecycleActionExecutionPhaseRunning && newStatus.StartTime != nil {
		return fmt.Errorf("pre-dispatch failure cannot set startTime")
	}
	return validateReasonEdge(oldStatus.Phase, newStatus)
}

func validPhaseEdge(oldPhase, newPhase LifecycleActionExecutionPhase) bool {
	switch oldPhase {
	case LifecycleActionExecutionPhaseUnobserved:
		return newPhase == LifecycleActionExecutionPhasePending || newPhase == LifecycleActionExecutionPhaseFailed || newPhase == LifecycleActionExecutionPhaseCancelled
	case LifecycleActionExecutionPhasePending:
		return newPhase == LifecycleActionExecutionPhaseRunning || newPhase == LifecycleActionExecutionPhaseFailed || newPhase == LifecycleActionExecutionPhaseCancelled
	case LifecycleActionExecutionPhaseRunning:
		return newPhase == LifecycleActionExecutionPhaseSucceeded || newPhase == LifecycleActionExecutionPhaseFailed
	default:
		return false
	}
}

func validateStatusShape(status LifecycleActionExecutionStatus) error {
	hasFailure := status.FailureClass != ""
	hasReason := status.Reason != ""
	hasDetail := status.Detail != nil
	switch status.Phase {
	case LifecycleActionExecutionPhaseUnobserved, LifecycleActionExecutionPhasePending:
		if hasFailure || hasReason || hasDetail || status.StartTime != nil || status.FinishedTime != nil {
			return fmt.Errorf("%s status contains execution or terminal data", status.Phase)
		}
	case LifecycleActionExecutionPhaseRunning:
		if status.StartTime == nil || hasFailure || hasReason || hasDetail || status.FinishedTime != nil {
			return fmt.Errorf("running status shape is invalid")
		}
	case LifecycleActionExecutionPhaseSucceeded:
		if status.StartTime == nil || status.FinishedTime == nil || hasFailure || hasReason || hasDetail {
			return fmt.Errorf("succeeded status shape is invalid")
		}
	case LifecycleActionExecutionPhaseFailed:
		if !hasFailure || !hasReason || status.FinishedTime == nil {
			return fmt.Errorf("failed status shape is invalid")
		}
	case LifecycleActionExecutionPhaseCancelled:
		if status.Reason != LifecycleActionReasonCancelledBySource || status.FinishedTime == nil || status.StartTime != nil || hasFailure || hasDetail {
			return fmt.Errorf("cancelled status shape is invalid")
		}
	default:
		return fmt.Errorf("unknown phase %q", status.Phase)
	}
	if status.StartTime != nil && status.FinishedTime != nil && status.FinishedTime.Before(status.StartTime) {
		return fmt.Errorf("finishedTime precedes startTime")
	}
	return nil
}

func validateReasonEdge(oldPhase LifecycleActionExecutionPhase, status LifecycleActionExecutionStatus) error {
	if status.Phase != LifecycleActionExecutionPhaseFailed {
		return nil
	}
	validClassReason := false
	switch status.FailureClass {
	case LifecycleActionFailureClassPermanent:
		validClassReason = status.Reason == LifecycleActionReasonActionRejected || status.Reason == LifecycleActionReasonActionFailed || status.Reason == LifecycleActionReasonInvalidExecutionIdentity || status.Reason == LifecycleActionReasonTargetIdentityChanged
	case LifecycleActionFailureClassRetryable:
		validClassReason = status.Reason == LifecycleActionReasonActionFailed || status.Reason == LifecycleActionReasonTransportError || status.Reason == LifecycleActionReasonTargetUnavailable
	case LifecycleActionFailureClassUnknown:
		validClassReason = status.Reason == LifecycleActionReasonActionFailed || status.Reason == LifecycleActionReasonTransportError || status.Reason == LifecycleActionReasonConfirmationLost
	}
	if !validClassReason {
		return fmt.Errorf("invalid failureClass/reason combination")
	}
	if status.Detail != nil && (status.FailureClass != LifecycleActionFailureClassPermanent || status.Reason != LifecycleActionReasonActionRejected || status.Detail.Type != LifecycleActionFailureDetailTypeReconfigure || status.Detail.Reconfigure == nil) {
		return fmt.Errorf("failure detail is invalid")
	}
	if status.Detail != nil && status.Detail.Reconfigure.Reason != LifecycleActionReconfigureFailureInvalidParameter && status.Detail.Reconfigure.Reason != LifecycleActionReconfigureFailureUnsupportedParameter {
		return fmt.Errorf("reconfigure failure detail is invalid")
	}
	switch status.Reason {
	case LifecycleActionReasonInvalidExecutionIdentity, LifecycleActionReasonTargetIdentityChanged:
		if oldPhase != LifecycleActionExecutionPhaseUnobserved && oldPhase != LifecycleActionExecutionPhasePending {
			return fmt.Errorf("identity failure is not pre-dispatch")
		}
	case LifecycleActionReasonTargetUnavailable:
		if oldPhase != LifecycleActionExecutionPhasePending {
			return fmt.Errorf("target unavailable must originate from pending")
		}
	case LifecycleActionReasonActionRejected:
		if oldPhase != LifecycleActionExecutionPhasePending && oldPhase != LifecycleActionExecutionPhaseRunning {
			return fmt.Errorf("action rejection has an invalid source phase")
		}
	case LifecycleActionReasonActionFailed, LifecycleActionReasonTransportError, LifecycleActionReasonConfirmationLost:
		if oldPhase != LifecycleActionExecutionPhaseRunning {
			return fmt.Errorf("post-dispatch failure must originate from running")
		}
	}
	return nil
}
