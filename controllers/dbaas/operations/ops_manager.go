/*
Copyright ApeCloud Inc.

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

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var (
	opsManagerOnce sync.Once
	opsManager     *OpsManager
)

// RegisterOps register operation with OpsType and OpsBehaviour
func (opsMgr *OpsManager) RegisterOps(opsType dbaasv1alpha1.OpsType, opsBehaviour *OpsBehaviour) {
	opsManager.OpsMap[opsType] = opsBehaviour
	dbaasv1alpha1.OpsRequestBehaviourMapper[opsType] = dbaasv1alpha1.OpsRequestBehaviour{
		FromClusterPhases: opsBehaviour.FromClusterPhases,
		ToClusterPhase:    opsBehaviour.ToClusterPhase,
	}
}

// Do the common entry function for handling OpsRequest
func (opsMgr *OpsManager) Do(opsRes *OpsResource) error {
	var (
		opsBehaviour *OpsBehaviour
		err          error
		ok           bool
		opsRequest   = opsRes.OpsRequest
	)

	if opsBehaviour, ok = opsMgr.OpsMap[opsRequest.Spec.Type]; !ok {
		return patchOpsBehaviourNotFound(opsRes)
	} else if opsBehaviour.Action == nil {
		return nil
	}
	if ok, err = opsMgr.validateClusterPhaseAndOperations(opsRes, opsBehaviour); err != nil || !ok {
		return err
	}

	if opsRequest.Status.Phase != dbaasv1alpha1.RunningPhase {
		if err = patchOpsRequestToRunning(opsRes, opsBehaviour); err != nil {
			return err
		}
	}

	// If the operation cause the cluster state to change, the cluster needs to be locked.
	// At the same time, only one operation is running if these operations is mutex(exists same opsBehaviour.ToClusterPhase).
	if err = addOpsRequestAnnotationToCluster(opsRes, opsBehaviour.ToClusterPhase); err != nil {
		return err
	}

	if err = opsBehaviour.Action(opsRes); err != nil {
		return err
	}
	// patch cluster.status after update cluster.spec
	// because cluster controller probably reconciled status.phase to Running if cluster no updating
	return patchClusterStatus(opsRes, opsBehaviour.ToClusterPhase)
}

// Reconcile entry function when OpsRequest.status.phase is Running.
// loop until the operation is completed.
func (opsMgr *OpsManager) Reconcile(opsRes *OpsResource) (time.Duration, error) {
	var (
		opsBehaviour    *OpsBehaviour
		ok              bool
		err             error
		requeueAfter    time.Duration
		opsRequestPhase dbaasv1alpha1.Phase
		opsRequest      = opsRes.OpsRequest
	)

	if opsRes.OpsRequest.Status.Phase != dbaasv1alpha1.RunningPhase {
		return requeueAfter, nil
	}

	if opsBehaviour, ok = opsMgr.OpsMap[opsRes.OpsRequest.Spec.Type]; !ok {
		return requeueAfter, patchOpsBehaviourNotFound(opsRes)
	}

	if opsBehaviour == nil || opsBehaviour.ReconcileAction == nil {
		return requeueAfter, nil
	}
	if opsRequestPhase, requeueAfter, err = opsBehaviour.ReconcileAction(opsRes); err != nil && !isOpsRequestFailedPhase(opsRequestPhase) {
		// if the opsRequest phase is Failed, skipped
		return requeueAfter, err
	}
	switch opsRequestPhase {
	case dbaasv1alpha1.SucceedPhase:
		return requeueAfter, PatchOpsStatus(opsRes, opsRequestPhase, dbaasv1alpha1.NewSucceedCondition(opsRequest))
	case dbaasv1alpha1.FailedPhase:
		return requeueAfter, PatchOpsStatus(opsRes, opsRequestPhase, dbaasv1alpha1.NewFailedCondition(opsRequest, err))
	default:
		return requeueAfter, nil
	}
}

// validateClusterPhase validate Cluster.status.phase is in opsBehaviour.FromClusterPhases or OpsRequest is reentry
func (opsMgr *OpsManager) validateClusterPhaseAndOperations(opsRes *OpsResource, behaviour *OpsBehaviour) (bool, error) {
	isOkClusterPhase := false
	for _, v := range behaviour.FromClusterPhases {
		if opsRes.Cluster.Status.Phase != v {
			continue
		}
		isOkClusterPhase = true
		break
	}
	opsRequestName := getOpsRequestNameFromAnnotation(opsRes.Cluster, behaviour.ToClusterPhase)
	if behaviour.ToClusterPhase != "" && opsRequestName != "" {
		// OpsRequest is reentry
		if opsRequestName == opsRes.OpsRequest.Name {
			return true, nil
		}
		return false, patchClusterExistOtherOperation(opsRes, opsRequestName)
	}

	if !isOkClusterPhase {
		return false, patchClusterPhaseMisMatch(opsRes)
	}
	return true, nil
}

func GetOpsManager() *OpsManager {
	opsManagerOnce.Do(func() {
		opsManager = &OpsManager{OpsMap: make(map[dbaasv1alpha1.OpsType]*OpsBehaviour)}
	})
	return opsManager
}
