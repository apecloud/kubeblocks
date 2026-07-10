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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateLifecycleActionExecutionStatusTransition(t *testing.T) {
	start := metav1.NewTime(time.Unix(100, 0).UTC())
	finish := metav1.NewTime(time.Unix(200, 0).UTC())

	tests := []struct {
		name string
		old  LifecycleActionExecutionStatus
		new  LifecycleActionExecutionStatus
		ok   bool
	}{
		{name: "default equal", old: status(LifecycleActionExecutionPhaseUnobserved), new: status(LifecycleActionExecutionPhaseUnobserved), ok: true},
		{name: "equal malformed pending", old: LifecycleActionExecutionStatus{Phase: LifecycleActionExecutionPhasePending, Reason: LifecycleActionReasonActionRejected}, new: LifecycleActionExecutionStatus{Phase: LifecycleActionExecutionPhasePending, Reason: LifecycleActionReasonActionRejected}},
		{name: "pending", old: status(LifecycleActionExecutionPhaseUnobserved), new: status(LifecycleActionExecutionPhasePending), ok: true},
		{name: "running", old: status(LifecycleActionExecutionPhasePending), new: runningStatus(start), ok: true},
		{name: "succeeded", old: runningStatus(start), new: succeededStatus(start, finish), ok: true},
		{name: "cancel before dispatch", old: status(LifecycleActionExecutionPhasePending), new: cancelledStatus(finish), ok: true},
		{name: "invalid identity before pending", old: status(LifecycleActionExecutionPhaseUnobserved), new: failedStatus(nil, finish, LifecycleActionFailureClassPermanent, LifecycleActionReasonInvalidExecutionIdentity, nil), ok: true},
		{name: "target identity changed pending", old: status(LifecycleActionExecutionPhasePending), new: failedStatus(nil, finish, LifecycleActionFailureClassPermanent, LifecycleActionReasonTargetIdentityChanged, nil), ok: true},
		{name: "target unavailable pending", old: status(LifecycleActionExecutionPhasePending), new: failedStatus(nil, finish, LifecycleActionFailureClassRetryable, LifecycleActionReasonTargetUnavailable, nil), ok: true},
		{name: "rejected pending", old: status(LifecycleActionExecutionPhasePending), new: failedStatus(nil, finish, LifecycleActionFailureClassPermanent, LifecycleActionReasonActionRejected, reconfigureDetail(LifecycleActionReconfigureFailureInvalidParameter)), ok: true},
		{name: "rejected running", old: runningStatus(start), new: failedStatus(&start, finish, LifecycleActionFailureClassPermanent, LifecycleActionReasonActionRejected, nil), ok: true},
		{name: "action permanent", old: runningStatus(start), new: failedStatus(&start, finish, LifecycleActionFailureClassPermanent, LifecycleActionReasonActionFailed, nil), ok: true},
		{name: "action retryable", old: runningStatus(start), new: failedStatus(&start, finish, LifecycleActionFailureClassRetryable, LifecycleActionReasonActionFailed, nil), ok: true},
		{name: "transport retryable", old: runningStatus(start), new: failedStatus(&start, finish, LifecycleActionFailureClassRetryable, LifecycleActionReasonTransportError, nil), ok: true},
		{name: "action unknown", old: runningStatus(start), new: failedStatus(&start, finish, LifecycleActionFailureClassUnknown, LifecycleActionReasonActionFailed, nil), ok: true},
		{name: "transport unknown", old: runningStatus(start), new: failedStatus(&start, finish, LifecycleActionFailureClassUnknown, LifecycleActionReasonTransportError, nil), ok: true},
		{name: "confirmation lost", old: runningStatus(start), new: failedStatus(&start, finish, LifecycleActionFailureClassUnknown, LifecycleActionReasonConfirmationLost, nil), ok: true},
		{name: "running cancelled", old: runningStatus(start), new: LifecycleActionExecutionStatus{Phase: LifecycleActionExecutionPhaseCancelled, Reason: LifecycleActionReasonCancelledBySource, StartTime: &start, FinishedTime: &finish}},
		{name: "cancel before dispatch with start", old: status(LifecycleActionExecutionPhasePending), new: LifecycleActionExecutionStatus{Phase: LifecycleActionExecutionPhaseCancelled, Reason: LifecycleActionReasonCancelledBySource, StartTime: &start, FinishedTime: &finish}},
		{name: "pending failed with invented start", old: status(LifecycleActionExecutionPhasePending), new: failedStatus(&start, finish, LifecycleActionFailureClassPermanent, LifecycleActionReasonActionRejected, nil)},
		{name: "unobserved failed with invented start", old: status(LifecycleActionExecutionPhaseUnobserved), new: failedStatus(&start, finish, LifecycleActionFailureClassPermanent, LifecycleActionReasonInvalidExecutionIdentity, nil)},
		{name: "running failed missing start", old: runningStatus(start), new: failedStatus(nil, finish, LifecycleActionFailureClassUnknown, LifecycleActionReasonConfirmationLost, nil)},
		{name: "running failed changed start", old: runningStatus(start), new: failedStatus(ptrTime(metav1.NewTime(time.Unix(101, 0).UTC())), finish, LifecycleActionFailureClassUnknown, LifecycleActionReasonConfirmationLost, nil)},
		{name: "running succeeded changed start", old: runningStatus(start), new: succeededStatus(metav1.NewTime(time.Unix(101, 0).UTC()), finish)},
		{name: "skip pending", old: status(LifecycleActionExecutionPhaseUnobserved), new: runningStatus(start)},
		{name: "same phase mutation", old: runningStatus(start), new: runningStatus(metav1.NewTime(time.Unix(101, 0).UTC()))},
		{name: "terminal mutation", old: succeededStatus(start, finish), new: failedStatus(&start, finish, LifecycleActionFailureClassUnknown, LifecycleActionReasonConfirmationLost, nil)},
		{name: "identity failure after running", old: runningStatus(start), new: failedStatus(&start, finish, LifecycleActionFailureClassPermanent, LifecycleActionReasonInvalidExecutionIdentity, nil)},
		{name: "target unavailable after running", old: runningStatus(start), new: failedStatus(&start, finish, LifecycleActionFailureClassRetryable, LifecycleActionReasonTargetUnavailable, nil)},
		{name: "confirmation lost pending", old: status(LifecycleActionExecutionPhasePending), new: failedStatus(nil, finish, LifecycleActionFailureClassUnknown, LifecycleActionReasonConfirmationLost, nil)},
		{name: "permanent transport", old: runningStatus(start), new: failedStatus(&start, finish, LifecycleActionFailureClassPermanent, LifecycleActionReasonTransportError, nil)},
		{name: "retryable rejected", old: runningStatus(start), new: failedStatus(&start, finish, LifecycleActionFailureClassRetryable, LifecycleActionReasonActionRejected, nil)},
		{name: "detail on action failed", old: runningStatus(start), new: failedStatus(&start, finish, LifecycleActionFailureClassPermanent, LifecycleActionReasonActionFailed, reconfigureDetail(LifecycleActionReconfigureFailureInvalidParameter))},
		{name: "unknown reconfigure detail", old: status(LifecycleActionExecutionPhasePending), new: failedStatus(nil, finish, LifecycleActionFailureClassPermanent, LifecycleActionReasonActionRejected, reconfigureDetail("Other"))},
		{name: "finished before start", old: runningStatus(finish), new: failedStatus(&finish, start, LifecycleActionFailureClassUnknown, LifecycleActionReasonConfirmationLost, nil)},
		{name: "pending with reason", old: status(LifecycleActionExecutionPhaseUnobserved), new: LifecycleActionExecutionStatus{Phase: LifecycleActionExecutionPhasePending, Reason: LifecycleActionReasonActionRejected}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLifecycleActionExecutionStatusTransition(tt.old, tt.new)
			if tt.ok && err != nil {
				t.Fatalf("valid transition rejected: %v", err)
			}
			if !tt.ok && err == nil {
				t.Fatalf("invalid transition accepted")
			}
		})
	}
}

