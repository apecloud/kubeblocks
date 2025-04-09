/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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
		cluster  = opsRes.Cluster
		stopList = opsRes.OpsRequest.Spec.StopList
	)

	// if the cluster is already stopping or stopped, return
	if slices.Contains([]appsv1.ClusterPhase{appsv1.StoppedClusterPhase,
		appsv1.StoppingClusterPhase}, opsRes.Cluster.Status.Phase) {
		return nil
	}
	compOpsHelper := newComponentOpsHelper(stopList)
	// abort earlier running opsRequests.
	if err := abortEarlierOpsRequestWithSameKind(reqCtx, cli, opsRes, []opsv1alpha1.OpsType{opsv1alpha1.HorizontalScalingType,
		opsv1alpha1.StartType, opsv1alpha1.RestartType, opsv1alpha1.VerticalScalingType},
		func(earlierOps *opsv1alpha1.OpsRequest) (bool, error) {
			if len(stopList) == 0 {
				// stop all components
				return true, nil
			}
			switch earlierOps.Spec.Type {
			case opsv1alpha1.RestartType:
				return hasIntersectionCompOpsList(compOpsHelper.componentOpsSet, earlierOps.Spec.RestartList), nil
			case opsv1alpha1.VerticalScalingType:
				return hasIntersectionCompOpsList(compOpsHelper.componentOpsSet, earlierOps.Spec.VerticalScalingList), nil
			case opsv1alpha1.HorizontalScalingType:
				return hasIntersectionCompOpsList(compOpsHelper.componentOpsSet, earlierOps.Spec.HorizontalScalingList), nil
			case opsv1alpha1.StartType:
				return len(earlierOps.Spec.StartList) == 0 || hasIntersectionCompOpsList(compOpsHelper.componentOpsSet, earlierOps.Spec.StartList), nil
			}
			return false, nil
		}); err != nil {
		return err
	}

	stopComp := func(compSpec *appsv1.ClusterComponentSpec, clusterCompName string) {
		if len(stopList) > 0 {
			if _, ok := compOpsHelper.componentOpsSet[clusterCompName]; !ok {
				return
			}
		}
		compSpec.Stop = pointer.Bool(true)
	}

	for i, v := range cluster.Spec.ComponentSpecs {
		stopComp(&cluster.Spec.ComponentSpecs[i], v.Name)
	}
	for i, v := range cluster.Spec.Shardings {
		stopComp(&cluster.Spec.Shardings[i].Template, v.Name)
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
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.StopList)

	phase, duration, err := compOpsHelper.reconcileActionWithComponentOps(reqCtx, cli, opsRes, "stop", handleComponentProgress)

	// 新增逻辑：当集群进入停止状态时暂停相关备份
	if opsRes.Cluster.Status.Phase == appsv1.StoppingClusterPhase || opsRes.Cluster.Status.Phase == appsv1.StoppedClusterPhase {
		if err := pauseRelatedBackups(reqCtx, cli, opsRes.Cluster); err != nil {
			return opsv1alpha1.OpsFailedPhase, 0, err
		}
	}

	return phase, duration, err
	//return compOpsHelper.reconcileActionWithComponentOps(reqCtx, cli, opsRes, "stop", handleComponentProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (stop StopOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

// pauseRelatedBackups 暂停与集群关联的所有运行中的备份
func pauseRelatedBackups(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1.Cluster) error {
	// 1. 通过标签获取关联的所有Backup资源
	backupList := &dpv1alpha1.BackupList{}
	labels := client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.Name, // 假设Backup使用该标签关联Cluster
	}
	if err := cli.List(reqCtx.Ctx, backupList, client.InNamespace(cluster.Namespace), labels); err != nil {
		return err
	}

	// 2. 过滤出需要暂停的备份
	var needUpdateBackups []*dpv1alpha1.Backup
	for i := range backupList.Items {
		backup := &backupList.Items[i]
		if backup.Status.Phase == dpv1alpha1.BackupPhaseRunning {
			needUpdateBackups = append(needUpdateBackups, backup)
		}
	}

	// 3. 批量更新备份状态为Paused
	for _, backup := range needUpdateBackups {
		patch := client.MergeFrom(backup.DeepCopy())
		backup.Status.Phase = dpv1alpha1.BackupPhasePaused
		backup.Status.CompletionTimestamp = &metav1.Time{Time: time.Now()}
		if backup.Status.StartTimestamp != nil {
			duration := backup.Status.CompletionTimestamp.Sub(backup.Status.StartTimestamp.Time).Round(time.Second)
			backup.Status.Duration = &metav1.Duration{Duration: duration}
		}
		if err := cli.Status().Patch(reqCtx.Ctx, backup, patch); err != nil {
			return err
		}
		reqCtx.Log.Info("paused backup due to cluster stopping",
			"backup", client.ObjectKeyFromObject(backup),
			"cluster", cluster.Name)
	}
	return nil
}
