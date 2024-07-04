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
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	// componentPhaseTransition the event reason indicates that the component transits to a new phase.
	componentPhaseTransition = "ComponentPhaseTransition"

	// defaultRoleProbeTimeoutAfterPodsReady the default role probe timeout for application when all pods of component are ready.
	defaultRoleProbeTimeoutAfterPodsReady int32 = 60
)

// componentStatusTransformer computes the current status: read the underlying workload status and update the component status
type componentStatusTransformer struct {
	client.Client

	cluster        *appsv1alpha1.Cluster
	comp           *appsv1alpha1.Component
	synthesizeComp *component.SynthesizedComponent
	dag            *graph.DAG

	// runningITS is a snapshot of the ITS that is already running
	runningITS *workloads.InstanceSet
	// protoITS is the ITS object that is rebuilt from scratch during each reconcile process
	protoITS *workloads.InstanceSet
}

var _ graph.Transformer = &componentStatusTransformer{}

func (t *componentStatusTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}

	comp := transCtx.Component
	if transCtx.RunningWorkload == nil {
		transCtx.Logger.Info(fmt.Sprintf("skip reconcile status because underlying workload not found, generation: %d", comp.Generation))
		return nil
	}

	t.init(transCtx, dag)

	switch {
	case model.IsObjectUpdating(transCtx.ComponentOrig):
		transCtx.Logger.Info(fmt.Sprintf("update status after applying new spec, generation: %d", comp.Generation))
		comp.Status.ObservedGeneration = comp.Generation
	case model.IsObjectStatusUpdating(transCtx.ComponentOrig):
		if err := t.reconcileStatus(transCtx); err != nil {
			return err
		}
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	if vertex := graphCli.FindMatchedVertex(dag, comp); vertex != nil {
		// check if the component needs to do other action.
		ov, _ := vertex.(*model.ObjectVertex)
		if ov.Action != model.ActionNoopPtr() {
			return nil
		}
	}
	graphCli.Status(dag, transCtx.ComponentOrig, comp)
	return nil
}

func (t *componentStatusTransformer) init(transCtx *componentTransformContext, dag *graph.DAG) {
	t.cluster = transCtx.Cluster
	t.comp = transCtx.Component
	t.synthesizeComp = transCtx.SynthesizeComponent
	t.runningITS = transCtx.RunningWorkload.(*workloads.InstanceSet)
	t.protoITS = transCtx.ProtoWorkload.(*workloads.InstanceSet)
	t.dag = dag
}

// reconcileStatus reconciles component status.
func (t *componentStatusTransformer) reconcileStatus(transCtx *componentTransformContext) error {
	if t.runningITS == nil {
		return nil
	}

	// check if the ITS is deleting
	isDeleting := func() bool {
		return !t.runningITS.DeletionTimestamp.IsZero()
	}()

	stopped := isCompStopped(t.synthesizeComp)

	hasRunningPods := func() bool {
		return t.runningITS.Status.Replicas > 0
	}()

	// check if the ITS is running
	isITSUpdatedNRunning := t.isInstanceSetRunning()

	// check if all configTemplates are synced
	isAllConfigSynced, err := t.isAllConfigSynced(transCtx)
	if err != nil {
		return err
	}

	// check if the component has failed pod
	hasFailedPod, messages := t.hasFailedPod()

	// check if the component scale out failed
	isScaleOutFailed, err := t.isScaleOutFailed(transCtx)
	if err != nil {
		return err
	}

	// check if the volume expansion is running
	hasRunningVolumeExpansion, hasFailedVolumeExpansion, err := t.hasVolumeExpansionRunning(transCtx)
	if err != nil {
		return err
	}

	// calculate if the component has failure
	hasFailure := func() bool {
		return hasFailedPod || isScaleOutFailed || hasFailedVolumeExpansion
	}()

	// check if the component is available
	isComponentAvailable := t.isComponentAvailable()

	// check if the component is in creating phase
	isInCreatingPhase := func() bool {
		phase := t.comp.Status.Phase
		return phase == "" || phase == appsv1alpha1.CreatingClusterCompPhase
	}()

	transCtx.Logger.Info(
		fmt.Sprintf("status conditions, creating: %v, available: %v, its running: %v, has failure: %v, updating: %v, config synced: %v",
			isInCreatingPhase, isComponentAvailable, isITSUpdatedNRunning, hasFailure, hasRunningVolumeExpansion, isAllConfigSynced))

	switch {
	case isDeleting:
		t.setComponentStatusPhase(transCtx, appsv1alpha1.DeletingClusterCompPhase, nil, "component is Deleting")
	case stopped && hasRunningPods:
		t.setComponentStatusPhase(transCtx, appsv1alpha1.StoppingClusterCompPhase, nil, "component is Stopping")
	case stopped:
		t.setComponentStatusPhase(transCtx, appsv1alpha1.StoppedClusterCompPhase, nil, "component is Stopped")
	case isITSUpdatedNRunning && isAllConfigSynced && !hasRunningVolumeExpansion:
		t.setComponentStatusPhase(transCtx, appsv1alpha1.RunningClusterCompPhase, nil, "component is Running")
	case !hasFailure && isInCreatingPhase:
		t.setComponentStatusPhase(transCtx, appsv1alpha1.CreatingClusterCompPhase, nil, "component is Creating")
	case !hasFailure:
		t.setComponentStatusPhase(transCtx, appsv1alpha1.UpdatingClusterCompPhase, nil, "component is Updating")
	case !isComponentAvailable:
		t.setComponentStatusPhase(transCtx, appsv1alpha1.FailedClusterCompPhase, messages, "component is Failed")
	default:
		t.setComponentStatusPhase(transCtx, appsv1alpha1.AbnormalClusterCompPhase, nil, "component is Abnormal")
	}

	return nil
}

