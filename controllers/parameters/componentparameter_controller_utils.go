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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

func reconcileConfigItemDetailsIntoSpec(ctx context.Context, cli client.Client, compParam *parametersv1alpha1.ComponentParameter, fetchTask *Task) (bool, error) {
	configDescs, paramsDefs, err := parameters.ResolveCmpdParametersDefs(ctx, cli, fetchTask.ComponentDefObj)
	if err != nil {
		return false, err
	}
	if !parameters.HasValidParameterTemplate(configDescs) {
		return false, nil
	}
	templates, err := resolveComponentTemplate(ctx, cli, fetchTask.ComponentDefObj)
	if err != nil {
		return false, err
	}
	configItemDetails, err := parameters.ClassifyParamsFromConfigTemplate(nil, fetchTask.ComponentDefObj, paramsDefs, templates, configDescs)
	if err != nil {
		return false, err
	}
	expected := compParam.DeepCopy()
	expected.Spec.ConfigItemDetails = configItemDetails
	merged := parameters.MergeComponentParameter(expected, compParam, func(dest, expected *parametersv1alpha1.ConfigTemplateItemDetail) {
		if len(dest.ConfigFileParams) == 0 && len(expected.ConfigFileParams) != 0 {
			dest.ConfigFileParams = expected.ConfigFileParams
		}
		if dest.CustomTemplates == nil && expected.CustomTemplates != nil {
			dest.CustomTemplates = expected.CustomTemplates
		}
		dest.ConfigSpec = expected.ConfigSpec
	})
	if reflect.DeepEqual(compParam.Spec.ConfigItemDetails, merged.Spec.ConfigItemDetails) {
		return false, nil
	}
	patch := client.MergeFrom(compParam.DeepCopy())
	compParam.Spec.ConfigItemDetails = merged.Spec.ConfigItemDetails
	return true, cli.Patch(ctx, compParam, patch)
}

func reconcileParameterValuesIntoSpec(ctx context.Context, cli client.Client, compParam *parametersv1alpha1.ComponentParameter, fetchTask *Task) (bool, error) {
	specCopy := compParam.Spec.DeepCopy()
	configmaps, err := resolveComponentRefConfigMap(ctx, cli, compParam.Namespace, compParam.Spec.ClusterName, compParam.Spec.ComponentName)
	if err != nil {
		return false, err
	}
	configDescs, paramsDefs, err := parameters.ResolveCmpdParametersDefs(ctx, cli, fetchTask.ComponentDefObj)
	if err != nil {
		return false, err
	}
	if err := applyParameterValues(specCopy, compParam.Spec.Init, false, ctx, cli, fetchTask, configmaps, configDescs, paramsDefs); err != nil {
		return false, err
	}
	if err := applyParameterValues(specCopy, compParam.Spec.Desired, true, ctx, cli, fetchTask, configmaps, configDescs, paramsDefs); err != nil {
		return false, err
	}
	if reflect.DeepEqual(compParam.Spec, *specCopy) {
		return false, nil
	}
	patch := client.MergeFrom(compParam.DeepCopy())
	compParam.Spec = *specCopy
	return true, cli.Patch(ctx, compParam, patch)
}

