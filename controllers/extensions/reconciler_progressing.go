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
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

type progressingReconciler struct {
	stageCtx
	enablingReconciler enablingReconciler
	disablingStage     disablingReconciler
}

func (r *progressingReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
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

func (r *progressingReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	r.enablingReconciler.stageCtx = r.stageCtx
	r.disablingStage.stageCtx = r.stageCtx

	addon := tree.GetRoot().(*extensionsv1alpha1.Addon)
	r.reqCtx.Log.V(1).Info("progressingReconciler", "phase", addon.Status.Phase)

	helmInstallJob, err1 := r.reconciler.GetInstallJob(r.reqCtx.Ctx, "install", tree)
	helmUninstallJob, err2 := r.reconciler.GetInstallJob(r.reqCtx.Ctx, "uninstall", tree)

	// decision enabling or disabling
	if !addon.Spec.InstallSpec.GetEnabled() {
		r.reqCtx.Log.V(1).Info("progress to disabling stage handler")
		if apierrors.IsNotFound(err1) && apierrors.IsNotFound(err2) {
			return kubebuilderx.Continue, nil
		}
		if err2 != nil && helmUninstallJob.Status.Active == 0 && addon.Status.Phase != extensionsv1alpha1.AddonDisabling {
			if err := r.reconciler.PatchPhase(addon, r.stageCtx, extensionsv1alpha1.AddonDisabling, DisablingAddon); err != nil {
				return kubebuilderx.Continue, err
			}
			return kubebuilderx.Continue, nil
		}
		r.disablingStage.Reconcile(tree)
		return kubebuilderx.Continue, nil
	}
	// handling enabling state
	if err1 != nil && helmInstallJob.Status.Active == 0 && addon.Status.Phase != extensionsv1alpha1.AddonEnabling {
		if helmInstallJob.Status.Failed > 0 {
			// clean up existing failed installation job
			if helmInstallJob.GetDeletionTimestamp().IsZero() {
				if err := r.reconciler.Delete(r.reqCtx.Ctx, helmInstallJob); err != nil {
					r.setRequeueWithErr(err, "")
					return kubebuilderx.Continue, err
				}
			}
		}
		if err := r.reconciler.PatchPhase(addon, r.stageCtx, extensionsv1alpha1.AddonEnabling, EnablingAddon); err != nil {
			return kubebuilderx.Continue, err
		}
		return kubebuilderx.Continue, nil
	}
	r.reqCtx.Log.V(1).Info("progress to enabling stage handler")
	r.enablingReconciler.Reconcile(tree)

	return kubebuilderx.Continue, nil
}

func NewProgressingReconciler(reqCtx intctrlutil.RequestCtx, buildStageCtx func() stageCtx) kubebuilderx.Reconciler {
	return &progressingReconciler{
		stageCtx: buildStageCtx(),
	}
}

var _ kubebuilderx.Reconciler = &progressingReconciler{}
