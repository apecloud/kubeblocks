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
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
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
	cc       clusterRefResources
	cli      client.Client
	ctx      intctrlutil.RequestCtx
	recorder record.EventRecorder
	conMgr   clusterConditionManager2
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
		// TODO: move from object action, check it again
		for _, vertex := range dag.Vertices() {
			v, _ := vertex.(*lifecycleVertex)
			v.action = actionPtr(DELETE)
		}
		// TODO: delete orphan resources which are not in dag?
	case isClusterUpdating(*origCluster):
		c.ctx.Log.Info("update cluster status")
		defer func() {
			rootVertex.action = actionPtr(STATUS)
			// update components' phase in cluster.status
			updateComponentPhase()
		}()
		cluster.Status.ObservedGeneration = cluster.Generation
		cluster.Status.ClusterDefGeneration = c.cc.cd.Generation
		if cluster.Status.Phase == "" {
			// REVIEW: may need to start with "validating" phase
			cluster.Status.Phase = appsv1alpha1.StartingClusterPhase
			return nil
		}
		if cluster.Status.Phase != appsv1alpha1.StartingClusterPhase {
			cluster.Status.Phase = appsv1alpha1.SpecReconcilingClusterPhase
		}
		if err := c.conMgr.setProvisioningStartedCondition(cluster); util.IgnoreNoOps(err) != nil {
			return err
		}
		applyResourcesCondition := newApplyResourcesCondition()
		oldApplyCondition := meta.FindStatusCondition(cluster.Status.Conditions, applyResourcesCondition.Type)
		meta.SetStatusCondition(&cluster.Status.Conditions, applyResourcesCondition)
		if oldApplyCondition == nil || oldApplyCondition.Status != applyResourcesCondition.Status {
			c.recorder.Event(cluster, corev1.EventTypeNormal, applyResourcesCondition.Reason, applyResourcesCondition.Message)
		}
	case isClusterStatusUpdating(*origCluster):
		defer func() {
			rootVertex.action = actionPtr(STATUS)
			// TODO: move from object action, check it again
			vertices := findAllNot[*appsv1alpha1.Cluster](dag)
			for _, vertex := range vertices {
				v, _ := vertex.(*lifecycleVertex)
				// TODO: fix me, workaround for h-scaling to update stateful set
				if _, ok := v.obj.(*appsv1.StatefulSet); !ok {
					v.immutable = true
				}
			}
		}()
		// checks if the controller is handling the garbage of restore.
		if err := c.handleGarbageOfRestoreBeforeRunning(cluster); err == nil {
			return nil
		} else if util.IgnoreNoOps(err) != nil {
			return err
		}
		// reconcile the phase and conditions of the Cluster.status
		if err := c.reconcileClusterStatus(cluster, c.cc.cd); util.IgnoreNoOps(err) != nil {
			return err
		}
		c.cleanupAnnotationsAfterRunning(cluster)
	}

	return nil
}

