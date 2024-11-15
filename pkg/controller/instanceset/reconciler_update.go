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
	"fmt"
	"time"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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
		return kubebuilderx.ConditionUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	its, _ := tree.GetRoot().(*workloads.InstanceSet)
	if err := validateSpec(its, tree); err != nil {
		return kubebuilderx.CheckResultWithError(err)
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *updateReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its, _ := tree.GetRoot().(*workloads.InstanceSet)
	itsExt, err := buildInstanceSetExt(its, tree)
	if err != nil {
		return kubebuilderx.Continue, err
	}

	// 1. build desired name to template map
	nameToTemplateMap, err := buildInstanceName2TemplateMap(itsExt)
	if err != nil {
		return kubebuilderx.Continue, err
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
		tree.Logger.Info(fmt.Sprintf("InstanceSet %s/%s instances are not aligned", its.Namespace, its.Name))
		return kubebuilderx.Continue, nil
	}

	// 3. do update
	partition, maxUnavailable, err := parsePartitionNMaxUnavailable(its.Spec.UpdateStrategy, len(oldPodList))
	if err != nil {
		return kubebuilderx.Continue, err
	}
	currentUnavailable := 0
	for _, pod := range oldPodList {
		if !isHealthy(pod) {
			currentUnavailable++
		}
	}
	unavailable := maxUnavailable - currentUnavailable

	// if it's a roleful InstanceSet, we use updateCount to represent Pods can be updated according to the spec.memberUpdateStrategy.
	updateCount := len(oldPodList)
	if len(its.Spec.Roles) > 0 {
		itsForPlan := getInstanceSetForUpdatePlan(its)
		plan := NewUpdatePlan(*itsForPlan, oldPodList, IsPodUpdated)
		podsToBeUpdated, err := plan.Execute()
		if err != nil {
			return kubebuilderx.Continue, err
		}
		updateCount = len(podsToBeUpdated)
	}

	updatingPods := 0
	updatedPods := 0
	priorities := ComposeRolePriorityMap(its.Spec.Roles)
	isBlocked := false
	needRetry := false
	sortObjects(oldPodList, priorities, false)
	for _, pod := range oldPodList {
		if updatingPods >= updateCount || updatingPods >= unavailable {
			break
		}
		if updatedPods >= partition {
			break
		}

		if !isImageMatched(pod) {
			tree.Logger.Info(fmt.Sprintf("InstanceSet %s/%s blocks on update as the image(s) of pod %s is not matched", its.Namespace, its.Name, pod.Name))
			break
		}
		if !isContainersReady(pod) {
			tree.Logger.Info(fmt.Sprintf("InstanceSet %s/%s blocks on update as some the container(s) of pod %s are not started at least for %d seconds", its.Namespace, its.Name, pod.Name, its.Spec.MinReadySeconds))
			// as no further event triggers the next reconciliation, we need a retry
			needRetry = true
			break
		}
		if !isHealthy(pod) {
			tree.Logger.Info(fmt.Sprintf("InstanceSet %s/%s blocks on update as the pod %s is not healthy", its.Namespace, its.Name, pod.Name))
			break
		}
		if !isRunningAndAvailable(pod, its.Spec.MinReadySeconds) {
			tree.Logger.Info(fmt.Sprintf("InstanceSet %s/%s blocks on update as the pod %s is not available", its.Namespace, its.Name, pod.Name))
			break
		}
		if !isRoleReady(pod, its.Spec.Roles) {
			tree.Logger.Info(fmt.Sprintf("InstanceSet %s/%s blocks on update as the role of pod %s is not ready", its.Namespace, its.Name, pod.Name))
			break
		}

		// the final update policy will act:
		// PodUpdatePolicy  \   supportedUpdatePolicy
		//                      In-Place		Recreate		NoOps
		//  StrictInPlace       In-Place		X(Blocked)		NoOps
		//  PreferInPlace       In-Place		Recreate		NoOps
		//  Recreate            Recreate		Recreate		NoOps
		supportedUpdatePolicy, err := getPodUpdatePolicy(its, pod)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if its.Spec.PodUpdatePolicy == workloads.StrictInPlacePodUpdatePolicyType && supportedUpdatePolicy == RecreatePolicy {
			message := fmt.Sprintf("InstanceSet %s/%s blocks on update as the PodUpdatePolicy is %s and the pod %s can not inplace update",
				its.Namespace, its.Name, workloads.StrictInPlacePodUpdatePolicyType, pod.Name)
			if tree != nil && tree.EventRecorder != nil {
				tree.EventRecorder.Eventf(its, corev1.EventTypeWarning, EventReasonStrictInPlace, message)
			}
			meta.SetStatusCondition(&its.Status.Conditions, *buildBlockedCondition(its, message))
			isBlocked = true
			break
		}
		if its.Spec.PodUpdatePolicy != workloads.RecreatePodUpdatePolicyType && supportedUpdatePolicy == InPlaceUpdatePolicy {
			newInstance, err := buildInstanceByTemplate(pod.Name, nameToTemplateMap[pod.Name], its, getPodRevision(pod))
			if err != nil {
				return kubebuilderx.Continue, err
			}
			newPod := copyAndMerge(pod, newInstance.pod)
			if err = tree.Update(newPod); err != nil {
				return kubebuilderx.Continue, err
			}
			updatingPods++
		} else if supportedUpdatePolicy != NoOpsPolicy {
			if !isTerminating(pod) {
				if err = tree.Delete(pod); err != nil {
					return kubebuilderx.Continue, err
				}
			}
			updatingPods++
		}
		updatedPods++
	}
	if !isBlocked {
		meta.RemoveStatusCondition(&its.Status.Conditions, string(workloads.InstanceUpdateRestricted))
	}
	if needRetry {
		return kubebuilderx.RetryAfter(2 * time.Second), nil
	}
	return kubebuilderx.Continue, nil
}

func buildBlockedCondition(its *workloads.InstanceSet, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:               string(workloads.InstanceUpdateRestricted),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: its.Generation,
		Reason:             workloads.ReasonInstanceUpdateRestricted,
		Message:            message,
	}
}

func getInstanceSetForUpdatePlan(its *workloads.InstanceSet) *workloads.InstanceSet {
	if its.Spec.UpdateStrategy != nil && its.Spec.UpdateStrategy.MemberUpdateStrategy != nil {
		its.Spec.MemberUpdateStrategy = its.Spec.UpdateStrategy.MemberUpdateStrategy
	}
	if its.Spec.MemberUpdateStrategy != nil {
		return its
	}
	itsForPlan := its.DeepCopy()
	updateStrategy := workloads.SerialUpdateStrategy
	if its.Spec.PodManagementPolicy == apps.ParallelPodManagement {
		updateStrategy = workloads.ParallelUpdateStrategy
	}
	itsForPlan.Spec.MemberUpdateStrategy = &updateStrategy
	return itsForPlan
}

func parsePartitionNMaxUnavailable(rollingUpdate *workloads.InstanceUpdateStrategy, replicas int) (int, int, error) {
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
