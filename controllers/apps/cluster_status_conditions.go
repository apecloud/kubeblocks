/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package apps

import (
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	ReasonPreCheckSucceed       = "PreCheckSucceed"       // ReasonPreCheckSucceed preChecks succeeded for provisioning started
	ReasonPreCheckFailed        = "PreCheckFailed"        // ReasonPreCheckFailed preChecks failed for provisioning started
	ReasonApplyResourcesFailed  = "ApplyResourcesFailed"  // ReasonApplyResourcesFailed applies resources failed to create or change the cluster
	ReasonApplyResourcesSucceed = "ApplyResourcesSucceed" // ReasonApplyResourcesSucceed applies resources succeeded to create or change the cluster
	ReasonClusterReady          = "ClusterReady"          // ReasonClusterReady the components of cluster are ready, the component phase is running
	ReasonComponentsNotReady    = "ComponentsNotReady"    // ReasonComponentsNotReady the components of cluster are not ready
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
		Reason:             ReasonPreCheckSucceed,
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
		Reason:  getConditionReasonWithError(ReasonPreCheckFailed, err),
	}
}

func setApplyResourceCondition(conditions *[]metav1.Condition, clusterGeneration int64, err error) {
	condition := newApplyResourcesCondition(clusterGeneration)
	// ignore requeue error
	if err != nil && !intctrlutil.IsRequeueError(err) {
		condition = newFailedApplyResourcesCondition(err)
	}
	meta.SetStatusCondition(conditions, condition)
}

func newApplyResourcesCondition(clusterGeneration int64) metav1.Condition {
	return metav1.Condition{
		Type:               appsv1.ConditionTypeApplyResources,
		ObservedGeneration: clusterGeneration,
		Status:             metav1.ConditionTrue,
		Message:            "Successfully applied for resources",
		Reason:             ReasonApplyResourcesSucceed,
	}
}

func newFailedApplyResourcesCondition(err error) metav1.Condition {
	return metav1.Condition{
		Type:    appsv1.ConditionTypeApplyResources,
		Status:  metav1.ConditionFalse,
		Message: err.Error(),
		Reason:  getConditionReasonWithError(ReasonApplyResourcesFailed, err),
	}
}

func newClusterReadyCondition(clusterName string) metav1.Condition {
	return metav1.Condition{
		Type:    appsv1.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Message: fmt.Sprintf("cluster %s is ready", clusterName),
		Reason:  ReasonClusterReady,
	}
}

func newClusterNotReadyCondition(clusterName string, kindNames map[string][]string) metav1.Condition {
	messages := make([]string, 0)
	for kind, names := range kindNames {
		if len(names) > 0 {
			slices.Sort(names)
			messages = append(messages, fmt.Sprintf("unavailable %ss: %s", kind, strings.Join(names, ",")))
		}
	}
	return metav1.Condition{
		Type:    appsv1.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Message: fmt.Sprintf("cluster %s is NOT ready, %s", clusterName, strings.Join(messages, ",")),
		Reason:  ReasonComponentsNotReady,
	}
}
