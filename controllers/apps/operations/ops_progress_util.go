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
	progressDetails *[]appsv1alpha1.ProgressDetail,
	newProgressDetail appsv1alpha1.ProgressDetail) {
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
func FindStatusProgressDetail(progressDetails []appsv1alpha1.ProgressDetail,
	objectKey string) *appsv1alpha1.ProgressDetail {
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
	progressDetail appsv1alpha1.ProgressDetail) {
	status := progressDetail.Status
	if status == appsv1alpha1.PendingProgressStatus {
		return
	}
	recorder.Event(opsRequest, getProgressDetailEventType(status),
		getProgressDetailEventReason(status), progressDetail.Message)
}

// updateProgressDetailTime updates the progressDetail startTime or endTime according to the status.
func updateProgressDetailTime(progressDetail *appsv1alpha1.ProgressDetail) {
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
	progressDetails []appsv1alpha1.ProgressDetail) []appsv1alpha1.ProgressDetail {
	podObjectKeyMap := convertPodObjectKeyMap(podList)
	newProgressDetails := make([]appsv1alpha1.ProgressDetail, 0)
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
	if podList, err = util.GetComponentPodList(opsRes.Ctx, opsRes.Client, opsRes.Cluster, clusterComponent.Name); err != nil {
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
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (completedCount int32, err error) {
	if compStatus.Phase == appsv1alpha1.RunningPhase && pgRes.clusterComponent.Replicas != int32(len(podList.Items)) {
		err = fmt.Errorf("wait for the pods of deployment to be synchronized to client-go cache")
		return
	}

	currComponent := stateless.NewStateless(opsRes.Ctx, opsRes.Client, opsRes.Cluster,
		pgRes.clusterComponent, pgRes.clusterComponentDef)
	if currComponent == nil {
		return
	}
	var componentName = pgRes.clusterComponent.Name
	minReadySeconds, err := util.GetComponentDeployMinReadySeconds(opsRes.Ctx,
		opsRes.Client, opsRes.Cluster, componentName)
	if err != nil {
		return
	}
	opsRequest := opsRes.OpsRequest
	opsStartTime := opsRequest.Status.StartTimestamp
	for _, v := range podList.Items {
		objectKey := GetProgressObjectKey(v.Kind, v.Name)
		progressDetail := appsv1alpha1.ProgressDetail{ObjectKey: objectKey}
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

// handleStatefulSetProgress handles the component progressDetails which using statefulSet workloads.
func handleStatefulSetProgress(opsRes *OpsResource,
	podList *corev1.PodList,
	pgRes progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (completedCount int32, err error) {
	currComponent := components.NewComponentByType(opsRes.Ctx, opsRes.Client,
		opsRes.Cluster, pgRes.clusterComponentDef, pgRes.clusterComponent)
	if currComponent == nil {
		return
	}
	var componentName = pgRes.clusterComponent.Name
	minReadySeconds, err := util.GetComponentStsMinReadySeconds(opsRes.Ctx,
		opsRes.Client, opsRes.Cluster, componentName)
	if err != nil {
		return
	}
	opsRequest := opsRes.OpsRequest
	opsStartTime := opsRequest.Status.StartTimestamp
	for _, v := range podList.Items {
		objectKey := GetProgressObjectKey(v.Kind, v.Name)
		progressDetail := appsv1alpha1.ProgressDetail{ObjectKey: objectKey}
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
	progressDetail appsv1alpha1.ProgressDetail,
) {
	progressDetail.Status = appsv1alpha1.PendingProgressStatus
	SetComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
		&compStatus.ProgressDetails, progressDetail)
}

// handleSucceedProgressDetail handles the successful progressDetail and sets it to progressDetails.
func handleSucceedProgressDetail(opsRes *OpsResource,
	pgRes progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus,
	progressDetail appsv1alpha1.ProgressDetail,
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
	progressDetail appsv1alpha1.ProgressDetail,
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
	componentMessage := clusterCompStatus.GetMessage()
	return componentMessage.GetObjectMessage(pod.Kind, pod.Name)
}
