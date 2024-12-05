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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type restartOpsHandler struct {
	compOpsHelper componentOpsHelper
}

var _ OpsHandler = restartOpsHandler{}

func init() {
	restartBehaviour := OpsBehaviour{
		// if cluster is Abnormal or Failed, new opsRequest may repair it.
		FromClusterPhases: appsv1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1.UpdatingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        restartOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.RestartType, restartBehaviour)
}

// ActionStartedCondition the started condition when handle the restart request.
func (r restartOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewRestartingCondition(opsRes.OpsRequest), nil
}

// Action restarts components by updating StatefulSet.
func (r restartOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	if opsRes.OpsRequest.Status.StartTimestamp.IsZero() {
		return fmt.Errorf("status.startTimestamp can not be null")
	}
	r.compOpsHelper = newComponentOpsHelper(opsRes.OpsRequest.Spec.RestartList)
	// abort earlier running 'Restart' opsRequest.
	if err := abortEarlierOpsRequestWithSameKind(reqCtx, cli, opsRes, []opsv1alpha1.OpsType{opsv1alpha1.RestartType},
		func(earlierOps *opsv1alpha1.OpsRequest) (bool, error) {
			return hasIntersectionCompOpsList(r.compOpsHelper.componentOpsSet, earlierOps.Spec.RestartList), nil
		}); err != nil {
		return err
	}
	return r.restartComponents(reqCtx, cli, opsRes, opsRes.OpsRequest.Spec.RestartList)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for restart opsRequest.
func (r restartOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	r.compOpsHelper = newComponentOpsHelper(opsRes.OpsRequest.Spec.RestartList)
	handleRestartProgress := func(reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes *progressResource,
		compStatus *opsv1alpha1.OpsRequestComponentStatus) (expectProgressCount int32, completedCount int32, err error) {
		return handleComponentStatusProgress(reqCtx, cli, opsRes, pgRes, compStatus, r.podApplyCompOps)
	}
	return r.compOpsHelper.reconcileActionWithComponentOps(reqCtx, cli, opsRes,
		"restart", handleRestartProgress)
}

// SaveLastConfiguration this operation only restart the pods of the component, no changes for Cluster.spec.
// empty implementation here.
func (r restartOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func (r restartOpsHandler) podApplyCompOps(
	ops *opsv1alpha1.OpsRequest,
	pod *corev1.Pod,
	pgRes *progressResource) bool {
	return !pod.CreationTimestamp.Before(&ops.Status.StartTimestamp)
}

func (r restartOpsHandler) restartComponents(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource, comOpsList []opsv1alpha1.ComponentOps) error {
	doRestart := func(compSpec *appsv1.ClusterComponentSpec, currCompName, targetCompName string) (bool, error) {
		if targetCompName != currCompName {
			return false, nil
		}
		if r.isRestarted(opsRes, compSpec) {
			return false, nil
		}
		if err := cli.Update(reqCtx.Ctx, opsRes.Cluster); err != nil {
			return false, err
		}
		return true, nil
	}

	restartComponent := func(targetCompName string) error {
		for i := range opsRes.Cluster.Spec.ComponentSpecs {
			componentSpec := &opsRes.Cluster.Spec.ComponentSpecs[i]
			if ok, err := doRestart(componentSpec, componentSpec.Name, targetCompName); ok || err != nil {
				return err
			}
		}
		for i := range opsRes.Cluster.Spec.Shardings {
			shardingSpec := &opsRes.Cluster.Spec.Shardings[i]
			if ok, err := doRestart(&shardingSpec.Template, shardingSpec.Name, targetCompName); ok || err != nil {
				return err
			}
		}
		return nil
	}

	for _, compOps := range comOpsList {
		if err := restartComponent(compOps.GetComponentName()); err != nil {
			return err
		}

	}
	return nil
}

// isRestarted checks whether the component has been restarted
func (r restartOpsHandler) isRestarted(opsRes *OpsResource, compSpec *appsv1.ClusterComponentSpec) bool {
	if compSpec.Annotations == nil {
		compSpec.Annotations = map[string]string{}
	}
	hasRestarted := true
	startTimestamp := opsRes.OpsRequest.Status.StartTimestamp
	workloadRestartTimeStamp := compSpec.Annotations[constant.RestartAnnotationKey]
	if res, _ := time.Parse(time.RFC3339, workloadRestartTimeStamp); startTimestamp.After(res) {
		compSpec.Annotations[constant.RestartAnnotationKey] = startTimestamp.Format(time.RFC3339)
		hasRestarted = false
	}
	return hasRestarted
}
