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

package operations

import (
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateless"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// getProgressObjectKey gets progress object key from the object of client.Object.
func getProgressObjectKey(kind, name string) string {
	return fmt.Sprintf("%s/%s", kind, name)
}

// isCompletedProgressStatus the progress detail is in final state, either Failed or Succeed.
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
	existingProgressDetail := findStatusProgressDetail(*progressDetails, newProgressDetail.ObjectKey)
	if existingProgressDetail == nil {
		updateProgressDetailTime(&newProgressDetail)
		*progressDetails = append(*progressDetails, newProgressDetail)
		sendProgressDetailEvent(recorder, opsRequest, newProgressDetail)
		return
	}
	if existingProgressDetail.Status == newProgressDetail.Status {
		return
	}
	// if existing progress detail is 'Failed' and new progress detail is not 'Succeed', ignores the new one.
	if existingProgressDetail.Status == appsv1alpha1.FailedProgressStatus &&
		newProgressDetail.Status != appsv1alpha1.SucceedProgressStatus {
		return
	}
	existingProgressDetail.Status = newProgressDetail.Status
	existingProgressDetail.Message = newProgressDetail.Message
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

// convertPodObjectKeyMap converts the object key map from the pod list.
func convertPodObjectKeyMap(podList *corev1.PodList) map[string]struct{} {
	podObjectKeyMap := map[string]struct{}{}
	for _, v := range podList.Items {
		objectKey := getProgressObjectKey(v.Kind, v.Name)
		podObjectKeyMap[objectKey] = struct{}{}
	}
	return podObjectKeyMap
}

// removeStatelessExpiredPod if the object of progressDetail is not existing in k8s cluster, it indicates the pod is deleted.
// For example, a replicaSet may attempt to create a pod multiple times till it succeeds.
// so some pod may be expired, we should clear them.
func removeStatelessExpiredPod(podList *corev1.PodList,
	progressDetails []appsv1alpha1.ProgressStatusDetail) []appsv1alpha1.ProgressStatusDetail {
	podObjectKeyMap := convertPodObjectKeyMap(podList)
	newProgressDetails := make([]appsv1alpha1.ProgressStatusDetail, 0)
	for _, v := range progressDetails {
		if _, ok := podObjectKeyMap[v.ObjectKey]; ok {
			newProgressDetails = append(newProgressDetails, v)
		}
	}
	return newProgressDetails
}

// handleComponentStatusProgress handles the component status progressDetails.
// if all the pods of the component are affected, use this common function to reconcile the progressDetails.
func handleComponentStatusProgress(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (expectProgressCount int32, completedCount int32, err error) {
	var (
		podList             *corev1.PodList
		clusterComponentDef = pgRes.clusterComponentDef
		clusterComponent    = pgRes.clusterComponent
	)
	if clusterComponent == nil || clusterComponentDef == nil {
		return
	}
	if podList, err = util.GetComponentPodList(reqCtx.Ctx, cli, *opsRes.Cluster, clusterComponent.Name); err != nil {
		return
	}
	switch clusterComponentDef.WorkloadType {
	case appsv1alpha1.Stateless:
		completedCount, err = handleStatelessProgress(reqCtx, cli, opsRes, podList, pgRes, compStatus)
	default:
		completedCount, err = handleStatefulSetProgress(reqCtx, cli, opsRes, podList, pgRes, compStatus)
	}
	return clusterComponent.Replicas, completedCount, err
}

// handleStatelessProgress handles the stateless component progressDetails.
// For stateless component changes, it applies the Deployment updating policy.
func handleStatelessProgress(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	podList *corev1.PodList,
	pgRes progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, error) {
	if compStatus.Phase == appsv1alpha1.RunningClusterCompPhase && pgRes.clusterComponent.Replicas != int32(len(podList.Items)) {
		return 0, intctrlutil.NewError(intctrlutil.ErrorWaitCacheRefresh, "wait for the pods of deployment to be synchronized")
	}

	currComponent, err := stateless.NewStatelessComponent(cli, opsRes.Cluster,
		pgRes.clusterComponent, *pgRes.clusterComponentDef)
	if err != nil {
		return 0, err
	}

	if currComponent == nil {
		return 0, nil
	}
	var componentName = pgRes.clusterComponent.Name
	minReadySeconds, err := util.GetComponentDeployMinReadySeconds(reqCtx.Ctx,
		cli, *opsRes.Cluster, componentName)
	if err != nil {
		return 0, err
	}
	var completedCount int32
	opsRequest := opsRes.OpsRequest
	opsStartTime := opsRequest.Status.StartTimestamp
	for _, v := range podList.Items {
		objectKey := getProgressObjectKey(v.Kind, v.Name)
		progressDetail := appsv1alpha1.ProgressStatusDetail{ObjectKey: objectKey}
		if podProcessedSuccessful(currComponent, opsStartTime, &v,
			minReadySeconds, compStatus.Phase, pgRes.opsIsCompleted) {
			completedCount += 1
			handleSucceedProgressDetail(opsRes, pgRes, compStatus, progressDetail)
			continue
		}
		if podIsPendingDuringOperation(opsStartTime, &v) {
			handlePendingProgressDetail(opsRes, compStatus, progressDetail)
			continue
		}
		completedCount += handleFailedOrProcessingProgressDetail(opsRes, pgRes, compStatus, progressDetail, &v)
	}
	compStatus.ProgressDetails = removeStatelessExpiredPod(podList, compStatus.ProgressDetails)
	return completedCount, err
}

// REVIEW/TOD: similar code pattern (do de-dupe)
// handleStatefulSetProgress handles the component progressDetails which using statefulSet workloads.
func handleStatefulSetProgress(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	podList *corev1.PodList,
	pgRes progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, error) {
	currComponent, minReadySeconds, err := getCompImplAndMinReadySeconds(reqCtx, cli, opsRes, pgRes)
	if err != nil {
		return 0, err
	}
	opsRequest := opsRes.OpsRequest
	opsStartTime := opsRequest.Status.StartTimestamp
	var completedCount int32
	for _, v := range podList.Items {
		objectKey := getProgressObjectKey(v.Kind, v.Name)
		progressDetail := appsv1alpha1.ProgressStatusDetail{ObjectKey: objectKey}
		if podProcessedSuccessful(currComponent, opsStartTime, &v,
			minReadySeconds, compStatus.Phase, pgRes.opsIsCompleted) {
			completedCount += 1
			handleSucceedProgressDetail(opsRes, pgRes, compStatus, progressDetail)
			continue
		}
		if podIsPendingDuringOperation(opsStartTime, &v) {
			handlePendingProgressDetail(opsRes, compStatus, progressDetail)
			continue
		}
		completedCount += handleFailedOrProcessingProgressDetail(opsRes, pgRes, compStatus, progressDetail, &v)
	}
	return completedCount, err
}

func getCompImplAndMinReadySeconds(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes progressResource) (types.Component, int32, error) {
	currComponent, err := components.NewComponentByType(cli,
		opsRes.Cluster, pgRes.clusterComponent, *pgRes.clusterComponentDef)
	if err != nil {
		return nil, 0, err
	}
	minReadySeconds, err := util.GetComponentStsMinReadySeconds(reqCtx.Ctx,
		cli, *opsRes.Cluster, pgRes.clusterComponent.Name)
	if err != nil {
		return nil, 0, err
	}
	return currComponent, minReadySeconds, nil
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
	pgRes progressResource,
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
	pgRes progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus,
	progressDetail appsv1alpha1.ProgressStatusDetail,
	pod *corev1.Pod) (completedCount int32) {
	componentName := pgRes.clusterComponent.Name
	opsStartTime := opsRes.OpsRequest.Status.StartTimestamp
	if podIsFailedDuringOperation(opsStartTime, pod, compStatus.Phase, pgRes.opsIsCompleted) {
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

// podIsPendingDuringOperation checks if pod is pending during the component is doing operation.
func podIsPendingDuringOperation(opsStartTime metav1.Time, pod *corev1.Pod) bool {
	return pod.CreationTimestamp.Before(&opsStartTime) && pod.DeletionTimestamp.IsZero()
}

// podIsFailedDuringOperation checks if pod is failed during operation.
func podIsFailedDuringOperation(
	opsStartTime metav1.Time,
	pod *corev1.Pod,
	componentPhase appsv1alpha1.ClusterComponentPhase,
	opsIsCompleted bool) bool {
	if !util.IsFailedOrAbnormal(componentPhase) {
		return false
	}
	return !pod.CreationTimestamp.Before(&opsStartTime) || opsIsCompleted
}

// podProcessedSuccessful checks if the pod has been processed successfully:
// 1. the pod is recreated after OpsRequest.status.startTime and pod is available.
// 2. the component is running and pod is available.
func podProcessedSuccessful(componentImpl types.Component,
	opsStartTime metav1.Time,
	pod *corev1.Pod,
	minReadySeconds int32,
	componentPhase appsv1alpha1.ClusterComponentPhase,
	opsIsCompleted bool) bool {
	if !componentImpl.PodIsAvailable(pod, minReadySeconds) {
		return false
	}
	return (opsIsCompleted && componentPhase == appsv1alpha1.RunningClusterCompPhase) || !pod.CreationTimestamp.Before(&opsStartTime)
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
	return clusterCompStatus.GetObjectMessage(pod.Kind, pod.Name)
}

func getComponentLastReplicas(opsRequest *appsv1alpha1.OpsRequest, componentName string) *int32 {
	for k, v := range opsRequest.Status.LastConfiguration.Components {
		if k == componentName {
			return v.Replicas
		}
	}
	return nil
}

// handleComponentProgressDetails handles the component progressDetails when scale the replicas.
// @return expectProgressCount,
// @return completedCount
// @return error
func handleComponentProgressForScalingReplicas(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus,
	getExpectReplicas func(opsRequest *appsv1alpha1.OpsRequest, componentName string) *int32) (int32, int32, error) {
	var (
		podList             *corev1.PodList
		clusterComponent    = pgRes.clusterComponent
		opsRequest          = opsRes.OpsRequest
		isScaleOut          bool
		expectProgressCount int32
		completedCount      int32
		err                 error
	)
	if clusterComponent == nil || pgRes.clusterComponentDef == nil {
		return 0, 0, nil
	}
	expectReplicas := getExpectReplicas(opsRequest, clusterComponent.Name)
	if expectReplicas == nil {
		return 0, 0, nil
	}
	lastComponentReplicas := getComponentLastReplicas(opsRequest, clusterComponent.Name)
	if lastComponentReplicas == nil {
		return 0, 0, nil
	}
	// if replicas are not changed, return
	if *lastComponentReplicas == *expectReplicas {
		return 0, 0, nil
	}
	if podList, err = util.GetComponentPodList(reqCtx.Ctx, cli, *opsRes.Cluster, clusterComponent.Name); err != nil {
		return 0, 0, err
	}
	if compStatus.Phase == appsv1alpha1.RunningClusterCompPhase && pgRes.clusterComponent.Replicas != int32(len(podList.Items)) {
		return 0, 0, intctrlutil.NewError(intctrlutil.ErrorWaitCacheRefresh, "wait for the pods of component to be synchronized")
	}
	dValue := *expectReplicas - *lastComponentReplicas
	if dValue > 0 {
		expectProgressCount = dValue
		isScaleOut = true
	} else {
		expectProgressCount = dValue * -1
	}
	if !isScaleOut {
		completedCount, err = handleScaleDownProgress(reqCtx, cli, opsRes, pgRes, podList, compStatus)
		expectProgressCount = getFinalExpectCount(compStatus, expectProgressCount)
		return expectProgressCount, completedCount, err
	}
	completedCount, err = handleScaleOutProgress(reqCtx, cli, opsRes, pgRes, podList, compStatus)
	// if the workload type is Stateless, remove the progressDetails of the expired pods.
	// because a replicaSet may attempt to create a pod multiple times till it succeeds when scale out the replicas.
	if pgRes.clusterComponentDef.WorkloadType == appsv1alpha1.Stateless {
		compStatus.ProgressDetails = removeStatelessExpiredPod(podList, compStatus.ProgressDetails)
	}
	return getFinalExpectCount(compStatus, expectProgressCount), completedCount, err
}

// handleScaleOutProgress handles the progressDetails of scaled out replicas.
func handleScaleOutProgress(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes progressResource,
	podList *corev1.PodList,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, error) {
	currComponent, minReadySeconds, err := getCompImplAndMinReadySeconds(reqCtx, cli, opsRes, pgRes)
	if err != nil {
		return 0, err
	}
	var completedCount int32
	for _, v := range podList.Items {
		// only focus on the newly created pod when scaling out the replicas.
		if v.CreationTimestamp.Before(&opsRes.OpsRequest.Status.StartTimestamp) {
			continue
		}
		objectKey := getProgressObjectKey(v.Kind, v.Name)
		progressDetail := appsv1alpha1.ProgressStatusDetail{ObjectKey: objectKey}
		if currComponent.PodIsAvailable(&v, minReadySeconds) {
			completedCount += 1
			pgRes.opsMessageKey = "created"
			handleSucceedProgressDetail(opsRes, pgRes, compStatus, progressDetail)
			continue
		}
		completedCount += handleFailedOrProcessingProgressDetail(opsRes, pgRes, compStatus, progressDetail, &v)
	}
	return completedCount, nil
}

// handleScaleDownProgress handles the progressDetails of scaled down replicas.
func handleScaleDownProgress(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes progressResource,
	podList *corev1.PodList,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (completedCount int32, err error) {
	podMap := map[string]struct{}{}
	// record the deleting pod progressDetail
	for _, v := range podList.Items {
		objectKey := getProgressObjectKey(constant.PodKind, v.Name)
		podMap[objectKey] = struct{}{}
		if v.DeletionTimestamp.IsZero() {
			continue
		}
		progressDetail := appsv1alpha1.ProgressStatusDetail{
			ObjectKey: objectKey,
			Status:    appsv1alpha1.ProcessingProgressStatus,
			Message:   fmt.Sprintf("Start to delete pod: %s in Component: %s", objectKey, pgRes.clusterComponent.Name),
		}
		setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
			&compStatus.ProgressDetails, progressDetail)
	}
	lastComponentConfigs := opsRes.OpsRequest.Status.LastConfiguration.Components[pgRes.clusterComponent.Name]
	lastComponentPodNames := lastComponentConfigs.TargetResources[appsv1alpha1.PodsCompResourceKey]
	for _, v := range lastComponentPodNames {
		objectKey := getProgressObjectKey(constant.PodKind, v)
		if _, ok := podMap[objectKey]; ok {
			continue
		}
		// if the pod is not in the podList, it means the pod has been deleted.
		progressDetail := appsv1alpha1.ProgressStatusDetail{
			ObjectKey: objectKey,
			Status:    appsv1alpha1.SucceedProgressStatus,
			Message:   fmt.Sprintf("Successfully deleted pod: %s in Component: %s", objectKey, pgRes.clusterComponent.Name),
		}
		completedCount += 1
		setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
			&compStatus.ProgressDetails, progressDetail)
	}
	// handle the re-created pods if these pods are failed before doing horizontal scaling.
	currComponent, minReadySeconds, err := getCompImplAndMinReadySeconds(reqCtx, cli, opsRes, pgRes)
	if err != nil {
		return 0, err
	}
	for _, v := range podList.Items {
		objectKey := getProgressObjectKey(constant.PodKind, v.Name)
		progressDetail := findStatusProgressDetail(compStatus.ProgressDetails, objectKey)
		if progressDetail == nil {
			continue
		}
		if isCompletedProgressStatus(progressDetail.Status) {
			completedCount += 1
			continue
		}
		pgRes.opsMessageKey = "re-create"
		if currComponent.PodIsAvailable(&v, minReadySeconds) {
			completedCount += 1
			handleSucceedProgressDetail(opsRes, pgRes, compStatus, *progressDetail)
			continue
		}
		completedCount += handleFailedOrProcessingProgressDetail(opsRes, pgRes, compStatus, *progressDetail, &v)
	}
	return completedCount, nil
}

// getFinalExpectCount gets the number of pods which has been processed by controller.
func getFinalExpectCount(compStatus *appsv1alpha1.OpsRequestComponentStatus, expectProgressCount int32) int32 {
	progressDetailsLen := int32(len(compStatus.ProgressDetails))
	if progressDetailsLen > expectProgressCount {
		expectProgressCount = progressDetailsLen
	}
	return expectProgressCount
}
