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
	"fmt"
	"reflect"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlcomp "github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// getProgressObjectKey gets progress object key from the client.Object.
func getProgressObjectKey(kind, name string) string {
	return fmt.Sprintf("%s/%s", kind, name)
}

// isCompletedProgressStatus checks the progress detail with final state, either Failed or Succeed.
func isCompletedProgressStatus(status appsv1alpha1.ProgressStatus) bool {
	return slices.Contains([]appsv1alpha1.ProgressStatus{appsv1alpha1.SucceedProgressStatus,
		appsv1alpha1.FailedProgressStatus}, status)
}

// setComponentStatusProgressDetail sets the corresponding progressDetail in progressDetails to newProgressDetail.
// progressDetails must be non-nil.
// 1. the startTime and endTime will be filled automatically.
// 2. if the progressDetail of the specified objectKey does not exist, it will be appended to the progressDetails.
func setComponentStatusProgressDetail(
	recorder record.EventRecorder,
	opsRequest *appsv1alpha1.OpsRequest,
	progressDetails *[]appsv1alpha1.ProgressStatusDetail,
	newProgressDetail appsv1alpha1.ProgressStatusDetail) {
	if progressDetails == nil {
		return
	}
	var existingProgressDetail *appsv1alpha1.ProgressStatusDetail
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
	if existingProgressDetail.Status == appsv1alpha1.FailedProgressStatus &&
		newProgressDetail.Status != appsv1alpha1.SucceedProgressStatus {
		return
	}
	existingProgressDetail.Status = newProgressDetail.Status
	existingProgressDetail.Message = newProgressDetail.Message
	existingProgressDetail.ActionTasks = newProgressDetail.ActionTasks
	updateProgressDetailTime(existingProgressDetail)
	sendProgressDetailEvent(recorder, opsRequest, newProgressDetail)
}

// findStatusProgressDetail finds the progressDetail of the specified objectKey in progressDetails.
func findStatusProgressDetail(progressDetails []appsv1alpha1.ProgressStatusDetail,
	objectKey string) *appsv1alpha1.ProgressStatusDetail {
	for i := range progressDetails {
		if progressDetails[i].ObjectKey == objectKey {
			return &progressDetails[i]
		}
	}
	return nil
}

func findActionProgress(progressDetails []appsv1alpha1.ProgressStatusDetail, actionName string) *appsv1alpha1.ProgressStatusDetail {
	for i := range progressDetails {
		if actionName == progressDetails[i].ActionName {
			return &progressDetails[i]
		}
	}
	return nil
}

// getProgressDetailEventType gets the event type with progressDetail status.
func getProgressDetailEventType(status appsv1alpha1.ProgressStatus) string {
	if status == appsv1alpha1.FailedProgressStatus {
		return corev1.EventTypeWarning
	}
	return corev1.EventTypeNormal
}

// getProgressDetailEventReason gets the event reason with progressDetail status.
func getProgressDetailEventReason(status appsv1alpha1.ProgressStatus) string {
	switch status {
	case appsv1alpha1.SucceedProgressStatus:
		return "Succeed"
	case appsv1alpha1.ProcessingProgressStatus:
		return "Processing"
	case appsv1alpha1.FailedProgressStatus:
		return "Failed"
	}
	return ""
}

// sendProgressDetailEvent sends the progress detail changed events.
func sendProgressDetailEvent(recorder record.EventRecorder,
	opsRequest *appsv1alpha1.OpsRequest,
	progressDetail appsv1alpha1.ProgressStatusDetail) {
	status := progressDetail.Status
	if status == appsv1alpha1.PendingProgressStatus {
		return
	}
	recorder.Event(opsRequest, getProgressDetailEventType(status),
		getProgressDetailEventReason(status), progressDetail.Message)
}

// updateProgressDetailTime updates the progressDetail startTime or endTime according to the status.
func updateProgressDetailTime(progressDetail *appsv1alpha1.ProgressStatusDetail) {
	if progressDetail.Status == appsv1alpha1.ProcessingProgressStatus &&
		progressDetail.StartTime.IsZero() {
		progressDetail.StartTime = metav1.NewTime(time.Now())
	}
	if isCompletedProgressStatus(progressDetail.Status) &&
		progressDetail.EndTime.IsZero() {
		progressDetail.EndTime = metav1.NewTime(time.Now())
	}
}

