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
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset/instancetemplate"
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
	replicas, maxUnavailable, err := parseReplicasNMaxUnavailable(its.Spec.InstanceUpdateStrategy, len(oldPodList))
	if err != nil {
		return kubebuilderx.Continue, err
	}
	currentUnavailable := 0
	for _, pod := range oldPodList {
		if !intctrlutil.IsPodAvailable(pod, its.Spec.MinReadySeconds) {
			currentUnavailable++
		}
	}
	unavailable := maxUnavailable - currentUnavailable

	// if it's a roleful InstanceSet, we use updateCount to represent Pods can be updated according to the spec.memberUpdateStrategy.
	updateCount := len(oldPodList)
	if len(its.Spec.Roles) > 0 {
		plan := NewUpdatePlan(*its, oldPodList, r.isPodOrConfigUpdated)
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

	// treat old and Pending pod as a special case, as they can be updated without a consequence
	// PodUpdatePolicy is ignored here since in-place update for a pending pod doesn't make much sense.
	for _, pod := range oldPodList {
		updatePolicy, err := getPodUpdatePolicy(its, pod)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if isPodPending(pod) && updatePolicy != NoOpsPolicy {
			err = tree.Delete(pod)
			// wait another reconciliation, so that the following update process won't be confused
			return kubebuilderx.Continue, err
		}
	}

	canBeUpdated := func(pod *corev1.Pod) bool {
		if !isImageMatched(pod) {
			tree.Logger.Info(fmt.Sprintf("InstanceSet %s/%s blocks on update as the pod %s does not have the same image(s) in the status and in the spec", its.Namespace, its.Name, pod.Name))
			return false
		}
		if !intctrlutil.IsPodReady(pod) {
			tree.Logger.Info(fmt.Sprintf("InstanceSet %s/%s blocks on update as the pod %s is not ready", its.Namespace, its.Name, pod.Name))
			return false
		}
		if !intctrlutil.IsPodAvailable(pod, its.Spec.MinReadySeconds) {
			tree.Logger.Info(fmt.Sprintf("InstanceSet %s/%s blocks on update as the pod %s is not available", its.Namespace, its.Name, pod.Name))
			// no pod event will trigger the next reconciliation, so retry it
			needRetry = true
			return false
		}
		if !isRoleReady(pod, its.Spec.Roles) {
			tree.Logger.Info(fmt.Sprintf("InstanceSet %s/%s blocks on update as the role of pod %s is not ready", its.Namespace, its.Name, pod.Name))
			return false
		}

		return true
	}

	for _, pod := range oldPodList {
		if updatingPods >= updateCount || updatingPods >= unavailable {
			break
		}
		if updatedPods >= replicas {
			break
		}

		if !canBeUpdated(pod) {
			break
		}

		updatePolicy, err := getPodUpdatePolicy(its, pod)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if its.Spec.PodUpdatePolicy == kbappsv1.StrictInPlacePodUpdatePolicyType && updatePolicy == RecreatePolicy {
			message := fmt.Sprintf("InstanceSet %s/%s blocks on update as the PodUpdatePolicy is %s and the pod %s can not inplace update",
				its.Namespace, its.Name, kbappsv1.StrictInPlacePodUpdatePolicyType, pod.Name)
			if tree != nil && tree.EventRecorder != nil {
				tree.EventRecorder.Eventf(its, corev1.EventTypeWarning, EventReasonStrictInPlace, message)
			}
			meta.SetStatusCondition(&its.Status.Conditions, *buildBlockedCondition(its, message))
			isBlocked = true
			break
		}
		if updatePolicy == InPlaceUpdatePolicy {
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
		} else if updatePolicy == RecreatePolicy {
			if !isTerminating(pod) {
				if err = r.switchover(tree, its, pod); err != nil {
					return kubebuilderx.Continue, err
				}
				if err = tree.Delete(pod); err != nil {
					return kubebuilderx.Continue, err
				}
			}
			updatingPods++
		}

		// actively reload the new configuration when the pod or container has not been updated
		if updatePolicy == NoOpsPolicy {
			allUpdated, err := r.reconfigure(tree, its, pod)
			if err != nil {
				return kubebuilderx.Continue, err
			}
			if !allUpdated {
				updatingPods++
			}
		}

		updatedPods++
	}
	if !isBlocked {
		meta.RemoveStatusCondition(&its.Status.Conditions, string(workloads.InstanceUpdateRestricted))
	}
	if needRetry {
		return kubebuilderx.RetryAfter(time.Second * time.Duration(its.Spec.MinReadySeconds)), nil
	}
	return kubebuilderx.Continue, nil
}

func (r *updateReconciler) switchover(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pod *corev1.Pod) error {
	if its.Spec.MembershipReconfiguration == nil || its.Spec.MembershipReconfiguration.Switchover == nil {
		return nil
	}

	clusterName, err := r.clusterName(its)
	if err != nil {
		return err
	}
	lifecycleActions := &kbappsv1.ComponentLifecycleActions{
		Switchover: its.Spec.MembershipReconfiguration.Switchover,
	}
	templateVars := func() map[string]any {
		if its.Spec.TemplateVars == nil {
			return nil
		}
		m := make(map[string]any)
		for k, v := range its.Spec.TemplateVars {
			m[k] = v
		}
		return m
	}()
	lfa, err := lifecycle.New(its.Namespace, clusterName, its.Labels[constant.KBAppComponentLabelKey], lifecycleActions, templateVars, pod)
	if err != nil {
		return err
	}

	err = lfa.Switchover(tree.Context, nil, nil, "")
	if err != nil {
		if errors.Is(err, lifecycle.ErrActionNotDefined) {
			return nil
		}
		return err
	}
	tree.Logger.Info("successfully call switchover action for pod", "pod", pod.Name)
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

	clusterName, err := r.clusterName(its)
	if err != nil {
		return err
	}

	lifecycleActions := &kbappsv1.ComponentLifecycleActions{
		Reconfigure: config.Reconfigure,
	}
	templateVars := func() map[string]any {
		if its.Spec.TemplateVars == nil {
			return nil
		}
		m := make(map[string]any)
		for k, v := range its.Spec.TemplateVars {
			m[k] = v
		}
		return m
	}()
	lfa, err := lifecycle.New(its.Namespace, clusterName, its.Labels[constant.KBAppComponentLabelKey], lifecycleActions, templateVars, pod)
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
	policy, err := getPodUpdatePolicy(its, pod)
	if err != nil {
		return false, err
	}
	if policy != NoOpsPolicy {
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

func (r *updateReconciler) clusterName(its *workloads.InstanceSet) (string, error) {
	var clusterName string
	if its.Labels != nil {
		clusterName = its.Labels[constant.AppInstanceLabelKey]
	}
	if len(clusterName) == 0 {
		return "", fmt.Errorf("InstanceSet %s/%s has no label %s", its.Namespace, its.Name, constant.AppInstanceLabelKey)
	}
	return clusterName, nil
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
