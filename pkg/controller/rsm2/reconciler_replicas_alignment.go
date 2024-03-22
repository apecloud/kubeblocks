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

package rsm2

import (
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// replicasAlignmentReconciler is responsible for aligning the actual replicas(i.e. number of pods) with the desired replicas specified in the spec,
// including horizontal scaling and recovering from unintended pod deletions etc.
// only handle replica count, don't care replica revision.
//
// TODO(free6om): support membership reconfiguration
type replicasAlignmentReconciler struct{}

func NewReplicasAlignmentReconciler() kubebuilderx.Reconciler {
	return &replicasAlignmentReconciler{}
}

func (r *replicasAlignmentReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ResultUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ResultUnsatisfied
	}
	rsm, _ := tree.GetRoot().(*workloads.ReplicatedStateMachine)
	if err := validateSpec(rsm); err != nil {
		return kubebuilderx.CheckResultWithError(err)
	}
	return kubebuilderx.ResultSatisfied
}

func (r *replicasAlignmentReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
	rsm, _ := tree.GetRoot().(*workloads.ReplicatedStateMachine)
	// 1. build desired name to template map
	nameToTemplateMap, err := buildReplicaName2TemplateMap(rsm)
	if err != nil {
		return nil, err
	}

	// 2. find the create and delete set
	newNameSet := sets.NewString()
	for name := range nameToTemplateMap {
		newNameSet.Insert(name)
	}
	oldNameSet := sets.NewString()
	oldReplicaMap := make(map[string]*corev1.Pod)
	oldReplicaList := tree.List(&corev1.Pod{})
	for _, object := range oldReplicaList {
		oldNameSet.Insert(object.GetName())
		pod, _ := object.(*corev1.Pod)
		oldReplicaMap[object.GetName()] = pod
	}
	createNameSet := newNameSet.Difference(oldNameSet)
	deleteNameSet := oldNameSet.Difference(newNameSet)

	// default OrderedReady policy
	createCount, deleteCount := 1, 1
	shouldReady := true
	if rsm.Spec.PodManagementPolicy == apps.ParallelPodManagement {
		createCount = len(createNameSet)
		deleteCount = len(deleteNameSet)
		shouldReady = false
	}
	// TODO(free6om): handle BestEffortParallel: always keep the majority available.

	// 3. handle alignment (create new replicas and delete useless replicas)
	// create new replicas
	newNameList := newNameSet.List()
	baseSort(newNameList, func(i int) string {
		return newNameList[i]
	}, nil, false)
	getPredecessor := func(i int) *corev1.Pod {
		if i <= 0 {
			return nil
		}
		return oldReplicaMap[newNameList[i-1]]
	}
	for i, name := range newNameList {
		if _, ok := createNameSet[name]; !ok {
			continue
		}
		if createCount <= 0 {
			break
		}
		predecessor := getPredecessor(i)
		if shouldReady && predecessor != nil && !isHealthy(predecessor) {
			break
		}
		replica, err := buildReplicaByTemplate(name, nameToTemplateMap[name], rsm)
		if err != nil {
			return nil, err
		}
		if err := tree.Add(replica.pod); err != nil {
			return nil, err
		}
		for _, pvc := range replica.pvcs {
			switch oldPvc, err := tree.Get(pvc); {
			case err != nil:
				return nil, err
			case oldPvc == nil:
				if err = tree.Add(pvc); err != nil {
					return nil, err
				}
			default:
				pvcObj := copyAndMerge(oldPvc, pvc)
				if pvcObj != nil {
					if err = tree.Update(pvcObj); err != nil {
						return nil, err
					}
				}
			}
		}
		createCount--
	}

	// delete useless replicas
	priorities := make(map[string]int)
	sortObjects(oldReplicaList, priorities, true)
	for _, object := range oldReplicaList {
		pod, _ := object.(*corev1.Pod)
		if _, ok := deleteNameSet[pod.Name]; !ok {
			continue
		}
		if deleteCount <= 0 {
			break
		}
		if shouldReady && !isRunningAndReady(pod) {
			tree.EventRecorder.Eventf(rsm, corev1.EventTypeWarning, "RSM %s/%s is waiting for Pod %s to be Running and Ready",
				rsm.Namespace,
				rsm.Name,
				pod.Name)
		}
		if err := tree.Delete(pod); err != nil {
			return nil, err
		}
		// TODO(free6om): handle pvc management policy
		// Retain by default.
		deleteCount--
	}

	return tree, nil
}

var _ kubebuilderx.Reconciler = &replicasAlignmentReconciler{}
