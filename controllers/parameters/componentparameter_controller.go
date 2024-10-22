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

package parameters

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// ComponentParameterReconciler reconciles a ComponentParameter object
type ComponentParameterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=componentparameters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=componentparameters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=componentparameters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ComponentParameterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Recorder: r.Recorder,
		Log: log.FromContext(ctx).
			WithName("ComponentParameterReconciler").
			WithValues("Namespace", req.Namespace, "ComponentParameter", req.Name),
	}

	componentParam := &parametersv1alpha1.ComponentParameter{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, componentParam); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, componentParam, constant.ConfigFinalizerName, nil)
	if res != nil {
		return *res, err
	}
	return r.reconcile(reqCtx, componentParam)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentParameterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&parametersv1alpha1.ComponentParameter{}).
		Complete(r)
}

func (r *ComponentParameterReconciler) reconcile(reqCtx intctrlutil.RequestCtx, componentParam *parametersv1alpha1.ComponentParameter) (ctrl.Result, error) {
	panic("")
}
