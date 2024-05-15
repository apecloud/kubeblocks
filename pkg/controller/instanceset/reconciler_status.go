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

package instanceset

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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
		return kubebuilderx.ResultUnsatisfied
	}
	return kubebuilderx.ResultSatisfied
}

func (r *statusReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
	its, _ := tree.GetRoot().(*workloads.InstanceSet)
	// 1. get all pods
	pods := tree.List(&corev1.Pod{})
	var podList []*corev1.Pod
	for _, object := range pods {
		pod, _ := object.(*corev1.Pod)
		podList = append(podList, pod)
	}
	// 2. calculate status summary
	updateRevisions, err := getUpdateRevisions(its.Status.UpdateRevisions)
	if err != nil {
		return nil, err
	}
	replicas := int32(0)
	currentReplicas, updatedReplicas := int32(0), int32(0)
	readyReplicas, availableReplicas := int32(0), int32(0)
	notReadyNames := sets.New[string]()
	for _, pod := range podList {
		if isCreated(pod) {
			notReadyNames.Insert(pod.Name)
			replicas++
		}
		if isRunningAndReady(pod) && !isTerminating(pod) {
			readyReplicas++
			notReadyNames.Delete(pod.Name)
			if isRunningAndAvailable(pod, its.Spec.MinReadySeconds) {
				availableReplicas++
			}
		}
		if isCreated(pod) && !isTerminating(pod) {
			isPodUpdated, err := IsPodUpdated(its, pod)
			if err != nil {
				return nil, err
			}
			switch _, ok := updateRevisions[pod.Name]; {
			case !ok, !isPodUpdated:
				currentReplicas++
			default:
				updatedReplicas++
			}
		}
	}
	its.Status.Replicas = replicas
	its.Status.ReadyReplicas = readyReplicas
	its.Status.AvailableReplicas = availableReplicas
	its.Status.CurrentReplicas = currentReplicas
	its.Status.UpdatedReplicas = updatedReplicas
	// all pods have been updated
	totalReplicas := int32(1)
	if its.Spec.Replicas != nil {
		totalReplicas = *its.Spec.Replicas
	}
	if its.Status.Replicas == totalReplicas && its.Status.UpdatedReplicas == totalReplicas {
		its.Status.CurrentRevisions = its.Status.UpdateRevisions
		its.Status.CurrentRevision = its.Status.UpdateRevision
		its.Status.CurrentReplicas = totalReplicas
	}
	readyCondition, err := buildReadyCondition(its, readyReplicas >= replicas, notReadyNames)
	if err != nil {
		return nil, err
	}
	meta.SetStatusCondition(&its.Status.Conditions, *readyCondition)

	// 3. set InstanceFailure condition
	failureCondition, err := buildFailureCondition(its, podList)
	if err != nil {
		return nil, err
	}
	if failureCondition != nil {
		meta.SetStatusCondition(&its.Status.Conditions, *failureCondition)
	} else {
		meta.RemoveStatusCondition(&its.Status.Conditions, string(workloads.InstanceFailure))
	}

	// 4. set members status
	setMembersStatus(its, podList)

	// 5. set readyWithoutPrimary
	// TODO(free6om): should put this field to the spec
	setReadyWithPrimary(its, podList)

	return tree, nil
}

func buildReadyCondition(its *workloads.InstanceSet, ready bool, notReadyNames sets.Set[string]) (*metav1.Condition, error) {
	condition := &metav1.Condition{
		Type:               string(workloads.InstanceReady),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: its.Generation,
		Reason:             workloads.ReasonReady,
	}
	if !ready {
		condition.Status = metav1.ConditionFalse
		condition.Reason = workloads.ReasonNotReady
		names := notReadyNames.UnsortedList()
		baseSort(names, func(i int) (string, int) {
			return ParseParentNameAndOrdinal(names[i])
		}, nil, true)
		message, err := json.Marshal(names)
		if err != nil {
			return nil, err
		}
		condition.Message = string(message)
	}
	return condition, nil
}

func buildFailureCondition(its *workloads.InstanceSet, pods []*corev1.Pod) (*metav1.Condition, error) {
	var failureNames []string
	for _, pod := range pods {
		if isTerminating(pod) {
			continue
		}
		// Kubernetes says the Pod is 'Failed'
		if pod.Status.Phase == corev1.PodFailed {
			failureNames = append(failureNames, pod.Name)
			continue
		}
		// KubeBlocks says the Pod is 'Failed'
		isFailed, isTimedOut, _ := intctrlutil.IsPodFailedAndTimedOut(pod)
		if isFailed || isTimedOut {
			failureNames = append(failureNames, pod.Name)
		}
	}
	if len(failureNames) == 0 {
		return nil, nil
	}
	baseSort(failureNames, func(i int) (string, int) {
		return ParseParentNameAndOrdinal(failureNames[i])
	}, nil, true)
	message, err := json.Marshal(failureNames)
	if err != nil {
		return nil, err
	}
	return &metav1.Condition{
		Type:               string(workloads.InstanceFailure),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: its.Generation,
		Reason:             workloads.ReasonInstanceFailure,
		Message:            string(message),
	}, nil
}

func setReadyWithPrimary(its *workloads.InstanceSet, pods []*corev1.Pod) {
	readyWithoutPrimary := false
	for _, pod := range pods {
		if value, ok := pod.Labels[constant.ReadyWithoutPrimaryKey]; ok && value == "true" {
			readyWithoutPrimary = true
			break
		}
	}
	its.Status.ReadyWithoutPrimary = readyWithoutPrimary
}

func setMembersStatus(its *workloads.InstanceSet, pods []*corev1.Pod) {
	// no roles defined
	if its.Spec.Roles == nil {
		return
	}
	// compose new status
	newMembersStatus := make([]workloads.MemberStatus, 0)
	roleMap := composeRoleMap(*its)
	for _, pod := range pods {
		if !intctrlutil.PodIsReadyWithLabel(*pod) {
			continue
		}
		roleName := getRoleName(pod)
		role, ok := roleMap[roleName]
		if !ok {
			continue
		}
		memberStatus := workloads.MemberStatus{
			PodName:     pod.Name,
			ReplicaRole: &role,
		}
		newMembersStatus = append(newMembersStatus, memberStatus)
	}

	// sort and set
	rolePriorityMap := ComposeRolePriorityMap(its.Spec.Roles)
	sortMembersStatus(newMembersStatus, rolePriorityMap)
	its.Status.MembersStatus = newMembersStatus
}

func sortMembersStatus(membersStatus []workloads.MemberStatus, rolePriorityMap map[string]int) {
	getRolePriorityFunc := func(i int) int {
		role := membersStatus[i].ReplicaRole.Name
		return rolePriorityMap[role]
	}
	getNameNOrdinalFunc := func(i int) (string, int) {
		return ParseParentNameAndOrdinal(membersStatus[i].PodName)
	}
	baseSort(membersStatus, getNameNOrdinalFunc, getRolePriorityFunc, true)
}
