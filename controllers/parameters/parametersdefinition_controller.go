/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	paramutil "github.com/apecloud/kubeblocks/pkg/parameters"
	cfgcore "github.com/apecloud/kubeblocks/pkg/parameters/core"
	"github.com/apecloud/kubeblocks/pkg/parameters/openapi"
	"github.com/apecloud/kubeblocks/pkg/parameters/validate"
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
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&parametersv1alpha1.ParametersDefinition{}).
		Watches(
			&appsv1.ComponentDefinition{},
			handler.EnqueueRequestsFromMapFunc(r.mapCmpdToPDs),
		).
		Complete(r)
}

func (r *ParametersDefinitionReconciler) reconcile(reqCtx intctrlutil.RequestCtx, parametersDef *parametersv1alpha1.ParametersDefinition) (ctrl.Result, error) {

	if err := r.validate(reqCtx, parametersDef); err != nil {
		return r.failed(reqCtx, parametersDef, err)
	}

	// Automatically convert cue to openAPISchema.
	if err := updateParametersSchema(parametersDef, r.Client, reqCtx.Ctx); err != nil {
		return r.failed(reqCtx, parametersDef, err)
	}

	phaseChanged, err := r.status(reqCtx, parametersDef, parametersv1alpha1.PDAvailablePhase)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if phaseChanged {
		intctrlutil.RecordCreatedEvent(r.Recorder, parametersDef)
	}
	return intctrlutil.Reconciled()
}

func (r *ParametersDefinitionReconciler) validate(reqCtx intctrlutil.RequestCtx, parametersDef *parametersv1alpha1.ParametersDefinition) error {
	if err := validateSchema(parametersDef); err != nil {
		return err
	}
	if err := r.validateTemplateName(reqCtx.Ctx, parametersDef); err != nil {
		return err
	}
	return nil
}

func (r *ParametersDefinitionReconciler) failed(reqCtx intctrlutil.RequestCtx, parametersDef *parametersv1alpha1.ParametersDefinition, err error) (ctrl.Result, error) {
	if _, err1 := r.status(reqCtx, parametersDef, parametersv1alpha1.PDUnavailablePhase); err1 != nil {
		return intctrlutil.CheckedRequeueWithError(err1, reqCtx.Log, "")
	}
	return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
}

func (r *ParametersDefinitionReconciler) status(reqCtx intctrlutil.RequestCtx, parametersDef *parametersv1alpha1.ParametersDefinition, phase parametersv1alpha1.ParametersDescPhase) (bool, error) {
	base := parametersDef.DeepCopy()
	patch := client.MergeFrom(base)
	phaseChanged := parametersDef.Status.Phase != phase
	parametersDef.Status.Phase = phase
	parametersDef.Status.ObservedGeneration = parametersDef.Generation
	if reflect.DeepEqual(parametersDef.Status, base.Status) {
		return false, nil
	}
	return phaseChanged, r.Client.Status().Patch(reqCtx.Ctx, parametersDef, patch)
}

func (r *ParametersDefinitionReconciler) mapCmpdToPDs(ctx context.Context, obj client.Object) []reconcile.Request {
	cmpd, ok := obj.(*appsv1.ComponentDefinition)
	if !ok {
		return nil
	}
	paramsDefList := &parametersv1alpha1.ParametersDefinitionList{}
	if err := r.Client.List(ctx, paramsDefList); err != nil {
		log.FromContext(ctx).WithName("ParametersDefinitionReconcile").Error(err,
			"failed to list ParametersDefinitions for ComponentDefinition watch", "ComponentDefinition", cmpd.Name)
		return nil
	}
	requests := make([]reconcile.Request, 0, len(paramsDefList.Items))
	for i := range paramsDefList.Items {
		paramsDef := &paramsDefList.Items[i]
		matched, err := paramutil.MatchParametersDefinition(cmpd, paramsDef)
		if err != nil {
			log.FromContext(ctx).WithName("ParametersDefinitionReconcile").Error(err,
				"failed to match ParametersDefinition for ComponentDefinition watch",
				"ParametersDefinition", paramsDef.Name,
				"ComponentDefinition", cmpd.Name)
			continue
		}
		if !matched {
			continue
		}
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: paramsDef.Name},
		})
	}
	return requests
}

func (r *ParametersDefinitionReconciler) validateTemplateName(ctx context.Context, parametersDef *parametersv1alpha1.ParametersDefinition) error {
	if parametersDef.Spec.ComponentDef == "" || parametersDef.Spec.TemplateName == "" {
		return nil
	}
	cmpdList := &appsv1.ComponentDefinitionList{}
	if err := r.Client.List(ctx, cmpdList); err != nil {
		return err
	}
	for i := range cmpdList.Items {
		cmpd := &cmpdList.Items[i]
		matched, err := paramutil.MatchParametersDefinition(cmpd, parametersDef)
		if err != nil {
			return err
		}
		if !matched {
			continue
		}
		if !hasConfigTemplate(cmpd, parametersDef.Spec.TemplateName) {
			return fmt.Errorf("parametersdefinition[%s] references config template[%s], but matched ComponentDefinition[%s] does not define it",
				parametersDef.Name, parametersDef.Spec.TemplateName, cmpd.Name)
		}
	}
	return nil
}

func hasConfigTemplate(cmpd *appsv1.ComponentDefinition, templateName string) bool {
	for _, config := range cmpd.Spec.Configs {
		if config.Name == templateName {
			return true
		}
	}
	return false
}

func (r *ParametersDefinitionReconciler) deletionHandler(parametersDef *parametersv1alpha1.ParametersDefinition, reqCtx intctrlutil.RequestCtx) func() (*ctrl.Result, error) {
	recordEvent := func() {
		r.Recorder.Event(parametersDef, corev1.EventTypeWarning, "ExistsReferencedResources",
			"cannot be deleted because of existing referencing of ClusterDefinition.")
	}

	return func() (*ctrl.Result, error) {
		if parametersDef.Status.Phase != parametersv1alpha1.PDDeletingPhase {
			_, err := r.status(reqCtx, parametersDef, parametersv1alpha1.PDDeletingPhase)
			if err != nil {
				return nil, err
			}
		}
		if res, err := intctrlutil.ValidateReferenceCR(reqCtx, r.Client, parametersDef,
			cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(parametersDef.GetName()),
			recordEvent, &parametersv1alpha1.ParamConfigRendererList{}); res != nil || err != nil {
			return res, err
		}
		return nil, nil
	}
}

func validateSchema(parametersDef *parametersv1alpha1.ParametersDefinition) error {
	schema := parametersDef.Spec.ParametersSchema
	if schema == nil || len(schema.CUE) == 0 {
		return nil
	}
	return validate.CueValidate(schema.CUE)
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