func applyParameterValues(spec *parametersv1alpha1.ComponentParameterSpec,
	values *parametersv1alpha1.ParameterValues, override bool,
	ctx context.Context, cli client.Client, fetchTask *Task,
	configmaps map[string]*corev1.ConfigMap,
	configDescs []parametersv1alpha1.ComponentConfigDescription,
	paramsDefs []*parametersv1alpha1.ParametersDefinition) error {
	if values == nil {
		return nil
	}
	if err := validateCustomTemplate(ctx, cli, values.Templates); err != nil {
		return err
	}
	if len(values.Parameters) != 0 {
		classifiedParameters, err := parameters.ClassifyComponentParameters(
			parametersv1alpha1.ComponentParameters(values.Parameters),
			paramsDefs,
			fetchTask.ComponentDefObj.Spec.Configs,
			configmaps,
			configDescs,
		)
		if err != nil {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "%s", err.Error())
		}
		for templateName, paramsInFile := range classifiedParameters {
			configDescriptions := parameters.GetComponentConfigDescriptions(configDescs, templateName)
			if len(configDescriptions) == 0 {
				return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "not found config description for template: %s", templateName)
			}
			if _, err := parameters.DoMerge(resolveBaseData(paramsInFile), parameters.DerefMapValues(paramsInFile), paramsDefs, configDescriptions); err != nil {
				return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "%s", err.Error())
			}
			item := parameters.GetConfigTemplateItem(spec, templateName)
			if item == nil {
				return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "not found config template item: %s", templateName)
			}
			mergeItemParameters(item, parameters.DerefMapValues(paramsInFile), override)
		}
	}
	for templateName, templateExtension := range values.Templates {
		item := parameters.GetConfigTemplateItem(spec, templateName)
		if item == nil {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "not found config template item: %s", templateName)
		}
		if override || item.CustomTemplates == nil {
			item.CustomTemplates = templateExtension.DeepCopy()
		}
	}
	return nil
}

func resolveComponentRefConfigMap(ctx context.Context, cli client.Client, namespace, clusterName, componentName string) (map[string]*corev1.ConfigMap, error) {
	configMapList := &corev1.ConfigMapList{}
	if err := cli.List(ctx, configMapList,
		client.InNamespace(namespace),
		client.MatchingLabels(constant.GetCompLabels(clusterName, componentName)),
		client.HasLabels([]string{
			constant.AppInstanceLabelKey,
			constant.KBAppComponentLabelKey,
			constant.CMConfigurationTemplateNameLabelKey,
			constant.CMConfigurationTypeLabelKey,
			constant.CMConfigurationSpecProviderLabelKey,
		}),
	); err != nil {
		return nil, err
	}
	configmaps := make(map[string]*corev1.ConfigMap, len(configMapList.Items))
	for i := range configMapList.Items {
		item := &configMapList.Items[i]
		configmaps[item.Labels[constant.CMConfigurationSpecProviderLabelKey]] = item
	}
	return configmaps, nil
}

func mergeItemParameters(item *parametersv1alpha1.ConfigTemplateItemDetail, updatedParameters map[string]parametersv1alpha1.ParametersInFile, override bool) {
	if item.ConfigFileParams == nil {
		item.ConfigFileParams = updatedParameters
		return
	}
	if !override && len(item.ConfigFileParams) != 0 {
		return
	}
	for key, parametersInFile := range updatedParameters {
		merged := item.ConfigFileParams[key]
		if parametersInFile.Content != nil {
			merged.Content = parametersInFile.Content
		}
		if merged.Parameters == nil && len(parametersInFile.Parameters) > 0 {
			merged.Parameters = map[string]*string{}
		}
		for paramKey, paramValue := range parametersInFile.Parameters {
			merged.Parameters[paramKey] = paramValue
		}
		item.ConfigFileParams[key] = merged
	}
}

func resolveBaseData(updatedParameters map[string]*parametersv1alpha1.ParametersInFile) map[string]string {
	baseData := make(map[string]string, len(updatedParameters))
	for key := range updatedParameters {
		baseData[key] = ""
	}
	return baseData
}

type Task struct {
	parameters.ResourceFetcher[Task]

	Status *parametersv1alpha1.ConfigTemplateItemDetailStatus
	Name   string

	Do func(resource *Task, taskCtx *taskContext, revision string) error
}

type taskContext struct {
	componentParameter *parametersv1alpha1.ComponentParameter
	configDescs        []parametersv1alpha1.ComponentConfigDescription
	ctx                context.Context
	component          *component.SynthesizedComponent
	paramsDefs         []*parametersv1alpha1.ParametersDefinition
}

