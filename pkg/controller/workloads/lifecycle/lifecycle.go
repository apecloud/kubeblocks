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

// Package lifecycle provides InstanceSet membership policy and action helpers.
package lifecycle

import (
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	apps "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	publiclifecycle "github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
)

func Enabled(its *workloads.InstanceSet) bool {
	a := its.Spec.LifecycleActions
	return a != nil && (a.MemberJoin != nil || a.MemberLeave != nil)
}

func RetirementBlocked(its *workloads.InstanceSet, old *workloads.InstanceStatus) bool {
	if !Enabled(its) || old == nil {
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

func Validate(its *workloads.InstanceSet) error {
	if !Enabled(its) {
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

// Register records a complete current allocation before any runtime is created.
// Existing resources prevent a lost observation from being classified as a fresh bootstrap.
func Register(its, original *workloads.InstanceSet, assignments []instancetemplate.Assignment, runtimeNames, resourceNames sets.Set[string]) bool {
	if !Enabled(its) {
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

func NeedsLeave(its *workloads.InstanceSet, status *workloads.InstanceStatus) bool {
	if !Enabled(its) || its.Spec.LifecycleActions.MemberLeave == nil || status == nil {
		return false
	}
	return ptr.Deref(status.MemberJoined, false) || (status.MemberJoined == nil && status.Provisioned)
}

func NeedsMemberRetirement(its *workloads.InstanceSet, status *workloads.InstanceStatus) bool {
	if NeedsLeave(its, status) {
		return true
	}
	return Enabled(its) && its.Spec.LifecycleActions.MemberJoin != nil && status != nil && ptr.Deref(status.MemberJoined, false)
}

// MembershipChanged reports allocation and membership changes that alignment must commit before proceeding.
func MembershipChanged(previous, current []workloads.InstanceStatus) bool {
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

func ValidateExecution(action *apps.Action, target *corev1.Pod, pods []*corev1.Pod) (*publiclifecycle.Options, error) {
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
