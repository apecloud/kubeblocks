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

package operations

import (
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/stateless"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
)

// GetProgressObjectKey gets progress object key from the object of client.Object.
func GetProgressObjectKey(kind, name string) string {
	return fmt.Sprintf("%s/%s", kind, name)
}

// isCompletedProgressStatus the progress detail status is Failed or Succeed, means it is completed.
func isCompletedProgressStatus(status dbaasv1alpha1.ProgressStatus) bool {
	return slices.Contains([]dbaasv1alpha1.ProgressStatus{dbaasv1alpha1.SucceedProgressStatus,
		dbaasv1alpha1.FailedProgressStatus}, status)
}

// SetStatusComponentProgressDetail sets the corresponding progressDetail in progressDetails to newProgressDetail.
// progressDetails must be non-nil.
// 1. the startTime and endTime will be filled automatically.
// 2. if the progressDetail of the specified objectKey not exists, it will be appended to the progressDetails.
func SetStatusComponentProgressDetail(
	recorder record.EventRecorder,
	opsRequest *dbaasv1alpha1.OpsRequest,
	progressDetails *[]dbaasv1alpha1.ProgressDetail,
	newProgressDetail dbaasv1alpha1.ProgressDetail) {
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
	// if existing progress detail is Failed status and new progress detail is not Succeed status,
	// ignores the new progress detail.
	if existingProgressDetail.Status == dbaasv1alpha1.FailedProgressStatus &&
		newProgressDetail.Status != dbaasv1alpha1.SucceedProgressStatus {
		return
	}
	existingProgressDetail.Status = newProgressDetail.Status
	existingProgressDetail.Message = newProgressDetail.Message
	updateProgressDetailTime(existingProgressDetail)
	sendProgressDetailEvent(recorder, opsRequest, newProgressDetail)
}

// FindStatusProgressDetail finds the progressDetail of the specified objectKey in progressDetails.
func FindStatusProgressDetail(progressDetails []dbaasv1alpha1.ProgressDetail,
	objectKey string) *dbaasv1alpha1.ProgressDetail {
	for i := range progressDetails {
		if progressDetails[i].ObjectKey == objectKey {
			return &progressDetails[i]
		}
	}
	return nil
}

// getProgressDetailEventType gets the event type with progressDetail status.
func getProgressDetailEventType(status dbaasv1alpha1.ProgressStatus) string {
	if status == dbaasv1alpha1.FailedProgressStatus {
		return corev1.EventTypeWarning
	}
	return corev1.EventTypeNormal
}

// getProgressDetailEventReason gets the event reason with progressDetail status.
func getProgressDetailEventReason(status dbaasv1alpha1.ProgressStatus) string {
	switch status {
	case dbaasv1alpha1.SucceedProgressStatus:
		return "Succeed"
	case dbaasv1alpha1.ProcessingProgressStatus:
		return "Processing"
	case dbaasv1alpha1.FailedProgressStatus:
		return "Failed"
	}
	return ""
}

// sendProgressDetailEvent sends the progress detail changed events.
func sendProgressDetailEvent(recorder record.EventRecorder,
	opsRequest *dbaasv1alpha1.OpsRequest,
	progressDetail dbaasv1alpha1.ProgressDetail) {
	status := progressDetail.Status
	if status == dbaasv1alpha1.PendingProgressStatus {
		return
	}
	recorder.Event(opsRequest, getProgressDetailEventType(status),
		getProgressDetailEventReason(status), progressDetail.Message)
}

// updateProgressDetailTime updates the progressDetail startTime or endTime according to the status.
func updateProgressDetailTime(progressDetail *dbaasv1alpha1.ProgressDetail) {
	if progressDetail.Status == dbaasv1alpha1.ProcessingProgressStatus &&
		progressDetail.StartTime.IsZero() {
		progressDetail.StartTime = metav1.NewTime(time.Now())
	}
	if isCompletedProgressStatus(progressDetail.Status) &&
		progressDetail.EndTime.IsZero() {
		progressDetail.EndTime = metav1.NewTime(time.Now())
	}
}

// covertPodObjectKeyMap coverts the object key map from the pod list.
func covertPodObjectKeyMap(podList *corev1.PodList) map[string]struct{} {
	podObjectKeyMap := map[string]struct{}{}
	for _, v := range podList.Items {
		objectKey := GetProgressObjectKey(v.Kind, v.Name)
		podObjectKeyMap[objectKey] = struct{}{}
	}
	return podObjectKeyMap
}

// removeStatelessExpiredPod if the object of progressDetail is not existing in k8s cluster, means the pod is deleted.
// Some scenes, a replicaSet may attempt to create a pod multiple times until it succeeds.
// so some pod will be expired, we should clear them.
func removeStatelessExpiredPod(podList *corev1.PodList,
	progressDetails []dbaasv1alpha1.ProgressDetail) []dbaasv1alpha1.ProgressDetail {
	podObjectKeyMap := covertPodObjectKeyMap(podList)
	newProgressDetails := make([]dbaasv1alpha1.ProgressDetail, 0)
	for _, v := range progressDetails {
		if _, ok := podObjectKeyMap[v.ObjectKey]; ok {
			newProgressDetails = append(newProgressDetails, v)
		}
	}
	return newProgressDetails
}

