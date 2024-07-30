/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// instanceAlignmentReconciler is responsible for aligning the actual instances(pods) with the desired replicas specified in the spec,
// including horizontal scaling and recovering from unintended pod deletions etc.
// only handle instance count, don't care instance revision.
//
// TODO(free6om): support membership reconfiguration
type instanceAlignmentReconciler struct{}

func NewReplicasAlignmentReconciler() kubebuilderx.Reconciler {
	return &instanceAlignmentReconciler{}
}

func (r *instanceAlignmentReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	its, _ := tree.GetRoot().(*workloads.InstanceSet)
	if err := validateSpec(its, tree); err != nil {
		return kubebuilderx.CheckResultWithError(err)
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *instanceAlignmentReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its, _ := tree.GetRoot().(*workloads.InstanceSet)
	itsExt, err := buildInstanceSetExt(its, tree)
	if err != nil {
		return kubebuilderx.Continue, err
	}

	// 1. build desired name to template map
	nameToTemplateMap, err := buildInstanceName2TemplateMap(itsExt)
	if err != nil {
		return kubebuilderx.Continue, err
	}

	// 2. find the create and delete set
	newNameSet := sets.NewString()
	for name := range nameToTemplateMap {
		newNameSet.Insert(name)
	}
	oldNameSet := sets.NewString()
	oldInstanceMap := make(map[string]*corev1.Pod)
	oldInstanceList := tree.List(&corev1.Pod{})
	for _, object := range oldInstanceList {
		oldNameSet.Insert(object.GetName())
		pod, _ := object.(*corev1.Pod)
		oldInstanceMap[object.GetName()] = pod
	}
	createNameSet := newNameSet.Difference(oldNameSet)
	deleteNameSet := oldNameSet.Difference(newNameSet)

	// default OrderedReady policy
	isOrderedReady := true
	concurrency := 0
	if its.Spec.PodManagementPolicy == appsv1.ParallelPodManagement {
		concurrency, err = CalculateConcurrencyReplicas(its.Spec.ParallelPodManagementConcurrency, int(*its.Spec.Replicas))
		if err != nil {
			return nil, err
		}
		isOrderedReady = false
	}
	// TODO(free6om): handle BestEffortParallel: always keep the majority available.

	// 3. handle alignment (create new instances and delete useless instances)
	// create new instances
	newNameList := newNameSet.List()
	baseSort(newNameList, func(i int) (string, int) {
		return ParseParentNameAndOrdinal(newNameList[i])
	}, nil, true)
	getPredecessor := func(i int) *corev1.Pod {
		if i <= 0 {
			return nil
		}
		return oldInstanceMap[newNameList[i-1]]
	}
	if !isOrderedReady {
		for _, name := range newNameList {
			if _, ok := createNameSet[name]; !ok {
				if !isHealthy(oldInstanceMap[name]) {
					concurrency--
				}
			}
		}
	}
	var currentAlignedNameList []string
	for i, name := range newNameList {
		if _, ok := createNameSet[name]; !ok {
			currentAlignedNameList = append(currentAlignedNameList, name)
			continue
		}
		if !isOrderedReady && concurrency <= 0 {
			break
		}
		predecessor := getPredecessor(i)
		if isOrderedReady && predecessor != nil && !isHealthy(predecessor) {
			break
		}
		inst, err := buildInstanceByTemplate(name, nameToTemplateMap[name], its, "")
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if err := tree.Add(inst.pod); err != nil {
			return kubebuilderx.Continue, err
		}
		currentAlignedNameList = append(currentAlignedNameList, name)

		if isOrderedReady {
			break
		}
		concurrency--
	}

	// create PVCs
	for _, name := range currentAlignedNameList {
		pvcs := buildInstancePVCByTemplate(name, nameToTemplateMap[name], its)
		for _, pvc := range pvcs {
			switch oldPvc, err := tree.Get(pvc); {
			case err != nil:
				return kubebuilderx.Continue, err
			case oldPvc == nil:
				if err = tree.Add(pvc); err != nil {
					return kubebuilderx.Continue, err
				}
			default:
				pvcObj := copyAndMerge(oldPvc, pvc)
				if pvcObj != nil {
					if err = tree.Update(pvcObj); err != nil {
						return kubebuilderx.Continue, err
					}
				}
			}
		}
	}

	// delete useless instances
	priorities := make(map[string]int)
	sortObjects(oldInstanceList, priorities, false)
	for _, object := range oldInstanceList {
		pod, _ := object.(*corev1.Pod)
		if _, ok := deleteNameSet[pod.Name]; !ok {
			continue
		}
		if !isOrderedReady && concurrency <= 0 {
			break
		}
		if isOrderedReady && !isRunningAndReady(pod) {
			tree.EventRecorder.Eventf(its, corev1.EventTypeWarning, "InstanceSet %s/%s is waiting for Pod %s to be Running and Ready",
				its.Namespace,
				its.Name,
				pod.Name)
		}
		if err := tree.Delete(pod); err != nil {
			return kubebuilderx.Continue, err
		}
		// TODO(free6om): handle pvc management policy
		// Retain by default.

		if isOrderedReady {
			break
		}
		concurrency--
	}

	return kubebuilderx.Continue, nil
}

var _ kubebuilderx.Reconciler = &instanceAlignmentReconciler{}
