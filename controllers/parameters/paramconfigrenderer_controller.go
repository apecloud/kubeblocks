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
	"fmt"

	corev1 "k8s.io/api/core/v1"
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
	"github.com/apecloud/kubeblocks/pkg/generics"
)

// ParameterDrivenConfigRenderReconciler reconciles a ParamConfigRenderer object
type ParameterDrivenConfigRenderReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=paramconfigrenderers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=paramconfigrenderers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=paramconfigrenderers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ParameterDrivenConfigRenderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Recorder: r.Recorder,
		Log: log.FromContext(ctx).
			WithName("ParameterDrivenConfigRenderReconciler").
			WithValues("ParamConfigRenderer", req.Name),
	}

	parameterTemplate := &parametersv1alpha1.ParamConfigRenderer{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, parameterTemplate); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, parameterTemplate, constant.ConfigFinalizerName, nil)
	if res != nil {
		return *res, err
	}
	return r.reconcile(reqCtx, parameterTemplate)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ParameterDrivenConfigRenderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&parametersv1alpha1.ParamConfigRenderer{}).
		Complete(r)
}

func (r *ParameterDrivenConfigRenderReconciler) reconcile(reqCtx intctrlutil.RequestCtx, parameterTemplate *parametersv1alpha1.ParamConfigRenderer) (ctrl.Result, error) {
	if intctrlutil.ParametersDrivenConfigRenderTerminalPhases(parameterTemplate.Status, parameterTemplate.Generation) {
		return intctrlutil.Reconciled()
	}
	cmpd := &appsv1.ComponentDefinition{}
	if err := r.Get(reqCtx.Ctx, client.ObjectKey{Name: parameterTemplate.Spec.ComponentDef}, cmpd); err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}
	if err := fillParameterTemplate(reqCtx, r.Client, parameterTemplate, cmpd); err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}
	if err := r.validate(reqCtx, r.Client, &parameterTemplate.Spec, cmpd); err != nil {
		if err2 := r.unavailable(reqCtx.Ctx, r.Client, parameterTemplate, err); err2 != nil {
			return intctrlutil.RequeueWithError(err2, reqCtx.Log, "")
		}
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}
	if err := r.available(reqCtx.Ctx, r.Client, parameterTemplate); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	intctrlutil.RecordCreatedEvent(r.Recorder, parameterTemplate)
	return intctrlutil.Reconciled()
}

func (r *ParameterDrivenConfigRenderReconciler) validate(ctx intctrlutil.RequestCtx, cli client.Client, parameterTemplate *parametersv1alpha1.ParamConfigRendererSpec, cmpd *appsv1.ComponentDefinition) error {
	if err := validateParametersDefs(ctx, cli, parameterTemplate.ParametersDefs); err != nil {
		return err
	}
	if err := validateParametersConfigs(parameterTemplate.Configs, cmpd.Spec.Configs); err != nil {
		return err
	}
	return nil
}

func (r *ParameterDrivenConfigRenderReconciler) available(ctx context.Context, cli client.Client, parameterTemplate *parametersv1alpha1.ParamConfigRenderer) error {
	return r.status(ctx, cli, parameterTemplate, parametersv1alpha1.PDAvailablePhase, nil)
}

func (r *ParameterDrivenConfigRenderReconciler) unavailable(ctx context.Context, cli client.Client, parameterTemplate *parametersv1alpha1.ParamConfigRenderer, err error) error {
	return r.status(ctx, cli, parameterTemplate, parametersv1alpha1.PDUnavailablePhase, err)
}

func (r *ParameterDrivenConfigRenderReconciler) status(ctx context.Context, cli client.Client, parameterRender *parametersv1alpha1.ParamConfigRenderer, phase parametersv1alpha1.ParametersDescPhase, err error) error {
	patch := client.MergeFrom(parameterRender.DeepCopy())
	parameterRender.Status.ObservedGeneration = parameterRender.Generation
	parameterRender.Status.Phase = phase
	parameterRender.Status.Message = ""
	if err != nil {
		parameterRender.Status.Message = err.Error()
	}
	return cli.Status().Patch(ctx, parameterRender, patch)
}

func fillParameterTemplate(reqCtx intctrlutil.RequestCtx, cli client.Client, template *parametersv1alpha1.ParamConfigRenderer, cmpd *appsv1.ComponentDefinition) (err error) {
	var tpls map[string]*corev1.ConfigMap

	match := func(spec parametersv1alpha1.ComponentConfigDescription) bool {
		return spec.TemplateName == ""
	}
	resolveConfigTemplate := func(config string) string {
		for name, configTemplate := range tpls {
			if _, ok := configTemplate.Data[config]; ok {
				return name
			}
		}
		return ""
	}

	if generics.CountFunc(template.Spec.Configs, match) == 0 {
		return nil
	}
	if tpls, err = configctrl.ResolveComponentTemplate(reqCtx.Ctx, cli, cmpd); err != nil {
		return err
	}
	deepCopy := template.DeepCopy()
	for i, config := range deepCopy.Spec.Configs {
		if tplName := resolveConfigTemplate(config.Name); tplName != "" {
			deepCopy.Spec.Configs[i].TemplateName = tplName
		}
	}
	return cli.Patch(reqCtx.Ctx, deepCopy, client.MergeFrom(template))
}

func validateParametersConfigs(configs []parametersv1alpha1.ComponentConfigDescription, templates []appsv1.ComponentTemplateSpec) error {
	for _, config := range configs {
		match := func(spec appsv1.ComponentTemplateSpec) bool {
			return config.TemplateName == spec.Name
		}
		if len(generics.FindFunc(templates, match)) == 0 {
			return fmt.Errorf("config template[%s] not found in component definition", config.TemplateName)
		}
	}
	return nil
}

func validateParametersDefs(reqCtx intctrlutil.RequestCtx, cli client.Client, paramsDefs []string) error {
	paramsDefObjs := make(map[string]*parametersv1alpha1.ParametersDefinition, len(paramsDefs))
	for _, paramsDef := range paramsDefs {
		obj := &parametersv1alpha1.ParametersDefinition{}
		if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: paramsDef}, obj); err != nil {
			return err
		}
		if def, ok := paramsDefObjs[obj.Spec.FileName]; ok {
			return fmt.Errorf("config file[%s] has been defined in other parametersdefinition[%s]", obj.Spec.FileName, def.Name)
		}
	}
	return nil
}
