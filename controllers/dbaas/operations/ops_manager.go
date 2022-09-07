/*
Copyright 2022.

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
	"context"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var (
	opsManagerOnce sync.Once
	opsManager     *OpsManager
	clusterLockIns = &ClusterLock{}
)

type ClusterLock struct {
	clusterLockMap map[string]*sync.Mutex

	mu sync.Mutex
}

type OpsBehaviour struct {
	FromClusterPhases []dbaasv1alpha1.Phase
	ToClusterPhase    dbaasv1alpha1.Phase
	// Action The action running time should be short. if it fails, it will be reconciled by the OpsRequest controller.
	// if you do not want to be reconciled when the operation fails,
	// you need to call PatchOpsStatus function in ops_util.go and set OpsRequest.status.phase to Failed
	Action func(opsResource *OpsResource) error
	// ReconcileAction loop until the operation is completed when OpsRequest.status.phase is Running
	ReconcileAction func(opsResource *OpsResource) error
	// ActionStartedCondition append to OpsRequest.status.conditions when start performing Action function
	ActionStartedCondition func(opsRequest *dbaasv1alpha1.OpsRequest) *metav1.Condition
}

type OpsResource struct {
	Ctx        context.Context
	Client     client.Client
	OpsRequest *dbaasv1alpha1.OpsRequest
	Cluster    *dbaasv1alpha1.Cluster
	Recorder   record.EventRecorder
}

type OpsManager struct {
	OpsMap map[dbaasv1alpha1.OpsType]*OpsBehaviour
}

func (cl *ClusterLock) GetLock(cluster *dbaasv1alpha1.Cluster) *sync.Mutex {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if cl.clusterLockMap == nil {
		cl.clusterLockMap = map[string]*sync.Mutex{}
	}
	if mu, ok := cl.clusterLockMap[cluster.Namespace]; ok {
		return mu
	} else {
		mu = &sync.Mutex{}
		cl.clusterLockMap[cluster.Name] = mu
		return mu
	}
}

// RegisterOps register operation with OpsType and OpsBehaviour
func (opsMgr *OpsManager) RegisterOps(opsType dbaasv1alpha1.OpsType, opsBehaviour *OpsBehaviour) {
	opsManager.OpsMap[opsType] = opsBehaviour
}

// MainEnter the common entry function for handling OpsRequest
func (opsMgr *OpsManager) MainEnter(opsRes *OpsResource) (*ctrl.Result, error) {
	var (
		opsBehaviour *OpsBehaviour
		err          error
		ok           bool
	)

	clusterMu := clusterLockIns.GetLock(opsRes.Cluster)
	clusterMu.Lock()
	defer clusterMu.Unlock()

	if opsBehaviour, ok = opsMgr.OpsMap[opsRes.OpsRequest.Spec.Type]; !ok {
		return nil, patchOpsBehaviourNotFound(opsRes)
	}

	if ok, err = opsMgr.validateClusterPhaseAndOperations(opsRes, opsBehaviour); err != nil || !ok {
		return nil, err
	}

	if opsRes.OpsRequest.Status.Phase != dbaasv1alpha1.RunningPhase {
		return &reconcile.Result{}, patchOpsRequestToRunning(opsRes, opsBehaviour)
	}

	if opsBehaviour.ToClusterPhase != "" {
		// If the operation cause the cluster state to change,
		// the cluster needs to be locked. At the same time, only one operation is running
		if err = addOpsRequestAnnotationToCluster(opsRes); err != nil {
			return nil, err
		}
	}

	if opsBehaviour.Action == nil {
		return nil, nil
	}
	if err = opsBehaviour.Action(opsRes); err != nil {
		return nil, err
	}

	// patch cluster.status should after update cluster.spec
	// because cluster controller probably reconciled status.phase to Running if cluster no updating
	return nil, patchClusterStatus(opsRes, opsBehaviour.ToClusterPhase)
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

	if opsBehaviour != nil && opsBehaviour.ReconcileAction != nil {
		if err = opsBehaviour.ReconcileAction(opsRes); err != nil {
			return err
		}
	}
	return PatchOpsStatus(opsRes, dbaasv1alpha1.SucceedPhase, dbaasv1alpha1.NewSucceedCondition(opsRequest))
}

// validateClusterPhase validate Cluster.status.phase is in opsBehaviour.FromClusterPhases or OpsRequest is reentry
func (opsMgr *OpsManager) validateClusterPhaseAndOperations(opsRes *OpsResource, behaviour *OpsBehaviour) (bool, error) {
	isOkClusterPhase := false
	for _, v := range behaviour.FromClusterPhases {
		if opsRes.Cluster.Status.Phase == v {
			isOkClusterPhase = true
			break
		}
	}
	opsRequestAnnotation := getOpsRequestAnnotation(opsRes.Cluster)
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
