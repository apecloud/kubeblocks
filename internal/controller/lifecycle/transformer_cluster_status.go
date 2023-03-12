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
	"context"
	"fmt"
	"golang.org/x/exp/slices"
	"reflect"
	"time"

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

type clusterStatusTransformer struct {
	cc       compoundCluster
	cli      client.Client
	ctx      intctrlutil.RequestCtx
	recorder record.EventRecorder
}

func (c *clusterStatusTransformer) Transform(dag *graph.DAG) error {
	cluster := c.cc.cluster.DeepCopy()
	if !cluster.DeletionTimestamp.IsZero() {
		return nil
	}

	// handle cluster.status when spec is updating
	if cluster.Status.ObservedGeneration != cluster.Generation {
		c.ctx.Log.Info("update cluster status")
		if err := c.updateClusterPhaseToCreatingOrUpdating(c.ctx, cluster); err != nil {
			return err
		}
		if err := c.setProvisioningStartedCondition(); err != nil {
			return err
		}
		return nil
	}

	// checks if the controller is handling the garbage of restore.
	if handlingRestoreGarbage, err := c.handleGarbageOfRestoreBeforeRunning(cluster); err != nil {
		return err
	} else if handlingRestoreGarbage {
		return nil
	}
	// reconcile the phase and conditions of the Cluster.status
	if err := c.reconcileClusterStatus(cluster, &c.cc.cd); err != nil {
		return err
	}
	if err := c.cleanupAnnotationsAfterRunning(cluster); err != nil {
		return err
	}

	return nil
}

// setProvisioningStartedCondition sets the provisioning started condition in cluster conditions.
func (c *clusterStatusTransformer) setProvisioningStartedCondition() error {
	condition := metav1.Condition{
		Type:    ConditionTypeProvisioningStarted,
		Status:  metav1.ConditionTrue,
		Message: fmt.Sprintf("The operator has started the provisioning of Cluster: %s", c.cc.cluster.Name),
		Reason:  ReasonPreCheckSucceed,
	}
	return c.updateStatusConditions(condition)
}

// updateClusterConditions updates cluster.status condition and records event.
func (c *clusterStatusTransformer) updateStatusConditions(condition metav1.Condition) error {
	patch := client.MergeFrom(c.cc.cluster.DeepCopy())
	oldCondition := meta.FindStatusCondition(c.cc.cluster.Status.Conditions, condition.Type)
	phaseChanged := c.handleConditionForClusterPhase(oldCondition, condition)
	conditionChanged := !reflect.DeepEqual(oldCondition, condition)
	if conditionChanged || phaseChanged {
		c.cc.cluster.SetStatusCondition(condition)
		if err := c.cli.Status().Patch(c.ctx.Ctx, c.cc.cluster, patch); err != nil {
			return err
		}
	}
	if conditionChanged {
		eventType := corev1.EventTypeWarning
		if condition.Status == metav1.ConditionTrue {
			eventType = corev1.EventTypeNormal
		}
		c.recorder.Event(c.cc.cluster, eventType, condition.Reason, condition.Message)
	}
	if phaseChanged {
		// if cluster status changed, do it
		return opsutil.MarkRunningOpsRequestAnnotation(c.ctx.Ctx, c.cli, c.cc.cluster)
	}
	return nil
}

// handleConditionForClusterPhase checks whether the condition can be repaired by cluster.
// if it cannot be repaired after 30 seconds, set the cluster status to ConditionsError
func (c *clusterStatusTransformer) handleConditionForClusterPhase(oldCondition *metav1.Condition, condition metav1.Condition) bool {
	if condition.Status == metav1.ConditionTrue {
		return false
	}

	if oldCondition == nil || oldCondition.Reason != condition.Reason {
		return false
	}

	if time.Now().Before(oldCondition.LastTransitionTime.Add(ClusterControllerErrorDuration)) {
		return false
	}
	if !util.IsFailedOrAbnormal(c.cc.cluster.Status.Phase) &&
		c.cc.cluster.Status.Phase != appsv1alpha1.ConditionsErrorPhase {
		// the condition has occurred for more than 30 seconds and cluster status is not Failed/Abnormal, do it
		c.cc.cluster.Status.Phase = appsv1alpha1.ConditionsErrorPhase
		return true
	}
	return false
}

