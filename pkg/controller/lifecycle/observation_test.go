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

package lifecycle

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func TestReconcileActionObservationPersistsBeforeExecutionAndNormalizesFailure(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-0", UID: types.UID("pod-uid")}}
	key := NewActionObservationKey(appsv1.LifecycleActionReconfigure, "mysql", "hash", pod)
	statuses := []appsv1.LifecycleActionStatus(nil)
	executions := 0
	execute := func() error {
		executions++
		return &actionResultError{code: "InvalidParameter", retryable: ptr.To(false), err: ErrActionFailed}
	}

	if state, err := ReconcileActionObservation(&statuses, key, execute); err != nil || state != ActionObservationUpdated {
		t.Fatalf("expected initial observation update, got %q", state)
	}
	if executions != 0 || len(statuses) != 1 || statuses[0].Phase != appsv1.LifecycleActionPending {
		t.Fatalf("expected a persisted Pending phase before execution, executions=%d status=%+v", executions, statuses)
	}

	if state, err := ReconcileActionObservation(&statuses, key, execute); err != nil || state != ActionObservationUpdated {
		t.Fatalf("expected Running observation update, got %q", state)
	}
	if executions != 0 || statuses[0].Phase != appsv1.LifecycleActionRunning || statuses[0].StartTime == nil {
		t.Fatalf("expected a persisted Running phase before execution, executions=%d status=%+v", executions, statuses[0])
	}

	if state, err := ReconcileActionObservation(&statuses, key, execute); err != nil || state != ActionObservationUpdated {
		t.Fatalf("expected terminal observation update, got %q", state)
	}
	if executions != 1 || statuses[0].Phase != appsv1.LifecycleActionFailed || statuses[0].Code != "InvalidParameter" {
		t.Fatalf("expected one normalized failure, executions=%d status=%+v", executions, statuses[0])
	}
	if statuses[0].Retryable == nil || *statuses[0].Retryable || statuses[0].CompletionTime == nil {
		t.Fatalf("expected an explicitly non-retryable terminal result, status=%+v", statuses[0])
	}
	if statuses[0].Message != "lifecycle action failed" {
		t.Fatalf("raw action error must not be exposed, got %q", statuses[0].Message)
	}

	if state, err := ReconcileActionObservation(&statuses, key, execute); err != nil || state != ActionObservationFailed {
		t.Fatalf("expected terminal failure replay, got %q", state)
	}
	if executions != 1 {
		t.Fatalf("terminal failure must not execute again, executions=%d", executions)
	}
}

func TestReconcileActionObservationRejectsIncompleteExactKey(t *testing.T) {
	complete := ActionObservationKey{
		Action: "reconfigure", Subject: "mysql", Revision: "hash", PodName: "pod-0", PodUID: "pod-uid",
	}
	tests := map[string]func(*ActionObservationKey){
		"action":   func(key *ActionObservationKey) { key.Action = "" },
		"subject":  func(key *ActionObservationKey) { key.Subject = "" },
		"revision": func(key *ActionObservationKey) { key.Revision = "" },
		"podName":  func(key *ActionObservationKey) { key.PodName = "" },
		"podUID":   func(key *ActionObservationKey) { key.PodUID = "" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			key := complete
			mutate(&key)
			statuses := []appsv1.LifecycleActionStatus(nil)
			executed := false
			state, err := ReconcileActionObservation(&statuses, key, func() error {
				executed = true
				return nil
			})
			if err == nil || state != "" || executed || len(statuses) != 0 {
				t.Fatalf("expected incomplete %s to fail before persistence or execution, state=%q err=%v executed=%v statuses=%+v",
					name, state, err, executed, statuses)
			}
		})
	}
}

func TestReconcileActionObservationRetriesTransientAndUnclassifiedErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "normalized retryable failure",
			err:  &actionResultError{code: "Busy", retryable: ptr.To(true), err: ErrActionFailed},
		},
		{
			name: "normalized failure without code",
			err:  &actionResultError{retryable: ptr.To(false), err: ErrActionFailed},
		},
		{name: "busy", err: ErrActionBusy},
		{name: "timeout", err: ErrActionTimedOut},
		{name: "internal", err: ErrActionInternalError},
		{name: "transport", err: errors.New("connection reset")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-0", UID: types.UID("pod-uid")}}
			key := NewActionObservationKey(appsv1.LifecycleActionReconfigure, "mysql", "hash", pod)
			start := metav1.Now()
			statuses := []appsv1.LifecycleActionStatus{{
				Action: key.Action, Subject: key.Subject, Revision: key.Revision,
				Target: &appsv1.LifecycleActionTarget{PodName: key.PodName, PodUID: key.PodUID},
				Phase:  appsv1.LifecycleActionRunning, StartTime: &start,
			}}
			executions := 0
			execute := func() error {
				executions++
				return test.err
			}

			for attempt := 1; attempt <= 2; attempt++ {
				state, err := ReconcileActionObservation(&statuses, key, execute)
				if !errors.Is(err, test.err) || state != "" {
					t.Fatalf("expected retry-path error on attempt %d, state=%q err=%v", attempt, state, err)
				}
				if executions != attempt || statuses[0].Phase != appsv1.LifecycleActionRunning ||
					statuses[0].CompletionTime != nil || statuses[0].Code != "" || statuses[0].Retryable != nil {
					t.Fatalf("transient failure must stay retryable on attempt %d, executions=%d status=%+v",
						attempt, executions, statuses[0])
				}
			}
		})
	}
}

func TestFilterActionObservationsForPodDropsOldPodIncarnation(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-0", UID: types.UID("new-uid")}}
	statuses := []appsv1.LifecycleActionStatus{
		{Action: appsv1.LifecycleActionReconfigure, Subject: "mysql", Revision: "missing-target"},
		{Action: appsv1.LifecycleActionReconfigure, Subject: "mysql", Revision: "old", Target: &appsv1.LifecycleActionTarget{PodName: pod.Name, PodUID: "old-uid"}},
		{Action: appsv1.LifecycleActionReconfigure, Subject: "mysql", Revision: "new", Target: &appsv1.LifecycleActionTarget{PodName: pod.Name, PodUID: string(pod.UID)}},
	}

	filtered := FilterActionObservationsForPod(statuses, pod)
	if len(filtered) != 1 || filtered[0].Revision != "new" {
		t.Fatalf("expected only the current Pod result, got %+v", filtered)
	}
}