// handleComponentStatusProgress handles the component status progressDetails.
// if all the pods of the component are affected, use this function to reconcile the progressDetails.
func handleComponentStatusProgress(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes *progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus,
	podApplyOps func(*corev1.Pod, ComponentOpsInteface, metav1.Time, string) bool) (int32, int32, error) {
	var (
		pods             []*corev1.Pod
		clusterComponent = pgRes.clusterComponent
		completedCount   int32
		err              error
	)
	if clusterComponent == nil {
		return 0, 0, nil
	}
	if pods, err = intctrlcomp.ListOwnedPods(reqCtx.Ctx, cli, opsRes.Cluster.Namespace, opsRes.Cluster.Name, pgRes.fullComponentName); err != nil {
		return 0, completedCount, err
	}
	expectReplicas := clusterComponent.Replicas
	if len(pgRes.updatedPodSet) > 0 {
		//  pods need to updated during this operation.
		var updatedPods []*corev1.Pod
		for i := range pods {
			if _, ok := pgRes.updatedPodSet[pods[i].Name]; ok {
				updatedPods = append(updatedPods, pods[i])
			}
		}
		pods = updatedPods
		expectReplicas = int32(len(pgRes.updatedPodSet))
	}
	minReadySeconds, err := intctrlcomp.GetMinReadySeconds(reqCtx.Ctx, cli, *opsRes.Cluster, pgRes.clusterComponent.Name)
	if err != nil {
		return expectReplicas, completedCount, err
	}
	if opsRes.OpsRequest.Status.Phase == appsv1alpha1.OpsCancellingPhase {
		completedCount = handleCancelProgressForPodsRollingUpdate(opsRes, pods, pgRes, compStatus, minReadySeconds)
	} else {
		completedCount = handleProgressForPodsRollingUpdate(opsRes, pods, pgRes, compStatus, minReadySeconds, podApplyOps)
	}
	if opsRes.OpsRequest.Status.Phase == appsv1alpha1.OpsCancellingPhase {
		// only rollback the actual re-created pod during cancelling.
		expectReplicas = int32(len(compStatus.ProgressDetails))
	}
	return expectReplicas, completedCount, err
}

// handleProgressForPodsRollingUpdate handles the progress of pods during rolling update.
func handleProgressForPodsRollingUpdate(
	opsRes *OpsResource,
	pods []*corev1.Pod,
	pgRes *progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus,
	minReadySeconds int32,
	podApplyOps func(*corev1.Pod, ComponentOpsInteface, metav1.Time, string) bool) int32 {
	opsRequest := opsRes.OpsRequest
	opsStartTime := opsRequest.Status.StartTimestamp
	var completedCount int32
	for _, v := range pods {
		objectKey := getProgressObjectKey(constant.PodKind, v.Name)
		progressDetail := appsv1alpha1.ProgressStatusDetail{ObjectKey: objectKey}
		if podProcessedSuccessful(pgRes, opsStartTime, v, minReadySeconds, podApplyOps) {
			completedCount += 1
			handleSucceedProgressDetail(opsRes, pgRes, compStatus, progressDetail)
			continue
		}
		if podIsPendingDuringOperation(opsStartTime, v) {
			handlePendingProgressDetail(opsRes, compStatus, progressDetail)
			continue
		}
		completedCount += handleFailedOrProcessingProgressDetail(opsRes, pgRes, compStatus, progressDetail, v)
	}
	return completedCount
}

// handleCancelProgressForPodsRollingUpdate handles the cancel progress of pods during rolling update.
func handleCancelProgressForPodsRollingUpdate(
	opsRes *OpsResource,
	pods []*corev1.Pod,
	pgRes *progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus,
	minReadySeconds int32) int32 {
	var newProgressDetails []appsv1alpha1.ProgressStatusDetail
	for _, v := range compStatus.ProgressDetails {
		// remove the pending progressDetail
		if v.Status != appsv1alpha1.PendingProgressStatus {
			newProgressDetails = append(newProgressDetails, v)
		}
	}
	compStatus.ProgressDetails = newProgressDetails
	opsCancelTime := opsRes.OpsRequest.Status.CancelTimestamp
	pgRes.opsMessageKey = fmt.Sprintf("%s with rollback", pgRes.opsMessageKey)
	var completedCount int32
	for _, pod := range pods {
		objectKey := getProgressObjectKey(constant.PodKind, pod.Name)
		progressDetail := appsv1alpha1.ProgressStatusDetail{ObjectKey: objectKey}
		if !pod.CreationTimestamp.Before(&opsCancelTime) &&
			podIsAvailable(pgRes, pod, minReadySeconds) {
			completedCount += 1
			handleSucceedProgressDetail(opsRes, pgRes, compStatus, progressDetail)
			continue
		}
		if podIsPendingDuringOperation(opsCancelTime, pod) {
			continue
		}
		completedCount += handleFailedOrProcessingProgressDetail(opsRes, pgRes, compStatus, progressDetail, pod)
	}
	return completedCount
}

