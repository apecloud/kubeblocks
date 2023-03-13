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

package operations

import (
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateless"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
)

// GetProgressObjectKey gets progress object key from the object of client.Object.
func GetProgressObjectKey(kind, name string) string {
	return fmt.Sprintf("%s/%s", kind, name)
}

// isCompletedProgressStatus the progress detail is in final state, either Failed or Succeed.
func isCompletedProgressStatus(status appsv1alpha1.ProgressStatus) bool {
	return slices.Contains([]appsv1alpha1.ProgressStatus{appsv1alpha1.SucceedProgressStatus,
		appsv1alpha1.FailedProgressStatus}, status)
}

// SetComponentStatusProgressDetail sets the corresponding progressDetail in progressDetails to newProgressDetail.
// progressDetails must be non-nil.
// 1. the startTime and endTime will be filled automatically.
// 2. if the progressDetail of the specified objectKey does not exist, it will be appended to the progressDetails.
func SetComponentStatusProgressDetail(
	recorder record.EventRecorder,
	opsRequest *appsv1alpha1.OpsRequest,
	progressDetails *[]appsv1alpha1.ProgressStatusDetail,
	newProgressDetail appsv1alpha1.ProgressStatusDetail) {
	if progressDetails == nil {
		return
	}
	existingProgressDetail := FindStatusProgressDetail(*progressDetails, newProgressDetail.ObjectKey)
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

// FindStatusProgressDetail finds the progressDetail of the specified objectKey in progressDetails.
func FindStatusProgressDetail(progressDetails []appsv1alpha1.ProgressStatusDetail,
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
		objectKey := GetProgressObjectKey(v.Kind, v.Name)
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
	if podList, err = util.GetComponentPodList(opsRes.Ctx, opsRes.Client, *opsRes.Cluster, clusterComponent.Name); err != nil {
		return
	}
	switch clusterComponentDef.WorkloadType {
	case appsv1alpha1.Stateless:
		completedCount, err = handleStatelessProgress(opsRes, podList, pgRes, compStatus)
	default:
		completedCount, err = handleStatefulSetProgress(opsRes, podList, pgRes, compStatus)
	}
	return clusterComponent.Replicas, completedCount, err
}

// handleStatelessProgress handles the stateless component progressDetails.
// For stateless component changes, it applies the Deployment updating policy.
func handleStatelessProgress(opsRes *OpsResource,
	podList *corev1.PodList,
	pgRes progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, error) {
	if compStatus.Phase == appsv1alpha1.RunningPhase && pgRes.clusterComponent.Replicas != int32(len(podList.Items)) {
		return 0, fmt.Errorf("wait for the pods of deployment to be synchronized to client-go cache")
	}

	currComponent, err := stateless.NewStateless(opsRes.Client, opsRes.Cluster,
		pgRes.clusterComponent, *pgRes.clusterComponentDef)
	if err != nil {
		return 0, err
	}

	if currComponent == nil {
		return 0, nil
	}
	var componentName = pgRes.clusterComponent.Name
	minReadySeconds, err := util.GetComponentDeployMinReadySeconds(opsRes.Ctx,
		opsRes.Client, *opsRes.Cluster, componentName)
	if err != nil {
		return 0, err
	}
	var completedCount int32
	opsRequest := opsRes.OpsRequest
	opsStartTime := opsRequest.Status.StartTimestamp
	for _, v := range podList.Items {
		objectKey := GetProgressObjectKey(v.Kind, v.Name)
		progressDetail := appsv1alpha1.ProgressStatusDetail{ObjectKey: objectKey}
		if podIsPendingDuringOperation(opsStartTime, &v, compStatus.Phase) {
			handlePendingProgressDetail(opsRes, compStatus, progressDetail)
			continue
		}

		if podProcessedSuccessful(currComponent, opsStartTime, &v, minReadySeconds, compStatus.Phase) {
			completedCount += 1
			handleSucceedProgressDetail(opsRes, pgRes, compStatus, progressDetail)
			continue
		}
		completedCount += handleFailedOrProcessingProgressDetail(opsRes, pgRes, compStatus, progressDetail, &v)
	}
	compStatus.ProgressDetails = removeStatelessExpiredPod(podList, compStatus.ProgressDetails)
	return completedCount, err
}

// REVIEW/TOD: similar code pattern (do de-dupe)
// handleStatefulSetProgress handles the component progressDetails which using statefulSet workloads.
func handleStatefulSetProgress(opsRes *OpsResource,
	podList *corev1.PodList,
	pgRes progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, error) {
	currComponent, err := components.NewComponentByType(opsRes.Client,
		opsRes.Cluster, pgRes.clusterComponent, *pgRes.clusterComponentDef)
	if err != nil {
		return 0, err
	}
	var componentName = pgRes.clusterComponent.Name
	minReadySeconds, err := util.GetComponentStsMinReadySeconds(opsRes.Ctx,
		opsRes.Client, *opsRes.Cluster, componentName)
	if err != nil {
		return 0, err
	}
	opsRequest := opsRes.OpsRequest
	opsStartTime := opsRequest.Status.StartTimestamp
	var completedCount int32
	for _, v := range podList.Items {
		objectKey := GetProgressObjectKey(v.Kind, v.Name)
		progressDetail := appsv1alpha1.ProgressStatusDetail{ObjectKey: objectKey}
		if podIsPendingDuringOperation(opsStartTime, &v, compStatus.Phase) {
			handlePendingProgressDetail(opsRes, compStatus, progressDetail)
			continue
		}
		if podProcessedSuccessful(currComponent, opsStartTime, &v, minReadySeconds, compStatus.Phase) {
			completedCount += 1
			handleSucceedProgressDetail(opsRes, pgRes, compStatus, progressDetail)
			continue
		}
		completedCount += handleFailedOrProcessingProgressDetail(opsRes, pgRes, compStatus, progressDetail, &v)
	}
	return completedCount, err
}

// handlePendingProgressDetail handles the pending progressDetail and sets it to progressDetails.
func handlePendingProgressDetail(opsRes *OpsResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus,
	progressDetail appsv1alpha1.ProgressStatusDetail,
) {
	progressDetail.Status = appsv1alpha1.PendingProgressStatus
	SetComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
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
	SetComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
		&compStatus.ProgressDetails, progressDetail)
}

