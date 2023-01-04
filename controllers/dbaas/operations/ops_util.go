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
	"context"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ReconcileActionWithCluster it will be performed when action is done and loop util OpsRequest.status.phase is Succeed/Failed.
// if OpsRequest is a cluster scope operation, you can use it to OpsBehaviour.ReconcileAction, such as Upgrade.
// return the OpsRequest.status.phase
func ReconcileActionWithCluster(opsRes *OpsResource) (dbaasv1alpha1.Phase, time.Duration, error) {
	var (
		opsRequest      = opsRes.OpsRequest
		isChanged       bool
		opsRequestPhase = dbaasv1alpha1.RunningPhase
		requeueAfter    time.Duration
	)
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = map[string]dbaasv1alpha1.OpsRequestStatusComponent{}
	}
	for k, v := range opsRes.Cluster.Status.Components {
		// the operation occurs in the cluster, such as upgrade.
		// However, it is also possible that only the corresponding components have changed,
		// and the phase is updating. so we need to monitor these components and send the corresponding event
		if statusComponent, ok := opsRequest.Status.Components[k]; (!ok && v.Phase == opsRes.Cluster.Status.Phase) || statusComponent.Phase != v.Phase {
			isChanged = true
			opsRequest.Status.Components[k] = dbaasv1alpha1.OpsRequestStatusComponent{Phase: v.Phase}
			sendEventWhenComponentPhaseChanged(opsRes, k, &v)
		}
	}
	if isChanged {
		if err := opsRes.Client.Status().Patch(opsRes.Ctx, opsRequest, patch); err != nil {
			return opsRequestPhase, requeueAfter, err
		}
	}
	switch opsRes.Cluster.Status.Phase {
	case dbaasv1alpha1.RunningPhase:
		opsRequestPhase = dbaasv1alpha1.SucceedPhase
	case dbaasv1alpha1.FailedPhase, dbaasv1alpha1.AbnormalPhase:
		opsRequestPhase = dbaasv1alpha1.FailedPhase
	}
	return opsRequestPhase, requeueAfter, nil
}

// ReconcileActionWithComponentOps it will be performed when action is done and loop util OpsRequest.status.phase is Succeed/Failed.
// if OpsRequest.spec.componentOps is not null, you can use it to OpsBehaviour.ReconcileAction.
// return the OpsRequest.status.phase
func ReconcileActionWithComponentOps(opsRes *OpsResource) (dbaasv1alpha1.Phase, time.Duration, error) {
	var (
		opsRequest      = opsRes.OpsRequest
		isCompleted     = true
		isChanged       bool
		isFailed        bool
		opsRequestPhase = dbaasv1alpha1.RunningPhase
	)
	componentNameMap := opsRequest.GetComponentNameMap()
	if len(componentNameMap) == 0 {
		return opsRequestPhase, 0, nil
	}
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = map[string]dbaasv1alpha1.OpsRequestStatusComponent{}
	}
	for k, v := range opsRes.Cluster.Status.Components {
		if _, ok := componentNameMap[k]; !ok {
			continue
		}
		if !util.IsCompleted(v.Phase) {
			isCompleted = false
		}
		if util.IsFailedOrAbnormal(v.Phase) {
			isFailed = true
		}
		if statusComponent, ok := opsRequest.Status.Components[k]; !ok || statusComponent.Phase != v.Phase {
			isChanged = true
			opsRequest.Status.Components[k] = dbaasv1alpha1.OpsRequestStatusComponent{Phase: v.Phase}
			sendEventWhenComponentPhaseChanged(opsRes, k, &v)
		}
	}
	if isChanged {
		if err := opsRes.Client.Status().Patch(opsRes.Ctx, opsRequest, patch); err != nil {
			return opsRequestPhase, 0, err
		}
	}
	if isFailed {
		opsRequestPhase = dbaasv1alpha1.FailedPhase
	} else if isCompleted {
		opsRequestPhase = dbaasv1alpha1.SucceedPhase
	}
	return opsRequestPhase, 0, nil
}

// opsRequestIsCompleted check OpsRequest is completed
func opsRequestIsCompleted(phase dbaasv1alpha1.Phase) bool {
	return slices.Index([]dbaasv1alpha1.Phase{dbaasv1alpha1.FailedPhase, dbaasv1alpha1.SucceedPhase}, phase) != -1
}

