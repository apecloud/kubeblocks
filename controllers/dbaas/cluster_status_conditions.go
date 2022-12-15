/*
Copyright ApeCloud Inc.

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

package dbaas

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/component/util"
	"github.com/apecloud/kubeblocks/controllers/dbaas/operations"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type clusterConditionManager struct {
	client.Client
	Recorder record.EventRecorder
	ctx      context.Context
	cluster  *dbaasv1alpha1.Cluster
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

	// ReasonPreCheckSucceed preCheck succeed for provisioning started
	ReasonPreCheckSucceed = "PreCheckSucceed"
	// ReasonPreCheckFailed preCheck failed for provisioning started
	ReasonPreCheckFailed = "PreCheckFailed"
	// ReasonApplyResourcesFailed apply resources failed to create or change the cluster
	ReasonApplyResourcesFailed = "ApplyResourcesFailed"
	// ReasonApplyResourcesSucceed apply resources succeed to create or change the cluster
	ReasonApplyResourcesSucceed = "ApplyResourcesSucceed"
	// ReasonReplicasNotReady the pods of components are not ready
	ReasonReplicasNotReady = "ReplicasNotReady"
	// ReasonAllReplicasReady the pods of components are ready
	ReasonAllReplicasReady = "AllReplicasReady"
	// ReasonComponentsNotReady the components of cluster are not ready
	ReasonComponentsNotReady = "ComponentsNotReady"
	// ReasonClusterReady the components of cluster are ready, the component phase are running
	ReasonClusterReady = "ClusterReady"
)

// updateClusterConditions update cluster.status condition and record event.
func (conMgr clusterConditionManager) updateStatusConditions(condition metav1.Condition) error {
	patch := client.MergeFrom(conMgr.cluster.DeepCopy())
	changed := conMgr.handleConditionForClusterPhase(condition)
	conMgr.cluster.SetStatusCondition(condition)
	if err := conMgr.Client.Status().Patch(conMgr.ctx, conMgr.cluster, patch); err != nil {
		return err
	}
	eventType := corev1.EventTypeWarning
	if condition.Status == metav1.ConditionTrue {
		eventType = corev1.EventTypeNormal
	}
	conMgr.Recorder.Event(conMgr.cluster, eventType, condition.Reason, condition.Message)
	if changed {
		// if cluster status changed, do it
		return operations.MarkRunningOpsRequestAnnotation(conMgr.ctx, conMgr.Client, conMgr.cluster)
	}
	return nil
}

// handleConditionForClusterPhase check whether the condition can repair by cluster.
// if it cannot be repaired after 30 seconds, modify the cluster status to ConditionsError
func (conMgr clusterConditionManager) handleConditionForClusterPhase(condition metav1.Condition) bool {
	if condition.Status == metav1.ConditionTrue {
		return false
	}
	oldCondition := meta.FindStatusCondition(conMgr.cluster.Status.Conditions, condition.Type)
	if oldCondition == nil || oldCondition.Reason != condition.Reason {
		return false
	}

	if time.Now().Before(oldCondition.LastTransitionTime.Add(30 * time.Second)) {
		return false
	}
	if !util.IsFailedOrAbnormal(conMgr.cluster.Status.Phase) {
		// the condition has occurred for more than 30 seconds and cluster status is not Failed/Abnormal, do it
		conMgr.cluster.Status.Phase = dbaasv1alpha1.ConditionsErrorPhase
		return true
	}
	return false
}

// setProvisioningStartedCondition set the provisioning started condition in cluster conditions.
func (conMgr clusterConditionManager) setProvisioningStartedCondition() error {
	condition := metav1.Condition{
		Type:    ConditionTypeProvisioningStarted,
		Status:  metav1.ConditionTrue,
		Message: fmt.Sprintf("The operator has started the provisioning of Cluster: %s", conMgr.cluster.Name),
		Reason:  ReasonPreCheckSucceed,
	}
	return conMgr.updateStatusConditions(condition)
}

// setPreCheckErrorCondition set the error condition when preCheck failed.
func (conMgr clusterConditionManager) setPreCheckErrorCondition(err error) error {
	reason := ReasonPreCheckFailed
	if apierrors.IsNotFound(err) {
		reason = intctrlutil.ReasonNotFoundCR
	}
	condition := metav1.Condition{
		Type:    ConditionTypeProvisioningStarted,
		Status:  metav1.ConditionFalse,
		Message: err.Error(),
		Reason:  reason,
	}
	return conMgr.updateStatusConditions(condition)
}

// setUnavailableCondition set the condition that reference CRs are unavailable.
func (conMgr clusterConditionManager) setReferenceCRUnavailableCondition(message string) error {
	condition := metav1.Condition{
		Type:    ConditionTypeProvisioningStarted,
		Status:  metav1.ConditionFalse,
		Message: message,
		Reason:  intctrlutil.ReasonRefCRUnavailable,
	}
	return conMgr.updateStatusConditions(condition)
}

// setApplyResourcesFailedCondition set apply resources failed condition in cluster conditions.
func (conMgr clusterConditionManager) setApplyResourcesFailedCondition(err error) error {
	condition := metav1.Condition{
		Type:    ConditionTypeApplyResources,
		Status:  metav1.ConditionFalse,
		Message: err.Error(),
		Reason:  ReasonApplyResourcesFailed,
	}
	return conMgr.updateStatusConditions(condition)
}

// newApplyResourcesCondition new a condition when apply resources succeed.
func newApplyResourcesCondition() metav1.Condition {
	return metav1.Condition{
		Type:    ConditionTypeApplyResources,
		Status:  metav1.ConditionTrue,
		Message: "Successfully applied for resources",
		Reason:  ReasonApplyResourcesSucceed,
	}
}

// newAllReplicasPodsReadyConditions new a condition when all pods of components are ready
func newAllReplicasPodsReadyConditions() metav1.Condition {
	return metav1.Condition{
		Type:    ConditionTypeReplicasReady,
		Status:  metav1.ConditionTrue,
		Message: "all pods of components are ready, waiting for the probe detection successful",
		Reason:  ReasonAllReplicasReady,
	}
}

// newReplicasNotReadyCondition new a condition when pods of components are not ready
func newReplicasNotReadyCondition(message string) metav1.Condition {
	return metav1.Condition{
		Type:    ConditionTypeReplicasReady,
		Status:  metav1.ConditionFalse,
		Message: message,
		Reason:  ReasonReplicasNotReady,
	}
}

// newClusterReadyCondition new a condition when all components of cluster are running
func newClusterReadyCondition(clusterName string) metav1.Condition {
	return metav1.Condition{
		Type:    ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Message: fmt.Sprintf("Cluster: %s is ready, current phase is Running", clusterName),
		Reason:  ReasonClusterReady,
	}
}

// newComponentsNotReadyCondition new a condition when components of cluster are not ready
func newComponentsNotReadyCondition(message string) metav1.Condition {
	return metav1.Condition{
		Type:    ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Message: message,
		Reason:  ReasonComponentsNotReady,
	}
}
