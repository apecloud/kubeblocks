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
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type apiVersionReconciler struct{}

func (r *apiVersionReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *apiVersionReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	if !intctrlutil.ObjectAPIVersionSupported(tree.GetRoot()) {
		return kubebuilderx.Commit, nil
	}
	its := tree.GetRoot().(*workloads.InstanceSet)
	if ptr.Deref(its.Spec.EnableInstanceAPI, false) {
		return kubebuilderx.Commit, nil
	}
	return kubebuilderx.Continue, nil
}

func NewAPIVersionReconciler() kubebuilderx.Reconciler {
	return &apiVersionReconciler{}
}

var _ kubebuilderx.Reconciler = &apiVersionReconciler{}