func (t *componentStatusTransformer) isWorkloadUpdated() bool {
	if t.cluster == nil || t.runningITS == nil {
		return false
	}
	// check whether component spec has been sent to the underlying workload
	itsComponentGeneration := t.runningITS.GetAnnotations()[constant.KubeBlocksGenerationKey]
	return itsComponentGeneration == strconv.FormatInt(t.cluster.Generation, 10)
}

// isComponentAvailable tells whether the component is basically available, ether working well or in a fragile state:
// 1. at least one pod is available
// 2. with latest revision
// 3. and with leader role label set
func (t *componentStatusTransformer) isComponentAvailable() bool {
	if !t.isWorkloadUpdated() {
		return false
	}
	if t.runningITS.Status.CurrentRevision != t.runningITS.Status.UpdateRevision {
		return false
	}
	if t.runningITS.Status.AvailableReplicas <= 0 {
		return false
	}
	if len(t.synthesizeComp.Roles) == 0 {
		return true
	}
	for _, status := range t.runningITS.Status.MembersStatus {
		if status.ReplicaRole.IsLeader {
			return true
		}
	}
	return false
}

// isRunning checks if the component underlying workload is running.
func (t *componentStatusTransformer) isInstanceSetRunning() bool {
	if t.runningITS == nil {
		return false
	}
	if !t.isWorkloadUpdated() {
		return false
	}
	return instanceset.IsInstanceSetReady(t.runningITS)
}

// isAllConfigSynced checks if all configTemplates are synced.
func (t *componentStatusTransformer) isAllConfigSynced(transCtx *componentTransformContext) (bool, error) {
	var (
		cmKey client.ObjectKey
		cmObj = &corev1.ConfigMap{}
	)

	if len(t.synthesizeComp.ConfigTemplates) == 0 {
		return true, nil
	}

	configurationKey := client.ObjectKey{
		Namespace: t.cluster.Namespace,
		Name:      cfgcore.GenerateComponentConfigurationName(t.cluster.Name, t.synthesizeComp.Name),
	}
	configuration := &appsv1alpha1.Configuration{}
	if err := t.Client.Get(transCtx.Context, configurationKey, configuration); err != nil {
		return false, err
	}
	for _, configSpec := range t.synthesizeComp.ConfigTemplates {
		item := configuration.Spec.GetConfigurationItem(configSpec.Name)
		status := configuration.Status.GetItemStatus(configSpec.Name)
		// for creating phase
		if item == nil || status == nil {
			return false, nil
		}
		cmKey = client.ObjectKey{
			Namespace: t.cluster.Namespace,
			Name:      cfgcore.GetComponentCfgName(t.cluster.Name, t.synthesizeComp.Name, configSpec.Name),
		}
		if err := t.Client.Get(transCtx.Context, cmKey, cmObj, inDataContext4C()); err != nil {
			return false, err
		}
		if intctrlutil.GetConfigSpecReconcilePhase(cmObj, *item, status) != appsv1alpha1.CFinishedPhase {
			return false, nil
		}
	}
	return true, nil
}

// isScaleOutFailed checks if the component scale out failed.
func (t *componentStatusTransformer) isScaleOutFailed(transCtx *componentTransformContext) (bool, error) {
	if t.runningITS == nil {
		return false, nil
	}
	if t.runningITS.Spec.Replicas == nil {
		return false, nil
	}
	if t.synthesizeComp.Replicas <= *t.runningITS.Spec.Replicas {
		return false, nil
	}

	reqCtx := intctrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}
	backupKey := types.NamespacedName{
		Namespace: t.runningITS.Namespace,
		Name:      constant.GenerateResourceNameWithScalingSuffix(t.runningITS.Name),
	}
	d, err := newDataClone(reqCtx, t.Client, t.cluster, t.synthesizeComp, t.runningITS, t.protoITS, backupKey)
	if err != nil {
		return false, err
	}
	if status, err := d.CheckBackupStatus(); err != nil {
		return false, err
	} else if status == backupStatusFailed {
		return true, nil
	}
	desiredPodNames := generatePodNames(t.synthesizeComp)
	currentPodNameSet := sets.New(generatePodNamesByITS(t.runningITS)...)
	for _, podName := range desiredPodNames {
		if _, ok := currentPodNameSet[podName]; ok {
			continue
		}
		// backup's ready, then start to check restore
		templateName, index, err := component.GetTemplateNameAndOrdinal(t.runningITS.Name, podName)
		if err != nil {
			return false, err
		}
		if status, err := d.CheckRestoreStatus(templateName, index); err != nil {
			return false, err
		} else if status == dpv1alpha1.RestorePhaseFailed {
			return true, nil
		}
	}
	return false, nil
}

