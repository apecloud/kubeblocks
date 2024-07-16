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

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	//corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	//"k8s.io/apimachinery/pkg/api/meta"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"sigs.k8s.io/controller-runtime/pkg/client"
)

type autoInstallCheckReconciler struct {
	stageCtx
}

func (r *autoInstallCheckReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ResultUnsatisfied
	}

	return kubebuilderx.ResultSatisfied
}

func (r *autoInstallCheckReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
	r.process(func(addon *extensionsv1alpha1.Addon) {
		r.reqCtx.Log.V(1).Info("autoInstallCheckStage", "phase", addon.Status.Phase)
		if addon.Spec.Installable == nil || !addon.Spec.Installable.AutoInstall {
			return
		}
		// proceed if has specified addon.spec.installSpec
		if addon.Spec.InstallSpec != nil {
			r.reqCtx.Log.V(1).Info("has specified addon.spec.installSpec")
			return
		}
		enabledAddonWithDefaultValues(r.reqCtx.Ctx, &r.stageCtx, addon, AddonAutoInstall, "Addon enabled auto-install")
	})
	//r.next.Handle(r.reqCtx.Ctx)
	return tree, nil
}

func NewAutoInstallCheckReconciler(reqCtx intctrlutil.RequestCtx, buildStageCtx func() stageCtx) kubebuilderx.Reconciler {

	return &autoInstallCheckReconciler{
		stageCtx: buildStageCtx(),
	}
}

var _ kubebuilderx.Reconciler = &autoInstallCheckReconciler{}