// sendEventWhenComponentStatusChanged send an event when OpsRequest.status.components[*].phase is changed
func sendEventWhenComponentPhaseChanged(opsRes *OpsResource, componentName string, statusComponent *dbaasv1alpha1.ClusterStatusComponent) {
	var (
		tip          string
		reason       = dbaasv1alpha1.ReasonStarting
		eventType    = corev1.EventTypeNormal
		extraMessage string
	)

	switch statusComponent.Phase {
	// component is running
	case dbaasv1alpha1.RunningPhase:
		tip = "Successfully"
		reason = dbaasv1alpha1.ReasonSuccessful
		// component is failed
	case dbaasv1alpha1.FailedPhase, dbaasv1alpha1.AbnormalPhase:
		tip = "Failed"
		reason = dbaasv1alpha1.ReasonComponentFailed
		eventType = corev1.EventTypeWarning
		for k, v := range statusComponent.Message {
			extraMessage += fmt.Sprintf("%s:%s", k, v) + ";"
		}
	}
	message := fmt.Sprintf("%s %s component: %s in Cluster: %s%s",
		tip, opsRes.OpsRequest.Spec.Type, componentName, opsRes.OpsRequest.Spec.ClusterRef, extraMessage)
	opsRes.Recorder.Event(opsRes.OpsRequest, eventType, reason, message)
}

// PatchOpsStatus patch OpsRequest.status
func PatchOpsStatus(opsRes *OpsResource,
	phase dbaasv1alpha1.Phase,
	condition ...*metav1.Condition) error {

	opsRequest := opsRes.OpsRequest
	patch := client.MergeFrom(opsRequest.DeepCopy())
	for _, v := range condition {
		if v == nil {
			continue
		}
		opsRequest.SetStatusCondition(*v)
		// provide an event
		eventType := corev1.EventTypeNormal
		if phase == dbaasv1alpha1.FailedPhase {
			eventType = corev1.EventTypeWarning
		}
		opsRes.Recorder.Event(opsRequest, eventType, v.Reason, v.Message)
	}
	if opsRequestIsCompleted(phase) {
		opsRequest.Status.CompletionTimestamp = &metav1.Time{Time: time.Now()}
		// when OpsRequest is completed, do it
		if err := deleteOpsRequestAnnotationInCluster(opsRes); err != nil {
			return err
		}
	}
	if phase == dbaasv1alpha1.RunningPhase && opsRequest.Status.Phase != phase {
		opsRequest.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
	}
	opsRequest.Status.Phase = phase
	return opsRes.Client.Status().Patch(opsRes.Ctx, opsRequest, patch)
}

func PatchClusterNotFound(opsRes *OpsResource) error {
	message := fmt.Sprintf("spec.clusterRef %s is not Found", opsRes.OpsRequest.Spec.ClusterRef)
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonClusterNotFound, message)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

func patchOpsBehaviourNotFound(opsRes *OpsResource) error {
	message := fmt.Sprintf("spec.type %s is not supported", opsRes.OpsRequest.Spec.Type)
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonOpsTypeNotSupported, message)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

func patchClusterPhaseMisMatch(opsRes *OpsResource) error {
	message := fmt.Sprintf("can not run the OpsRequest when Cluster.status.phase is %s in spec.clusterRef: %s",
		opsRes.Cluster.Status.Phase, opsRes.Cluster.Name)
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonClusterPhaseMisMatch, message)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

func patchClusterExistOtherOperation(opsRes *OpsResource, opsRequestName string) error {
	message := fmt.Sprintf("Existing OpsRequest: %s is running in Cluster: %s, handle this OpsRequest first",
		opsRequestName, opsRes.Cluster.Name)
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonClusterExistOtherOperation, message)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

// GetOpsRequestSliceFromCluster get OpsRequest slice from cluster annotations.
// this record what OpsRequests are running in cluster
func GetOpsRequestSliceFromCluster(cluster *dbaasv1alpha1.Cluster) ([]dbaasv1alpha1.OpsRecorder, error) {
	var (
		opsRequestValue string
		opsRequestSlice []dbaasv1alpha1.OpsRecorder
		ok              bool
	)
	if cluster == nil || cluster.Annotations == nil {
		return nil, nil
	}
	if opsRequestValue, ok = cluster.Annotations[intctrlutil.OpsRequestAnnotationKey]; !ok {
		return nil, nil
	}
	// opsRequest annotation value in cluster to slice
	if err := json.Unmarshal([]byte(opsRequestValue), &opsRequestSlice); err != nil {
		return nil, err
	}
	return opsRequestSlice, nil
}

// getOpsRequestNameFromAnnotation get OpsRequest.name from cluster.annotations
func getOpsRequestNameFromAnnotation(cluster *dbaasv1alpha1.Cluster, toClusterPhase dbaasv1alpha1.Phase) string {
	opsRequestSlice, _ := GetOpsRequestSliceFromCluster(cluster)
	opsRecorder := getOpsRecorderWithClusterPhase(opsRequestSlice, toClusterPhase)
	return opsRecorder.Name
}

