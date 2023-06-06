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
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

var (
	opsManagerOnce sync.Once
	opsManager     *OpsManager
)

// RegisterOps registers operation with OpsType and OpsBehaviour
func (opsMgr *OpsManager) RegisterOps(opsType appsv1alpha1.OpsType, opsBehaviour OpsBehaviour) {
	opsManager.OpsMap[opsType] = opsBehaviour
	appsv1alpha1.OpsRequestBehaviourMapper[opsType] = appsv1alpha1.OpsRequestBehaviour{
		FromClusterPhases:                  opsBehaviour.FromClusterPhases,
		ToClusterPhase:                     opsBehaviour.ToClusterPhase,
		ProcessingReasonInClusterCondition: opsBehaviour.ProcessingReasonInClusterCondition,
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
		return nil, patchOpsHandlerNotSupported(reqCtx.Ctx, cli, opsRes)
	}

	// validate OpsRequest.spec
	if err = opsRequest.Validate(reqCtx.Ctx, cli, opsRes.Cluster, true); err != nil {
		if patchErr := patchValidateErrorCondition(reqCtx.Ctx, cli, opsRes, err.Error()); patchErr != nil {
			return nil, patchErr
		}
		return nil, err
	}
	if opsRequest.Status.Phase != appsv1alpha1.OpsCreatingPhase {
		// If the operation causes the cluster phase to change, the cluster needs to be locked.
		// At the same time, only one operation is running if these operations are mutually exclusive(exist opsBehaviour.ToClusterPhase).
		if err = addOpsRequestAnnotationToCluster(reqCtx.Ctx, cli, opsRes, opsBehaviour); err != nil {
			return nil, err
		}

		opsDeepCopy := opsRequest.DeepCopy()
		// save last configuration into status.lastConfiguration
		if err = opsBehaviour.OpsHandler.SaveLastConfiguration(reqCtx, cli, opsRes); err != nil {
			return nil, err
		}

		return &ctrl.Result{}, patchOpsRequestToCreating(reqCtx.Ctx, cli, opsRes, opsDeepCopy, opsBehaviour.OpsHandler)
	}
	if err = opsBehaviour.OpsHandler.Action(reqCtx, cli, opsRes); err != nil {
		return nil, err
	}

	// patch cluster.status after updating cluster.spec.
	// because cluster controller probably reconciles status.phase to Running if cluster is not updated.
	return nil, patchClusterStatusAndRecordEvent(reqCtx, cli, opsRes, opsBehaviour)
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
		return 0, patchOpsHandlerNotSupported(reqCtx.Ctx, cli, opsRes)
	}
	opsRes.ToClusterPhase = opsBehaviour.ToClusterPhase
	if opsRequestPhase, requeueAfter, err = opsBehaviour.OpsHandler.ReconcileAction(reqCtx, cli, opsRes); err != nil &&
		!isOpsRequestFailedPhase(opsRequestPhase) {
		// if the opsRequest phase is Failed, skipped
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
