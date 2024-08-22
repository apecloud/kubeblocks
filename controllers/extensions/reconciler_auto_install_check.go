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

type autoInstallCheckReconciler struct {
	stageCtx
}

func (r *autoInstallCheckReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	if res, _ := r.reqCtx.Ctx.Value(resultValueKey).(*ctrl.Result); res != nil {
		return kubebuilderx.ConditionUnsatisfied
	}
	if err, _ := r.reqCtx.Ctx.Value(errorValueKey).(error); err != nil {
		return kubebuilderx.ConditionUnsatisfied
	}

	return kubebuilderx.ConditionSatisfied
}

func (r *autoInstallCheckReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	addon := tree.GetRoot().(*extensionsv1alpha1.Addon)
	r.reqCtx.Log.V(1).Info("autoInstallCheckReconciler", "phase", addon.Status.Phase)
	if addon.Spec.Installable == nil || !addon.Spec.Installable.AutoInstall {
		return kubebuilderx.Continue, nil
	}
	// proceed if has specified addon.spec.installSpec
	if addon.Spec.InstallSpec != nil {
		r.reqCtx.Log.V(1).Info("has specified addon.spec.installSpec")
		return kubebuilderx.Continue, nil
	}
	enabledAddonWithDefaultValues(r.reqCtx.Ctx, &r.stageCtx, addon, AddonAutoInstall, "Addon enabled auto-install")

	return kubebuilderx.Continue, nil
}

func NewAutoInstallCheckReconciler(reqCtx intctrlutil.RequestCtx, buildStageCtx func() stageCtx) kubebuilderx.Reconciler {
	return &autoInstallCheckReconciler{
		stageCtx: buildStageCtx(),
	}
}

var _ kubebuilderx.Reconciler = &autoInstallCheckReconciler{}