// updateClusterPhase updates cluster.status.phase
func (c *clusterStatusTransformer) updateClusterPhaseToCreatingOrUpdating(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster) error {
	needPatch := false
	patch := client.MergeFrom(cluster.DeepCopy())
	if cluster.Status.Phase == "" {
		needPatch = true
		cluster.Status.Phase = appsv1alpha1.CreatingPhase
		cluster.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{}
		for _, v := range cluster.Spec.ComponentSpecs {
			cluster.Status.Components[v.Name] = appsv1alpha1.ClusterComponentStatus{
				Phase: appsv1alpha1.CreatingPhase,
			}
		}
	} else if util.IsCompleted(cluster.Status.Phase) && !existsOperations(cluster) {
		needPatch = true
		cluster.Status.Phase = appsv1alpha1.SpecUpdatingPhase
	}
	if !needPatch {
		return nil
	}
	if err := c.cli.Status().Patch(c.ctx.Ctx, cluster, patch); err != nil {
		return err
	}
	// send an event when cluster perform operations
	c.recorder.Eventf(cluster, corev1.EventTypeNormal, string(cluster.Status.Phase),
		"Start %s in Cluster: %s", cluster.Status.Phase, cluster.Name)
	return nil
}

// existsOperations checks if the cluster is doing operations
func existsOperations(cluster *appsv1alpha1.Cluster) bool {
	opsRequestMap, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	_, isRestoring := cluster.Annotations[constant.RestoreFromBackUpAnnotationKey]
	return len(opsRequestMap) > 0 || isRestoring
}

func (c *clusterStatusTransformer) needCheckClusterForReady(cluster *appsv1alpha1.Cluster) bool {
	return slices.Index([]appsv1alpha1.Phase{"", appsv1alpha1.DeletingPhase, appsv1alpha1.VolumeExpandingPhase},
		cluster.Status.Phase) == -1
}

