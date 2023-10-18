/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

	"k8s.io/apimachinery/pkg/api/meta"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type ClusterStatusTransformer struct {
	// replicasNotReadyCompNames records the component names that are not ready.
	notReadyCompNames map[string]struct{}
	// replicasNotReadyCompNames records the component names which replicas are not ready.
	replicasNotReadyCompNames map[string]struct{}
}

var _ graph.Transformer = &ClusterStatusTransformer{}

func (t *ClusterStatusTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	origCluster := transCtx.OrigCluster
	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)

	updateObservedGeneration := func() {
		cluster.Status.ObservedGeneration = cluster.Generation
		cluster.Status.ClusterDefGeneration = transCtx.ClusterDef.Generation
	}

	switch {
	case origCluster.IsUpdating():
		transCtx.Logger.Info(fmt.Sprintf("update cluster status after applying resources, generation: %d", cluster.Generation))
		updateObservedGeneration()
		graphCli.Status(dag, origCluster, cluster)
	case origCluster.IsStatusUpdating():
		defer func() { graphCli.Status(dag, origCluster, cluster) }()
		// reconcile the phase and conditions of the Cluster.status
		if err := t.reconcileClusterStatus(cluster); err != nil {
			return err
		}
	case origCluster.IsDeleting():
		return fmt.Errorf("unexpected cluster status: %+v", origCluster)
	default:
		panic(fmt.Sprintf("runtime error - unknown cluster status: %+v", origCluster))
	}

	return nil
}

func (t *ClusterStatusTransformer) reconcileClusterPhase(cluster *appsv1alpha1.Cluster) {
	var (
		isAllComponentCreating = true
		isAllComponentRunning  = true
		isAllComponentWorking  = true
		hasComponentStopping   = false
		isAllComponentStopped  = true
		isAllComponentFailed   = true
	)
	isPhaseIn := func(phase appsv1alpha1.ClusterComponentPhase, phases ...appsv1alpha1.ClusterComponentPhase) bool {
		for _, p := range phases {
			if p == phase {
				return true
			}
		}
		return false
	}
	for _, status := range cluster.Status.Components {
		phase := status.Phase
		if !isPhaseIn(phase, appsv1alpha1.CreatingClusterCompPhase) {
			isAllComponentCreating = false
		}
		if !isPhaseIn(phase, appsv1alpha1.RunningClusterCompPhase) {
			isAllComponentRunning = false
		}
		if !isPhaseIn(phase, appsv1alpha1.CreatingClusterCompPhase,
			appsv1alpha1.RunningClusterCompPhase,
			appsv1alpha1.UpdatingClusterCompPhase) {
			isAllComponentWorking = false
		}
		if isPhaseIn(phase, appsv1alpha1.StoppingClusterCompPhase) {
			hasComponentStopping = true
		}
		if !isPhaseIn(phase, appsv1alpha1.StoppedClusterCompPhase) {
			isAllComponentStopped = false
		}
		if !isPhaseIn(phase, appsv1alpha1.FailedClusterCompPhase) {
			isAllComponentFailed = false
		}
	}

	switch {
	case isAllComponentRunning:
		if cluster.Status.Phase != appsv1alpha1.RunningClusterPhase {
			t.syncClusterPhaseToRunning(cluster)
		}
	case isAllComponentCreating:
		cluster.Status.Phase = appsv1alpha1.CreatingClusterPhase
	case isAllComponentWorking:
		cluster.Status.Phase = appsv1alpha1.UpdatingClusterPhase
	case isAllComponentStopped:
		if cluster.Status.Phase != appsv1alpha1.StoppedClusterPhase {
			t.syncClusterPhaseToStopped(cluster)
		}
	case hasComponentStopping:
		cluster.Status.Phase = appsv1alpha1.StoppingClusterPhase
	case isAllComponentFailed:
		cluster.Status.Phase = appsv1alpha1.FailedClusterPhase
	default:
		cluster.Status.Phase = appsv1alpha1.AbnormalClusterPhase
	}
}

