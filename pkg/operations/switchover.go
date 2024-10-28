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
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/component/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type switchoverOpsHandler struct{}

var _ OpsHandler = switchoverOpsHandler{}

// SwitchoverMessage is the OpsRequest.Status.Condition.Message for switchover.
type SwitchoverMessage struct {
	opsv1alpha1.Switchover
	OldPrimary string
	Cluster    string
}

func init() {
	switchoverBehaviour := OpsBehaviour{
		FromClusterPhases: appsv1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1.UpdatingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        switchoverOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.SwitchoverType, switchoverBehaviour)
}

// ActionStartedCondition the started condition when handle the switchover request.
func (r switchoverOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	switchoverMessageMap := make(map[string]SwitchoverMessage)
	for _, switchover := range opsRes.OpsRequest.Spec.SwitchoverList {
		compSpec := opsRes.Cluster.Spec.GetComponentByName(switchover.ComponentName)
		synthesizedComp, err := buildSynthesizedComp(reqCtx.Ctx, cli, opsRes, compSpec)
		if err != nil {
			return nil, err
		}
		pod, err := getServiceableNWritablePod(reqCtx.Ctx, cli, *synthesizedComp)
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
	return opsv1alpha1.NewSwitchoveringCondition(opsRes.Cluster.Generation, string(msg)), nil
}

func (r switchoverOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return switchoverPreCheck(reqCtx, cli, opsRes, opsRes.OpsRequest.Spec.SwitchoverList)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for switchover opsRequest.
func (r switchoverOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	var (
		opsRequestPhase = opsv1alpha1.OpsRunningPhase
	)

	expectCount, actualCount, failedCount, err := handleSwitchovers(reqCtx, cli, opsRes)
	if err != nil {
		return "", 0, err
	}

	if expectCount == actualCount {
		opsRequestPhase = opsv1alpha1.OpsSucceedPhase
		if failedCount > 0 {
			opsRequestPhase = opsv1alpha1.OpsFailedPhase
		}
	}

	return opsRequestPhase, time.Second, nil
}

// SaveLastConfiguration this operation only restart the pods of the component, no changes for Cluster.spec.
// empty implementation here.
func (r switchoverOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

// switchoverPreCheck checks whether the component need switchover.
func switchoverPreCheck(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource, switchoverList []opsv1alpha1.Switchover) error {
	var (
		opsRequest          = opsRes.OpsRequest
		oldOpsRequestStatus = opsRequest.Status.DeepCopy()
	)
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = make(map[string]opsv1alpha1.OpsRequestComponentStatus)
	}
	for _, switchover := range switchoverList {
		compSpec := opsRes.Cluster.Spec.GetComponentByName(switchover.ComponentName)
		synthesizedComp, err := buildSynthesizedComp(reqCtx.Ctx, cli, opsRes, compSpec)
		if err != nil {
			return err
		}
		needSwitchover, err := needDoSwitchover(reqCtx.Ctx, cli, synthesizedComp, &switchover)
		if err != nil {
			return err
		}
		if !needSwitchover {
			opsRequest.Status.Components[switchover.ComponentName] = opsv1alpha1.OpsRequestComponentStatus{
				Phase:           appsv1.RunningComponentPhase,
				Reason:          OpsReasonForSkipSwitchover,
				Message:         fmt.Sprintf("This component %s is already in the expected state, skip the switchover operation", switchover.ComponentName),
				ProgressDetails: []opsv1alpha1.ProgressStatusDetail{},
			}
			continue
		} else {
			opsRequest.Status.Components[switchover.ComponentName] = opsv1alpha1.OpsRequestComponentStatus{
				Phase:           appsv1.UpdatingComponentPhase,
				ProgressDetails: []opsv1alpha1.ProgressStatusDetail{},
			}
		}
	}
	if !reflect.DeepEqual(*oldOpsRequestStatus, opsRequest.Status) {
		if err := cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return err
		}
	}
	return nil
}

// handleSwitchovers handles the component progressDetails during switchover.
// Returns:
// - expectCount: the expected count of switchover operations
// - completedCount: the number of completed switchover operations
// - failedCount: the number of failed switchover operations
// - error: any error that occurred during the handling
func handleSwitchovers(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (int32, int32, int32, error) {
	expectCount := int32(len(opsRes.OpsRequest.Spec.SwitchoverList))
	var completedCount, failedCount int32

	opsRequest := opsRes.OpsRequest
	oldOpsRequestStatus := opsRequest.Status.DeepCopy()
	patch := client.MergeFrom(opsRequest.DeepCopy())

	for _, switchover := range opsRequest.Spec.SwitchoverList {
		if err := handleSwitchover(reqCtx, cli, opsRes, &switchover, opsRequest, &completedCount, &failedCount); err != nil {
			return expectCount, completedCount, failedCount, err
		}
	}

	opsRequest.Status.Progress = fmt.Sprintf("%d/%d", completedCount, expectCount)
	if !reflect.DeepEqual(*oldOpsRequestStatus, opsRequest.Status) {
		if err := cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return expectCount, completedCount, failedCount, err
		}
	}

	return expectCount, completedCount, failedCount, nil
}

func handleSwitchover(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource, switchover *opsv1alpha1.Switchover, opsRequest *opsv1alpha1.OpsRequest, completedCount, failedCount *int32) error {
	switchoverCondition := meta.FindStatusCondition(opsRequest.Status.Conditions, opsv1alpha1.ConditionTypeSwitchover)
	if switchoverCondition == nil {
		return errors.New("switchover condition is nil")
	}

	// Skip if the component does not need switchover
	reason := opsRequest.Status.Components[switchover.ComponentName].Reason
	if reason == OpsReasonForSkipSwitchover {
		*completedCount++
		return nil
	}

	detail := opsv1alpha1.ProgressStatusDetail{
		ObjectKey: getProgressObjectKey(KBSwitchoverDoNCheckRoleChangeKey, switchover.ComponentName),
		Status:    opsv1alpha1.ProcessingProgressStatus,
		Message:   fmt.Sprintf("do switchover for component %s and check role label", switchover.ComponentName),
	}

	synthesizedComp, err := buildSynthesizedComp(reqCtx.Ctx, cli, opsRes, opsRes.Cluster.Spec.GetComponentByName(switchover.ComponentName))
	if err != nil {
		return handleError(reqCtx, opsRequest, &detail, switchover.ComponentName, fmt.Sprintf("build synthesizedComponent failed: %s", err.Error()), failedCount)
	}

	compDef, err := component.GetCompDefByName(reqCtx.Ctx, cli, synthesizedComp.CompDefName)
	if err != nil {
		return handleError(reqCtx, opsRequest, &detail, switchover.ComponentName, fmt.Sprintf("get component definition failed: %s", err.Error()), failedCount)
	}

	synthesizedComp.TemplateVars, _, err = component.ResolveTemplateNEnvVars(reqCtx.Ctx, cli, synthesizedComp, compDef.Spec.Vars)
	if err != nil {
		return handleError(reqCtx, opsRequest, &detail, switchover.ComponentName, fmt.Sprintf("build synthesizedComponent template vars failed: %s", err.Error()), failedCount)
	}

	if err = doSwitchover(reqCtx.Ctx, cli, synthesizedComp, switchover, switchoverCondition); err != nil {
		return handleError(reqCtx, opsRequest, &detail, switchover.ComponentName, fmt.Sprintf("call switchover action and check role label failed: %s", err.Error()), failedCount)
	}

	*completedCount++
	detail.Message = fmt.Sprintf("do switchover for component %s succeeded", switchover.ComponentName)
	detail.Status = opsv1alpha1.SucceedProgressStatus
	setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1.RunningComponentPhase, detail, switchover.ComponentName)
	return nil
}

