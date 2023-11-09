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
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// componentFailedTimeout when the duration of component failure exceeds this threshold, it is determined that opsRequest has failed
const componentFailedTimeout = 30 * time.Second

var _ error = &WaitForClusterPhaseErr{}

type WaitForClusterPhaseErr struct {
	clusterName   string
	currentPhase  appsv1alpha1.ClusterPhase
	expectedPhase []appsv1alpha1.ClusterPhase
}

func (e *WaitForClusterPhaseErr) Error() string {
	return fmt.Sprintf("wait for cluster %s to reach phase %v, current status is :%s", e.clusterName, e.expectedPhase, e.currentPhase)
}

type handleStatusProgressWithComponent func(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes progressResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus) (expectProgressCount int32, succeedCount int32, err error)

type handleReconfigureOpsStatus func(cmStatus *appsv1alpha1.ConfigurationItemStatus) error

// reconcileActionWithComponentOps will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the common function to reconcile opsRequest status when the opsRequest will affect the lifecycle of the components.
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
		requeueTimeAfterFailed   time.Duration
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
	opsIsCompleted := opsRequestHasProcessed(reqCtx, cli, *opsRes)
	for k, v := range opsRes.Cluster.Status.Components {
		if _, ok = componentNameMap[k]; !ok && !checkAllClusterComponent {
			continue
		}
		var compStatus appsv1alpha1.OpsRequestComponentStatus
		if compStatus, ok = opsRequest.Status.Components[k]; !ok {
			compStatus = appsv1alpha1.OpsRequestComponentStatus{}
		}
		lastFailedTime := compStatus.LastFailedTime
		if components.IsFailedOrAbnormal(v.Phase) {
			isFailed = true
			if lastFailedTime.IsZero() {
				lastFailedTime = metav1.Now()
			}
			if time.Now().Before(lastFailedTime.Add(componentFailedTimeout)) {
				requeueTimeAfterFailed = componentFailedTimeout - time.Since(lastFailedTime.Time)
			}
		} else if !lastFailedTime.IsZero() {
			// reset lastFailedTime if component is not failed
			lastFailedTime = metav1.Time{}
		}
		if compStatus.Phase != v.Phase {
			compStatus.Phase = v.Phase
			compStatus.LastFailedTime = lastFailedTime
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
	if !reflect.DeepEqual(opsRequest.Status, *oldOpsRequestStatus) {
		if err = cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return opsRequestPhase, 0, err
		}
	}
	// check if the cluster has applied the changes of the opsRequest and wait for the cluster to finish processing the ops.
	if !opsIsCompleted {
		return opsRequestPhase, 0, nil
	}

	if isFailed {
		if requeueTimeAfterFailed != 0 {
			// component failure may be temporary, waiting for component failure timeout.
			return opsRequestPhase, requeueTimeAfterFailed, nil
		}
		return appsv1alpha1.OpsFailedPhase, 0, nil
	}
	if completedProgressCount != expectProgressCount {
		return opsRequestPhase, time.Second, nil
	}
	return appsv1alpha1.OpsSucceedPhase, 0, nil
}

// opsRequestHasProcessed checks if the opsRequest has been processed.
func opsRequestHasProcessed(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes OpsResource) bool {
	if opsRes.ToClusterPhase == opsRes.Cluster.Status.Phase {
		return false
	}
	// if all pods of all components are with latest revision, ops has processed
	rsmList := &workloads.ReplicatedStateMachineList{}
	if err := cli.List(reqCtx.Ctx, rsmList,
		client.InNamespace(opsRes.Cluster.Namespace),
		client.MatchingLabels{constant.AppInstanceLabelKey: opsRes.Cluster.Name}); err != nil {
		return false
	}
	for _, rsm := range rsmList.Items {
		isLatestRevision, err := components.IsComponentPodsWithLatestRevision(reqCtx.Ctx, cli, opsRes.Cluster, &rsm)
		if err != nil {
			return false
		}
		if !isLatestRevision {
			return false
		}
	}
	return true
}

