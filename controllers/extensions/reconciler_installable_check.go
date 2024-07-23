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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type installableCheckReconciler struct {
	stageCtx
}

func (r *installableCheckReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
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

func (r *installableCheckReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
	addon := tree.GetRoot().(*extensionsv1alpha1.Addon)
	// XValidation was introduced as an alpha feature in Kubernetes v1.23 and requires additional enablement.
	// It became more stable after Kubernetes 1.25. Users may encounter error in Kubernetes versions prior to 1.25.
	// additional check to the addon YAML to ensure support for Kubernetes versions prior to 1.25
	if err := checkAddonSpec(addon); err != nil {
		setAddonErrorConditions(r.reqCtx.Ctx, &r.stageCtx, addon, true, true, AddonCheckError, err.Error())
		r.setReconciled()
		return tree, nil
	}

	r.reqCtx.Log.V(1).Info("installableCheckReconciler", "phase", addon.Status.Phase)
	fmt.Println("installableCheckReconciler, phase: ", addon.Status.Phase)
	// check the annotations constraint about Kubeblocks Version
	check, err := checkAnnotationsConstraint(r.reqCtx.Ctx, r.reconciler, addon)
	if err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, r.reqCtx.Log, "")
		r.updateResultNErr(&res, err)
		return tree, nil
	}
	if !check {
		r.setReconciled()
		return tree, nil
	}

	if addon.Spec.Installable == nil {
		return tree, nil
	}
	// proceed if has specified addon.spec.installSpec
	if addon.Spec.InstallSpec != nil {
		return tree, nil
	}
	if addon.Annotations != nil && addon.Annotations[SkipInstallableCheck] == trueVal {
		r.reconciler.Event(addon, corev1.EventTypeWarning, InstallableCheckSkipped,
			"Installable check skipped.")
		return tree, nil
	}
	switch addon.Status.Phase {
	case extensionsv1alpha1.AddonEnabling, extensionsv1alpha1.AddonDisabling:
		return tree, nil
	}
	for _, s := range addon.Spec.Installable.Selectors {
		if s.MatchesFromConfig() {
			continue
		}
		patch := client.MergeFrom(addon.DeepCopy())
		addon.Status.ObservedGeneration = addon.Generation
		addon.Status.Phase = extensionsv1alpha1.AddonDisabled
		meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
			Type:               extensionsv1alpha1.ConditionTypeChecked,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: addon.Generation,
			Reason:             InstallableRequirementUnmatched,
			Message:            "spec.installable.selectors has no matching requirement.",
			LastTransitionTime: metav1.Now(),
		})

		if err := r.reconciler.Status().Patch(r.reqCtx.Ctx, addon, patch); err != nil {
			r.setRequeueWithErr(err, "")
			return tree, nil
		}
		r.reconciler.Event(addon, corev1.EventTypeWarning, InstallableRequirementUnmatched,
			fmt.Sprintf("Does not meet installable requirements for key %v", s))
		r.setReconciled()
		return tree, nil
	}

	return tree, nil
}

func NewInstallableCheckReconciler(reqCtx intctrlutil.RequestCtx, buildStageCtx func() stageCtx) kubebuilderx.Reconciler {
	return &installableCheckReconciler{
		stageCtx: buildStageCtx(),
	}
}

var _ kubebuilderx.Reconciler = &installableCheckReconciler{}
