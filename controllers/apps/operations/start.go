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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlcomp "github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type StartOpsHandler struct{}

var _ OpsHandler = StartOpsHandler{}

func init() {
	startBehaviour := OpsBehaviour{
		FromClusterPhases: append(appsv1alpha1.GetClusterUpRunningPhases(), appsv1alpha1.UpdatingClusterPhase,
			appsv1alpha1.StoppedClusterPhase, appsv1alpha1.StoppingClusterPhase),
		ToClusterPhase: appsv1alpha1.UpdatingClusterPhase,
		QueueByCluster: true,
		OpsHandler:     StartOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.StartType, startBehaviour)
}

// ActionStartedCondition the started condition when handling the start request.
func (start StartOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewStartCondition(opsRes.OpsRequest), nil
}

// Action modifies Cluster.spec.components[*].replicas from the opsRequest
func (start StartOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	var (
		cluster   = opsRes.Cluster
		startList = opsRes.OpsRequest.Spec.StartList
	)
	compOpsHelper := newComponentOpsHelper(startList)
	// abort earlier running opsRequests.
	if err := abortEarlierOpsRequestWithSameKind(reqCtx, cli, opsRes, []appsv1alpha1.OpsType{appsv1alpha1.StopType},
		func(earlierOps *appsv1alpha1.OpsRequest) (bool, error) {
			if len(startList) == 0 {
				// start all components
				return true, nil
			}
			return len(earlierOps.Spec.StopList) == 0 || hasIntersectionCompOpsList(compOpsHelper.componentOpsSet, earlierOps.Spec.StopList), nil
		}); err != nil {
		return err
	}
	startComp := func(compSpec *appsv1alpha1.ClusterComponentSpec, clusterCompName string) {
		if len(startList) > 0 {
			if _, ok := compOpsHelper.componentOpsSet[clusterCompName]; !ok {
				return
			}
		}
		compSpec.Stop = nil
	}
	for i, v := range cluster.Spec.ComponentSpecs {
		startComp(&cluster.Spec.ComponentSpecs[i], v.Name)
	}
	for i, v := range cluster.Spec.ShardingSpecs {
		startComp(&cluster.Spec.ShardingSpecs[i].Template, v.Name)
	}
	return cli.Update(reqCtx.Ctx, cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for start opsRequest.
func (start StartOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	handleComponentProgress := func(reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes *progressResource,
		compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
		var err error
		pgRes.createdPodSet, err = intctrlcomp.GenerateAllPodNamesToSet(pgRes.clusterComponent.Replicas, pgRes.clusterComponent.Instances,
			pgRes.clusterComponent.OfflineInstances, opsRes.Cluster.Name, pgRes.fullComponentName)
		if err != nil {
			return 0, 0, err
		}
		return handleComponentProgressForScalingReplicas(reqCtx, cli, opsRes, pgRes, compStatus)
	}
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.StartList)
	return compOpsHelper.reconcileActionWithComponentOps(reqCtx, cli, opsRes, "start", handleComponentProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (start StartOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}
