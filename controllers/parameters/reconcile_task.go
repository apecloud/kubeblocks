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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
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
	configRender       *parametersv1alpha1.ParameterDrivenConfigRender
	ctx                context.Context
	component          *component.SynthesizedComponent
	paramsDefs         []*parametersv1alpha1.ParametersDefinition
}

func NewTaskContext(ctx context.Context, cli client.Client, componentParameter *parametersv1alpha1.ComponentParameter, fetchTask *Task) (*TaskContext, error) {
	// build synthesized component for the component
	cmpd := fetchTask.ComponentDefObj
	synthesizedComp, err := component.BuildSynthesizedComponent(ctx, cli, cmpd, fetchTask.ComponentObj, fetchTask.ClusterObj)
	if err == nil {
		err = buildTemplateVars(ctx, cli, fetchTask.ComponentDefObj, synthesizedComp)
	}
	if err != nil {
		return nil, err
	}

	configDefList := &parametersv1alpha1.ParameterDrivenConfigRenderList{}
	if err := cli.List(ctx, configDefList); err != nil {
		return nil, err
	}

	var paramsDefs []*parametersv1alpha1.ParametersDefinition
	var configRender *parametersv1alpha1.ParameterDrivenConfigRender
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
				return err
			}
			// Do reconcile for config template
			configMap := resource.ConfigMapObj
			switch intctrlutil.GetConfigSpecReconcilePhase(configMap, item, status) {
			default:
				return syncStatus(configMap, status)
			case appsv1alpha1.CPendingPhase,
				appsv1alpha1.CMergeFailedPhase:
				return syncImpl(taskCtx, resource, item, status, revision, configMap)
			case appsv1alpha1.CCreatingPhase:
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
	if intctrlutil.IsApplyConfigChanged(configMap, item) {
		return nil
	}

	reconcileCtx := &configctrl.ReconcileCtx{
		ResourceCtx:          fetcher.ResourceCtx,
		Cluster:              fetcher.ClusterObj,
		Component:            fetcher.ComponentObj,
		SynthesizedComponent: taskCtx.component,
		PodSpec:              taskCtx.component.PodSpec,
	}

	failStatus := func(err error) error {
		status.Message = pointer.String(err.Error())
		status.Phase = parametersv1alpha1.CMergeFailedPhase
		return err
	}

	var cm *corev1.ConfigMap
	if intctrlutil.IsRerender(configMap, item) {
		if cm, err = configctrl.RerenderParametersTemplate(reconcileCtx, *item.ConfigSpec, taskCtx.configRender, taskCtx.paramsDefs); err != nil {
			return failStatus(err)
		}
	} else if len(item.ConfigFileParams) != 0 {
		if cm, err = configctrl.ApplyParameters(item, configMap, taskCtx.configRender, taskCtx.paramsDefs, revision); err != nil {
			return failStatus(err)
		}
	}

	if err = mergeAndUpdate(fetcher.ResourceCtx, cm, configMap, fetcher.ComponentParameterObj); err != nil {
		return failStatus(err)
	}

	status.Message = nil
	status.Phase = parametersv1alpha1.CMergedPhase
	status.UpdateRevision = revision
	return nil
}

func mergeAndUpdate(resourceCtx *configctrl.ResourceCtx, expected *corev1.ConfigMap, running *corev1.ConfigMap, owner client.Object) error {
	if expected == nil {
		return nil
	}

	configmapDeep := running.DeepCopy()
	configmapDeep.Data = expected.Data
	configmapDeep.Labels = intctrlutil.MergeMetadataMaps(expected.Labels, running.Labels)
	configmapDeep.Annotations = intctrlutil.MergeMetadataMaps(expected.Annotations, running.Annotations)
	if err := intctrlutil.SetControllerReference(owner, configmapDeep); err != nil {
		return err
	}
	return resourceCtx.Client.Patch(resourceCtx, expected, client.MergeFrom(running))
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
	err := fetcherTask.Init(&configctrl.ResourceCtx{
		Context:       reqCtx.Ctx,
		Client:        cli,
		Namespace:     componentParameter.Namespace,
		ClusterName:   componentParameter.Spec.ClusterName,
		ComponentName: componentParameter.Spec.ComponentName,
	}, fetcherTask).Cluster().
		ComponentAndComponentDef().
		ComponentSpec().
		Complete()
	return fetcherTask, err
}
