/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"fmt"
	"slices"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// getProgressObjectKey gets progress object key from the client.Object.
func getProgressObjectKey(kind, name string) string {
	return fmt.Sprintf("%s/%s", kind, name)
}

// isCompletedProgressStatus checks the progress detail with final state, either Failed or Succeed.
func isCompletedProgressStatus(status opsv1alpha1.ProgressStatus) bool {
	return slices.Contains([]opsv1alpha1.ProgressStatus{opsv1alpha1.SucceedProgressStatus,
		opsv1alpha1.FailedProgressStatus}, status)
}

// setComponentStatusProgressDetail sets the corresponding progressDetail in progressDetails to newProgressDetail.
// progressDetails must be non-nil.
// 1. the startTime and endTime will be filled automatically.
// 2. if the progressDetail of the specified objectKey does not exist, it will be appended to the progressDetails.
func setComponentStatusProgressDetail(
	recorder record.EventRecorder,
	opsRequest *opsv1alpha1.OpsRequest,
	progressDetails *[]opsv1alpha1.ProgressStatusDetail,
	newProgressDetail opsv1alpha1.ProgressStatusDetail) {
	if progressDetails == nil {
		return
	}
	var existingProgressDetail *opsv1alpha1.ProgressStatusDetail
	if newProgressDetail.ObjectKey != "" {
		existingProgressDetail = findStatusProgressDetail(*progressDetails, newProgressDetail.ObjectKey)
	} else {
		existingProgressDetail = findActionProgress(*progressDetails, newProgressDetail.ActionName)
	}
	if existingProgressDetail == nil {
		updateProgressDetailTime(&newProgressDetail)
		*progressDetails = append(*progressDetails, newProgressDetail)
		sendProgressDetailEvent(recorder, opsRequest, newProgressDetail)
		return
	}
	if existingProgressDetail.Status == newProgressDetail.Status &&
		existingProgressDetail.Message == newProgressDetail.Message {
		return
	}
	// if existing progress detail is 'Failed' and new progress detail is not 'Succeed', ignores the new one.
	if existingProgressDetail.Status == opsv1alpha1.FailedProgressStatus &&
		newProgressDetail.Status != opsv1alpha1.SucceedProgressStatus {
		return
	}
	existingProgressDetail.Status = newProgressDetail.Status
	existingProgressDetail.Message = newProgressDetail.Message
	existingProgressDetail.ActionTasks = newProgressDetail.ActionTasks
	updateProgressDetailTime(existingProgressDetail)
	sendProgressDetailEvent(recorder, opsRequest, newProgressDetail)
}

// findStatusProgressDetail finds the progressDetail of the specified objectKey in progressDetails.
func findStatusProgressDetail(progressDetails []opsv1alpha1.ProgressStatusDetail,
	objectKey string) *opsv1alpha1.ProgressStatusDetail {
	for i := range progressDetails {
		if progressDetails[i].ObjectKey == objectKey {
			return &progressDetails[i]
		}
	}
	return nil
}

func findActionProgress(progressDetails []opsv1alpha1.ProgressStatusDetail, actionName string) *opsv1alpha1.ProgressStatusDetail {
	for i := range progressDetails {
		if actionName == progressDetails[i].ActionName {
			return &progressDetails[i]
		}
	}
	return nil
}

// getProgressDetailEventType gets the event type with progressDetail status.
func getProgressDetailEventType(status opsv1alpha1.ProgressStatus) string {
	if status == opsv1alpha1.FailedProgressStatus {
		return "Warning"
	}
	return "Normal"
}

// getProgressDetailEventReason gets the event reason with progressDetail status.
func getProgressDetailEventReason(status opsv1alpha1.ProgressStatus) string {
	switch status {
	case opsv1alpha1.SucceedProgressStatus:
		return "Succeed"
	case opsv1alpha1.ProcessingProgressStatus:
		return "Processing"
	case opsv1alpha1.FailedProgressStatus:
		return "Failed"
	}
	return ""
}