// handleComponentStatusProgress handles the component status progressDetails.
// if all the pods of the component are affected, you can use this common function to reconcile the progressDetails.
func handleComponentStatusProgress(
	opsRes *OpsResource,
	pgRes progressResource,
	statusComponent *dbaasv1alpha1.OpsRequestStatusComponent) (expectProgressCount int32, succeedCount int32, err error) {
	var (
		podList             *corev1.PodList
		clusterComponentDef = pgRes.clusterComponentDef
		clusterComponent    = pgRes.clusterComponent
	)
	expectProgressCount = util.GetComponentReplicas(clusterComponent, clusterComponentDef)
	if podList, err = util.GetComponentPodList(opsRes.Ctx, opsRes.Client, opsRes.Cluster, clusterComponent.Name); err != nil {
		return
	}
	switch clusterComponentDef.ComponentType {
	case dbaasv1alpha1.Stateless:
		succeedCount, err = handleStatelessProgress(opsRes, podList, pgRes, statusComponent)
	default:
		succeedCount, err = handleStatefulSetProgress(opsRes, podList, pgRes, statusComponent)
	}
	return expectProgressCount, succeedCount, err
}

// handleStatelessProgress handles the stateless component progressDetails.
// the stateless component changes will use the Deployment updating policy.
func handleStatelessProgress(opsRes *OpsResource,
	podList *corev1.PodList,
	pgRes progressResource,
	statusComponent *dbaasv1alpha1.OpsRequestStatusComponent) (succeedCount int32, err error) {
	currComponent := stateless.NewStateless(opsRes.Ctx, opsRes.Client, opsRes.Cluster)
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
	for _, v := range podList.Items {
		// maybe the resources is equals last resources and the pod is not updated, then the pod will not rebuild too.
		// and the component status also is already Running/Failed/Abnormal, the progressDetails are already completed.
		if v.CreationTimestamp.Before(&opsRequest.Status.StartTimestamp) &&
			!util.IsCompleted(statusComponent.Phase) {
			continue
		}
		// if the DeletionTimestamp of the stateless component pod is not zero,
		// means the pod is discarded and can ignore it.
		if !v.DeletionTimestamp.IsZero() {
			continue
		}

		objectKey := GetProgressObjectKey(v.Kind, v.Name)
		progressDetail := dbaasv1alpha1.ProgressDetail{ObjectKey: objectKey}
		if currComponent.PodIsAvailable(&v, minReadySeconds) && v.DeletionTimestamp.IsZero() {
			succeedCount += 1
			progressDetail.SetStatusAndMessage(dbaasv1alpha1.SucceedProgressStatus,
				getProgressSucceedMessage(pgRes.opsMessageKey, objectKey, componentName))
			SetStatusComponentProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
				&statusComponent.ProgressDetails, progressDetail)
			continue
		}
		if util.IsFailedOrAbnormal(statusComponent.Phase) {
			// means the pod is failed.
			podMessage := getFailedPodMessage(opsRes.Cluster, componentName, &v)
			message := getProgressFailedMessage(pgRes.opsMessageKey, objectKey, componentName, podMessage)
			progressDetail.SetStatusAndMessage(dbaasv1alpha1.FailedProgressStatus, message)
		} else {
			progressDetail.SetStatusAndMessage(dbaasv1alpha1.ProcessingProgressStatus,
				getProgressProcessingMessage(pgRes.opsMessageKey, objectKey, componentName))
		}
		SetStatusComponentProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
			&statusComponent.ProgressDetails, progressDetail)
	}
	statusComponent.ProgressDetails = removeStatelessExpiredPod(podList, statusComponent.ProgressDetails)
	return succeedCount, err
}

// handleStatefulSetProgress handles the component progressDetails which using statefulSet workloads.
func handleStatefulSetProgress(opsRes *OpsResource,
	podList *corev1.PodList,
	pgRes progressResource,
	statusComponent *dbaasv1alpha1.OpsRequestStatusComponent) (succeedCount int32, err error) {
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
	for _, v := range podList.Items {
		objectKey := GetProgressObjectKey(v.Kind, v.Name)
		progressDetail := dbaasv1alpha1.ProgressDetail{ObjectKey: objectKey}
		// maybe the resources is equals last resources and the pod is not updated, then the pod will not rebuild too.
		// and the component status also is already Running/Failed/Abnormal, the progressDetails are already completed.
		if v.CreationTimestamp.Before(&opsRequest.Status.StartTimestamp) &&
			v.DeletionTimestamp.IsZero() && !util.IsCompleted(statusComponent.Phase) {
			progressDetail.Status = dbaasv1alpha1.PendingProgressStatus
			SetStatusComponentProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
				&statusComponent.ProgressDetails, progressDetail)
			continue
		}
		if currComponent.PodIsAvailable(&v, minReadySeconds) {
			succeedCount += 1
			progressDetail.SetStatusAndMessage(dbaasv1alpha1.SucceedProgressStatus,
				getProgressSucceedMessage(pgRes.opsMessageKey, objectKey, componentName))
			SetStatusComponentProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
				&statusComponent.ProgressDetails, progressDetail)
			continue
		}

		if util.IsFailedOrAbnormal(statusComponent.Phase) {
			// means the pod is failed.
			podMessage := getFailedPodMessage(opsRes.Cluster, componentName, &v)
			message := getProgressFailedMessage(pgRes.opsMessageKey, objectKey, componentName, podMessage)
			progressDetail.SetStatusAndMessage(dbaasv1alpha1.FailedProgressStatus, message)
		} else {
			progressDetail.SetStatusAndMessage(dbaasv1alpha1.ProcessingProgressStatus,
				getProgressProcessingMessage(pgRes.opsMessageKey, objectKey, componentName))
		}
		SetStatusComponentProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
			&statusComponent.ProgressDetails, progressDetail)
	}
	return succeedCount, err
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

// getFailedPodMessage gets the failed pod message from cluster status component
func getFailedPodMessage(cluster *dbaasv1alpha1.Cluster, componentName string, pod *corev1.Pod) string {
	clusterStatusComponent := cluster.Status.Components[componentName]
	componentMessage := clusterStatusComponent.GetMessage()
	return componentMessage.GetObjectMessage(pod.Kind, pod.Name)
}
