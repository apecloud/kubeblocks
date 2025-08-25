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

package instance

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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
	inst := tree.GetRoot().(*workloads.Instance)

	// create pod
	obj, err := tree.Get(podObj(inst))
	if err != nil {
		return kubebuilderx.Continue, err
	}
	if obj == nil {
		newPod, err := buildInstancePod(inst, "")
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if err := tree.Add(newPod); err != nil {
			return kubebuilderx.Continue, err
		}
	}

	// handle pvcs
	newPVCList, err := buildInstancePVCs(inst)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	oldPVCList := tree.List(&corev1.PersistentVolumeClaim{})

	newPVCs, oldPVCs := map[string]*corev1.PersistentVolumeClaim{}, map[string]*corev1.PersistentVolumeClaim{}
	newPVCNameSet, oldPVCNameSet := sets.New[string](), sets.New[string]()
	for i, pvc := range newPVCList {
		newPVCs[pvc.Name] = newPVCList[i]
		newPVCNameSet.Insert(pvc.Name)
	}
	for i, pvc := range oldPVCList {
		oldPVCs[pvc.GetName()] = oldPVCList[i].(*corev1.PersistentVolumeClaim)
		oldPVCNameSet.Insert(pvc.GetName())
	}

	createSet := newPVCNameSet.Difference(oldPVCNameSet)
	deleteSet := oldPVCNameSet.Difference(newPVCNameSet)
	updateSet := newPVCNameSet.Intersection(oldPVCNameSet)

	for pvcName := range deleteSet {
		if err = tree.Delete(oldPVCs[pvcName]); err != nil {
			return kubebuilderx.Continue, err
		}
	}
	for pvcName := range createSet {
		if err = tree.Add(newPVCs[pvcName]); err != nil {
			return kubebuilderx.Continue, err
		}
	}
	for pvcName := range updateSet {
		// TODO: do not update PVC here
		pvcObj := copyAndMerge(oldPVCs[pvcName], newPVCs[pvcName])
		if pvcObj != nil {
			if err = tree.Update(pvcObj); err != nil {
				return kubebuilderx.Continue, err
			}
		}
	}

	return kubebuilderx.Continue, nil
}
