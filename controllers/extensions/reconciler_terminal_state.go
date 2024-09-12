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
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

type terminalStateReconciler struct {
	stageCtx
}

func (r *terminalStateReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
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

func (r *terminalStateReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	addon := tree.GetRoot().(*extensionsv1alpha1.Addon)
	r.reqCtx.Log.V(1).Info("terminalStateReconciler", "phase", addon.Status.Phase)

	helmInstallJob, err1 := r.reconciler.GetInstallJob(r.reqCtx.Ctx, "install", tree)
	helmUninstallJob, err2 := r.reconciler.GetInstallJob(r.reqCtx.Ctx, "uninstall", tree)

	if apierrors.IsNotFound(err1) && apierrors.IsNotFound(err2) {
		if err := r.reconciler.PatchPhase(addon, r.stageCtx, extensionsv1alpha1.AddonDisabled, AddonDisabled); err != nil {
			return kubebuilderx.Continue, err
		}
		return kubebuilderx.Continue, nil
	}

	if err1 == nil && helmInstallJob.Status.Active > 0 {
		if addon.Status.Phase == extensionsv1alpha1.AddonEnabling {
			if err := r.reconciler.PatchPhase(addon, r.stageCtx, extensionsv1alpha1.AddonEnabled, AddonEnabled); err != nil {
				return kubebuilderx.Continue, err
			}
		}
	}
	if err2 == nil && helmUninstallJob.Status.Active > 0 {
		if addon.Status.Phase == extensionsv1alpha1.AddonDisabling {
			if err := r.reconciler.PatchPhase(addon, r.stageCtx, extensionsv1alpha1.AddonDisabled, AddonDisabled); err != nil {
				return kubebuilderx.Continue, err
			}
		}
	}

	return kubebuilderx.Continue, nil
}

func NewTerminalStateReconciler(reqCtx intctrlutil.RequestCtx, buildStageCtx func() stageCtx) kubebuilderx.Reconciler {
	return &terminalStateReconciler{
		stageCtx: buildStageCtx(),
	}
}

var _ kubebuilderx.Reconciler = &terminalStateReconciler{}
