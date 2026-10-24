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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	workloadlifecycle "github.com/apecloud/kubeblocks/pkg/controller/workloads/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func NewAlignmentReconciler() kubebuilderx.Reconciler {
	return &alignmentReconciler{}
}

type alignmentReconciler struct{}

var _ kubebuilderx.Reconciler = &alignmentReconciler{}

func (r *alignmentReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *alignmentReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its, _ := tree.GetRoot().(*workloads.InstanceSet)
	itsExt, err := instancetemplate.BuildInstanceSetExt(its, tree)
	if err != nil {
		return kubebuilderx.Continue, err
	}

	// 1. build desired name to template map
	nameBuilder, err := instancetemplate.NewPodNameBuilder(
		itsExt, &instancetemplate.PodNameBuilderOpts{EventLogger: tree.EventRecorder},
	)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	nameToTemplateMap, err := nameBuilder.BuildInstanceName2TemplateMap()
	if err != nil {
		return kubebuilderx.Continue, err
	}

	// 2. find the create and delete set
	newNameSet := sets.New[string]()
	for name := range nameToTemplateMap {
		newNameSet.Insert(name)
	}
	oldNameSet := sets.New[string]()
	oldInstanceMap := make(map[string]*workloads.Instance)
	oldInstanceList := tree.List(&workloads.Instance{})
	for _, object := range oldInstanceList {
		oldNameSet.Insert(object.GetName())
		inst, _ := object.(*workloads.Instance)
		oldInstanceMap[object.GetName()] = inst
	}
	createNameSet := newNameSet.Difference(oldNameSet)
	deleteNameSet := oldNameSet.Difference(newNameSet)
	runtimeChanged := false
	for name, inst := range oldInstanceMap {
		if !newNameSet.Has(name) || model.IsObjectDeleting(inst) || !ptr.Deref(inst.Spec.ScaledDown, false) {
			continue
		}
		instCopy := inst.DeepCopy()
		instCopy.Spec.ScaledDown = nil
		if err := tree.Update(instCopy); err != nil {
			return kubebuilderx.Continue, err
		}
		runtimeChanged = true
		oldInstanceMap[name] = instCopy
	}
	oldInstanceList = tree.List(&workloads.Instance{})

	// default OrderedReady policy
	isOrderedReady := true
	concurrency := 0
	if its.Spec.PodManagementPolicy == appsv1.ParallelPodManagement {
		concurrency, err = calculateConcurrencyReplicas(its.Spec.ParallelPodManagementConcurrency, int(*its.Spec.Replicas))
		if err != nil {
			return kubebuilderx.Continue, err
		}
		isOrderedReady = false
	}
	if isStopRequested(its) {
		return r.stopInstances(tree, oldInstanceList, isOrderedReady, concurrency)
	}
	for name, inst := range oldInstanceMap {
		if newNameSet.Has(name) && ptr.Deref(inst.Spec.Stop, false) && !model.IsObjectDeleting(inst) {
			createNameSet.Insert(name)
		}
	}

	// 3. handle alignment (create new instances and delete useless instances)
	newNameList := sets.List(newNameSet)
	baseSort(newNameList, func(i int) (string, int) {
		return parseParentNameAndOrdinal(newNameList[i])
	}, nil, true)
	workloadlifecycle.SortDataSourcesFirst(its, newNameList)
	getPredecessor := func(i int) *workloads.Instance {
		if i <= 0 {
			return nil
		}
		return oldInstanceMap[newNameList[i-1]]
	}
	if !isOrderedReady {
		for _, name := range newNameList {
			if _, ok := createNameSet[name]; !ok {
				if !intctrlutil.IsInstanceAvailable(oldInstanceMap[name]) {
					concurrency--
				}
			} else if inst := oldInstanceMap[name]; inst != nil && !hasObservedStoppedRuntime(inst) {
				concurrency--
			}
		}
	}
	retirementNames := deleteNameSet.Clone()
	retire := func(names sets.Set[string]) (kubebuilderx.Result, error) {
		waiting := false
		priorities := make(map[string]int)
		sortObjects(oldInstanceList, priorities, false)
		for _, object := range oldInstanceList {
			inst, _ := object.(*workloads.Instance)
			if !names.Has(inst.Name) {
				continue
			}
			absentRetirement := hasObservedStoppedRuntime(inst)
			if !absentRetirement && !isOrderedReady && concurrency <= 0 {
				continue
			}
			if isOrderedReady && !intctrlutil.IsInstanceReady(inst) {
				tree.EventRecorder.Eventf(its, corev1.EventTypeWarning, "InstanceSet %s/%s is waiting for Instance %s to be Ready",
					its.Namespace,
					its.Name,
					inst.Name)
			}
			if workloadlifecycle.Enabled(its) && !ptr.Deref(its.Spec.Stop, false) {
				if workloadlifecycle.RetirementBlocked(its, its.FindInstanceStatus(inst.Name)) {
					waiting = true
					continue
				}
			}
			if model.IsObjectDeleting(inst) {
				continue
			}
			scaledDown := ptr.Deref(inst.Spec.ScaledDown, false)
			retentionMatches := equality.Semantic.DeepEqual(inst.Spec.PersistentVolumeClaimRetentionPolicy, its.Spec.PersistentVolumeClaimRetentionPolicy)
			if scaledDown && retentionMatches {
				if !instanceRetirementObserved(inst) {
					waiting = true
					continue
				}
				if err := tree.Delete(inst); err != nil {
					return kubebuilderx.Continue, err
				}
			} else {
				instCopy := inst.DeepCopy()
				instCopy.Spec.ScaledDown = ptr.To(true)
				instCopy.Spec.PersistentVolumeClaimRetentionPolicy = its.Spec.PersistentVolumeClaimRetentionPolicy.DeepCopy()
				if err := tree.Update(instCopy); err != nil {
					return kubebuilderx.Continue, err
				}
			}
			if workloadlifecycle.Enabled(its) {
				return kubebuilderx.RetryAfter(time.Second), nil
			}
			if isOrderedReady && !absentRetirement {
				break
			}
			if !absentRetirement {
				concurrency--
			}
		}

		if waiting {
			return kubebuilderx.RetryAfter(time.Second), nil
		}
		return kubebuilderx.Continue, nil
	}
	if workloadlifecycle.Enabled(its) && !isStopRequested(its) {
		independentNames := sets.New[string]()
		for name := range retirementNames {
			status := its.FindInstanceStatus(name)
			if status != nil && !workloadlifecycle.NeedsLeave(its, status) && ((status.MemberJoined != nil && !*status.MemberJoined) || (status.DataLoaded != nil && !*status.DataLoaded)) {
				independentNames.Insert(name)
			}
		}
		if result, err := retire(independentNames); err != nil || result != kubebuilderx.Continue {
			return result, err
		}
	}

	for i, name := range newNameList {
		if _, ok := createNameSet[name]; !ok {
			continue
		}
		if !isOrderedReady && concurrency <= 0 {
			break
		}
		current := oldInstanceMap[name]
		if current != nil && !hasObservedStoppedRuntime(current) {
			if isOrderedReady {
				break
			}
			continue
		}
		predecessor := getPredecessor(i)
		if isOrderedReady && predecessor != nil && !intctrlutil.IsInstanceAvailable(predecessor) {
			break
		}
		newInst, err := buildInstanceByTemplate(tree, name, nameToTemplateMap[name], its)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if err := lifecycleCreateInstance(tree, its, newInst); err != nil {
			return lifecycleWait(tree, its, err)
		}
		if current != nil {
			merged := copyAndMergeInstance(current, newInst)
			if merged == nil {
				merged = current.DeepCopy()
			}
			merged.Spec.Stop = ptr.To(false)
			if err := tree.Update(merged); err != nil {
				return kubebuilderx.Continue, err
			}
		} else {
			if status := its.FindInstanceStatus(name); status != nil && its.Spec.LifecycleActions != nil && its.Spec.LifecycleActions.MemberJoin != nil {
				newInst.Spec.ScaledUp = ptr.To(status.MemberJoined != nil && !*status.MemberJoined)
			}
			if err := tree.Add(newInst); err != nil {
				return kubebuilderx.Continue, err
			}
		}

		runtimeChanged = true
		if isOrderedReady {
			break
		}
		concurrency--
	}

	if workloadlifecycle.Enabled(its) && runtimeChanged {
		return kubebuilderx.RetryAfter(time.Second), nil
	}
	for _, name := range newNameList {
		if createNameSet.Has(name) {
			continue
		}
		inst := oldInstanceMap[name]
		if !workloadlifecycle.Enabled(its) || model.IsObjectDeleting(inst) || ptr.Deref(inst.Spec.Stop, false) {
			continue
		}
		desired := inst.DeepCopy()
		if err := refreshInstanceData(tree, its, desired); err != nil {
			return lifecycleWait(tree, its, err)
		}
		if !equality.Semantic.DeepEqual(inst.Spec.Template.Spec, desired.Spec.Template.Spec) {
			if err := tree.Update(desired); err != nil {
				return kubebuilderx.Continue, err
			}
			return kubebuilderx.RetryAfter(time.Second), nil
		}
	}

	if result, err := retire(retirementNames); err != nil || result != kubebuilderx.Continue {
		return result, err
	}

	if workloadlifecycle.Enabled(its) {
		meta.RemoveStatusCondition(&its.Status.Conditions, "InstanceLifecycle")
	}
	return kubebuilderx.Continue, nil
}

