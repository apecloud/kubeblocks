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
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// ActionObservationKey identifies one Action result on one exact Pod incarnation.
type ActionObservationKey struct {
	Action   string
	Subject  string
	Revision string
	PodName  string
	PodUID   string
}

// NewActionObservationKey builds a key for an Action executed on pod.
func NewActionObservationKey(action, subject, revision string, pod *corev1.Pod) ActionObservationKey {
	key := ActionObservationKey{Action: action, Subject: subject, Revision: revision}
	if pod != nil {
		key.PodName = pod.Name
		key.PodUID = string(pod.UID)
	}
	return key
}

// ActionObservationState tells a workload reconciler whether it should proceed,
// persist an observation transition, or stop on a terminal Action failure.
type ActionObservationState string

const (
	ActionObservationUpdated   ActionObservationState = "Updated"
	ActionObservationSucceeded ActionObservationState = "Succeeded"
	ActionObservationFailed    ActionObservationState = "Failed"
)

// ReconcileActionObservation advances one durable Action observation.
//
// A new invocation is persisted as Pending and then Running before execute is
// called. Explicitly non-retryable normalized failures are committed as terminal
// status, and an undefined action is committed as Skipped; retryable and
// unclassified execution errors are returned for retry.
func ReconcileActionObservation(statuses *[]appsv1.LifecycleActionStatus, key ActionObservationKey,
	execute func() error) (ActionObservationState, error) {
	if statuses == nil {
		return "", fmt.Errorf("lifecycle action observation status destination must not be nil")
	}
	for _, item := range []struct {
		field string
		value string
	}{
		{field: "action", value: key.Action},
		{field: "subject", value: key.Subject},
		{field: "revision", value: key.Revision},
		{field: "podName", value: key.PodName},
		{field: "podUID", value: key.PodUID},
	} {
		if item.value == "" {
			return "", fmt.Errorf("lifecycle action observation key %s must not be empty", item.field)
		}
	}
	if execute == nil {
		return "", fmt.Errorf("lifecycle action observation executor must not be nil")
	}
	index := findActionObservation(*statuses, key)
	if index < 0 {
		*statuses = upsertActionObservation(*statuses, appsv1.LifecycleActionStatus{
			Action:   key.Action,
			Subject:  key.Subject,
			Revision: key.Revision,
			Target: &appsv1.LifecycleActionTarget{
				PodName: key.PodName,
				PodUID:  key.PodUID,
			},
			Phase:   appsv1.LifecycleActionPending,
			Message: "waiting for lifecycle action execution",
		})
		return ActionObservationUpdated, nil
	}

	status := (*statuses)[index]
	switch status.Phase {
	case appsv1.LifecycleActionSucceeded, appsv1.LifecycleActionSkipped:
		return ActionObservationSucceeded, nil
	case appsv1.LifecycleActionFailed:
		return ActionObservationFailed, nil
	case appsv1.LifecycleActionPending:
		now := metav1.Now()
		status.Phase = appsv1.LifecycleActionRunning
		status.Message = "lifecycle action is running"
		status.StartTime = &now
		status.CompletionTime = nil
		status.Code = ""
		status.Retryable = nil
		*statuses = upsertActionObservation(*statuses, status)
		return ActionObservationUpdated, nil
	case appsv1.LifecycleActionRunning:
		// Continue below and execute the previously persisted invocation.
	default:
		status.Phase = appsv1.LifecycleActionPending
		status.Message = "waiting for lifecycle action execution"
		status.StartTime = nil
		status.CompletionTime = nil
		*statuses = upsertActionObservation(*statuses, status)
		return ActionObservationUpdated, nil
	}

	err := execute()
	if errors.Is(err, ErrPreconditionFailed) {
		status.Phase = appsv1.LifecycleActionPending
		status.Message = "waiting for lifecycle action preconditions"
		status.StartTime = nil
		status.CompletionTime = nil
		*statuses = upsertActionObservation(*statuses, status)
		return ActionObservationUpdated, nil
	}
	if errors.Is(err, ErrActionNotDefined) {
		now := metav1.Now()
		status.Phase = appsv1.LifecycleActionSkipped
		status.Message = "lifecycle action is not defined"
		status.CompletionTime = &now
		status.Code = ""
		status.Retryable = nil
		*statuses = upsertActionObservation(*statuses, status)
		return ActionObservationUpdated, nil
	}
	if err != nil && !isTerminalActionFailure(err) {
		return "", err
	}

	now := metav1.Now()
	status.CompletionTime = &now
	switch err {
	case nil:
		status.Phase = appsv1.LifecycleActionSucceeded
		status.Message = "lifecycle action succeeded"
	default:
		status.Phase = appsv1.LifecycleActionFailed
		status.Message = "lifecycle action failed"
		status.Code, _ = ActionErrorCode(err)
		status.Retryable = ActionErrorRetryable(err)
	}
	*statuses = upsertActionObservation(*statuses, status)
	return ActionObservationUpdated, nil
}

