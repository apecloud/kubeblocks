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

package lifecycle

import (
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
)

// phaseSyncLevel defines a phase synchronization level to notify the status synchronizer how to handle cluster phase.
type phaseSyncLevel int

const (
	clusterPhaseNoChange         phaseSyncLevel = iota
	clusterIsRunning                            // cluster is running
	clusterIsStopped                            // cluster is stopped
	clusterExistFailedOrAbnormal                // cluster exists failed or abnormal component
)

type ClusterStatusTransformer struct {
	// phaseSyncLevel defines a phase synchronization level to indicate how to handle cluster phase.
	phaseSyncLevel phaseSyncLevel
	// existsAbnormalOrFailed indicates whether the cluster exists abnormal or failed component.
	existsAbnormalOrFailed bool
	// replicasNotReadyCompNames records the component names that are not ready.
	notReadyCompNames map[string]struct{}
	// replicasNotReadyCompNames records the component names which replicas are not ready.
	replicasNotReadyCompNames map[string]struct{}
}

var _ graph.Transformer = &ClusterStatusTransformer{}

func (t *ClusterStatusTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	origCluster := transCtx.OrigCluster
	cluster := transCtx.Cluster
	rootVertex, err := ictrltypes.FindRootVertex(dag)
	if err != nil {
		return err
	}

	updateObservedGeneration := func() {
		cluster.Status.ObservedGeneration = cluster.Generation
		cluster.Status.ClusterDefGeneration = transCtx.ClusterDef.Generation
	}

	initClusterStatusParams := func() {
		t.phaseSyncLevel = clusterPhaseNoChange
		t.notReadyCompNames = map[string]struct{}{}
		t.replicasNotReadyCompNames = map[string]struct{}{}
	}

	switch {
	case origCluster.IsDeleting():
		// if cluster is deleting, set root(cluster) vertex.action to DELETE
		rootVertex.Action = ictrltypes.ActionPtr(ictrltypes.DELETE)
		// TODO(refactor): move from object action, check it again
		for _, vertex := range dag.Vertices() {
			v, _ := vertex.(*ictrltypes.LifecycleVertex)
			if *v.Action == ictrltypes.CREATE {
				v.Action = ictrltypes.ActionPtr(ictrltypes.NOOP)
			} else {
				v.Action = ictrltypes.ActionPtr(ictrltypes.DELETE)
			}
		}
	case origCluster.IsUpdating():
		transCtx.Logger.Info("update cluster status after applying resources ")
		updateObservedGeneration()
		rootVertex.Action = ictrltypes.ActionStatusPtr()
	case origCluster.IsStatusUpdating():
		initClusterStatusParams()
		defer func() { rootVertex.Action = ictrltypes.ActionPtr(ictrltypes.STATUS) }()
		// reconcile the phase and conditions of the Cluster.status
		if err := t.reconcileClusterStatus(transCtx, dag, cluster); err != nil {
			return err
		}
		t.cleanupAnnotationsAfterRunning(cluster)
	}

	return nil
}

