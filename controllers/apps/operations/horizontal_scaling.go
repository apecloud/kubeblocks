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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
)

type horizontalScalingOpsHandler struct{}

var _ OpsHandler = horizontalScalingOpsHandler{}

func init() {
	horizontalScalingBehaviour := OpsBehaviour{
		FromClusterPhases: []appsv1alpha1.Phase{appsv1alpha1.RunningPhase, appsv1alpha1.FailedPhase, appsv1alpha1.AbnormalPhase},
		ToClusterPhase:    appsv1alpha1.HorizontalScalingPhase,
		OpsHandler:        horizontalScalingOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.HorizontalScalingType, horizontalScalingBehaviour)
}

// ActionStartedCondition the started condition when handling the horizontal scaling request.
func (hs horizontalScalingOpsHandler) ActionStartedCondition(opsRequest *appsv1alpha1.OpsRequest) *metav1.Condition {
	return appsv1alpha1.NewHorizontalScalingCondition(opsRequest)
}

// Action modifies Cluster.spec.components[*].replicas from the opsRequest
func (hs horizontalScalingOpsHandler) Action(opsRes *OpsResource) error {
	var (
		horizontalScalingMap = opsRes.OpsRequest.CovertHorizontalScalingListToMap()
		horizontalScaling    appsv1alpha1.HorizontalScaling
		ok                   bool
	)

	for index, component := range opsRes.Cluster.Spec.ComponentSpecs {
		if horizontalScaling, ok = horizontalScalingMap[component.Name]; !ok {
			continue
		}
		if horizontalScaling.Replicas != 0 {
			r := horizontalScaling.Replicas
			opsRes.Cluster.Spec.ComponentSpecs[index].Replicas = &r
		}
	}
	return opsRes.Client.Update(opsRes.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for horizontal scaling opsRequest.
func (hs horizontalScalingOpsHandler) ReconcileAction(opsRes *OpsResource) (appsv1alpha1.Phase, time.Duration, error) {
	return ReconcileActionWithComponentOps(opsRes, "", hs.handleComponentProgressDetails)
}

// GetRealAffectedComponentMap gets the real affected component map for the operation
func (hs horizontalScalingOpsHandler) GetRealAffectedComponentMap(opsRequest *appsv1alpha1.OpsRequest) realAffectedComponentMap {
	realChangedMap := realAffectedComponentMap{}
	hsMap := opsRequest.CovertHorizontalScalingListToMap()
	for k, v := range opsRequest.Status.LastConfiguration.Components {
		currHs, ok := hsMap[k]
		if !ok {
			continue
		}
		if v.Replicas != currHs.Replicas {
			realChangedMap[k] = struct{}{}
		}
	}
	return realChangedMap
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (hs horizontalScalingOpsHandler) SaveLastConfiguration(opsRes *OpsResource) error {
	opsRequest := opsRes.OpsRequest
	lastComponentInfo := map[string]appsv1alpha1.LastComponentConfiguration{}
	clusterDef, err := GetClusterDefByName(opsRes.Ctx, opsRes.Client, opsRes.Cluster.Spec.ClusterDefRef)
	if err != nil {
		return err
	}
	componentNameMap := opsRequest.GetComponentNameMap()
	for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
		if _, ok := componentNameMap[v.Name]; !ok {
			continue
		}
		clusterComponentDef := clusterDef.GetComponentDefByTypeName(v.ComponentDefRef)
		lastComponentInfo[v.Name] = appsv1alpha1.LastComponentConfiguration{
			Replicas: util.GetComponentReplicas(&v, clusterComponentDef),
		}
	}
	patch := client.MergeFrom(opsRequest.DeepCopy())
	opsRequest.Status.LastConfiguration = appsv1alpha1.LastConfiguration{
		Components: lastComponentInfo,
	}
	return opsRes.Client.Status().Patch(opsRes.Ctx, opsRequest, patch)
}

func (hs horizontalScalingOpsHandler) getExpectReplicas(opsRequest *appsv1alpha1.OpsRequest, componentName string) *int32 {
	for _, v := range opsRequest.Spec.HorizontalScalingList {
		if v.ComponentName == componentName {
			return &v.Replicas
		}
	}
	return nil
}

func (hs horizontalScalingOpsHandler) getComponentLastReplicas(opsRequest *appsv1alpha1.OpsRequest, componentName string) *int32 {
	for k, v := range opsRequest.Status.LastConfiguration.Components {
		if k == componentName {
			return &v.Replicas
		}
	}
	return nil
}

// handleComponentProgressDetails handles the component progressDetails when horizontal scale the replicas.
func (hs horizontalScalingOpsHandler) handleComponentProgressDetails(opsRes *OpsResource,
	pgRes progressResource,
	statusComponent *appsv1alpha1.OpsRequestStatusComponent) (expectProgressCount int32, succeedCount int32, err error) {
	var (
		podList          *corev1.PodList
		clusterComponent = pgRes.clusterComponent
		opsRequest       = opsRes.OpsRequest
		isScaleOut       bool
	)
	if clusterComponent == nil || pgRes.clusterComponentDef == nil {
		return
	}
	expectReplicas := hs.getExpectReplicas(opsRequest, clusterComponent.Name)
	if expectReplicas == nil {
		return
	}
	lastComponentReplicas := hs.getComponentLastReplicas(opsRequest, clusterComponent.Name)
	if lastComponentReplicas == nil {
		return
	}
	// if replicas are not changed, return
	if *lastComponentReplicas == *expectReplicas {
		return
	}
	dValue := *expectReplicas - *lastComponentReplicas
	if dValue > 0 {
		expectProgressCount = dValue
		isScaleOut = true
	} else {
		expectProgressCount = dValue * -1
	}
	if podList, err = util.GetComponentPodList(opsRes.Ctx, opsRes.Client, opsRes.Cluster, clusterComponent.Name); err != nil {
		return
	}
	if !isScaleOut {
		succeedCount, err = hs.handleScaleDownProgress(opsRes, pgRes, podList, statusComponent)
		return
	}
	succeedCount, err = hs.handleScaleOutProgress(opsRes, pgRes, podList, statusComponent)
	// if the component type is Stateless, remove the progressDetails of the expired pods.
	// because a replicaSet may attempt to create a pod multiple times till it succeeds when scale out the replicas.
	if pgRes.clusterComponentDef.WorkloadType == appsv1alpha1.Stateless {
		statusComponent.ProgressDetails = removeStatelessExpiredPod(podList, statusComponent.ProgressDetails)
	}
	return expectProgressCount, succeedCount, err
}

// handleScaleOutProgress handles the progressDetails of scaled out replicas.
func (hs horizontalScalingOpsHandler) handleScaleOutProgress(
	opsRes *OpsResource,
	pgRes progressResource,
	podList *corev1.PodList,
	statusComponent *appsv1alpha1.OpsRequestStatusComponent) (succeedCount int32, err error) {
	var componentName = pgRes.clusterComponent.Name
	currComponent := components.NewComponentByType(opsRes.Ctx, opsRes.Client,
		opsRes.Cluster, pgRes.clusterComponentDef, pgRes.clusterComponent)
	if currComponent == nil {
		return
	}
	minReadySeconds, err := util.GetComponentWorkloadMinReadySeconds(opsRes.Ctx,
		opsRes.Client, opsRes.Cluster, pgRes.clusterComponentDef.WorkloadType, componentName)
	if err != nil {
		return
	}
	for _, v := range podList.Items {
		// only focus on the newly created pod when scaling out the replicas.
		if v.CreationTimestamp.Before(&opsRes.OpsRequest.Status.StartTimestamp) {
			continue
		}
		objectKey := GetProgressObjectKey(v.Kind, v.Name)
		progressDetail := appsv1alpha1.ProgressDetail{ObjectKey: objectKey}
		if currComponent.PodIsAvailable(&v, minReadySeconds) {
			succeedCount += 1
			message := fmt.Sprintf("Successfully created pod: %s in Component: %s", objectKey, componentName)
			progressDetail.SetStatusAndMessage(appsv1alpha1.SucceedProgressStatus, message)
			SetStatusComponentProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
				&statusComponent.ProgressDetails, progressDetail)
			continue
		}

		if util.IsFailedOrAbnormal(statusComponent.Phase) {
			// means the pod is failed.
			podMessage := getFailedPodMessage(opsRes.Cluster, componentName, &v)
			message := fmt.Sprintf("Failed to create pod: %s in Component: %s, message: %s", objectKey, componentName, podMessage)
			progressDetail.SetStatusAndMessage(appsv1alpha1.FailedProgressStatus, message)
		} else {
			progressDetail.SetStatusAndMessage(appsv1alpha1.ProcessingProgressStatus, "Start to create pod: "+objectKey)
		}
		SetStatusComponentProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
			&statusComponent.ProgressDetails, progressDetail)
	}
	return succeedCount, nil
}

