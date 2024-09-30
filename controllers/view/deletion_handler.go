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

package view

import (
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type deletionHandler struct {
	store ObjectRevisionStore
}

func (h *deletionHandler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || !model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (h *deletionHandler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	view, _ := tree.GetRoot().(*viewv1.ReconciliationView)

	// store cleanup
	for _, change := range view.Status.CurrentState.Changes {
		objectRef := objectReferenceToRef(&change.ObjectReference)
		h.store.Delete(objectRef, view, change.Revision)
	}
	// TODO(free6om): events cleanup

	// remove finalizer
	tree.DeleteRoot()

	return kubebuilderx.Commit, nil
}

func handleDeletion(store ObjectRevisionStore) kubebuilderx.Reconciler {
	return &deletionHandler{store: store}
}

var _ kubebuilderx.Reconciler = &deletionHandler{}
