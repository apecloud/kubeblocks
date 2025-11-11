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

package instanceset

import (
	"errors"
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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
	return kubebuilderx.ConditionSatisfied
}

func (r *updateReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its, _ := tree.GetRoot().(*workloads.InstanceSet)
	itsExt, err := instancetemplate.BuildInstanceSetExt(its, tree)
	if err != nil {
		return kubebuilderx.Continue, err
	}

	// 1. build desired name to template map
	nameBuilder, err := instancetemplate.NewPodNameBuilder(itsExt, nil)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	nameToTemplateMap, err := nameBuilder.BuildInstanceName2TemplateMap()
	if err != nil {
		return kubebuilderx.Continue, err
	}

	// 2. validate the update set
	newNameSet := sets.New[string]()
	for name := range nameToTemplateMap {
		newNameSet.Insert(name)
	}
	oldNameSet := sets.New[string]()
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
	// do nothing if update strategy type is 'OnDelete'
	if its.Spec.InstanceUpdateStrategy != nil && its.Spec.InstanceUpdateStrategy.Type == kbappsv1.OnDeleteStrategyType {
		return kubebuilderx.Continue, nil
	}

	// handle 'RollingUpdate'
	rollingUpdateQuota, unavailableQuota, err := r.rollingUpdateQuota(its, oldPodList)
	if err != nil {
		return kubebuilderx.Continue, err
	}

	// handle 'MemberUpdate'
	memberUpdateQuota, err := r.memberUpdateQuota(its, oldPodList)
	if err != nil {
		return kubebuilderx.Continue, err
	}

	priorities := ComposeRolePriorityMap(its.Spec.Roles)
	sortObjects(oldPodList, priorities, false)

	// treat old and Pending pod as a special case, as they can be updated without a consequence
	// PodUpdatePolicy is ignored here since in-place update for a pending pod doesn't make much sense.
	for _, pod := range oldPodList {
		updatePolicy, _, err := getPodUpdatePolicy(its, pod)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if isPodPending(pod) && updatePolicy != noOpsPolicy {
			err = tree.Delete(pod)
			// wait another reconciliation, so that the following update process won't be confused
			return kubebuilderx.Continue, err
		}
	}

	updatingPods := 0
	unavailableConsumed := 0
	isBlocked := false
	needRetry := false
	for _, pod := range oldPodList {
		if updatingPods >= rollingUpdateQuota {
			break
		}
		if updatingPods >= memberUpdateQuota {
			break
		}
		// determine whether updating this pod would consume unavailable quota
		// updating an already-unavailable pod should not consume unavailable quota
		wouldConsumeUnavailable := 0
		if intctrlutil.IsPodAvailable(pod, its.Spec.MinReadySeconds) {
			wouldConsumeUnavailable = 1
		}
		if unavailableConsumed+wouldConsumeUnavailable > unavailableQuota {
			// skip pods that would exceed unavailable quota; try next pod
			continue
		}
		if canBeUpdated, retry := r.isPodCanBeUpdated(tree, its, pod); !canBeUpdated {
			needRetry = retry
			break
		}

		updatePolicy, specUpdatePolicy, err := getPodUpdatePolicy(its, pod)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if updatePolicy == recreatePolicy && specUpdatePolicy == kbappsv1.StrictInPlacePodUpdatePolicyType {
			message := fmt.Sprintf("InstanceSet %s/%s blocks on update as the PodUpdatePolicy is %s and the pod %s can not inplace update",
				its.Namespace, its.Name, kbappsv1.StrictInPlacePodUpdatePolicyType, pod.Name)
			if tree != nil && tree.EventRecorder != nil {
				tree.EventRecorder.Eventf(its, corev1.EventTypeWarning, EventReasonStrictInPlace, message)
			}
			meta.SetStatusCondition(&its.Status.Conditions, *buildBlockedCondition(its, message))
			isBlocked = true
			break
		}
		if updatePolicy == inPlaceUpdatePolicy && specUpdatePolicy == kbappsv1.ReCreatePodUpdatePolicyType {
			updatePolicy = recreatePolicy
		}
		if updatePolicy == inPlaceUpdatePolicy {
			newPod, err := buildInstancePodByTemplate(pod.Name, nameToTemplateMap[pod.Name], its, getPodRevision(pod))
			if err != nil {
				return kubebuilderx.Continue, err
			}
			newMergedPod := copyAndMerge(pod, newPod)
			supportResizeSubResource, err := intctrlutil.SupportResizeSubResource()
			if err != nil {
				tree.Logger.Error(err, "check support resize sub resource error")
				return kubebuilderx.Continue, err
			}

			// if already updating using subresource, don't update it again, because without subresource, those fields are considered immutable.
			// Another reconciliation will be triggered since pod status will be updated.
			if !equalResourcesInPlaceFields(pod, newPod) && supportResizeSubResource {
				err = tree.Update(newMergedPod, kubebuilderx.WithSubResource("resize"))
			} else {
				if err = r.switchover(tree, its, newMergedPod.(*corev1.Pod)); err != nil {
					return kubebuilderx.Continue, err
				}
				err = tree.Update(newMergedPod)
			}
			if err != nil {
				return kubebuilderx.Continue, err
			}
			updatingPods++
			unavailableConsumed += wouldConsumeUnavailable
		} else if updatePolicy == recreatePolicy {
			if !isTerminating(pod) {
				if err = r.switchover(tree, its, pod); err != nil {
					return kubebuilderx.Continue, err
				}
				if err = tree.Delete(pod); err != nil {
					return kubebuilderx.Continue, err
				}
			}
			updatingPods++
			unavailableConsumed += wouldConsumeUnavailable
		}

		// actively reload the new configuration when the pod or container has not been updated
		if updatePolicy == noOpsPolicy {
			allUpdated, err := r.reconfigure(tree, its, pod)
			if err != nil {
				return kubebuilderx.Continue, err
			}
			if !allUpdated {
				updatingPods++
			}
		}
	}

	if !isBlocked {
		meta.RemoveStatusCondition(&its.Status.Conditions, string(workloads.InstanceUpdateRestricted))
	}
	if needRetry {
		return kubebuilderx.RetryAfter(time.Second * time.Duration(its.Spec.MinReadySeconds)), nil
	}
	return kubebuilderx.Continue, nil
}

