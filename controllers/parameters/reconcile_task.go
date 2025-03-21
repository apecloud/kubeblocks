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
	"reflect"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type Task struct {
	configctrl.ResourceFetcher[Task]

	Status *parametersv1alpha1.ConfigTemplateItemDetailStatus
	Name   string

	Do func(resource *Task, taskCtx *TaskContext, revision string) error
}

type TaskContext struct {
	componentParameter *parametersv1alpha1.ComponentParameter
	configRender       *parametersv1alpha1.ParamConfigRenderer
	ctx                context.Context
	component          *component.SynthesizedComponent
	paramsDefs         []*parametersv1alpha1.ParametersDefinition
}

func NewTaskContext(ctx context.Context, cli client.Client, componentParameter *parametersv1alpha1.ComponentParameter, fetchTask *Task) (*TaskContext, error) {
	// build synthesized component for the component
	cmpd := fetchTask.ComponentDefObj
	synthesizedComp, err := component.BuildSynthesizedComponent(ctx, cli, cmpd, fetchTask.ComponentObj)
	if err == nil {
		err = buildTemplateVars(ctx, cli, fetchTask.ComponentDefObj, synthesizedComp)
	}
	if err != nil {
		return nil, err
	}

	configDefList := &parametersv1alpha1.ParamConfigRendererList{}
	if err := cli.List(ctx, configDefList); err != nil {
		return nil, err
	}

	var paramsDefs []*parametersv1alpha1.ParametersDefinition
	var configRender *parametersv1alpha1.ParamConfigRenderer
	for i, item := range configDefList.Items {
		if item.Spec.ComponentDef != cmpd.Name {
			continue
		}
		if item.Spec.ServiceVersion == "" || item.Spec.ServiceVersion == cmpd.Spec.ServiceVersion {
			configRender = &configDefList.Items[i]
			break
		}
	}

	if configRender != nil {
		for _, paramsDef := range configRender.Spec.ParametersDefs {
			var param = &parametersv1alpha1.ParametersDefinition{}
			if err := cli.Get(ctx, client.ObjectKey{Name: paramsDef}, param); err != nil {
				return nil, err
			}
			paramsDefs = append(paramsDefs, param)
		}
	}

	return &TaskContext{ctx: ctx,
		componentParameter: componentParameter,
		configRender:       configRender,
		component:          synthesizedComp,
		paramsDefs:         paramsDefs,
	}, nil
}

