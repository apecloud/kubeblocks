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
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func NewRevisionUpdateReconciler() kubebuilderx.Reconciler {
	return &revisionUpdateReconciler{}
}

type revisionUpdateReconciler struct{}

var _ kubebuilderx.Reconciler = &revisionUpdateReconciler{}

func (r *revisionUpdateReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || !model.IsObjectUpdating(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *revisionUpdateReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	inst := tree.GetRoot().(*workloads.Instance)
	revision, err := buildInstancePodRevision(&inst.Spec.Template, inst)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	inst.Status.UpdateRevision = revision
	// The 'ObservedGeneration' field is used to indicate whether the revisions have been updated.
	// Computing these revisions in each reconciliation loop can be time-consuming, so we optimize it by
	// performing the computation only when the 'spec' is updated.
	inst.Status.ObservedGeneration = inst.Generation

	return kubebuilderx.Continue, nil
}