func newTaskContext(ctx context.Context, cli client.Client, componentParameter *parametersv1alpha1.ComponentParameter, fetchTask *Task) (*taskContext, error) {
	// build synthesized component for the component
	cmpd := fetchTask.ComponentDefObj
	synthesizedComp, err := component.BuildSynthesizedComponent(ctx, cli, cmpd, fetchTask.ComponentObj)
	if err == nil {
		err = buildTemplateVars(ctx, cli, fetchTask.ComponentDefObj, synthesizedComp)
	}
	if err != nil {
		return nil, err
	}

	configDescs, paramsDefs, err := parameters.ResolveCmpdParametersDefs(ctx, cli, cmpd)
	if err != nil {
		return nil, err
	}

	return &taskContext{ctx: ctx,
		componentParameter: componentParameter,
		configDescs:        configDescs,
		component:          synthesizedComp,
		paramsDefs:         paramsDefs,
	}, nil
}

func buildTemplateVars(ctx context.Context, cli client.Reader,
	compDef *appsv1.ComponentDefinition, synthesizedComp *component.SynthesizedComponent) error {
	if compDef != nil && len(compDef.Spec.Vars) > 0 {
		templateVars, _, err := component.ResolveTemplateNEnvVars(ctx, cli, synthesizedComp, compDef.Spec.Vars)
		if err != nil {
			return err
		}
		synthesizedComp.TemplateVars = templateVars
	}
	return nil
}

func generateReconcileTasks(reqCtx intctrlutil.RequestCtx,
	componentParameter *parametersv1alpha1.ComponentParameter, compGeneration int64) []Task {
	tasks := make([]Task, 0, len(componentParameter.Spec.ConfigItemDetails))
	for _, item := range componentParameter.Spec.ConfigItemDetails {
		if status := fromItemStatus(reqCtx, &componentParameter.Status, item, componentParameter.GetGeneration()); status != nil {
			tasks = append(tasks, newTask(item, status, compGeneration))
		}
	}
	return tasks
}

func fromItemStatus(ctx intctrlutil.RequestCtx, status *parametersv1alpha1.ComponentParameterStatus, item parametersv1alpha1.ConfigTemplateItemDetail, generation int64) *parametersv1alpha1.ConfigTemplateItemDetailStatus {
	if item.ConfigSpec == nil {
		ctx.Log.V(1).WithName(item.Name).Info(fmt.Sprintf("configuration is creating and pass: %s", item.Name))
		return nil
	}
	itemStatus := parameters.GetItemStatus(status, item.Name)
	if itemStatus == nil || itemStatus.Phase == "" {
		ctx.Log.WithName(item.Name).Info(fmt.Sprintf("ComponentParameters cr is creating: %v", item))
		status.ConfigurationItemStatus = append(status.ConfigurationItemStatus, parametersv1alpha1.ConfigTemplateItemDetailStatus{
			Name:           item.Name,
			Phase:          parametersv1alpha1.CInitPhase,
			UpdateRevision: strconv.FormatInt(generation, 10),
		})
		itemStatus = parameters.GetItemStatus(status, item.Name)
	}
	if !isReconcileStatus(itemStatus.Phase) {
		ctx.Log.V(1).WithName(item.Name).Info(fmt.Sprintf("configuration cr is creating or deleting and pass: %v", itemStatus))
		return nil
	}
	return itemStatus
}

func isReconcileStatus(phase parametersv1alpha1.ParameterPhase) bool {
	return phase != "" &&
		phase != parametersv1alpha1.CCreatingPhase &&
		phase != parametersv1alpha1.CDeletingPhase
}

func newTask(item parametersv1alpha1.ConfigTemplateItemDetail,
	status *parametersv1alpha1.ConfigTemplateItemDetailStatus, compGeneration int64) Task {
	return Task{
		Name: item.Name,
		Do: func(resource *Task, taskCtx *taskContext, revision string) error {
			if item.ConfigSpec == nil {
				return core.MakeError("not found config spec: %s", item.Name)
			}
			if err := resource.ConfigMap(item.Name).Complete(); err != nil {
				if apierrors.IsNotFound(err) {
					return syncImpl(taskCtx, resource, item, status, revision, nil)
				}
				return err
			}
			// Do reconcile for config template
			configMap := resource.ConfigMapObj
			switch parameters.GetUpdatedParametersReconciledPhase(configMap, item, status, compGeneration) {
			default:
				return syncStatus(configMap, status)
			case parametersv1alpha1.CInitPhase,
				parametersv1alpha1.CPendingPhase,
				parametersv1alpha1.CMergeFailedPhase:
				return syncImpl(taskCtx, resource, item, status, revision, configMap)
			case parametersv1alpha1.CCreatingPhase:
				return nil
			}
		},
		Status: status,
	}
}

