/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package apps

import (
	"context"
	"fmt"

	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/class"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentclassdefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentclassdefinitions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentclassdefinitions/finalizers,verbs=update

type ComponentClassReconciler struct {
	client.Client
	Scheme   *k8sruntime.Scheme
	Recorder record.EventRecorder
}

func (r *ComponentClassReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("classDefinition", req.NamespacedName),
		Recorder: r.Recorder,
	}

	classDefinition := &appsv1alpha1.ComponentClassDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, classDefinition); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, classDefinition, constant.DBClusterFinalizerName, func() (*ctrl.Result, error) {
		// TODO validate if existing cluster reference classes being deleted
		return nil, nil
	})
	if res != nil {
		return *res, err
	}

	ml := []client.ListOption{
		client.HasLabels{types.ResourceConstraintProviderLabelKey},
	}
	constraintsList := &appsv1alpha1.ComponentResourceConstraintList{}
	if err := r.Client.List(reqCtx.Ctx, constraintsList, ml...); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	constraintsMap := make(map[string]appsv1alpha1.ComponentResourceConstraint)
	for idx := range constraintsList.Items {
		cf := constraintsList.Items[idx]
		if _, ok := cf.GetLabels()[types.ResourceConstraintProviderLabelKey]; !ok {
			continue
		}
		constraintsMap[cf.GetName()] = cf
	}

	if classDefinition.Status.ObservedGeneration == classDefinition.Generation {
		return intctrlutil.Reconciled()
	}

	classes, err := class.ParseComponentClasses(*classDefinition)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "parse component classes failed")
	}

	patch := client.MergeFrom(classDefinition.DeepCopy())
	var classList []appsv1alpha1.ComponentClassInstance
	for _, v := range classes {
		constraint, ok := constraintsMap[v.ResourceConstraintRef]
		if !ok {
			return intctrlutil.CheckedRequeueWithError(nil, reqCtx.Log, fmt.Sprintf("resource constraint %s not found", v.ResourceConstraintRef))
		}
		if !constraint.MatchClass(v) {
			return intctrlutil.CheckedRequeueWithError(nil, reqCtx.Log, fmt.Sprintf("class %s does not conform to constraint %s", v.Name, v.ResourceConstraintRef))
		}
		classList = append(classList, *v)
	}
	classDefinition.Status.Classes = classList
	classDefinition.Status.ObservedGeneration = classDefinition.Generation
	if err = r.Client.Status().Patch(ctx, classDefinition, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "patch component class status failed")
	}

	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&appsv1alpha1.ComponentClassDefinition{}).Complete(r)
}
