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
	"encoding/json"
	"fmt"
	"reflect"
	"slices"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	configcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

// ComponentDrivenParameterReconciler reconciles a Parameter object
type ComponentDrivenParameterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=components,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=components/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=components/finalizers,verbs=update

// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=componentparameters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=componentparameters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=componentparameters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ComponentDrivenParameterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Recorder: r.Recorder,
		Log: log.FromContext(ctx).
			WithName("ComponentParameterReconciler").
			WithValues("Namespace", req.Namespace, "Parameter", req.Name),
	}

	comp := &appsv1.Component{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, comp); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return r.reconcile(reqCtx, comp)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentDrivenParameterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Component{}).
		Complete(r)
}

func (r *ComponentDrivenParameterReconciler) reconcile(reqCtx intctrlutil.RequestCtx, component *appsv1.Component) (ctrl.Result, error) {
	var err error
	var existingObject *parametersv1alpha1.ComponentParameter
	var expectedObject *parametersv1alpha1.ComponentParameter

	if existingObject, err = runningComponentParameter(reqCtx, r.Client, component); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if model.IsObjectDeleting(component) {
		return r.delete(reqCtx, existingObject)
	}
	if expectedObject, err = buildComponentParameter(reqCtx, r.Client, component); err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}

	switch {
	case expectedObject == nil:
		return r.delete(reqCtx, existingObject)
	case existingObject == nil:
		return r.create(reqCtx, expectedObject)
	default:
		return r.update(reqCtx, expectedObject, existingObject)
	}
}

func (r *ComponentDrivenParameterReconciler) create(reqCtx intctrlutil.RequestCtx, object *parametersv1alpha1.ComponentParameter) (ctrl.Result, error) {
	if err := r.Client.Create(reqCtx.Ctx, object); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	reqCtx.Log.Info("ComponentParameter created")
	intctrlutil.RecordCreatedEvent(r.Recorder, object)
	return intctrlutil.Reconciled()
}

func (r *ComponentDrivenParameterReconciler) delete(reqCtx intctrlutil.RequestCtx, object *parametersv1alpha1.ComponentParameter) (ctrl.Result, error) {
	if object == nil {
		return intctrlutil.Reconciled()
	}
	if err := r.Client.Delete(reqCtx.Ctx, object); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	reqCtx.Log.Info("ComponentParameter deleted")
	return intctrlutil.Reconciled()
}

