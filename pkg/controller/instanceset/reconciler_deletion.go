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
	secondaryObjects, err := r.getSecondaryObjects(tree, retainPVC)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	if len(secondaryObjects) > 0 {
		if err := r.deleteSecondaryObjects(tree, secondaryObjects, retainPVC); err != nil {
			return kubebuilderx.Continue, err
		}
		return kubebuilderx.Continue, nil
	}

	// delete root object
	tree.DeleteRoot()
	return kubebuilderx.Continue, nil
}

func (r *deletionReconciler) getSecondaryObjects(tree *kubebuilderx.ObjectTree, retainPVC bool) (model.ObjectSnapshot, error) {
	secondaryObjects := maps.Clone(tree.GetSecondaryObjects())
	if retainPVC {
		pvcList := tree.List(&corev1.PersistentVolumeClaim{})
		for _, pvc := range pvcList {
			name, err := model.GetGVKName(pvc)
			if err != nil {
				return nil, err
			}
			delete(secondaryObjects, *name)
		}
	}
	return secondaryObjects, nil
}

func (r *deletionReconciler) deleteSecondaryObjects(tree *kubebuilderx.ObjectTree, secondaryObjects model.ObjectSnapshot, retainPVC bool) error {
	if retainPVC {
		for _, obj := range secondaryObjects {
			if err := tree.Delete(obj); err != nil {
				return err
			}
		}
	} else {
		// fast path
		tree.DeleteSecondaryObjects()
	}
	return nil
}

func NewDeletionReconciler() kubebuilderx.Reconciler {
	return &deletionReconciler{}
}

var _ kubebuilderx.Reconciler = &deletionReconciler{}
