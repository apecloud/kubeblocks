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

package apps

import (
	"fmt"

	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/api/meta"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type clusterStatusTransformer struct{}

var _ graph.Transformer = &clusterStatusTransformer{}

func (t *clusterStatusTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	origCluster := transCtx.OrigCluster
	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)

	switch {
	case origCluster.IsUpdating():
		transCtx.Logger.Info(fmt.Sprintf("update cluster status after applying resources, generation: %d", cluster.Generation))
		cluster.Status.ObservedGeneration = cluster.Generation
		t.markClusterDagStatusAction(graphCli, dag, origCluster, cluster)
	case origCluster.IsStatusUpdating():
		defer func() { t.markClusterDagStatusAction(graphCli, dag, origCluster, cluster) }()
		// reconcile the phase and conditions of the cluster.status
		if err := t.reconcileClusterStatus(transCtx, cluster); err != nil {
			return err
		}
	case origCluster.IsDeleting():
		return fmt.Errorf("unexpected cluster status: %s", origCluster.Status.Phase)
	default:
		panic(fmt.Sprintf("runtime error - unknown cluster status: %+v", origCluster))
	}

	return nil
}

func (t *clusterStatusTransformer) markClusterDagStatusAction(graphCli model.GraphClient, dag *graph.DAG, origCluster, cluster *appsv1.Cluster) {
	if v := graphCli.FindMatchedVertex(dag, cluster); v == nil {
		graphCli.Status(dag, origCluster, cluster)
	}
}

func (t *clusterStatusTransformer) reconcileClusterStatus(transCtx *clusterTransformContext, cluster *appsv1.Cluster) error {
	if len(cluster.Status.Components) == 0 && len(cluster.Status.Shardings) == 0 {
		return nil
	}

	// t.removeDeletedCompNSharding(transCtx, cluster)

	oldPhase := t.reconcileClusterPhase(cluster)

	t.syncClusterConditions(cluster, oldPhase)

	return nil
}

func (t *clusterStatusTransformer) reconcileClusterPhase(cluster *appsv1.Cluster) appsv1.ClusterPhase {
	statusList := make([]appsv1.ClusterComponentStatus, 0)
	if cluster.Status.Components != nil {
		statusList = append(statusList, maps.Values(cluster.Status.Components)...)
	}
	if cluster.Status.Shardings != nil {
		statusList = append(statusList, maps.Values(cluster.Status.Shardings)...)
	}
	newPhase := composeClusterPhase(statusList)

	phase := cluster.Status.Phase
	if newPhase != "" {
		cluster.Status.Phase = newPhase
	}
	return phase
}

func (t *clusterStatusTransformer) syncClusterConditions(cluster *appsv1.Cluster, oldPhase appsv1.ClusterPhase) {
	if cluster.Status.Phase == appsv1.RunningClusterPhase && oldPhase != cluster.Status.Phase {
		meta.SetStatusCondition(&cluster.Status.Conditions, newClusterReadyCondition(cluster.Name))
		return
	}

	kindNames := map[string][]string{}
	for kind, statusMap := range map[string]map[string]appsv1.ClusterComponentStatus{
		"component": cluster.Status.Components,
		"sharding":  cluster.Status.Shardings,
	} {
		for name, status := range statusMap {
			if status.Phase == appsv1.FailedComponentPhase {
				if _, ok := kindNames[kind]; !ok {
					kindNames[kind] = []string{}
				}
				kindNames[kind] = append(kindNames[kind], name)
			}
		}
	}
	if len(kindNames) > 0 {
		meta.SetStatusCondition(&cluster.Status.Conditions, newClusterNotReadyCondition(cluster.Name, kindNames))
	}
}

func composeClusterPhase(statusList []appsv1.ClusterComponentStatus) appsv1.ClusterPhase {
	var (
		isAllComponentCreating         = true
		isAllComponentWorking          = true
		hasComponentStopping           = false
		isAllComponentStopped          = true
		isAllComponentFailed           = true
		hasComponentFailed             = false
		isAllComponentRunningOrStopped = true
	)
	isPhaseIn := func(phase appsv1.ComponentPhase, phases ...appsv1.ComponentPhase) bool {
		for _, p := range phases {
			if p == phase {
				return true
			}
		}
		return false
	}
	for _, status := range statusList {
		phase := status.Phase
		if !isPhaseIn(phase, appsv1.CreatingComponentPhase) {
			isAllComponentCreating = false
		}
		if !isPhaseIn(phase, appsv1.RunningComponentPhase, appsv1.StoppedComponentPhase) {
			isAllComponentRunningOrStopped = false
		}
		if !isPhaseIn(phase, appsv1.CreatingComponentPhase, appsv1.RunningComponentPhase, appsv1.UpdatingComponentPhase) {
			isAllComponentWorking = false
		}
		if isPhaseIn(phase, appsv1.StoppingComponentPhase) {
			hasComponentStopping = true
		}
		if !isPhaseIn(phase, appsv1.StoppedComponentPhase) {
			isAllComponentStopped = false
		}
		if !isPhaseIn(phase, appsv1.FailedComponentPhase) {
			isAllComponentFailed = false
		}
		if isPhaseIn(phase, appsv1.FailedComponentPhase) {
			hasComponentFailed = true
		}

	}

	switch {
	case isAllComponentStopped:
		return appsv1.StoppedClusterPhase
	case isAllComponentRunningOrStopped:
		return appsv1.RunningClusterPhase
	case isAllComponentCreating:
		return appsv1.CreatingClusterPhase
	case isAllComponentWorking:
		return appsv1.UpdatingClusterPhase
	case hasComponentStopping:
		return appsv1.StoppingClusterPhase
	case isAllComponentFailed:
		return appsv1.FailedClusterPhase
	case hasComponentFailed:
		return appsv1.AbnormalClusterPhase
	default:
		return ""
	}
}
