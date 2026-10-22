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

package instanceset2

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	workloadlifecycle "github.com/apecloud/kubeblocks/pkg/controller/workloads/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const lifecycleCondition = "InstanceLifecycle"

var newMemberLifecycle = workloadlifecycle.NewLifecycle

func observeLifecycleStatus(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its := tree.GetRoot().(*workloads.InstanceSet)
	assignments, _, err := instancetemplate.BuildAssignments(tree, its)
	if err != nil {
		if instancetemplate.IsAssignmentIncomplete(err) {
			return lifecycleWait(tree, its, err)
		}
		return kubebuilderx.Continue, err
	}
	runtimeNames, resourceNames := sets.New[string](), sets.New[string]()
	pods := lifecyclePods(tree)
	for _, pod := range pods {
		runtimeNames.Insert(pod.Name)
		resourceNames.Insert(pod.Name)
	}
	instances := []*workloads.Instance{}
	for _, obj := range tree.List(&workloads.Instance{}) {
		inst := obj.(*workloads.Instance)
		instances = append(instances, inst)
		resourceNames.Insert(inst.Name)
		if inst.Status.CurrentState == workloads.InstanceCurrentStatePresent || inst.Status.CurrentState == workloads.InstanceCurrentStateTerminating {
			runtimeNames.Insert(inst.Name)
		}
	}
	registered := workloadlifecycle.Register(its, assignments, runtimeNames, resourceNames)
	previous := its.DeepCopy().Status.InstanceStatus
	childStatusChanged := false
	for _, inst := range instances {
		if status := instanceLifecycleStatus(its, inst.Name); status != nil {
			status.Provisioned = status.Provisioned || inst.Status.Provisioned || runtimeNames.Has(inst.Name)
			if its.Spec.LifecycleActions.MemberJoin != nil && ptr.Deref(status.MemberJoined, true) && inst.Status.MemberJoined == nil && runtimeNames.Has(inst.Name) {
				current := inst.DeepCopy()
				current.Status.MemberJoined = ptr.To(true)
				if err := tree.Update(current, kubebuilderx.WithSubResource("status")); err != nil {
					return kubebuilderx.Continue, err
				}
				inst = current
				childStatusChanged = true
			}
			if inst.Status.MemberJoined != nil {
				status.MemberJoined = ptr.To(*inst.Status.MemberJoined)
			}
		}
	}

	if registered || childStatusChanged || workloadlifecycle.CriticalStatusChanged(previous, its.Status.InstanceStatus) {
		return kubebuilderx.RetryAfter(time.Second), nil
	}
	return kubebuilderx.Continue, nil
}

func reconcileMember(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, inst *workloads.Instance, join bool, reader client.Reader) (kubebuilderx.Result, error) {
	status := instanceLifecycleStatus(its, inst.Name)
	if status == nil || !workloadlifecycle.Enabled(its) || isStopRequested(its) {
		return kubebuilderx.Continue, nil
	}
	if join && (its.Spec.LifecycleActions.MemberJoin == nil || status.MemberJoined == nil || *status.MemberJoined) {
		return kubebuilderx.Continue, nil
	}
	if !join && !workloadlifecycle.NeedsMemberRetirement(its, status) {
		return kubebuilderx.Continue, nil
	}
	if join || its.Spec.LifecycleActions.MemberLeave != nil {
		pods := lifecyclePods(tree)
		leave := !join
		if join {
			current, err := tree.Get(&workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: status.PodName, Namespace: its.Namespace}})
			if err != nil {
				return kubebuilderx.Continue, err
			}
			if current == nil || !current.GetDeletionTimestamp().IsZero() || ptr.Deref(current.(*workloads.Instance).Spec.Stop, false) {
				return kubebuilderx.Continue, nil
			}
		}
		var pod *corev1.Pod
		for _, p := range pods {
			if p.Name == status.PodName {
				pod = p
				break
			}
		}
		if join && (pod == nil || !pod.DeletionTimestamp.IsZero()) {
			return lifecycleWait(tree, its, fmt.Errorf("waiting for runtime of member %s", status.PodName))
		}
		if pod == nil {
			pod = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: inst.Name, Namespace: inst.Namespace}}
		}
		if join && !intctrlutil.IsPodAvailable(pod, its.Spec.MinReadySeconds) {
			return kubebuilderx.Continue, nil
		}
		action := its.Spec.LifecycleActions.MemberJoin
		if leave {
			action = its.Spec.LifecycleActions.MemberLeave
		}
		opts, err := workloadlifecycle.ValidateExecution(action, pod, pods)
		if err != nil {
			return lifecycleWait(tree, its, err)
		}
		lfa, err := newMemberLifecycle(its, pod, pods)
		if err != nil {
			return lifecycleWait(tree, its, err)
		}
		if leave && its.Spec.LifecycleActions.Switchover != nil && exclusiveRole(its, pod) {
			opts, err := workloadlifecycle.ValidateExecution(its.Spec.LifecycleActions.Switchover, pod, pods)
			if err != nil {
				return lifecycleWait(tree, its, err)
			}
			if err := lfa.Switchover(tree.Context, reader, opts, ""); err != nil {
				return lifecycleWait(tree, its, err)
			}
			return lifecycleWait(tree, its, fmt.Errorf("waiting for %s to relinquish its exclusive role", pod.Name))
		}
		if leave {
			err = lfa.MemberLeave(tree.Context, reader, opts)
		} else {
			err = lfa.MemberJoin(tree.Context, reader, opts)
		}
		if err != nil {
			return lifecycleWait(tree, its, err)
		}
	}
	current := inst.DeepCopy()
	current.Status.MemberJoined = ptr.To(join)
	if err := tree.Update(current, kubebuilderx.WithSubResource("status")); err != nil {
		return kubebuilderx.Continue, err
	}
	status.MemberJoined = ptr.To(*current.Status.MemberJoined)
	meta.RemoveStatusCondition(&its.Status.Conditions, lifecycleCondition)
	return kubebuilderx.RetryAfter(time.Second), nil
}

