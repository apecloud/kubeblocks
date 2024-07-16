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
	//"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"context"
	"fmt"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	//ctrlerihandler "github.com/authzed/controller-idioms/handler"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type fetchNDeletionCheckReconciler struct {
	stageCtx
	deletionStage deletionStage
}

func (r *fetchNDeletionCheckReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ResultUnsatisfied
	}

	return kubebuilderx.ResultSatisfied
}

func (r *fetchNDeletionCheckReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
	//addon := tree.GetRoot().(*extensionsv1alpha1.Addon)
	addon := &extensionsv1alpha1.Addon{}
	if err := r.reconciler.Client.Get(r.reqCtx.Ctx, r.reqCtx.Req.NamespacedName, addon); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, r.reqCtx.Log, "")
		r.updateResultNErr(&res, err)
		return tree, err
	}
	r.reqCtx.Log.V(1).Info("get addon", "generation", addon.Generation, "observedGeneration", addon.Status.ObservedGeneration)
	r.reqCtx.UpdateCtxValue(operandValueKey, addon)

	// CheckIfAddonUsedByCluster, if err, skip the deletion stage
	if !addon.GetDeletionTimestamp().IsZero() || !addon.Spec.InstallSpec.GetEnabled() {
		recordEvent := func() {
			r.reconciler.Event(addon, corev1.EventTypeWarning, "Addon is used by some clusters",
				"Addon is used by cluster, please check")
		}
		if res, err := intctrlutil.ValidateReferenceCR(*r.reqCtx, r.reconciler.Client, addon, constant.ClusterDefLabelKey,
			recordEvent, &appsv1alpha1.ClusterList{}); res != nil || err != nil {
			r.updateResultNErr(res, err)
			return tree, err
		}
	}
	res, err := intctrlutil.HandleCRDeletion(*r.reqCtx, r.reconciler, addon, addonFinalizerName, func() (*ctrl.Result, error) {
		r.deletionStage.Handle2(r.reqCtx.Ctx)
		return r.deletionStage.doReturn()
	})
	if res != nil || err != nil {
		r.updateResultNErr(res, err)
		return tree, err
	}
	r.reqCtx.Log.V(1).Info("start normal reconcile")
	//r.next.Handle(r.reqCtx.Ctx)
	return tree, nil
}

func (r *deletionStage) Handle2(ctx context.Context) {
	r.disablingStage.stageCtx = r.stageCtx
	r.process(func(addon *extensionsv1alpha1.Addon) {
		r.reqCtx.Log.V(1).Info("deletionStage", "phase", addon.Status.Phase)
		patchPhase := func(phase extensionsv1alpha1.AddonPhase, reason string) {
			r.reqCtx.Log.V(1).Info("patching status", "phase", phase)
			patch := client.MergeFrom(addon.DeepCopy())
			addon.Status.Phase = phase
			addon.Status.ObservedGeneration = addon.Generation
			if err := r.reconciler.Status().Patch(ctx, addon, patch); err != nil {
				r.setRequeueWithErr(err, "")
				return
			}
			r.reqCtx.Log.V(1).Info("progress to", "phase", phase)
			r.reconciler.Event(addon, corev1.EventTypeNormal, reason,
				fmt.Sprintf("Progress to %s phase", phase))
			r.setReconciled()
		}
		switch addon.Status.Phase {
		case extensionsv1alpha1.AddonEnabling:
			// delete running jobs
			res, err := r.reconciler.deleteExternalResources(*r.reqCtx, addon)
			if err != nil {
				r.updateResultNErr(res, err)
				return
			}
			patchPhase(extensionsv1alpha1.AddonDisabling, DisablingAddon)
			return
		case extensionsv1alpha1.AddonEnabled:
			patchPhase(extensionsv1alpha1.AddonDisabling, DisablingAddon)
			return
		case extensionsv1alpha1.AddonDisabling:
			r.disablingStage.Handle(ctx)
			res, err := r.disablingStage.doReturn()

			if res != nil || err != nil {
				return
			}
			patchPhase(extensionsv1alpha1.AddonDisabled, AddonDisabled)
			return
		default:
			r.reqCtx.Log.V(1).Info("delete external resources", "phase", addon.Status.Phase)
			res, err := r.reconciler.deleteExternalResources(*r.reqCtx, addon)
			if res != nil || err != nil {
				r.updateResultNErr(res, err)
				return
			}
			return
		}
	})
	//r.next.Handle(ctx)
}

func NewfetchNDeletionCheckReconciler(reqCtx intctrlutil.RequestCtx, buildStageCtx func() stageCtx) kubebuilderx.Reconciler {

	return &fetchNDeletionCheckReconciler{
		stageCtx: buildStageCtx(),
		deletionStage: deletionStage{
			stageCtx: buildStageCtx(),
		},
	}
}

var _ kubebuilderx.Reconciler = &fetchNDeletionCheckReconciler{}
