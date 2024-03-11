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
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// objectDeletionReconciler handles object and its secondary resources' deletion
type objectDeletionReconciler struct{}

func (r *objectDeletionReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if !model.IsObjectDeleting(tree.Root) {
		return kubebuilderx.ResultUnsatisfied
	}
	if model.IsReconciliationPaused(tree.Root) {
		return kubebuilderx.ResultUnsatisfied
	}
	return kubebuilderx.ResultSatisfied
}

func (r *objectDeletionReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
	tree.Children = nil
	return tree, nil
}

func NewObjectDeletionReconciler() kubebuilderx.Reconciler {
	return &objectDeletionReconciler{}
}

var _ kubebuilderx.Reconciler = &objectDeletionReconciler{}
