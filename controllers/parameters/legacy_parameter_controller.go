/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package parameters

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	parameterDeprecatedMessage = "Parameter is deprecated; use OpsRequest type Reconfiguring for runtime updates and cluster init parameters for initialization"
)

// LegacyParameterReconciler marks Parameter requests as unsupported.
type LegacyParameterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=parameters,verbs=get;list;watch
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=parameters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=parameters/finalizers,verbs=update

func (r *LegacyParameterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Recorder: r.Recorder,
		Log: log.FromContext(ctx).
			WithName("LegacyParameterReconciler").
			WithValues("Namespace", req.Namespace, "Parameter", req.Name),
	}

	parameter := &parametersv1alpha1.Parameter{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, parameter); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if !parameter.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(parameter, constant.ConfigFinalizerName) {
			return intctrlutil.Reconciled()
		}
		patch := client.MergeFrom(parameter.DeepCopy())
		controllerutil.RemoveFinalizer(parameter, constant.ConfigFinalizerName)
		if err := r.Client.Patch(reqCtx.Ctx, parameter, patch); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	if parameter.Status.ObservedGeneration == parameter.Generation &&
		parameter.Status.Phase == parametersv1alpha1.CMergeFailedPhase &&
		parameter.Status.Message == parameterDeprecatedMessage {
		return intctrlutil.Reconciled()
	}

	patch := client.MergeFrom(parameter.DeepCopy())
	parameter.Status.ObservedGeneration = parameter.Generation
	parameter.Status.Phase = parametersv1alpha1.CMergeFailedPhase
	parameter.Status.Message = parameterDeprecatedMessage
	if err := r.Client.Status().Patch(reqCtx.Ctx, parameter, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *LegacyParameterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&parametersv1alpha1.Parameter{}).
		Complete(r)
}
