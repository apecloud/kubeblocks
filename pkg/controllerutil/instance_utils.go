/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package controllerutil

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloadsv1alpha1 "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// IsInstanceReady returns true if an instance is ready
func IsInstanceReady(inst *workloadsv1alpha1.Instance) bool {
	return isInstanceReady(inst) && !isInstanceTerminating(inst)
}

// IsInstanceReadyWithRole checks if an instance is ready with the role label.
func IsInstanceReadyWithRole(inst *workloadsv1alpha1.Instance) bool {
	if _, ok := inst.Labels[constant.RoleLabelKey]; !ok {
		return false
	}
	return IsInstanceReady(inst)
}

// IsInstanceAvailable returns true if an instance is ready for at least minReadySeconds
func IsInstanceAvailable(inst *workloadsv1alpha1.Instance) bool {
	return isInstanceAvailable(inst) && !isInstanceTerminating(inst)
}

// isInstanceTerminating returns true if instance's DeletionTimestamp has been set
func isInstanceTerminating(inst *workloadsv1alpha1.Instance) bool {
	return inst.DeletionTimestamp != nil
}

// isInstanceAvailable returns true if an instance is available; false otherwise.
// Precondition for an available instance is that it must be ready. On top
// of that, there are two cases when an instance can be considered available:
// 1. minReadySeconds == 0, or
// 2. LastTransitionTime (is set) + minReadySeconds < current time
func isInstanceAvailable(inst *workloadsv1alpha1.Instance) bool {
	if !isInstanceReady(inst) {
		return false
	}

	minReadySeconds := inst.Spec.MinReadySeconds
	if minReadySeconds == 0 {
		return true
	}

	c := getInstanceReadyCondition(inst.Status)
	minReadySecondsDuration := time.Duration(minReadySeconds) * time.Second
	now := metav1.Now()
	return !c.LastTransitionTime.IsZero() && c.LastTransitionTime.Add(minReadySecondsDuration).Before(now.Time)
}

// isInstanceReady returns true if an instance is ready; false otherwise.
func isInstanceReady(inst *workloadsv1alpha1.Instance) bool {
	return isInstanceReadyConditionTrue(inst.Status)
}

// IsPodReadyConditionTrue returns true if a pod is ready; false otherwise.
func isInstanceReadyConditionTrue(status workloadsv1alpha1.InstanceStatus) bool {
	condition := getInstanceReadyCondition(status)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

// getInstanceReadyCondition extracts the instance ready condition from the given status and returns that.
// Returns nil if the condition is not present.
func getInstanceReadyCondition(status workloadsv1alpha1.InstanceStatus) *metav1.Condition {
	_, condition := getInstanceCondition(&status, "Ready")
	return condition
}

// getInstanceCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func getInstanceCondition(status *workloadsv1alpha1.InstanceStatus, conditionType string) (int, *metav1.Condition) {
	if status == nil {
		return -1, nil
	}
	return getInstanceConditionFromList(status.Conditions, conditionType)
}

// getInstanceConditionFromList extracts the provided condition from the given list of condition and
// returns the index of the condition and the condition. Returns -1 and nil if the condition is not present.
func getInstanceConditionFromList(conditions []metav1.Condition, conditionType string) (int, *metav1.Condition) {
	if conditions == nil {
		return -1, nil
	}
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return i, &conditions[i]
		}
	}
	return -1, nil
}
