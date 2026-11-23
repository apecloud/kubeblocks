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
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type updateReconciler struct {
	reader client.Reader
}

var _ kubebuilderx.Reconciler = &updateReconciler{}

func NewUpdateReconciler(reader client.Reader) kubebuilderx.Reconciler {
	return &updateReconciler{reader: reader}
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

	// The alignment check above ensures oldPodList contains only the current Pod for this Instance.
	// Both update passes can share a desired Pod built with that Pod's revision.
	desiredPod, err := buildInstancePod(inst, getPodRevision(oldPodList[0]))
	if err != nil {
		return kubebuilderx.Continue, err
	}

	// treat old and Pending pod as a special case, as they can be updated without a consequence
	// podUpdatePolicy is ignored here since in-place update for a pending pod doesn't make much sense.
	for _, pod := range oldPodList {
		updatePolicy, _, err := getPodUpdatePolicyForDesiredPod(inst, pod, desiredPod)
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

		updatePolicy, specUpdatePolicy, err := getPodUpdatePolicyForDesiredPod(inst, pod, desiredPod)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if updatePolicy == recreatePolicy && specUpdatePolicy == kbappsv1.StrictInPlacePodUpdatePolicyType {
			message := fmt.Sprintf("Instance %s/%s blocks on update as the podUpdatePolicy is %s and the pod %s can not inplace update",
				inst.Namespace, inst.Name, kbappsv1.StrictInPlacePodUpdatePolicyType, pod.Name)
			if tree != nil && tree.EventRecorder != nil {
				tree.EventRecorder.Eventf(inst, corev1.EventTypeWarning, EventReasonStrictInPlace, message)
			}
			meta.SetStatusCondition(&inst.Status.Conditions, *buildBlockedCondition(inst, message))
			isBlocked = true
			break
		}
		if updatePolicy == inPlaceUpdatePolicy && specUpdatePolicy == kbappsv1.ReCreatePodUpdatePolicyType {
			updatePolicy = recreatePolicy
		}

		allUpdated, err := r.reconfigure(tree, inst, pod, desiredPod)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if !allUpdated && updatePolicy == noOpsPolicy {
			continue
		}

		switch updatePolicy {
		case noOpsPolicy:
			if patch := instancetemplate.ConfigReloadScopePatch(pod, desiredPod); patch != nil {
				if err := tree.Update(patch, kubebuilderx.WithPatch(true)); err != nil {
					return kubebuilderx.Continue, err
				}
			}
		case inPlaceUpdatePolicy:
			newMergedPod := copyAndMerge(pod, desiredPod)
			instancetemplate.SyncConfigReloadScope(newMergedPod.(*corev1.Pod), desiredPod)
			supportResizeSubResource, err := intctrlutil.SupportResizeSubResource()
			if err != nil {
				tree.Logger.Error(err, "check support resize sub resource error")
				return kubebuilderx.Continue, err
			}

			switch {
			case !equalResourcesInPlaceFields(pod, desiredPod) && supportResizeSubResource:
				err = tree.Update(newMergedPod, kubebuilderx.WithSubResource("resize"))
			case safeMetadataOnlyInPlaceUpdate(pod, desiredPod), safeKBManagedImageOnlyInPlaceUpdate(pod, desiredPod):
				err = tree.Update(newMergedPod, kubebuilderx.WithPatch(true))
			default:
				if err = r.switchover(tree, inst, newMergedPod.(*corev1.Pod)); err != nil {
					return kubebuilderx.Continue, err
				}
				err = tree.Update(newMergedPod)
			}
			if err != nil {
				return kubebuilderx.Continue, err
			}
		case recreatePolicy:
			if !isTerminating(pod) {
				if err = r.switchover(tree, inst, pod); err != nil {
					return kubebuilderx.Continue, err
				}
				if err = tree.Delete(pod); err != nil {
					return kubebuilderx.Continue, err
				}
			}
		}
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
	if inst.Spec.LifecycleActions == nil || inst.Spec.LifecycleActions.Switchover == nil {
		return nil
	}

	// FIXME: select all available pods
	lfa, err := newLifecycleAction(inst, []*corev1.Pod{pod}, pod)
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

func (r *updateReconciler) reconfigure(tree *kubebuilderx.ObjectTree, inst *workloads.Instance, pod, desiredPod *corev1.Pod) (bool, error) {
	configs := instancetemplate.ConfigsForMountedVolumes(inst.Spec.Configs, pod, desiredPod)
	toUpdate, err := configsToUpdate(configs, pod)
	if err != nil {
		return false, err
	}
	reloadScope := instancetemplate.ConfigReloadScope(desiredPod).Union(
		instancetemplate.AppliedConfigReloadOverrides(inst.Spec.InstanceSetName, pod),
	)
	if inst.Spec.InstanceSetName != "" && slices.ContainsFunc(toUpdate, func(config workloads.ConfigTemplate) bool {
		return config.Reconfigure != nil && !reloadScope.Has(config.Name)
	}) {
		peerScope, err := r.appliedConfigOverrides(tree.Context, inst)
		if err != nil {
			return false, err
		}
		reloadScope = reloadScope.Union(peerScope)
	}
	for _, config := range toUpdate {
		targetPod, eligible := instancetemplate.ConfigReloadTarget(pod, config, reloadScope.Has(config.Name))
		if !eligible {
			continue
		}
		if err = r.reconfigureInst(tree, inst, pod, config, targetPod); err != nil {
			return false, err
		}
	}
	return len(toUpdate) == 0, nil
}

func (r *updateReconciler) appliedConfigOverrides(ctx context.Context, inst *workloads.Instance) (sets.Set[string], error) {
	pods := &corev1.PodList{}
	placement := multicluster.InControlContext()
	if inst.Annotations[constant.KBAppMultiClusterPlacementKey] != "" {
		placement = multicluster.InDataContextUnspecified()
	}
	if err := r.reader.List(ctx, pods, client.InNamespace(inst.Namespace), placement,
		client.MatchingLabels{constant.AppManagedByLabelKey: constant.AppName}); err != nil {
		return nil, err
	}

	// Sibling Pods supply observations only. Reload actions still use this
	// Instance's own Pod as their only candidate.
	observed := make([]*corev1.Pod, len(pods.Items))
	for i := range pods.Items {
		observed[i] = &pods.Items[i]
	}
	return instancetemplate.AppliedConfigReloadOverrides(inst.Spec.InstanceSetName, observed...), nil
}

func (r *updateReconciler) reconfigureInst(tree *kubebuilderx.ObjectTree, inst *workloads.Instance, pod *corev1.Pod, config workloads.ConfigTemplate, targetPod string) error {
	if config.Reconfigure == nil {
		return nil
	}

	instCopy := inst.DeepCopy()
	if instCopy.Spec.LifecycleActions == nil {
		instCopy.Spec.LifecycleActions = &workloads.LifecycleActions{}
	}
	instCopy.Spec.LifecycleActions.Reconfigure = config.Reconfigure

	lfa, err := newLifecycleAction(instCopy, []*corev1.Pod{pod}, pod)
	if err != nil {
		return err
	}

	opts := reconfigureOptions(config, targetPod)
	if len(config.ReconfigureActionName) == 0 {
		err = lfa.Reconfigure(tree.Context, nil, opts, config.Parameters)
	} else {
		err = lfa.UserDefined(tree.Context, nil, opts, config.ReconfigureActionName, config.Reconfigure, config.Parameters)
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
	tree.Logger.Info("successfully reconfigure the pod", "pod", pod.Name, "configHash", ptr.Deref(config.ConfigHash, ""))
	return nil
}

func reconfigureOptions(config workloads.ConfigTemplate, targetPod string) *lifecycle.Options {
	if len(config.ReconfigureArgs) == 0 && targetPod == "" {
		return nil
	}
	return &lifecycle.Options{Arguments: config.ReconfigureArgs, TargetPodName: targetPod}
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
