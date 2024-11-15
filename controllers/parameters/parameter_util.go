/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package parameters

import (
	"fmt"
	"reflect"

	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type reconfigureReconcileHandle func(*ReconcileContext, *parametersv1alpha1.Parameter) error

func updateComponentParameterStatus(configmaps map[string]*corev1.ConfigMap) func(rctx *ReconcileContext, parameter *parametersv1alpha1.Parameter) error {
	return func(rctx *ReconcileContext, parameter *parametersv1alpha1.Parameter) error {
		status := safeResolveComponentStatus(&parameter.Status, rctx.ComponentName)
		if !intctrlutil.IsParameterFinished(status.Phase) {
			syncReconfiguringPhase(rctx, status, configmaps)
		}
		return nil
	}
}

func syncReconfiguringPhase(rctx *ReconcileContext, status *parametersv1alpha1.ComponentReconfiguringStatus, configmaps map[string]*corev1.ConfigMap) {
	var finished = true

	updateStatus := func() {
		if finished {
			status.Phase = parametersv1alpha1.CFinishedPhase
		} else {
			status.Phase = parametersv1alpha1.CRunningPhase
		}
	}

	for _, parameterStatus := range status.ParameterStatus {
		if parameterStatus.Phase == parametersv1alpha1.CMergeFailedPhase {
			status.Phase = parametersv1alpha1.CMergeFailedPhase
			return
		}
		cm := configmaps[parameterStatus.Name]
		compSpec := intctrlutil.GetConfigTemplateItem(&rctx.ComponentParameterObj.Spec, parameterStatus.Name)
		compStatus := intctrlutil.GetItemStatus(&rctx.ComponentParameterObj.Status, parameterStatus.Name)
		if compStatus == nil || compSpec == nil || cm == nil {
			rctx.Log.Info("component status or spec not found", "component", parameterStatus.Name, "template", parameterStatus.Name)
			continue
		}
		parameterStatus.Phase = intctrlutil.GetConfigSpecReconcilePhase(cm, *compSpec, compStatus)
		if finished {
			finished = intctrlutil.IsParameterFinished(parameterStatus.Phase)
		}
		if parameterStatus.Phase == parametersv1alpha1.CFailedAndPausePhase {
			status.Phase = parametersv1alpha1.CFailedAndPausePhase
			return
		}
	}

	updateStatus()
}

func mergeWithOverride(dst, src interface{}) error {
	return mergo.Merge(dst, src, mergo.WithOverride)
}

func updateParameters(rctx *ReconcileContext, parameter *parametersv1alpha1.Parameter) error {
	var updated bool

	compStatus := intctrlutil.GetParameterStatus(&parameter.Status, rctx.ComponentName)
	if compStatus == nil || intctrlutil.IsParameterFinished(compStatus.Phase) {
		return nil
	}

	patch := rctx.ComponentParameterObj.DeepCopy()
	var item *parametersv1alpha1.ConfigTemplateItemDetail
	for _, status := range compStatus.ParameterStatus {
		if item = intctrlutil.GetConfigTemplateItem(&rctx.ComponentParameterObj.Spec, status.Name); item == nil {
			status.Phase = parametersv1alpha1.CMergeFailedPhase
			continue
		}
		if err := mergeWithOverride(&item.ConfigFileParams, status.UpdatedParameters); err != nil {
			status.Phase = parametersv1alpha1.CMergeFailedPhase
			return err
		}
		if status.CustomTemplate != nil {
			item.CustomTemplates = status.CustomTemplate
		}
		updated = true
		status.Phase = parametersv1alpha1.CMergedPhase
	}

	if updated && !reflect.DeepEqual(patch, rctx.ComponentParameterObj) {
		return rctx.Client.Patch(rctx.Ctx, rctx.ComponentParameterObj, client.MergeFrom(patch))
	}
	return nil
}

func updateCustomTemplates(rctx *ReconcileContext, parameter *parametersv1alpha1.Parameter) error {
	component := intctrlutil.GetParameter(&parameter.Spec, rctx.ComponentName)
	if component == nil || len(component.CustomTemplates) == 0 {
		return nil
	}

	for tpl, componentParameter := range component.CustomTemplates {
		status := safeResolveComponentParameterStatus(&parameter.Status, component.ComponentName, tpl)
		status.CustomTemplate = componentParameter.DeepCopy()
	}
	return nil
}

func classifyParameters(updatedParameters appsv1.ComponentParameters, configmaps map[string]*corev1.ConfigMap) func(*ReconcileContext, *parametersv1alpha1.Parameter) error {
	return func(rctx *ReconcileContext, parameter *parametersv1alpha1.Parameter) error {
		classParameters := configctrl.ClassifyComponentParameters(updatedParameters,
			flatten(rctx.ParametersDefs),
			rctx.ComponentDefObj.Spec.Configs,
			configmaps,
		)
		for tpl, m := range classParameters {
			configDescs := intctrlutil.GetComponentConfigDescriptions(&rctx.ConfigRender.Spec, tpl)
			if len(configDescs) == 0 {
				return fmt.Errorf("not found config description from pdcr: %s", tpl)
			}
			if err := validateComponentParameter(toArray(rctx.ParametersDefs), configDescs, m); err != nil {
				return intctrlutil.NewFatalError(err.Error())
			}
			safeUpdateComponentParameterStatus(&parameter.Status, rctx.ComponentName, tpl, m)
		}
		return nil
	}
}

func validateComponentParameter(parametersDefs []*parametersv1alpha1.ParametersDefinition, descs []parametersv1alpha1.ComponentConfigDescription, paramters map[string]*parametersv1alpha1.ParametersInFile) error {
	if len(parametersDefs) == 0 || len(descs) == 0 {
		return nil
	}
	_, err := configctrl.DoMerge(resolveBaseData(paramters), configctrl.DerefMapValues(paramters), parametersDefs, descs)
	return err
}

func resolveBaseData(updatedParameters map[string]*parametersv1alpha1.ParametersInFile) map[string]string {
	baseData := make(map[string]string)
	for key := range updatedParameters {
		baseData[key] = ""
	}
	return baseData
}

func toArray(paramsDefs map[string]*parametersv1alpha1.ParametersDefinition) []*parametersv1alpha1.ParametersDefinition {
	var defs []*parametersv1alpha1.ParametersDefinition
	for _, def := range paramsDefs {
		defs = append(defs, def)
	}
	return defs
}

func safeResolveComponentStatus(status *parametersv1alpha1.ParameterStatus, componentName string) *parametersv1alpha1.ComponentReconfiguringStatus {
	compStatus := intctrlutil.GetParameterStatus(status, componentName)
	if compStatus != nil {
		return compStatus
	}

	status.ReconfiguringStatus = append(status.ReconfiguringStatus,
		parametersv1alpha1.ComponentReconfiguringStatus{
			ComponentName: componentName,
			Phase:         parametersv1alpha1.CInitPhase,
		})
	return intctrlutil.GetParameterStatus(status, componentName)
}

func safeResolveComponentParameterStatus(status *parametersv1alpha1.ParameterStatus, componentName string, tpl string) *parametersv1alpha1.ReconfiguringStatus {
	compStatus := safeResolveComponentStatus(status, componentName)
	parameterStatus := intctrlutil.GetParameterReconfiguringStatus(compStatus, tpl)
	if parameterStatus != nil {
		return parameterStatus
	}

	compStatus.ParameterStatus = append(compStatus.ParameterStatus,
		parametersv1alpha1.ReconfiguringStatus{
			ConfigTemplateItemDetailStatus: parametersv1alpha1.ConfigTemplateItemDetailStatus{
				Name: tpl,
			},
		})
	return intctrlutil.GetParameterReconfiguringStatus(compStatus, tpl)
}

func safeUpdateComponentParameterStatus(status *parametersv1alpha1.ParameterStatus, componentName string, tpl string, updatedParams map[string]*parametersv1alpha1.ParametersInFile) {
	parameterStatus := safeResolveComponentParameterStatus(status, componentName, tpl)
	parameterStatus.UpdatedParameters = configctrl.DerefMapValues(updatedParams)
}

func flatten(parametersDefs map[string]*parametersv1alpha1.ParametersDefinition) []*parametersv1alpha1.ParametersDefinition {
	var defs []*parametersv1alpha1.ParametersDefinition
	for _, paramsDef := range parametersDefs {
		defs = append(defs, paramsDef)
	}
	return defs
}

func resolveComponentRefConfigMap(rctx *ReconcileContext) (map[string]*corev1.ConfigMap, error) {
	configMapList := &corev1.ConfigMapList{}
	matchLabels := client.MatchingLabels(constant.GetCompLabels(rctx.ClusterName, rctx.ComponentName))
	requiredLabels := client.HasLabels(reconfigureRequiredLabels)
	if err := rctx.Client.List(rctx.Ctx, configMapList, client.InNamespace(rctx.Namespace), matchLabels, requiredLabels); err != nil {
		return nil, err
	}

	configs := make(map[string]*corev1.ConfigMap)
	for i, cm := range configMapList.Items {
		configs[cm.Labels[constant.CMConfigurationSpecProviderLabelKey]] = &configMapList.Items[i]
	}
	return configs, nil
}

func prepareResources(rctx *ReconcileContext, _ *parametersv1alpha1.Parameter) error {
	return rctx.Cluster().
		ComponentAndComponentDef().
		SynthesizedComponent().
		ComponentParameter().
		ParametersDefinitions().
		Complete()
}

func syncComponentParameterStatus(rctx *ReconcileContext, parameter *parametersv1alpha1.Parameter) error {
	syncConfigTemplateStatus := func(status *parametersv1alpha1.ComponentReconfiguringStatus, compParamStatus *parametersv1alpha1.ComponentParameterStatus) {
		for i, parameterStatus := range status.ParameterStatus {
			itemStatus := intctrlutil.GetItemStatus(compParamStatus, parameterStatus.Name)
			if itemStatus != nil {
				status.ParameterStatus[i].ConfigTemplateItemDetailStatus = *itemStatus
			}
		}
	}

	for i := range parameter.Status.ReconfiguringStatus {
		syncConfigTemplateStatus(&parameter.Status.ReconfiguringStatus[i], &rctx.ComponentParameterObj.Status)
	}
	return nil
}