func (r *alignmentReconciler) stopInstances(tree *kubebuilderx.ObjectTree, instances []client.Object, ordered bool, concurrency int) (kubebuilderx.Result, error) {
	result := kubebuilderx.Continue
	sortObjects(instances, nil, false)
	if !ordered {
		for _, obj := range instances {
			inst := obj.(*workloads.Instance)
			if (model.IsObjectDeleting(inst) || ptr.Deref(inst.Spec.Stop, false)) && !hasObservedStoppedRuntime(inst) {
				concurrency--
			}
		}
	}
	for _, obj := range instances {
		inst := obj.(*workloads.Instance)
		if hasObservedStoppedRuntime(inst) {
			continue
		}
		if ptr.Deref(inst.Spec.Stop, false) {
			if ordered {
				break
			}
			continue
		}
		deleting := model.IsObjectDeleting(inst)
		if !deleting && !ordered && concurrency <= 0 {
			continue
		}
		next := inst.DeepCopy()
		next.Spec.Stop = ptr.To(true)
		if err := tree.Update(next); err != nil {
			return kubebuilderx.Continue, err
		}
		// Reload the API-assigned generation before evaluating Stop completion.
		result = kubebuilderx.RetryAfter(time.Second)
		if ordered {
			break
		}
		if !deleting {
			concurrency--
		}
	}
	return result, nil
}
