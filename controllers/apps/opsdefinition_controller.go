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
	"text/template"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// OpsDefinitionReconciler reconciles a OpsDefinition object
type OpsDefinitionReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=opsdefinitions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=opsdefinitions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=opsdefinitions/finalizers,verbs=update

func (r *OpsDefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("opsDefinition", req.NamespacedName),
	}

	opsDef := &appsv1alpha1.OpsDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, opsDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, opsDef, opsDefinitionFinalizerName, func() (*ctrl.Result, error) {
		return nil, nil
	})
	if res != nil {
		return *res, err
	}

	if opsDef.Status.ObservedGeneration == opsDef.Generation &&
		opsDef.Status.Phase == appsv1alpha1.AvailablePhase {
		return intctrlutil.Reconciled()
	}

	// check go template of the expression.
	for _, v := range opsDef.Spec.PreConditions {
		if v.Rule == nil {
			continue
		}
		if _, err = template.New("opsDefTemplate").Parse(v.Rule.Expression); err != nil {
			if patchErr := r.updateStatusUnavailable(reqCtx, opsDef, err); patchErr != nil {
				return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
			}
			return intctrlutil.Reconciled()
		}
	}

	// TODO: check serviceKind, connectionCredentialName and serviceName
	statusPatch := client.MergeFrom(opsDef.DeepCopy())
	opsDef.Status.ObservedGeneration = opsDef.Generation
	opsDef.Status.Phase = appsv1alpha1.AvailablePhase
	if err = r.Client.Status().Patch(reqCtx.Ctx, opsDef, statusPatch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	intctrlutil.RecordCreatedEvent(r.Recorder, opsDef)
	return intctrlutil.Reconciled()
}

func (r *OpsDefinitionReconciler) updateStatusUnavailable(reqCtx intctrlutil.RequestCtx, opsDef *appsv1alpha1.OpsDefinition, err error) error {
	statusPatch := client.MergeFrom(opsDef.DeepCopy())
	opsDef.Status.Phase = appsv1alpha1.UnavailablePhase
	opsDef.Status.ObservedGeneration = opsDef.Generation
	opsDef.Status.Message = err.Error()
	return r.Client.Status().Patch(reqCtx.Ctx, opsDef, statusPatch)
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpsDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.OpsDefinition{}).
		Complete(r)
}
