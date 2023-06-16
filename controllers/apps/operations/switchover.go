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
	"fmt"
	"reflect"
	"time"

	"encoding/json"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	componentutil "github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type switchoverOpsHandler struct{}

var _ OpsHandler = switchoverOpsHandler{}

// SwitchoverMessage is the OpsRequest.Status.Condition.Message for switchover.
type SwitchoverMessage struct {
	appsv1alpha1.Switchover
	OldPrimaryOrLeader string
	Cluster            string
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
			Switchover:         switchover,
			OldPrimaryOrLeader: pod.Name,
			Cluster:            opsRes.Cluster.Name,
		}
	}
	msg, err := json.Marshal(switchoverMessageMap)
	if err != nil {
		return nil, err
	}
	return appsv1alpha1.NewSwitchoveringCondition(opsRes.Cluster.Generation, string(msg)), nil
}

// Action restarts components by updating StatefulSet.
func (r switchoverOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	if opsRes.OpsRequest.Status.StartTimestamp.IsZero() {
		return errors.New("status.startTimestamp can not be null")
	}
	switchoverMap := opsRes.OpsRequest.Spec.ToSwitchoverListToMap()
	return doSwitchoverComponents(reqCtx, cli, opsRes, switchoverMap)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for restart opsRequest.
func (r switchoverOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	var (
		opsRequest          = opsRes.OpsRequest
		oldOpsRequestStatus = opsRequest.Status.DeepCopy()
		opsRequestPhase     = appsv1alpha1.OpsRunningPhase
	)

	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = make(map[string]appsv1alpha1.OpsRequestComponentStatus)
		for _, v := range opsRequest.Spec.SwitchoverList {
			opsRequest.Status.Components[v.ComponentName] = appsv1alpha1.OpsRequestComponentStatus{
				Phase: appsv1alpha1.SpecReconcilingClusterCompPhase,
			}
		}
	}

	expectCount, actualCount, err := handleSwitchoverProgress(reqCtx, cli, opsRes)
	if err != nil {
		return "", 0, err
	}

	opsRequest.Status.Progress = fmt.Sprintf("%d/%d", actualCount, expectCount)
	// patch OpsRequest.status.components
	if !reflect.DeepEqual(*oldOpsRequestStatus, opsRequest.Status) {
		if err := cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return opsRequestPhase, 0, err
		}
	}

	if actualCount == actualCount {
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
		if err := createSwitchoverJob(reqCtx, cli, opsRes.Cluster, opsRes.Cluster.Spec.GetComponentByName(compSpecName), compDef, &switchover); err != nil {
			return err
		}
	}
	return nil
}

// handleComponentProgressDetails handles the component progressDetails when switchover.
// @return expectProgressCount,
// @return completedCount
// @return error
func handleSwitchoverProgress(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (int32, int32, error) {
	var (
		expectCount    = int32(len(opsRes.OpsRequest.Spec.SwitchoverList))
		completedCount int32
	)
	succeedJobCount := make([]string, 0, len(opsRes.OpsRequest.Spec.SwitchoverList))
	switchoverMap := opsRes.OpsRequest.Spec.ToSwitchoverListToMap()
	for compSpecName, switchover := range switchoverMap {
		switchoverCondition := meta.FindStatusCondition(opsRes.OpsRequest.Status.Conditions, appsv1alpha1.ConditionTypeSwitchover)
		if switchoverCondition == nil {
			return 0, 0, errors.New("switchover condition is nil")
		}
		jobName := fmt.Sprintf("%s-%s-%s-%d", constant.KBSwitchoverJobNamePrefix, opsRes.Cluster.Name, compSpecName, switchoverCondition.ObservedGeneration)
		// check the current generation switchoverJob whether succeed
		if err := checkJobSucceed(reqCtx.Ctx, cli, opsRes.Cluster, jobName); err != nil {
			return 0, 0, err
		}
		compDef, err := componentutil.GetComponentDefByCluster(reqCtx.Ctx, cli, *opsRes.Cluster, compSpecName)
		if err != nil {
			return 0, 0, err
		}
		consistency, err := checkPodRoleLabelConsistency(reqCtx.Ctx, cli, opsRes.Cluster, opsRes.Cluster.Spec.GetComponentByName(compSpecName), compDef, &switchover, switchoverCondition)
		if err != nil {
			return 0, 0, err
		}

		if !consistency {
			return expectCount, 0, intctrlutil.NewErrorf(intctrlutil.ErrorWaitCacheRefresh, "requeue to waiting for pod role label consistency.")
		}

		completedCount += 1
		succeedJobCount = append(succeedJobCount, jobName)
		p := opsRes.OpsRequest.Status.Components[compSpecName]
		p.Phase = appsv1alpha1.RunningClusterCompPhase
	}

	if completedCount == int32(expectCount) {
		for _, jobName := range succeedJobCount {
			_ = cleanJobByName(reqCtx.Ctx, cli, opsRes.Cluster, jobName)
		}
	}

	return expectCount, completedCount, nil
}