func (r *ComponentDrivenParameterReconciler) update(reqCtx intctrlutil.RequestCtx, expected, existing *parametersv1alpha1.ComponentParameter) (ctrl.Result, error) {
	mergedObject := r.mergeComponentParameter(expected, existing)
	if reflect.DeepEqual(mergedObject, existing) {
		return intctrlutil.Reconciled()
	}
	if err := r.Client.Patch(reqCtx.Ctx, mergedObject, client.MergeFrom(existing)); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func runningComponentParameter(reqCtx intctrlutil.RequestCtx, reader client.Reader, comp *appsv1.Component) (*parametersv1alpha1.ComponentParameter, error) {
	var componentParameter = &parametersv1alpha1.ComponentParameter{}

	clusterName, _ := component.GetClusterName(comp)
	componentName, _ := component.ShortName(clusterName, comp.Name)

	parameterKey := types.NamespacedName{
		Name:      configcore.GenerateComponentConfigurationName(clusterName, componentName),
		Namespace: comp.Namespace,
	}
	if err := reader.Get(reqCtx.Ctx, parameterKey, componentParameter); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return componentParameter, nil
}

func getCompDefinition(ctx context.Context, cli client.Reader, comp *appsv1.Component) (*appsv1.ComponentDefinition, error) {
	compKey := types.NamespacedName{
		Name: comp.Spec.CompDef,
	}
	cmpd := &appsv1.ComponentDefinition{}
	if err := cli.Get(ctx, compKey, cmpd); err != nil {
		return nil, err
	}
	if cmpd.Status.Phase != appsv1.AvailablePhase {
		return nil, fmt.Errorf("the referenced ComponentDefinition is unavailable: %s", cmpd.Name)
	}
	return cmpd, nil
}

func buildComponentParameter(reqCtx intctrlutil.RequestCtx, reader client.Reader, comp *appsv1.Component) (*parametersv1alpha1.ComponentParameter, error) {
	var err error
	var cmpd *appsv1.ComponentDefinition

	if cmpd, err = getCompDefinition(reqCtx.Ctx, reader, comp); err != nil {
		return nil, err
	}
	if len(cmpd.Spec.Configs) == 0 {
		return nil, nil
	}

	configRender, paramsDefs, err := intctrlutil.ResolveCmpdParametersDefs(reqCtx.Ctx, reader, cmpd)
	if err != nil {
		return nil, err
	}
	tpls, err := resolveComponentTemplate(reqCtx.Ctx, reader, cmpd)
	if err != nil {
		return nil, err
	}
	initParameters, err := resolveInitParameters(reqCtx, reader, comp)
	if err != nil {
		return nil, err
	}
	parameterSpecs := configctrl.ClassifyParamsFromConfigTemplate(initParameters, cmpd, paramsDefs, tpls)
	if err = handleCustomParameterTemplate(reqCtx.Ctx, reader, comp.Spec.Annotations, parameterSpecs); err != nil {
		return nil, err
	}

	clusterName, _ := component.GetClusterName(comp)
	componentName, _ := component.ShortName(clusterName, comp.Name)
	parameterObj := builder.NewComponentParameterBuilder(comp.Namespace,
		configcore.GenerateComponentConfigurationName(clusterName, componentName)).
		AddLabelsInMap(constant.GetCompLabelsWithDef(clusterName, componentName, cmpd.Name)).
		ClusterRef(clusterName).
		Component(componentName).
		SetConfigurationItem(parameterSpecs).
		GetObject()
	if err = intctrlutil.SetOwnerReference(comp, parameterObj); err != nil {
		return nil, err
	}
	sharding, err := configctrl.ResolveShardingReference(reqCtx.Ctx, reader, comp)
	if err != nil {
		return nil, err
	}
	if configRender != nil {
		err = configctrl.UpdateConfigPayload(&parameterObj.Spec, &comp.Spec, &configRender.Spec, sharding)
	}
	return parameterObj, err
}

func handleCustomParameterTemplate(ctx context.Context, reader client.Reader, annotations map[string]string, specs []parametersv1alpha1.ConfigTemplateItemDetail) error {
	if len(annotations) == 0 {
		return nil
	}
	customParamsTpl := annotations[constant.CustomParameterTemplateAnnotationKey]
	if customParamsTpl == "" {
		return nil
	}

	var customTemplates map[string]parametersv1alpha1.ConfigTemplateExtension
	if err := json.Unmarshal([]byte(customParamsTpl), &customTemplates); err != nil {
		return errors.Wrap(err, "failed to unmarshal custom parameter template")
	}
	if err := validateCustomTemplate(ctx, reader, customTemplates); err != nil {
		return errors.Wrap(err, "failed to validate custom parameter template")
	}

	for tplName, tpl := range customTemplates {
		match := func(spec parametersv1alpha1.ConfigTemplateItemDetail) bool {
			return spec.Name == tplName
		}
		index := generics.FindFirstFunc(specs, match)
		if index < 0 {
			return fmt.Errorf("custom template[%s] not found in component definition", tplName)
		}
		specs[index].CustomTemplates = tpl.DeepCopy()
	}
	return nil
}

func resolveInitParameters(reqCtx intctrlutil.RequestCtx, reader client.Reader, comp *appsv1.Component) (parametersv1alpha1.ComponentParameters, error) {
	resolveShardingName := func(comp *appsv1.Component) string {
		if len(comp.Labels) == 0 {
			return ""
		}
		return comp.Labels[constant.KBAppShardingNameLabelKey]
	}
	componentMatcher := func(clusterName, compName string) func(parametersv1alpha1.Parameter) bool {
		return func(pcr parametersv1alpha1.Parameter) bool {
			if model.IsObjectDeleting(&pcr) {
				return false
			}
			if pcr.Spec.ClusterName != clusterName {
				return false
			}
			for _, parameter := range pcr.Spec.ComponentParameters {
				if parameter.ComponentName == compName {
					return true
				}
			}
			return false
		}
	}

	parameters := &parametersv1alpha1.ParameterList{}
	clusterName, _ := component.GetClusterName(comp)
	listOptions := []client.ListOption{
		client.MatchingLabels{
			constant.AppInstanceLabelKey:    clusterName,
			constant.ParametersInitLabelKey: "true",
		},
		client.InNamespace(comp.Namespace),
	}
	if err := reader.List(reqCtx.Ctx, parameters, listOptions...); err != nil {
		return nil, errors.Wrapf(err, "failed to list init parameters: %v", client.ObjectKeyFromObject(comp))
	}

	compRealName, _ := component.ShortName(clusterName, comp.Name)
	if shardingName := resolveShardingName(comp); shardingName != "" {
		compRealName = shardingName
	}

	parameterCRs := generics.FindFunc(parameters.Items, componentMatcher(clusterName, compRealName))
	if len(parameterCRs) == 0 {
		return nil, nil
	}
	return resolveInitParametersFromParameterCR(parameterCRs, compRealName)
}

func resolveInitParametersFromParameterCR(parameterCRs []parametersv1alpha1.Parameter, compName string) (parametersv1alpha1.ComponentParameters, error) {
	var initParams parametersv1alpha1.ComponentParameters

	slices.SortStableFunc(parameterCRs, func(a, b parametersv1alpha1.Parameter) int {
		return a.CreationTimestamp.Compare(b.CreationTimestamp.Time)
	})
	for _, pcr := range parameterCRs {
		for _, compParams := range pcr.Spec.ComponentParameters {
			if compParams.ComponentName == compName {
				if err := mergeWithOverride(&initParams, compParams.Parameters); err != nil {
					return nil, err
				}
			}
		}
	}
	return initParams, nil
}

func resolveComponentTemplate(ctx context.Context, reader client.Reader, cmpd *appsv1.ComponentDefinition) (map[string]*corev1.ConfigMap, error) {
	tpls := make(map[string]*corev1.ConfigMap, len(cmpd.Spec.Configs))
	for _, config := range cmpd.Spec.Configs {
		cm := &corev1.ConfigMap{}
		if err := reader.Get(ctx, client.ObjectKey{Name: config.TemplateRef, Namespace: config.Namespace}, cm); err != nil {
			return nil, err
		}
		tpls[config.Name] = cm
	}
	return tpls, nil
}

func (r *ComponentDrivenParameterReconciler) mergeComponentParameter(expected *parametersv1alpha1.ComponentParameter, existing *parametersv1alpha1.ComponentParameter) *parametersv1alpha1.ComponentParameter {
	return configctrl.MergeComponentParameter(expected, existing, func(dest, expected *parametersv1alpha1.ConfigTemplateItemDetail) {
		if len(dest.ConfigFileParams) == 0 && len(expected.ConfigFileParams) != 0 {
			dest.ConfigFileParams = expected.ConfigFileParams
		}
		if expected.CustomTemplates != nil {
			dest.CustomTemplates = expected.CustomTemplates
		}
		dest.Payload = expected.Payload
	})
}
