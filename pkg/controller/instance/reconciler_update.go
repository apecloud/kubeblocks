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
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

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
	inst := tree.GetRoot().(*workloads.Instance)

	newNameSet := sets.New[string](podName(inst))
	oldNameSet := sets.New[string]()
	oldPodList := make([]*corev1.Pod, 0)
	for _, object := range tree.List(&corev1.Pod{}) {
		oldNameSet.Insert(object.GetName())
		oldPodList = append(oldPodList, object.(*corev1.Pod))
	}
	updateNameSet := oldNameSet.Intersection(newNameSet)
	if len(updateNameSet) != len(oldNameSet) || len(updateNameSet) != len(newNameSet) {
		tree.Logger.Info(fmt.Sprintf("Instance %s/%s is not aligned", inst.Namespace, inst.Name))
		return kubebuilderx.Continue, nil
	}

	// do nothing if update strategy type is 'OnDelete'
	if inst.Spec.InstanceUpdateStrategyType != nil && *inst.Spec.InstanceUpdateStrategyType == kbappsv1.OnDeleteStrategyType {
		return kubebuilderx.Continue, nil
	}

	// treat old and Pending pod as a special case, as they can be updated without a consequence
	// podUpdatePolicy is ignored here since in-place update for a pending pod doesn't make much sense.
	for _, pod := range oldPodList {
		updatePolicy, err := getPodUpdatePolicy(inst, pod)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if isPodPending(pod) && updatePolicy != noOpsPolicy {
			err = tree.Delete(pod)
			// wait another reconciliation, so that the following update process won't be confused
			return kubebuilderx.Continue, err
		}
	}

	needRetry := false
	isBlocked := false
	canBeUpdated := func(pod *corev1.Pod) bool {
		if !isImageMatched(pod) {
			tree.Logger.Info(fmt.Sprintf("Instance %s/%s blocks on update as the pod %s does not have the same image(s) in the status and in the spec", inst.Namespace, inst.Name, pod.Name))
			return false
		}
		if !intctrlutil.IsPodReady(pod) {
			tree.Logger.Info(fmt.Sprintf("Instance %s/%s blocks on update as the pod %s is not ready", inst.Namespace, inst.Name, pod.Name))
			return false
		}
		if !intctrlutil.IsPodAvailable(pod, inst.Spec.MinReadySeconds) {
			tree.Logger.Info(fmt.Sprintf("Instance %s/%s blocks on update as the pod %s is not available", inst.Namespace, inst.Name, pod.Name))
			// no pod event will trigger the next reconciliation, so retry it
			needRetry = true
			return false
		}
		if !isRoleReady(pod, inst.Spec.Roles) {
			tree.Logger.Info(fmt.Sprintf("Instance %s/%s blocks on update as the role of pod %s is not ready", inst.Namespace, inst.Name, pod.Name))
			return false
		}
		return true
	}

	for _, pod := range oldPodList {
		if !canBeUpdated(pod) {
			break
		}

		updatePolicy, err := getPodUpdatePolicy(inst, pod)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if inst.Spec.PodUpdatePolicy == kbappsv1.StrictInPlacePodUpdatePolicyType && updatePolicy == recreatePolicy {
			message := fmt.Sprintf("Instance %s/%s blocks on update as the podUpdatePolicy is %s and the pod %s can not inplace update",
				inst.Namespace, inst.Name, kbappsv1.StrictInPlacePodUpdatePolicyType, pod.Name)
			if tree != nil && tree.EventRecorder != nil {
				tree.EventRecorder.Eventf(inst, corev1.EventTypeWarning, EventReasonStrictInPlace, message)
			}
			meta.SetStatusCondition(&inst.Status.Conditions, *buildBlockedCondition(inst, message))
			isBlocked = true
			break
		}
		if updatePolicy == inPlaceUpdatePolicy {
			newPod, err := buildInstancePod(inst, getPodRevision(pod))
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
				if err = r.switchover(tree, inst, newMergedPod.(*corev1.Pod)); err != nil {
					return kubebuilderx.Continue, err
				}
				err = tree.Update(newMergedPod)
			}
			if err != nil {
				return kubebuilderx.Continue, err
			}
		} else if updatePolicy == recreatePolicy {
			if !isTerminating(pod) {
				if err = r.switchover(tree, inst, pod); err != nil {
					return kubebuilderx.Continue, err
				}
				if err = tree.Delete(pod); err != nil {
					return kubebuilderx.Continue, err
				}
			}
		}

		// TODO: ???
		//// actively reload the new configuration when the pod or container has not been updated
		// if updatePolicy == noOpsPolicy {
		//	_, err := r.reconfigure(tree, inst, pod)
		//	if err != nil {
		//		return kubebuilderx.Continue, err
		//	}
		// }
	}
	if !isBlocked {
		meta.RemoveStatusCondition(&inst.Status.Conditions, string(workloads.InstanceUpdateRestricted))
	}
	if needRetry {
		return kubebuilderx.RetryAfter(time.Second * time.Duration(inst.Spec.MinReadySeconds)), nil
	}
	return kubebuilderx.Continue, nil
}

