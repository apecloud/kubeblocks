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
	"context"
	"fmt"
	"math"
	"reflect"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type handleStatusProgressWithComponent func(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (expectProgressCount int32, succeedCount int32, err error)

type handleReconfigureOpsStatus func(cmStatus *appsv1alpha1.ConfigurationStatus) error

// reconcileActionWithComponentOps will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// if OpsRequest.spec.componentOps is not null, you can use it to OpsBehaviour.ReconcileAction.
// return the OpsRequest.status.phase
func reconcileActionWithComponentOps(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	opsMessageKey string,
	handleStatusProgress handleStatusProgressWithComponent,
) (appsv1alpha1.OpsPhase, time.Duration, error) {
	if opsRes == nil {
		return "", 0, nil
	}
	opsRequestPhase := appsv1alpha1.OpsRunningPhase
	clusterDef, err := getClusterDefByName(reqCtx.Ctx, cli,
		opsRes.Cluster.Spec.ClusterDefRef)
	if err != nil {
		return opsRequestPhase, 0, err
	}
	var (
		opsRequest               = opsRes.OpsRequest
		isFailed                 bool
		ok                       bool
		expectProgressCount      int32
		completedProgressCount   int32
		checkAllClusterComponent bool
	)
	componentNameMap := opsRequest.GetComponentNameSet()
	// if no specified components, we should check the all components phase of cluster.
	if len(componentNameMap) == 0 {
		checkAllClusterComponent = true
	}
	patch := client.MergeFrom(opsRequest.DeepCopy())
	oldOpsRequestStatus := opsRequest.Status.DeepCopy()
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = map[string]appsv1alpha1.OpsRequestComponentStatus{}
	}
	opsIsCompleted := opsRequestHasProcessed(*opsRes)
	for k, v := range opsRes.Cluster.Status.Components {
		if _, ok = componentNameMap[k]; !ok && !checkAllClusterComponent {
			continue
		}
		if util.IsFailedOrAbnormal(v.Phase) {
			isFailed = true
		}
		var compStatus appsv1alpha1.OpsRequestComponentStatus
		if compStatus, ok = opsRequest.Status.Components[k]; !ok {
			compStatus = appsv1alpha1.OpsRequestComponentStatus{}
		}
		if compStatus.Phase != v.Phase {
			compStatus.Phase = v.Phase
		}
		clusterComponent := opsRes.Cluster.Spec.GetComponentByName(k)
		expectCount, completedCount, err := handleStatusProgress(reqCtx, cli, opsRes, progressResource{
			opsMessageKey:       opsMessageKey,
			clusterComponent:    clusterComponent,
			clusterComponentDef: clusterDef.GetComponentDefByName(clusterComponent.ComponentDefRef),
			opsIsCompleted:      opsIsCompleted,
		}, &compStatus)
		if err != nil {
			if intctrlutil.IsTargetError(err, intctrlutil.ErrorWaitCacheRefresh) {
				return opsRequestPhase, time.Second, nil
			}
			return opsRequestPhase, 0, err
		}
		expectProgressCount += expectCount
		completedProgressCount += completedCount
		opsRequest.Status.Components[k] = compStatus
	}
	opsRequest.Status.Progress = fmt.Sprintf("%d/%d", completedProgressCount, expectProgressCount)
	if !reflect.DeepEqual(opsRequest.Status, oldOpsRequestStatus) {
		if err = cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return opsRequestPhase, 0, err
		}
	}
	if opsRes.ToClusterPhase == opsRes.Cluster.Status.Phase {
		// wait for the cluster to finish processing ops.
		return opsRequestPhase, 0, nil
	}
	// TODO: judge whether ops is Failed according to whether progressDetail has failed pods.
	// now we check the ops is Failed by the component phase, it may be not accurate during h-scale replicas.
	if isFailed && opsRes.Cluster.Status.ObservedGeneration >= opsRes.OpsRequest.Status.ClusterGeneration {
		return appsv1alpha1.OpsFailedPhase, 0, nil
	}
	if completedProgressCount != expectProgressCount {
		return opsRequestPhase, time.Second, nil
	}
	return appsv1alpha1.OpsSucceedPhase, 0, nil
}