// sendProgressDetailEvent sends the progress detail changed events.
func sendProgressDetailEvent(recorder record.EventRecorder,
	opsRequest *opsv1alpha1.OpsRequest,
	progressDetail opsv1alpha1.ProgressStatusDetail) {
	status := progressDetail.Status
	if status == opsv1alpha1.PendingProgressStatus {
		return
	}
	recorder.Event(opsRequest, getProgressDetailEventType(status),
		getProgressDetailEventReason(status), progressDetail.Message)
}

// updateProgressDetailTime updates the progressDetail startTime or endTime according to the status.
func updateProgressDetailTime(progressDetail *opsv1alpha1.ProgressStatusDetail) {
	if progressDetail.Status == opsv1alpha1.ProcessingProgressStatus &&
		progressDetail.StartTime.IsZero() {
		progressDetail.StartTime = metav1.NewTime(time.Now())
	}
	if isCompletedProgressStatus(progressDetail.Status) &&
		progressDetail.EndTime.IsZero() {
		progressDetail.EndTime = metav1.NewTime(time.Now())
	}
}

func getProgressProcessingMessage(opsMessageKey, objectKey, componentName string) string {
	return fmt.Sprintf("Start to %s: %s in Component: %s", opsMessageKey, objectKey, componentName)
}

func handleRunningInstanceProgress(opsRes *OpsResource, pgRes *progressResource, its *workloads.InstanceSet) instanceProgress {
	expectedCount := ptr.Deref(its.Spec.Replicas, int32(1))
	result := instanceProgress{expectedCount: expectedCount}
	for i := range its.Status.InstanceStatus {
		instance := &its.Status.InstanceStatus[i]
		if instance.EffectiveDesiredState() != workloads.InstanceDesiredStateActive {
			continue
		}
		objectKey := getProgressObjectKey(constant.PodKind, instance.PodName)
		detail := opsv1alpha1.ProgressStatusDetail{ObjectKey: objectKey}
		targetApplied := instance.EffectiveCurrentState() == workloads.InstanceCurrentStatePresent && instance.UpToDate
		switch {
		case targetApplied && instance.Failed:
			detail.SetStatusAndMessage(opsv1alpha1.FailedProgressStatus,
				getProgressFailedMessage(pgRes.opsMessageKey, objectKey, pgRes.fullComponentName,
					getFailedPodMessage(opsRes.Cluster, pgRes.fullComponentName, instance.PodName)))
			result.completedCount++
		case targetApplied && instance.Ready && instance.Available:
			detail.SetStatusAndMessage(opsv1alpha1.SucceedProgressStatus,
				getProgressSucceedMessage(pgRes.opsMessageKey, objectKey, pgRes.fullComponentName))
			result.completedCount++
			result.succeededCount++
		default:
			detail.SetStatusAndMessage(opsv1alpha1.ProcessingProgressStatus,
				getProgressProcessingMessage(pgRes.opsMessageKey, objectKey, pgRes.fullComponentName))
		}
		result.details = append(result.details, detail)
	}
	result.observationsComplete = its.Status.ObservedGeneration == its.Generation &&
		!ptr.Deref(its.Spec.Stop, false) && int32(len(result.details)) == expectedCount
	return result
}

func getProgressSucceedMessage(opsMessageKey, objectKey, componentName string) string {
	return fmt.Sprintf("Successfully %s: %s in Component: %s", opsMessageKey, objectKey, componentName)
}

func getProgressFailedMessage(opsMessageKey, objectKey, componentName, podMessage string) string {
	return fmt.Sprintf("Failed to %s: %s in Component: %s, message: %s", opsMessageKey, objectKey, componentName, podMessage)
}

// getFailedPodMessage gets the failed pod message from cluster component status
func getFailedPodMessage(cluster *appsv1.Cluster, componentName, instanceName string) string {
	clusterCompStatus := cluster.Status.Components[componentName]
	return clusterCompStatus.GetObjectMessage(constant.PodKind, instanceName)
}