// getClusterDefByName gets the ClusterDefinition object by the name.
func getClusterDefByName(ctx context.Context, cli client.Client, clusterDefName string) (*appsv1alpha1.ClusterDefinition, error) {
	clusterDef := &appsv1alpha1.ClusterDefinition{}
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
	if opsRequest.IsComplete(phase) {
		opsRequest.Status.CompletionTimestamp = metav1.Time{Time: time.Now()}
		// when OpsRequest is completed, remove it from annotation
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
	return PatchOpsStatusWithOpsDeepCopy(ctx, cli, opsRes, opsRes.OpsRequest.DeepCopy(), phase, condition...)
}

// PatchClusterNotFound patches ClusterNotFound condition to the OpsRequest.status.conditions.
func PatchClusterNotFound(ctx context.Context, cli client.Client, opsRes *OpsResource) error {
	message := fmt.Sprintf("spec.clusterRef %s is not found", opsRes.OpsRequest.Spec.ClusterRef)
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

// patchFastFailErrorCondition patches a new failed condition to the OpsRequest.status.conditions.
func patchFastFailErrorCondition(ctx context.Context, cli client.Client, opsRes *OpsResource, err error) error {
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

// DeleteOpsRequestAnnotationInCluster when OpsRequest.status.phase is Succeeded or Failed
// we should remove the OpsRequest Annotation of cluster, then unlock cluster
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
	reconfiguringStatus.ConfigurationStatus = append(reconfiguringStatus.ConfigurationStatus, appsv1alpha1.ConfigurationItemStatus{
		Name:          tplName,
		Status:        appsv1alpha1.ReasonReconfigurePersisting,
		SucceedCount:  core.NotStarted,
		ExpectedCount: core.Unconfirmed,
	})
	cmStatus := &reconfiguringStatus.ConfigurationStatus[cmCount]
	return handleReconfigureStatus(cmStatus)
}

// patchReconfigureOpsStatus when Reconfigure is running, we should update status to OpsRequest.Status.ConfigurationStatus.
//
// NOTES:
// opsStatus describes status of OpsRequest.
// reconfiguringStatus describes status of reconfiguring operation, which contains multiple configuration templates.
// cmStatus describes status of configmap, it is uniquely associated with a configuration template, which contains multiple keys, each key is name of a configuration file.
// execStatus describes the result of the execution of the state machine, which is designed to solve how to conduct the reconfiguring operation, such as whether to restart, how to send a signal to the process.
func patchReconfigureOpsStatus(
	opsRes *OpsResource,
	tplName string,
	handleReconfigureStatus handleReconfigureOpsStatus) error {
	var opsRequest = opsRes.OpsRequest
	if opsRequest.Status.ReconfiguringStatus == nil {
		opsRequest.Status.ReconfiguringStatus = &appsv1alpha1.ReconfiguringStatus{
			ConfigurationStatus: make([]appsv1alpha1.ConfigurationItemStatus, 0),
		}
	}

	reconfiguringStatus := opsRequest.Status.ReconfiguringStatus
	return updateReconfigureStatusByCM(reconfiguringStatus, tplName, handleReconfigureStatus)
}

// getSlowestReconfiguringProgress gets the progress of the reconfiguring operations.
func getSlowestReconfiguringProgress(status []appsv1alpha1.ConfigurationItemStatus) string {
	slowest := appsv1alpha1.ConfigurationItemStatus{
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

func getTargetResourcesOfLastComponent(lastConfiguration appsv1alpha1.LastConfiguration, compName string, resourceKey appsv1alpha1.ComponentResourceKey) []string {
	lastComponentConfigs := lastConfiguration.Components[compName]
	return lastComponentConfigs.TargetResources[resourceKey]
}

// cancelComponentOps the common function to cancel th opsRequest which updates the component attributes.
func cancelComponentOps(ctx context.Context,
	cli client.Client,
	opsRes *OpsResource,
	updateComp func(lastConfig *appsv1alpha1.LastComponentConfiguration, comp *appsv1alpha1.ClusterComponentSpec) error) error {
	opsRequest := opsRes.OpsRequest
	lastCompInfos := opsRequest.Status.LastConfiguration.Components
	if lastCompInfos == nil {
		return nil
	}
	for index, comp := range opsRes.Cluster.Spec.ComponentSpecs {
		lastConfig, ok := lastCompInfos[comp.Name]
		if !ok {
			continue
		}
		if err := updateComp(&lastConfig, &comp); err != nil {
			return err
		}
		opsRes.Cluster.Spec.ComponentSpecs[index] = comp
		lastCompInfos[comp.Name] = lastConfig
	}
	return cli.Update(ctx, opsRes.Cluster)
}

// validateOpsWaitingPhase validates whether the current cluster phase is expected, and whether the waiting time exceeds the limit.
// only requests with `Pending` phase will be validated.
func validateOpsWaitingPhase(cluster *appsv1alpha1.Cluster, ops *appsv1alpha1.OpsRequest, opsBehaviour OpsBehaviour) error {
	// if opsRequest don't need to wait for the cluster phase
	// or opsRequest status.phase is not Pending,
	// or opsRequest will create cluster,
	// we don't validate the cluster phase.
	if len(opsBehaviour.FromClusterPhases) == 0 || ops.Status.Phase != appsv1alpha1.OpsPendingPhase {
		return nil
	}
	// check if the opsRequest can be executed in the current cluster phase unless this opsRequest is reentrant.
	if !slices.Contains(opsBehaviour.FromClusterPhases, cluster.Status.Phase) {
		// check if entry-condition is met
		// if the cluster is not in the expected phase, we should wait for it for up to TTLSecondsBeforeAbort seconds.
		// if len(opsRecorder) == 0 && !slices.Contains(opsBehaviour.FromClusterPhases, cluster.Status.Phase) {
		// TTLSecondsBeforeAbort is 0 means that the we do not need to wait for the cluster to reach the expected phase.
		if ops.Spec.TTLSecondsBeforeAbort == nil || (time.Now().After(ops.GetCreationTimestamp().Add(time.Duration(*ops.Spec.TTLSecondsBeforeAbort) * time.Second))) {
			return fmt.Errorf("OpsRequest.spec.type=%s is forbidden when Cluster.status.phase=%s", ops.Spec.Type, cluster.Status.Phase)
		}

		return &WaitForClusterPhaseErr{
			clusterName:   cluster.Name,
			currentPhase:  cluster.Status.Phase,
			expectedPhase: opsBehaviour.FromClusterPhases,
		}
	}
	return nil
}
