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
	"slices"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	intctrlcomp "github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type StopOpsHandler struct{}

var _ OpsHandler = StopOpsHandler{}

func init() {
	stopBehaviour := OpsBehaviour{
		FromClusterPhases: append(appsv1.GetClusterUpRunningPhases(), appsv1.UpdatingClusterPhase),
		ToClusterPhase:    appsv1.StoppingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        StopOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.StopType, stopBehaviour)
}

// ActionStartedCondition the started condition when handling the stop request.
func (stop StopOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewStopCondition(opsRes.OpsRequest), nil
}

// Action modifies Cluster.spec.components[*].replicas from the opsRequest
func (stop StopOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	var (
		cluster = opsRes.Cluster
	)

	// if the cluster is already stopping or stopped, return
	if slices.Contains([]appsv1.ClusterPhase{appsv1.StoppedClusterPhase,
		appsv1.StoppingClusterPhase}, opsRes.Cluster.Status.Phase) {
		return nil
	}

	// abort earlier running vertical scaling opsRequest.
	if err := abortEarlierOpsRequestWithSameKind(reqCtx, cli, opsRes, []opsv1alpha1.OpsType{opsv1alpha1.HorizontalScalingType,
		opsv1alpha1.StartType, opsv1alpha1.RestartType, opsv1alpha1.VerticalScalingType},
		func(earlierOps *opsv1alpha1.OpsRequest) (bool, error) {
			return true, nil
		}); err != nil {
		return err
	}

	stopComp := func(compSpec *appsv1.ClusterComponentSpec) {
		compSpec.Stop = func() *bool { b := true; return &b }()
	}

	for i := range cluster.Spec.ComponentSpecs {
		stopComp(&cluster.Spec.ComponentSpecs[i])
	}
	for i := range cluster.Spec.ShardingSpecs {
		stopComp(&cluster.Spec.ShardingSpecs[i].Template)
	}
	return cli.Update(reqCtx.Ctx, cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for stop opsRequest.
func (stop StopOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	handleComponentProgress := func(reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes *progressResource,
		compStatus *opsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
		var err error
		pgRes.deletedPodSet, err = intctrlcomp.GenerateAllPodNamesToSet(pgRes.clusterComponent.Replicas, pgRes.clusterComponent.Instances,
			pgRes.clusterComponent.OfflineInstances, opsRes.Cluster.Name, pgRes.fullComponentName)
		if err != nil {
			return 0, 0, err
		}
		expectProgressCount, completedCount, err := handleComponentProgressForScalingReplicas(reqCtx, cli, opsRes, pgRes, compStatus)
		if err != nil {
			return expectProgressCount, completedCount, err
		}
		return expectProgressCount, completedCount, nil
	}
	compOpsHelper := newComponentOpsHelper([]opsv1alpha1.ComponentOps{})
	return compOpsHelper.reconcileActionWithComponentOps(reqCtx, cli, opsRes, "stop", handleComponentProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (stop StopOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}
