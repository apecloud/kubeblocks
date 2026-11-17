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

package instanceset

import (
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apps "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
)

func prepareMemberLeave(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its := tree.GetRoot().(*workloads.InstanceSet)
	assignments, _, err := instancetemplate.BuildAssignments(tree, its)
	if err != nil {
		if instancetemplate.IsAssignmentIncomplete(err) {
			return memberActionWait(tree, its, err)
		}
		return kubebuilderx.Continue, err
	}
	allocated := sets.New[string]()
	for _, assignment := range assignments {
		allocated.Insert(assignment.InstanceName)
	}
	changed := false
	for i := range its.Status.InstanceStatus {
		status := &its.Status.InstanceStatus[i]
		if !allocated.Has(status.PodName) && status.MemberLeft == nil && needsMemberLeave(its, status) {
			status.MemberLeft = ptr.To(false)
			changed = true
		}
	}
	if changed {
		return kubebuilderx.RetryAfter(time.Second), nil
	}
	return kubebuilderx.Continue, nil
}

func reconcileMember(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pod *corev1.Pod, join bool, reader client.Reader) (kubebuilderx.Result, error) {
	status := its.FindInstanceStatus(pod.Name)
	if status == nil || !memberActionsEnabled(its) || isStopRequested(its) {
		return kubebuilderx.Continue, nil
	}
	if join && !needsMemberJoin(its, status) {
		return kubebuilderx.Continue, nil
	}
	if !join && !needsMemberRetirement(its, status) {
		return kubebuilderx.Continue, nil
	}
	if join || its.Spec.LifecycleActions.MemberLeave != nil {
		pods := make([]*corev1.Pod, 0)
		for _, obj := range tree.List(&corev1.Pod{}) {
			pods = append(pods, obj.(*corev1.Pod))
		}
		leave := !join
		if join && (!pod.DeletionTimestamp.IsZero() || !intctrlutil.IsPodAvailable(pod, its.Spec.MinReadySeconds)) {
			return kubebuilderx.Continue, nil
		}
		action := its.Spec.LifecycleActions.MemberJoin
		if leave {
			action = its.Spec.LifecycleActions.MemberLeave
		}
		opts, err := validateMemberExecution(action, pod, pods)
		if err != nil {
			return memberActionWait(tree, its, err)
		}
		lfa, err := newLifecycleAction(its, tree, pod)
		if err != nil {
			return memberActionWait(tree, its, err)
		}
		if leave && its.Spec.LifecycleActions.Switchover != nil {
			exclusive := false
			for _, role := range its.Spec.Roles {
				if role.IsExclusive && role.Name == pod.Labels[constant.RoleLabelKey] {
					exclusive = true
				}
			}
			if exclusive {
				opts, err := validateMemberExecution(its.Spec.LifecycleActions.Switchover, pod, pods)
				if err != nil {
					return memberActionWait(tree, its, err)
				}
				if err := lfa.Switchover(tree.Context, reader, opts, ""); err != nil {
					return memberActionWait(tree, its, err)
				}
				return memberActionWait(tree, its, fmt.Errorf("waiting for %s to relinquish its exclusive role", pod.Name))
			}
		}
		if join {
			err = lfa.MemberJoin(tree.Context, reader, opts)
		} else {
			err = lfa.MemberLeave(tree.Context, reader, opts)
		}
		if err != nil {
			return memberActionWait(tree, its, err)
		}
	}
	status.MemberJoined = ptr.To(join)
	if join {
		status.MemberLeft = nil
	} else if its.Spec.LifecycleActions.MemberLeave != nil {
		status.MemberLeft = ptr.To(true)
	}
	meta.RemoveStatusCondition(&its.Status.Conditions, "InstanceLifecycle")
	return kubebuilderx.RetryAfter(time.Second), nil
}

func memberActionWait(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, err error) (kubebuilderx.Result, error) {
	previous := meta.FindStatusCondition(its.Status.Conditions, "InstanceLifecycle")
	if tree.EventRecorder != nil && (previous == nil || previous.Message != err.Error()) {
		tree.EventRecorder.Event(its, corev1.EventTypeWarning, "InstanceLifecycle", err.Error())
	}
	meta.SetStatusCondition(&its.Status.Conditions, metav1.Condition{Type: "InstanceLifecycle", Status: metav1.ConditionFalse, ObservedGeneration: its.Generation, Reason: "Waiting", Message: err.Error()})
	return kubebuilderx.RetryAfter(2 * time.Second), nil
}

