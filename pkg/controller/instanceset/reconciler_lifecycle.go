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
	"slices"
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
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	workloadlifecycle "github.com/apecloud/kubeblocks/pkg/controller/workloads/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type lifecycleReconciler struct {
	client   client.Reader
	register bool
}

func NewLifecycleRegistrationReconciler() kubebuilderx.Reconciler {
	return &lifecycleReconciler{register: true}
}
func NewLifecycleReconciler(reader client.Reader) kubebuilderx.Reconciler {
	return &lifecycleReconciler{client: reader}
}

func (r *lifecycleReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) || model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	its := tree.GetRoot().(*workloads.InstanceSet)
	if !workloadlifecycle.Enabled(its) || isStopRequested(its) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *lifecycleReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
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
	if r.register {
		if workloadlifecycle.HasData(its) {
			templates, err := desiredInstanceTemplates(its, tree)
			if err != nil {
				return kubebuilderx.Continue, err
			}
			for name, template := range templates {
				pod, err := buildInstancePodByTemplate(name, template, its, "")
				if err != nil {
					return lifecycleWait(tree, its, err)
				}
				pvcName, err := workloadlifecycle.DataPVCName(its, pod)
				if err != nil {
					return lifecycleWait(tree, its, err)
				}
				pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: its.Namespace}}
				obj, err := tree.Get(pvc)
				if err != nil {
					return kubebuilderx.Continue, err
				}
				if obj != nil {
					resources.Insert(name)
				}
			}
		}
		if workloadlifecycle.Register(its, assignments, names, resources) {
			return kubebuilderx.RetryAfter(time.Second), nil
		}
		return kubebuilderx.Continue, nil
	}
	desired := sets.New[string]()
	for _, a := range assignments {
		desired.Insert(a.InstanceName)
	}
	// Read the actual completion fact first and commit it alone before cleanup or a member action.
	for i := range its.Status.InstanceStatus {
		status := &its.Status.InstanceStatus[i]
		if !workloadlifecycle.HasData(its) || !desired.Has(status.PodName) {
			continue
		}
		pod := findLifecyclePod(pods, status.PodName)
		if pod == nil {
			ext, err := instancetemplate.BuildInstanceSetExt(its, tree)
			if err != nil {
				return kubebuilderx.Continue, err
			}
			nb, err := instancetemplate.NewPodNameBuilder(ext, nil)
			if err != nil {
				return kubebuilderx.Continue, err
			}
			templates, err := nb.BuildInstanceName2TemplateMap()
			if err != nil {
				return kubebuilderx.Continue, err
			}
			if template := templates[status.PodName]; template != nil {
				pod, err = buildInstancePodByTemplate(status.PodName, template, its, "")
				if err != nil {
					return lifecycleWait(tree, its, err)
				}
			}
		}
		if pod == nil {
			continue
		}
		pvcName, err := workloadlifecycle.DataPVCName(its, pod)
		if err != nil {
			return lifecycleWait(tree, its, err)
		}
		obj, err := tree.Get(&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Namespace: pod.Namespace, Name: pvcName}})
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if obj == nil {
			continue
		}
		if workloadlifecycle.ObserveData(status, obj.(*corev1.PersistentVolumeClaim)) {
			return kubebuilderx.RetryAfter(time.Second), nil
		}
	}
	for i := range its.Status.InstanceStatus {
		status := &its.Status.InstanceStatus[i]
		pod := findLifecyclePod(pods, status.PodName)
		active := desired.Has(status.PodName)
		join := active && its.Spec.LifecycleActions.MemberJoin != nil && status.MemberJoined != nil && !*status.MemberJoined
		leave := !active && !slices.Contains(its.Spec.OfflineInstances, status.PodName) && workloadlifecycle.NeedsLeave(its, status)
		if !join && !leave {
			continue
		}
		if join && ((workloadlifecycle.HasData(its) && status.DataLoaded != nil && !*status.DataLoaded) || pod == nil || !intctrlutil.IsPodAvailable(pod, its.Spec.MinReadySeconds)) {
			continue
		}
		action := its.Spec.LifecycleActions.MemberJoin
		if leave {
			action = its.Spec.LifecycleActions.MemberLeave
		}
		if err := workloadlifecycle.ValidateExecution(action, pod, pods); err != nil {
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
				if err := workloadlifecycle.ValidateExecution(its.Spec.LifecycleActions.Switchover, pod, pods); err != nil {
					return lifecycleWait(tree, its, err)
				}
				if err := lfa.Switchover(tree.Context, r.client, nil, ""); err != nil {
					return lifecycleWait(tree, its, err)
				}
				return kubebuilderx.RetryAfter(time.Second), nil
			}
		}
		if join {
			err = lfa.MemberJoin(tree.Context, r.client, nil)
		} else {
			err = lfa.MemberLeave(tree.Context, r.client, nil)
		}
		if err != nil {
			return lifecycleWait(tree, its, err)
		}
		status.MemberJoined = ptr.To(join)
		meta.RemoveStatusCondition(&its.Status.Conditions, "InstanceLifecycle")
		return kubebuilderx.RetryAfter(time.Second), nil
	}
	meta.RemoveStatusCondition(&its.Status.Conditions, "InstanceLifecycle")
	return kubebuilderx.Continue, nil
}

func findLifecyclePod(pods []*corev1.Pod, name string) *corev1.Pod {
	for _, pod := range pods {
		if pod.Name == name {
			return pod
		}
	}
	return nil
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
	pvcName, err := workloadlifecycle.DataPVCName(its, pod)
	if err != nil {
		return err
	}
	obj, err := tree.Get(&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Namespace: pod.Namespace, Name: pvcName}})
	if err != nil {
		return err
	}
	// An unread PVC retains the previous result and startup check. A newly observed empty PVC regenerates the task.
	complete := ptr.Deref(status.DataLoaded, false)
	if obj != nil {
		complete = obj.GetAnnotations()["workloads.kubeblocks.io/data-loaded"] == "true"
	}
	var source *corev1.Pod
	if !complete {
		var pods []*corev1.Pod
		for _, obj := range tree.List(&corev1.Pod{}) {
			pods = append(pods, obj.(*corev1.Pod))
		}
		source, err = workloadlifecycle.SourcePod(its, pods, pod.Name)
		if err != nil {
			return err
		}
	}
	return workloadlifecycle.ConfigurePod(its, pod, source, true, complete)
}
