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
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubectl/pkg/util/podutils"

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
	var podList []corev1.Pod
	for _, object := range pods {
		pod, _ := object.(*corev1.Pod)
		podList = append(podList, *pod)
	}
	// 2. calculate status summary
	updateRevisions, err := getUpdateRevisions(its.Status.UpdateRevisions)
	if err != nil {
		return nil, err
	}
	replicas := int32(0)
	currentReplicas, updatedReplicas := int32(0), int32(0)
	readyReplicas, availableReplicas := int32(0), int32(0)
	for i := range podList {
		pod := &podList[i]
		if isCreated(pod) {
			replicas++
		}
		if isRunningAndReady(pod) {
			readyReplicas++
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
	its.Status.CurrentGeneration = its.Generation
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

	// 3. set members status
	setMembersStatus(its, &podList)

	return tree, nil
}

func setMembersStatus(its *workloads.InstanceSet, pods *[]corev1.Pod) {
	// no roles defined
	if its.Spec.Roles == nil {
		setMembersStatusWithoutRole(its, pods)
		return
	}
	// compose new status
	newMembersStatus := make([]workloads.MemberStatus, 0)
	roleMap := composeRoleMap(*its)
	for _, pod := range *pods {
		if !intctrlutil.PodIsReadyWithLabel(pod) {
			continue
		}
		readyWithoutPrimary := false
		roleName := GetRoleName(pod)
		role, ok := roleMap[roleName]
		if !ok {
			continue
		}
		if value, ok := pod.Labels[constant.ReadyWithoutPrimaryKey]; ok && value == "true" {
			readyWithoutPrimary = true
		}
		memberStatus := workloads.MemberStatus{
			PodName:             pod.Name,
			ReplicaRole:         &role,
			Ready:               true,
			ReadyWithoutPrimary: readyWithoutPrimary,
		}
		newMembersStatus = append(newMembersStatus, memberStatus)
	}

	// sort and set
	rolePriorityMap := ComposeRolePriorityMap(its.Spec.Roles)
	sortMembersStatus(newMembersStatus, rolePriorityMap)
	its.Status.MembersStatus = newMembersStatus
}

func setMembersStatusWithoutRole(its *workloads.InstanceSet, pods *[]corev1.Pod) {
	var membersStatus []workloads.MemberStatus
	for _, pod := range *pods {
		memberStatus := workloads.MemberStatus{
			PodName: pod.Name,
			Ready:   podutils.IsPodReady(&pod),
		}
		membersStatus = append(membersStatus, memberStatus)
	}
	slices.SortStableFunc(membersStatus, func(a, b workloads.MemberStatus) bool {
		return a.PodName < b.PodName
	})
	its.Status.MembersStatus = membersStatus
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
