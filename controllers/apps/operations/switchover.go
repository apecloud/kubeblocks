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
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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
		FromClusterPhases: appsv1alpha1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1alpha1.UpdatingClusterPhase,
		OpsHandler:        switchoverOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.SwitchoverType, switchoverBehaviour)
}

// ActionStartedCondition the started condition when handle the switchover request.
func (r switchoverOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	switchoverMessageMap := make(map[string]SwitchoverMessage)
	for _, switchover := range opsRes.OpsRequest.Spec.SwitchoverList {
		compSpec := opsRes.Cluster.Spec.GetComponentByName(switchover.ComponentName)
		synthesizedComp, err := component.BuildSynthesizedComponentWrapper(reqCtx, cli, opsRes.Cluster, compSpec)
		if err != nil {
			return nil, err
		}
		pod, err := getServiceableNWritablePod(reqCtx.Ctx, cli, *opsRes.Cluster, *synthesizedComp)
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
	return doSwitchoverComponents(reqCtx, cli, opsRes, opsRes.OpsRequest.Spec.SwitchoverList)
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

	return opsRequestPhase, time.Second, nil
}

// SaveLastConfiguration this operation only restart the pods of the component, no changes for Cluster.spec.
// empty implementation here.
func (r switchoverOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

// doSwitchoverComponents creates the switchover job for each component.
func doSwitchoverComponents(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource, switchoverList []appsv1alpha1.Switchover) error {
	var (
		opsRequest          = opsRes.OpsRequest
		oldOpsRequestStatus = opsRequest.Status.DeepCopy()
	)
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = make(map[string]appsv1alpha1.OpsRequestComponentStatus)
	}
	for _, switchover := range switchoverList {
		compSpec := opsRes.Cluster.Spec.GetComponentByName(switchover.ComponentName)
		synthesizedComp, err := component.BuildSynthesizedComponentWrapper(reqCtx, cli, opsRes.Cluster, compSpec)
		if err != nil {
			return err
		}
		needSwitchover, err := needDoSwitchover(reqCtx.Ctx, cli, opsRes.Cluster, synthesizedComp, &switchover)
		if err != nil {
			return err
		}
		if !needSwitchover {
			opsRequest.Status.Components[switchover.ComponentName] = appsv1alpha1.OpsRequestComponentStatus{
				Phase:           appsv1alpha1.RunningClusterCompPhase,
				Reason:          OpsReasonForSkipSwitchover,
				Message:         fmt.Sprintf("This component %s is already in the expected state, skip the switchover operation", switchover.ComponentName),
				ProgressDetails: []appsv1alpha1.ProgressStatusDetail{},
			}
			continue
		} else {
			opsRequest.Status.Components[switchover.ComponentName] = appsv1alpha1.OpsRequestComponentStatus{
				Phase:           appsv1alpha1.UpdatingClusterCompPhase,
				ProgressDetails: []appsv1alpha1.ProgressStatusDetail{},
			}
		}
		if err := createSwitchoverJob(reqCtx, cli, opsRes.Cluster, synthesizedComp, &switchover); err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(*oldOpsRequestStatus, opsRequest.Status) {
		if err := cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
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
		consistency         bool
		err                 error
	)
	patch := client.MergeFrom(opsRequest.DeepCopy())
	succeedJobs := make([]string, 0, len(opsRes.OpsRequest.Spec.SwitchoverList))
	for _, switchover := range opsRequest.Spec.SwitchoverList {
		switchoverCondition := meta.FindStatusCondition(opsRes.OpsRequest.Status.Conditions, appsv1alpha1.ConditionTypeSwitchover)
		if switchoverCondition == nil {
			err = errors.New("switchover condition is nil")
			break
		}

		// if the component do not need switchover, skip it
		reason := opsRequest.Status.Components[switchover.ComponentName].Reason
		if reason == OpsReasonForSkipSwitchover {
			completedCount += 1
			continue
		}
		// check the current component switchoverJob whether succeed
		jobName := genSwitchoverJobName(opsRes.Cluster.Name, switchover.ComponentName, switchoverCondition.ObservedGeneration)
		checkJobProcessDetail := appsv1alpha1.ProgressStatusDetail{
			ObjectKey: getProgressObjectKey(KBSwitchoverCheckJobKey, jobName),
			Status:    appsv1alpha1.ProcessingProgressStatus,
		}
		if err = component.CheckJobSucceed(reqCtx.Ctx, cli, opsRes.Cluster, jobName); err != nil {
			checkJobProcessDetail.Message = fmt.Sprintf("switchover job %s is not succeed", jobName)
			setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1alpha1.UpdatingClusterCompPhase, checkJobProcessDetail, switchover.ComponentName)
			continue
		} else {
			checkJobProcessDetail.Message = fmt.Sprintf("switchover job %s is succeed", jobName)
			checkJobProcessDetail.Status = appsv1alpha1.SucceedProgressStatus
			setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1alpha1.UpdatingClusterCompPhase, checkJobProcessDetail, switchover.ComponentName)
		}

		// check the current component pod role label whether correct
		checkRoleLabelProcessDetail := appsv1alpha1.ProgressStatusDetail{
			ObjectKey: getProgressObjectKey(KBSwitchoverCheckRoleLabelKey, switchover.ComponentName),
			Status:    appsv1alpha1.ProcessingProgressStatus,
			Message:   fmt.Sprintf("waiting for component %s pod role label consistency after switchover", switchover.ComponentName),
		}
		compSpec := opsRes.Cluster.Spec.GetComponentByName(switchover.ComponentName)
		synthesizedComp, errBuild := component.BuildSynthesizedComponentWrapper(reqCtx, cli, opsRes.Cluster, compSpec)
		if errBuild != nil {
			checkRoleLabelProcessDetail.Message = fmt.Sprintf("handleSwitchoverProgress build synthesizedComponent %s failed", switchover.ComponentName)
			checkRoleLabelProcessDetail.Status = appsv1alpha1.FailedProgressStatus
			setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1alpha1.UpdatingClusterCompPhase, checkRoleLabelProcessDetail, switchover.ComponentName)
			continue
		}
		consistency, err = checkPodRoleLabelConsistency(reqCtx.Ctx, cli, opsRes.Cluster, *synthesizedComp, &switchover, switchoverCondition)
		if err != nil {
			checkRoleLabelProcessDetail.Message = fmt.Sprintf("waiting for component %s pod role label consistency after switchover", switchover.ComponentName)
			setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1alpha1.UpdatingClusterCompPhase, checkRoleLabelProcessDetail, switchover.ComponentName)
			continue
		}

		if !consistency {
			err = intctrlutil.NewErrorf(intctrlutil.ErrorWaitCacheRefresh, "requeue to waiting for pod role label consistency.")
			setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1alpha1.UpdatingClusterCompPhase, checkRoleLabelProcessDetail, switchover.ComponentName)
			continue
		} else {
			checkRoleLabelProcessDetail.Message = fmt.Sprintf("check component %s pod role label consistency after switchover is succeed", switchover.ComponentName)
			checkRoleLabelProcessDetail.Status = appsv1alpha1.SucceedProgressStatus
			setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1alpha1.UpdatingClusterCompPhase, checkRoleLabelProcessDetail, switchover.ComponentName)
		}

		// component switchover is successful
		completedCount += 1
		succeedJobs = append(succeedJobs, jobName)
		componentProcessDetail := appsv1alpha1.ProgressStatusDetail{
			ObjectKey: switchover.ComponentName,
			Message:   fmt.Sprintf("switchover job %s is succeed", jobName),
			Status:    appsv1alpha1.SucceedProgressStatus,
		}
		setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1alpha1.RunningClusterCompPhase, componentProcessDetail, switchover.ComponentName)
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
			if err := component.CleanJobByName(reqCtx.Ctx, cli, opsRes.Cluster, jobName); err != nil {
				reqCtx.Log.Error(err, "clean switchover job failed", "jobName", jobName)
				return expectCount, completedCount, err
			}
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