func podIsAvailable(pgRes *progressResource, pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	var needCheckRole bool
	if pgRes.componentDef != nil {
		needCheckRole = len(pgRes.componentDef.Spec.Roles) > 0
	} else if pgRes.clusterDef != nil {
		// TODO: get the componentDefinition by clusterDefinition after v0.9
		compDef := pgRes.clusterDef.GetComponentDefByName(pgRes.clusterComponent.ComponentDefRef)
		needCheckRole = compDef != nil && (compDef.WorkloadType == appsv1alpha1.Replication || compDef.WorkloadType == appsv1alpha1.Consensus)
	}
	if needCheckRole {
		return intctrlutil.PodIsReadyWithLabel(*pod)
	}
	return podutils.IsPodAvailable(pod, minReadySeconds, metav1.Time{Time: time.Now()})
}

// handlePendingProgressDetail handles the pending progressDetail and sets it to progressDetails.
func handlePendingProgressDetail(opsRes *OpsResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus,
	progressDetail appsv1alpha1.ProgressStatusDetail,
) {
	progressDetail.Status = appsv1alpha1.PendingProgressStatus
	setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
		&compStatus.ProgressDetails, progressDetail)
}

// handleSucceedProgressDetail handles the successful progressDetail and sets it to progressDetails.
func handleSucceedProgressDetail(opsRes *OpsResource,
	pgRes *progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus,
	progressDetail appsv1alpha1.ProgressStatusDetail,
) {
	progressDetail.SetStatusAndMessage(appsv1alpha1.SucceedProgressStatus,
		getProgressSucceedMessage(pgRes.opsMessageKey, progressDetail.ObjectKey, pgRes.clusterComponent.Name))
	setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
		&compStatus.ProgressDetails, progressDetail)
}

// handleFailedOrProcessingProgressDetail handles failed or processing progressDetail and sets it to progressDetails.
func handleFailedOrProcessingProgressDetail(opsRes *OpsResource,
	pgRes *progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus,
	progressDetail appsv1alpha1.ProgressStatusDetail,
	pod *corev1.Pod) (completedCount int32) {
	componentName := pgRes.clusterComponent.Name
	opsStartTime := opsRes.OpsRequest.Status.StartTimestamp
	if podIsFailedDuringOperation(opsStartTime, pod, compStatus.Phase) {
		podMessage := getFailedPodMessage(opsRes.Cluster, componentName, pod)
		// if the pod is not failed, return
		if len(podMessage) == 0 {
			return
		}
		message := getProgressFailedMessage(pgRes.opsMessageKey, progressDetail.ObjectKey, componentName, podMessage)
		progressDetail.SetStatusAndMessage(appsv1alpha1.FailedProgressStatus, message)
		completedCount = 1
	} else {
		progressDetail.SetStatusAndMessage(appsv1alpha1.ProcessingProgressStatus,
			getProgressProcessingMessage(pgRes.opsMessageKey, progressDetail.ObjectKey, componentName))
	}
	setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
		&compStatus.ProgressDetails, progressDetail)
	return completedCount
}

// podIsPendingDuringOperation checks if pod is pending during the component's operation.
func podIsPendingDuringOperation(opsStartTime metav1.Time, pod *corev1.Pod) bool {
	return pod.CreationTimestamp.Before(&opsStartTime) && pod.DeletionTimestamp.IsZero()
}

// podIsFailedDuringOperation checks if pod is failed during operation.
func podIsFailedDuringOperation(
	opsStartTime metav1.Time,
	pod *corev1.Pod,
	componentPhase appsv1alpha1.ClusterComponentPhase) bool {
	if !isFailedOrAbnormal(componentPhase) {
		return false
	}
	// When the component is running and the pod has been created after opsStartTime,
	// but it does not meet the success condition, it indicates that the changes made
	// to the operations have been overwritten, resulting in a failed status.
	return !pod.CreationTimestamp.Before(&opsStartTime) && componentPhase == appsv1alpha1.RunningClusterCompPhase
}

