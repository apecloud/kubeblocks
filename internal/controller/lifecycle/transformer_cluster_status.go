/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lifecycle

import (
	"fmt"
	"reflect"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// phaseSyncLevel defines a phase synchronization level to notify the status synchronizer how to handle cluster phase.
type phaseSyncLevel int

const (
	clusterPhaseNoChange         phaseSyncLevel = iota
	clusterIsRunning                            // cluster is running
	clusterIsStopped                            // cluster is stopped
	clusterExistFailedOrAbnormal                // cluster exists failed or abnormal component
)

type clusterStatusTransformer struct {
	cc       clusterRefResources
	cli      client.Client
	ctx      intctrlutil.RequestCtx
	recorder record.EventRecorder
	// phaseSyncLevel defines a phase synchronization level to indicate how to handle cluster phase.
	phaseSyncLevel phaseSyncLevel
	// existsAbnormalOrFailed indicates whether the cluster exists abnormal or failed component.
	existsAbnormalOrFailed bool
	// replicasNotReadyCompNames records the component names that are not ready.
	notReadyCompNames map[string]struct{}
	// replicasNotReadyCompNames records the component names which replicas are not ready.
	replicasNotReadyCompNames map[string]struct{}
}

func newClusterStatusTransformer(ctx intctrlutil.RequestCtx,
	cli client.Client,
	recorder record.EventRecorder,
	cc clusterRefResources) *clusterStatusTransformer {
	return &clusterStatusTransformer{
		ctx:                       ctx,
		cc:                        cc,
		cli:                       cli,
		recorder:                  recorder,
		phaseSyncLevel:            clusterPhaseNoChange,
		notReadyCompNames:         map[string]struct{}{},
		replicasNotReadyCompNames: map[string]struct{}{},
	}
}
func (c *clusterStatusTransformer) Transform(dag *graph.DAG) error {
	rootVertex, err := findRootVertex(dag)
	if err != nil {
		return err
	}
	origCluster, _ := rootVertex.oriObj.(*appsv1alpha1.Cluster)
	cluster, _ := rootVertex.obj.(*appsv1alpha1.Cluster)

	updateComponentPhase := func() {
		vertices := findAllNot[*appsv1alpha1.Cluster](dag)
		for _, vertex := range vertices {
			v, _ := vertex.(*lifecycleVertex)
			if v.immutable || v.action == nil || *v.action != CREATE {
				continue
			}
			switch v.obj.(type) {
			case *appsv1.StatefulSet, *appsv1.Deployment:
				updateComponentPhaseWithOperation(cluster, v.obj.GetLabels()[constant.KBAppComponentLabelKey])
			}
		}
	}

	switch {
	case isClusterDeleting(*origCluster):
		// if cluster is deleting, set root(cluster) vertex.action to DELETE
		rootVertex.action = actionPtr(DELETE)
		// TODO(refactor): move from object action, check it again
		for _, vertex := range dag.Vertices() {
			v, _ := vertex.(*lifecycleVertex)
			v.action = actionPtr(DELETE)
		}
		// TODO(refactor): delete orphan resources which are not in dag?
	case isClusterUpdating(*origCluster):
		c.ctx.Log.Info("update cluster status after applying resources ")
		defer func() {
			// update components' phase in cluster.status
			updateComponentPhase()
			rootVertex.action = actionPtr(STATUS)
			rootVertex.immutable = reflect.DeepEqual(cluster.Status, origCluster.Status)
		}()
		cluster.Status.ObservedGeneration = cluster.Generation
		cluster.Status.ClusterDefGeneration = c.cc.cd.Generation
		applyResourcesCondition := newApplyResourcesCondition(cluster.Generation)
		oldApplyCondition := meta.FindStatusCondition(cluster.Status.Conditions, applyResourcesCondition.Type)
		if !conditionIsChanged(oldApplyCondition, applyResourcesCondition) {
			return nil
		}
		meta.SetStatusCondition(&cluster.Status.Conditions, applyResourcesCondition)
		rootVertex.postHandleAfterStatusPatch = append(rootVertex.postHandleAfterStatusPatch, func() error {
			c.recorder.Event(cluster, corev1.EventTypeNormal, applyResourcesCondition.Reason, applyResourcesCondition.Message)
			return nil
		})
	case isClusterStatusUpdating(*origCluster):
		defer func() {
			rootVertex.action = actionPtr(STATUS)
			rootVertex.immutable = reflect.DeepEqual(cluster.Status, origCluster.Status)
			// TODO(refactor): move from object action, check it again
			vertices := findAllNot[*appsv1alpha1.Cluster](dag)
			for _, vertex := range vertices {
				v, _ := vertex.(*lifecycleVertex)
				// TODO(refactor): fix me, workaround for h-scaling to update stateful set
				if _, ok := v.obj.(*appsv1.StatefulSet); !ok {
					v.immutable = true
				}
			}
		}()
		// checks if the controller is handling the garbage of restore.
		if err := c.handleGarbageOfRestoreBeforeRunning(cluster); err != nil {
			return err
		}
		// reconcile the phase and conditions of the Cluster.status
		if err := c.reconcileClusterStatus(cluster, rootVertex); err != nil {
			return err
		}
		c.cleanupAnnotationsAfterRunning(cluster)
	}

	return nil
}

// reconcileClusterStatus reconciles phase and conditions of the Cluster.status.
func (c *clusterStatusTransformer) reconcileClusterStatus(cluster *appsv1alpha1.Cluster, rootVertex *lifecycleVertex) error {
	if len(cluster.Status.Components) == 0 {
		return nil
	}
	// removes the invalid component of status.components which is deleted from spec.components.
	c.removeInvalidCompStatus(cluster)

	// do analysis of Cluster.Status.component and update the results to status synchronizer.
	c.doAnalysisAndUpdateSynchronizer(cluster)

	// sync the LatestOpsRequestProcessed condition.
	c.syncOpsRequestProcessedCondition(cluster, rootVertex)

	// handle the ready condition.
	c.syncReadyConditionForCluster(cluster, rootVertex)

	// sync the cluster phase.
	switch c.phaseSyncLevel {
	case clusterIsRunning:
		if cluster.Status.Phase != appsv1alpha1.RunningClusterPhase {
			c.syncClusterPhaseToRunning(cluster, rootVertex)
		}
	case clusterIsStopped:
		if cluster.Status.Phase != appsv1alpha1.StoppedClusterPhase {
			c.syncClusterPhaseToStopped(cluster, rootVertex)
		}
	case clusterExistFailedOrAbnormal:
		c.handleExistAbnormalOrFailed(cluster, rootVertex)
	}
	return nil
}

// removeInvalidCompStatus removes the invalid component of status.components which is deleted from spec.components.
func (c *clusterStatusTransformer) removeInvalidCompStatus(cluster *appsv1alpha1.Cluster) {
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
func (c *clusterStatusTransformer) doAnalysisAndUpdateSynchronizer(cluster *appsv1alpha1.Cluster) {
	var (
		runningCompCount int
		stoppedCompCount int
	)
	// analysis the status of components and calculate the cluster phase.
	for k, v := range cluster.Status.Components {
		if v.PodsReady == nil || !*v.PodsReady {
			c.replicasNotReadyCompNames[k] = struct{}{}
			c.notReadyCompNames[k] = struct{}{}
		}
		switch v.Phase {
		case appsv1alpha1.AbnormalClusterCompPhase, appsv1alpha1.FailedClusterCompPhase:
			c.existsAbnormalOrFailed, c.notReadyCompNames[k] = true, struct{}{}
		case appsv1alpha1.RunningClusterCompPhase:
			runningCompCount += 1
		case appsv1alpha1.StoppedClusterCompPhase:
			stoppedCompCount += 1
		}
	}
	if c.existsAbnormalOrFailed {
		c.phaseSyncLevel = clusterExistFailedOrAbnormal
		return
	}
	switch len(cluster.Status.Components) {
	case runningCompCount:
		c.phaseSyncLevel = clusterIsRunning
	case stoppedCompCount:
		// cluster is Stopped when cluster is not Running and all components are Stopped or Running
		c.phaseSyncLevel = clusterIsStopped
	}
}

// handleOpsRequestProcessedCondition syncs the condition that OpsRequest has been processed.
func (c *clusterStatusTransformer) syncOpsRequestProcessedCondition(cluster *appsv1alpha1.Cluster, rootVertex *lifecycleVertex) {
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
	rootVertex.postHandleAfterStatusPatch = append(rootVertex.postHandleAfterStatusPatch, func() error {
		// send an event when all pods of the components are ready.
		c.recorder.Event(cluster, corev1.EventTypeNormal, processedCondition.Reason, processedCondition.Message)
		return nil
	})
}

// syncReadyConditionForCluster syncs the cluster conditions with ClusterReady and ReplicasReady type.
func (c *clusterStatusTransformer) syncReadyConditionForCluster(cluster *appsv1alpha1.Cluster, rootVertex *lifecycleVertex) {
	if len(c.replicasNotReadyCompNames) == 0 {
		oldReplicasReadyCondition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1alpha1.ConditionTypeReplicasReady)
		// if all replicas of cluster are ready, set ReasonAllReplicasReady to status.conditions
		readyCondition := newAllReplicasPodsReadyConditions()
		if oldReplicasReadyCondition == nil || oldReplicasReadyCondition.Status == metav1.ConditionFalse {
			rootVertex.postHandleAfterStatusPatch = append(rootVertex.postHandleAfterStatusPatch, func() error {
				// send an event when all pods of the components are ready.
				c.recorder.Event(cluster, corev1.EventTypeNormal, readyCondition.Reason, readyCondition.Message)
				return nil
			})
		}
		meta.SetStatusCondition(&cluster.Status.Conditions, readyCondition)
	} else {
		meta.SetStatusCondition(&cluster.Status.Conditions, newReplicasNotReadyCondition(c.replicasNotReadyCompNames))
	}

	if len(c.notReadyCompNames) > 0 {
		meta.SetStatusCondition(&cluster.Status.Conditions, newComponentsNotReadyCondition(c.notReadyCompNames))
	}
}

// syncClusterPhaseToRunning syncs the cluster phase to Running.
func (c *clusterStatusTransformer) syncClusterPhaseToRunning(cluster *appsv1alpha1.Cluster, rootVertex *lifecycleVertex) {
	cluster.Status.Phase = appsv1alpha1.RunningClusterPhase
	meta.SetStatusCondition(&cluster.Status.Conditions, newClusterReadyCondition(cluster.Name))
	rootVertex.postHandleAfterStatusPatch = append(rootVertex.postHandleAfterStatusPatch, func() error {
		message := fmt.Sprintf("Cluster: %s is ready, current phase is Running", cluster.Name)
		c.recorder.Event(cluster, corev1.EventTypeNormal, string(appsv1alpha1.RunningClusterPhase), message)
		return opsutil.MarkRunningOpsRequestAnnotation(c.ctx.Ctx, c.cli, cluster)
	})
}

// syncClusterToStopped syncs the cluster phase to Stopped.
func (c *clusterStatusTransformer) syncClusterPhaseToStopped(cluster *appsv1alpha1.Cluster, rootVertex *lifecycleVertex) {
	cluster.Status.Phase = appsv1alpha1.StoppedClusterPhase
	rootVertex.postHandleAfterStatusPatch = append(rootVertex.postHandleAfterStatusPatch, func() error {
		message := fmt.Sprintf("Cluster: %s stopped successfully.", cluster.Name)
		c.recorder.Event(cluster, corev1.EventTypeNormal, string(cluster.Status.Phase), message)
		return opsutil.MarkRunningOpsRequestAnnotation(c.ctx.Ctx, c.cli, cluster)
	})
}

// handleExistAbnormalOrFailed handles the cluster status when some components are not ready.
func (c *clusterStatusTransformer) handleExistAbnormalOrFailed(cluster *appsv1alpha1.Cluster, rootVertex *lifecycleVertex) {
	oldPhase := cluster.Status.Phase
	componentMap, clusterAvailabilityEffectMap, _ := getComponentRelatedInfo(cluster,
		c.cc.cd, "")
	// handle the cluster status when some components are not ready.
	handleClusterPhaseWhenCompsNotReady(cluster, componentMap, clusterAvailabilityEffectMap)
	currPhase := cluster.Status.Phase
	if slices.Contains(appsv1alpha1.GetClusterFailedPhases(), currPhase) && oldPhase != currPhase {
		rootVertex.postHandleAfterStatusPatch = append(rootVertex.postHandleAfterStatusPatch, func() error {
			message := fmt.Sprintf("Cluster: %s is %s, check according to the components message",
				cluster.Name, currPhase)
			c.recorder.Event(cluster, corev1.EventTypeWarning, string(cluster.Status.Phase), message)
			return opsutil.MarkRunningOpsRequestAnnotation(c.ctx.Ctx, c.cli, cluster)
		})
	}
}

// cleanupAnnotationsAfterRunning cleans up the cluster annotations after cluster is Running.
func (c *clusterStatusTransformer) cleanupAnnotationsAfterRunning(cluster *appsv1alpha1.Cluster) {
	if !slices.Contains(appsv1alpha1.GetClusterTerminalPhases(), cluster.Status.Phase) {
		return
	}
	if _, ok := cluster.Annotations[constant.RestoreFromBackUpAnnotationKey]; !ok {
		return
	}
	delete(cluster.Annotations, constant.RestoreFromBackUpAnnotationKey)
}

// REVIEW: this handling is rather hackish, call for refactor.
// handleRestoreGarbageBeforeRunning handles the garbage for restore before cluster phase changes to Running.
// @return ErrNoOps if no operation
// Deprecated: to be removed by PITR feature.
func (c *clusterStatusTransformer) handleGarbageOfRestoreBeforeRunning(cluster *appsv1alpha1.Cluster) error {
	clusterBackupResourceMap, err := getClusterBackupSourceMap(cluster)
	if err != nil {
		return err
	}
	if clusterBackupResourceMap == nil {
		return nil
	}
	// check if all components are running.
	for _, v := range cluster.Status.Components {
		if v.Phase != appsv1alpha1.RunningClusterCompPhase {
			return nil
		}
	}
	// remove the garbage for restore if the cluster restores from backup.
	return c.removeGarbageWithRestore(cluster, clusterBackupResourceMap)
}

// REVIEW: this handling is rather hackish, call for refactor.
// removeGarbageWithRestore removes the garbage for restore when all components are Running.
// @return ErrNoOps if no operation
// Deprecated:
func (c *clusterStatusTransformer) removeGarbageWithRestore(
	cluster *appsv1alpha1.Cluster,
	clusterBackupResourceMap map[string]string) error {
	var (
		err error
	)
	for k, v := range clusterBackupResourceMap {
		// remove the init container for restore
		if _, err = c.removeStsInitContainerForRestore(cluster, k, v); err != nil {
			return err
		}
	}
	return nil
}

// removeStsInitContainerForRestore removes the statefulSet's init container which restores data from backup.
func (c *clusterStatusTransformer) removeStsInitContainerForRestore(
	cluster *appsv1alpha1.Cluster,
	componentName,
	backupName string) (bool, error) {
	// get the sts list of component
	stsList := &appsv1.StatefulSetList{}
	if err := util.GetObjectListByComponentName(c.ctx.Ctx, c.cli, *cluster, stsList, componentName); err != nil {
		return false, err
	}
	var doRemoveInitContainers bool
	for _, sts := range stsList.Items {
		initContainers := sts.Spec.Template.Spec.InitContainers
		restoreInitContainerName := component.GetRestoredInitContainerName(backupName)
		restoreInitContainerIndex, _ := intctrlutil.GetContainerByName(initContainers, restoreInitContainerName)
		if restoreInitContainerIndex == -1 {
			continue
		}
		doRemoveInitContainers = true
		initContainers = append(initContainers[:restoreInitContainerIndex], initContainers[restoreInitContainerIndex+1:]...)
		sts.Spec.Template.Spec.InitContainers = initContainers
		if err := c.cli.Update(c.ctx.Ctx, &sts); err != nil {
			return false, err
		}
	}
	if doRemoveInitContainers {
		// if need to remove init container, reset component to Creating.
		compStatus := cluster.Status.Components[componentName]
		compStatus.Phase = appsv1alpha1.CreatingClusterCompPhase
		cluster.Status.Components[componentName] = compStatus
	}
	return doRemoveInitContainers, nil
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
	case appsv1alpha1.Consensus:
		return true
	case appsv1alpha1.Replication:
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
