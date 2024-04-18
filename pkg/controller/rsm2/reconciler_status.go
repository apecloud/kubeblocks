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

package rsm2

import (
	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	rsm1 "github.com/apecloud/kubeblocks/pkg/controller/rsm"
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
	rsm, _ := tree.GetRoot().(*workloads.InstanceSet)
	// 1. get all pods
	pods := tree.List(&corev1.Pod{})
	var podList []corev1.Pod
	for _, object := range pods {
		pod, _ := object.(*corev1.Pod)
		podList = append(podList, *pod)
	}
	// 2. calculate status summary
	updateRevisions, err := getUpdateRevisions(rsm.Status.UpdateRevisions)
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
			if isRunningAndAvailable(pod, rsm.Spec.MinReadySeconds) {
				availableReplicas++
			}
		}
		if isCreated(pod) && !isTerminating(pod) {
			isPodUpdated, err := IsPodUpdated(rsm, pod)
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
	rsm.Status.Replicas = replicas
	rsm.Status.ReadyReplicas = readyReplicas
	rsm.Status.AvailableReplicas = availableReplicas
	rsm.Status.CurrentReplicas = currentReplicas
	rsm.Status.UpdatedReplicas = updatedReplicas
	rsm.Status.CurrentGeneration = rsm.Generation
	// all pods have been updated
	totalReplicas := int32(1)
	if rsm.Spec.Replicas != nil {
		totalReplicas = *rsm.Spec.Replicas
	}
	if rsm.Status.Replicas == totalReplicas && rsm.Status.UpdatedReplicas == totalReplicas {
		rsm.Status.CurrentRevisions = rsm.Status.UpdateRevisions
		rsm.Status.CurrentRevision = rsm.Status.UpdateRevision
		rsm.Status.CurrentReplicas = totalReplicas
	}

	// 3. set members status
	rsm1.SetMembersStatus(rsm, &podList)

	return tree, nil
}
