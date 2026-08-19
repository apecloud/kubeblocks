/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package instance

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func NewStatusReconciler() kubebuilderx.Reconciler {
	return &statusReconciler{}
}

type statusReconciler struct{}

var _ kubebuilderx.Reconciler = &statusReconciler{}

type instanceStatusObservation struct {
	currentState     workloads.InstanceCurrentState
	currentRevision  string
	upToDate         bool
	ready            bool
	available        bool
	role             string
	volumeExpansion  bool
	configs          []workloads.InstanceConfigStatus
	notReadyName     string
	notAvailableName string
	failureCondition *metav1.Condition
}

func (r *statusReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || !model.IsObjectStatusUpdating(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *statusReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	inst := tree.GetRoot().(*workloads.Instance)

	obj, err := tree.Get(podObj(inst))
	if err != nil {
		return kubebuilderx.Continue, err
	}
	var pod *corev1.Pod
	if obj != nil {
		pod = obj.(*corev1.Pod)
	}
	observation, err := r.observeInstanceStatus(tree, inst, pod)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	r.setInstanceStatus(inst, observation)
	r.setInstanceConditions(inst, observation)

	if observation.currentState == workloads.InstanceCurrentStatePresent && inst.Spec.MinReadySeconds > 0 && !observation.available {
		return kubebuilderx.RetryAfter(time.Second), nil
	}
	return kubebuilderx.Continue, nil
}

func (r *statusReconciler) observeInstanceStatus(tree *kubebuilderx.ObjectTree, inst *workloads.Instance, pod *corev1.Pod) (instanceStatusObservation, error) {
	observation := instanceStatusObservation{
		currentState:     workloads.InstanceCurrentStateAbsent,
		notReadyName:     inst.Name,
		notAvailableName: inst.Name,
	}
	if pod == nil {
		return observation, nil
	}
	if isTerminating(pod) {
		observation.currentState = workloads.InstanceCurrentStateTerminating
		observation.currentRevision = getPodRevision(pod)
		observation.notReadyName = pod.Name
		observation.notAvailableName = pod.Name
		return observation, nil
	}

	observation.currentState = workloads.InstanceCurrentStatePresent
	observation.notReadyName = ""
	observation.notAvailableName = ""

	if isCreated(pod) {
		observation.notReadyName = pod.Name
	}
	if isImageMatched(pod) && intctrlutil.IsPodReady(pod) {
		observation.ready = true
		observation.notReadyName = ""
		if intctrlutil.IsPodAvailable(pod, inst.Spec.MinReadySeconds) {
			observation.available = true
		} else {
			observation.notAvailableName = pod.Name
		}
	}
	if isCreated(pod) && !isTerminating(pod) {
		updated, err := isPodUpdated(inst, pod)
		if err != nil {
			return instanceStatusObservation{}, err
		}
		observation.upToDate = updated
	}
	observation.currentRevision = getPodRevision(pod)
	if observation.upToDate {
		observation.currentRevision = inst.Status.UpdateRevision
	}
	observation.failureCondition = r.buildFailureCondition(inst, pod)
	observation.role = r.observedRoleOfPod(inst, pod)
	observation.volumeExpansion = r.hasRunningVolumeExpansion(tree, inst)
	configs, err := r.observedConfigsOfPod(pod)
	if err != nil {
		return instanceStatusObservation{}, err
	}
	observation.configs = configs
	return observation, nil
}

// setInstanceStatus is the only writer for status derived from the current Pod observation.
// Desired revision and observed generation remain owned by the revision reconciler.
func (r *statusReconciler) setInstanceStatus(inst *workloads.Instance, observation instanceStatusObservation) {
	inst.Status.CurrentState = observation.currentState
	inst.Status.CurrentRevision = observation.currentRevision
	inst.Status.UpToDate = observation.upToDate
	inst.Status.Ready = observation.ready
	inst.Status.Available = observation.available
	inst.Status.Role = observation.role
	inst.Status.VolumeExpansion = observation.volumeExpansion
	inst.Status.Configs = observation.configs
}

// setInstanceConditions is the only writer for observation-derived Ready, Available, and Failure conditions.
// UpdateRestricted remains owned by the update reconciler.
func (r *statusReconciler) setInstanceConditions(inst *workloads.Instance, observation instanceStatusObservation) {
	meta.SetStatusCondition(&inst.Status.Conditions, *r.buildReadyCondition(inst, observation.ready, observation.notReadyName))
	meta.SetStatusCondition(&inst.Status.Conditions, *r.buildAvailableCondition(inst, observation.available, observation.notAvailableName))
	if observation.failureCondition != nil {
		meta.SetStatusCondition(&inst.Status.Conditions, *observation.failureCondition)
	} else {
		meta.RemoveStatusCondition(&inst.Status.Conditions, string(workloads.InstanceFailure))
	}
}

func (r *statusReconciler) buildReadyCondition(inst *workloads.Instance, ready bool, notReadyName string) *metav1.Condition {
	condition := &metav1.Condition{
		Type:               string(workloads.InstanceReady),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inst.Generation,
		Reason:             workloads.ReasonReady,
	}
	if !ready {
		condition.Status = metav1.ConditionFalse
		condition.Reason = workloads.ReasonNotReady
		condition.Message = notReadyName
	}
	return condition
}

func (r *statusReconciler) buildAvailableCondition(inst *workloads.Instance, available bool, notAvailableName string) *metav1.Condition {
	condition := &metav1.Condition{
		Type:               string(workloads.InstanceAvailable),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inst.Generation,
		Reason:             workloads.ReasonAvailable,
	}
	if !available {
		condition.Status = metav1.ConditionFalse
		condition.Reason = workloads.ReasonNotAvailable
		condition.Message = notAvailableName
	}
	return condition
}

func (r *statusReconciler) buildFailureCondition(inst *workloads.Instance, pod *corev1.Pod) *metav1.Condition {
	if isTerminating(pod) {
		return nil
	}
	var failureName string
	// Kubernetes says the Pod is 'Failed'
	if pod.Status.Phase == corev1.PodFailed {
		failureName = pod.Name
	}
	// KubeBlocks says the Pod is 'Failed'
	isFailed, isTimedOut, _ := intctrlutil.IsPodFailedAndTimedOut(pod)
	if len(failureName) == 0 && isFailed && isTimedOut {
		failureName = pod.Name
	}
	if len(failureName) == 0 {
		return nil
	}
	return &metav1.Condition{
		Type:               string(workloads.InstanceFailure),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inst.Generation,
		Reason:             workloads.ReasonInstanceFailure,
		Message:            failureName,
	}
}

func (r *statusReconciler) observedRoleOfPod(inst *workloads.Instance, pod *corev1.Pod) string {
	if inst.Spec.Roles != nil && intctrlutil.PodIsReadyWithLabel(*pod) {
		roleMap := composeRoleMap(inst)
		roleName := getRoleName(pod)
		role, ok := roleMap[roleName]
		if ok {
			return role.Name
		}
	}
	return ""
}

func (r *statusReconciler) hasRunningVolumeExpansion(tree *kubebuilderx.ObjectTree, inst *workloads.Instance) bool {
	pvcs := tree.List(&corev1.PersistentVolumeClaim{})
	var pvcList []*corev1.PersistentVolumeClaim
	for _, obj := range pvcs {
		pvc, _ := obj.(*corev1.PersistentVolumeClaim)
		pvcList = append(pvcList, pvc)
	}
	for _, vct := range inst.Spec.VolumeClaimTemplates {
		prefix := fmt.Sprintf("%s-%s", vct.Name, inst.Name)
		for _, pvc := range pvcList {
			if !strings.HasPrefix(pvc.Name, prefix) {
				continue
			}
			if pvc.Status.Capacity == nil || pvc.Status.Capacity.Storage().Cmp(pvc.Spec.Resources.Requests[corev1.ResourceStorage]) >= 0 {
				continue
			}
			instName := ""
			if pvc.Labels != nil {
				instName = pvc.Labels[constant.KBAppPodNameLabelKey]
			}
			if len(instName) > 0 {
				return true
			}
		}
	}
	return false
}

func (r *statusReconciler) observedConfigsOfPod(pod *corev1.Pod) ([]workloads.InstanceConfigStatus, error) {
	configs, err := configsFromPod(pod)
	if err != nil {
		return nil, err
	}
	if len(configs) == 0 {
		return nil, nil
	}
	status := make([]workloads.InstanceConfigStatus, 0, len(configs))
	for _, config := range configs {
		status = append(status, workloads.InstanceConfigStatus{
			Name:       config.Name,
			ConfigHash: config.ConfigHash,
		})
	}
	return status, nil
}
