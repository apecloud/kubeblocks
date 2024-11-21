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
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/openapi"
	"github.com/apecloud/kubeblocks/pkg/configuration/validate"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// ParametersDefinitionReconciler reconciles a ParametersDefinition object
type ParametersDefinitionReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=parametersdefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=parametersdefinitions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=parametersdefinitions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ParametersDefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Recorder: r.Recorder,
		Log: log.FromContext(ctx).
			WithName("ParametersDefinitionReconcile").
			WithValues("ParametersDefinition", req.Name),
	}

	parametersDef := &parametersv1alpha1.ParametersDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, parametersDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, parametersDef, constant.ConfigFinalizerName, r.deletionHandler(parametersDef, reqCtx))
	if res != nil {
		return *res, err
	}
	return r.reconcile(reqCtx, parametersDef)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ParametersDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewNamespacedControllerManagedBy(mgr).
		For(&parametersv1alpha1.ParametersDefinition{}).
		Complete(r)
}

func (r *ParametersDefinitionReconciler) reconcile(reqCtx intctrlutil.RequestCtx, parametersDef *parametersv1alpha1.ParametersDefinition) (ctrl.Result, error) {

	if intctrlutil.ParametersDefinitionTerminalPhases(parametersDef.Status, parametersDef.Generation) {
		return intctrlutil.Reconciled()
	}

	if ok, err := checkParametersSchema(reqCtx, parametersDef); !ok || err != nil {
		return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, "ValidateConfigurationTemplate")
	}

	// Automatically convert cue to openAPISchema.
	if err := updateParametersSchema(parametersDef, r.Client, reqCtx.Ctx); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to generate openAPISchema")
	}

	if err := updateParamDefinitionStatus(r.Client, reqCtx, parametersDef, parametersv1alpha1.PDAvailablePhase); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	intctrlutil.RecordCreatedEvent(r.Recorder, parametersDef)
	return intctrlutil.Reconciled()
}

func (r *ParametersDefinitionReconciler) deletionHandler(parametersDef *parametersv1alpha1.ParametersDefinition, reqCtx intctrlutil.RequestCtx) func() (*ctrl.Result, error) {
	recordEvent := func() {
		r.Recorder.Event(parametersDef, corev1.EventTypeWarning, "ExistsReferencedResources",
			"cannot be deleted because of existing referencing of ClusterDefinition.")
	}

	return func() (*ctrl.Result, error) {
		if parametersDef.Status.Phase != parametersv1alpha1.PDDeletingPhase {
			err := updateParamDefinitionStatus(r.Client, reqCtx, parametersDef, parametersv1alpha1.PDDeletingPhase)
			if err != nil {
				return nil, err
			}
		}
		if res, err := intctrlutil.ValidateReferenceCR(reqCtx, r.Client, parametersDef,
			cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(parametersDef.GetName()),
			recordEvent, &parametersv1alpha1.ParameterDrivenConfigRenderList{}); res != nil || err != nil {
			return res, err
		}
		return nil, nil
	}
}

func updateParamDefinitionStatus(cli client.Client, ctx intctrlutil.RequestCtx, parametersDef *parametersv1alpha1.ParametersDefinition, phase parametersv1alpha1.ParametersDescPhase) error {
	patch := client.MergeFrom(parametersDef.DeepCopy())
	parametersDef.Status.Phase = phase
	parametersDef.Status.ObservedGeneration = parametersDef.Generation
	return cli.Status().Patch(ctx.Ctx, parametersDef, patch)
}

func checkParametersSchema(ctx intctrlutil.RequestCtx, parametersDef *parametersv1alpha1.ParametersDefinition) (bool, error) {
	// validate configuration template
	validateConfigSchema := func(ccSchema *parametersv1alpha1.ParametersSchema) (bool, error) {
		if ccSchema == nil || len(ccSchema.CUE) == 0 {
			return true, nil
		}
		err := validate.CueValidate(ccSchema.CUE)
		return err == nil, err
	}

	// validate schema
	if ok, err := validateConfigSchema(parametersDef.Spec.ParametersSchema); !ok || err != nil {
		ctx.Log.Error(err, "failed to validate template schema!",
			"configMapName", fmt.Sprintf("%v", parametersDef.Spec.ParametersSchema))
		return ok, err
	}
	return true, nil
}

func updateParametersSchema(parametersDef *parametersv1alpha1.ParametersDefinition, cli client.Client, ctx context.Context) error {
	schema := parametersDef.Spec.ParametersSchema
	if schema == nil || schema.CUE == "" {
		return nil
	}

	// Because the conversion of cue to openAPISchema is restricted, and the definition of some cue may not be converted into openAPISchema, and won't return error.
	openAPISchema, err := openapi.GenerateOpenAPISchema(schema.CUE, schema.TopLevelKey)
	if err != nil {
		return err
	}
	if openAPISchema == nil {
		return nil
	}
	if reflect.DeepEqual(openAPISchema, schema.SchemaInJSON) {
		return nil
	}

	ccPatch := client.MergeFrom(parametersDef.DeepCopy())
	parametersDef.Spec.ParametersSchema.SchemaInJSON = openAPISchema
	return cli.Patch(ctx, parametersDef, ccPatch)
}