func (r *updateReconciler) rollingUpdateQuota(its *workloads.InstanceSet, podList []*corev1.Pod) (int, int, error) {
	// handle 'RollingUpdate'
	replicas, maxUnavailable, err := parseReplicasNMaxUnavailable(its.Spec.InstanceUpdateStrategy, len(podList))
	if err != nil {
		return -1, -1, err
	}
	currentUnavailable := 0
	for _, pod := range podList {
		if !intctrlutil.IsPodAvailable(pod, its.Spec.MinReadySeconds) {
			currentUnavailable++
		}
	}
	unavailable := maxUnavailable - currentUnavailable
	return replicas, unavailable, nil
}

func (r *updateReconciler) memberUpdateQuota(its *workloads.InstanceSet, podList []*corev1.Pod) (int, error) {
	// if it's a roleful InstanceSet, we use updateCount to represent Pods can be updated according to the spec.memberUpdateStrategy.
	updateCount := len(podList)
	if len(its.Spec.Roles) > 0 {
		plan := NewUpdatePlan(*its, podList, r.isPodOrConfigUpdated)
		podsToBeUpdated, err := plan.Execute()
		if err != nil {
			return -1, err
		}
		updateCount = len(podsToBeUpdated)
	}
	return updateCount, nil
}

func (r *updateReconciler) isPodCanBeUpdated(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pod *corev1.Pod) (bool, bool) {
	if !isImageMatched(pod) {
		tree.Logger.Info(fmt.Sprintf("InstanceSet %s/%s blocks on update as the pod %s does not have the same image(s) in the status and in the spec", its.Namespace, its.Name, pod.Name))
		return false, false
	}
	// Allow updates even when the pod is not ready/available or its role is not ready.
	// Rolling constraints are enforced by quotas; do not block here.
	return true, false
}

func (r *updateReconciler) switchover(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pod *corev1.Pod) error {
	if its.Spec.LifecycleActions == nil || its.Spec.LifecycleActions.Switchover == nil {
		return nil
	}

	lfa, err := newLifecycleAction(its, nil, pod)
	if err != nil {
		return err
	}

	err = lfa.Switchover(tree.Context, nil, nil, "")
	if err == nil {
		tree.Logger.Info("succeed to call switchover action", "pod", pod.Name)
	} else if !errors.Is(err, lifecycle.ErrActionNotDefined) {
		tree.Logger.Info("failed to call switchover action, ignore it", "pod", pod.Name, "error", err)
	}
	return nil
}

func (r *updateReconciler) reconfigure(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pod *corev1.Pod) (bool, error) {
	allUpdated := true
	for _, config := range its.Spec.Configs {
		if !r.isConfigUpdated(its, pod, config) {
			allUpdated = false
			if err := r.reconfigureConfig(tree, its, pod, config); err != nil {
				return false, err
			}
		}
		// TODO: compose the status from pods but not the its spec and status
		r.setInstanceConfigStatus(its, pod, config)
	}
	return allUpdated, nil
}

