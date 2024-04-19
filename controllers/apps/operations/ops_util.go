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
	"context"
	"fmt"
	"reflect"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlcomp "github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	// componentFailedTimeout when the duration of component failure exceeds this threshold, it is determined that opsRequest has failed
	componentFailedTimeout = 30 * time.Second

	opsRequestQueueLimitSize = 20
)

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

type syncOverrideByOps func(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error

func isFailedOrAbnormal(phase appsv1alpha1.ClusterComponentPhase) bool {
	return slices.Index([]appsv1alpha1.ClusterComponentPhase{
		appsv1alpha1.FailedClusterCompPhase,
		appsv1alpha1.AbnormalClusterCompPhase}, phase) != -1
}

// reconcileActionWithComponentOps will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the common function to reconcile opsRequest status when the opsRequest will affect the lifecycle of the components.
func reconcileActionWithComponentOps(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	opsMessageKey string,
	syncOverrideBy syncOverrideByOps,
	handleStatusProgress handleStatusProgressWithComponent,
) (appsv1alpha1.OpsPhase, time.Duration, error) {
	if opsRes == nil {
		return "", 0, nil
	}
	var (
		opsRequestPhase          = appsv1alpha1.OpsRunningPhase
		opsRequest               = opsRes.OpsRequest
		isFailed                 bool
		ok                       bool
		expectProgressCount      int32
		completedProgressCount   int32
		checkAllClusterComponent bool
		requeueTimeAfterFailed   time.Duration
		err                      error
		clusterDef               *appsv1alpha1.ClusterDefinition
	)
	if opsRes.Cluster.Spec.ClusterDefRef != "" {
		if clusterDef, err = getClusterDefByName(reqCtx.Ctx, cli, opsRes.Cluster.Spec.ClusterDefRef); err != nil {
			return opsRequestPhase, 0, err
		}
	}
	componentNameMap := opsRequest.GetComponentNameSet()
	// if no specified components, we should check the all components phase of cluster.
	if len(componentNameMap) == 0 {
		checkAllClusterComponent = true
	}
	oldOpsRequest := opsRequest.DeepCopy()
	patch := client.MergeFrom(oldOpsRequest)
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = map[string]appsv1alpha1.OpsRequestComponentStatus{}
	}
	if syncOverrideBy != nil {
		if err = syncOverrideBy(reqCtx, cli, opsRes); err != nil {
			return "", 0, nil
		}
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
		if isFailedOrAbnormal(v.Phase) {
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
		var componentDefinition *appsv1alpha1.ComponentDefinition
		if clusterComponent.ComponentDef != "" {
			componentDefinition, err = intctrlcomp.GetCompDefinition(reqCtx, cli, opsRes.Cluster, k)
			if err != nil {
				return opsRequestPhase, 0, err
			}
		}
		expectCount, completedCount, err := handleStatusProgress(reqCtx, cli, opsRes, progressResource{
			opsMessageKey:    opsMessageKey,
			clusterComponent: clusterComponent,
			clusterDef:       clusterDef,
			componentDef:     componentDefinition,
			opsIsCompleted:   opsIsCompleted,
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
	if !reflect.DeepEqual(opsRequest.Status, oldOpsRequest.Status) {
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
	itsList := &workloads.InstanceSetList{}
	if err := cli.List(reqCtx.Ctx, itsList,
		client.InNamespace(opsRes.Cluster.Namespace),
		client.MatchingLabels{constant.AppInstanceLabelKey: opsRes.Cluster.Name}); err != nil {
		return false
	}
	for _, its := range itsList.Items {
		isLatestRevision, err := intctrlcomp.IsComponentPodsWithLatestRevision(reqCtx.Ctx, cli, opsRes.Cluster, &its)
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
	opsRequest.Status.Phase = phase
	if opsRequest.IsComplete(phase) {
		opsRequest.Status.CompletionTimestamp = metav1.Time{Time: time.Now()}
		// when OpsRequest is completed, remove it from annotation
		if err := DequeueOpsRequestInClusterAnnotation(ctx, cli, opsRes); err != nil {
			return err
		}
	}
	if phase == appsv1alpha1.OpsCreatingPhase && opsRequest.Status.StartTimestamp.IsZero() {
		opsRequest.Status.StartTimestamp = metav1.Time{Time: time.Now()}
	}
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

// patchFatalFailErrorCondition patches a new failed condition to the OpsRequest.status.conditions.
func patchFatalFailErrorCondition(ctx context.Context, cli client.Client, opsRes *OpsResource, err error) error {
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

// isOpsRequestFailedPhase checks the OpsRequest phase is Failed
func isOpsRequestFailedPhase(opsRequestPhase appsv1alpha1.OpsPhase) bool {
	return opsRequestPhase == appsv1alpha1.OpsFailedPhase
}

// patchReconfigureOpsStatus when Reconfigure is running, we should update status to OpsRequest.Status.ConfigurationStatus.
//
// NOTES:
// opsStatus describes status of OpsRequest.
// reconfiguringStatus describes status of reconfiguring operation, which contains multiple configuration templates.
// cmStatus describes status of configmap, it is uniquely associated with a configuration template, which contains multiple keys, each key is name of a configuration file.
// execStatus describes the result of the execution of the state machine, which is designed to solve how to conduct the reconfiguring operation, such as whether to restart, how to send a signal to the process.
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
	if ops.Force() {
		return nil
	}
	// if opsRequest don't need to wait for the cluster phase
	// or opsRequest status.phase is not Pending,
	// or opsRequest will create cluster,
	// we don't validate the cluster phase.
	if len(opsBehaviour.FromClusterPhases) == 0 || ops.Status.Phase != appsv1alpha1.OpsPendingPhase || opsBehaviour.IsClusterCreation {
		return nil
	}
	if slices.Contains(opsBehaviour.FromClusterPhases, cluster.Status.Phase) {
		return nil
	}
	// check if entry-condition is met
	// if the cluster is not in the expected phase, we should wait for it for up to TTLSecondsBeforeAbort seconds.
	if ops.Spec.TTLSecondsBeforeAbort == nil || (time.Now().After(ops.GetCreationTimestamp().Add(time.Duration(*ops.Spec.TTLSecondsBeforeAbort) * time.Second))) {
		return nil
	}

	return &WaitForClusterPhaseErr{
		clusterName:   cluster.Name,
		currentPhase:  cluster.Status.Phase,
		expectedPhase: opsBehaviour.FromClusterPhases,
	}
}

func getRunningOpsNamesWithSameKind(cluster *appsv1alpha1.Cluster, types ...appsv1alpha1.OpsType) ([]string, error) {
	opsRequestSlice, err := opsutil.GetOpsRequestSliceFromCluster(cluster)
	if err != nil {
		return nil, err
	}
	var runningVScaleOps []string
	for _, v := range opsRequestSlice {
		if slices.Contains(types, v.Type) && !v.InQueue {
			runningVScaleOps = append(runningVScaleOps, v.Name)
		}
	}
	return runningVScaleOps, nil
}

// getRunningOpsRequestWithSameKind gets the running opsRequests with the same kind.
func getRunningOpsRequestsWithSameKind(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1alpha1.Cluster, types ...appsv1alpha1.OpsType) ([]*appsv1alpha1.OpsRequest, error) {
	runningVScaleOps, err := getRunningOpsNamesWithSameKind(cluster, types...)
	if err != nil {
		return nil, err
	}
	runningVScaleOpsLen := len(runningVScaleOps)
	if runningVScaleOpsLen == 1 {
		// If there are no concurrent executions opsRequests of the same type, return
		return nil, nil
	}

	// get the opsRequests by sorting in reverse order according to queue order
	var runningOpsRequests []*appsv1alpha1.OpsRequest
	for i := runningVScaleOpsLen - 1; i >= 0; i-- {
		ops := &appsv1alpha1.OpsRequest{}
		if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: runningVScaleOps[i], Namespace: cluster.Namespace}, ops); err != nil {
			return nil, err
		}
		if ops.Status.Phase == appsv1alpha1.OpsRunningPhase {
			runningOpsRequests = append(runningOpsRequests, ops)
		}
	}
	return runningOpsRequests, nil
}

func syncOverrideByOpsForScaleReplicas(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	runningOpsRequests, err := getRunningOpsRequestsWithSameKind(reqCtx, cli, opsRes.Cluster, appsv1alpha1.HorizontalScalingType, appsv1alpha1.StopType, appsv1alpha1.StartType)
	if err != nil || len(runningOpsRequests) == 0 {
		return err
	}

	// get the latest opsName which has the same replicas with the component replicas.
	getTheLatestOpsName := func(compName string, compReplicas int32) string {
		for _, ops := range runningOpsRequests {
			switch ops.Spec.Type {
			case appsv1alpha1.HorizontalScalingType:
				for _, v := range ops.Spec.HorizontalScalingList {
					if v.ComponentName == compName && v.Replicas == compReplicas {
						return ops.Name
					}
				}
			case appsv1alpha1.StopType:
				if compReplicas == 0 {
					return ops.Name
				}
			case appsv1alpha1.StartType:
				opsCompReplicasMap, _ := getComponentReplicasSnapshot(ops.Annotations)
				if replicas, ok := opsCompReplicasMap[compName]; ok && replicas == compReplicas {
					return ops.Name
				}
			}
		}
		return ""
	}

	compReplicasMap := map[string]int32{}
	for _, comp := range opsRes.Cluster.Spec.ComponentSpecs {
		compReplicasMap[comp.Name] = comp.Replicas
	}
	doComponentOverrideBy := func(compName string, desiredCompReplicas int32) {
		compReplicas, ok := compReplicasMap[compName]
		if !ok || desiredCompReplicas == compReplicas {
			return
		}
		componentStatus := opsRes.OpsRequest.Status.Components[compName]
		componentStatus.OverrideBy = &appsv1alpha1.OverrideBy{
			OpsName: getTheLatestOpsName(compName, compReplicas),
			LastComponentConfiguration: appsv1alpha1.LastComponentConfiguration{
				Replicas: &compReplicas,
			},
		}
		opsRes.OpsRequest.Status.Components[compName] = componentStatus
	}
	// checks if the number of replicas applied by the current opsRequest matches the desired number of replicas for the component.
	// if not matched, set the Override info in the opsRequest.status.components.
	switch opsRes.OpsRequest.Spec.Type {
	case appsv1alpha1.HorizontalScalingType:
		for _, opsComp := range opsRes.OpsRequest.Spec.HorizontalScalingList {
			doComponentOverrideBy(opsComp.ComponentName, opsComp.Replicas)
		}
	case appsv1alpha1.StopType:
		for compName := range opsRes.OpsRequest.Status.Components {
			doComponentOverrideBy(compName, 0)
		}
	case appsv1alpha1.StartType:
		opsCompReplicasMap, _ := getComponentReplicasSnapshot(opsRes.OpsRequest.Annotations)
		for compName := range opsRes.OpsRequest.Status.Components {
			replicas, ok := opsCompReplicasMap[compName]
			if !ok {
				continue
			}
			doComponentOverrideBy(compName, replicas)
		}
	}
	return nil
}
