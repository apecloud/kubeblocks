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
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	reconfigurectrl "github.com/apecloud/kubeblocks/controllers/parameters/reconfigure"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	parameterscore "github.com/apecloud/kubeblocks/pkg/parameters/core"
)

// ComponentDrivenParameterReconciler reconciles a Component object into the corresponding ComponentParameter.
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
			WithName("ComponentDrivenParameterReconciler").
			WithValues("Namespace", req.Namespace, "Component", req.Name),
	}

	comp := &appsv1.Component{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, comp); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if !intctrlutil.ObjectAPIVersionSupported(comp) {
		return intctrlutil.Reconciled()
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

	if existingObject, err = r.runningComponentParameter(reqCtx, r.Client, component); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if model.IsObjectDeleting(component) {
		if err = r.syncLegacyConfigManagerRequirement(reqCtx, component, false); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, errors.Wrap(err, "failed to clear legacy config-manager compatibility annotation").Error())
		}
		return r.delete(reqCtx, existingObject)
	}
	required, err := resolveLegacyConfigManagerRequirement(reqCtx.Ctx, r.Client, component)
	if err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}
	if err = r.syncLegacyConfigManagerRequirement(reqCtx, component, required); err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, errors.Wrap(err, "failed to sync legacy config-manager compatibility annotation").Error())
	}
	includeInitOverlay := existingObject == nil
	if expectedObject, err = r.buildComponentParameter(reqCtx, r.Client, component, includeInitOverlay); err != nil {
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
	// By design, init values are seeded only when the ComponentParameter is first created.
	// Subsequent Component-driven updates only synchronize metadata and preserve the existing init payload.
	mergedObject := r.mergeComponentParameter(expected, existing)
	if reflect.DeepEqual(mergedObject, existing) {
		return intctrlutil.Reconciled()
	}
	if err := r.Client.Patch(reqCtx.Ctx, mergedObject, client.MergeFrom(existing)); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *ComponentDrivenParameterReconciler) mergeComponentParameter(expected *parametersv1alpha1.ComponentParameter, existing *parametersv1alpha1.ComponentParameter) *parametersv1alpha1.ComponentParameter {
	updated := existing.DeepCopy()
	updated.SetLabels(intctrlutil.MergeMetadataMaps(expected.GetLabels(), updated.GetLabels()))
	if len(expected.GetOwnerReferences()) != 0 {
		updated.SetOwnerReferences(expected.GetOwnerReferences())
	}
	return updated
}

