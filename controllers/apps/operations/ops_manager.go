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
	"sync"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

var (
	opsManagerOnce sync.Once
	opsManager     *OpsManager
)

// RegisterOps registers operation with OpsType and OpsBehaviour
func (opsMgr *OpsManager) RegisterOps(opsType appsv1alpha1.OpsType, opsBehaviour OpsBehaviour) {
	opsManager.OpsMap[opsType] = opsBehaviour
	appsv1alpha1.OpsRequestBehaviourMapper[opsType] = appsv1alpha1.OpsRequestBehaviour{
		FromClusterPhases: opsBehaviour.FromClusterPhases,
		ToClusterPhase:    opsBehaviour.ToClusterPhase,
	}
}

// Do the entry function for handling OpsRequest
func (opsMgr *OpsManager) Do(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*ctrl.Result, error) {
	var (
		opsBehaviour OpsBehaviour
		err          error
		ok           bool
		opsRequest   = opsRes.OpsRequest
	)
	if opsBehaviour, ok = opsMgr.OpsMap[opsRequest.Spec.Type]; !ok || opsBehaviour.OpsHandler == nil {
		return &ctrl.Result{}, PatchOpsHandlerNotSupported(reqCtx.Ctx, cli, opsRes)
	}

	// validate OpsRequest.spec
	// if the operation will create a new cluster, don't validate the cluster
	if err = opsRequest.Validate(reqCtx.Ctx, cli, opsRes.Cluster, !opsBehaviour.IsClusterCreation); err != nil {
		return &ctrl.Result{}, patchValidateErrorCondition(reqCtx.Ctx, cli, opsRes, err.Error())
	}

	if opsRequest.Status.Phase == appsv1alpha1.OpsPendingPhase {
		// validate entry condition for OpsRequest, check if the cluster is in the right phase
		if err = validateOpsWaitingPhase(opsRes.Cluster, opsRequest, opsBehaviour); err != nil {
			// check if the error is caused by WaitForClusterPhaseErr  error
			if _, ok := err.(*WaitForClusterPhaseErr); ok {
				return intctrlutil.ResultToP(intctrlutil.RequeueAfter(time.Second, reqCtx.Log, ""))
			}
			return &ctrl.Result{}, patchValidateErrorCondition(reqCtx.Ctx, cli, opsRes, err.Error())
		}
		if opsBehaviour.ToClusterPhase != "" {
			// if ToClusterPhase is not empty, enqueue OpsRequest to the cluster Annotation.
			opsRecordeSlice, err := enqueueOpsRequestToClusterAnnotation(reqCtx.Ctx, cli, opsRes, opsBehaviour)
			if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
				return &ctrl.Result{}, patchValidateErrorCondition(reqCtx.Ctx, cli, opsRes, err.Error())
			} else if err != nil {
				return nil, err
			}
			// only one operation can be running at a time if these operations are mutually exclusive(exist opsBehaviour.ToClusterPhase).
			// other opsRequest should be reconciled.
			if len(opsRecordeSlice) > 0 && opsRecordeSlice[0].Name != opsRequest.Name {
				return intctrlutil.ResultToP(intctrlutil.Reconciled())
			}
		}
		opsDeepCopy := opsRequest.DeepCopy()
		// save last configuration into status.lastConfiguration
		if err = opsBehaviour.OpsHandler.SaveLastConfiguration(reqCtx, cli, opsRes); err != nil {
			return nil, err
		}

		return &ctrl.Result{}, patchOpsRequestToCreating(reqCtx, cli, opsRes, opsDeepCopy, opsBehaviour.OpsHandler)
	}

	if err = opsBehaviour.OpsHandler.Action(reqCtx, cli, opsRes); err != nil {
		// patch the status.phase to Failed when the error is FastFailError, which means the operation is failed and there is no need to retry
		if _, ok := err.(*FastFailError); ok {
			return &ctrl.Result{}, patchFastFailErrorCondition(reqCtx.Ctx, cli, opsRes, err)
		}
		return nil, err
	}
	return nil, nil
}

// Reconcile entry function when OpsRequest.status.phase is Running.
// loops till the operation is completed.
func (opsMgr *OpsManager) Reconcile(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (time.Duration, error) {
	var (
		opsBehaviour    OpsBehaviour
		ok              bool
		err             error
		requeueAfter    time.Duration
		opsRequestPhase appsv1alpha1.OpsPhase
		opsRequest      = opsRes.OpsRequest
	)

	if opsBehaviour, ok = opsMgr.OpsMap[opsRes.OpsRequest.Spec.Type]; !ok || opsBehaviour.OpsHandler == nil {
		return 0, PatchOpsHandlerNotSupported(reqCtx.Ctx, cli, opsRes)
	}
	opsRes.ToClusterPhase = opsBehaviour.ToClusterPhase
	if opsRequestPhase, requeueAfter, err = opsBehaviour.OpsHandler.ReconcileAction(reqCtx, cli, opsRes); err != nil &&
		!isOpsRequestFailedPhase(opsRequestPhase) {
		// if the opsRequest phase is not failed, skipped
		return requeueAfter, err
	}
	switch opsRequestPhase {
	case appsv1alpha1.OpsSucceedPhase:
		if opsRequest.Status.Phase == appsv1alpha1.OpsCancellingPhase {
			return 0, PatchOpsStatus(reqCtx.Ctx, cli, opsRes, appsv1alpha1.OpsCancelledPhase, appsv1alpha1.NewCancelSucceedCondition(opsRequest.Name))
		}
		return 0, PatchOpsStatus(reqCtx.Ctx, cli, opsRes, opsRequestPhase, appsv1alpha1.NewSucceedCondition(opsRequest))
	case appsv1alpha1.OpsFailedPhase:
		if opsRequest.Status.Phase == appsv1alpha1.OpsCancellingPhase {
			return 0, PatchOpsStatus(reqCtx.Ctx, cli, opsRes, appsv1alpha1.OpsCancelledPhase, appsv1alpha1.NewCancelFailedCondition(opsRequest, err))
		}
		return 0, PatchOpsStatus(reqCtx.Ctx, cli, opsRes, opsRequestPhase, appsv1alpha1.NewFailedCondition(opsRequest, err))
	default:
		return requeueAfter, nil
	}
}

func GetOpsManager() *OpsManager {
	opsManagerOnce.Do(func() {
		opsManager = &OpsManager{OpsMap: make(map[appsv1alpha1.OpsType]OpsBehaviour)}
	})
	return opsManager
}
