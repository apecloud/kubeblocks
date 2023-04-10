/*
Copyright ApeCloud, Inc.

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

// RegisterOps register operation with OpsType and OpsBehaviour
func (opsMgr *OpsManager) RegisterOps(opsType appsv1alpha1.OpsType, opsBehaviour OpsBehaviour) {
	opsManager.OpsMap[opsType] = opsBehaviour
	appsv1alpha1.OpsRequestBehaviourMapper[opsType] = appsv1alpha1.OpsRequestBehaviour{
		FromClusterPhases:                  opsBehaviour.FromClusterPhases,
		ToClusterPhase:                     opsBehaviour.ToClusterPhase,
		ProcessingReasonInClusterCondition: opsBehaviour.ProcessingReasonInClusterCondition,
	}
}

// Do the common entry function for handling OpsRequest
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
		if patchErr := PatchValidateErrorCondition(reqCtx.Ctx, cli, opsRes, err.Error()); patchErr != nil {
			return nil, patchErr
		}
		return nil, err
	}
	if opsRequest.Status.Phase != appsv1alpha1.OpsCreatingPhase {
		// If the operation causes the cluster phase to change, the cluster needs to be locked.
		// At the same time, only one operation is running if these operations are mutex(exist opsBehaviour.ToClusterPhase).
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
	if opsRequestPhase, requeueAfter, err = opsBehaviour.OpsHandler.ReconcileAction(reqCtx, cli, opsRes); err != nil && !isOpsRequestFailedPhase(opsRequestPhase) {
		// if the opsRequest phase is Failed, skipped
		return requeueAfter, err
	}
	switch opsRequestPhase {
	case appsv1alpha1.OpsSucceedPhase:
		return requeueAfter, PatchOpsStatus(reqCtx.Ctx, cli, opsRes, opsRequestPhase, appsv1alpha1.NewSucceedCondition(opsRequest))
	case appsv1alpha1.OpsFailedPhase:
		return requeueAfter, PatchOpsStatus(reqCtx.Ctx, cli, opsRes, opsRequestPhase, appsv1alpha1.NewFailedCondition(opsRequest, err))
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
