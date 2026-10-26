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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	workloadlifecycle "github.com/apecloud/kubeblocks/pkg/controller/workloads/lifecycle"
)

const lifecycleCondition = "InstanceLifecycle"

func observeLifecycleStatus(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its := tree.GetRoot().(*workloads.InstanceSet)
	assignments, _, err := instancetemplate.BuildAssignments(tree, its)
	if err != nil {
		if instancetemplate.IsAssignmentIncomplete(err) {
			if its.Generation != its.Status.ObservedGeneration {
				return kubebuilderx.Continue, nil
			}
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
	if claimsByInstance, ok := tree.Context.Value(lifecyclePVCsKey{}).(map[string][]*corev1.PersistentVolumeClaim); ok {
		for name := range claimsByInstance {
			resourceNames.Insert(name)
		}
	}
	registered := workloadlifecycle.Register(its, assignments, runtimeNames, resourceNames)
	previous := its.DeepCopy().Status.InstanceStatus
	for _, inst := range instances {
		if status := its.FindInstanceStatus(inst.Name); status != nil {
			status.Provisioned = status.Provisioned || inst.Status.Provisioned || runtimeNames.Has(inst.Name)
			if inst.Status.DataLoaded != nil {
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
	status := its.FindInstanceStatus(inst.Name)
	completed := status == nil || status.DataLoaded == nil || *status.DataLoaded
	currentSpec := &inst.Spec.Template.Spec
	if current, err := tree.Get(inst); err != nil {
		return err
	} else if current != nil {
		child := current.(*workloads.Instance)
		completed = ptr.Deref(child.Status.DataLoaded, completed)
		currentSpec = &child.Spec.Template.Spec
	}
	if completed {
		if _, err := workloadlifecycle.CleanTask(&inst.Spec.Template.Spec, inst.Name); err != nil {
			return err
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

func instanceRetirementObserved(inst *workloads.Instance) bool {
	if inst.Status.ObservedGeneration != inst.Generation {
		return false
	}
	if !instanceMemberActionsEnabled(inst) {
		return true
	}
	return inst.Status.MemberJoined != nil && !*inst.Status.MemberJoined
}

func needsRetirementObservation(its *workloads.InstanceSet, inst *workloads.Instance) bool {
	return workloadlifecycle.NeedsLeave(its, its.FindInstanceStatus(inst.Name)) &&
		inst.Status.MemberJoined == nil && !inst.Status.Provisioned
}

func instanceMemberActionsEnabled(inst *workloads.Instance) bool {
	members := inst.Spec.LifecycleActions
	return members != nil && (members.MemberJoin != nil || members.MemberLeave != nil)
}

func restoreRequiredDataSources(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet) (bool, error) {
	for _, status := range its.Status.InstanceStatus {
		if status.EffectiveDesiredState() == workloads.InstanceDesiredStateActive || !workloadlifecycle.DataSourceRequired(its, &status) {
			continue
		}
		key := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Namespace: its.Namespace, Name: status.PodName}}
		obj, err := tree.Get(key)
		if err != nil {
			return false, err
		}
		if obj == nil || model.IsObjectDeleting(obj) {
			continue
		}
		inst := obj.(*workloads.Instance)
		if !ptr.Deref(inst.Spec.Stop, false) || !ptr.Deref(inst.Status.DataLoaded, true) || !ptr.Deref(inst.Status.MemberJoined, true) {
			continue
		}
		instCopy := inst.DeepCopy()
		instCopy.Spec.Stop = ptr.To(false)
		return true, tree.Update(instCopy)
	}
	return false, nil
}
