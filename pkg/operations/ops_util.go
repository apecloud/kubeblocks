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

package operations

import (
	"context"
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	opsutil "github.com/apecloud/kubeblocks/pkg/operations/util"
)

var _ error = &WaitForClusterPhaseErr{}

type WaitForClusterPhaseErr struct {
	clusterName   string
	currentPhase  appsv1.ClusterPhase
	expectedPhase []appsv1.ClusterPhase
}

func (e *WaitForClusterPhaseErr) Error() string {
	return fmt.Sprintf("wait for cluster %s to reach phase %v, current status is :%s", e.clusterName, e.expectedPhase, e.currentPhase)
}

type handleStatusProgressWithComponent func(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes *progressResource,
	compStatus *opsv1alpha1.OpsRequestComponentStatus) (expectProgressCount int32, succeedCount int32, err error)

// getClusterDefByName gets the ClusterDefinition object by the name.
func getClusterDefByName(ctx context.Context, cli client.Client, clusterDefName string) (*appsv1.ClusterDefinition, error) {
	clusterDef := &appsv1.ClusterDefinition{}
	if err := cli.Get(ctx, client.ObjectKey{Name: clusterDefName}, clusterDef); err != nil {
		return nil, err
	}
	return clusterDef, nil
}

// PatchOpsStatusWithOpsDeepCopy patches OpsRequest.status with the deepCopy opsRequest.
func PatchOpsStatusWithOpsDeepCopy(ctx context.Context,
	cli client.Client,
	opsRes *OpsResource,
	opsRequestDeepCopy *opsv1alpha1.OpsRequest,
	phase opsv1alpha1.OpsPhase,
	condition ...*metav1.Condition) error {

	opsRequest := opsRes.OpsRequest
	patch := client.MergeFrom(opsRequestDeepCopy)
	for _, v := range condition {
		if v == nil {
			continue
		}
		opsRequest.SetStatusCondition(*v)
		// emit an event
		eventType := corev1.EventTypeNormal
		if phase == opsv1alpha1.OpsFailedPhase {
			eventType = corev1.EventTypeWarning
		}
		opsRes.Recorder.Event(opsRequest, eventType, v.Reason, v.Message)
	}
	opsRequest.Status.Phase = phase
	if opsRequest.IsComplete(phase) {
		opsRequest.Status.CompletionTimestamp = metav1.Time{Time: time.Now()}
		// when OpsRequest is completed, remove it from annotation
		if err := DequeueOpsRequestInClusterAnnotation(ctx, cli, opsRes); err != nil {
			return err
		}
	}
	if phase == opsv1alpha1.OpsCreatingPhase && opsRequest.Status.StartTimestamp.IsZero() {
		opsRequest.Status.StartTimestamp = metav1.Time{Time: time.Now()}
	}
	return cli.Status().Patch(ctx, opsRequest, patch)
}

// PatchOpsStatus patches OpsRequest.status
func PatchOpsStatus(ctx context.Context,
	cli client.Client,
	opsRes *OpsResource,
	phase opsv1alpha1.OpsPhase,
	condition ...*metav1.Condition) error {
	return PatchOpsStatusWithOpsDeepCopy(ctx, cli, opsRes, opsRes.OpsRequest.DeepCopy(), phase, condition...)
}

// PatchClusterNotFound patches ClusterNotFound condition to the OpsRequest.status.conditions.
func PatchClusterNotFound(ctx context.Context, cli client.Client, opsRes *OpsResource) error {
	message := fmt.Sprintf("spec.clusterName %s is not found", opsRes.OpsRequest.Spec.GetClusterName())
	condition := opsv1alpha1.NewValidateFailedCondition(opsv1alpha1.ReasonClusterNotFound, message)
	opsPhase := opsv1alpha1.OpsFailedPhase
	if opsRes.OpsRequest.IsComplete() {
		opsPhase = opsRes.OpsRequest.Status.Phase
	}
	return PatchOpsStatus(ctx, cli, opsRes, opsPhase, condition)
}

// PatchOpsHandlerNotSupported patches OpsNotSupported condition to the OpsRequest.status.conditions.
func PatchOpsHandlerNotSupported(ctx context.Context, cli client.Client, opsRes *OpsResource) error {
	message := fmt.Sprintf("spec.type %s is not supported by operator", opsRes.OpsRequest.Spec.Type)
	condition := opsv1alpha1.NewValidateFailedCondition(opsv1alpha1.ReasonOpsTypeNotSupported, message)
	return PatchOpsStatus(ctx, cli, opsRes, opsv1alpha1.OpsFailedPhase, condition)
}

