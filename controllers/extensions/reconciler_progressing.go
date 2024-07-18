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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type progressingReconciler struct {
	stageCtx
	enablingReconciler enablingReconciler
	disablingStage     disablingReconciler
}

func (r *progressingReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
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

func (r *progressingReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
	r.enablingReconciler.stageCtx = r.stageCtx
	r.disablingStage.stageCtx = r.stageCtx

	addon := tree.GetRoot().(*extensionsv1alpha1.Addon)
	r.reqCtx.Log.V(1).Info("progressingHandler", "phase", addon.Status.Phase)
	patchPhase := func(phase extensionsv1alpha1.AddonPhase, reason string) error {
		r.reqCtx.Log.V(1).Info("patching status", "phase", phase)
		patch := client.MergeFrom(addon.DeepCopy())
		addon.Status.Phase = phase
		addon.Status.ObservedGeneration = addon.Generation
		if err := r.reconciler.Status().Patch(r.reqCtx.Ctx, addon, patch); err != nil {
			r.setRequeueWithErr(err, "")
			return err
		}
		r.reconciler.Event(addon, corev1.EventTypeNormal, reason,
			fmt.Sprintf("Progress to %s phase", phase))
		r.setReconciled()
		return nil
	}

	// decision enabling or disabling
	if !addon.Spec.InstallSpec.GetEnabled() {
		r.reqCtx.Log.V(1).Info("progress to disabling stage handler")
		// if it's new simply return
		if addon.Status.Phase == "" {
			return tree, nil
		}
		if addon.Status.Phase != extensionsv1alpha1.AddonDisabling {
			if err := patchPhase(extensionsv1alpha1.AddonDisabling, DisablingAddon); err != nil {
				return tree, err
			}
			return tree, nil
		}
		r.disablingStage.Reconcile(tree)
		return tree, nil
	}
	// handling enabling state
	if addon.Status.Phase != extensionsv1alpha1.AddonEnabling {
		if addon.Status.Phase == extensionsv1alpha1.AddonFailed {
			// clean up existing failed installation job
			mgrNS := viper.GetString(constant.CfgKeyCtrlrMgrNS)
			key := client.ObjectKey{
				Namespace: mgrNS,
				Name:      getInstallJobName(addon),
			}
			installJob := &batchv1.Job{}
			if err := r.reconciler.Get(r.reqCtx.Ctx, key, installJob); client.IgnoreNotFound(err) != nil {
				r.setRequeueWithErr(err, "")
				return tree, err
			} else if err == nil && installJob.GetDeletionTimestamp().IsZero() {
				if err = r.reconciler.Delete(r.reqCtx.Ctx, installJob); err != nil {
					r.setRequeueWithErr(err, "")
					return tree, err
				}
			}
		}
		if err := patchPhase(extensionsv1alpha1.AddonEnabling, EnablingAddon); err != nil {
			return tree, err
		}
		return tree, nil
	}
	r.reqCtx.Log.V(1).Info("progress to enabling stage handler")
	r.enablingReconciler.Reconcile(tree)

	return tree, nil
}

func NewProgressingReconciler(reqCtx intctrlutil.RequestCtx, buildStageCtx func() stageCtx) kubebuilderx.Reconciler {
	return &progressingReconciler{
		stageCtx: buildStageCtx(),
	}
}

var _ kubebuilderx.Reconciler = &progressingReconciler{}