// handleFailedOrProcessingProgressDetail handles failed or processing progressDetail and sets it to progressDetails.
func handleFailedOrProcessingProgressDetail(opsRes *OpsResource,
	pgRes progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus,
	progressDetail appsv1alpha1.ProgressStatusDetail,
	pod *corev1.Pod) (completedCount int32) {
	componentName := pgRes.clusterComponent.Name
	if util.IsFailedOrAbnormal(compStatus.Phase) {
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
	SetComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
		&compStatus.ProgressDetails, progressDetail)
	return completedCount
}

// podIsPendingDuringOperation checks if pod is pending during the component is doing operation.
func podIsPendingDuringOperation(opsStartTime metav1.Time, pod *corev1.Pod, componentPhase appsv1alpha1.Phase) bool {
	return pod.CreationTimestamp.Before(&opsStartTime) && !util.IsCompleted(componentPhase) && pod.DeletionTimestamp.IsZero()
}

// podProcessedSuccessful checks if the pod has been processed successfully:
// 1. the pod is recreated after OpsRequest.status.startTime and pod is available.
// 2. the component is running and pod is available.
func podProcessedSuccessful(componentImpl types.Component,
	opsStartTime metav1.Time,
	pod *corev1.Pod,
	minReadySeconds int32,
	componentPhase appsv1alpha1.Phase) bool {
	return (!pod.CreationTimestamp.Before(&opsStartTime) || componentPhase == appsv1alpha1.RunningPhase) &&
		componentImpl.PodIsAvailable(pod, minReadySeconds)
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
func handleComponentProgressForScalingReplicas(opsRes *OpsResource,
	pgRes progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus,
	getExpectReplicas func(opsRequest *appsv1alpha1.OpsRequest, componentName string) *int32) (expectProgressCount int32, completedCount int32, err error) {
	var (
		podList          *corev1.PodList
		clusterComponent = pgRes.clusterComponent
		opsRequest       = opsRes.OpsRequest
		isScaleOut       bool
	)
	if clusterComponent == nil || pgRes.clusterComponentDef == nil {
		return
	}
	expectReplicas := getExpectReplicas(opsRequest, clusterComponent.Name)
	if expectReplicas == nil {
		return
	}
	lastComponentReplicas := getComponentLastReplicas(opsRequest, clusterComponent.Name)
	if lastComponentReplicas == nil {
		return
	}
	// if replicas are not changed, return
	if *lastComponentReplicas == *expectReplicas {
		return
	}
	if podList, err = util.GetComponentPodList(opsRes.Ctx, opsRes.Client, *opsRes.Cluster, clusterComponent.Name); err != nil {
		return
	}
	if compStatus.Phase == appsv1alpha1.RunningPhase && pgRes.clusterComponent.Replicas != int32(len(podList.Items)) {
		err = fmt.Errorf("wait for the pods of deployment to be synchronized to client-go cache")
		return
	}
	dValue := *expectReplicas - *lastComponentReplicas
	if dValue > 0 {
		expectProgressCount = dValue
		isScaleOut = true
	} else {
		expectProgressCount = dValue * -1
	}
	if !isScaleOut {
		completedCount, err = handleScaleDownProgress(opsRes, pgRes, podList, compStatus)
		expectProgressCount = getFinalExpectCount(compStatus, expectProgressCount)
		return
	}
	completedCount, err = handleScaleOutProgress(opsRes, pgRes, podList, compStatus)
	// if the workload type is Stateless, remove the progressDetails of the expired pods.
	// because a replicaSet may attempt to create a pod multiple times till it succeeds when scale out the replicas.
	if pgRes.clusterComponentDef.WorkloadType == appsv1alpha1.Stateless {
		compStatus.ProgressDetails = removeStatelessExpiredPod(podList, compStatus.ProgressDetails)
	}
	return getFinalExpectCount(compStatus, expectProgressCount), completedCount, err
}

// handleScaleOutProgress handles the progressDetails of scaled out replicas.
func handleScaleOutProgress(
	opsRes *OpsResource,
	pgRes progressResource,
	podList *corev1.PodList,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, error) {
	var componentName = pgRes.clusterComponent.Name
	currComponent, err := components.NewComponentByType(opsRes.Client,
		opsRes.Cluster, pgRes.clusterComponent, *pgRes.clusterComponentDef)
	if err != nil {
		return 0, err
	}
	minReadySeconds, err := util.GetComponentWorkloadMinReadySeconds(opsRes.Ctx,
		opsRes.Client, *opsRes.Cluster, pgRes.clusterComponentDef.WorkloadType, componentName)
	if err != nil {
		return 0, err
	}
	var completedCount int32
	for _, v := range podList.Items {
		// only focus on the newly created pod when scaling out the replicas.
		if v.CreationTimestamp.Before(&opsRes.OpsRequest.Status.StartTimestamp) {
			continue
		}
		objectKey := GetProgressObjectKey(v.Kind, v.Name)
		progressDetail := appsv1alpha1.ProgressStatusDetail{ObjectKey: objectKey}
		if currComponent.PodIsAvailable(&v, minReadySeconds) {
			completedCount += 1
			message := fmt.Sprintf("Successfully created pod: %s in Component: %s", objectKey, componentName)
			progressDetail.SetStatusAndMessage(appsv1alpha1.SucceedProgressStatus, message)
			SetComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
				&compStatus.ProgressDetails, progressDetail)
			continue
		}

		if util.IsFailedOrAbnormal(compStatus.Phase) {
			// means the pod is failed.
			podMessage := getFailedPodMessage(opsRes.Cluster, componentName, &v)
			message := fmt.Sprintf("Failed to create pod: %s in Component: %s, message: %s", objectKey, componentName, podMessage)
			progressDetail.SetStatusAndMessage(appsv1alpha1.FailedProgressStatus, message)
			completedCount += 1
		} else {
			progressDetail.SetStatusAndMessage(appsv1alpha1.ProcessingProgressStatus, "Start to create pod: "+objectKey)
		}
		SetComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
			&compStatus.ProgressDetails, progressDetail)
	}
	return completedCount, nil
}