// handleReplicaScalingProgress projects the current allocation without predicting identities.
func handleReplicaScalingProgress(opsRes *OpsResource, resource *progressResource,
	its *workloads.InstanceSet) instanceProgress {
	result := handleRunningInstanceProgress(opsRes, resource, its)
	if resource.shards == nil {
		active, err := activeInstanceTemplates(its.Status.InstanceStatus)
		result.observationsComplete = result.observationsComplete && err == nil &&
			assignmentsMatchComponent(active, resource.clusterComponent)
	}
	target := resource.compOps.(opsv1alpha1.HorizontalScaling)
	last := opsRes.OpsRequest.Status.LastConfiguration.Components[target.ComponentName]
	explicit := sets.New[string]()
	if target.ScaleIn != nil {
		for _, name := range target.ScaleIn.OnlineInstancesToOffline {
			if slices.Contains(resource.clusterComponent.OfflineInstances, name) {
				explicit.Insert(name)
			}
		}
	}
	if target.ScaleOut != nil {
		for _, name := range target.ScaleOut.OfflineInstancesToOnline {
			if slices.Contains(last.OfflineInstances, name) {
				explicit.Insert(name)
			}
		}
	}
	for name := range explicit {
		instance := its.FindInstanceStatus(name)
		desired := workloads.InstanceDesiredStateActive
		if slices.Contains(resource.clusterComponent.OfflineInstances, name) {
			desired = workloads.InstanceDesiredStateOffline
		}
		if instance == nil || instance.EffectiveDesiredState() != desired {
			result.observationsComplete = false
		}
	}
	for i := range its.Status.InstanceStatus {
		instance := &its.Status.InstanceStatus[i]
		if instance.EffectiveDesiredState() == workloads.InstanceDesiredStateActive {
			explicit.Delete(instance.PodName)
			continue
		}
		if instance.EffectiveCurrentState() == workloads.InstanceCurrentStateAbsent && !explicit.Has(instance.PodName) {
			continue
		}
		explicit.Delete(instance.PodName)
		result.expectedCount++
		detail := opsv1alpha1.ProgressStatusDetail{ObjectKey: getProgressObjectKey(constant.PodKind, instance.PodName),
			Status: opsv1alpha1.ProcessingProgressStatus}
		if instance.EffectiveCurrentState() == workloads.InstanceCurrentStateAbsent {
			detail.Status = opsv1alpha1.SucceedProgressStatus
			result.completedCount++
			result.succeededCount++
		}
		detail.Message = fmt.Sprintf("%s instance %s in Component: %s", detail.Status, instance.PodName, resource.fullComponentName)
		result.details = append(result.details, detail)
	}
	for name := range explicit {
		result.expectedCount++
		result.observationsComplete = false
		result.details = append(result.details, opsv1alpha1.ProgressStatusDetail{
			ObjectKey: getProgressObjectKey(constant.PodKind, name), Status: opsv1alpha1.PendingProgressStatus,
			Message: fmt.Sprintf("Waiting for instance %s in Component: %s", name, resource.fullComponentName)})
	}
	return result
}

func updateProgressDetailForHScale(
	opsRes *OpsResource,
	pgRes *progressResource,
	compStatus *opsv1alpha1.OpsRequestComponentStatus,
	objectKey string, status opsv1alpha1.ProgressStatus) {
	var group string
	if pgRes.fullComponentName != "" {
		group = fmt.Sprintf("%s/%s", pgRes.fullComponentName, pgRes.opsMessageKey)
	}
	progressDetail := opsv1alpha1.ProgressStatusDetail{
		Group:     group,
		ObjectKey: objectKey,
		Status:    status,
	}
	var messagePrefix string
	switch status {
	case opsv1alpha1.SucceedProgressStatus:
		messagePrefix = "Successfully"
	case opsv1alpha1.ProcessingProgressStatus:
		messagePrefix = "Start to"
	case opsv1alpha1.PendingProgressStatus:
		messagePrefix = "wait to"
	}
	progressDetail.Message = fmt.Sprintf(`%s %s "%s" in Component: %s`,
		messagePrefix, strings.ToLower(pgRes.opsMessageKey), objectKey, pgRes.clusterComponent.Name)
	setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
		&compStatus.ProgressDetails, progressDetail)
}