// patchValidateErrorCondition patches ValidateError condition to the OpsRequest.status.conditions.
func patchValidateErrorCondition(ctx context.Context, cli client.Client, opsRes *OpsResource, errMessage string) error {
	condition := opsv1alpha1.NewValidateFailedCondition(opsv1alpha1.ReasonValidateFailed, errMessage)
	return PatchOpsStatus(ctx, cli, opsRes, opsv1alpha1.OpsFailedPhase, condition)
}

// patchFatalFailErrorCondition patches a new failed condition to the OpsRequest.status.conditions.
func patchFatalFailErrorCondition(ctx context.Context, cli client.Client, opsRes *OpsResource, err error) error {
	condition := opsv1alpha1.NewFailedCondition(opsRes.OpsRequest, err)
	return PatchOpsStatus(ctx, cli, opsRes, opsv1alpha1.OpsFailedPhase, condition)
}

// GetOpsRecorderFromSlice gets OpsRequest recorder from slice by target cluster phase
func GetOpsRecorderFromSlice(opsRequestSlice []opsv1alpha1.OpsRecorder,
	opsRequestName string) (int, opsv1alpha1.OpsRecorder) {
	for i, v := range opsRequestSlice {
		if v.Name == opsRequestName {
			return i, v
		}
	}
	// if not found, return -1 and an empty OpsRecorder object
	return -1, opsv1alpha1.OpsRecorder{}
}

// patchOpsRequestToCreating patches OpsRequest.status.phase to Running
func patchOpsRequestToCreating(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	opsDeepCoy *opsv1alpha1.OpsRequest,
	opsHandler OpsHandler) error {
	var condition *metav1.Condition
	validatePassCondition := opsv1alpha1.NewValidatePassedCondition(opsRes.OpsRequest.Name)
	condition, err := opsHandler.ActionStartedCondition(reqCtx, cli, opsRes)
	if err != nil {
		return err
	}
	return PatchOpsStatusWithOpsDeepCopy(reqCtx.Ctx, cli, opsRes, opsDeepCoy, opsv1alpha1.OpsCreatingPhase, validatePassCondition, condition)
}

// isOpsRequestFailedPhase checks the OpsRequest phase is Failed
func isOpsRequestFailedPhase(opsRequestPhase opsv1alpha1.OpsPhase) bool {
	return opsRequestPhase == opsv1alpha1.OpsFailedPhase
}

// validateOpsWaitingPhase validates whether the current cluster phase is expected, and whether the waiting time exceeds the limit.
// only requests with `Pending` phase will be validated.
func validateOpsNeedWaitingClusterPhase(cluster *appsv1.Cluster, ops *opsv1alpha1.OpsRequest, opsBehaviour OpsBehaviour) error {
	if ops.Force() {
		return nil
	}
	// if opsRequest don't need to wait for the cluster phase
	// or opsRequest status.phase is not Pending,
	// or opsRequest will create cluster,
	// we don't validate the cluster phase.
	if len(opsBehaviour.FromClusterPhases) == 0 || ops.Status.Phase != opsv1alpha1.OpsPendingPhase || opsBehaviour.IsClusterCreation {
		return nil
	}
	if slices.Contains(opsBehaviour.FromClusterPhases, cluster.Status.Phase) {
		return nil
	}

	opsRequestSlice, err := opsutil.GetOpsRequestSliceFromCluster(cluster)
	if err != nil {
		return intctrlutil.NewFatalError(err.Error())
	}
	// skip the preConditionDeadline check if the ops is in queue.
	index, opsRecorder := GetOpsRecorderFromSlice(opsRequestSlice, ops.Name)
	if index != -1 && opsRecorder.InQueue {
		return nil
	}

	// check if entry-condition is met
	// if the cluster is not in the expected phase, we should wait for it for up to TTLSecondsBeforeAbort seconds.
	if !needWaitPreConditionDeadline(ops) {
		return nil
	}

	return &WaitForClusterPhaseErr{
		clusterName:   cluster.Name,
		currentPhase:  cluster.Status.Phase,
		expectedPhase: opsBehaviour.FromClusterPhases,
	}
}

func preConditionDeadlineSecondsIsSet(ops *opsv1alpha1.OpsRequest) bool {
	return ops.Spec.PreConditionDeadlineSeconds != nil && *ops.Spec.PreConditionDeadlineSeconds != 0
}

