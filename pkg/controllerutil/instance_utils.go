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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

// IsInstanceReadyWithRole checks if an instance is ready with the role label.
func IsInstanceReadyWithRole(inst *workloads.Instance) bool {
	if len(inst.Spec.Roles) > 0 && inst.Status.Role == nil {
		return false
	}
	return IsInstanceReady(inst)
}

// IsInstanceReady returns true if an instance is ready
func IsInstanceReady(inst *workloads.Instance) bool {
	return isInstanceReady(inst) && !IsInstanceTerminating(inst)
}

// IsInstanceAvailable returns true if an instance is ready for at least minReadySeconds
func IsInstanceAvailable(inst *workloads.Instance) bool {
	return isInstanceAvailable(inst) && !IsInstanceTerminating(inst)
}

func IsInstanceFailure(inst *workloads.Instance) bool {
	return isInstanceFailure(inst) && !IsInstanceTerminating(inst)
}

// IsInstanceTerminating returns true if instance's DeletionTimestamp has been set
func IsInstanceTerminating(inst *workloads.Instance) bool {
	return inst.DeletionTimestamp != nil
}

// isInstanceReady returns true if an instance is ready; false otherwise.
func isInstanceReady(inst *workloads.Instance) bool {
	_, condition := getInstanceCondition(inst, workloads.InstanceReady)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

// isInstanceAvailable returns true if an instance is available; false otherwise.
func isInstanceAvailable(inst *workloads.Instance) bool {
	_, condition := getInstanceCondition(inst, workloads.InstanceAvailable)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

func isInstanceFailure(inst *workloads.Instance) bool {
	_, condition := getInstanceCondition(inst, workloads.InstanceFailure)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

// getInstanceCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func getInstanceCondition(inst *workloads.Instance, conditionType workloads.ConditionType) (int, *metav1.Condition) {
	return getInstanceConditionFromList(inst.Status.Conditions, string(conditionType))
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
