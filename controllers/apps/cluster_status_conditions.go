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

package apps

import (
	"context"
	"fmt"
	"reflect"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	componentutil "github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type clusterConditionManager struct {
	client.Client
	Recorder record.EventRecorder
	ctx      context.Context
	cluster  *appsv1alpha1.Cluster
}

const (
	// ConditionTypeProvisioningStarted the operator starts resource provisioning to create or change the cluster
	ConditionTypeProvisioningStarted = "ProvisioningStarted"
	// ConditionTypeApplyResources the operator start to apply resources to create or change the cluster
	ConditionTypeApplyResources = "ApplyResources"
	// ConditionTypeReplicasReady all pods of components are ready
	ConditionTypeReplicasReady = "ReplicasReady"
	// ConditionTypeReady all components are running
	ConditionTypeReady = "Ready"

	// ReasonPreCheckSucceed preChecks succeed for provisioning started
	ReasonPreCheckSucceed = "PreCheckSucceed"
	// ReasonPreCheckFailed preChecks failed for provisioning started
	ReasonPreCheckFailed = "PreCheckFailed"
	// ReasonApplyResourcesFailed applies resources failed to create or change the cluster
	ReasonApplyResourcesFailed = "ApplyResourcesFailed"
	// ReasonApplyResourcesSucceed applies resources succeed to create or change the cluster
	ReasonApplyResourcesSucceed = "ApplyResourcesSucceed"
	// ReasonReplicasNotReady the pods of components are not ready
	ReasonReplicasNotReady = "ReplicasNotReady"
	// ReasonAllReplicasReady the pods of components are ready
	ReasonAllReplicasReady = "AllReplicasReady"
	// ReasonComponentsNotReady the components of cluster are not ready
	ReasonComponentsNotReady = "ComponentsNotReady"
	// ReasonClusterReady the components of cluster are ready, the component phase are running
	ReasonClusterReady = "ClusterReady"

	// // ClusterControllerErrorDuration if there is an error in the cluster controller,
	// // it will not be automatically repaired unless there is network jitter.
	// // so if the error lasts more than 5s, the cluster will enter the ConditionsError phase
	// // and prompt the user to repair manually according to the message.
	// ClusterControllerErrorDuration = 5 * time.Second

	// // ControllerErrorRequeueTime the requeue time to reconcile the error event of the cluster controller
	// // which need to respond to user repair events timely.
	// ControllerErrorRequeueTime = 5 * time.Second
)

// REVIEW: this handling patches co-relation object upon condition patch succeed (cascade issue),
// need better handling technique; function handling is monolithic, call for refactor.
//
// updateClusterConditions updates cluster.status condition and records event.
// Deprecated: avoid monolithic and cascade processing
func (conMgr clusterConditionManager) updateStatusConditions(condition metav1.Condition) error {
	patch := client.MergeFrom(conMgr.cluster.DeepCopy())
	oldCondition := meta.FindStatusCondition(conMgr.cluster.Status.Conditions, condition.Type)
	conditionChanged := !reflect.DeepEqual(oldCondition, condition)
	if conditionChanged {
		meta.SetStatusCondition(&conMgr.cluster.Status.Conditions, condition)
		if err := conMgr.Client.Status().Patch(conMgr.ctx, conMgr.cluster, patch); err != nil {
			return err
		}
	}
	if conditionChanged {
		eventType := corev1.EventTypeWarning
		if condition.Status == metav1.ConditionTrue {
			eventType = corev1.EventTypeNormal
		}
		conMgr.Recorder.Event(conMgr.cluster, eventType, condition.Reason, condition.Message)
	}
	// REVIEW/TODO: tmp remove following for interaction with OpsRequest
	// if phaseChanged {
	// 	// if cluster status changed, do it
	// 	return opsutil.MarkRunningOpsRequestAnnotation(conMgr.ctx, conMgr.Client, conMgr.cluster)
	// }
	return componentutil.ErrNoOps
}

// setProvisioningStartedCondition sets the provisioning started condition in cluster conditions.
// @return could return ErrNoOps
// Deprecated: avoid monolithic handling
func (conMgr clusterConditionManager) setProvisioningStartedCondition() error {
	condition := metav1.Condition{
		Type:    ConditionTypeProvisioningStarted,
		Status:  metav1.ConditionTrue,
		Message: fmt.Sprintf("The operator has started the provisioning of Cluster: %s", conMgr.cluster.Name),
		Reason:  ReasonPreCheckSucceed,
	}
	return conMgr.updateStatusConditions(condition)
}

// setPreCheckErrorCondition sets the error condition when preCheck failed.
// @return could return ErrNoOps
// Deprecated: avoid monolithic handling
func (conMgr clusterConditionManager) setPreCheckErrorCondition(err error) error {
	var message string
	if err != nil {
		message = err.Error()
	}
	reason := ReasonPreCheckFailed
	if apierrors.IsNotFound(err) {
		reason = constant.ReasonNotFoundCR
	}
	return conMgr.updateStatusConditions(newFailedProvisioningStartedCondition(err.Error(), reason))
}

