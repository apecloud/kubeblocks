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

package operations

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (expectProgressCount int32, succeedCount int32, err error)

type handleReconfigureOpsStatus func(cmStatus *appsv1alpha1.ConfigurationItemStatus) error

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
	opsRequestDeepCopy *appsv1alpha1.OpsRequest,
	phase appsv1alpha1.OpsPhase,
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
		if phase == appsv1alpha1.OpsFailedPhase {
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
	if phase == appsv1alpha1.OpsCreatingPhase && opsRequest.Status.StartTimestamp.IsZero() {
		opsRequest.Status.StartTimestamp = metav1.Time{Time: time.Now()}
	}
	return cli.Status().Patch(ctx, opsRequest, patch)
}

// PatchOpsStatus patches OpsRequest.status
func PatchOpsStatus(ctx context.Context,
	cli client.Client,
	opsRes *OpsResource,
	phase appsv1alpha1.OpsPhase,
	condition ...*metav1.Condition) error {
	return PatchOpsStatusWithOpsDeepCopy(ctx, cli, opsRes, opsRes.OpsRequest.DeepCopy(), phase, condition...)
}

// PatchClusterNotFound patches ClusterNotFound condition to the OpsRequest.status.conditions.
func PatchClusterNotFound(ctx context.Context, cli client.Client, opsRes *OpsResource) error {
	message := fmt.Sprintf("spec.clusterRef %s is not found", opsRes.OpsRequest.Spec.GetClusterName())
	condition := appsv1alpha1.NewValidateFailedCondition(appsv1alpha1.ReasonClusterNotFound, message)
	return PatchOpsStatus(ctx, cli, opsRes, appsv1alpha1.OpsFailedPhase, condition)
}

// PatchOpsHandlerNotSupported patches OpsNotSupported condition to the OpsRequest.status.conditions.
func PatchOpsHandlerNotSupported(ctx context.Context, cli client.Client, opsRes *OpsResource) error {
	message := fmt.Sprintf("spec.type %s is not supported by operator", opsRes.OpsRequest.Spec.Type)
	condition := appsv1alpha1.NewValidateFailedCondition(appsv1alpha1.ReasonOpsTypeNotSupported, message)
	return PatchOpsStatus(ctx, cli, opsRes, appsv1alpha1.OpsFailedPhase, condition)
}

// patchValidateErrorCondition patches ValidateError condition to the OpsRequest.status.conditions.
func patchValidateErrorCondition(ctx context.Context, cli client.Client, opsRes *OpsResource, errMessage string) error {
	condition := appsv1alpha1.NewValidateFailedCondition(appsv1alpha1.ReasonValidateFailed, errMessage)
	return PatchOpsStatus(ctx, cli, opsRes, appsv1alpha1.OpsFailedPhase, condition)
}

// patchFatalFailErrorCondition patches a new failed condition to the OpsRequest.status.conditions.
func patchFatalFailErrorCondition(ctx context.Context, cli client.Client, opsRes *OpsResource, err error) error {
	condition := appsv1alpha1.NewFailedCondition(opsRes.OpsRequest, err)
	return PatchOpsStatus(ctx, cli, opsRes, appsv1alpha1.OpsFailedPhase, condition)
}

// GetOpsRecorderFromSlice gets OpsRequest recorder from slice by target cluster phase
func GetOpsRecorderFromSlice(opsRequestSlice []appsv1alpha1.OpsRecorder,
	opsRequestName string) (int, appsv1alpha1.OpsRecorder) {
	for i, v := range opsRequestSlice {
		if v.Name == opsRequestName {
			return i, v
		}
	}
	// if not found, return -1 and an empty OpsRecorder object
	return -1, appsv1alpha1.OpsRecorder{}
}

// patchOpsRequestToCreating patches OpsRequest.status.phase to Running
func patchOpsRequestToCreating(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	opsDeepCoy *appsv1alpha1.OpsRequest,
	opsHandler OpsHandler) error {
	var condition *metav1.Condition
	validatePassCondition := appsv1alpha1.NewValidatePassedCondition(opsRes.OpsRequest.Name)
	condition, err := opsHandler.ActionStartedCondition(reqCtx, cli, opsRes)
	if err != nil {
		return err
	}
	return PatchOpsStatusWithOpsDeepCopy(reqCtx.Ctx, cli, opsRes, opsDeepCoy, appsv1alpha1.OpsCreatingPhase, validatePassCondition, condition)
}

// isOpsRequestFailedPhase checks the OpsRequest phase is Failed
func isOpsRequestFailedPhase(opsRequestPhase appsv1alpha1.OpsPhase) bool {
	return opsRequestPhase == appsv1alpha1.OpsFailedPhase
}