func isTerminalActionFailure(err error) bool {
	code, coded := ActionErrorCode(err)
	retryable := ActionErrorRetryable(err)
	return errors.Is(err, ErrActionFailed) && coded && code != "" && retryable != nil && !*retryable
}

// FilterActionObservationsForPod drops results from an older Pod incarnation.
func FilterActionObservationsForPod(statuses []appsv1.LifecycleActionStatus, pod *corev1.Pod) []appsv1.LifecycleActionStatus {
	if pod == nil || len(statuses) == 0 {
		return nil
	}
	result := make([]appsv1.LifecycleActionStatus, 0, len(statuses))
	for _, status := range statuses {
		if status.Target != nil && status.Target.PodName == pod.Name && status.Target.PodUID == string(pod.UID) {
			result = append(result, copyActionObservation(status))
		}
	}
	sortActionObservations(result)
	return result
}

func findActionObservation(statuses []appsv1.LifecycleActionStatus, key ActionObservationKey) int {
	for i := range statuses {
		status := &statuses[i]
		if status.Target != nil && status.Action == key.Action && status.Subject == key.Subject && status.Revision == key.Revision &&
			status.Target.PodName == key.PodName && status.Target.PodUID == key.PodUID {
			return i
		}
	}
	return -1
}

func upsertActionObservation(statuses []appsv1.LifecycleActionStatus, observation appsv1.LifecycleActionStatus) []appsv1.LifecycleActionStatus {
	result := make([]appsv1.LifecycleActionStatus, 0, len(statuses)+1)
	for _, status := range statuses {
		if sameActionObservationSubject(status, observation) {
			continue
		}
		result = append(result, copyActionObservation(status))
	}
	result = append(result, copyActionObservation(observation))
	sortActionObservations(result)
	return result
}

func sameActionObservationSubject(left, right appsv1.LifecycleActionStatus) bool {
	if left.Action != right.Action || left.Subject != right.Subject || left.Target == nil || right.Target == nil {
		return false
	}
	return left.Target.PodName == right.Target.PodName
}

func sortActionObservations(statuses []appsv1.LifecycleActionStatus) {
	sort.SliceStable(statuses, func(i, j int) bool {
		left, right := statuses[i], statuses[j]
		leftPod, rightPod := "", ""
		if left.Target != nil {
			leftPod = left.Target.PodName
		}
		if right.Target != nil {
			rightPod = right.Target.PodName
		}
		if leftPod != rightPod {
			return leftPod < rightPod
		}
		if left.Action != right.Action {
			return left.Action < right.Action
		}
		if left.Subject != right.Subject {
			return left.Subject < right.Subject
		}
		return left.Revision < right.Revision
	})
}

func copyActionObservation(status appsv1.LifecycleActionStatus) appsv1.LifecycleActionStatus {
	copy := status
	if status.Target != nil {
		target := *status.Target
		copy.Target = &target
	}
	if status.Retryable != nil {
		retryable := *status.Retryable
		copy.Retryable = &retryable
	}
	if status.StartTime != nil {
		startTime := status.StartTime.DeepCopy()
		copy.StartTime = startTime
	}
	if status.CompletionTime != nil {
		completionTime := status.CompletionTime.DeepCopy()
		copy.CompletionTime = completionTime
	}
	return copy
}
