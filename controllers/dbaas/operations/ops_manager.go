/*
Copyright 2022 The Kubeblocks Authors

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

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var (
	opsManagerOnce sync.Once
	opsManager     *OpsManager
)

// RegisterOps register operation with OpsType and OpsBehaviour
func (opsMgr *OpsManager) RegisterOps(opsType dbaasv1alpha1.OpsType, opsBehaviour *OpsBehaviour) {
	opsManager.OpsMap[opsType] = opsBehaviour
}

// MainEnter the common entry function for handling OpsRequest
func (opsMgr *OpsManager) MainEnter(opsRes *OpsResource) error {
	var (
		opsBehaviour *OpsBehaviour
		err          error
		ok           bool
	)

	if opsBehaviour, ok = opsMgr.OpsMap[opsRes.OpsRequest.Spec.Type]; !ok {
		return patchOpsBehaviourNotFound(opsRes)
	} else if opsBehaviour.Action == nil {
		return nil
	}
	if ok, err = opsMgr.validateClusterPhaseAndOperations(opsRes, opsBehaviour); err != nil || !ok {
		return err
	}

	if opsRes.OpsRequest.Status.Phase != dbaasv1alpha1.RunningPhase {
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

// ReconcileMainEnter reconcile entry function when OpsRequest.status.phase is Running.
// loop until the operation is completed.
func (opsMgr *OpsManager) ReconcileMainEnter(opsRes *OpsResource) error {
	var (
		opsBehaviour *OpsBehaviour
		ok           bool
		err          error
		opsRequest   = opsRes.OpsRequest
	)

	if opsRes.OpsRequest.Status.Phase != dbaasv1alpha1.RunningPhase {
		return nil
	}

	if opsBehaviour, ok = opsMgr.OpsMap[opsRes.OpsRequest.Spec.Type]; !ok {
		return patchOpsBehaviourNotFound(opsRes)
	}

	if opsBehaviour == nil || opsBehaviour.ReconcileAction == nil {
		return nil
	}
	if err = opsBehaviour.ReconcileAction(opsRes); err != nil {
		return err
	}
	return PatchOpsStatus(opsRes, dbaasv1alpha1.SucceedPhase, dbaasv1alpha1.NewSucceedCondition(opsRequest))
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
	opsRequestAnnotation := getOpsRequestAnnotation(opsRes.Cluster, behaviour.ToClusterPhase)
	if behaviour.ToClusterPhase != "" && opsRequestAnnotation != nil {
		// OpsRequest is reentry
		if *opsRequestAnnotation == opsRes.OpsRequest.Name {
			return true, nil
		}
		return false, patchClusterExistOtherOperation(opsRes, *opsRequestAnnotation)
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
