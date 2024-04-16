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
	"fmt"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	rsm1 "github.com/apecloud/kubeblocks/pkg/controller/rsm"
)

// updateReconciler handles the updates of instances based on the UpdateStrategy.
// Currently, two update strategies are supported: 'OnDelete' and 'RollingUpdate'.
type updateReconciler struct{}

var _ kubebuilderx.Reconciler = &updateReconciler{}

func NewUpdateReconciler() kubebuilderx.Reconciler {
	return &updateReconciler{}
}

func (r *updateReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ResultUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ResultUnsatisfied
	}
	rsm, _ := tree.GetRoot().(*workloads.ReplicatedStateMachine)
	if err := validateSpec(rsm, tree); err != nil {
		return kubebuilderx.CheckResultWithError(err)
	}
	return kubebuilderx.ResultSatisfied
}

func (r *updateReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
	rsm, _ := tree.GetRoot().(*workloads.ReplicatedStateMachine)
	rsmExt, err := buildRSMExt(rsm, tree)
	if err != nil {
		return nil, err
	}

	// 1. build desired name to template map
	nameToTemplateMap, err := buildInstanceName2TemplateMap(rsmExt)
	if err != nil {
		return nil, err
	}

	// 2. validate the update set
	newNameSet := sets.NewString()
	for name := range nameToTemplateMap {
		newNameSet.Insert(name)
	}
	oldNameSet := sets.NewString()
	oldInstanceMap := make(map[string]*corev1.Pod)
	var oldPodList []*corev1.Pod
	for _, object := range tree.List(&corev1.Pod{}) {
		oldNameSet.Insert(object.GetName())
		pod, _ := object.(*corev1.Pod)
		oldInstanceMap[object.GetName()] = pod
		oldPodList = append(oldPodList, pod)
	}
	updateNameSet := oldNameSet.Intersection(newNameSet)
	if len(updateNameSet) != len(oldNameSet) || len(updateNameSet) != len(newNameSet) {
		tree.Logger.Info(fmt.Sprintf("RSM %s/%s instances are not aligned", rsm.Namespace, rsm.Name))
		return tree, nil
	}

	// 3. do update
	// do nothing if UpdateStrategyType is 'OnDelete'
	if rsm.Spec.UpdateStrategy.Type == apps.OnDeleteStatefulSetStrategyType {
		return tree, nil
	}

	// handle 'RollingUpdate'
	partition, maxUnavailable, err := parsePartitionNMaxUnavailable(rsm.Spec.UpdateStrategy.RollingUpdate, len(oldPodList))
	if err != nil {
		return nil, err
	}
	currentUnavailable := 0
	for _, pod := range oldPodList {
		if !isHealthy(pod) {
			currentUnavailable++
		}
	}
	unavailable := maxUnavailable - currentUnavailable

	// TODO(free6om): compute updateCount from PodManagementPolicy(Serial/OrderedReady, Parallel, BestEffortParallel).
	// align MemberUpdateStrategy with PodManagementPolicy if it has nil value.
	rsmForPlan := getRSMForUpdatePlan(rsm)
	plan := rsm1.NewUpdatePlan(*rsmForPlan, oldPodList, IsPodUpdated)
	podsToBeUpdated, err := plan.Execute()
	if err != nil {
		return nil, err
	}
	updateCount := len(podsToBeUpdated)

	updatingPods := 0
	updatedPods := 0
	priorities := rsm1.ComposeRolePriorityMap(rsm.Spec.Roles)
	sortObjects(oldPodList, priorities, true)
	for _, pod := range oldPodList {
		if updatingPods >= updateCount || updatingPods >= unavailable {
			break
		}
		if updatedPods >= partition {
			break
		}

		if !isHealthy(pod) {
			tree.Logger.Info(fmt.Sprintf("RSM %s/%s blocks on scale-in as the pod %s is not healthy", rsm.Namespace, rsm.Name, pod.Name))
			break
		}
		if err != nil {
			return nil, err
		}

		updatePolicy, err := getPodUpdatePolicy(rsm, pod)
		if err != nil {
			return nil, err
		}
		if updatePolicy == InPlaceUpdatePolicy {
			newInstance, err := buildInstanceByTemplate(pod.Name, nameToTemplateMap[pod.Name], rsm, getPodRevision(pod))
			if err != nil {
				return nil, err
			}
			newPod := copyAndMerge(pod, newInstance.pod)
			if err = tree.Update(newPod); err != nil {
				return nil, err
			}
			updatingPods++
		} else if updatePolicy == RecreatePolicy {
			if !isTerminating(pod) {
				if err = tree.Delete(pod); err != nil {
					return nil, err
				}
			}
			updatingPods++
		}
		updatedPods++
	}
	return tree, nil
}

func getRSMForUpdatePlan(rsm *workloads.ReplicatedStateMachine) *workloads.ReplicatedStateMachine {
	if rsm.Spec.MemberUpdateStrategy != nil {
		return rsm
	}
	rsmForPlan := rsm.DeepCopy()
	updateStrategy := workloads.SerialUpdateStrategy
	if rsm.Spec.PodManagementPolicy == apps.ParallelPodManagement {
		updateStrategy = workloads.ParallelUpdateStrategy
	}
	rsmForPlan.Spec.MemberUpdateStrategy = &updateStrategy
	return rsmForPlan
}

func parsePartitionNMaxUnavailable(rollingUpdate *apps.RollingUpdateStatefulSetStrategy, replicas int) (int, int, error) {
	partition := replicas
	maxUnavailable := 1
	if rollingUpdate == nil {
		return partition, maxUnavailable, nil
	}
	if rollingUpdate.Partition != nil {
		partition = int(*rollingUpdate.Partition)
	}
	if rollingUpdate.MaxUnavailable != nil {
		maxUnavailableNum, err := intstr.GetScaledValueFromIntOrPercent(intstr.ValueOrDefault(rollingUpdate.MaxUnavailable, intstr.FromInt32(1)), replicas, false)
		if err != nil {
			return 0, 0, err
		}
		// maxUnavailable might be zero for small percentage with round down.
		// So we have to enforce it not to be less than 1.
		if maxUnavailableNum < 1 {
			maxUnavailableNum = 1
		}
		maxUnavailable = maxUnavailableNum
	}
	return partition, maxUnavailable, nil
}