func needWaitPreConditionDeadline(ops *opsv1alpha1.OpsRequest) bool {
	if !preConditionDeadlineSecondsIsSet(ops) {
		return false
	}
	baseTime := ops.GetCreationTimestamp()
	if queueEndTimeStr, ok := ops.Annotations[constant.QueueEndTimeAnnotationKey]; ok {
		queueEndTime, _ := time.Parse(time.RFC3339, queueEndTimeStr)
		if !queueEndTime.IsZero() {
			baseTime = metav1.Time{Time: queueEndTime}
		}
	}
	return time.Now().Before(baseTime.Add(time.Duration(*ops.Spec.PreConditionDeadlineSeconds) * time.Second))
}

func abortEarlierOpsRequestWithSameKind(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	sameKinds []opsv1alpha1.OpsType,
	matchAbortCondition func(earlierOps *opsv1alpha1.OpsRequest) (bool, error)) error {
	opsRequestSlice, err := opsutil.GetOpsRequestSliceFromCluster(opsRes.Cluster)
	if err != nil {
		return err
	}
	// get the running opsRequest before this opsRequest to running.
	var earlierRunningOpsSlice []opsv1alpha1.OpsRecorder
	for i := range opsRequestSlice {
		if !slices.Contains(sameKinds, opsRequestSlice[i].Type) {
			continue
		}
		if opsRequestSlice[i].Name == opsRes.OpsRequest.Name {
			break
		}
		earlierRunningOpsSlice = append(earlierRunningOpsSlice, opsRequestSlice[i])
	}
	if len(earlierRunningOpsSlice) == 0 {
		return nil
	}
	for _, v := range earlierRunningOpsSlice {
		earlierOps := &opsv1alpha1.OpsRequest{}
		err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: v.Name, Namespace: opsRes.OpsRequest.Namespace}, earlierOps)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		if slices.Contains([]opsv1alpha1.OpsPhase{opsv1alpha1.OpsSucceedPhase, opsv1alpha1.OpsFailedPhase,
			opsv1alpha1.OpsCancelledPhase}, earlierOps.Status.Phase) {
			continue
		}
		needAborted, err := matchAbortCondition(earlierOps)
		if err != nil {
			return err
		}
		if needAborted {
			// abort the opsRequest that matches the abort condition.
			patch := client.MergeFrom(earlierOps.DeepCopy())
			earlierOps.Status.Phase = opsv1alpha1.OpsAbortedPhase
			abortedCondition := opsv1alpha1.NewAbortedCondition(fmt.Sprintf(`Aborted as a result of the latest opsRequest "%s" being overridden`, earlierOps.Name))
			earlierOps.SetStatusCondition(*abortedCondition)
			earlierOps.Status.CompletionTimestamp = metav1.Time{Time: time.Now()}
			if err = cli.Status().Patch(reqCtx.Ctx, earlierOps, patch); err != nil {
				return err
			}
			opsRes.Recorder.Event(earlierOps, corev1.EventTypeNormal, abortedCondition.Type, abortedCondition.Message)
			index, _ := GetOpsRecorderFromSlice(opsRequestSlice, earlierOps.Name)
			if index != -1 {
				opsRequestSlice = slices.Delete(opsRequestSlice, index, index+1)
			}
		}
	}
	return opsutil.UpdateClusterOpsAnnotations(reqCtx.Ctx, cli, opsRes.Cluster, opsRequestSlice)
}

func updateHAConfigIfNecessary(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRequest *opsv1alpha1.OpsRequest, switchBoolStr string) error {
	haConfigName, ok := opsRequest.Annotations[constant.DisableHAAnnotationKey]
	if !ok {
		return nil
	}
	haConfig := &corev1.ConfigMap{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: haConfigName, Namespace: opsRequest.Namespace}, haConfig); err != nil {
		return err
	}
	val, ok := haConfig.Annotations["enable"]
	if !ok || val == switchBoolStr {
		return nil
	}
	haConfig.Annotations["enable"] = switchBoolStr
	return cli.Update(reqCtx.Ctx, haConfig)
}

func getComponentSpecOrShardingTemplate(cluster *appsv1.Cluster, componentName string) *appsv1.ClusterComponentSpec {
	for _, v := range cluster.Spec.ComponentSpecs {
		if v.Name == componentName {
			return &v
		}
	}
	for _, v := range cluster.Spec.Shardings {
		if v.Name == componentName {
			return &v.Template
		}
	}
	return nil
}
