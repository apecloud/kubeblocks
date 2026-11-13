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
	publiclifecycle "github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
)

func reconcileMemberStatus(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its := tree.GetRoot().(*workloads.InstanceSet)
	original, ok := tree.GetOriginalRoot().(*workloads.InstanceSet)
	if !ok {
		return kubebuilderx.Continue, fmt.Errorf("member alignment requires a prepared object tree")
	}
	assignments, _, err := instancetemplate.BuildAssignments(tree, its)
	if err != nil {
		if instancetemplate.IsAssignmentIncomplete(err) {
			return lifecycleWait(tree, its, err)
		}
		return kubebuilderx.Continue, err
	}
	names := sets.New[string]()
	resources := sets.New[string]()
	for _, obj := range tree.List(&corev1.Pod{}) {
		names.Insert(obj.GetName())
	}
	for _, obj := range tree.List(&corev1.PersistentVolumeClaim{}) {
		resources.Insert(obj.GetLabels()[constant.KBAppPodNameLabelKey])
	}
	registered := registerMemberStatus(its, original, assignments, names, resources)
	desired := make(map[string]string, len(assignments))
	for _, assignment := range assignments {
		desired[assignment.InstanceName] = assignment.TemplateName
	}
	offline := sets.New(its.Spec.OfflineInstances...)
	for i := range its.Status.InstanceStatus {
		status := &its.Status.InstanceStatus[i]
		template, allocated := desired[status.PodName]
		switch {
		case allocated:
			status.DesiredState = workloads.InstanceDesiredStateActive
			if isStopRequested(its) {
				status.DesiredState = workloads.InstanceDesiredStateOffline
			}
			status.TemplateName = ptr.To(template)
		case offline.Has(status.PodName):
			status.DesiredState = workloads.InstanceDesiredStateOffline
		default:
			status.DesiredState = workloads.InstanceDesiredStateReleased
		}
		if !allocated || its.Spec.LifecycleActions.MemberJoin == nil {
			continue
		}
		previous := original.FindInstanceStatus(status.PodName)
		if previous != nil && previous.EffectiveDesiredState() == workloads.InstanceDesiredStateReleased {
			// A cancelled shrink may have completed Leave without saving its result.
			status.MemberJoined = ptr.To(false)
		} else if names.Has(status.PodName) && status.MemberJoined == nil {
			status.MemberJoined = ptr.To(true)
		}
	}
	if registered || membershipChanged(original.Status.InstanceStatus, its.Status.InstanceStatus) {
		return kubebuilderx.RetryAfter(time.Second), nil
	}
	return kubebuilderx.Continue, nil
}

func reconcileMember(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pod *corev1.Pod, join bool, reader client.Reader) (kubebuilderx.Result, error) {
	status := its.FindInstanceStatus(pod.Name)
	if status == nil || !memberActionsEnabled(its) || isStopRequested(its) {
		return kubebuilderx.Continue, nil
	}
	if join && (its.Spec.LifecycleActions.MemberJoin == nil || status.MemberJoined == nil || *status.MemberJoined) {
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
			return lifecycleWait(tree, its, err)
		}
		lfa, err := newLifecycleAction(its, tree, pod)
		if err != nil {
			return lifecycleWait(tree, its, err)
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
					return lifecycleWait(tree, its, err)
				}
				if err := lfa.Switchover(tree.Context, reader, opts, ""); err != nil {
					return lifecycleWait(tree, its, err)
				}
				return lifecycleWait(tree, its, fmt.Errorf("waiting for %s to relinquish its exclusive role", pod.Name))
			}
		}
		if join {
			err = lfa.MemberJoin(tree.Context, reader, opts)
		} else {
			err = lfa.MemberLeave(tree.Context, reader, opts)
		}
		if err != nil {
			return lifecycleWait(tree, its, err)
		}
	}
	status.MemberJoined = ptr.To(join)
	meta.RemoveStatusCondition(&its.Status.Conditions, "InstanceLifecycle")
	return kubebuilderx.RetryAfter(time.Second), nil
}