func memberActionsEnabled(its *workloads.InstanceSet) bool {
	a := its.Spec.LifecycleActions
	return a != nil && (a.MemberJoin != nil || a.MemberLeave != nil)
}

func needsMemberJoin(its *workloads.InstanceSet, status *workloads.InstanceStatus) bool {
	if !memberActionsEnabled(its) || its.Spec.LifecycleActions.MemberJoin == nil || status == nil {
		return false
	}
	return status.MemberLeft != nil || (status.MemberJoined != nil && !*status.MemberJoined)
}

func memberJoinRequiresRuntimeReady(its *workloads.InstanceSet) bool {
	if !memberActionsEnabled(its) || its.Spec.LifecycleActions.MemberJoin == nil {
		return false
	}
	return ptr.Deref(its.Spec.LifecycleActions.MemberJoin.PreCondition, "") == apps.RuntimeReadyPreConditionType
}

func memberRetirementBlocked(its *workloads.InstanceSet, old *workloads.InstanceStatus) bool {
	if !memberActionsEnabled(its) || old == nil {
		return false
	}
	established := ptr.Deref(old.MemberJoined, false) || (old.Provisioned && !ptr.Deref(old.MemberLeft, false))
	if !established {
		return false
	}
	pendingJoin := slices.ContainsFunc(its.Status.InstanceStatus, func(status workloads.InstanceStatus) bool {
		if status.EffectiveDesiredState() != workloads.InstanceDesiredStateActive {
			return false
		}
		return needsMemberJoin(its, &status)
	})
	if !pendingJoin {
		return false
	}
	if !memberJoinRequiresRuntimeReady(its) {
		return true
	}
	// RuntimeReady requires excess Pods to be released before Join. Keep the old
	// member until all desired instances are available, then allow that release.
	return slices.ContainsFunc(its.Status.InstanceStatus, func(status workloads.InstanceStatus) bool {
		if status.EffectiveDesiredState() != workloads.InstanceDesiredStateActive {
			return false
		}
		return !status.Available
	})
}

func validateMemberActions(its *workloads.InstanceSet) error {
	if !memberActionsEnabled(its) {
		return nil
	}
	a := its.Spec.LifecycleActions
	for _, action := range []*apps.Action{a.MemberJoin, a.MemberLeave, a.Switchover} {
		if action != nil && !action.Defined() {
			return fmt.Errorf("configured lifecycle actions must define an execution handler")
		}
		if action != nil && action.PreCondition != nil && *action.PreCondition != apps.ImmediatelyPreConditionType && *action.PreCondition != apps.RuntimeReadyPreConditionType {
			return fmt.Errorf("workload-owned actions support only Immediately and RuntimeReady preconditions")
		}
		if action != nil && action.NonBlocking {
			return fmt.Errorf("workload-owned lifecycle actions must use blocking execution")
		}
	}
	return nil
}

func needsMemberLeave(its *workloads.InstanceSet, status *workloads.InstanceStatus) bool {
	if !memberActionsEnabled(its) || its.Spec.LifecycleActions.MemberLeave == nil || status == nil {
		return false
	}
	if status.MemberLeft != nil {
		return !*status.MemberLeft
	}
	return ptr.Deref(status.MemberJoined, false) || status.Provisioned
}

func needsMemberRetirement(its *workloads.InstanceSet, status *workloads.InstanceStatus) bool {
	if needsMemberLeave(its, status) {
		return true
	}
	return memberActionsEnabled(its) && its.Spec.LifecycleActions.MemberJoin != nil && status != nil && ptr.Deref(status.MemberJoined, false)
}

func validateMemberExecution(action *apps.Action, target *corev1.Pod, pods []*corev1.Pod) (*lifecycle.Options, error) {
	if target == nil {
		return nil, fmt.Errorf("member target runtime is absent")
	}
	if len(pods) == 0 {
		return nil, fmt.Errorf("no available executor for member action")
	}
	selected, err := lifecycle.SelectTargetPods(pods, target, action)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no available executor for member action")
	}
	for _, pod := range selected {
		port, err := intctrlutil.GetPortByName(*pod, kbagent.ContainerName, kbagent.DefaultHTTPPortName)
		if err != nil || port <= 0 || pod.Status.PodIP == "" || pod.DeletionTimestamp != nil {
			return nil, fmt.Errorf("instance %s has no available agent endpoint", pod.Name)
		}
	}
	selector := action.TargetPodSelector
	if selector == "" && action.Exec != nil {
		selector = action.Exec.TargetPodSelector
	}
	if selector == apps.AnyReplica {
		return &lifecycle.Options{TargetPodName: selected[0].Name}, nil
	}
	return nil, nil
}