// podProcessedSuccessful checks if the pod has been processed successfully:
// 1. the pod is recreated after OpsRequest.status.startTime and pod is available.
// 2. the component is running and pod is available.
func podProcessedSuccessful(pgRes *progressResource,
	opsStartTime metav1.Time,
	pod *corev1.Pod,
	minReadySeconds int32,
	podApplyOps func(*corev1.Pod, ComponentOpsInteface, metav1.Time, string) bool) bool {
	if !pod.DeletionTimestamp.IsZero() {
		return false
	}
	if !podIsAvailable(pgRes, pod, minReadySeconds) {
		return false
	}
	return podApplyOps(pod, pgRes.compOps, opsStartTime, pgRes.updatedPodSet[pod.Name])
}

func getProgressProcessingMessage(opsMessageKey, objectKey, componentName string) string {
	return fmt.Sprintf("Start to %s: %s in Component: %s", opsMessageKey, objectKey, componentName)
}

func getProgressSucceedMessage(opsMessageKey, objectKey, componentName string) string {
	return fmt.Sprintf("Successfully %s: %s in Component: %s", opsMessageKey, objectKey, componentName)
}

func getProgressFailedMessage(opsMessageKey, objectKey, componentName, podMessage string) string {
	return fmt.Sprintf("Failed to %s: %s in Component: %s, message: %s", opsMessageKey, objectKey, componentName, podMessage)
}

// getFailedPodMessage gets the failed pod message from cluster component status
func getFailedPodMessage(cluster *appsv1alpha1.Cluster, componentName string, pod *corev1.Pod) string {
	clusterCompStatus := cluster.Status.Components[componentName]
	return clusterCompStatus.GetObjectMessage(constant.PodKind, pod.Name)
}

// handleComponentProgressDetails handles the component progressDetails when scale the replicas.
// @return expectProgressCount,
// @return completedCount
// @return error
func handleComponentProgressForScalingReplicas(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes *progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
	var (
		pods             []*corev1.Pod
		clusterComponent = pgRes.clusterComponent
		opsRequest       = opsRes.OpsRequest
		err              error
		updatedPodCount  = int32(len(pgRes.createdPodSet) + len(pgRes.deletedPodSet))
		completedCount   int32
	)
	if clusterComponent == nil {
		return 0, 0, nil
	}
	lastComponentReplicas := opsRequest.Status.LastConfiguration.Components[pgRes.compOps.GetComponentName()].Replicas
	if lastComponentReplicas == nil {
		return 0, 0, nil
	}
	// if no any pod needs to create or delete, return
	if updatedPodCount == 0 {
		return 0, 0, nil
	}
	if pods, err = intctrlcomp.ListOwnedPods(reqCtx.Ctx, cli, opsRes.Cluster.Namespace, opsRes.Cluster.Name, pgRes.fullComponentName); err != nil {
		return 0, 0, err
	}
	if len(pgRes.createdPodSet) > 0 {
		scaleOutCompletedCount, scaleOutErr := handleScaleOutProgress(reqCtx, cli, opsRes, pgRes, pods, compStatus)
		if scaleOutErr != nil {
			err = scaleOutErr
		}
		completedCount += scaleOutCompletedCount
	}
	if len(pgRes.deletedPodSet) > 0 {
		scaleDownCompletedCount, scaleDownErr := handleScaleDownProgress(opsRes, pgRes, pods, compStatus)
		if scaleDownErr != nil {
			err = fmt.Errorf(scaleDownErr.Error(), err)
		}
		completedCount += scaleDownCompletedCount
	}
	return updatedPodCount, completedCount, err
}