// patchReconfigureOpsStatus when Reconfigure is running, we should update status to OpsRequest.Status.ConfigurationStatus.
//
// NOTES:
// opsStatus describes status of OpsRequest.
// reconfiguringStatus describes status of reconfiguring operation, which contains multiple configuration templates.
// cmStatus describes status of configmap, it is uniquely associated with a configuration template, which contains multiple keys, each key is name of a configuration file.
// execStatus describes the result of the execution of the state machine, which is designed to solve how to conduct the reconfiguring operation, such as whether to restart, how to send a signal to the process.
func updateReconfigureStatusByCM(reconfiguringStatus *appsv1alpha1.ReconfiguringStatus, tplName string,
	handleReconfigureStatus handleReconfigureOpsStatus) error {
	for i, cmStatus := range reconfiguringStatus.ConfigurationStatus {
		if cmStatus.Name == tplName {
			// update cmStatus
			return handleReconfigureStatus(&reconfiguringStatus.ConfigurationStatus[i])
		}
	}
	cmCount := len(reconfiguringStatus.ConfigurationStatus)
	reconfiguringStatus.ConfigurationStatus = append(reconfiguringStatus.ConfigurationStatus, appsv1alpha1.ConfigurationItemStatus{
		Name:          tplName,
		Status:        appsv1alpha1.ReasonReconfigurePersisting,
		SucceedCount:  core.NotStarted,
		ExpectedCount: core.Unconfirmed,
	})
	cmStatus := &reconfiguringStatus.ConfigurationStatus[cmCount]
	return handleReconfigureStatus(cmStatus)
}

// validateOpsWaitingPhase validates whether the current cluster phase is expected, and whether the waiting time exceeds the limit.
// only requests with `Pending` phase will be validated.
func validateOpsWaitingPhase(cluster *appsv1.Cluster, ops *appsv1alpha1.OpsRequest, opsBehaviour OpsBehaviour) error {
	if ops.Force() {
		return nil
	}
	// if opsRequest don't need to wait for the cluster phase
	// or opsRequest status.phase is not Pending,
	// or opsRequest will create cluster,
	// we don't validate the cluster phase.
	if len(opsBehaviour.FromClusterPhases) == 0 || ops.Status.Phase != appsv1alpha1.OpsPendingPhase || opsBehaviour.IsClusterCreation {
		return nil
	}
	if slices.Contains(opsBehaviour.FromClusterPhases, cluster.Status.Phase) {
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

func needWaitPreConditionDeadline(ops *appsv1alpha1.OpsRequest) bool {
	if ops.Spec.PreConditionDeadlineSeconds == nil {
		return false
	}
	return time.Now().Before(ops.GetCreationTimestamp().Add(time.Duration(*ops.Spec.PreConditionDeadlineSeconds) * time.Second))
}

func abortEarlierOpsRequestWithSameKind(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	sameKinds []appsv1alpha1.OpsType,
	matchAbortCondition func(earlierOps *appsv1alpha1.OpsRequest) (bool, error)) error {
	opsRequestSlice, err := opsutil.GetOpsRequestSliceFromCluster(opsRes.Cluster)
	if err != nil {
		return err
	}
	// get the running opsRequest before this opsRequest to running.
	var earlierRunningOpsSlice []appsv1alpha1.OpsRecorder
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
		earlierOps := &appsv1alpha1.OpsRequest{}
		err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: v.Name, Namespace: opsRes.OpsRequest.Namespace}, earlierOps)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		if slices.Contains([]appsv1alpha1.OpsPhase{appsv1alpha1.OpsSucceedPhase, appsv1alpha1.OpsFailedPhase,
			appsv1alpha1.OpsCancelledPhase}, earlierOps.Status.Phase) {
			continue
		}
		needAborted, err := matchAbortCondition(earlierOps)
		if err != nil {
			return err
		}
		if needAborted {
			// abort the opsRequest that matches the abort condition.
			patch := client.MergeFrom(earlierOps.DeepCopy())
			earlierOps.Status.Phase = appsv1alpha1.OpsAbortedPhase
			abortedCondition := appsv1alpha1.NewAbortedCondition(fmt.Sprintf(`Aborted as a result of the latest opsRequest "%s" being overridden`, earlierOps.Name))
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

func updateHAConfigIfNecessary(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRequest *appsv1alpha1.OpsRequest, switchBoolStr string) error {
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
	for _, v := range cluster.Spec.ShardingSpecs {
		if v.Name == componentName {
			return &v.Template
		}
	}
	return nil
}