func lifecycleWait(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, err error) (kubebuilderx.Result, error) {
	previous := meta.FindStatusCondition(its.Status.Conditions, lifecycleCondition)
	if tree.EventRecorder != nil && (previous == nil || previous.Message != err.Error()) {
		tree.EventRecorder.Eventf(its, corev1.EventTypeWarning, "LifecycleWaiting", "%s", err.Error())
	}
	meta.SetStatusCondition(&its.Status.Conditions, metav1.Condition{Type: lifecycleCondition, Status: metav1.ConditionFalse, ObservedGeneration: its.Generation, Reason: "Waiting", Message: err.Error()})
	return kubebuilderx.RetryAfter(time.Second), nil
}
func lifecyclePods(tree *kubebuilderx.ObjectTree) []*corev1.Pod {
	pods := []*corev1.Pod{}
	for _, obj := range tree.List(&corev1.Pod{}) {
		pods = append(pods, obj.(*corev1.Pod))
	}
	return pods
}
func instanceLifecycleStatus(its *workloads.InstanceSet, name string) *workloads.InstanceStatus {
	for i := range its.Status.InstanceStatus {
		if its.Status.InstanceStatus[i].PodName == name {
			return &its.Status.InstanceStatus[i]
		}
	}
	return nil
}
func exclusiveRole(its *workloads.InstanceSet, pod *corev1.Pod) bool {
	for _, role := range its.Spec.Roles {
		if role.Name == pod.Labels[constant.RoleLabelKey] {
			return role.IsExclusive
		}
	}
	return false
}

func configureInstance(its *workloads.InstanceSet, inst *workloads.Instance) error {
	pod := &corev1.Pod{ObjectMeta: inst.ObjectMeta, Spec: inst.Spec.Template.Spec}
	if err := workloadlifecycle.ConfigurePod(its, pod); err != nil {
		return err
	}
	inst.Spec.Template.Spec = pod.Spec
	return nil
}

func adoptLeaveAction(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, inst *workloads.Instance) (kubebuilderx.Result, error) {
	pods := lifecyclePods(tree)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: inst.Name, Namespace: inst.Namespace}}
	for _, current := range pods {
		if current.Name == inst.Name {
			pod = current
		}
	}
	targets, err := workloadlifecycle.ActionTargets(its.Spec.LifecycleActions.MemberLeave, pod, pods)
	if err != nil {
		return lifecycleWait(tree, its, err)
	}
	required := sets.New[string]()
	missing := false
	for _, target := range targets {
		required.Insert(target.Name)
		configured, err := workloadlifecycle.ActionConfigured(its, target, "memberLeave")
		if err != nil {
			return kubebuilderx.Continue, err
		}
		missing = missing || !configured
	}
	if !missing {
		return kubebuilderx.Continue, nil
	}
	desired := map[string]*workloads.Instance{}
	for _, obj := range tree.List(&workloads.Instance{}) {
		current := obj.(*workloads.Instance)
		copy := current.DeepCopy()
		if required.Has(current.Name) {
			copy.Spec.LifecycleActions = its.Spec.LifecycleActions.DeepCopy()
			copy.Spec.InstanceUpdateStrategyType = nil
			if its.Spec.InstanceUpdateStrategy != nil {
				copy.Spec.InstanceUpdateStrategyType = ptr.To(its.Spec.InstanceUpdateStrategy.Type)
			}
			copy.Spec.PodUpdatePolicy = its.Spec.PodUpdatePolicy
			copy.Spec.PodUpgradePolicy = its.Spec.PodUpgradePolicy
			if err := configureInstance(its, copy); err != nil {
				return kubebuilderx.Continue, err
			}
			stampInstanceRevision(copy)
		}
		desired[copy.Name] = copy
	}
	satisfied := func(current *workloads.Instance) bool {
		if !required.Has(current.Name) {
			return true
		}
		if getInstanceRevision(current) != getInstanceRevision(desired[current.Name]) || current.Generation != current.Status.ObservedGeneration || !current.Status.UpToDate {
			return false
		}
		for _, pod := range pods {
			if pod.Name == current.Name {
				configured, err := workloadlifecycle.ActionConfigured(its, pod, "memberLeave")
				return err == nil && configured
			}
		}
		return false
	}
	if _, err := (&updateReconciler{}).reconcileUpdates(tree, desired, satisfied); err != nil {
		return kubebuilderx.Continue, err
	}
	return lifecycleWait(tree, its, fmt.Errorf("waiting for memberLeave runtime configuration"))
}
