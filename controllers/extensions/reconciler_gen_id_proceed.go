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
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type genIDProceedReconciler struct {
	stageCtx
}

func (r *genIDProceedReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
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

func (r *genIDProceedReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	addon := tree.GetRoot().(*extensionsv1alpha1.Addon)
	r.reqCtx.Log.V(1).Info("genIDProceedCheckReconciler", "phase", addon.Status.Phase)

	helmInstallJob, err1 := r.reconciler.GetInstallJob(r.reqCtx.Ctx, "install", tree)
	helmUninstallJob, err2 := r.reconciler.GetInstallJob(r.reqCtx.Ctx, "uninstall", tree)

	if apierrors.IsNotFound(err1) && apierrors.IsNotFound(err2) {
		return kubebuilderx.Continue, nil
	}
	if (err1 == nil && helmInstallJob.Status.Failed > 0) || (err2 == nil && helmUninstallJob.Status.Failed > 0) {
		if helmInstallJob.Status.Failed > 0 && addon.Status.Phase == "Enabling" {
			setAddonErrorConditions(r.reqCtx.Ctx, &r.stageCtx, addon, true, true, InstallationFailed,
				fmt.Sprintf("Installation failed, do inspect error from jobs.batch %s", helmInstallJob.Name))
			if viper.GetInt(maxConcurrentReconcilesKey) > 1 {
				if err := logFailedJobPodToCondError(r.reqCtx.Ctx, &r.stageCtx, addon, helmInstallJob.Name, InstallationFailedLogs); err != nil {
					r.setRequeueWithErr(err, "")
					return kubebuilderx.Continue, err
				}
			}
		}
		if helmUninstallJob.Status.Failed > 0 && addon.Status.Phase == "Disabling" {
			if viper.GetInt(maxConcurrentReconcilesKey) > 1 {
				if err := logFailedJobPodToCondError(r.reqCtx.Ctx, &r.stageCtx, addon, helmUninstallJob.Name, UninstallationFailedLogs); err != nil {
					r.setRequeueWithErr(err, "")
					return kubebuilderx.Continue, err
				}
			}
			if err := r.reconciler.Delete(r.reqCtx.Ctx, helmUninstallJob); client.IgnoreNotFound(err) != nil {
				r.setRequeueWithErr(err, "")
				return kubebuilderx.Continue, err
			}
			if err := r.reconciler.cleanupJobPods(*r.reqCtx); err != nil {
				r.setRequeueWithErr(err, "")
				return kubebuilderx.Continue, err
			}
		}

		if addon.Generation == addon.Status.ObservedGeneration {
			r.setReconciled()
			return kubebuilderx.RetryAfter(time.Second), nil
		}
	} else if (err1 == nil && helmInstallJob.Status.Succeeded > 0) || (err2 == nil && helmUninstallJob.Status.Succeeded > 0) {
		if addon.Status.Phase == "Enabling" && helmInstallJob.Status.Succeeded > 0 {
			err := r.reconciler.PatchPhase(addon, r.stageCtx, "Enabled", AddonEnabled)
			return kubebuilderx.Continue, err
		} else if addon.Status.Phase == "Disabling" && helmUninstallJob.Status.Succeeded > 0 {
			err := r.reconciler.PatchPhase(addon, r.stageCtx, "Disabled", AddonDisabled)
			return kubebuilderx.Continue, err
		}
		if (!helmInstallJob.DeletionTimestamp.IsZero() && helmInstallJob.Status.Succeeded > 0) ||
			(!helmUninstallJob.DeletionTimestamp.IsZero() && helmUninstallJob.Status.Succeeded > 0) {
			return kubebuilderx.Continue, nil
		}

		if addon.Generation == addon.Status.ObservedGeneration {
			res, err := r.reconciler.deleteExternalResources(*r.reqCtx, addon)
			if res != nil || err != nil {
				r.updateResultNErr(res, err)
				return kubebuilderx.Continue, err
			}
			r.setReconciled()
			return kubebuilderx.Continue, nil
		}
	}

	return kubebuilderx.Continue, nil
}

func NewGenIDProceedCheckReconciler(reqCtx intctrlutil.RequestCtx, buildStageCtx func() stageCtx) kubebuilderx.Reconciler {
	return &genIDProceedReconciler{
		stageCtx: buildStageCtx(),
	}
}

var _ kubebuilderx.Reconciler = &genIDProceedReconciler{}