// getOpsRecorderWithClusterPhase get OpsRequest recorder from slice by target cluster phase
func getOpsRecorderWithClusterPhase(opsRequestSlice []dbaasv1alpha1.OpsRecorder,
	toClusterPhase dbaasv1alpha1.Phase) dbaasv1alpha1.OpsRecorder {
	for _, v := range opsRequestSlice {
		if v.ToClusterPhase == toClusterPhase {
			return v
		}
	}
	return dbaasv1alpha1.OpsRecorder{}
}

// GetOpsRecorderFromSlice get OpsRequest recorder from slice by target cluster phase
func GetOpsRecorderFromSlice(opsRequestSlice []dbaasv1alpha1.OpsRecorder,
	opsRequestName string) (int, dbaasv1alpha1.OpsRecorder) {
	for i, v := range opsRequestSlice {
		if v.Name == opsRequestName {
			return i, v
		}
	}
	return 0, dbaasv1alpha1.OpsRecorder{}
}

// patchOpsRequestToRunning patch OpsRequest.status.phase to Running
func patchOpsRequestToRunning(opsRes *OpsResource, opsBehaviour *OpsBehaviour) error {
	var condition *metav1.Condition
	validatePassCondition := dbaasv1alpha1.NewValidatePassedCondition(opsRes.OpsRequest.Name)
	if opsBehaviour.ActionStartedCondition != nil {
		condition = opsBehaviour.ActionStartedCondition(opsRes.OpsRequest)
	}
	return PatchOpsStatus(opsRes, dbaasv1alpha1.RunningPhase, validatePassCondition, condition)
}

// patchClusterStatus update Cluster.status to record cluster and components information
func patchClusterStatus(opsRes *OpsResource, toClusterState dbaasv1alpha1.Phase) error {
	if toClusterState == "" {
		return nil
	}
	patch := client.MergeFrom(opsRes.Cluster.DeepCopy())
	opsRes.Cluster.Status.Phase = toClusterState
	componentNameMap := opsRes.OpsRequest.GetComponentNameMap()
	// if the OpsRequest is components scope, we should update the cluster components together.
	// otherwise, OpsRequest maybe reconcile the status to succeed immediately.
	if componentNameMap != nil && opsRes.Cluster.Status.Components != nil {
		for k, v := range opsRes.Cluster.Status.Components {
			if _, ok := componentNameMap[k]; ok {
				v.Phase = toClusterState
				opsRes.Cluster.Status.Components[k] = v
			}
		}
	}
	if err := opsRes.Client.Status().Patch(opsRes.Ctx, opsRes.Cluster, patch); err != nil {
		return err
	}
	opsRes.Recorder.Eventf(opsRes.Cluster, corev1.EventTypeNormal, string(opsRes.OpsRequest.Spec.Type),
		"Start %s in Cluster: %s", opsRes.OpsRequest.Spec.Type, opsRes.Cluster.Name)
	return nil
}

// deleteOpsRequestAnnotationInCluster when OpsRequest.status.phase is Succeed or Failed
// we should delete the OpsRequest Annotation in cluster, unlock cluster
func deleteOpsRequestAnnotationInCluster(opsRes *OpsResource) error {
	var (
		opsRequestSlice []dbaasv1alpha1.OpsRecorder
		err             error
	)
	if opsRequestSlice, err = GetOpsRequestSliceFromCluster(opsRes.Cluster); err != nil {
		return err
	}
	index, opsRecord := GetOpsRecorderFromSlice(opsRequestSlice, opsRes.OpsRequest.Name)
	if opsRecord.Name == "" {
		return nil
	}
	// delete the opsRequest information in Cluster.annotations
	opsRequestSlice = slices.Delete(opsRequestSlice, index, index+1)
	if err = patchClusterPhaseWhenExistsOtherOps(opsRes, opsRequestSlice); err != nil {
		return err
	}
	return PatchClusterOpsAnnotations(opsRes.Ctx, opsRes.Client, opsRes.Cluster, opsRequestSlice)
}

// addOpsRequestAnnotationToCluster when OpsRequest.phase is Running, we should add the OpsRequest Annotation to Cluster.metadata.Annotations
func addOpsRequestAnnotationToCluster(opsRes *OpsResource, toClusterPhase dbaasv1alpha1.Phase) error {
	var (
		opsRequestSlice []dbaasv1alpha1.OpsRecorder
		err             error
	)
	if toClusterPhase == "" {
		return nil
	}
	if opsRequestSlice, err = GetOpsRequestSliceFromCluster(opsRes.Cluster); err != nil {
		return err
	}
	// check the OpsRequest is existed
	if _, opsRecorder := GetOpsRecorderFromSlice(opsRequestSlice, opsRes.OpsRequest.Name); opsRecorder.Name != "" {
		return nil
	}
	if opsRequestSlice == nil {
		opsRequestSlice = make([]dbaasv1alpha1.OpsRecorder, 0)
	}
	opsRequestSlice = append(opsRequestSlice, dbaasv1alpha1.OpsRecorder{
		Name:           opsRes.OpsRequest.Name,
		ToClusterPhase: toClusterPhase,
	})
	return PatchClusterOpsAnnotations(opsRes.Ctx, opsRes.Client, opsRes.Cluster, opsRequestSlice)
}

