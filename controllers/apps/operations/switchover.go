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
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	componentutil "github.com/apecloud/kubeblocks/controllers/apps/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type switchoverOpsHandler struct{}

var _ OpsHandler = switchoverOpsHandler{}

// SwitchoverMessage is the OpsRequest.Status.Condition.Message for switchover.
type SwitchoverMessage struct {
	appsv1alpha1.Switchover
	OldPrimary string
	Cluster    string
}

func init() {
	switchoverBehaviour := OpsBehaviour{
		FromClusterPhases:                  appsv1alpha1.GetClusterUpRunningPhases(),
		ToClusterPhase:                     appsv1alpha1.SpecReconcilingClusterPhase,
		OpsHandler:                         switchoverOpsHandler{},
		MaintainClusterPhaseBySelf:         true,
		ProcessingReasonInClusterCondition: ProcessingReasonVersionSwitchovering,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.SwitchoverType, switchoverBehaviour)
}

// ActionStartedCondition the started condition when handle the switchover request.
func (r switchoverOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	switchoverMessageMap := make(map[string]SwitchoverMessage)
	for _, switchover := range opsRes.OpsRequest.Spec.SwitchoverList {
		pod, err := getPrimaryOrLeaderPod(reqCtx.Ctx, cli, *opsRes.Cluster, switchover.ComponentName, opsRes.Cluster.Spec.GetComponentDefRefName(switchover.ComponentName))
		if err != nil {
			return nil, err
		}
		switchoverMessageMap[switchover.ComponentName] = SwitchoverMessage{
			Switchover: switchover,
			OldPrimary: pod.Name,
			Cluster:    opsRes.Cluster.Name,
		}
	}
	msg, err := json.Marshal(switchoverMessageMap)
	if err != nil {
		return nil, err
	}
	return appsv1alpha1.NewSwitchoveringCondition(opsRes.Cluster.Generation, string(msg)), nil
}