// reconcileClusterStatus reconciles phase and conditions of the Cluster.status.
func (t *ClusterStatusTransformer) reconcileClusterStatus(transCtx *ClusterTransformContext, dag *graph.DAG, cluster *appsv1alpha1.Cluster) error {
	if len(cluster.Status.Components) == 0 {
		return nil
	}
	// removes the invalid component of status.components which is deleted from spec.components.
	t.removeInvalidCompStatus(cluster)

	// do analysis of Cluster.Status.component and update the results to status synchronizer.
	t.doAnalysisAndUpdateSynchronizer(dag, cluster)

	// sync the LatestOpsRequestProcessed condition.
	t.syncOpsRequestProcessedCondition(cluster)

	// handle the ready condition.
	t.syncReadyConditionForCluster(cluster)

	// sync the cluster phase.
	switch t.phaseSyncLevel {
	case clusterIsRunning:
		if cluster.Status.Phase != appsv1alpha1.RunningClusterPhase {
			t.syncClusterPhaseToRunning(cluster)
		}
	case clusterIsStopped:
		if cluster.Status.Phase != appsv1alpha1.StoppedClusterPhase {
			t.syncClusterPhaseToStopped(cluster)
		}
	case clusterExistFailedOrAbnormal:
		t.handleExistAbnormalOrFailed(transCtx, cluster)
	}
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

// doAnalysisAndUpdateSynchronizer analyses the Cluster.Status.Components and updates the results to the synchronizer.
func (t *ClusterStatusTransformer) doAnalysisAndUpdateSynchronizer(dag *graph.DAG, cluster *appsv1alpha1.Cluster) {
	var (
		runningCompCount int
		stoppedCompCount int
	)
	// analysis the status of components and calculate the cluster phase.
	for k, v := range cluster.Status.Components {
		if v.PodsReady == nil || !*v.PodsReady {
			t.replicasNotReadyCompNames[k] = struct{}{}
			t.notReadyCompNames[k] = struct{}{}
		}
		switch v.Phase {
		case appsv1alpha1.AbnormalClusterCompPhase, appsv1alpha1.FailedClusterCompPhase:
			t.existsAbnormalOrFailed, t.notReadyCompNames[k] = true, struct{}{}
		case appsv1alpha1.RunningClusterCompPhase, appsv1alpha1.SpecReconcilingClusterCompPhase:
			// if !isComponentInHorizontalScaling(dag, k) {
			runningCompCount += 1
			// }
		case appsv1alpha1.StoppedClusterCompPhase:
			stoppedCompCount += 1
		}
	}
	if t.existsAbnormalOrFailed {
		t.phaseSyncLevel = clusterExistFailedOrAbnormal
		return
	}
	switch len(cluster.Status.Components) {
	case runningCompCount:
		t.phaseSyncLevel = clusterIsRunning
	case stoppedCompCount:
		// cluster is Stopped when cluster is not Running and all components are Stopped or Running
		t.phaseSyncLevel = clusterIsStopped
	}
}

// handleOpsRequestProcessedCondition syncs the condition that OpsRequest has been processed.
func (t *ClusterStatusTransformer) syncOpsRequestProcessedCondition(cluster *appsv1alpha1.Cluster) {
	opsCondition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1alpha1.ConditionTypeLatestOpsRequestProcessed)
	if opsCondition == nil || opsCondition.Status == metav1.ConditionTrue {
		return
	}
	opsRecords, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	if len(opsRecords) != 0 {
		return
	}
	processedCondition := newOpsRequestProcessedCondition(opsCondition.Message)
	oldCondition := meta.FindStatusCondition(cluster.Status.Conditions, processedCondition.Type)
	if !conditionIsChanged(oldCondition, processedCondition) {
		return
	}
	meta.SetStatusCondition(&cluster.Status.Conditions, processedCondition)
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

// syncClusterToStopped syncs the cluster phase to Stopped.
func (t *ClusterStatusTransformer) syncClusterPhaseToStopped(cluster *appsv1alpha1.Cluster) {
	cluster.Status.Phase = appsv1alpha1.StoppedClusterPhase
}

// handleExistAbnormalOrFailed handles the cluster status when some components are not ready.
func (t *ClusterStatusTransformer) handleExistAbnormalOrFailed(transCtx *ClusterTransformContext, cluster *appsv1alpha1.Cluster) {
	componentMap, clusterAvailabilityEffectMap, _ := getComponentRelatedInfo(cluster,
		*transCtx.ClusterDef, "")
	// handle the cluster status when some components are not ready.
	handleClusterPhaseWhenCompsNotReady(cluster, componentMap, clusterAvailabilityEffectMap)
}

// cleanupAnnotationsAfterRunning cleans up the cluster annotations after cluster is Running.
func (t *ClusterStatusTransformer) cleanupAnnotationsAfterRunning(cluster *appsv1alpha1.Cluster) {
	if !slices.Contains(appsv1alpha1.GetClusterTerminalPhases(), cluster.Status.Phase) {
		return
	}
	if _, ok := cluster.Annotations[constant.RestoreFromBackUpAnnotationKey]; !ok {
		return
	}
	delete(cluster.Annotations, constant.RestoreFromBackUpAnnotationKey)
}

// handleClusterPhaseWhenCompsNotReady handles the Cluster.status.phase when some components are Abnormal or Failed.
// REVIEW: seem duplicated handling
// Deprecated:
func handleClusterPhaseWhenCompsNotReady(cluster *appsv1alpha1.Cluster,
	componentMap map[string]string,
	clusterAvailabilityEffectMap map[string]bool) {
	var (
		clusterIsFailed   bool
		failedCompCount   int
		isVolumeExpanding bool
	)

	opsRecords, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	if len(opsRecords) != 0 && opsRecords[0].Type == appsv1alpha1.VolumeExpansionType {
		isVolumeExpanding = true
	}
	for k, v := range cluster.Status.Components {
		// determine whether other components are still doing operation, i.e., create/restart/scaling.
		// waiting for operation to complete except for volumeExpansion operation.
		// because this operation will not affect cluster availability.
		if !slices.Contains(appsv1alpha1.GetComponentTerminalPhases(), v.Phase) && !isVolumeExpanding {
			return
		}
		if v.Phase == appsv1alpha1.FailedClusterCompPhase {
			failedCompCount += 1
			componentDefName := componentMap[k]
			// if the component can affect cluster availability, set Cluster.status.phase to Failed
			if clusterAvailabilityEffectMap[componentDefName] {
				clusterIsFailed = true
				break
			}
		}
	}
	// If all components fail or there are failed components that affect the availability of the cluster, set phase to Failed
	if failedCompCount == len(cluster.Status.Components) || clusterIsFailed {
		cluster.Status.Phase = appsv1alpha1.FailedClusterPhase
	} else {
		cluster.Status.Phase = appsv1alpha1.AbnormalClusterPhase
	}
}

// getClusterAvailabilityEffect whether the component will affect the cluster availability.
// if the component can affect and be Failed, the cluster will be Failed too.
func getClusterAvailabilityEffect(componentDef *appsv1alpha1.ClusterComponentDefinition) bool {
	switch componentDef.WorkloadType {
	case appsv1alpha1.Consensus, appsv1alpha1.Replication:
		return true
	default:
		return componentDef.MaxUnavailable != nil
	}
}

// getComponentRelatedInfo gets componentMap, clusterAvailabilityMap and component definition information
func getComponentRelatedInfo(cluster *appsv1alpha1.Cluster, clusterDef appsv1alpha1.ClusterDefinition, componentName string) (map[string]string, map[string]bool, appsv1alpha1.ClusterComponentDefinition) {
	var (
		compDefName  string
		componentMap = map[string]string{}
		componentDef appsv1alpha1.ClusterComponentDefinition
	)
	for _, v := range cluster.Spec.ComponentSpecs {
		if v.Name == componentName {
			compDefName = v.ComponentDefRef
		}
		componentMap[v.Name] = v.ComponentDefRef
	}
	clusterAvailabilityEffectMap := map[string]bool{}
	for _, v := range clusterDef.Spec.ComponentDefs {
		clusterAvailabilityEffectMap[v.Name] = getClusterAvailabilityEffect(&v)
		if v.Name == compDefName {
			componentDef = v
		}
	}
	return componentMap, clusterAvailabilityEffectMap, componentDef
}