func syncImpl(taskCtx *taskContext,
	fetcher *Task,
	item parametersv1alpha1.ConfigTemplateItemDetail,
	status *parametersv1alpha1.ConfigTemplateItemDetailStatus,
	revision string,
	configMap *corev1.ConfigMap) (err error) {
	if parameters.IsApplyUpdatedParameters(configMap, item, fetcher.ComponentObj.Generation) {
		return syncStatus(configMap, status)
	}

	failStatus := func(err error) error {
		status.Message = pointer.String(err.Error())
		status.Phase = parametersv1alpha1.CMergeFailedPhase
		return err
	}

	reconcileCtx := &render.ReconcileCtx{
		ResourceCtx:          fetcher.ResourceCtx,
		Cluster:              fetcher.ClusterObj,
		Component:            fetcher.ComponentObj,
		SynthesizedComponent: taskCtx.component,
		PodSpec:              taskCtx.component.PodSpec,
	}

	var baseConfig *corev1.ConfigMap
	var updatedConfig *corev1.ConfigMap
	if baseConfig, err = parameters.RerenderParametersTemplate(reconcileCtx, item, taskCtx.configDescs, taskCtx.paramsDefs); err != nil {
		return failStatus(err)
	}
	updatedConfig = baseConfig
	if len(item.ConfigFileParams) != 0 {
		if updatedConfig, err = parameters.ApplyParameters(item, baseConfig, taskCtx.configDescs, taskCtx.paramsDefs); err != nil {
			return failStatus(err)
		}
	}
	if err = mergeAndApplyConfig(fetcher.ResourceCtx, updatedConfig, configMap, fetcher.ComponentParameterObj, item, fetcher.ComponentObj.Generation, revision); err != nil {
		return failStatus(err)
	}

	status.Message = nil
	status.Phase = parametersv1alpha1.CMergedPhase
	status.UpdateRevision = revision
	return nil
}

func mergeAndApplyConfig(resourceCtx *render.ResourceCtx, expected, running *corev1.ConfigMap, owner client.Object,
	item parametersv1alpha1.ConfigTemplateItemDetail, compGeneration int64, revision string) error {
	fn := updateReconcileObject(item, owner, compGeneration, revision)
	switch {
	case expected == nil: // not update
		return update(resourceCtx.Context, resourceCtx.Client, running, running, fn)
	case running == nil: // cm been deleted
		return create(resourceCtx.Context, resourceCtx.Client, expected, fn)
	default:
		return update(resourceCtx.Context, resourceCtx.Client, running, running, mergedConfigmap(expected, fn))
	}
}

func mergedConfigmap(expected *corev1.ConfigMap, setter func(*corev1.ConfigMap) error) func(*corev1.ConfigMap) error {
	return func(cmObj *corev1.ConfigMap) error {
		cmObj.Data = expected.Data
		cmObj.Labels = intctrlutil.MergeMetadataMaps(expected.Labels, cmObj.Labels)
		cmObj.Annotations = intctrlutil.MergeMetadataMaps(expected.Annotations, cmObj.Annotations)
		return setter(cmObj)
	}
}

func update(ctx context.Context, cli client.Client, expected, origin *corev1.ConfigMap, setter func(*corev1.ConfigMap) error) error {
	objectDeep := expected.DeepCopy()
	if err := setter(objectDeep); err != nil {
		return err
	}
	if reflect.DeepEqual(objectDeep.Data, origin.Data) &&
		reflect.DeepEqual(objectDeep.Annotations, origin.Annotations) &&
		reflect.DeepEqual(objectDeep.Labels, origin.Labels) &&
		reflect.DeepEqual(objectDeep.Finalizers, origin.Finalizers) &&
		reflect.DeepEqual(objectDeep.OwnerReferences, origin.OwnerReferences) {
		return nil
	}
	return cli.Patch(ctx, objectDeep, client.MergeFrom(origin))
}

