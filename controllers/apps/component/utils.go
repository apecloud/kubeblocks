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

package component

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	// reasonPreCheckSucceed preChecks succeeded for provisioning started
	reasonPreCheckSucceed = "PreCheckSucceed"

	// reasonPreCheckFailed preChecks failed for provisioning started
	reasonPreCheckFailed = "PreCheckFailed"
)

func setProvisioningStartedCondition(conditions *[]metav1.Condition, clusterName string, clusterGeneration int64, err error) {
	var condition metav1.Condition
	if err == nil {
		condition = newProvisioningStartedCondition(clusterName, clusterGeneration)
	} else {
		condition = newFailedProvisioningStartedCondition(err)
	}
	meta.SetStatusCondition(conditions, condition)
}

// newProvisioningStartedCondition creates the provisioning started condition in cluster conditions.
func newProvisioningStartedCondition(clusterName string, clusterGeneration int64) metav1.Condition {
	return metav1.Condition{
		Type:               appsv1.ConditionTypeProvisioningStarted,
		ObservedGeneration: clusterGeneration,
		Status:             metav1.ConditionTrue,
		Message:            fmt.Sprintf("The operator has started the provisioning of Cluster: %s", clusterName),
		Reason:             reasonPreCheckSucceed,
	}
}

func getConditionReasonWithError(defaultReason string, err error) string {
	if err == nil {
		return defaultReason
	}
	controllerErr := intctrlutil.UnwrapControllerError(err)
	if controllerErr != nil {
		defaultReason = string(controllerErr.Type)
	}
	return defaultReason
}

// newApplyResourcesCondition creates a condition when applied resources succeed.
func newFailedProvisioningStartedCondition(err error) metav1.Condition {
	return metav1.Condition{
		Type:    appsv1.ConditionTypeProvisioningStarted,
		Status:  metav1.ConditionFalse,
		Message: err.Error(),
		Reason:  getConditionReasonWithError(reasonPreCheckFailed, err),
	}
}

func setDiff(s1, s2 sets.Set[string]) (sets.Set[string], sets.Set[string], sets.Set[string]) {
	return s2.Difference(s1), s1.Difference(s2), s1.Intersection(s2)
}

func mapDiff[T interface{}](m1, m2 map[string]T) (sets.Set[string], sets.Set[string], sets.Set[string]) {
	s1, s2 := sets.KeySet(m1), sets.KeySet(m2)
	return setDiff(s1, s2)
}
