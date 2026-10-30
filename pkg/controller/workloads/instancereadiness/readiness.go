/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package instancereadiness

import (
	"encoding/json"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// Health is one trusted observation of an identity's runtime health.
type Health struct {
	Ready     bool
	Available bool
	Pending   bool
}

// Pod observes runtime health using the current parent's availability policy.
func Pod(pod *corev1.Pod, minReadySeconds int32) Health {
	if pod == nil || !intctrlutil.IsPodReady(pod) {
		return Health{}
	}
	for _, container := range pod.Spec.Containers {
		index := slices.IndexFunc(pod.Status.ContainerStatuses, func(status corev1.ContainerStatus) bool {
			return status.Name == container.Name
		})
		if index < 0 {
			continue
		}
		status := pod.Status.ContainerStatuses[index]
		if !intctrlutil.MatchContainerImageInStatus(container.Image, status.Image, status.ImageID) {
			return Health{}
		}
	}
	return Health{Ready: true, Available: intctrlutil.IsPodAvailable(pod, minReadySeconds)}
}

// Publish evaluates configured identities independently of extra runtime objects.
func Publish(its *workloads.InstanceSet, assignments []instancetemplate.Assignment, health map[string]Health, assignmentErr error) (bool, error) {
	if assignmentErr != nil {
		if !instancetemplate.IsAssignmentIncomplete(assignmentErr) {
			return false, assignmentErr
		}
		for _, kind := range []workloads.ConditionType{workloads.InstanceReady, workloads.InstanceAvailable} {
			meta.SetStatusCondition(&its.Status.Conditions, metav1.Condition{
				Type: string(kind), Status: metav1.ConditionUnknown, ObservedGeneration: its.Generation,
				Reason: "AssignmentIncomplete", Message: assignmentErr.Error(),
			})
		}
		return false, nil
	}
	var notReady, notAvailable []string
	pending := false
	for _, member := range health {
		pending = pending || member.Pending || member.Ready && !member.Available
	}
	for _, assignment := range assignments {
		member := health[assignment.InstanceName]
		if !member.Ready {
			notReady = append(notReady, assignment.InstanceName)
		}
		if !member.Ready || !member.Available {
			notAvailable = append(notAvailable, assignment.InstanceName)
		}
	}
	for _, result := range []struct {
		kind          workloads.ConditionType
		unhealthy     []string
		successReason string
		failureReason string
	}{
		{workloads.InstanceReady, notReady, workloads.ReasonReady, workloads.ReasonNotReady},
		{workloads.InstanceAvailable, notAvailable, workloads.ReasonAvailable, workloads.ReasonNotAvailable},
	} {
		condition := metav1.Condition{Type: string(result.kind), Status: metav1.ConditionTrue, ObservedGeneration: its.Generation, Reason: result.successReason}
		if len(result.unhealthy) > 0 {
			condition.Status = metav1.ConditionFalse
			condition.Reason = result.failureReason
			message, err := json.Marshal(result.unhealthy)
			if err != nil {
				return false, err
			}
			condition.Message = string(message)
		}
		meta.SetStatusCondition(&its.Status.Conditions, condition)
	}
	return pending, nil
}

// PublicationChanged identifies observations that must persist before child changes.
func PublicationChanged(previous, current []metav1.Condition) bool {
	for _, kind := range []string{string(workloads.InstanceReady), string(workloads.InstanceAvailable)} {
		condition := meta.FindStatusCondition(current, kind)
		old := meta.FindStatusCondition(previous, kind)
		if condition != nil && (old == nil || old.Status != condition.Status || old.ObservedGeneration != condition.ObservedGeneration) {
			return true
		}
	}
	return false
}