func NewTask(item parametersv1alpha1.ConfigTemplateItemDetail, status *parametersv1alpha1.ConfigTemplateItemDetailStatus) Task {
	return Task{
		Name: item.Name,
		Do: func(resource *Task, taskCtx *TaskContext, revision string) error {
			configSpec := item.ConfigSpec
			if configSpec == nil {
				return core.MakeError("not found config spec: %s", item.Name)
			}
			if err := resource.ConfigMap(item.Name).Complete(); err != nil {
				return syncImpl(taskCtx, resource, item, status, revision, nil)
			}
			// Do reconcile for config template
			configMap := resource.ConfigMapObj
			switch intctrlutil.GetUpdatedParametersReconciledPhase(configMap, item, status) {
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

func syncImpl(taskCtx *TaskContext,
	fetcher *Task,
	item parametersv1alpha1.ConfigTemplateItemDetail,
	status *parametersv1alpha1.ConfigTemplateItemDetailStatus,
	revision string,
	configMap *corev1.ConfigMap) (err error) {
	if intctrlutil.IsApplyUpdatedParameters(configMap, item) {
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

	var baseConfig = configMap
	var updatedConfig *corev1.ConfigMap
	if intctrlutil.IsRerender(configMap, item) {
		log.FromContext(taskCtx.ctx).
			WithName("ParameterReconcileTask").
			WithValues("cluster", taskCtx.component.ClusterName,
				"component", taskCtx.component.Name,
				"parameterTpl", item.Name).
			Info("rerender parameter template",
				"appliedConfigMeta", resolveLastConfigMeta(configMap),
				"revision", revision,
				"configMeta", item,
			)
		if baseConfig, err = configctrl.RerenderParametersTemplate(reconcileCtx, item, taskCtx.configRender, taskCtx.paramsDefs); err != nil {
			return failStatus(err)
		}
		updatedConfig = baseConfig
	}
	if len(item.ConfigFileParams) != 0 {
		if updatedConfig, err = configctrl.ApplyParameters(item, baseConfig, taskCtx.configRender, taskCtx.paramsDefs); err != nil {
			return failStatus(err)
		}
	}
	if err = mergeAndApplyConfig(fetcher.ResourceCtx, updatedConfig, configMap, fetcher.ComponentParameterObj, item, revision); err != nil {
		return failStatus(err)
	}

	status.Message = nil
	status.Phase = parametersv1alpha1.CMergedPhase
	status.UpdateRevision = revision
	return nil
}

func resolveLastConfigMeta(configMap *corev1.ConfigMap) any {
	if configMap == nil || len(configMap.Annotations) == 0 {
		return nil
	}
	return map[string]string{
		"revision":   configMap.Annotations[constant.ConfigurationRevision],
		"configMeta": configMap.Annotations[constant.ConfigAppliedVersionAnnotationKey],
	}
}

func mergeAndApplyConfig(resourceCtx *render.ResourceCtx,
	expected *corev1.ConfigMap,
	running *corev1.ConfigMap,
	owner client.Object,
	item parametersv1alpha1.ConfigTemplateItemDetail,
	revision string) error {
	switch {
	case expected == nil: // not update
		return update(resourceCtx.Context, resourceCtx.Client, running, running, updateReconcileObject(item, owner, revision))
	case running == nil: // cm been deleted
		return create(resourceCtx.Context, resourceCtx.Client, expected, updateReconcileObject(item, owner, revision))
	default:
		return update(resourceCtx.Context, resourceCtx.Client, running, running, mergedConfigmap(expected, updateReconcileObject(item, owner, revision)))
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
	owner client.Object,
	revision string) func(*corev1.ConfigMap) error {
	return func(cmObj *corev1.ConfigMap) error {
		if !controllerutil.ContainsFinalizer(cmObj, constant.ConfigFinalizerName) {
			controllerutil.AddFinalizer(cmObj, constant.ConfigFinalizerName)
		}
		if !model.IsOwnerOf(owner, cmObj) {
			if err := intctrlutil.SetControllerReference(owner, cmObj); err != nil {
				return err
			}
		}
		return updateConfigLabels(cmObj, item, revision)
	}
}

func updateConfigLabels(obj *corev1.ConfigMap,
	item parametersv1alpha1.ConfigTemplateItemDetail,
	revision string) error {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	b, err := json.Marshal(&item)
	if err != nil {
		return err
	}
	obj.Annotations[constant.ConfigAppliedVersionAnnotationKey] = string(b)
	obj.Annotations[constant.ConfigurationRevision] = revision

	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	hash, _ := cfgutil.ComputeHash(obj.Data)
	obj.Labels[constant.CMInsConfigurationHashLabelKey] = hash
	obj.Labels[constant.CMConfigurationSpecProviderLabelKey] = item.Name
	obj.Labels[constant.CMConfigurationTemplateNameLabelKey] = item.ConfigSpec.Template
	return nil
}

func syncStatus(configMap *corev1.ConfigMap, status *parametersv1alpha1.ConfigTemplateItemDetailStatus) (err error) {
	annotations := configMap.GetAnnotations()
	// status.CurrentRevision = GetCurrentRevision(annotations)
	revisions := RetrieveRevision(annotations)
	if len(revisions) == 0 {
		return
	}

	for i := 0; i < len(revisions); i++ {
		updateRevision(revisions[i], status)
		updateLastDoneRevision(revisions[i], status)
	}

	return
}

func updateLastDoneRevision(revision ConfigurationRevision, status *parametersv1alpha1.ConfigTemplateItemDetailStatus) {
	if revision.Phase == parametersv1alpha1.CFinishedPhase {
		status.LastDoneRevision = strconv.FormatInt(revision.Revision, 10)
	}
}

func updateRevision(revision ConfigurationRevision, status *parametersv1alpha1.ConfigTemplateItemDetailStatus) {
	if revision.StrRevision == status.UpdateRevision {
		status.Phase = revision.Phase
		status.ReconcileDetail = &parametersv1alpha1.ReconcileDetail{
			CurrentRevision: revision.StrRevision,
			Policy:          revision.Result.Policy,
			SucceedCount:    revision.Result.SucceedCount,
			ExpectedCount:   revision.Result.ExpectedCount,
			ExecResult:      revision.Result.ExecResult,
			ErrMessage:      revision.Result.Message,
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
