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

	apps "github.com/apecloud/kubeblocks/apis/apps/v1"
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
	desired := sets.New[string]()
	for _, a := range assignments {
		desired.Insert(a.InstanceName)
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
		name := obj.GetLabels()[constant.KBAppPodNameLabelKey]
		status := workloadlifecycle.Status(its, name)
		if workloadlifecycle.HasData(its) && desired.Has(name) && status != nil && status.EffectiveDesiredState() == workloads.InstanceDesiredStateReleased {
			if !obj.GetDeletionTimestamp().IsZero() {
				return lifecycleWait(tree, its, fmt.Errorf("waiting for storage of released instance %s to be deleted", name))
			}
			retention := its.Spec.PersistentVolumeClaimRetentionPolicy
			if !isStopRequested(its) && !names.Has(name) && metav1.IsControlledBy(obj, its) && (retention == nil || retention.WhenScaled != apps.RetainPersistentVolumeClaimRetentionPolicyType) {
				if err := tree.Delete(obj); err != nil {
					return kubebuilderx.Continue, err
				}
				return lifecycleWait(tree, its, fmt.Errorf("waiting for storage of released instance %s to be deleted", name))
			}
		}
		resources.Insert(name)
	}
	registered := workloadlifecycle.Register(its, assignments, names, resources)
	previous := its.DeepCopy().Status.InstanceStatus
	for _, pod := range pods {
		if status := workloadlifecycle.Status(its, pod.Name); status != nil {
			status.Provisioned = true
			if its.Spec.LifecycleActions.MemberJoin != nil && status.MemberJoined == nil {
				status.MemberJoined = ptr.To(true)
			}
		}
	}
	for _, pod := range pods {
		status := workloadlifecycle.Status(its, pod.Name)
		if workloadlifecycle.HasData(its) && desired.Has(pod.Name) {
			task, err := workloadlifecycle.DataTask(&pod.Spec, pod.Name)
			if err != nil {
				return kubebuilderx.Continue, err
			}
			if status != nil && status.DataLoaded == nil && task == nil {
				status.DataLoaded = ptr.To(true)
			}
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
	if !join && !workloadlifecycle.NeedsMemberRetirement(its, status) {
		return kubebuilderx.Continue, nil
	}
	if join || its.Spec.LifecycleActions.MemberLeave != nil {
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

func adoptLeaveAction(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pod *corev1.Pod) (kubebuilderx.Result, error) {
	pods := []*corev1.Pod{}
	for _, obj := range tree.List(&corev1.Pod{}) {
		pods = append(pods, obj.(*corev1.Pod))
	}
	targets, err := workloadlifecycle.ActionTargets(its.Spec.LifecycleActions.MemberLeave, pod, pods)
	if err != nil {
		return lifecycleWait(tree, its, err)
	}
	if len(pods) == 0 && len(targets) == 0 {
		targets = []*corev1.Pod{pod}
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
	return adoptActionConfiguration(tree, its, targets, required, "memberLeave")
}

func adoptActionConfiguration(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, targets []*corev1.Pod, required sets.Set[string], action string) (kubebuilderx.Result, error) {
	pods := []*corev1.Pod{}
	for _, obj := range tree.List(&corev1.Pod{}) {
		pods = append(pods, obj.(*corev1.Pod))
	}
	itsExt, err := instancetemplate.BuildInstanceSetExt(its, tree)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	if _, ok := itsExt.InstanceTemplates[instancetemplate.DefaultTemplateName]; !ok {
		itsExt.InstanceTemplates[instancetemplate.DefaultTemplateName] = &workloads.InstanceTemplate{}
	}
	templates := map[string]*instancetemplate.InstanceTemplateExt{}
	for _, template := range instancetemplate.BuildInstanceTemplateExt(itsExt) {
		templates[template.Name] = template
	}
	participants := map[string]*instancetemplate.InstanceTemplateExt{}
	for _, current := range pods {
		templateName, ok := instancetemplate.TemplateNameFromLabels(current.Labels)
		if !ok {
			if status := workloadlifecycle.Status(its, current.Name); status != nil && status.TemplateName != nil {
				templateName = *status.TemplateName
			}
		}
		participants[current.Name] = templates[templateName]
		if participants[current.Name] == nil {
			return lifecycleWait(tree, its, fmt.Errorf("waiting for template of action executor %s", current.Name))
		}
	}
	for _, target := range targets {
		if _, ok := participants[target.Name]; ok {
			continue
		}
		status := workloadlifecycle.Status(its, target.Name)
		if status == nil || status.TemplateName == nil || templates[*status.TemplateName] == nil {
			return lifecycleWait(tree, its, fmt.Errorf("waiting for template of action executor %s", target.Name))
		}
		replacement, err := buildInstancePodByTemplate(target.Name, templates[*status.TemplateName], its, "")
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if err := tree.Add(replacement); err != nil {
			return kubebuilderx.Continue, err
		}
		return lifecycleWait(tree, its, fmt.Errorf("waiting for action runtime of %s", target.Name))
	}
	satisfied := func(current *corev1.Pod) (bool, error) {
		if !required.Has(current.Name) {
			return true, nil
		}
		return workloadlifecycle.ActionConfigured(its, current, action)
	}
	if _, err := (&updateReconciler{}).reconcileUpdates(tree, participants, satisfied); err != nil {
		return kubebuilderx.Continue, err
	}
	return lifecycleWait(tree, its, fmt.Errorf("waiting for %s runtime configuration", action))
}

func adoptDataActions(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, target string, cause error) (kubebuilderx.Result, error) {
	pods := []*corev1.Pod{}
	for _, obj := range tree.List(&corev1.Pod{}) {
		pods = append(pods, obj.(*corev1.Pod))
	}
	targets, err := workloadlifecycle.DataSources(its, pods, target)
	if err != nil {
		return lifecycleWait(tree, its, err)
	}
	required := sets.New[string]()
	for _, pod := range targets {
		configured, err := workloadlifecycle.ActionConfigured(its, pod, "dataDump")
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if !configured {
			required.Insert(pod.Name)
		}
	}
	if required.Len() == 0 {
		return lifecycleWait(tree, its, cause)
	}
	return adoptActionConfiguration(tree, its, targets, required, "dataDump")
}