// handleComponentProgressForScalingShards handles the component progressDetails when scaling the shards.
// @return expectProgressCount,
// @return completedCount
// @return error
func handleComponentProgressForScalingShards(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes *progressResource,
	compStatus *opsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
	var (
		lastCompConfiguration = opsRes.OpsRequest.Status.LastConfiguration.Components[pgRes.compOps.GetComponentName()]
		err                   error
		completedCount        int32
		updateShards          int32
	)
	updateShards = *pgRes.shards - *lastCompConfiguration.Shards
	if updateShards > 0 {
		completedCount, err = handleScaleOutForShards(reqCtx, cli, opsRes, pgRes, compStatus)
	} else if updateShards < 0 {
		updateShards *= -1
		completedCount, err = handleScaleInForShards(reqCtx, cli, opsRes, pgRes, compStatus, updateShards)
	}
	if completedCount > updateShards {
		// completedCount may exceed updated shards if components have been rebuilt by other operations.
		completedCount = updateShards
	}
	return updateShards, completedCount, err
}

func handleScaleOutForShards(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes *progressResource,
	compStatus *opsv1alpha1.OpsRequestComponentStatus) (int32, error) {
	compList, err := sharding.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, pgRes.compOps.GetComponentName())
	if err != nil {
		return 0, err
	}
	var completedCount int32
	for _, comp := range compList {
		if comp.CreationTimestamp.Before(&opsRes.OpsRequest.Status.StartTimestamp) {
			continue
		}
		objectKey := getProgressObjectKey(appsv1.ComponentKind, comp.Name)
		pgRes.opsMessageKey = "create"
		switch comp.Status.Phase {
		case appsv1.RunningComponentPhase:
			completedCount += 1
			updateProgressDetailForHScale(opsRes, pgRes, compStatus, objectKey, opsv1alpha1.SucceedProgressStatus)
		case appsv1.FailedComponentPhase:
			completedCount += 1
			updateProgressDetailForHScale(opsRes, pgRes, compStatus, objectKey, opsv1alpha1.FailedProgressStatus)
		default:
			updateProgressDetailForHScale(opsRes, pgRes, compStatus, objectKey, opsv1alpha1.ProcessingProgressStatus)
		}
	}
	return completedCount, nil
}

func handleScaleInForShards(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes *progressResource,
	compStatus *opsv1alpha1.OpsRequestComponentStatus,
	updateShards int32) (int32, error) {
	compList, err := sharding.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, pgRes.compOps.GetComponentName())
	if err != nil {
		return 0, err
	}
	var completedCount int32
	compMap := make(map[string]struct{}, len(compList))
	for _, comp := range compList {
		objKey := getProgressObjectKey(appsv1.ComponentKind, comp.Name)
		compMap[objKey] = struct{}{}
		if comp.DeletionTimestamp.IsZero() || comp.DeletionTimestamp.Before(&opsRes.OpsRequest.Status.StartTimestamp) {
			continue
		}
		pgRes.opsMessageKey = "delete"
		updateProgressDetailForHScale(opsRes, pgRes, compStatus, objKey, opsv1alpha1.ProcessingProgressStatus)
	}
	for i := range compStatus.ProgressDetails {
		progressDetail := &compStatus.ProgressDetails[i]
		if _, ok := compMap[progressDetail.ObjectKey]; !ok {
			completedCount += 1
			updateProgressDetailForHScale(opsRes, pgRes, compStatus, progressDetail.ObjectKey, opsv1alpha1.SucceedProgressStatus)
		}
	}
	if int32(len(compList)) == *pgRes.compOps.(opsv1alpha1.HorizontalScaling).Shards {
		completedCount = updateShards
	}
	return completedCount, nil
}
