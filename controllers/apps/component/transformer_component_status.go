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

package component

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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
	if isCompDeleting(transCtx.ComponentOrig) {
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
	t.comp = transCtx.Component
	t.synthesizeComp = transCtx.SynthesizeComponent
	t.runningITS = transCtx.RunningWorkload.(*workloads.InstanceSet)
	t.protoITS = transCtx.ProtoWorkload.(*workloads.InstanceSet)
	t.dag = dag
}

// reconcileStatus reconciles component status.
func (t *componentStatusTransformer) reconcileStatus(transCtx *componentTransformContext) error {
	if err := t.reconcileStatusCondition(transCtx); err != nil {
		return err
	}
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

	isITSUpdatedNRunning := meta.FindStatusCondition(t.comp.Status.Conditions, appsv1.ConditionTypeWorkloadRunning).Status
	hasFailure := meta.FindStatusCondition(t.comp.Status.Conditions, appsv1.ConditionTypeHasFailure).Status
	isUpdating := meta.FindStatusCondition(t.comp.Status.Conditions, appsv1.ConditionTypeUpdating).Status

	_, podMessages := t.hasFailedPod()

	// check if the component is in creating phase
	isInCreatingPhase := func() bool {
		phase := t.comp.Status.Phase
		return phase == "" || phase == appsv1.CreatingComponentPhase
	}()

	// check if the component is in starting phase
	// TODO: differentiate between starting and updating based on replicas status
	isInStartingPhase := func() bool {
		phase := t.comp.Status.Phase
		return slices.Contains([]appsv1.ComponentPhase{
			appsv1.StoppedComponentPhase, appsv1.StoppingComponentPhase, appsv1.StartingComponentPhase,
		}, phase)
	}()

	switch {
	case isDeleting:
		t.setComponentStatusPhase(transCtx, appsv1.DeletingComponentPhase, nil, "component is Deleting")
	case stopped && (hasRunningPods || !checkPostProvisionDone(transCtx)):
		t.setComponentStatusPhase(transCtx, appsv1.StoppingComponentPhase, nil, "component is Stopping")
	case stopped:
		t.setComponentStatusPhase(transCtx, appsv1.StoppedComponentPhase, nil, "component is Stopped")
	case isITSUpdatedNRunning == metav1.ConditionTrue && isUpdating == metav1.ConditionFalse:
		t.setComponentStatusPhase(transCtx, appsv1.RunningComponentPhase, nil, "component is Running")
	case hasFailure == metav1.ConditionFalse && isInCreatingPhase:
		t.setComponentStatusPhase(transCtx, appsv1.CreatingComponentPhase, nil, "component is Creating")
	case hasFailure == metav1.ConditionFalse && isInStartingPhase:
		t.setComponentStatusPhase(transCtx, appsv1.StartingComponentPhase, nil, "component is Starting")
	case hasFailure == metav1.ConditionFalse:
		t.setComponentStatusPhase(transCtx, appsv1.UpdatingComponentPhase, nil, "component is Updating")
	default:
		t.setComponentStatusPhase(transCtx, appsv1.FailedComponentPhase, podMessages, "component is Failed")
	}

	return nil
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

func (t *componentStatusTransformer) hasVolumeExpansionRunning() bool {
	if t.runningITS == nil {
		return false
	}
	for _, inst := range t.runningITS.Status.InstanceStatus {
		if inst.VolumeExpansion {
			return true
		}
	}
	return false
}

// hasFailedPod checks if the instance set has failed pod.
func (t *componentStatusTransformer) hasFailedPod() (bool, appsv1alpha1.ComponentMessageMap) {
	if t.runningITS == nil {
		return false, nil
	}

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
	if t.runningITS.IsRoleProbeDone() {
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
	err1 := t.reconcileAvailableCondition(transCtx)
	err2 := t.reconcileHasFailureCondition(transCtx)
	err3 := t.reconcileUpdatingCondition(transCtx)
	err4 := t.reconcileWorkloadRunningCondition(transCtx)
	return errors.Join(err1, err2, err3, err4)
}

func (t *componentStatusTransformer) checkNSetCondition(
	eventRecorder record.EventRecorder,
	conditionType string,
	checker func() (status metav1.ConditionStatus, reason, message string, err error),
) error {
	status, reason, message, err := checker()
	if err != nil {
		return err
	}
	cond := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: t.comp.Generation,
		Reason:             reason,
		Message:            message,
	}
	if meta.SetStatusCondition(&t.comp.Status.Conditions, cond) {
		eventRecorder.Event(t.comp, corev1.EventTypeNormal, reason, message)
	}
	return nil
}

func (t *componentStatusTransformer) reconcileHasFailureCondition(transCtx *componentTransformContext) error {
	return t.checkNSetCondition(
		transCtx.EventRecorder,
		appsv1.ConditionTypeHasFailure,
		func() (status metav1.ConditionStatus, reason string, message string, err error) {
			hasFailedPod, messages := t.hasFailedPod()
			if hasFailedPod {
				message = "component has failed pod(s)"
				for _, msg := range messages {
					message += "; " + msg
				}
				return metav1.ConditionTrue, "PodFailure", message, nil
			}

			_, hasFailedScaleOut, err := t.hasScaleOutRunning(transCtx)
			if err != nil {
				return "", "", "", err
			}
			if hasFailedScaleOut {
				return metav1.ConditionTrue, "ScaleOutFailure", "component scale out has failure", nil
			}

			return metav1.ConditionFalse, "NoFailure", "", nil
		},
	)
}

func (t *componentStatusTransformer) reconcileUpdatingCondition(transCtx *componentTransformContext) error {
	return t.checkNSetCondition(
		transCtx.EventRecorder,
		appsv1.ConditionTypeUpdating,
		func() (status metav1.ConditionStatus, reason string, message string, err error) {
			hasRunningScaleOut, _, err := t.hasScaleOutRunning(transCtx)
			if err != nil {
				return "", "", "", err
			}
			if hasRunningScaleOut {
				status = metav1.ConditionTrue
				reason = "ScaleOutRunning"
				message = "component scale out is running"
				return status, reason, message, nil
			}

			hasRunningVolumeExpansion := t.hasVolumeExpansionRunning()
			if hasRunningVolumeExpansion {
				status = metav1.ConditionTrue
				reason = "VolumeExpansionRunning"
				message = "component volume expansion is running"
				return status, reason, message, nil
			}

			return metav1.ConditionFalse, "NotUpdating", "", nil
		},
	)
}

func (t *componentStatusTransformer) reconcileWorkloadRunningCondition(transCtx *componentTransformContext) error {
	return t.checkNSetCondition(
		transCtx.EventRecorder,
		appsv1.ConditionTypeWorkloadRunning,
		func() (status metav1.ConditionStatus, reason string, message string, err error) {
			status = metav1.ConditionTrue
			reason = "WorkloadRunning"
			message = ""

			switch {
			case t.runningITS == nil:
				status = metav1.ConditionFalse
				reason = "WorkloadNotExist"
				message = "waiting for workload to be created"
			case !t.isWorkloadUpdated():
				status = metav1.ConditionFalse
				reason = "WorkloadNotUpdated"
				message = "observed workload's generation not matching component's"
			case !t.runningITS.IsInstanceSetReady():
				status = metav1.ConditionFalse
				reason = "WorkloadNotReady"
				message = "workload not ready"
			}

			return status, reason, message, nil
		},
	)
}

func (t *componentStatusTransformer) reconcileAvailableCondition(transCtx *componentTransformContext) error {
	policy := component.GetComponentAvailablePolicy(transCtx.CompDef)
	if policy.WithPhases == nil && policy.WithRole == nil {
		return nil
	}

	return t.checkNSetCondition(
		transCtx.EventRecorder,
		appsv1.ConditionTypeAvailable,
		func() (status metav1.ConditionStatus, reason string, message string, err error) {
			if policy.WithPhases != nil {
				status, message1 := t.availableWithPhases(transCtx, transCtx.Component, policy)
				if status != metav1.ConditionTrue {
					return status, "PhaseCheckFail", message, nil
				}
				message += message1 + "; "
			}
			if policy.WithRole != nil {
				status, message2 := t.availableWithRole(transCtx, transCtx.Component, policy)
				if status != metav1.ConditionTrue {
					return status, "RoleCheckFail", message, nil
				}
				message += message2 + "; "
			}

			return metav1.ConditionTrue, "Available", message, nil
		},
	)
}

func (t *componentStatusTransformer) availableWithPhases(_ *componentTransformContext,
	comp *appsv1.Component, policy appsv1.ComponentAvailable) (metav1.ConditionStatus, string) {
	if comp.Status.Phase == "" {
		return metav1.ConditionUnknown, "the component phase is unknown"
	}
	phases := sets.New(strings.Split(strings.ToLower(*policy.WithPhases), ",")...)
	if phases.Has(strings.ToLower(string(comp.Status.Phase))) {
		return metav1.ConditionTrue, fmt.Sprintf("the component phase is %s", comp.Status.Phase)
	}
	return metav1.ConditionFalse, fmt.Sprintf("the component phase is %s", comp.Status.Phase)
}

func (t *componentStatusTransformer) availableWithRole(transCtx *componentTransformContext,
	_ *appsv1.Component, policy appsv1.ComponentAvailable) (metav1.ConditionStatus, string) {
	var its *workloads.InstanceSet
	if transCtx.RunningWorkload != nil {
		its = transCtx.RunningWorkload.(*workloads.InstanceSet)
	}
	if its == nil {
		return metav1.ConditionFalse, "the workload is not present"
	}
	for _, inst := range its.Status.InstanceStatus {
		if len(inst.Role) > 0 {
			if strings.EqualFold(inst.Role, *policy.WithRole) {
				return metav1.ConditionTrue, fmt.Sprintf("the role %s is present", *policy.WithRole)
			}
		}
	}
	return metav1.ConditionFalse, fmt.Sprintf("the role %s is not present", *policy.WithRole)
}
