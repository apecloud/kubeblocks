/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lifecycle

import (
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

const (
	ReasonOpsRequestProcessed   = "Processed"             // ReasonOpsRequestProcessed the latest OpsRequest has been processed.
	ReasonPreCheckSucceed       = "PreCheckSucceed"       // ReasonPreCheckSucceed preChecks succeed for provisioning started
	ReasonPreCheckFailed        = "PreCheckFailed"        // ReasonPreCheckFailed preChecks failed for provisioning started
	ReasonApplyResourcesFailed  = "ApplyResourcesFailed"  // ReasonApplyResourcesFailed applies resources failed to create or change the cluster
	ReasonApplyResourcesSucceed = "ApplyResourcesSucceed" // ReasonApplyResourcesSucceed applies resources succeed to create or change the cluster
	ReasonReplicasNotReady      = "ReplicasNotReady"      // ReasonReplicasNotReady the pods of components are not ready
	ReasonAllReplicasReady      = "AllReplicasReady"      // ReasonAllReplicasReady the pods of components are ready
	ReasonComponentsNotReady    = "ComponentsNotReady"    // ReasonComponentsNotReady the components of cluster are not ready
	ReasonClusterReady          = "ClusterReady"          // ReasonClusterReady the components of cluster are ready, the component phase are running
)

// conditionIsChanged checks if the condition is changed.
func conditionIsChanged(oldCondition *metav1.Condition, newCondition metav1.Condition) bool {
	if newCondition.LastTransitionTime.IsZero() && oldCondition != nil {
		// assign the old condition's LastTransitionTime to the new condition for "DeepEqual" checking.
		newCondition.LastTransitionTime = oldCondition.LastTransitionTime
	}
	return !reflect.DeepEqual(oldCondition, &newCondition)
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

// newApplyResourcesCondition creates a condition when applied resources succeed.
func newFailedProvisioningStartedCondition(message, reason string) metav1.Condition {
	return metav1.Condition{
		Type:    appsv1alpha1.ConditionTypeProvisioningStarted,
		Status:  metav1.ConditionFalse,
		Message: message,
		Reason:  reason,
	}
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
func newFailedApplyResourcesCondition(message string) metav1.Condition {
	return metav1.Condition{
		Type:    appsv1alpha1.ConditionTypeApplyResources,
		Status:  metav1.ConditionFalse,
		Message: message,
		Reason:  ReasonApplyResourcesFailed,
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
