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
	"time"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
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

	// ClusterControllerErrorDuration if there is an error in the cluster controller,
	// it will not be automatically repaired unless there is network jitter.
	// so if the error lasts more than 5s, the cluster will enter the ConditionsError phase
	// and prompt the user to repair manually according to the message.
	ClusterControllerErrorDuration = 5 * time.Second

	// ControllerErrorRequeueTime the requeue time to reconcile the error event of the cluster controller
	// which need to respond to user repair events timely.
	ControllerErrorRequeueTime = 5 * time.Second
)

// updateClusterConditions updates cluster.status condition and records event.
func (conMgr clusterConditionManager) updateStatusConditions(condition metav1.Condition) error {
	patch := client.MergeFrom(conMgr.cluster.DeepCopy())
	oldCondition := meta.FindStatusCondition(conMgr.cluster.Status.Conditions, condition.Type)
	phaseChanged := conMgr.handleConditionForClusterPhase(oldCondition, condition)
	conditionChanged := !reflect.DeepEqual(oldCondition, condition)
	if conditionChanged || phaseChanged {
		conMgr.cluster.SetStatusCondition(condition)
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
	if phaseChanged {
		// if cluster status changed, do it
		return opsutil.MarkRunningOpsRequestAnnotation(conMgr.ctx, conMgr.Client, conMgr.cluster)
	}
	return nil
}

// handleConditionForClusterPhase checks whether the condition can be repaired by cluster.
// if it cannot be repaired after 30 seconds, set the cluster status to ConditionsError
func (conMgr clusterConditionManager) handleConditionForClusterPhase(oldCondition *metav1.Condition, condition metav1.Condition) bool {
	if condition.Status == metav1.ConditionTrue {
		return false
	}

	if oldCondition == nil || oldCondition.Reason != condition.Reason {
		return false
	}

	if time.Now().Before(oldCondition.LastTransitionTime.Add(ClusterControllerErrorDuration)) {
		return false
	}
	if !util.IsFailedOrAbnormal(conMgr.cluster.Status.Phase) &&
		conMgr.cluster.Status.Phase != appsv1alpha1.ConditionsErrorPhase {
		// the condition has occurred for more than 30 seconds and cluster status is not Failed/Abnormal, do it
		conMgr.cluster.Status.Phase = appsv1alpha1.ConditionsErrorPhase
		return true
	}
	return false
}

// setProvisioningStartedCondition sets the provisioning started condition in cluster conditions.
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
func (conMgr clusterConditionManager) setPreCheckErrorCondition(err error) error {
	reason := ReasonPreCheckFailed
	if apierrors.IsNotFound(err) {
		reason = constant.ReasonNotFoundCR
	}
	condition := metav1.Condition{
		Type:    ConditionTypeProvisioningStarted,
		Status:  metav1.ConditionFalse,
		Message: err.Error(),
		Reason:  reason,
	}
	return conMgr.updateStatusConditions(condition)
}

// setUnavailableCondition sets the condition that reference CRs are unavailable.
func (conMgr clusterConditionManager) setReferenceCRUnavailableCondition(message string) error {
	condition := metav1.Condition{
		Type:    ConditionTypeProvisioningStarted,
		Status:  metav1.ConditionFalse,
		Message: message,
		Reason:  constant.ReasonRefCRUnavailable,
	}
	return conMgr.updateStatusConditions(condition)
}

// setApplyResourcesFailedCondition sets applied resources failed condition in cluster conditions.
func (conMgr clusterConditionManager) setApplyResourcesFailedCondition(err error) error {
	condition := metav1.Condition{
		Type:    ConditionTypeApplyResources,
		Status:  metav1.ConditionFalse,
		Message: err.Error(),
		Reason:  ReasonApplyResourcesFailed,
	}
	return conMgr.updateStatusConditions(condition)
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
		Message: fmt.Sprintf("pods are not ready in ComponentDefs: %v, refer to related component message in Cluster.status.components", cNameSlice),
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
func handleNotReadyConditionForCluster(cluster *appsv1alpha1.Cluster,
	recorder record.EventRecorder,
	replicasNotReadyCompNames map[string]struct{},
	notReadyCompNames map[string]struct{}) (needPatch bool, postFunc postHandler) {
	oldReplicasReadyCondition := meta.FindStatusCondition(cluster.Status.Conditions, ConditionTypeReplicasReady)
	if len(replicasNotReadyCompNames) == 0 {
		// if all replicas of cluster are ready, set ReasonAllReplicasReady to status.conditions
		readyCondition := newAllReplicasPodsReadyConditions()
		if checkConditionIsChanged(oldReplicasReadyCondition, readyCondition) {
			cluster.SetStatusCondition(readyCondition)
			needPatch = true
			postFunc = func(cluster *appsv1alpha1.Cluster) error {
				// send an event when all pods of the components are ready.
				recorder.Event(cluster, corev1.EventTypeNormal, readyCondition.Reason, readyCondition.Message)
				return nil
			}
		}
	} else {
		replicasNotReadyCond := newReplicasNotReadyCondition(replicasNotReadyCompNames)
		if checkConditionIsChanged(oldReplicasReadyCondition, replicasNotReadyCond) {
			cluster.SetStatusCondition(replicasNotReadyCond)
			needPatch = true
		}
	}

	if len(notReadyCompNames) > 0 {
		oldClusterReadyCondition := meta.FindStatusCondition(cluster.Status.Conditions, ConditionTypeReady)
		clusterNotReadyCondition := newComponentsNotReadyCondition(notReadyCompNames)
		if checkConditionIsChanged(oldClusterReadyCondition, clusterNotReadyCondition) {
			cluster.SetStatusCondition(clusterNotReadyCondition)
			needPatch = true
		}
	}
	return needPatch, postFunc
}