// opsRequestHasProcessed checks if the opsRequest has processed.
func opsRequestHasProcessed(opsRes OpsResource) bool {
	return opsRes.ToClusterPhase != opsRes.Cluster.Status.Phase &&
		opsRes.Cluster.Status.ObservedGeneration >= opsRes.OpsRequest.Status.ClusterGeneration
}

// getClusterDefByName gets the ClusterDefinition object by the name.
func getClusterDefByName(ctx context.Context, cli client.Client, clusterDefName string) (*appsv1alpha1.ClusterDefinition, error) {
	clusterDef := &appsv1alpha1.ClusterDefinition{}
	if err := cli.Get(ctx, client.ObjectKey{Name: clusterDefName}, clusterDef); err != nil {
		return nil, err
	}
	return clusterDef, nil
}

// opsRequestIsCompleted checks if OpsRequest is completed
func opsRequestIsCompleted(phase appsv1alpha1.OpsPhase) bool {
	return slices.Index([]appsv1alpha1.OpsPhase{appsv1alpha1.OpsFailedPhase, appsv1alpha1.OpsSucceedPhase}, phase) != -1
}

func patchOpsStatusWithOpsDeepCopy(ctx context.Context,
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
		// provide an event
		eventType := corev1.EventTypeNormal
		if phase == appsv1alpha1.OpsFailedPhase {
			eventType = corev1.EventTypeWarning
		}
		opsRes.Recorder.Event(opsRequest, eventType, v.Reason, v.Message)
	}
	if opsRequestIsCompleted(phase) {
		opsRequest.Status.CompletionTimestamp = metav1.Time{Time: time.Now()}
		// when OpsRequest is completed, do it
		if err := DeleteOpsRequestAnnotationInCluster(ctx, cli, opsRes); err != nil {
			return err
		}
	}
	if phase == appsv1alpha1.OpsCreatingPhase && opsRequest.Status.Phase != phase {
		opsRequest.Status.StartTimestamp = metav1.Time{Time: time.Now()}
	}
	opsRequest.Status.Phase = phase
	return cli.Status().Patch(ctx, opsRequest, patch)
}

// PatchOpsStatus patches OpsRequest.status
func PatchOpsStatus(ctx context.Context,
	cli client.Client,
	opsRes *OpsResource,
	phase appsv1alpha1.OpsPhase,
	condition ...*metav1.Condition) error {
	return patchOpsStatusWithOpsDeepCopy(ctx, cli, opsRes, opsRes.OpsRequest.DeepCopy(), phase, condition...)
}

// PatchClusterNotFound patches ClusterNotFound condition to the OpsRequest.status.conditions.
func PatchClusterNotFound(ctx context.Context, cli client.Client, opsRes *OpsResource) error {
	message := fmt.Sprintf("spec.clusterRef %s is not found", opsRes.OpsRequest.Spec.ClusterRef)
	condition := appsv1alpha1.NewValidateFailedCondition(appsv1alpha1.ReasonClusterNotFound, message)
	return PatchOpsStatus(ctx, cli, opsRes, appsv1alpha1.OpsFailedPhase, condition)
}

// patchOpsHandlerNotSupported patches OpsNotSupported condition to the OpsRequest.status.conditions.
func patchOpsHandlerNotSupported(ctx context.Context, cli client.Client, opsRes *OpsResource) error {
	message := fmt.Sprintf("spec.type %s is not supported by operator", opsRes.OpsRequest.Spec.Type)
	condition := appsv1alpha1.NewValidateFailedCondition(appsv1alpha1.ReasonOpsTypeNotSupported, message)
	return PatchOpsStatus(ctx, cli, opsRes, appsv1alpha1.OpsFailedPhase, condition)
}