// REVIEW: this handling rather monolithic
// reconcileClusterStatus reconciles phase and conditions of the Cluster.status.
// @return ErrNoOps if no operation
// Deprecated:
func (c *clusterStatusTransformer) reconcileClusterStatus(cluster *appsv1alpha1.Cluster,
	clusterDef appsv1alpha1.ClusterDefinition) error {
	if len(cluster.Status.Components) == 0 {
		return nil
	}

	var (
		currentClusterPhase       appsv1alpha1.ClusterPhase
		existsAbnormalOrFailed    bool
		notReadyCompNames         = map[string]struct{}{}
		replicasNotReadyCompNames = map[string]struct{}{}
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
			case appsv1alpha1.AbnormalClusterCompPhase, appsv1alpha1.FailedClusterCompPhase:
				existsAbnormalOrFailed = true
				notReadyCompNames[k] = struct{}{}
			case appsv1alpha1.RunningClusterCompPhase:
				runningCompCount += 1
			case appsv1alpha1.StoppedClusterCompPhase:
				stoppedCompCount += 1
			}
		}
		compLen := len(cluster.Status.Components)
		notReadyLen := len(notReadyCompNames)
		if existsAbnormalOrFailed && notReadyLen > 0 {
			if compLen == notReadyLen {
				currentClusterPhase = appsv1alpha1.FailedClusterPhase
			} else {
				currentClusterPhase = appsv1alpha1.AbnormalClusterPhase
			}
			return
		}
		switch len(cluster.Status.Components) {
		case 0:
			// if no components, return, and how could this possible?
			return
		case runningCompCount:
			currentClusterPhase = appsv1alpha1.RunningClusterPhase
		case stoppedCompCount:
			// cluster is Stopped when cluster is not Running and all components are Stopped or Running
			currentClusterPhase = appsv1alpha1.StoppedClusterPhase
		}
	}

	// remove the invalid component in status.components when spec.components changed and analysis the status of components.
	removeInvalidComponentsAndAnalysis := func(cluster *appsv1alpha1.Cluster) (postHandler, error) {
		tmpCompsStatus := map[string]appsv1alpha1.ClusterComponentStatus{}
		compsStatus := cluster.Status.Components
		for _, v := range cluster.Spec.ComponentSpecs {
			if compStatus, ok := compsStatus[v.Name]; ok {
				tmpCompsStatus[v.Name] = compStatus
			}
		}
		if len(tmpCompsStatus) != len(compsStatus) {
			// keep valid components' status
			cluster.Status.Components = tmpCompsStatus
			return nil, nil
		}
		analysisComponentsStatus(cluster)
		return nil, util.ErrNoOps
	}

	// handle the cluster conditions with ClusterReady and ReplicasReady type.
	handleClusterReadyCondition := func(cluster *appsv1alpha1.Cluster) (postHandler, error) {
		return handleNotReadyConditionForCluster(cluster, c.recorder, replicasNotReadyCompNames, notReadyCompNames)
	}

	// processes cluster phase changes.
	processClusterPhaseChanges := func(cluster *appsv1alpha1.Cluster,
		oldPhase,
		currPhase appsv1alpha1.ClusterPhase,
		eventType string,
		eventMessage string,
		doAction func(cluster *appsv1alpha1.Cluster)) (postHandler, error) {
		if oldPhase == currPhase {
			return nil, util.ErrNoOps
		}
		cluster.Status.Phase = currPhase
		if doAction != nil {
			doAction(cluster)
		}
		postFuncAfterPatch := func(currCluster *appsv1alpha1.Cluster) error {
			c.recorder.Event(currCluster, eventType, string(currPhase), eventMessage)
			return opsutil.MarkRunningOpsRequestAnnotation(c.ctx.Ctx, c.cli, currCluster)
		}
		return postFuncAfterPatch, nil
	}
	// handle the Cluster.status when some components of cluster are Abnormal or Failed.
	handleExistAbnormalOrFailed := func(cluster *appsv1alpha1.Cluster) (postHandler, error) {
		if !existsAbnormalOrFailed {
			return nil, util.ErrNoOps
		}
		oldPhase := cluster.Status.Phase
		componentMap, clusterAvailabilityEffectMap, _ := getComponentRelatedInfo(cluster,
			clusterDef, "")
		// handle the cluster status when some components are not ready.
		handleClusterPhaseWhenCompsNotReady(cluster, componentMap, clusterAvailabilityEffectMap)
		currPhase := cluster.Status.Phase
		if !slices.Contains(appsv1alpha1.GetClusterFailedPhases(), currPhase) {
			return nil, util.ErrNoOps
		}
		message := fmt.Sprintf("Cluster: %s is %s, check according to the components message",
			cluster.Name, currPhase)
		return processClusterPhaseChanges(cluster, oldPhase, currPhase,
			corev1.EventTypeWarning, message, nil)
	}

	// handle the Cluster.status when cluster is Stopped.
	handleClusterIsStopped := func(cluster *appsv1alpha1.Cluster) (postHandler, error) {
		if currentClusterPhase != appsv1alpha1.StoppedClusterPhase {
			return nil, util.ErrNoOps
		}
		message := fmt.Sprintf("Cluster: %s stopped successfully.", cluster.Name)
		oldPhase := cluster.Status.Phase
		return processClusterPhaseChanges(cluster, oldPhase, currentClusterPhase,
			corev1.EventTypeNormal, message, nil)
	}

	// handle the Cluster.status when cluster is Running.
	handleClusterIsRunning := func(cluster *appsv1alpha1.Cluster) (postHandler, error) {
		if currentClusterPhase != appsv1alpha1.RunningClusterPhase {
			return nil, util.ErrNoOps
		}
		message := fmt.Sprintf("Cluster: %s is ready, current phase is Running.", cluster.Name)
		action := func(currCluster *appsv1alpha1.Cluster) {
			meta.SetStatusCondition(&currCluster.Status.Conditions,
				newClusterReadyCondition(currCluster.Name))
		}
		oldPhase := cluster.Status.Phase
		return processClusterPhaseChanges(cluster, oldPhase, currentClusterPhase,
			corev1.EventTypeNormal, message, action)
	}
	if err := doChainClusterStatusHandler(cluster,
		removeInvalidComponentsAndAnalysis,
		handleClusterReadyCondition,
		handleExistAbnormalOrFailed,
		handleClusterIsStopped,
		handleClusterIsRunning); err != nil {
		return err
	}
	return nil
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
		return util.ErrNoOps
	}
	// check if all components are running.
	for _, v := range cluster.Status.Components {
		if v.Phase != appsv1alpha1.RunningClusterCompPhase {
			return util.ErrNoOps
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
	return util.ErrNoOps
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
		compStatus.Phase = appsv1alpha1.StartingClusterCompPhase
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
		clusterIsFailed bool
		failedCompCount int
	)
	for k, v := range cluster.Status.Components {
		// determine whether other components are still doing operation, i.e., create/restart/scaling.
		// waiting for operation to complete except for volumeExpansion operation.
		// because this operation will not affect cluster availability.
		// TODO: for appsv1alpha1.VolumeExpandingPhas requires extra handling
		if !util.IsCompleted(v.Phase) {
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

// doChainClusterStatusHandler chain processing clusterStatusHandler.
func doChainClusterStatusHandler(cluster *appsv1alpha1.Cluster,
	handlers ...clusterStatusHandler) error {
	var (
		needPatchStatus bool
		postHandlers    = make([]func(cluster *appsv1alpha1.Cluster) error, 0, len(handlers))
	)
	for _, statusHandler := range handlers {
		postFunc, err := statusHandler(cluster)
		if err != nil {
			if err == util.ErrNoOps {
				continue
			}
			return err
		}
		needPatchStatus = true
		if postFunc != nil {
			postHandlers = append(postHandlers, postFunc)
		}
	}
	if !needPatchStatus {
		return util.ErrNoOps
	}
	// perform the handlers after patched the cluster status.
	for _, postFunc := range postHandlers {
		if err := postFunc(cluster); err != nil {
			return err
		}
	}
	return nil
}

// TODO: dedup the following funcs

// PatchOpsRequestReconcileAnnotation patches the reconcile annotation to OpsRequest
func PatchOpsRequestReconcileAnnotation(ctx context.Context, cli client.Client, namespace string, opsRequestName string) error {
	opsRequest := &appsv1alpha1.OpsRequest{}
	if err := cli.Get(ctx, client.ObjectKey{Name: opsRequestName, Namespace: namespace}, opsRequest); err != nil {
		return err
	}
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Annotations == nil {
		opsRequest.Annotations = map[string]string{}
	}
	// because many changes may be triggered within one second, if the accuracy is only seconds, the event may be lost.
	// so we used RFC3339Nano format.
	opsRequest.Annotations[constant.ReconcileAnnotationKey] = time.Now().Format(time.RFC3339Nano)
	return cli.Patch(ctx, opsRequest, patch)
}

// GetOpsRequestSliceFromCluster gets OpsRequest slice from cluster annotations.
// this records what OpsRequests are running in cluster
func GetOpsRequestSliceFromCluster(cluster *appsv1alpha1.Cluster) ([]appsv1alpha1.OpsRecorder, error) {
	var (
		opsRequestValue string
		opsRequestSlice []appsv1alpha1.OpsRecorder
		ok              bool
	)
	if cluster == nil || cluster.Annotations == nil {
		return nil, nil
	}
	if opsRequestValue, ok = cluster.Annotations[constant.OpsRequestAnnotationKey]; !ok {
		return nil, nil
	}
	// opsRequest annotation value in cluster to slice
	if err := json.Unmarshal([]byte(opsRequestValue), &opsRequestSlice); err != nil {
		return nil, err
	}
	return opsRequestSlice, nil
}
