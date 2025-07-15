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

package instance

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// statusReconciler computes the current status
type statusReconciler struct{}

var _ kubebuilderx.Reconciler = &statusReconciler{}

func NewStatusReconciler() kubebuilderx.Reconciler {
	return &statusReconciler{}
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
	if obj == nil {
		return kubebuilderx.Continue, nil
	}
	pod := obj.(*corev1.Pod)

	ready, available, updated := false, false, false
	notReadyName, notAvailableName := "", ""

	// podToNodeMapping, err := ParseNodeSelectorOnceAnnotation(inst)
	// if err != nil {
	//	return kubebuilderx.Continue, err
	// }

	if isCreated(pod) {
		notReadyName = pod.Name
	}
	if isImageMatched(pod) && intctrlutil.IsPodReady(pod) {
		ready = true
		notReadyName = ""
		if intctrlutil.IsPodAvailable(pod, inst.Spec.MinReadySeconds) {
			available = true
		} else {
			notAvailableName = pod.Name
		}
	}
	if isCreated(pod) && !isTerminating(pod) {
		updated, err = IsPodUpdated(inst, pod)
		if err != nil {
			return kubebuilderx.Continue, err
		}
	}

	// TODO: ???
	// if nodeName, ok := podToNodeMapping[pod.Name]; ok {
	//	// there's chance that a pod is currently running and wait to be deleted so that it can be rescheduled
	//	if pod.Spec.NodeName == nodeName {
	//		if err := deleteNodeSelectorOnceAnnotation(its, pod.Name); err != nil {
	//			return kubebuilderx.Continue, err
	//		}
	//	}
	// }

	inst.Status.CurrentRevision = getPodRevision(pod)
	if updated {
		inst.Status.CurrentRevision = inst.Status.UpdateRevision
	}

	readyCondition := buildReadyCondition(inst, ready, notReadyName)
	meta.SetStatusCondition(&inst.Status.Conditions, *readyCondition)

	availableCondition := buildAvailableCondition(inst, available, notAvailableName)
	meta.SetStatusCondition(&inst.Status.Conditions, *availableCondition)

	failureCondition := buildFailureCondition(inst, pod)
	if failureCondition != nil {
		meta.SetStatusCondition(&inst.Status.Conditions, *failureCondition)
	} else {
		meta.RemoveStatusCondition(&inst.Status.Conditions, string(workloads.InstanceFailure))
	}

	// 4. set members status
	setMembersStatus(inst, pod)

	// TODO: 5. set instance status
	// setInstanceStatus(inst, podList)

	if inst.Spec.MinReadySeconds > 0 && !available {
		return kubebuilderx.RetryAfter(time.Second), nil
	}
	return kubebuilderx.Continue, nil
}

func buildReadyCondition(inst *workloads.Instance, ready bool, notReadyName string) *metav1.Condition {
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

func buildAvailableCondition(inst *workloads.Instance, available bool, notAvailableName string) *metav1.Condition {
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

func buildFailureCondition(inst *workloads.Instance, pod *corev1.Pod) *metav1.Condition {
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

func setMembersStatus(inst *workloads.Instance, pod *corev1.Pod) {
	// reset it first
	inst.Status.Role = nil

	// no roles defined
	if inst.Spec.Roles == nil {
		return
	}

	// compose new status
	inst.Status.Role = ptr.To("")
	if intctrlutil.PodIsReadyWithLabel(*pod) {
		roleMap := composeRoleMap(inst)
		roleName := getRoleName(pod)
		role, ok := roleMap[roleName]
		if ok {
			inst.Status.Role = ptr.To(role.Name)
		}
	}
}

func setInstanceStatus(its *workloads.InstanceSet, pods []*corev1.Pod) {
	// compose new instance status
	newInstanceStatus := make([]workloads.InstanceStatus, 0)
	for _, pod := range pods {
		instanceStatus := workloads.InstanceStatus{
			PodName: pod.Name,
		}
		newInstanceStatus = append(newInstanceStatus, instanceStatus)
	}

	syncInstanceConfigStatus(its, newInstanceStatus)

	// sortInstanceStatus(newInstanceStatus)
	its.Status.InstanceStatus = newInstanceStatus
}

func syncInstanceConfigStatus(its *workloads.InstanceSet, instanceStatus []workloads.InstanceStatus) {
	if its.Status.InstanceStatus == nil {
		// initialize
		configs := make([]workloads.InstanceConfigStatus, 0)
		for _, config := range its.Spec.Configs {
			configs = append(configs, workloads.InstanceConfigStatus{
				Name:       config.Name,
				Generation: config.Generation,
			})
		}
		for i := range instanceStatus {
			instanceStatus[i].Configs = configs
		}
	} else {
		// HACK: copy the existing config status from the current its.status.instanceStatus
		configs := sets.New[string]()
		for _, config := range its.Spec.Configs {
			configs.Insert(config.Name)
		}
		for i, newStatus := range instanceStatus {
			for _, status := range its.Status.InstanceStatus {
				if status.PodName == newStatus.PodName {
					if instanceStatus[i].Configs == nil {
						instanceStatus[i].Configs = make([]workloads.InstanceConfigStatus, 0)
					}
					for j, config := range status.Configs {
						if configs.Has(config.Name) {
							instanceStatus[i].Configs = append(instanceStatus[i].Configs, status.Configs[j])
						}
					}
					break
				}
			}
		}
	}
}

// func sortInstanceStatus(instanceStatus []workloads.InstanceStatus) {
//	getNameNOrdinalFunc := func(i int) (string, int) {
//		return parseParentNameAndOrdinal(instanceStatus[i].PodName)
//	}
//	baseSort(instanceStatus, getNameNOrdinalFunc, nil, true)
// }