// setUnavailableCondition sets the condition that reference CRs are unavailable.
// @return could return ErrNoOps
// Deprecated: avoid monolithic handling
func (conMgr clusterConditionManager) setReferenceCRUnavailableCondition(message string) error {
	return conMgr.updateStatusConditions(newFailedProvisioningStartedCondition(
		message, constant.ReasonRefCRUnavailable))
}

// setApplyResourcesFailedCondition sets applied resources failed condition in cluster conditions.
// @return could return ErrNoOps
// Deprecated: avoid monolithic handling
func (conMgr clusterConditionManager) setApplyResourcesFailedCondition(message string) error {
	return conMgr.updateStatusConditions(newFailedApplyResourcesCondition(message))
}

// newApplyResourcesCondition creates a condition when applied resources succeed.
func newFailedProvisioningStartedCondition(message, reason string) metav1.Condition {
	return metav1.Condition{
		Type:    ConditionTypeProvisioningStarted,
		Status:  metav1.ConditionFalse,
		Message: message,
		Reason:  reason,
	}
}

// newApplyResourcesCondition creates a condition when applied resources succeed.
func newApplyResourcesCondition() metav1.Condition {
	return metav1.Condition{
		Type:    ConditionTypeApplyResources,
		Status:  metav1.ConditionTrue,
		Message: "Successfully applied for resources",
		Reason:  ReasonApplyResourcesSucceed,
	}
}

// newApplyResourcesCondition creates a condition when applied resources succeed.
func newFailedApplyResourcesCondition(message string) metav1.Condition {
	return metav1.Condition{
		Type:    ConditionTypeApplyResources,
		Status:  metav1.ConditionFalse,
		Message: message,
		Reason:  ReasonApplyResourcesFailed,
	}
}

// newAllReplicasPodsReadyConditions creates a condition when all pods of components are ready
func newAllReplicasPodsReadyConditions() metav1.Condition {
	return metav1.Condition{
		Type:    ConditionTypeReplicasReady,
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
		Type:    ConditionTypeReplicasReady,
		Status:  metav1.ConditionFalse,
		Message: fmt.Sprintf("pods are not ready in Components: %v, refer to related component message in Cluster.status.components", cNameSlice),
		Reason:  ReasonReplicasNotReady,
	}
}

// newClusterReadyCondition creates a condition when all components of cluster are running
func newClusterReadyCondition(clusterName string) metav1.Condition {
	return metav1.Condition{
		Type:    ConditionTypeReady,
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
		Type:    ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Message: fmt.Sprintf("pods are unavailable in Components: %v, refer to related component message in Cluster.status.components", cNameSlice),
		Reason:  ReasonComponentsNotReady,
	}
}

// checkConditionIsChanged checks if the condition is changed.
func checkConditionIsChanged(oldCondition *metav1.Condition, newCondition metav1.Condition) bool {
	if oldCondition == nil {
		return true
	}
	return oldCondition.Message != newCondition.Message
}

// handleClusterReadyCondition handles the cluster conditions with ClusterReady and ReplicasReady type.
// @return could return ErrNoOps
func handleNotReadyConditionForCluster(cluster *appsv1alpha1.Cluster,
	recorder record.EventRecorder,
	replicasNotReadyCompNames map[string]struct{},
	notReadyCompNames map[string]struct{}) (postHandler, error) {
	oldReplicasReadyCondition := meta.FindStatusCondition(cluster.Status.Conditions, ConditionTypeReplicasReady)
	if len(replicasNotReadyCompNames) == 0 {
		// if all replicas of cluster are ready, set ReasonAllReplicasReady to status.conditions
		readyCondition := newAllReplicasPodsReadyConditions()
		if checkConditionIsChanged(oldReplicasReadyCondition, readyCondition) {
			meta.SetStatusCondition(&cluster.Status.Conditions, readyCondition)
			postFunc := func(cluster *appsv1alpha1.Cluster) error {
				// send an event when all pods of the components are ready.
				recorder.Event(cluster, corev1.EventTypeNormal, readyCondition.Reason, readyCondition.Message)
				return nil
			}
			return postFunc, nil
		}
	} else {
		replicasNotReadyCond := newReplicasNotReadyCondition(replicasNotReadyCompNames)
		if checkConditionIsChanged(oldReplicasReadyCondition, replicasNotReadyCond) {
			meta.SetStatusCondition(&cluster.Status.Conditions, replicasNotReadyCond)
			return nil, nil
		}
	}

	if len(notReadyCompNames) > 0 {
		oldClusterReadyCondition := meta.FindStatusCondition(cluster.Status.Conditions, ConditionTypeReady)
		clusterNotReadyCondition := newComponentsNotReadyCondition(notReadyCompNames)
		if checkConditionIsChanged(oldClusterReadyCondition, clusterNotReadyCondition) {
			meta.SetStatusCondition(&cluster.Status.Conditions, clusterNotReadyCondition)
			return nil, nil
		}
	}
	return nil, componentutil.ErrNoOps
}
