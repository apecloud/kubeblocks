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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// ParameterReconciler reconciles a Parameter object
type ParameterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=parameters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=parameters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=parameters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ParameterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Recorder: r.Recorder,
		Log: log.FromContext(ctx).
			WithName("ParameterReconciler").
			WithValues("Namespace", req.Namespace, "Parameter", req.Name),
	}

	parameter := &parametersv1alpha1.Parameter{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, parameter); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, parameter, constant.ConfigFinalizerName, nil)
	if res != nil {
		return *res, err
	}
	return r.reconcile(reqCtx, parameter)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ParameterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&parametersv1alpha1.Parameter{}).
		Complete(r)
}

func (r *ParameterReconciler) handleComponent(rctx *ReconcileContext, updatedParameters appsv1.ComponentParameters, parameter *parametersv1alpha1.Parameter) error {
	configmaps, err := resolveComponentRefConfigMap(rctx)
	if err != nil {
		return err
	}

	handles := []reconfigureReconcileHandle{
		prepareResources,
		classifyParameters(updatedParameters, configmaps),
		updateCustomTemplates,
		updateParameters,
		syncComponentParametersStatus(configmaps),
	}

	for _, handle := range handles {
		if err := handle(rctx, parameter); err != nil {
			return err
		}
	}
	return nil
}

func (r *ParameterReconciler) reconcile(reqCtx intctrlutil.RequestCtx, parameter *parametersv1alpha1.Parameter) (ctrl.Result, error) {
	if intctrlutil.ParametersTerminalPhases(parameter.Status, parameter.Generation) {
		return intctrlutil.Reconciled()
	}

	patch := parameter.DeepCopy()
	rctxs, params := r.generateParameterTaskContext(reqCtx, parameter)
	for i, rctx := range rctxs {
		if err := r.handleComponent(rctx, params[i], parameter); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}

	return updateParameterStatus(reqCtx, r.Client, parameter, patch)
}

func (r *ParameterReconciler) generateParameterTaskContext(reqCtx intctrlutil.RequestCtx, parameter *parametersv1alpha1.Parameter) ([]*ReconcileContext, []appsv1.ComponentParameters) {
	var rctxs []*ReconcileContext
	var params []appsv1.ComponentParameters
	for _, component := range parameter.Spec.ComponentParameters {
		params = append(params, component.Parameters)
		rctxs = append(rctxs, newParameterReconcileContext(reqCtx,
			&configctrl.ResourceCtx{
				Context:       reqCtx.Ctx,
				Client:        r.Client,
				Namespace:     parameter.Namespace,
				ClusterName:   parameter.Spec.ClusterName,
				ComponentName: component.ComponentName,
			}, nil, "", nil))
	}
	return rctxs, params
}

func updateParameterStatus(reqCtx intctrlutil.RequestCtx, cli client.Client, parameter *parametersv1alpha1.Parameter, patch *parametersv1alpha1.Parameter) (ctrl.Result, error) {
	finished := syncParameterStatus(&parameter.Status)
	parameter.Status.ObservedGeneration = parameter.Generation
	if err := cli.Status().Patch(reqCtx.Ctx, parameter, client.MergeFrom(patch)); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if finished {
		return intctrlutil.Reconciled()
	}
	return intctrlutil.RequeueAfter(ConfigReconcileInterval, reqCtx.Log, "")
}

func syncParameterStatus(parameterStatus *parametersv1alpha1.ParameterStatus) bool {
	var finished = true

	for _, status := range parameterStatus.ReconfiguringStatus {
		switch {
		case status.Phase == parametersv1alpha1.CMergeFailedPhase:
			parameterStatus.Phase = parametersv1alpha1.CMergeFailedPhase
			return true
		case status.Phase == parametersv1alpha1.CFailedAndPausePhase:
			parameterStatus.Phase = parametersv1alpha1.CFailedAndPausePhase
			return true
		case status.Phase != parametersv1alpha1.CFinishedPhase:
			parameterStatus.Phase = parametersv1alpha1.CRunningPhase
			finished = false
		}
	}
	return finished
}