// handleScaleDownProgress handles the progressDetails of scaled down replicas.
func (hs horizontalScalingOpsHandler) handleScaleDownProgress(opsRes *OpsResource,
	pgRes progressResource,
	podList *corev1.PodList,
	statusComponent *appsv1alpha1.OpsRequestStatusComponent) (succeedCount int32, err error) {
	podMap := map[string]struct{}{}
	// record the deleting pod progressDetail
	for _, v := range podList.Items {
		objectKey := GetProgressObjectKey(v.Kind, v.Name)
		podMap[objectKey] = struct{}{}
		if v.DeletionTimestamp.IsZero() {
			continue
		}
		progressDetail := appsv1alpha1.ProgressDetail{
			ObjectKey: objectKey,
			Status:    appsv1alpha1.ProcessingProgressStatus,
			Message:   fmt.Sprintf("Start to delete pod: %s in Component: %s", objectKey, pgRes.clusterComponent.Name),
		}
		SetStatusComponentProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
			&statusComponent.ProgressDetails, progressDetail)
	}

	// The deployment controller will not watch the cleaning events of the old replicaSet pods.
	// so when component status is completed, we should forward the progressDetails to succeed.
	markStatelessPodsSucceed := false
	if pgRes.clusterComponentDef.WorkloadType == appsv1alpha1.Stateless &&
		util.IsCompleted(statusComponent.Phase) {
		markStatelessPodsSucceed = true
	}

	for _, v := range statusComponent.ProgressDetails {
		if _, ok := podMap[v.ObjectKey]; ok && !markStatelessPodsSucceed {
			continue
		}
		// if the pod object of progressDetail is not existing in podMap, means successfully deleted.
		progressDetail := appsv1alpha1.ProgressDetail{
			ObjectKey: v.ObjectKey,
			Status:    appsv1alpha1.SucceedProgressStatus,
			Message:   fmt.Sprintf("Successfully deleted pod: %s in Component: %s", v.ObjectKey, pgRes.clusterComponent.Name),
		}
		succeedCount += 1
		SetStatusComponentProgressDetail(opsRes.Recorder, opsRes.OpsRequest,
			&statusComponent.ProgressDetails, progressDetail)
	}
	return succeedCount, nil
}