// patchValidateErrorCondition patches ValidateError condition to the OpsRequest.status.conditions.
func patchValidateErrorCondition(ctx context.Context, cli client.Client, opsRes *OpsResource, errMessage string) error {
	condition := appsv1alpha1.NewValidateFailedCondition(appsv1alpha1.ReasonValidateFailed, errMessage)
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
func patchOpsRequestToCreating(ctx context.Context,
	cli client.Client,
	opsRes *OpsResource,
	opsDeepCoy *appsv1alpha1.OpsRequest,
	opsHandler OpsHandler) error {
	var condition *metav1.Condition
	validatePassCondition := appsv1alpha1.NewValidatePassedCondition(opsRes.OpsRequest.Name)
	condition = opsHandler.ActionStartedCondition(opsRes.OpsRequest)
	return patchOpsStatusWithOpsDeepCopy(ctx, cli, opsRes, opsDeepCoy, appsv1alpha1.OpsCreatingPhase, validatePassCondition, condition)
}

// patchClusterStatusAndRecordEvent records the ops event in the cluster and
// will update the cluster status if the ops need to maintain cluster status by self.
func patchClusterStatusAndRecordEvent(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	opsBehaviour OpsBehaviour) error {
	sendStartOpsRequestEvent := func() {
		opsRes.Recorder.Eventf(opsRes.Cluster, corev1.EventTypeNormal, string(opsRes.OpsRequest.Spec.Type),
			`Start to process the %s opsRequest "%s" in Cluster: %s`, opsRes.OpsRequest.Spec.Type,
			opsRes.OpsRequest.Name, opsRes.Cluster.Name)
	}
	if opsBehaviour.ToClusterPhase == "" || !opsBehaviour.MaintainClusterPhaseBySelf {
		sendStartOpsRequestEvent()
		return nil
	}
	patch := client.MergeFrom(opsRes.Cluster.DeepCopy())
	opsRes.Cluster.Status.Phase = opsBehaviour.ToClusterPhase
	// update cluster.status.components phase
	realChangeCompMap := opsBehaviour.OpsHandler.GetRealAffectedComponentMap(opsRes.OpsRequest)
	for k := range realChangeCompMap {
		if compStatus, ok := opsRes.Cluster.Status.Components[k]; ok {
			compStatus.Phase = appsv1alpha1.SpecReconcilingClusterCompPhase
			opsRes.Cluster.Status.SetComponentStatus(k, compStatus)
		}
	}
	if err := cli.Status().Patch(reqCtx.Ctx, opsRes.Cluster, patch); err != nil {
		return err
	}
	sendStartOpsRequestEvent()
	return nil
}

// DeleteOpsRequestAnnotationInCluster when OpsRequest.status.phase is Succeed or Failed
// we should delete the OpsRequest Annotation in cluster, unlock cluster
func DeleteOpsRequestAnnotationInCluster(ctx context.Context, cli client.Client, opsRes *OpsResource) error {
	var (
		opsRequestSlice []appsv1alpha1.OpsRecorder
		err             error
	)
	if opsRequestSlice, err = opsutil.GetOpsRequestSliceFromCluster(opsRes.Cluster); err != nil {
		return err
	}
	index, opsRecord := GetOpsRecorderFromSlice(opsRequestSlice, opsRes.OpsRequest.Name)
	if opsRecord.Name == "" {
		return nil
	}
	// delete the opsRequest information in Cluster.annotations
	opsRequestSlice = slices.Delete(opsRequestSlice, index, index+1)
	return opsutil.PatchClusterOpsAnnotations(ctx, cli, opsRes.Cluster, opsRequestSlice)
}

// addOpsRequestAnnotationToCluster adds the OpsRequest Annotation to Cluster.metadata.Annotations to acquire the lock.
func addOpsRequestAnnotationToCluster(ctx context.Context, cli client.Client, opsRes *OpsResource, opsBehaviour OpsBehaviour) error {
	var (
		opsRequestSlice []appsv1alpha1.OpsRecorder
		err             error
	)
	if opsBehaviour.ToClusterPhase == "" {
		return nil
	}
	// if the running opsRequest is deleted, do not patch the opsRequest annotation on cluster.
	if !opsRes.OpsRequest.DeletionTimestamp.IsZero() {
		return nil
	}
	if opsRequestSlice, err = opsutil.GetOpsRequestSliceFromCluster(opsRes.Cluster); err != nil {
		return err
	}
	// check the OpsRequest is existed
	if _, opsRecorder := GetOpsRecorderFromSlice(opsRequestSlice, opsRes.OpsRequest.Name); opsRecorder.Name != "" {
		return nil
	}
	if opsRequestSlice == nil {
		opsRequestSlice = make([]appsv1alpha1.OpsRecorder, 0)
	}
	opsRequestSlice = append(opsRequestSlice, appsv1alpha1.OpsRecorder{
		Name: opsRes.OpsRequest.Name,
		Type: opsRes.OpsRequest.Spec.Type,
	})
	return opsutil.UpdateClusterOpsAnnotations(ctx, cli, opsRes.Cluster, opsRequestSlice)
}

// isOpsRequestFailedPhase checks the OpsRequest phase is Failed
func isOpsRequestFailedPhase(opsRequestPhase appsv1alpha1.OpsPhase) bool {
	return opsRequestPhase == appsv1alpha1.OpsFailedPhase
}

func updateReconfigureStatusByCM(reconfiguringStatus *appsv1alpha1.ReconfiguringStatus, tplName string,
	handleReconfigureStatus handleReconfigureOpsStatus) error {
	for i, cmStatus := range reconfiguringStatus.ConfigurationStatus {
		if cmStatus.Name == tplName {
			// update cmStatus
			return handleReconfigureStatus(&reconfiguringStatus.ConfigurationStatus[i])
		}
	}
	cmCount := len(reconfiguringStatus.ConfigurationStatus)
	reconfiguringStatus.ConfigurationStatus = append(reconfiguringStatus.ConfigurationStatus, appsv1alpha1.ConfigurationStatus{
		Name:   tplName,
		Status: appsv1alpha1.ReasonReconfigureMerging,
	})
	cmStatus := &reconfiguringStatus.ConfigurationStatus[cmCount]
	return handleReconfigureStatus(cmStatus)
}

// patchReconfigureOpsStatus when Reconfigure is running, we should update status to OpsRequest.Status.ConfigurationStatus.
//
// NOTES:
// opsStatus describes status of OpsRequest.
// reconfiguringStatus describes status of reconfiguring operation, which contains multi configuration templates.
// cmStatus describes status of configmap, it is uniquely associated with a configuration template, which contains multi key, each key represents name of a configuration file.
// execStatus describes the result of the execution of the state machine, which is designed to solve how to do the reconfiguring operation, such as whether to restart, how to send a signal to the process.
func patchReconfigureOpsStatus(
	opsRes *OpsResource,
	tplName string,
	handleReconfigureStatus handleReconfigureOpsStatus) error {
	var opsRequest = opsRes.OpsRequest
	if opsRequest.Status.ReconfiguringStatus == nil {
		opsRequest.Status.ReconfiguringStatus = &appsv1alpha1.ReconfiguringStatus{
			ConfigurationStatus: make([]appsv1alpha1.ConfigurationStatus, 0),
		}
	}

	reconfiguringStatus := opsRequest.Status.ReconfiguringStatus
	return updateReconfigureStatusByCM(reconfiguringStatus, tplName, handleReconfigureStatus)
}

// getSlowestReconfiguringProgress calculate the progress of the reconfiguring operations.
func getSlowestReconfiguringProgress(status []appsv1alpha1.ConfigurationStatus) string {
	slowest := appsv1alpha1.ConfigurationStatus{
		SucceedCount:  math.MaxInt32,
		ExpectedCount: -1,
	}

	for _, st := range status {
		if st.SucceedCount < slowest.SucceedCount {
			slowest = st
		}
	}
	return fmt.Sprintf("%d/%d", slowest.SucceedCount, slowest.ExpectedCount)
}
