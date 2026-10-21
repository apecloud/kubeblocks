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

func observeLifecycleStatus(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its := tree.GetRoot().(*workloads.InstanceSet)
	if err := workloadlifecycle.Validate(its); err != nil {
		return lifecycleWait(tree, its, err)
	}
	assignments, _, err := instancetemplate.BuildAssignments(tree, its)
	if err != nil {
		if instancetemplate.IsAssignmentIncomplete(err) {
			return lifecycleWait(tree, its, err)
		}
		return kubebuilderx.Continue, err
	}
	pods := make([]*corev1.Pod, 0)
	names := sets.New[string]()
	resources := sets.New[string]()
	for _, obj := range tree.List(&corev1.Pod{}) {
		pod := obj.(*corev1.Pod)
		pods = append(pods, pod)
		names.Insert(pod.Name)
	}
	for _, obj := range tree.List(&corev1.PersistentVolumeClaim{}) {
		resources.Insert(obj.GetLabels()[constant.KBAppPodNameLabelKey])
	}
	registered := workloadlifecycle.Register(its, assignments, names, resources)
	desired := sets.New[string]()
	for _, a := range assignments {
		desired.Insert(a.InstanceName)
	}
	// Read the actual completion fact first and commit it alone before cleanup or a member action.
	previous := its.DeepCopy().Status.InstanceStatus
	for _, pod := range pods {
		if status := workloadlifecycle.Status(its, pod.Name); status != nil {
			status.Provisioned = true
		}
	}
	for _, pod := range pods {
		status := workloadlifecycle.Status(its, pod.Name)
		if workloadlifecycle.HasData(its) && desired.Has(pod.Name) {
			if _, err := workloadlifecycle.ObserveData(status, pod); err != nil {
				return kubebuilderx.Continue, err
			}
		}
	}

	if registered || workloadlifecycle.CriticalStatusChanged(previous, its.Status.InstanceStatus) {
		return kubebuilderx.RetryAfter(time.Second), nil
	}
	return kubebuilderx.Continue, nil
}

func reconcileMember(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pod *corev1.Pod, join bool, reader client.Reader) (kubebuilderx.Result, error) {
	status := workloadlifecycle.Status(its, pod.Name)
	if status == nil || !workloadlifecycle.Enabled(its) || isStopRequested(its) {
		return kubebuilderx.Continue, nil
	}
	if join && (its.Spec.LifecycleActions.MemberJoin == nil || status.MemberJoined == nil || *status.MemberJoined) {
		return kubebuilderx.Continue, nil
	}
	if !join && !workloadlifecycle.NeedsLeave(its, status) {
		return kubebuilderx.Continue, nil
	}
	pods := make([]*corev1.Pod, 0)
	for _, obj := range tree.List(&corev1.Pod{}) {
		pods = append(pods, obj.(*corev1.Pod))
	}
	leave := !join
	if join && (!pod.DeletionTimestamp.IsZero() || (workloadlifecycle.HasData(its) && status.DataLoaded != nil && !*status.DataLoaded) || !intctrlutil.IsPodAvailable(pod, its.Spec.MinReadySeconds)) {
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
			opts, err := workloadlifecycle.ValidateExecution(its.Spec.LifecycleActions.Switchover, pod, pods)
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
	status.MemberJoined = ptr.To(join)
	meta.RemoveStatusCondition(&its.Status.Conditions, "InstanceLifecycle")
	return kubebuilderx.RetryAfter(time.Second), nil
}

func lifecycleWait(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, err error) (kubebuilderx.Result, error) {
	meta.SetStatusCondition(&its.Status.Conditions, metav1.Condition{Type: "InstanceLifecycle", Status: metav1.ConditionFalse, ObservedGeneration: its.Generation, Reason: "Waiting", Message: err.Error()})
	if tree.EventRecorder != nil {
		tree.EventRecorder.Event(its, corev1.EventTypeWarning, "InstanceLifecycle", err.Error())
	}
	return kubebuilderx.RetryAfter(2 * time.Second), nil
}

func lifecycleCreatePod(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pod *corev1.Pod) error {
	status := workloadlifecycle.Status(its, pod.Name)
	if status == nil || status.DataLoaded == nil || !workloadlifecycle.HasData(its) {
		return nil
	}
	if *status.DataLoaded {
		_, err := workloadlifecycle.CleanTask(&pod.Spec, pod.Name)
		return err
	}
	var pods []*corev1.Pod
	for _, obj := range tree.List(&corev1.Pod{}) {
		pods = append(pods, obj.(*corev1.Pod))
	}
	source, err := workloadlifecycle.SourcePod(its, pods, pod.Name, &pod.Spec)
	if err != nil {
		return err
	}
	return workloadlifecycle.ConfigureDataTask(its, pod, source)
}