func (r *updateReconciler) switchover(tree *kubebuilderx.ObjectTree, inst *workloads.Instance, pod *corev1.Pod) error {
	if inst.Spec.MembershipReconfiguration == nil || inst.Spec.MembershipReconfiguration.Switchover == nil {
		return nil
	}

	clusterName, err := r.clusterName(inst)
	if err != nil {
		return err
	}
	lifecycleActions := &kbappsv1.ComponentLifecycleActions{
		Switchover: inst.Spec.MembershipReconfiguration.Switchover,
	}
	templateVars := func() map[string]any {
		if inst.Spec.TemplateVars == nil {
			return nil
		}
		m := make(map[string]any)
		for k, v := range inst.Spec.TemplateVars {
			m[k] = v
		}
		return m
	}()
	lfa, err := lifecycle.New(inst.Namespace, clusterName, inst.Labels[constant.KBAppComponentLabelKey], lifecycleActions, templateVars, pod)
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

// func (r *updateReconciler) reconfigure(tree *kubebuilderx.ObjectTree, inst *workloads.Instance, pod *corev1.Pod) (bool, error) {
//	allUpdated := true
//	for _, config := range inst.Spec.Configs {
//		if !r.isConfigUpdated(inst, pod, config) {
//			allUpdated = false
//			if err := r.reconfigureConfig(tree, inst, pod, config); err != nil {
//				return false, err
//			}
//		}
//		// TODO: compose the status from pods but not the its spec and status
//		r.setInstanceConfigStatus(inst, pod, config)
//	}
//	return allUpdated, nil
// }
//
// func (r *updateReconciler) reconfigureConfig(tree *kubebuilderx.ObjectTree, inst *workloads.Instance, pod *corev1.Pod, config workloads.ConfigTemplate) error {
//	if config.Reconfigure == nil {
//		return nil // skip
//	}
//
//	clusterName, err := r.clusterName(inst)
//	if err != nil {
//		return err
//	}
//
//	lifecycleActions := &kbappsv1.ComponentLifecycleActions{
//		Reconfigure: config.Reconfigure,
//	}
//	templateVars := func() map[string]any {
//		if inst.Spec.TemplateVars == nil {
//			return nil
//		}
//		m := make(map[string]any)
//		for k, v := range inst.Spec.TemplateVars {
//			m[k] = v
//		}
//		return m
//	}()
//	lfa, err := lifecycle.New(inst.Namespace, clusterName, inst.Spec.InstanceSetName, lifecycleActions, templateVars, pod)
//	if err != nil {
//		return err
//	}
//
//	if len(config.ReconfigureActionName) == 0 {
//		err = lfa.Reconfigure(tree.Context, nil, nil, config.Parameters)
//	} else {
//		err = lfa.UserDefined(tree.Context, nil, nil, config.ReconfigureActionName, config.Reconfigure, config.Parameters)
//	}
//	if err != nil {
//		if errors.Is(err, lifecycle.ErrActionNotDefined) {
//			return nil
//		}
//		if errors.Is(err, lifecycle.ErrPreconditionFailed) {
//			return intctrlutil.NewDelayedRequeueError(time.Second,
//				fmt.Sprintf("replicas not up-to-date when reconfiguring: %s", err.Error()))
//		}
//		return err
//	}
//	tree.Logger.Info("successfully reconfigure the pod", "pod", pod.Name, "generation", config.Generation)
//	return nil
// }
//
// func (r *updateReconciler) setInstanceConfigStatus(its *workloads.InstanceSet, pod *corev1.Pod, config workloads.ConfigTemplate) {
//	if its.Status.InstanceStatus == nil {
//		its.Status.InstanceStatus = make([]workloads.InstanceStatus, 0)
//	}
//	idx := slices.IndexFunc(its.Status.InstanceStatus, func(instance workloads.InstanceStatus) bool {
//		return instance.PodName == pod.Name
//	})
//	if idx < 0 {
//		its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: pod.Name})
//		idx = len(its.Status.InstanceStatus) - 1
//	}
//
//	if its.Status.InstanceStatus[idx].Configs == nil {
//		its.Status.InstanceStatus[idx].Configs = make([]workloads.InstanceConfigStatus, 0)
//	}
//	status := workloads.InstanceConfigStatus{
//		Name:       config.Name,
//		Generation: config.Generation,
//	}
//	for i, configStatus := range its.Status.InstanceStatus[idx].Configs {
//		if configStatus.Name == config.Name {
//			its.Status.InstanceStatus[idx].Configs[i] = status
//			return
//		}
//	}
//	its.Status.InstanceStatus[idx].Configs = append(its.Status.InstanceStatus[idx].Configs, status)
// }
//
// func (r *updateReconciler) isPodOrConfigUpdated(inst *workloads.Instance, pod *corev1.Pod) (bool, error) {
//	policy, err := getPodUpdatePolicy(inst, pod)
//	if err != nil {
//		return false, err
//	}
//	if policy != noOpsPolicy {
//		return false, nil
//	}
//	for _, config := range inst.Spec.Configs {
//		if !r.isConfigUpdated(inst, pod, config) {
//			return false, nil
//		}
//	}
//	return true, nil
// }
//
// func (r *updateReconciler) isConfigUpdated(inst *workloads.Instance, pod *corev1.Pod, config workloads.ConfigTemplate) bool {
//	idx := slices.IndexFunc(inst.Status.InstanceStatus, func(instance workloads.InstanceStatus) bool {
//		return instance.PodName == pod.Name
//	})
//	if idx < 0 {
//		return true // new pod provisioned
//	}
//	for _, configStatus := range inst.Status.InstanceStatus[idx].Configs {
//		if configStatus.Name == config.Name {
//			return config.Generation <= configStatus.Generation
//		}
//	}
//	return config.Generation <= 0
// }

func (r *updateReconciler) clusterName(inst *workloads.Instance) (string, error) {
	var clusterName string
	if inst.Labels != nil {
		clusterName = inst.Labels[constant.AppInstanceLabelKey]
	}
	if len(clusterName) == 0 {
		return "", fmt.Errorf("instance %s/%s has no label %s", inst.Namespace, inst.Name, constant.AppInstanceLabelKey)
	}
	return clusterName, nil
}

func buildBlockedCondition(inst *workloads.Instance, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:               string(workloads.InstanceUpdateRestricted),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inst.Generation,
		Reason:             workloads.ReasonInstanceUpdateRestricted,
		Message:            message,
	}
}