func (r *ComponentDrivenParameterReconciler) syncLegacyConfigManagerRequirement(reqCtx intctrlutil.RequestCtx, comp *appsv1.Component, required bool) error {
	clusterName, _ := component.GetClusterName(comp)
	clusterKey := types.NamespacedName{Namespace: comp.Namespace, Name: clusterName}
	cluster := &appsv1.Cluster{}
	if err := r.Client.Get(reqCtx.Ctx, clusterKey, cluster); err != nil {
		return client.IgnoreNotFound(err)
	}
	aggregated, err := r.resolveClusterLegacyConfigManagerRequirement(reqCtx.Ctx, cluster, comp, required)
	if err != nil {
		return err
	}
	desiredValue := strconv.FormatBool(aggregated)
	currentValue := ""
	if cluster.Annotations != nil {
		currentValue = cluster.Annotations[constant.LegacyConfigManagerRequiredAnnotationKey]
	}
	if currentValue == desiredValue {
		return nil
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	// Keep an explicit "false" marker instead of deleting the key. The compatible
	// config-manager cleanup flow treats a missing key as "unknown, keep legacy
	// resources" during controller upgrade races, while "false" means cleanup is
	// now safe when the workload naturally recreates Pods.
	cluster.Annotations[constant.LegacyConfigManagerRequiredAnnotationKey] = desiredValue
	return r.Client.Patch(reqCtx.Ctx, cluster, patch)
}

func (r *ComponentDrivenParameterReconciler) resolveClusterLegacyConfigManagerRequirement(ctx context.Context, cluster *appsv1.Cluster, currentComp *appsv1.Component, currentRequired bool) (bool, error) {
	if cluster == nil {
		return false, nil
	}
	compList := &appsv1.ComponentList{}
	if err := r.Client.List(ctx, compList, client.InNamespace(cluster.Namespace), client.MatchingLabels{constant.AppInstanceLabelKey: cluster.Name}); err != nil {
		return false, err
	}
	for i := range compList.Items {
		comp := &compList.Items[i]
		if currentComp != nil && comp.Name == currentComp.Name {
			if currentRequired && !model.IsObjectDeleting(comp) {
				return true, nil
			}
			continue
		}
		if model.IsObjectDeleting(comp) {
			continue
		}
		required, err := resolveLegacyConfigManagerRequirement(ctx, r.Client, comp)
		if err != nil {
			return false, err
		}
		if required {
			return true, nil
		}
	}
	return false, nil
}

func (r *ComponentDrivenParameterReconciler) runningComponentParameter(reqCtx intctrlutil.RequestCtx, reader client.Reader, comp *appsv1.Component) (*parametersv1alpha1.ComponentParameter, error) {
	var componentParameter = &parametersv1alpha1.ComponentParameter{}

	clusterName, _ := component.GetClusterName(comp)
	componentName, _ := component.ShortName(clusterName, comp.Name)

	parameterKey := types.NamespacedName{
		Name:      parameterscore.GenerateComponentConfigurationName(clusterName, componentName),
		Namespace: comp.Namespace,
	}
	if err := reader.Get(reqCtx.Ctx, parameterKey, componentParameter); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return componentParameter, nil
}

func (r *ComponentDrivenParameterReconciler) buildComponentParameter(reqCtx intctrlutil.RequestCtx, reader client.Reader, comp *appsv1.Component, includeInitOverlay bool) (*parametersv1alpha1.ComponentParameter, error) {
	var err error
	var cmpd *appsv1.ComponentDefinition

	if cmpd, err = getCompDefinition(reqCtx.Ctx, reader, comp.Spec.CompDef); err != nil {
		return nil, err
	}
	if len(cmpd.Spec.Configs) == 0 {
		return nil, nil
	}

	configDescs, _, err := parameters.ResolveCmpdParametersDefs(reqCtx.Ctx, reader, cmpd)
	if err != nil {
		return nil, err
	}
	if !parameters.HasValidParameterTemplate(configDescs) {
		return nil, nil
	}

	var initValues *parametersv1alpha1.ParameterValues
	if includeInitOverlay {
		init, err := resolveInitParameters(reqCtx, reader, comp)
		if err != nil {
			return nil, err
		}
		if err = validateCustomTemplate(reqCtx.Ctx, reader, init.Templates); err != nil {
			return nil, err
		}
		if len(init.Parameters) != 0 || len(init.Templates) != 0 {
			initValues = init.DeepCopy()
		}
	}

	clusterName, _ := component.GetClusterName(comp)
	compName, _ := component.ShortName(clusterName, comp.Name)
	parameterObj := builder.NewComponentParameterBuilder(comp.Namespace,
		parameterscore.GenerateComponentConfigurationName(clusterName, compName)).
		AddLabelsInMap(constant.GetCompLabelsWithDef(clusterName, compName, cmpd.Name)).
		SetClusterName(clusterName).
		SetCompName(compName).
		SetInit(initValues).
		GetObject()
	if err = intctrlutil.SetOwnerReference(comp, parameterObj); err != nil {
		return nil, err
	}
	return parameterObj, nil
}

func resolveInitParameters(reqCtx intctrlutil.RequestCtx, reader client.Reader, comp *appsv1.Component) (*parametersv1alpha1.ParameterValues, error) {
	if comp == nil {
		return &parametersv1alpha1.ParameterValues{}, nil
	}
	clusterName, err := component.GetClusterName(comp)
	if err != nil {
		return nil, err
	}
	compName, err := component.ShortName(clusterName, comp.Name)
	if err != nil {
		return nil, err
	}
	cluster := &appsv1.Cluster{}
	if err := reader.Get(reqCtx.Ctx, types.NamespacedName{Namespace: comp.Namespace, Name: clusterName}, cluster); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}
		return &parametersv1alpha1.ParameterValues{}, nil
	}
	initParams, err := parametersv1alpha1.ParseInitParameters(cluster)
	if err != nil {
		return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal,
			"invalid cluster initialization payload: %v", err)
	}
	spec := initParams.Get(compName)
	if spec == nil {
		return &parametersv1alpha1.ParameterValues{}, nil
	}
	return spec, nil
}

