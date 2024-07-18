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

package extensions

import (
	ctrl "sigs.k8s.io/controller-runtime"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type genIDProceedReconciler struct {
	stageCtx
}

func (r *genIDProceedReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ResultUnsatisfied
	}
	if res, _ := r.reqCtx.Ctx.Value(resultValueKey).(*ctrl.Result); res != nil {
		return kubebuilderx.ResultUnsatisfied
	}
	if err, _ := r.reqCtx.Ctx.Value(errorValueKey).(error); err != nil {
		return kubebuilderx.ResultUnsatisfied
	}

	return kubebuilderx.ResultSatisfied
}

func (r *genIDProceedReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
	addon := tree.GetRoot().(*extensionsv1alpha1.Addon)
	r.reqCtx.Log.V(1).Info("genIDProceedCheckStage", "phase", addon.Status.Phase)
	switch addon.Status.Phase {
	case extensionsv1alpha1.AddonEnabled, extensionsv1alpha1.AddonDisabled:
		if addon.Generation == addon.Status.ObservedGeneration {
			res, err := r.reconciler.deleteExternalResources(*r.reqCtx, addon)
			if res != nil || err != nil {
				r.updateResultNErr(res, err)
				return tree, err
			}
			r.setReconciled()
			return tree, nil
		}
	case extensionsv1alpha1.AddonFailed:
		if addon.Generation == addon.Status.ObservedGeneration {
			r.setReconciled()
			return tree, nil
		}
	}
	return tree, nil
}

func NewGenIDProceedCheckReconciler(reqCtx intctrlutil.RequestCtx, buildStageCtx func() stageCtx) kubebuilderx.Reconciler {
	return &genIDProceedReconciler{
		stageCtx: buildStageCtx(),
	}
}

var _ kubebuilderx.Reconciler = &genIDProceedReconciler{}
