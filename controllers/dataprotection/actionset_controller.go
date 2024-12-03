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

package dataprotection

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

// ActionSetReconciler reconciles a ActionSet object
type ActionSetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=actionsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=actionsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=actionsets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the actionset closer to the desired state.
func (r *ActionSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("actionSet", req.Name),
		Recorder: r.Recorder,
	}

	actionSet := &dpv1alpha1.ActionSet{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, actionSet); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, actionSet, dptypes.DataProtectionFinalizerName,
		func() (*ctrl.Result, error) {
			return nil, r.deleteExternalResources(reqCtx, actionSet)
		})
	if res != nil {
		return *res, err
	}

	if actionSet.Status.ObservedGeneration == actionSet.Generation && actionSet.Status.Phase.IsAvailable() {
		return ctrl.Result{}, nil
	}

	patchStatus := func(phase dpv1alpha1.Phase, message string) error {
		patch := client.MergeFrom(actionSet.DeepCopy())
		actionSet.Status.Phase = phase
		actionSet.Status.Message = message
		actionSet.Status.ObservedGeneration = actionSet.Generation
		return r.Client.Status().Patch(reqCtx.Ctx, actionSet, patch)
	}

	phase := dpv1alpha1.AvailablePhase
	var message string
	if err = r.validate(actionSet); err != nil {
		phase = dpv1alpha1.UnavailablePhase
		message = err.Error()
	}

	if err = patchStatus(phase, message); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	intctrlutil.RecordCreatedEvent(r.Recorder, actionSet)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ActionSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&dpv1alpha1.ActionSet{}).
		Complete(r)
}

func (r *ActionSetReconciler) deleteExternalResources(
	_ intctrlutil.RequestCtx,
	_ *dpv1alpha1.ActionSet) error {
	return nil
}

func (r *ActionSetReconciler) validate(actionset *dpv1alpha1.ActionSet) error {
	validateWithParameters := func(withParameters []string) error {
		if len(withParameters) == 0 {
			return nil
		}
		schema := actionset.Spec.ParametersSchema
		if schema == nil || schema.OpenAPIV3Schema == nil || len(schema.OpenAPIV3Schema.Properties) == 0 {
			return fmt.Errorf("the parametersSchema is invalid")
		}
		properties := schema.OpenAPIV3Schema.Properties
		for _, parameter := range withParameters {
			if _, ok := properties[parameter]; !ok {
				return fmt.Errorf("parameter %s is not defined in the parametersSchema", parameter)
			}
		}
		return nil
	}

	// validate withParameters
	if actionset.Spec.Backup != nil {
		if err := validateWithParameters(actionset.Spec.Backup.WithParameters); err != nil {
			return fmt.Errorf("fails to validate backup withParameters: %v", err)
		}
	}
	if actionset.Spec.Restore != nil {
		if err := validateWithParameters(actionset.Spec.Restore.WithParameters); err != nil {
			return fmt.Errorf("fails to validate restore withParameters: %v", err)
		}
	}
	return nil
}
