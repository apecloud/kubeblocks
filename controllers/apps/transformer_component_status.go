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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
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

	cluster        *appsv1.Cluster
	comp           *appsv1.Component
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

	workloadGeneration, err := t.workloadGeneration()
	if err != nil {
		return err
	}
	if workloadGeneration == nil || *workloadGeneration >= comp.Status.ObservedGeneration {
		if err = t.reconcileStatus(transCtx); err != nil {
			return err
		}
		if workloadGeneration != nil {
			comp.Status.ObservedGeneration = *workloadGeneration
		}
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	if v := graphCli.FindMatchedVertex(dag, comp); v == nil {
		graphCli.Status(dag, transCtx.ComponentOrig, comp)
	}
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
		return t.reconcileStatusCondition(transCtx)
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
	hasRunningScaleOut, hasFailedScaleOut, err := t.hasScaleOutRunning(transCtx)
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
		return hasFailedPod || hasFailedScaleOut || hasFailedVolumeExpansion
	}()

	// check if the component is in creating phase
	isInCreatingPhase := func() bool {
		phase := t.comp.Status.Phase
		return phase == "" || phase == appsv1.CreatingComponentPhase
	}()

	transCtx.Logger.Info(
		fmt.Sprintf("status conditions, creating: %v, its running: %v, has failure: %v, updating: %v, config synced: %v",
			isInCreatingPhase, isITSUpdatedNRunning, hasFailure, hasRunningScaleOut || hasRunningVolumeExpansion, isAllConfigSynced))

	switch {
	case isDeleting:
		t.setComponentStatusPhase(transCtx, appsv1.DeletingComponentPhase, nil, "component is Deleting")
	case stopped && hasRunningPods:
		t.setComponentStatusPhase(transCtx, appsv1.StoppingComponentPhase, nil, "component is Stopping")
	case stopped:
		t.setComponentStatusPhase(transCtx, appsv1.StoppedComponentPhase, nil, "component is Stopped")
	case isITSUpdatedNRunning && isAllConfigSynced && !hasRunningScaleOut && !hasRunningVolumeExpansion:
		t.setComponentStatusPhase(transCtx, appsv1.RunningComponentPhase, nil, "component is Running")
	case !hasFailure && isInCreatingPhase:
		t.setComponentStatusPhase(transCtx, appsv1.CreatingComponentPhase, nil, "component is Creating")
	case !hasFailure:
		t.setComponentStatusPhase(transCtx, appsv1.UpdatingComponentPhase, nil, "component is Updating")
	default:
		t.setComponentStatusPhase(transCtx, appsv1.FailedComponentPhase, messages, "component is Failed")
	}

	return t.reconcileStatusCondition(transCtx)
}

func (t *componentStatusTransformer) workloadGeneration() (*int64, error) {
	if t.runningITS == nil {
		return nil, nil
	}
	generation, ok := t.runningITS.GetAnnotations()[constant.KubeBlocksGenerationKey]
	if !ok {
		return nil, nil
	}
	val, err := strconv.ParseInt(generation, 10, 64)
	if err != nil {
		return nil, err
	}
	return &val, nil
}

func (t *componentStatusTransformer) isWorkloadUpdated() bool {
	if t.comp == nil || t.runningITS == nil {
		return false
	}
	generation := t.runningITS.GetAnnotations()[constant.KubeBlocksGenerationKey]
	return generation == strconv.FormatInt(t.comp.Generation, 10)
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

// hasScaleOutRunning checks if the scale out is running.
func (t *componentStatusTransformer) hasScaleOutRunning(transCtx *componentTransformContext) (running bool, failed bool, err error) {
	if t.runningITS == nil || t.runningITS.Spec.Replicas == nil {
		return false, false, nil
	}

	replicas, err := component.GetReplicasStatusFunc(t.protoITS, func(status component.ReplicaStatus) bool {
		return status.DataLoaded != nil && !*status.DataLoaded ||
			status.MemberJoined != nil && !*status.MemberJoined
	})
	if err != nil {
		return false, false, err
	}
	if len(replicas) == 0 {
		return false, false, nil
	}

	// TODO: scale-out failed

	return true, false, nil
}

// hasVolumeExpansionRunning checks if the volume expansion is running.
func (t *componentStatusTransformer) hasVolumeExpansionRunning(transCtx *componentTransformContext) (running bool, failed bool, err error) {
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
		messages.SetObjectMessage(workloads.InstanceSetKind, t.runningITS.Name, failureCondition.Message)
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
		messages.SetObjectMessage(workloads.InstanceSetKind, t.runningITS.Name, "Role probe timeout, check whether the application is available")
		return true, messages
	}

	return false, nil
}

// setComponentStatusPhase sets the component phase and messages conditionally.
func (t *componentStatusTransformer) setComponentStatusPhase(transCtx *componentTransformContext,
	phase appsv1.ComponentPhase, statusMessage map[string]string, phaseTransitionMsg string) {
	updateFn := func(status *appsv1.ComponentStatus) error {
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
	phaseTransitionMsg string, updateFn func(status *appsv1.ComponentStatus) error) error {
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

func (t *componentStatusTransformer) reconcileStatusCondition(transCtx *componentTransformContext) error {
	return t.reconcileAvailableCondition(transCtx)
}

func (t *componentStatusTransformer) reconcileAvailableCondition(transCtx *componentTransformContext) error {
	policy := component.GetComponentAvailablePolicy(transCtx.CompDef)
	if policy.WithPhases == nil {
		return nil
	}

	var (
		comp = transCtx.Component
	)
	status, reason, message := func() (metav1.ConditionStatus, string, string) {
		if comp.Status.Phase == "" {
			return metav1.ConditionUnknown, "Unknown", "the component phase is unknown"
		}
		phases := sets.New[string](strings.Split(strings.ToLower(*policy.WithPhases), ",")...)
		if phases.Has(strings.ToLower(string(comp.Status.Phase))) {
			return metav1.ConditionTrue, "Available", fmt.Sprintf("the component phase is %s", comp.Status.Phase)
		}
		return metav1.ConditionFalse, "Unavailable", fmt.Sprintf("the component phase is %s", comp.Status.Phase)
	}()

	cond := metav1.Condition{
		Type:               appsv1.ConditionTypeAvailable,
		Status:             status,
		ObservedGeneration: comp.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
	if meta.SetStatusCondition(&comp.Status.Conditions, cond) {
		transCtx.EventRecorder.Event(comp, corev1.EventTypeNormal, reason, message)
	}

	return nil
}