// PatchClusterOpsAnnotations patch OpsRequest annotation in Cluster.annotations
func PatchClusterOpsAnnotations(ctx context.Context,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	opsRequestSlice []dbaasv1alpha1.OpsRecorder) error {
	patch := client.MergeFrom(cluster.DeepCopy())
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	if len(opsRequestSlice) > 0 {
		result, _ := json.Marshal(opsRequestSlice)
		cluster.Annotations[intctrlutil.OpsRequestAnnotationKey] = string(result)
	} else {
		delete(cluster.Annotations, intctrlutil.OpsRequestAnnotationKey)
	}
	return cli.Patch(ctx, cluster, patch)
}

// patchClusterPhaseWhenExistsOtherOps
func patchClusterPhaseWhenExistsOtherOps(opsRes *OpsResource, opsRequestSlice []dbaasv1alpha1.OpsRecorder) error {
	// If there are other OpsRequests running, modify the cluster.status.phase with other opsRequest's ToClusterPhase
	if len(opsRequestSlice) == 0 {
		return nil
	}
	patch := client.MergeFrom(opsRes.Cluster.DeepCopy())
	opsRes.Cluster.Status.Phase = opsRequestSlice[0].ToClusterPhase
	if err := opsRes.Client.Status().Patch(opsRes.Ctx, opsRes.Cluster, patch); err != nil {
		return err
	}
	return nil
}

// isOpsRequestFailedPhase check the OpsRequest phase is Failed
func isOpsRequestFailedPhase(opsRequestPhase dbaasv1alpha1.Phase) bool {
	return opsRequestPhase == dbaasv1alpha1.FailedPhase
}

// PatchOpsRequestAnnotation patch the reconcile annotation to OpsRequest
func PatchOpsRequestAnnotation(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster, opsRequestName string) error {
	opsRequest := &dbaasv1alpha1.OpsRequest{}
	if err := cli.Get(ctx, client.ObjectKey{Name: opsRequestName, Namespace: cluster.Namespace}, opsRequest); err != nil {
		return err
	}
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Annotations == nil {
		opsRequest.Annotations = map[string]string{}
	}
	opsRequest.Annotations[intctrlutil.OpsRequestReconcileAnnotationKey] = time.Now().Format(time.RFC3339Nano)
	return cli.Patch(ctx, opsRequest, patch)
}

// MarkRunningOpsRequestAnnotation mark reconcile annotation to the OpsRequest which is running in the cluster.
// then the related OpsRequest can reconcile
func MarkRunningOpsRequestAnnotation(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster) error {
	var (
		opsRequestSlice []dbaasv1alpha1.OpsRecorder
		err             error
	)
	if opsRequestSlice, err = GetOpsRequestSliceFromCluster(cluster); err != nil {
		return err
	}
	// mark annotation for operations
	var notExistOps = map[string]struct{}{}
	for _, v := range opsRequestSlice {
		if err = PatchOpsRequestAnnotation(ctx, cli, cluster, v.Name); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if apierrors.IsNotFound(err) {
			notExistOps[v.Name] = struct{}{}
		}
	}
	if len(notExistOps) != 0 {
		return removeClusterInvalidOpsRequestAnnotation(ctx, cli, cluster, opsRequestSlice, notExistOps)
	}
	return nil
}

// removeClusterInvalidOpsRequestAnnotation delete the OpsRequest annotation in cluster when the OpsRequest not existing.
func removeClusterInvalidOpsRequestAnnotation(
	ctx context.Context,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	opsRequestSlice []dbaasv1alpha1.OpsRecorder,
	notExistOps map[string]struct{}) error {
	// delete the OpsRequest annotation in cluster when the OpsRequest not existing.
	newOpsRequestSlice := make([]dbaasv1alpha1.OpsRecorder, 0, len(opsRequestSlice))
	for _, v := range opsRequestSlice {
		if _, ok := notExistOps[v.Name]; ok {
			continue
		}
		newOpsRequestSlice = append(newOpsRequestSlice, v)
	}
	return PatchClusterOpsAnnotations(ctx, cli, cluster, newOpsRequestSlice)
}