func lifecycleWait(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, err error) (kubebuilderx.Result, error) {
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

func memberRetirementBlocked(its *workloads.InstanceSet, old *workloads.InstanceStatus) bool {
	if !memberActionsEnabled(its) || old == nil {
		return false
	}
	established := ptr.Deref(old.MemberJoined, false) || (old.MemberJoined == nil && old.Provisioned)
	return slices.ContainsFunc(its.Status.InstanceStatus, func(status workloads.InstanceStatus) bool {
		if status.EffectiveDesiredState() != workloads.InstanceDesiredStateActive {
			return false
		}
		pendingJoin := established && its.Spec.LifecycleActions.MemberJoin != nil && status.MemberJoined != nil && !*status.MemberJoined
		return pendingJoin
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
		if action != nil && action.PreCondition != nil && *action.PreCondition != apps.ImmediatelyPreConditionType {
			return fmt.Errorf("workload-owned actions support only Immediately preconditions; runtime availability is observed by the workload controller")
		}
		if action != nil && action.NonBlocking {
			return fmt.Errorf("workload-owned lifecycle actions must use blocking execution")
		}
	}
	return nil
}

// registerMemberStatus records a complete current allocation before any runtime is created.
// Existing resources prevent a lost observation from being classified as a fresh bootstrap.
func registerMemberStatus(its, original *workloads.InstanceSet, assignments []instancetemplate.Assignment, runtimeNames, resourceNames sets.Set[string]) bool {
	if !memberActionsEnabled(its) {
		return false
	}
	bootstrap := original.Status.ObservedGeneration == 0 && len(original.Status.InstanceStatus) == 0 && runtimeNames.Len() == 0 && resourceNames.Len() == 0
	known := sets.New[string]()
	for _, s := range original.Status.InstanceStatus {
		known.Insert(s.PodName)
	}
	changed := false
	for _, a := range assignments {
		if known.Has(a.InstanceName) {
			continue
		}
		status := its.FindInstanceStatus(a.InstanceName)
		if status == nil {
			its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: a.InstanceName, TemplateName: ptr.To(a.TemplateName), DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStateAbsent})
			status = &its.Status.InstanceStatus[len(its.Status.InstanceStatus)-1]
		}
		if !bootstrap && !runtimeNames.Has(a.InstanceName) {
			if its.Spec.LifecycleActions.MemberJoin != nil {
				status.MemberJoined = ptr.To(false)
			}
		}
		changed = true
	}
	return changed
}

func needsMemberLeave(its *workloads.InstanceSet, status *workloads.InstanceStatus) bool {
	if !memberActionsEnabled(its) || its.Spec.LifecycleActions.MemberLeave == nil || status == nil {
		return false
	}
	return ptr.Deref(status.MemberJoined, false) || (status.MemberJoined == nil && status.Provisioned)
}

func needsMemberRetirement(its *workloads.InstanceSet, status *workloads.InstanceStatus) bool {
	if needsMemberLeave(its, status) {
		return true
	}
	return memberActionsEnabled(its) && its.Spec.LifecycleActions.MemberJoin != nil && status != nil && ptr.Deref(status.MemberJoined, false)
}

// membershipChanged reports allocation and membership changes that alignment must commit before proceeding.
func membershipChanged(previous, current []workloads.InstanceStatus) bool {
	if len(previous) != len(current) {
		return true
	}
	for _, status := range current {
		found := false
		for _, old := range previous {
			if status.PodName != old.PodName {
				continue
			}
			found = true
			if status.EffectiveDesiredState() != old.EffectiveDesiredState() || !ptr.Equal(status.TemplateName, old.TemplateName) ||
				!ptr.Equal(status.MemberJoined, old.MemberJoined) {
				return true
			}
			break
		}
		if !found {
			return true
		}
	}
	return false
}

func validateMemberExecution(action *apps.Action, target *corev1.Pod, pods []*corev1.Pod) (*publiclifecycle.Options, error) {
	if target == nil {
		return nil, fmt.Errorf("member target runtime is absent")
	}
	if len(pods) == 0 {
		return nil, fmt.Errorf("no available executor for member action")
	}
	selected, err := publiclifecycle.SelectTargetPods(pods, target, action)
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
		return &publiclifecycle.Options{TargetPodName: selected[0].Name}, nil
	}
	return nil, nil
}
