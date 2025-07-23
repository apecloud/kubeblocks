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
	"maps"

	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func NewDeletionReconciler() kubebuilderx.Reconciler {
	return &deletionReconciler{}
}

type deletionReconciler struct{}

var _ kubebuilderx.Reconciler = &deletionReconciler{}

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
	if has, err := r.deleteSecondaryObjects(tree); has {
		return kubebuilderx.Continue, err
	}

	tree.DeleteRoot()
	return kubebuilderx.Continue, nil
}

func (r *deletionReconciler) deleteSecondaryObjects(tree *kubebuilderx.ObjectTree) (bool, error) {
	// secondary objects to be deleted
	secondaryObjects := maps.Clone(tree.GetSecondaryObjects())
	for _, obj := range secondaryObjects {
		if err := tree.Delete(obj); err != nil {
			return true, err
		}
	}
	return len(secondaryObjects) > 0, nil
}
