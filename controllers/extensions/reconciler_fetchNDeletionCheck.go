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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

type fetchNDeletionCheckReconciler struct {
	stageCtx
	deletionStage deletionStage
}

func (r *fetchNDeletionCheckReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil {
		return kubebuilderx.ResultUnsatisfied
	}

	return kubebuilderx.ResultSatisfied
}

func (r *fetchNDeletionCheckReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
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
		r.deletionStage.Handle(r.reqCtx.Ctx)
		return r.deletionStage.doReturn()
	})
	if res != nil || err != nil {
		r.updateResultNErr(res, err)
		return tree, err
	}
	r.reqCtx.Log.V(1).Info("start normal reconcile")
	return tree, nil
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