// handleScaleDownProgress handles the progressDetails of scaled down replicas.
func handleScaleDownProgress(opsRes *OpsResource,
	pgRes progressResource,
	podList *corev1.PodList,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (completedCount int32, err error) {
	podMap := map[string]struct{}{}
	// record the deleting pod progressDetail
	for _, v := range podList.Items {
		objectKey := GetProgressObjectKey(v.Kind, v.Name)
		podMap[objectKey] = struct{}{}
		if v.DeletionTimestamp.IsZero() {
			continue
		}
		progressDetail := appsv1alpha1.ProgressStatusDetail{
			ObjectKey: objectKey,
			Status:    appsv1alpha1.ProcessingProgressStatus,
			Message:   fmt.Sprintf("Start to delete pod: %s in Component: %s", objectKey, pgRes.clusterComponent.Name),
		}
		SetComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
			&compStatus.ProgressDetails, progressDetail)
	}

	// The deployment controller will not watch the cleaning events of the old replicaSet pods.
	// so when component status is completed, we should forward the progressDetails to succeed.
	markStatelessPodsSucceed := false
	if pgRes.clusterComponentDef.WorkloadType == appsv1alpha1.Stateless &&
		util.IsCompleted(compStatus.Phase) {
		markStatelessPodsSucceed = true
	}

	for _, v := range compStatus.ProgressDetails {
		if _, ok := podMap[v.ObjectKey]; ok && !markStatelessPodsSucceed {
			continue
		}
		// if the pod object of progressDetail is not existing in podMap, means successfully deleted.
		progressDetail := appsv1alpha1.ProgressStatusDetail{
			ObjectKey: v.ObjectKey,
			Status:    appsv1alpha1.SucceedProgressStatus,
			Message:   fmt.Sprintf("Successfully deleted pod: %s in Component: %s", v.ObjectKey, pgRes.clusterComponent.Name),
		}
		completedCount += 1
		SetComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
			&compStatus.ProgressDetails, progressDetail)
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