// resolveLegacyConfigManagerRequirement reports whether this component still depends on the
// legacy config-manager compatibility path. The requirement exists only when the parameters
// definition still uses legacy actions and the running workload still carries the injected
// config-manager sidecar from older releases.
func resolveLegacyConfigManagerRequirement(ctx context.Context, reader client.Reader, comp *appsv1.Component) (bool, error) {
	cmpd, err := getCompDefinition(ctx, reader, comp.Spec.CompDef)
	if err != nil {
		return false, err
	}
	_, paramsDefs, err := parameters.ResolveCmpdParametersDefs(ctx, reader, cmpd)
	if err != nil {
		return false, err
	}
	if !parameters.LegacyConfigManagerRequiredForParamsDefs(paramsDefs) {
		return false, nil
	}
	its, err := resolveLegacyConfigManagerWorkload(ctx, reader, comp)
	if client.IgnoreNotFound(err) != nil {
		return false, err
	}
	return reconfigurectrl.HasLegacyConfigManagerRuntime(its), nil
}

func resolveLegacyConfigManagerWorkload(ctx context.Context, reader client.Reader, comp *appsv1.Component) (*workloads.InstanceSet, error) {
	clusterName, err := component.GetClusterName(comp)
	if err != nil {
		return nil, err
	}
	componentName, err := component.ShortName(clusterName, comp.Name)
	if err != nil {
		return nil, err
	}
	its := &workloads.InstanceSet{}
	key := types.NamespacedName{
		Namespace: comp.Namespace,
		Name:      constant.GenerateWorkloadNamePattern(clusterName, componentName),
	}
	if err := reader.Get(ctx, key, its); err != nil {
		return nil, err
	}
	return its, nil
}

func getCompDefinition(ctx context.Context, cli client.Reader, cmpdName string) (*appsv1.ComponentDefinition, error) {
	compKey := types.NamespacedName{
		Name: cmpdName,
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

func validateCustomTemplate(ctx context.Context, cli client.Reader, templates map[string]parametersv1alpha1.ConfigTemplateExtension) error {
	for configSpec, custom := range templates {
		cm := &corev1.ConfigMap{}
		err := cli.Get(ctx, types.NamespacedName{Name: custom.TemplateRef, Namespace: custom.Namespace}, cm)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "not found configmap[%s/%s] for custom template: %s",
					custom.Namespace, custom.TemplateRef, configSpec)
			}
			return err
		}
	}
	return nil
}

func resolveComponentTemplate(ctx context.Context, reader client.Reader, cmpd *appsv1.ComponentDefinition) (map[string]*corev1.ConfigMap, error) {
	tpls := make(map[string]*corev1.ConfigMap, len(cmpd.Spec.Configs))
	for _, config := range cmpd.Spec.Configs {
		cm := &corev1.ConfigMap{}
		if err := reader.Get(ctx, client.ObjectKey{Name: config.Template, Namespace: config.Namespace}, cm); err != nil {
			return nil, err
		}
		tpls[config.Name] = cm
	}
	return tpls, nil
}
