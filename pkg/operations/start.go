/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type StartOpsHandler struct{}

var _ OpsHandler = StartOpsHandler{}

func init() {
	startBehaviour := OpsBehaviour{
		FromClusterPhases: append(appsv1.GetClusterUpRunningPhases(), appsv1.UpdatingClusterPhase,
			appsv1.StoppedClusterPhase, appsv1.StoppingClusterPhase),
		ToClusterPhase: appsv1.UpdatingClusterPhase,
		QueueByCluster: true,
		OpsHandler:     StartOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.StartType, startBehaviour)
}

// ActionStartedCondition the started condition when handling the start request.
func (start StartOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewStartCondition(opsRes.OpsRequest), nil
}

// Action clears Cluster.spec.components[*].stop for the requested components.
func (start StartOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	var (
		cluster   = opsRes.Cluster
		startList = opsRes.OpsRequest.Spec.StartList
	)
	compOpsHelper := newComponentOpsHelper(startList)
	// abort earlier running opsRequests.
	if err := abortEarlierOpsRequestWithSameKind(reqCtx, cli, opsRes, []opsv1alpha1.OpsType{opsv1alpha1.StopType},
		func(earlierOps *opsv1alpha1.OpsRequest) (bool, error) {
			if len(startList) == 0 {
				// start all components
				return true, nil
			}
			return len(earlierOps.Spec.StopList) == 0 || hasIntersectionCompOpsList(compOpsHelper.componentOpsSet, earlierOps.Spec.StopList), nil
		}); err != nil {
		return err
	}
	startComp := func(compSpec *appsv1.ClusterComponentSpec, clusterCompName string) {
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
	for i, v := range cluster.Spec.Shardings {
		startComp(&cluster.Spec.Shardings[i].Template, v.Name)
	}
	return cli.Update(reqCtx.Ctx, cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for start opsRequest.
func (start StartOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	if !start.targetsStarted(opsRes) {
		return opsv1alpha1.OpsAbortedPhase, 0, nil
	}
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.StartList)
	return compOpsHelper.reconcileRollingAction(reqCtx, cli, opsRes,
		"start", handleRunningProgress, appsv1.RunningComponentPhase)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (start StartOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func (start StartOpsHandler) targetsStarted(opsRes *OpsResource) bool {
	if opsRes == nil || opsRes.Cluster == nil || opsRes.OpsRequest == nil {
		return false
	}
	startList := opsRes.OpsRequest.Spec.StartList
	if len(startList) > 0 {
		for i := range startList {
			compSpec := getComponentSpecOrShardingTemplate(opsRes.Cluster, startList[i].ComponentName)
			if compSpec == nil || ptr.Deref(compSpec.Stop, false) {
				return false
			}
		}
		return true
	}
	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		if ptr.Deref(opsRes.Cluster.Spec.ComponentSpecs[i].Stop, false) {
			return false
		}
	}
	for i := range opsRes.Cluster.Spec.Shardings {
		if ptr.Deref(opsRes.Cluster.Spec.Shardings[i].Template.Stop, false) {
			return false
		}
	}
	return true
}