func create(ctx context.Context, cli client.Client, expected *corev1.ConfigMap, setter func(*corev1.ConfigMap) error) error {
	if err := setter(expected); err != nil {
		return err
	}
	return cli.Create(ctx, expected)
}

func updateReconcileObject(item parametersv1alpha1.ConfigTemplateItemDetail,
	owner client.Object, compGeneration int64, revision string) func(*corev1.ConfigMap) error {
	return func(cmObj *corev1.ConfigMap) error {
		if !controllerutil.ContainsFinalizer(cmObj, constant.ConfigFinalizerName) {
			controllerutil.AddFinalizer(cmObj, constant.ConfigFinalizerName)
		}
		if !model.IsOwnerOf(owner, cmObj) {
			if err := intctrlutil.SetControllerReference(owner, cmObj); err != nil {
				return err
			}
		}
		return updateConfigLabels(cmObj, item, compGeneration, revision)
	}
}

func updateConfigLabels(obj *corev1.ConfigMap,
	item parametersv1alpha1.ConfigTemplateItemDetail, compGeneration int64, revision string) error {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	b, err := json.Marshal(&item)
	if err != nil {
		return err
	}
	obj.Annotations[constant.ConfigAppliedVersionAnnotationKey] = string(b)
	obj.Annotations[constant.ParametersAppliedComponentGenerationKey] = strconv.FormatInt(compGeneration, 10)
	obj.Annotations[constant.ConfigurationRevision] = revision

	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	hash, _ := intctrlutil.ComputeHash(obj.Data)
	obj.Labels[constant.CMInsConfigurationHashLabelKey] = hash
	obj.Labels[constant.CMConfigurationSpecProviderLabelKey] = item.Name
	obj.Labels[constant.CMConfigurationTemplateNameLabelKey] = item.ConfigSpec.Template
	return nil
}

func syncStatus(configMap *corev1.ConfigMap, status *parametersv1alpha1.ConfigTemplateItemDetailStatus) (err error) {
	annotations := configMap.GetAnnotations()
	// status.CurrentRevision = GetCurrentRevision(annotations)
	revisions := retrieveRevision(annotations)
	if len(revisions) == 0 {
		return
	}

	for i := 0; i < len(revisions); i++ {
		updateRevision(revisions[i], status)
		updateLastDoneRevision(revisions[i], status)
	}

	return
}

func updateLastDoneRevision(revision configurationRevision, status *parametersv1alpha1.ConfigTemplateItemDetailStatus) {
	if revision.phase == parametersv1alpha1.CFinishedPhase {
		status.LastDoneRevision = strconv.FormatInt(revision.revision, 10)
	}
}

func updateRevision(revision configurationRevision, status *parametersv1alpha1.ConfigTemplateItemDetailStatus) {
	if revision.strRevision == status.UpdateRevision {
		status.Phase = revision.phase
		status.ReconcileDetail = &parametersv1alpha1.ReconcileDetail{
			CurrentRevision: revision.strRevision,
			Policy:          revision.result.Policy,
			SucceedCount:    revision.result.SucceedCount,
			ExpectedCount:   revision.result.ExpectedCount,
			ExecResult:      revision.result.ExecResult,
			ErrMessage:      revision.result.Message,
		}
	}
}

func prepareReconcileTask(reqCtx intctrlutil.RequestCtx, cli client.Client, componentParameter *parametersv1alpha1.ComponentParameter) (*Task, error) {
	fetcherTask := &Task{}
	err := fetcherTask.Init(&render.ResourceCtx{
		Context:       reqCtx.Ctx,
		Client:        cli,
		Namespace:     componentParameter.Namespace,
		ClusterName:   componentParameter.Spec.ClusterName,
		ComponentName: componentParameter.Spec.ComponentName,
	}, fetcherTask).Cluster().
		ComponentAndComponentDef().
		ComponentSpec().
		Complete()
	fetcherTask.ComponentParameterObj = componentParameter
	return fetcherTask, err
}