// reconcileClusterStatus reconciles phase and conditions of the Cluster.status.
func (t *ClusterStatusTransformer) reconcileClusterStatus(cluster *appsv1alpha1.Cluster) error {
	if len(cluster.Status.Components) == 0 {
		return nil
	}
	initClusterStatusParams := func() {
		t.notReadyCompNames = map[string]struct{}{}
		t.replicasNotReadyCompNames = map[string]struct{}{}
	}
	initClusterStatusParams()

	// removes the invalid component of status.components which is deleted from spec.components.
	t.removeInvalidCompStatus(cluster)

	// do analysis of Cluster.Status.component and update the results to status synchronizer.
	t.doAnalysisAndUpdateSynchronizer(cluster)

	// handle the ready condition.
	t.syncReadyConditionForCluster(cluster)

	// sync the cluster phase.
	t.reconcileClusterPhase(cluster)
	return nil
}

// removeInvalidCompStatus removes the invalid component of status.components which is deleted from spec.components.
func (t *ClusterStatusTransformer) removeInvalidCompStatus(cluster *appsv1alpha1.Cluster) {
	// remove the invalid component in status.components when the component is deleted from spec.components.
	tmpCompsStatus := map[string]appsv1alpha1.ClusterComponentStatus{}
	compsStatus := cluster.Status.Components
	for _, v := range cluster.Spec.ComponentSpecs {
		if compStatus, ok := compsStatus[v.Name]; ok {
			tmpCompsStatus[v.Name] = compStatus
		}
	}
	// keep valid components' status
	cluster.Status.Components = tmpCompsStatus
}

// doAnalysisAndUpdateSynchronizer analyzes the Cluster.Status.Components and updates the results to the synchronizer.
func (t *ClusterStatusTransformer) doAnalysisAndUpdateSynchronizer(cluster *appsv1alpha1.Cluster) {
	// analysis the status of components and calculate the cluster phase.
	for k, v := range cluster.Status.Components {
		if v.PodsReady == nil || !*v.PodsReady {
			t.replicasNotReadyCompNames[k] = struct{}{}
			t.notReadyCompNames[k] = struct{}{}
		}
		switch v.Phase {
		case appsv1alpha1.AbnormalClusterCompPhase, appsv1alpha1.FailedClusterCompPhase:
			t.notReadyCompNames[k] = struct{}{}
		}
	}
}

// syncReadyConditionForCluster syncs the cluster conditions with ClusterReady and ReplicasReady type.
func (t *ClusterStatusTransformer) syncReadyConditionForCluster(cluster *appsv1alpha1.Cluster) {
	if len(t.replicasNotReadyCompNames) == 0 {
		// if all replicas of cluster are ready, set ReasonAllReplicasReady to status.conditions
		readyCondition := newAllReplicasPodsReadyConditions()
		meta.SetStatusCondition(&cluster.Status.Conditions, readyCondition)
	} else {
		meta.SetStatusCondition(&cluster.Status.Conditions, newReplicasNotReadyCondition(t.replicasNotReadyCompNames))
	}

	if len(t.notReadyCompNames) > 0 {
		meta.SetStatusCondition(&cluster.Status.Conditions, newComponentsNotReadyCondition(t.notReadyCompNames))
	}
}

// syncClusterPhaseToRunning syncs the cluster phase to Running.
func (t *ClusterStatusTransformer) syncClusterPhaseToRunning(cluster *appsv1alpha1.Cluster) {
	cluster.Status.Phase = appsv1alpha1.RunningClusterPhase
	meta.SetStatusCondition(&cluster.Status.Conditions, newClusterReadyCondition(cluster.Name))
}

// syncClusterPhaseToStopped syncs the cluster phase to Stopped.
func (t *ClusterStatusTransformer) syncClusterPhaseToStopped(cluster *appsv1alpha1.Cluster) {
	cluster.Status.Phase = appsv1alpha1.StoppedClusterPhase
}
