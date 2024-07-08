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
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type restartOpsHandler struct{}

var _ OpsHandler = restartOpsHandler{}

func init() {
	restartBehaviour := OpsBehaviour{
		// if cluster is Abnormal or Failed, new opsRequest may repair it.
		FromClusterPhases: appsv1alpha1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1alpha1.UpdatingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        restartOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.RestartType, restartBehaviour)
}

// ActionStartedCondition the started condition when handle the restart request.
func (r restartOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewRestartingCondition(opsRes.OpsRequest), nil
}

// Action restarts components by updating StatefulSet.
func (r restartOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	if opsRes.OpsRequest.Status.StartTimestamp.IsZero() {
		return fmt.Errorf("status.startTimestamp can not be null")
	}
	// abort earlier running vertical scaling opsRequest.
	if err := abortEarlierOpsRequestWithSameKind(reqCtx, cli, opsRes, []appsv1alpha1.OpsType{appsv1alpha1.RestartType},
		func(earlierOps *appsv1alpha1.OpsRequest) (bool, error) {
			return true, nil
		}); err != nil {
		return err
	}

	match := func(compName string) *appsv1alpha1.ClusterComponentSpec {
		for i, spec := range opsRes.Cluster.Spec.ComponentSpecs {
			if spec.Name == compName {
				return &opsRes.Cluster.Spec.ComponentSpecs[i]
			}
		}
		for i, spec := range opsRes.Cluster.Spec.ShardingSpecs {
			if spec.Name == compName {
				return &opsRes.Cluster.Spec.ShardingSpecs[i].Template
			}
		}
		return nil
	}
	restart := func(spec *appsv1alpha1.ClusterComponentSpec) {
		spec.State = &appsv1alpha1.State{
			Mode:       appsv1alpha1.StateModeRunning,
			Generation: func() *int64 { g := opsRes.Cluster.Generation + 1; return &g }(),
		}
	}
	for _, comp := range opsRes.OpsRequest.Spec.RestartList {
		if spec := match(comp.ComponentName); spec != nil {
			restart(spec)
		}
	}
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for restart opsRequest.
func (r restartOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	helper := newComponentOpsHelper(opsRes.OpsRequest.Spec.RestartList)
	return helper.reconcileActionWithComponentOps(reqCtx, cli, opsRes, "restart", r.progress)
}

// SaveLastConfiguration this operation only restart the pods of the component, no changes for Cluster.spec.
// empty implementation here.
func (r restartOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func (r restartOpsHandler) progress(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	pgRes *progressResource,
	_ *appsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
	// TODO: generation check
	itsKey := types.NamespacedName{
		Namespace: opsRes.Cluster.Namespace,
		Name:      constant.GenerateWorkloadNamePattern(opsRes.Cluster.Name, pgRes.fullComponentName),
	}
	its := &workloads.InstanceSet{}
	if err := cli.Get(reqCtx.Ctx, itsKey, its); err != nil {
		return 0, 0, err
	}

	if its.Spec.Replicas == nil {
		return 0, 0, fmt.Errorf("its.spec.replicas is nil")
	}
	if its.Generation != its.Status.ObservedGeneration {
		return 0, 0, fmt.Errorf("its is still in progress, generation: %d, observed: %d", its.Generation, its.Status.ObservedGeneration)
	}

	expected := pgRes.clusterComponent.Replicas
	if expected != *its.Spec.Replicas {
		return 0, 0, fmt.Errorf("its spec has not updated yet, expected: %d, actual: %d", expected, *its.Spec.Replicas)
	}
	// TODO: ready?
	return expected, its.Status.UpdatedReplicas, nil
}
