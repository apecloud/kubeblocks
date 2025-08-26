/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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

	// 3. handle alignment (create new instances and delete useless instances)
	// create new instances
	newNameList := sets.List(newNameSet)
	baseSort(newNameList, func(i int) (string, int) {
		return parseParentNameAndOrdinal(newNameList[i])
	}, nil, true)
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
			}
		}
	}
	for i, name := range newNameList {
		if _, ok := createNameSet[name]; !ok {
			continue
		}
		if !isOrderedReady && concurrency <= 0 {
			break
		}
		predecessor := getPredecessor(i)
		if isOrderedReady && predecessor != nil && !intctrlutil.IsInstanceAvailable(predecessor) {
			break
		}
		newInst, err := buildInstanceByTemplate(tree, name, nameToTemplateMap[name], its)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if err := tree.Add(newInst); err != nil {
			return kubebuilderx.Continue, err
		}

		if isOrderedReady {
			break
		}
		concurrency--
	}

	// delete useless instances
	priorities := make(map[string]int)
	sortObjects(oldInstanceList, priorities, false)
	for _, object := range oldInstanceList {
		inst, _ := object.(*workloads.Instance)
		if _, ok := deleteNameSet[inst.Name]; !ok {
			continue
		}
		if !isOrderedReady && concurrency <= 0 {
			break
		}
		if isOrderedReady && !intctrlutil.IsInstanceReady(inst) {
			tree.EventRecorder.Eventf(its, corev1.EventTypeWarning, "InstanceSet %s/%s is waiting for Instance %s to be Ready",
				its.Namespace,
				its.Name,
				inst.Name)
		}
		if ptr.Deref(inst.Spec.ScaledDown, false) {
			if err := tree.Delete(inst); err != nil {
				return kubebuilderx.Continue, err
			}
		} else {
			instCopy := inst.DeepCopy()
			instCopy.Spec.ScaledDown = ptr.To(true)
			if err := tree.Update(instCopy); err != nil {
				return kubebuilderx.Continue, err
			}
		}
		if isOrderedReady {
			break
		}
		concurrency--
	}

	return kubebuilderx.Continue, nil
}
