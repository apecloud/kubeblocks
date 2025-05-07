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
	"fmt"
	"reflect"

	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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
		parameterStatus.Phase = intctrlutil.GetUpdatedParametersReconciledPhase(cm, *compSpec, compStatus)
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

func mergeWithOverride(item *parametersv1alpha1.ConfigTemplateItemDetail, updatedParameters map[string]parametersv1alpha1.ParametersInFile) error {
	if item.ConfigFileParams == nil {
		item.ConfigFileParams = updatedParameters
		return nil
	}
	for key, parameters := range updatedParameters {
		if _, ok := item.ConfigFileParams[key]; !ok {
			item.ConfigFileParams[key] = parameters
			continue
		}
		merged := item.ConfigFileParams[key]
		if parameters.Content != nil {
			merged.Content = parameters.Content
		}
		if err := mergo.Merge(&merged.Parameters, parameters.Parameters, mergo.WithOverride); err != nil {
			return err
		}
		item.ConfigFileParams[key] = merged
	}
	return nil
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
		if err := mergeWithOverride(item, status.UpdatedParameters); err != nil {
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

func classifyParameters(updatedParameters parametersv1alpha1.ComponentParameters, configmaps map[string]*corev1.ConfigMap) func(*ReconcileContext, *parametersv1alpha1.Parameter) error {
	return func(rctx *ReconcileContext, parameter *parametersv1alpha1.Parameter) error {
		if !configctrl.HasValidParameterTemplate(rctx.ConfigRender) {
			return intctrlutil.NewFatalError(fmt.Sprintf("component[%s] does not support reconfigure", rctx.ComponentName))
		}
		classParameters, err := configctrl.ClassifyComponentParameters(updatedParameters,
			flatten(rctx.ParametersDefs),
			rctx.ComponentDefObj.Spec.Configs,
			configmaps,
			rctx.ConfigRender,
		)
		if err != nil {
			return err
		}
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

func validateComponentParameter(parametersDefs []*parametersv1alpha1.ParametersDefinition, descs []parametersv1alpha1.ComponentConfigDescription, parameters map[string]*parametersv1alpha1.ParametersInFile) error {
	if len(parametersDefs) == 0 || len(descs) == 0 {
		return nil
	}
	_, err := configctrl.DoMerge(resolveBaseData(parameters), configctrl.DerefMapValues(parameters), parametersDefs, descs)
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
	return rctx.ComponentAndComponentDef().
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

func handleClusterDeleted(reqCtx intctrlutil.RequestCtx, cli client.Client, parameter *parametersv1alpha1.Parameter, cluster *appsv1.Cluster) (*ctrl.Result, error) {
	if !cluster.IsDeleting() {
		return updateOwnerReference(reqCtx, cli, cluster, parameter)
	}
	reqCtx.Log.Info("cluster is deleting, delete parameter", "parameters", client.ObjectKeyFromObject(parameter))
	if err := cli.Delete(reqCtx.Ctx, parameter); err != nil {
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}
	return intctrlutil.ResultToP(intctrlutil.Reconciled())
}

func updateOwnerReference(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1.Cluster, parameter *parametersv1alpha1.Parameter) (*ctrl.Result, error) {
	clusterName := parameter.Labels[constant.AppInstanceLabelKey]
	if clusterName == parameter.Spec.ClusterName && model.IsOwnerOf(cluster, parameter) {
		return nil, nil
	}

	patch := client.MergeFrom(parameter.DeepCopy())
	if parameter.Labels == nil {
		parameter.Labels = make(map[string]string)
	}
	if !model.IsOwnerOf(cluster, parameter) {
		if err := intctrlutil.SetOwnerReference(cluster, parameter); err != nil {
			return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
		}
	}
	parameter.Labels[constant.AppInstanceLabelKey] = parameter.Spec.ClusterName
	if err := cli.Patch(reqCtx.Ctx, parameter, patch); err != nil {
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}
	return intctrlutil.ResultToP(intctrlutil.Reconciled())
}