func doSwitchover(ctx context.Context, cli client.Reader, synthesizedComp *component.SynthesizedComponent,
	switchover *opsv1alpha1.Switchover, switchoverCondition *metav1.Condition) error {
	consistency, err := checkPodRoleLabelConsistency(ctx, cli, *synthesizedComp, switchover, switchoverCondition)
	if err != nil {
		return err
	}
	if consistency {
		return nil
	}

	pods, err := component.ListOwnedPods(ctx, cli, synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name)
	if err != nil {
		return err
	}

	lfa, err := lifecycle.New(synthesizedComp, nil, pods...)
	if err != nil {
		return err
	}

	var candidate string
	if switchover.InstanceName == KBSwitchoverCandidateInstanceForAnyPod {
		candidate = ""
	} else {
		candidate = switchover.InstanceName
	}
	err = lfa.Switchover(ctx, cli, nil, candidate)
	if err != nil {
		return err
	} else {
		return fmt.Errorf("switchover succeed, wait role label to be updated")
	}
}

// setComponentSwitchoverProgressDetails sets component switchover progress details.
func setComponentSwitchoverProgressDetails(recorder record.EventRecorder,
	opsRequest *opsv1alpha1.OpsRequest,
	phase appsv1.ComponentPhase,
	processDetail opsv1alpha1.ProgressStatusDetail,
	componentName string) {
	componentProcessDetails := opsRequest.Status.Components[componentName].ProgressDetails
	setComponentStatusProgressDetail(recorder, opsRequest, &componentProcessDetails, processDetail)
	opsRequest.Status.Components[componentName] = opsv1alpha1.OpsRequestComponentStatus{
		Phase:           phase,
		ProgressDetails: componentProcessDetails,
	}
}

func buildSynthesizedComp(ctx context.Context, cli client.Client, opsRes *OpsResource, clusterCompSpec *appsv1.ClusterComponentSpec) (*component.SynthesizedComponent, error) {
	compObj, compDefObj, err := component.GetCompNCompDefByName(ctx, cli,
		opsRes.Cluster.Namespace, constant.GenerateClusterComponentName(opsRes.Cluster.Name, clusterCompSpec.Name))
	if err != nil {
		return nil, err
	}
	// build synthesized component for the component
	return component.BuildSynthesizedComponent(ctx, cli, compDefObj, compObj, opsRes.Cluster)
}

func handleError(reqCtx intctrlutil.RequestCtx, opsRequest *opsv1alpha1.OpsRequest, detail *opsv1alpha1.ProgressStatusDetail, componentName, errorMsg string, failedCount *int32) error {
	*failedCount++
	detail.Message = fmt.Sprintf("component %s %s", componentName, errorMsg)
	detail.Status = opsv1alpha1.FailedProgressStatus
	setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1.UpdatingComponentPhase, *detail, componentName)
	return nil
}