// Action to do the switchover operation.
func (r switchoverOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	switchoverMap := opsRes.OpsRequest.Spec.ToSwitchoverListToMap()
	return doSwitchoverComponents(reqCtx, cli, opsRes, switchoverMap)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for switchover opsRequest.
func (r switchoverOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	var (
		opsRequestPhase = appsv1alpha1.OpsRunningPhase
	)

	expectCount, actualCount, err := handleSwitchoverProgress(reqCtx, cli, opsRes)
	if err != nil {
		return "", 0, err
	}

	if expectCount == actualCount {
		opsRequestPhase = appsv1alpha1.OpsSucceedPhase
	}

	return opsRequestPhase, 5 * time.Second, nil
}

// GetRealAffectedComponentMap gets the real affected component map for the operation
func (r switchoverOpsHandler) GetRealAffectedComponentMap(opsRequest *appsv1alpha1.OpsRequest) realAffectedComponentMap {
	return realAffectedComponentMap(opsRequest.Spec.GetSwitchoverComponentNameSet())
}

// SaveLastConfiguration this operation only restart the pods of the component, no changes for Cluster.spec.
// empty implementation here.
func (r switchoverOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

// doSwitchoverComponents creates the switchover job for each component.
func doSwitchoverComponents(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource, switchoverMap map[string]appsv1alpha1.Switchover) error {
	for compSpecName, switchover := range switchoverMap {
		compDef, err := componentutil.GetComponentDefByCluster(reqCtx.Ctx, cli, *opsRes.Cluster, compSpecName)
		if err != nil {
			return err
		}
		needSwitchover, err := needDoSwitchover(reqCtx.Ctx, cli, opsRes.Cluster, opsRes.Cluster.Spec.GetComponentByName(compSpecName), &switchover)
		if err != nil {
			return err
		}
		if !needSwitchover {
			continue
		}
		if err := createSwitchoverJob(reqCtx, cli, opsRes.Cluster, opsRes.Cluster.Spec.GetComponentByName(compSpecName), compDef, &switchover); err != nil {
			return err
		}
	}
	return nil
}

// handleSwitchoverProgress handles the component progressDetails during switchover.
// Returns:
// - expectCount: the expected count of switchover operations
// - completedCount: the number of completed switchover operations
// - error: any error that occurred during the handling
func handleSwitchoverProgress(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (int32, int32, error) {
	var (
		expectCount         = int32(len(opsRes.OpsRequest.Spec.SwitchoverList))
		completedCount      int32
		opsRequest          = opsRes.OpsRequest
		oldOpsRequestStatus = opsRequest.Status.DeepCopy()
		compDef             *appsv1alpha1.ClusterComponentDefinition
		needSwitchover      bool
		consistency         bool
		jobExist            bool
		err                 error
	)
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = make(map[string]appsv1alpha1.OpsRequestComponentStatus)
		for _, v := range opsRequest.Spec.SwitchoverList {
			opsRequest.Status.Components[v.ComponentName] = appsv1alpha1.OpsRequestComponentStatus{
				Phase:           appsv1alpha1.SpecReconcilingClusterCompPhase,
				ProgressDetails: []appsv1alpha1.ProgressStatusDetail{},
			}
		}
	}

	succeedJobs := make([]string, 0, len(opsRes.OpsRequest.Spec.SwitchoverList))
	switchoverMap := opsRes.OpsRequest.Spec.ToSwitchoverListToMap()
	for compSpecName, switchover := range switchoverMap {
		switchoverCondition := meta.FindStatusCondition(opsRes.OpsRequest.Status.Conditions, appsv1alpha1.ConditionTypeSwitchover)
		if switchoverCondition == nil {
			err = errors.New("switchover condition is nil")
			break
		}

		// check the current component switchoverJob whether succeed
		jobName := genSwitchoverJobName(opsRes.Cluster.Name, compSpecName, switchoverCondition.ObservedGeneration)
		checkJobProcessDetail := appsv1alpha1.ProgressStatusDetail{
			ObjectKey: getProgressObjectKey(SwitchoverCheckJobKey, jobName),
			Status:    appsv1alpha1.ProcessingProgressStatus,
		}
		jobExist, err = checkJobSucceed(reqCtx.Ctx, cli, opsRes.Cluster, jobName)
		if err != nil {
			checkJobProcessDetail.Message = fmt.Sprintf("switchover job %s is not succeed", jobName)
			setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1alpha1.SpecReconcilingClusterCompPhase, checkJobProcessDetail, compSpecName)
			continue
		}
		if !jobExist {
			// if the job does not exist, it may not be necessary to perform a switchover because the specified instanceName is already the primary.
			needSwitchover, err = needDoSwitchover(reqCtx.Ctx, cli, opsRes.Cluster, opsRes.Cluster.Spec.GetComponentByName(compSpecName), &switchover)
			if err != nil {
				continue
			}
			if !needSwitchover {
				completedCount += 1
				skipSwitchoverProcessDetail := appsv1alpha1.ProgressStatusDetail{
					ObjectKey: getProgressObjectKey(SwitchoverCheckRoleLabelKey, compSpecName),
					Message:   fmt.Sprintf("current instance %s is already the primary, then no switchover will be performed", switchover.InstanceName),
					Status:    appsv1alpha1.SucceedProgressStatus,
				}
				setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1alpha1.SpecReconcilingClusterCompPhase, skipSwitchoverProcessDetail, compSpecName)
			}
			continue
		} else {
			checkJobProcessDetail.Message = fmt.Sprintf("switchover job %s is succeed", jobName)
			checkJobProcessDetail.Status = appsv1alpha1.SucceedProgressStatus
			setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1alpha1.SpecReconcilingClusterCompPhase, checkJobProcessDetail, compSpecName)
		}

		// check the current component pod role label whether correct
		checkRoleLabelProcessDetail := appsv1alpha1.ProgressStatusDetail{
			ObjectKey: getProgressObjectKey(SwitchoverCheckRoleLabelKey, compSpecName),
			Status:    appsv1alpha1.ProcessingProgressStatus,
			Message:   fmt.Sprintf("waiting for component %s pod role label consistency after switchover", compSpecName),
		}
		compDef, err = componentutil.GetComponentDefByCluster(reqCtx.Ctx, cli, *opsRes.Cluster, compSpecName)
		if err != nil {
			checkRoleLabelProcessDetail.Message = fmt.Sprintf("handleSwitchoverProgress get component %s definition failed", compSpecName)
			checkRoleLabelProcessDetail.Status = appsv1alpha1.FailedProgressStatus
			setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1alpha1.SpecReconcilingClusterCompPhase, checkRoleLabelProcessDetail, compSpecName)
			continue
		}
		consistency, err = checkPodRoleLabelConsistency(reqCtx.Ctx, cli, opsRes.Cluster, opsRes.Cluster.Spec.GetComponentByName(compSpecName), compDef, &switchover, switchoverCondition)
		if err != nil {
			checkRoleLabelProcessDetail.Message = fmt.Sprintf("waiting for component %s pod role label consistency after switchover", compSpecName)
			setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1alpha1.SpecReconcilingClusterCompPhase, checkRoleLabelProcessDetail, compSpecName)
			continue
		}

		if !consistency {
			err = intctrlutil.NewErrorf(intctrlutil.ErrorWaitCacheRefresh, "requeue to waiting for pod role label consistency.")
			setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1alpha1.SpecReconcilingClusterCompPhase, checkRoleLabelProcessDetail, compSpecName)
			continue
		} else {
			checkRoleLabelProcessDetail.Message = fmt.Sprintf("check component %s pod role label consistency after switchover is succeed", compSpecName)
			checkRoleLabelProcessDetail.Status = appsv1alpha1.SucceedProgressStatus
			setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1alpha1.SpecReconcilingClusterCompPhase, checkRoleLabelProcessDetail, compSpecName)
		}

		// component switchover is successful
		completedCount += 1
		succeedJobs = append(succeedJobs, jobName)
		componentProcessDetail := appsv1alpha1.ProgressStatusDetail{
			ObjectKey: switchover.ComponentName,
			Message:   fmt.Sprintf("switchover job %s is succeed", jobName),
			Status:    appsv1alpha1.SucceedProgressStatus,
		}
		setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1alpha1.RunningClusterCompPhase, componentProcessDetail, compSpecName)
	}

	opsRequest.Status.Progress = fmt.Sprintf("%d/%d", completedCount, expectCount)
	// patch OpsRequest.status.components
	if !reflect.DeepEqual(*oldOpsRequestStatus, opsRequest.Status) {
		if err := cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return expectCount, 0, err
		}
	}

	if err != nil {
		return expectCount, completedCount, err
	}

	if completedCount == expectCount {
		for _, jobName := range succeedJobs {
			_ = cleanJobByName(reqCtx.Ctx, cli, opsRes.Cluster, jobName)
		}
	}

	return expectCount, completedCount, nil
}

// setComponentSwitchoverProgressDetails sets component switchover progress details.
func setComponentSwitchoverProgressDetails(recorder record.EventRecorder,
	opsRequest *appsv1alpha1.OpsRequest,
	phase appsv1alpha1.ClusterComponentPhase,
	processDetail appsv1alpha1.ProgressStatusDetail,
	componentName string) {
	componentProcessDetails := opsRequest.Status.Components[componentName].ProgressDetails
	setComponentStatusProgressDetail(recorder, opsRequest, &componentProcessDetails, processDetail)
	opsRequest.Status.Components[componentName] = appsv1alpha1.OpsRequestComponentStatus{
		Phase:           phase,
		ProgressDetails: componentProcessDetails,
	}
}
