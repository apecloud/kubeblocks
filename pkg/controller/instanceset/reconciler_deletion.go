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

package instanceset

import (
	"maps"

	corev1 "k8s.io/api/core/v1"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// deletionReconciler handles object and its secondary resources' deletion
type deletionReconciler struct{}

func (r *deletionReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || !model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *deletionReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its, _ := tree.GetRoot().(*workloads.InstanceSet)
	pvcRetentionPolicy := its.Spec.PersistentVolumeClaimRetentionPolicy
	retainPVC := pvcRetentionPolicy != nil && pvcRetentionPolicy.WhenDeleted == kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType

	// delete secondary objects first
	if has, err := r.deleteSecondaryObjects(tree, retainPVC); has {
		return kubebuilderx.Continue, err
	}

	// delete root object
	tree.DeleteRoot()
	return kubebuilderx.Continue, nil
}

func (r *deletionReconciler) deleteSecondaryObjects(tree *kubebuilderx.ObjectTree, retainPVC bool) (bool, error) {
	// secondary objects to be deleted
	secondaryObjects := maps.Clone(tree.GetSecondaryObjects())
	if retainPVC {
		// exclude PVCs from them
		pvcList := tree.List(&corev1.PersistentVolumeClaim{})
		for _, pvc := range pvcList {
			name, err := model.GetGVKName(pvc)
			if err != nil {
				return true, err
			}
			delete(secondaryObjects, *name)
		}
	}
	// delete them
	for _, obj := range secondaryObjects {
		if err := tree.Delete(obj); err != nil {
			return true, err
		}
	}
	return len(secondaryObjects) > 0, nil
}

func NewDeletionReconciler() kubebuilderx.Reconciler {
	return &deletionReconciler{}
}

var _ kubebuilderx.Reconciler = &deletionReconciler{}