// handleScaleOutProgress handles the progressDetails of scaled out replicas.
func handleScaleOutProgress(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes *progressResource,
	pods []*corev1.Pod,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, error) {
	var componentName = pgRes.clusterComponent.Name
	minReadySeconds, err := intctrlcomp.GetMinReadySeconds(reqCtx.Ctx, cli, *opsRes.Cluster, componentName)
	if err != nil {
		return 0, err
	}
	podMap := map[string]*corev1.Pod{}
	for _, v := range pods {
		podMap[v.Name] = v
	}
	var completedCount int32
	for _, v := range pods {
		if _, ok := pgRes.createdPodSet[v.Name]; !ok {
			continue
		}
		objectKey := getProgressObjectKey(constant.PodKind, v.Name)
		progressDetail := appsv1alpha1.ProgressStatusDetail{Group: fmt.Sprintf("%s/Create", pgRes.fullComponentName), ObjectKey: objectKey}
		pgRes.opsMessageKey = "create"
		if podIsAvailable(pgRes, v, minReadySeconds) {
			completedCount += 1
			handleSucceedProgressDetail(opsRes, pgRes, compStatus, progressDetail)
			continue
		}
		completedCount += handleFailedOrProcessingProgressDetail(opsRes, pgRes, compStatus, progressDetail, v)
	}
	return completedCount, nil
}

// handleScaleDownProgress handles the progressDetails of scaled down replicas.
func handleScaleDownProgress(
	opsRes *OpsResource,
	pgRes *progressResource,
	pods []*corev1.Pod,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (completedCount int32, err error) {
	podMap := map[string]*corev1.Pod{}
	for i := range pods {
		objectKey := getProgressObjectKey(constant.PodKind, pods[i].Name)
		podMap[objectKey] = pods[i]
	}
	updateProgressDetail := func(objectKey string, status appsv1alpha1.ProgressStatus) {
		progressDetail := appsv1alpha1.ProgressStatusDetail{
			Group:     fmt.Sprintf("%s/Delete", pgRes.fullComponentName),
			ObjectKey: objectKey,
			Status:    status,
		}
		var messagePrefix string
		switch status {
		case appsv1alpha1.SucceedProgressStatus:
			completedCount += 1
			messagePrefix = "Successfully"
		case appsv1alpha1.ProcessingProgressStatus:
			messagePrefix = "Start to"
		case appsv1alpha1.PendingProgressStatus:
			messagePrefix = "wait to"
		}
		progressDetail.Message = fmt.Sprintf("%s delete pod: %s in Component: %s", messagePrefix, objectKey, pgRes.clusterComponent.Name)
		setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
			&compStatus.ProgressDetails, progressDetail)
	}
	for k := range pgRes.deletedPodSet {
		objectKey := getProgressObjectKey(constant.PodKind, k)
		pod, ok := podMap[objectKey]
		if !ok {
			updateProgressDetail(objectKey, appsv1alpha1.SucceedProgressStatus)
			continue
		}
		if !pod.DeletionTimestamp.IsZero() {
			updateProgressDetail(objectKey, appsv1alpha1.ProcessingProgressStatus)
			continue
		}
		updateProgressDetail(objectKey, appsv1alpha1.PendingProgressStatus)
	}
	return completedCount, nil
}

func syncProgressToOpsRequest(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	oldOpsRequest *appsv1alpha1.OpsRequest,
	completedCount, expectCount int) error {
	// sync progress
	opsRes.OpsRequest.Status.Progress = fmt.Sprintf("%d/%d", completedCount, expectCount)
	if !reflect.DeepEqual(opsRes.OpsRequest.Status, oldOpsRequest.Status) {
		return cli.Status().Patch(reqCtx.Ctx, opsRes.OpsRequest, client.MergeFrom(oldOpsRequest))
	}
	return nil
}

func getPodSetForComponent(
	instances []appsv1alpha1.InstanceTemplate,
	offlineInstances []string,
	clusterName,
	fullCompName string,
	compReplicas int32) map[string]string {
	workloadName := constant.GenerateWorkloadNamePattern(clusterName, fullCompName)
	podSet := map[string]string{}
	insTplReplicasCnt := int32(0)
	// get the pods these are created by insTemplate
	for _, ins := range instances {
		insReplicas := intctrlutil.TemplateReplicas(ins)
		insTplReplicasCnt += insReplicas
		templatePodNames := instanceset.GenerateInstanceNamesFromTemplate(workloadName, ins.Name, insReplicas, offlineInstances)
		for _, podName := range templatePodNames {
			podSet[podName] = ins.Name
		}
	}
	// get the pods these are created without any insTemplate
	if insTplReplicasCnt < compReplicas {
		podNames := instanceset.GenerateInstanceNamesFromTemplate(workloadName, "",
			compReplicas-insTplReplicasCnt, offlineInstances)
		for _, podName := range podNames {
			podSet[podName] = ""
		}
	}
	return podSet
}
