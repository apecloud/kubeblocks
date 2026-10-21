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
	if retained, ok := tree.Context.Value(retainedResourcesKey{}).(sets.Set[string]); ok {
		resourceNames.Insert(retained.UnsortedList()...)
	}
	registered := workloadlifecycle.Register(its, assignments, runtimeNames, resourceNames)
	previous := its.DeepCopy().Status.InstanceStatus
	for _, inst := range instances {
		if status := instanceLifecycleStatus(its, inst.Name); status != nil {
			status.Provisioned = status.Provisioned || inst.Status.Provisioned
			if inst.Status.DataLoaded != nil && (!ptr.Deref(status.DataLoaded, false) || *inst.Status.DataLoaded) {
				status.DataLoaded = ptr.To(*inst.Status.DataLoaded)
			}
			if inst.Status.MemberJoined != nil {
				status.MemberJoined = ptr.To(*inst.Status.MemberJoined)
			}
		}
	}

	if registered || workloadlifecycle.CriticalStatusChanged(previous, its.Status.InstanceStatus) {
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
	if !join && !workloadlifecycle.NeedsLeave(its, status) {
		return kubebuilderx.Continue, nil
	}
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
	if join && status.DataLoaded != nil && !*status.DataLoaded {
		return kubebuilderx.Continue, nil
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
		// Leave may run on a peer after Stop removed the member's Pod.
		pod = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: inst.Name, Namespace: inst.Namespace},
			Spec: corev1.PodSpec{Hostname: inst.Spec.Template.Spec.Hostname, Subdomain: inst.Spec.Template.Spec.Subdomain}}
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
	// Commit the Instance result before aggregation or cleanup; Update checks its resource version.
	current := inst.DeepCopy()
	current.Status.MemberJoined = ptr.To(!leave)
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

func refreshInstanceData(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, inst *workloads.Instance) error {
	applied := its.DeepCopy()
	applied.Spec.LifecycleActions = inst.Spec.LifecycleActions
	return lifecycleCreateInstance(tree, applied, inst)
}

func configureInstance(its *workloads.InstanceSet, inst *workloads.Instance) error {
	pod := &corev1.Pod{ObjectMeta: inst.ObjectMeta, Spec: inst.Spec.Template.Spec}
	if err := workloadlifecycle.ConfigurePod(its, pod); err != nil {
		return err
	}
	if _, err := workloadlifecycle.CleanTask(&pod.Spec, inst.Name); err != nil {
		return err
	}
	inst.Spec.Template.Spec = pod.Spec
	return nil
}

func lifecycleCreateInstance(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, inst *workloads.Instance) error {
	if !workloadlifecycle.HasData(its) {
		return nil
	}
	status := instanceLifecycleStatus(its, inst.Name)
	completed := status == nil || status.DataLoaded == nil || *status.DataLoaded
	currentSpec := &inst.Spec.Template.Spec
	if current, err := tree.Get(inst); err != nil {
		return err
	} else if current != nil {
		child := current.(*workloads.Instance)
		completed = completed || ptr.Deref(child.Status.DataLoaded, false)
		currentSpec = &child.Spec.Template.Spec
	}
	if completed {
		if _, err := workloadlifecycle.CleanTask(&inst.Spec.Template.Spec, inst.Name); err != nil {
			return err
		}
		for i := range inst.Spec.InstanceAssistantObjects {
			if cm := inst.Spec.InstanceAssistantObjects[i].ConfigMap; cm != nil {
				if _, err := workloadlifecycle.CleanTaskConfigMap(cm, inst.Name); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if ptr.Deref(its.Spec.Stop, false) {
		return nil
	}
	pod := &corev1.Pod{ObjectMeta: inst.ObjectMeta, Spec: inst.Spec.Template.Spec}
	source, err := workloadlifecycle.SourcePod(its, lifecyclePods(tree), inst.Name, currentSpec)
	if err != nil {
		return err
	}
	if err := workloadlifecycle.ConfigureDataTask(its, pod, source); err != nil {
		return err
	}
	inst.Spec.Template.Spec = pod.Spec
	return nil
}