// hasVolumeExpansionRunning checks if the volume expansion is running.
func (t *componentStatusTransformer) hasVolumeExpansionRunning(transCtx *componentTransformContext) (bool, bool, error) {
	var (
		running bool
		failed  bool
	)
	for _, vct := range t.runningITS.Spec.VolumeClaimTemplates {
		volumes, err := getRunningVolumes(transCtx.Context, t.Client, t.synthesizeComp, t.runningITS, vct.Name)
		if err != nil {
			return false, false, err
		}
		for _, v := range volumes {
			if v.Status.Capacity == nil || v.Status.Capacity.Storage().Cmp(v.Spec.Resources.Requests[corev1.ResourceStorage]) >= 0 {
				continue
			}
			running = true
			// TODO: how to check the expansion failed?
		}
	}
	return running, failed, nil
}

// hasFailedPod checks if the instance set has failed pod.
func (t *componentStatusTransformer) hasFailedPod() (bool, appsv1alpha1.ComponentMessageMap) {
	messages := appsv1alpha1.ComponentMessageMap{}
	// check InstanceFailure condition
	hasFailedPod := meta.IsStatusConditionTrue(t.runningITS.Status.Conditions, string(workloads.InstanceFailure))
	if hasFailedPod {
		failureCondition := meta.FindStatusCondition(t.runningITS.Status.Conditions, string(workloads.InstanceFailure))
		messages.SetObjectMessage(workloads.Kind, t.runningITS.Name, failureCondition.Message)
		return true, messages
	}

	// check InstanceReady condition
	if !meta.IsStatusConditionTrue(t.runningITS.Status.Conditions, string(workloads.InstanceReady)) {
		return false, nil
	}

	// all instances are in Ready condition, check role probe
	if len(t.runningITS.Spec.Roles) == 0 {
		return false, nil
	}
	if len(t.runningITS.Status.MembersStatus) == int(t.runningITS.Status.Replicas) {
		return false, nil
	}
	probeTimeoutDuration := time.Duration(defaultRoleProbeTimeoutAfterPodsReady) * time.Second
	condition := meta.FindStatusCondition(t.runningITS.Status.Conditions, string(workloads.InstanceReady))
	if time.Now().After(condition.LastTransitionTime.Add(probeTimeoutDuration)) {
		messages.SetObjectMessage(workloads.Kind, t.runningITS.Name, "Role probe timeout, check whether the application is available")
		return true, messages
	}

	return false, nil
}

// setComponentStatusPhase sets the component phase and messages conditionally.
func (t *componentStatusTransformer) setComponentStatusPhase(transCtx *componentTransformContext,
	phase appsv1alpha1.ClusterComponentPhase, statusMessage appsv1alpha1.ComponentMessageMap, phaseTransitionMsg string) {
	updateFn := func(status *appsv1alpha1.ComponentStatus) error {
		if status.Phase == phase {
			return nil
		}
		status.Phase = phase
		if status.Message == nil {
			status.Message = statusMessage
		} else {
			for k, v := range statusMessage {
				status.Message[k] = v
			}
		}
		return nil
	}
	if err := t.updateComponentStatus(transCtx, phaseTransitionMsg, updateFn); err != nil {
		panic(fmt.Sprintf("unexpected error occurred while updating component status: %s", err.Error()))
	}
}

// updateComponentStatus updates the component status by @updateFn, with additional message to explain the transition occurred.
func (t *componentStatusTransformer) updateComponentStatus(transCtx *componentTransformContext,
	phaseTransitionMsg string, updateFn func(status *appsv1alpha1.ComponentStatus) error) error {
	if updateFn == nil {
		return nil
	}
	phase := t.comp.Status.Phase
	err := updateFn(&t.comp.Status)
	if err != nil {
		return err
	}
	if phase != t.comp.Status.Phase {
		if transCtx.EventRecorder != nil && phaseTransitionMsg != "" {
			transCtx.EventRecorder.Eventf(t.comp, corev1.EventTypeNormal, componentPhaseTransition, phaseTransitionMsg)
		}
	}
	return nil
}