func status(phase LifecycleActionExecutionPhase) LifecycleActionExecutionStatus {
	return LifecycleActionExecutionStatus{Phase: phase}
}

func runningStatus(start metav1.Time) LifecycleActionExecutionStatus {
	return LifecycleActionExecutionStatus{Phase: LifecycleActionExecutionPhaseRunning, StartTime: &start}
}

func succeededStatus(start, finish metav1.Time) LifecycleActionExecutionStatus {
	return LifecycleActionExecutionStatus{Phase: LifecycleActionExecutionPhaseSucceeded, StartTime: &start, FinishedTime: &finish}
}

func cancelledStatus(finish metav1.Time) LifecycleActionExecutionStatus {
	return LifecycleActionExecutionStatus{Phase: LifecycleActionExecutionPhaseCancelled, Reason: LifecycleActionReasonCancelledBySource, FinishedTime: &finish}
}

func failedStatus(start *metav1.Time, finish metav1.Time, class LifecycleActionFailureClass, reason LifecycleActionReason, detail *LifecycleActionFailureDetail) LifecycleActionExecutionStatus {
	return LifecycleActionExecutionStatus{
		Phase:        LifecycleActionExecutionPhaseFailed,
		FailureClass: class,
		Reason:       reason,
		Detail:       detail,
		StartTime:    start,
		FinishedTime: &finish,
	}
}

func reconfigureDetail(reason LifecycleActionReconfigureFailureReason) *LifecycleActionFailureDetail {
	return &LifecycleActionFailureDetail{
		Type: LifecycleActionFailureDetailTypeReconfigure,
		Reconfigure: &ReconfigureLifecycleActionFailureDetail{
			Reason: reason,
		},
	}
}
