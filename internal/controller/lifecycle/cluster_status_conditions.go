/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package lifecycle

import (
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	ReasonOpsRequestProcessed   = "Processed"             // ReasonOpsRequestProcessed the latest OpsRequest has been processed.
	ReasonPreCheckSucceed       = "PreCheckSucceed"       // ReasonPreCheckSucceed preChecks succeeded for provisioning started
	ReasonPreCheckFailed        = "PreCheckFailed"        // ReasonPreCheckFailed preChecks failed for provisioning started
	ReasonApplyResourcesFailed  = "ApplyResourcesFailed"  // ReasonApplyResourcesFailed applies resources failed to create or change the cluster
	ReasonApplyResourcesSucceed = "ApplyResourcesSucceed" // ReasonApplyResourcesSucceed applies resources succeeded to create or change the cluster
	ReasonReplicasNotReady      = "ReplicasNotReady"      // ReasonReplicasNotReady the pods of components are not ready
	ReasonAllReplicasReady      = "AllReplicasReady"      // ReasonAllReplicasReady the pods of components are ready
	ReasonComponentsNotReady    = "ComponentsNotReady"    // ReasonComponentsNotReady the components of cluster are not ready
	ReasonClusterReady          = "ClusterReady"          // ReasonClusterReady the components of cluster are ready, the component phase is running
)

// conditionIsChanged checks if the condition is changed.
func conditionIsChanged(oldCondition *metav1.Condition, newCondition metav1.Condition) bool {
	if newCondition.LastTransitionTime.IsZero() && oldCondition != nil {
		// assign the old condition's LastTransitionTime to the new condition for "DeepEqual" checking.
		newCondition.LastTransitionTime = oldCondition.LastTransitionTime
	}
	return !reflect.DeepEqual(oldCondition, &newCondition)
}

func setProvisioningStartedCondition(conditions *[]metav1.Condition, clusterName string, clusterGeneration int64, err error) {
	condition := newProvisioningStartedCondition(clusterName, clusterGeneration)
	if err != nil {
		condition = newFailedProvisioningStartedCondition(err)
	}
	meta.SetStatusCondition(conditions, condition)
}

// newProvisioningStartedCondition creates the provisioning started condition in cluster conditions.
func newProvisioningStartedCondition(clusterName string, clusterGeneration int64) metav1.Condition {
	return metav1.Condition{
		Type:               appsv1alpha1.ConditionTypeProvisioningStarted,
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
	controllerErr := intctrlutil.ToControllerError(err)
	if controllerErr != nil {
		defaultReason = string(controllerErr.Type)
	}
	return defaultReason
}

// newApplyResourcesCondition creates a condition when applied resources succeed.
func newFailedProvisioningStartedCondition(err error) metav1.Condition {
	return metav1.Condition{
		Type:    appsv1alpha1.ConditionTypeProvisioningStarted,
		Status:  metav1.ConditionFalse,
		Message: err.Error(),
		Reason:  getConditionReasonWithError(ReasonPreCheckFailed, err),
	}
}

func setApplyResourceCondition(conditions *[]metav1.Condition, clusterGeneration int64, err error) {
	condition := newApplyResourcesCondition(clusterGeneration)
	// ignore requeue error
	if err != nil && !intctrlutil.IsRequeueOrDelayedRequeueError(err) {
		condition = newFailedApplyResourcesCondition(err)
	}
	meta.SetStatusCondition(conditions, condition)
}

// newApplyResourcesCondition creates a condition when applied resources succeed.
func newApplyResourcesCondition(clusterGeneration int64) metav1.Condition {
	return metav1.Condition{
		Type:               appsv1alpha1.ConditionTypeApplyResources,
		ObservedGeneration: clusterGeneration,
		Status:             metav1.ConditionTrue,
		Message:            "Successfully applied for resources",
		Reason:             ReasonApplyResourcesSucceed,
	}
}

// newApplyResourcesCondition creates a condition when applied resources succeed.
func newFailedApplyResourcesCondition(err error) metav1.Condition {
	return metav1.Condition{
		Type:    appsv1alpha1.ConditionTypeApplyResources,
		Status:  metav1.ConditionFalse,
		Message: err.Error(),
		Reason:  getConditionReasonWithError(ReasonApplyResourcesFailed, err),
	}
}

// newAllReplicasPodsReadyConditions creates a condition when all pods of components are ready
func newAllReplicasPodsReadyConditions() metav1.Condition {
	return metav1.Condition{
		Type:    appsv1alpha1.ConditionTypeReplicasReady,
		Status:  metav1.ConditionTrue,
		Message: "all pods of components are ready, waiting for the probe detection successful",
		Reason:  ReasonAllReplicasReady,
	}
}

// newReplicasNotReadyCondition creates a condition when pods of components are not ready
func newReplicasNotReadyCondition(notReadyComponentNames map[string]struct{}) metav1.Condition {
	cNameSlice := maps.Keys(notReadyComponentNames)
	slices.Sort(cNameSlice)
	return metav1.Condition{
		Type:    appsv1alpha1.ConditionTypeReplicasReady,
		Status:  metav1.ConditionFalse,
		Message: fmt.Sprintf("pods are not ready in Components: %v, refer to related component message in Cluster.status.components", cNameSlice),
		Reason:  ReasonReplicasNotReady,
	}
}

// newClusterReadyCondition creates a condition when all components of cluster are running
func newClusterReadyCondition(clusterName string) metav1.Condition {
	return metav1.Condition{
		Type:    appsv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Message: fmt.Sprintf("Cluster: %s is ready, current phase is Running", clusterName),
		Reason:  ReasonClusterReady,
	}
}

// newComponentsNotReadyCondition creates a condition when components of cluster are not ready
func newComponentsNotReadyCondition(notReadyComponentNames map[string]struct{}) metav1.Condition {
	cNameSlice := maps.Keys(notReadyComponentNames)
	slices.Sort(cNameSlice)
	return metav1.Condition{
		Type:    appsv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Message: fmt.Sprintf("pods are unavailable in Components: %v, refer to related component message in Cluster.status.components", cNameSlice),
		Reason:  ReasonComponentsNotReady,
	}
}

// newOpsRequestProcessingCondition creates a condition when the latest opsRequest of cluster is processing
func newOpsRequestProcessingCondition(opsName, opsType, reason string) metav1.Condition {
	return metav1.Condition{
		Type:    appsv1alpha1.ConditionTypeLatestOpsRequestProcessed,
		Status:  metav1.ConditionFalse,
		Message: fmt.Sprintf("%s opsRequest: %s is processing", opsType, opsName),
		Reason:  reason,
	}
}

// newOpsRequestProcessedCondition creates a condition when the latest opsRequest of cluster has been processed
func newOpsRequestProcessedCondition(processingMessage string) metav1.Condition {
	return metav1.Condition{
		Type:    appsv1alpha1.ConditionTypeLatestOpsRequestProcessed,
		Status:  metav1.ConditionTrue,
		Message: strings.Replace(processingMessage, "is processing", "has been processed", 1),
		Reason:  ReasonOpsRequestProcessed,
	}
}