// reconcileClusterStatus reconciles phase and conditions of the Cluster.status.
func (c *clusterStatusTransformer) reconcileClusterStatus(
	cluster *appsv1alpha1.Cluster,
	clusterDef *appsv1alpha1.ClusterDefinition) error {
	if !c.needCheckClusterForReady(cluster) {
		return nil
	}
	if cluster.Status.Components == nil {
		return nil
	}

	var (
		currentClusterPhase       appsv1alpha1.Phase
		existsAbnormalOrFailed    bool
		replicasNotReadyCompNames = map[string]struct{}{}
		notReadyCompNames         = map[string]struct{}{}
	)

	// analysis the status of components and calculate the cluster phase .
	analysisComponentsStatus := func(cluster *appsv1alpha1.Cluster) {
		var (
			runningCompCount int
			stoppedCompCount int
		)
		for k, v := range cluster.Status.Components {
			if v.PodsReady == nil || !*v.PodsReady {
				replicasNotReadyCompNames[k] = struct{}{}
				notReadyCompNames[k] = struct{}{}
			}
			switch v.Phase {
			case appsv1alpha1.AbnormalPhase, appsv1alpha1.FailedPhase:
				existsAbnormalOrFailed = true
				notReadyCompNames[k] = struct{}{}
			case appsv1alpha1.RunningPhase:
				runningCompCount += 1
			case appsv1alpha1.StoppedPhase:
				stoppedCompCount += 1
			}
		}
		switch len(cluster.Status.Components) {
		case 0:
			// if no components, return
			return
		case runningCompCount:
			currentClusterPhase = appsv1alpha1.RunningPhase
		case runningCompCount + stoppedCompCount:
			// cluster is Stopped when cluster is not Running and all components are Stopped or Running
			currentClusterPhase = appsv1alpha1.StoppedPhase
		}
	}

	// remove the invalid component in status.components when spec.components changed and analysis the status of components.
	removeInvalidComponentsAndAnalysis := func(cluster *appsv1alpha1.Cluster) (bool, postHandler) {
		tmpCompsStatus := map[string]appsv1alpha1.ClusterComponentStatus{}
		compsStatus := cluster.Status.Components
		for _, v := range cluster.Spec.ComponentSpecs {
			if compStatus, ok := compsStatus[v.Name]; ok {
				tmpCompsStatus[v.Name] = compStatus
			}
		}
		var needPatch bool
		if len(tmpCompsStatus) != len(compsStatus) {
			// keep valid components' status
			cluster.Status.Components = tmpCompsStatus
			needPatch = true
		}
		analysisComponentsStatus(cluster)
		return needPatch, nil
	}

	// handle the cluster conditions with ClusterReady and ReplicasReady type.
	handleClusterReadyCondition := func(cluster *appsv1alpha1.Cluster) (needPatch bool, postFunc postHandler) {
		return handleNotReadyConditionForCluster(cluster, c.recorder, replicasNotReadyCompNames, notReadyCompNames)
	}

	// processes cluster phase changes.
	processClusterPhaseChanges := func(cluster *appsv1alpha1.Cluster,
		oldPhase,
		currPhase appsv1alpha1.Phase,
		eventType string,
		eventMessage string,
		doAction func(cluster *appsv1alpha1.Cluster)) (bool, postHandler) {
		if oldPhase == currPhase {
			return false, nil
		}
		cluster.Status.Phase = currPhase
		if doAction != nil {
			doAction(cluster)
		}
		postFuncAfterPatch := func(currCluster *appsv1alpha1.Cluster) error {
			c.recorder.Event(currCluster, eventType, string(currPhase), eventMessage)
			return opsutil.MarkRunningOpsRequestAnnotation(c.ctx.Ctx, c.cli, currCluster)
		}
		return true, postFuncAfterPatch
	}
	// handle the Cluster.status when some components of cluster are Abnormal or Failed.
	handleExistAbnormalOrFailed := func(cluster *appsv1alpha1.Cluster) (bool, postHandler) {
		if !existsAbnormalOrFailed {
			return false, nil
		}
		oldPhase := cluster.Status.Phase
		componentMap, clusterAvailabilityEffectMap, _ := getComponentRelatedInfo(cluster, clusterDef, "")
		// handle the cluster status when some components are not ready.
		handleClusterPhaseWhenCompsNotReady(cluster, componentMap, clusterAvailabilityEffectMap)
		currPhase := cluster.Status.Phase
		if !util.IsFailedOrAbnormal(currPhase) {
			return false, nil
		}
		message := fmt.Sprintf("Cluster: %s is %s, check according to the components message", cluster.Name, currPhase)
		return processClusterPhaseChanges(cluster, oldPhase, currPhase, corev1.EventTypeWarning, message, nil)
	}

	// handle the Cluster.status when cluster is Stopped.
	handleClusterIsStopped := func(cluster *appsv1alpha1.Cluster) (bool, postHandler) {
		if currentClusterPhase != appsv1alpha1.StoppedPhase {
			return false, nil
		}
		message := fmt.Sprintf("Cluster: %s stopped successfully.", cluster.Name)
		oldPhase := cluster.Status.Phase
		return processClusterPhaseChanges(cluster, oldPhase, currentClusterPhase, corev1.EventTypeNormal, message, nil)
	}

	// handle the Cluster.status when cluster is Running.
	handleClusterIsRunning := func(cluster *appsv1alpha1.Cluster) (bool, postHandler) {
		if currentClusterPhase != appsv1alpha1.RunningPhase {
			return false, nil
		}
		message := fmt.Sprintf("Cluster: %s is ready, current phase is Running.", cluster.Name)
		action := func(currCluster *appsv1alpha1.Cluster) {
			currCluster.SetStatusCondition(newClusterReadyCondition(currCluster.Name))
		}
		oldPhase := cluster.Status.Phase
		return processClusterPhaseChanges(cluster, oldPhase, currentClusterPhase, corev1.EventTypeNormal, message, action)
	}
	return doChainClusterStatusHandler(c.ctx.Ctx, c.cli, cluster, removeInvalidComponentsAndAnalysis,
		handleClusterReadyCondition, handleExistAbnormalOrFailed, handleClusterIsStopped, handleClusterIsRunning)
}