func (r *updateReconciler) reconfigureConfig(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pod *corev1.Pod, config workloads.ConfigTemplate) error {
	if config.Reconfigure == nil {
		return nil // skip
	}

	itsCopy := its.DeepCopy()
	if itsCopy.Spec.LifecycleActions == nil {
		itsCopy.Spec.LifecycleActions = &workloads.LifecycleActions{}
	}
	itsCopy.Spec.LifecycleActions.Reconfigure = config.Reconfigure
	lfa, err := newLifecycleAction(itsCopy, nil, pod)
	if err != nil {
		return err
	}

	if len(config.ReconfigureActionName) == 0 {
		err = lfa.Reconfigure(tree.Context, nil, nil, config.Parameters)
	} else {
		err = lfa.UserDefined(tree.Context, nil, nil, config.ReconfigureActionName, config.Reconfigure, config.Parameters)
	}
	if err != nil {
		if errors.Is(err, lifecycle.ErrActionNotDefined) {
			return nil
		}
		if errors.Is(err, lifecycle.ErrPreconditionFailed) {
			return intctrlutil.NewDelayedRequeueError(time.Second,
				fmt.Sprintf("replicas not up-to-date when reconfiguring: %s", err.Error()))
		}
		return err
	}
	tree.Logger.Info("successfully reconfigure the pod", "pod", pod.Name, "generation", config.Generation)
	return nil
}

func (r *updateReconciler) setInstanceConfigStatus(its *workloads.InstanceSet, pod *corev1.Pod, config workloads.ConfigTemplate) {
	if its.Status.InstanceStatus == nil {
		its.Status.InstanceStatus = make([]workloads.InstanceStatus, 0)
	}
	idx := slices.IndexFunc(its.Status.InstanceStatus, func(instance workloads.InstanceStatus) bool {
		return instance.PodName == pod.Name
	})
	if idx < 0 {
		its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: pod.Name})
		idx = len(its.Status.InstanceStatus) - 1
	}

	if its.Status.InstanceStatus[idx].Configs == nil {
		its.Status.InstanceStatus[idx].Configs = make([]workloads.InstanceConfigStatus, 0)
	}
	status := workloads.InstanceConfigStatus{
		Name:       config.Name,
		Generation: config.Generation,
	}
	for i, configStatus := range its.Status.InstanceStatus[idx].Configs {
		if configStatus.Name == config.Name {
			its.Status.InstanceStatus[idx].Configs[i] = status
			return
		}
	}
	its.Status.InstanceStatus[idx].Configs = append(its.Status.InstanceStatus[idx].Configs, status)
}

func (r *updateReconciler) isPodOrConfigUpdated(its *workloads.InstanceSet, pod *corev1.Pod) (bool, error) {
	policy, _, err := getPodUpdatePolicy(its, pod)
	if err != nil {
		return false, err
	}
	if policy != noOpsPolicy {
		return false, nil
	}
	for _, config := range its.Spec.Configs {
		if !r.isConfigUpdated(its, pod, config) {
			return false, nil
		}
	}
	return true, nil
}

func (r *updateReconciler) isConfigUpdated(its *workloads.InstanceSet, pod *corev1.Pod, config workloads.ConfigTemplate) bool {
	idx := slices.IndexFunc(its.Status.InstanceStatus, func(instance workloads.InstanceStatus) bool {
		return instance.PodName == pod.Name
	})
	if idx < 0 {
		return true // new pod provisioned
	}
	for _, configStatus := range its.Status.InstanceStatus[idx].Configs {
		if configStatus.Name == config.Name {
			return config.Generation <= configStatus.Generation
		}
	}
	return config.Generation <= 0
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

func parseReplicasNMaxUnavailable(updateStrategy *workloads.InstanceUpdateStrategy, totalReplicas int) (int, int, error) {
	replicas := totalReplicas
	maxUnavailable := 1
	if updateStrategy == nil {
		return replicas, maxUnavailable, nil
	}
	rollingUpdate := updateStrategy.RollingUpdate
	if rollingUpdate == nil {
		return replicas, maxUnavailable, nil
	}
	var err error
	if rollingUpdate.Replicas != nil {
		replicas, err = intstr.GetScaledValueFromIntOrPercent(rollingUpdate.Replicas, totalReplicas, false)
		if err != nil {
			return replicas, maxUnavailable, err
		}
	}
	if rollingUpdate.MaxUnavailable != nil {
		maxUnavailable, err = intstr.GetScaledValueFromIntOrPercent(intstr.ValueOrDefault(rollingUpdate.MaxUnavailable, intstr.FromInt32(1)), totalReplicas, false)
		if err != nil {
			return 0, 0, err
		}
		// maxUnavailable might be zero for small percentage with round down.
		// So we have to enforce it not to be less than 1.
		if maxUnavailable < 1 {
			maxUnavailable = 1
		}
	}
	return replicas, maxUnavailable, nil
}
