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
	"encoding/json"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type StopOpsHandler struct{}

var _ OpsHandler = StopOpsHandler{}

func init() {
	stopBehaviour := OpsBehaviour{
		FromClusterPhases: append(appsv1alpha1.GetClusterUpRunningPhases(), appsv1alpha1.UpdatingClusterPhase),
		ToClusterPhase:    appsv1alpha1.StoppingClusterPhase,
		OpsHandler:        StopOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.StopType, stopBehaviour)
}

// ActionStartedCondition the started condition when handling the stop request.
func (stop StopOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewStopCondition(opsRes.OpsRequest), nil
}

// Action modifies Cluster.spec.components[*].replicas from the opsRequest
func (stop StopOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	var (
		expectReplicas       = int32(0)
		componentReplicasMap = map[string]int32{}
		cluster              = opsRes.Cluster
	)
	if _, ok := cluster.Annotations[constant.SnapShotForStartAnnotationKey]; ok {
		return nil
	}
	for i, v := range cluster.Spec.ComponentSpecs {
		componentReplicasMap[v.Name] = v.Replicas
		cluster.Spec.ComponentSpecs[i].Replicas = expectReplicas
	}
	componentReplicasSnapshot, err := json.Marshal(componentReplicasMap)
	if err != nil {
		return err
	}
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	// record the replicas snapshot of components to the annotations of cluster before stopping the cluster.
	cluster.Annotations[constant.SnapShotForStartAnnotationKey] = string(componentReplicasSnapshot)
	return cli.Update(reqCtx.Ctx, cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for stop opsRequest.
func (stop StopOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	getExpectReplicas := func(opsRequest *appsv1alpha1.OpsRequest, componentName string) *int32 {
		expectReplicas := int32(0)
		return &expectReplicas
	}
	handleComponentProgress := func(reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes progressResource,
		compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
		expectProgressCount, completedCount, err := handleComponentProgressForScalingReplicas(reqCtx, cli, opsRes, pgRes, compStatus, getExpectReplicas)
		if err != nil {
			return expectProgressCount, completedCount, err
		}
		return expectProgressCount, completedCount, nil
	}
	return reconcileActionWithComponentOps(reqCtx, cli, opsRes, "stop", handleComponentProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (stop StopOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	opsRequest := opsRes.OpsRequest
	lastComponentInfo := map[string]appsv1alpha1.LastComponentConfiguration{}
	for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
		if v.Replicas != 0 {
			podNames, err := getCompPodNamesBeforeScaleDownReplicas(reqCtx, cli, *opsRes.Cluster, v.Name)
			if err != nil {
				return err
			}
			copyReplicas := v.Replicas
			lastComponentInfo[v.Name] = appsv1alpha1.LastComponentConfiguration{
				Replicas: &copyReplicas,
				TargetResources: map[appsv1alpha1.ComponentResourceKey][]string{
					appsv1alpha1.PodsCompResourceKey: podNames,
				},
			}
		}
	}
	opsRequest.Status.LastConfiguration.Components = lastComponentInfo
	return nil
}