// cleanupAnnotationsAfterRunning cleans up the cluster annotations after cluster is Running.
func (c *clusterStatusTransformer) cleanupAnnotationsAfterRunning(cluster *appsv1alpha1.Cluster) error {
	if cluster.Status.Phase != appsv1alpha1.RunningPhase {
		return nil
	}
	if _, ok := cluster.Annotations[constant.RestoreFromBackUpAnnotationKey]; !ok {
		return nil
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	delete(cluster.Annotations, constant.RestoreFromBackUpAnnotationKey)
	return c.cli.Patch(c.ctx.Ctx, cluster, patch)
}

// handleRestoreGarbageBeforeRunning handles the garbage for restore before cluster phase changes to Running.
func (c *clusterStatusTransformer) handleGarbageOfRestoreBeforeRunning(cluster *appsv1alpha1.Cluster) (bool, error) {
	clusterBackupResourceMap, err := getClusterBackupSourceMap(cluster)
	if err != nil {
		return false, err
	}
	if clusterBackupResourceMap == nil {
		return false, nil
	}
	// check if all components are running.
	for _, v := range cluster.Status.Components {
		if v.Phase != appsv1alpha1.RunningPhase {
			return false, nil
		}
	}
	// remove the garbage for restore if the cluster restores from backup.
	return c.removeGarbageWithRestore(cluster, clusterBackupResourceMap)
}

// removeGarbageWithRestore removes the garbage for restore when all components are Running.
func (c *clusterStatusTransformer) removeGarbageWithRestore(
	cluster *appsv1alpha1.Cluster,
	clusterBackupResourceMap map[string]string) (bool, error) {
	var (
		doRemoveInitContainers bool
		err                    error
	)
	clusterPatch := client.MergeFrom(cluster.DeepCopy())
	for k, v := range clusterBackupResourceMap {
		// remove the init container for restore
		if doRemoveInitContainers, err = c.removeStsInitContainerForRestore(cluster, k, v); err != nil {
			return false, err
		}
	}
	if doRemoveInitContainers {
		// reset the component phase to Creating during removing the init containers of statefulSet.
		return doRemoveInitContainers, c.cli.Status().Patch(c.ctx.Ctx, cluster, clusterPatch)
	}
	return false, nil
}

// removeStsInitContainerForRestore removes the statefulSet's init container which restores data from backup.
func (c *clusterStatusTransformer) removeStsInitContainerForRestore(
	cluster *appsv1alpha1.Cluster,
	componentName,
	backupName string) (bool, error) {
	// get the sts list of component
	stsList := &appsv1.StatefulSetList{}
	if err := util.GetObjectListByComponentName(c.ctx.Ctx, c.cli, cluster, stsList, componentName); err != nil {
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
		compStatus.Phase = appsv1alpha1.CreatingPhase
		cluster.Status.Components[componentName] = compStatus
	}
	return doRemoveInitContainers, nil
}

// handleClusterPhaseWhenCompsNotReady handles the Cluster.status.phase when some components are Abnormal or Failed.
func handleClusterPhaseWhenCompsNotReady(cluster *appsv1alpha1.Cluster,
	componentMap map[string]string,
	clusterAvailabilityEffectMap map[string]bool) {
	var (
		clusterIsFailed bool
		failedCompCount int
	)
	for k, v := range cluster.Status.Components {
		// determine whether other components are still doing operation, i.e., create/restart/scaling.
		// waiting for operation to complete except for volumeExpansion operation.
		// because this operation will not affect cluster availability.
		if !util.IsCompleted(v.Phase) && v.Phase != appsv1alpha1.VolumeExpandingPhase {
			return
		}
		if v.Phase == appsv1alpha1.FailedPhase {
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
		cluster.Status.Phase = appsv1alpha1.FailedPhase
	} else {
		cluster.Status.Phase = appsv1alpha1.AbnormalPhase
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
func getComponentRelatedInfo(cluster *appsv1alpha1.Cluster, clusterDef *appsv1alpha1.ClusterDefinition, componentName string) (map[string]string, map[string]bool, appsv1alpha1.ClusterComponentDefinition) {
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

// doChainClusterStatusHandler chain processing clusterStatusHandler.
func doChainClusterStatusHandler(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	handlers ...clusterStatusHandler) error {
	patch := client.MergeFrom(cluster.DeepCopy())
	var (
		needPatchStatus bool
		postHandlers    = make([]func(cluster *appsv1alpha1.Cluster) error, 0, len(handlers))
	)
	for _, statusHandler := range handlers {
		needPatch, postFunc := statusHandler(cluster)
		if needPatch {
			needPatchStatus = true
		}
		if postFunc != nil {
			postHandlers = append(postHandlers, postFunc)
		}
	}
	if !needPatchStatus {
		return nil
	}
	if err := cli.Status().Patch(ctx, cluster, patch); err != nil {
		return err
	}
	// perform the handlers after patched the cluster status.
	for _, postFunc := range postHandlers {
		if err := postFunc(cluster); err != nil {
			return err
		}
	}
	return nil
}
