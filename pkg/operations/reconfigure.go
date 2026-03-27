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

package operations

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	configcore "github.com/apecloud/kubeblocks/pkg/parameters/core"
)

type reconfigureAction struct {
}

func init() {
	reAction := reconfigureAction{}
	opsManager := GetOpsManager()
	reconfigureBehaviour := OpsBehaviour{
		// REVIEW: can do opsrequest if not running?
		FromClusterPhases: appsv1.GetReconfiguringRunningPhases(),
		// TODO: add cluster reconcile Reconfiguring phase.
		ToClusterPhase: appsv1.UpdatingClusterPhase,
		QueueByCluster: true,
		OpsHandler:     &reAction,
	}
	opsManager.RegisterOps(opsv1alpha1.ReconfiguringType, reconfigureBehaviour)
}

var noRequeueAfter time.Duration = 0

// ActionStartedCondition the started condition when handle the reconfiguring request.
func (r *reconfigureAction) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewReconfigureCondition(opsRes.OpsRequest), nil
}

func (r *reconfigureAction) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func (r *reconfigureAction) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	opsDeepCopy := resource.OpsRequest.DeepCopy()
	phase, msg, err := r.aggregatePhase(reqCtx, cli, resource)
	if err != nil {
		return "", noRequeueAfter, err
	}
	if phase == opsv1alpha1.OpsRunningPhase {
		return syncReconfigureForOps(reqCtx, cli, resource, opsDeepCopy, opsv1alpha1.OpsRunningPhase)
	}
	if phase == opsv1alpha1.OpsSucceedPhase {
		return syncReconfigureForOps(reqCtx, cli, resource, opsDeepCopy, opsv1alpha1.OpsSucceedPhase)
	}
	return opsv1alpha1.OpsFailedPhase, 0, intctrlutil.NewFatalError(fmt.Sprintf("reconfigure component parameter failed: %s", msg))
}

func syncReconfigureForOps(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource, opsDeepCopy *opsv1alpha1.OpsRequest, phase opsv1alpha1.OpsPhase) (opsv1alpha1.OpsPhase, time.Duration, error) {
	if err := PatchOpsStatusWithOpsDeepCopy(reqCtx.Ctx, cli, resource, opsDeepCopy, phase); err != nil {
		return "", noRequeueAfter, err
	}
	return phase, noRequeueAfter, nil
}

func (r *reconfigureAction) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) (err error) {
	if !intctrlutil.ObjectAPIVersionSupported(resource.Cluster) {
		return intctrlutil.NewFatalError(fmt.Sprintf(`api version "%s" is not supported, you can upgrade the cluster to v1 version`, resource.Cluster.APIVersion))
	}

	if len(resource.OpsRequest.Spec.Reconfigures) == 0 {
		return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, `invalid reconfigure request: %s`, resource.OpsRequest.GetName())
	}
	for _, reconfigure := range resource.OpsRequest.Spec.Reconfigures {
		if len(reconfigure.Parameters) == 0 && len(reconfigure.UserConfigTemplates) == 0 {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "invalid reconfigure request for component %s: no parameters or userConfigTemplates", reconfigure.ComponentName)
		}
		componentNames, err := resolveReconfigureComponents(reqCtx.Ctx, cli, resource.Cluster, reconfigure.ComponentName)
		if err != nil {
			return err
		}
		for _, componentName := range componentNames {
			if err := applyReconfigureToComponentParameter(reqCtx, cli, resource.Cluster, componentName, reconfigure); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *reconfigureAction) aggregatePhase(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) (opsv1alpha1.OpsPhase, string, error) {
	for _, reconfigure := range resource.OpsRequest.Spec.Reconfigures {
		componentNames, err := resolveReconfigureComponents(reqCtx.Ctx, cli, resource.Cluster, reconfigure.ComponentName)
		if err != nil {
			return "", "", err
		}
		for _, componentName := range componentNames {
			targetTemplates, err := resolveReconfigureTargetTemplates(reqCtx, cli, resource.Cluster, componentName, reconfigure)
			if err != nil {
				return "", "", err
			}
			componentParameter, err := getRunningComponentParameter(reqCtx.Ctx, cli, resource.Cluster.Namespace, resource.Cluster.Name, componentName)
			if err != nil {
				return "", "", err
			}
			if componentParameter.Generation != componentParameter.Status.ObservedGeneration {
				return opsv1alpha1.OpsRunningPhase, "", nil
			}
			for _, templateName := range targetTemplates {
				itemStatus := parameters.GetItemStatus(&componentParameter.Status, templateName)
				if itemStatus == nil {
					return opsv1alpha1.OpsRunningPhase, "", nil
				}
				switch itemStatus.Phase {
				case parametersv1alpha1.CMergeFailedPhase, parametersv1alpha1.CFailedAndPausePhase:
					return opsv1alpha1.OpsFailedPhase, itemStatusMessage(itemStatus), nil
				case parametersv1alpha1.CFinishedPhase:
					continue
				default:
					return opsv1alpha1.OpsRunningPhase, "", nil
				}
			}
		}
	}
	return opsv1alpha1.OpsSucceedPhase, "", nil
}

func transformComponentParameters(params []opsv1alpha1.ParameterPair) parametersv1alpha1.ComponentParameters {
	ret := make(parametersv1alpha1.ComponentParameters, len(params))
	for _, param := range params {
		ret[param.Key] = param.Value
	}
	return ret
}

func applyReconfigureToComponentParameter(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1.Cluster, componentName string, reconfigure opsv1alpha1.Reconfigure) error {
	componentParameter, err := getRunningComponentParameter(reqCtx.Ctx, cli, cluster.Namespace, cluster.Name, componentName)
	if err != nil {
		return err
	}
	componentDefObj := &appsv1.ComponentDefinition{}
	componentDefName := componentParameter.Labels[constant.AppComponentLabelKey]
	if componentDefName == "" {
		componentObj, err := getComponentByShortName(reqCtx.Ctx, cli, cluster.Namespace, cluster.Name, componentName)
		if err != nil {
			return err
		}
		componentDefName = componentObj.Spec.CompDef
	}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: componentDefName}, componentDefObj); err != nil {
		return err
	}
	configDescs, paramsDefs, err := parameters.ResolveCmpdParametersDefs(reqCtx.Ctx, cli, componentDefObj)
	if err != nil {
		return err
	}
	configmaps, err := resolveComponentRefConfigMapForOps(reqCtx.Ctx, cli, cluster.Namespace, cluster.Name, componentName)
	if err != nil {
		return err
	}
	if err := validateCustomTemplates(reqCtx.Ctx, cli, reconfigure.UserConfigTemplates); err != nil {
		return err
	}
	patch := client.MergeFrom(componentParameter.DeepCopy())
	if len(reconfigure.Parameters) != 0 {
		classifiedParameters, err := parameters.ClassifyComponentParameters(
			transformComponentParameters(reconfigure.Parameters),
			paramsDefs,
			componentDefObj.Spec.Configs,
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
			if _, err := parameters.DoMerge(resolveBaseDataForOps(paramsInFile), parameters.DerefMapValues(paramsInFile), paramsDefs, configDescriptions); err != nil {
				return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "%s", err.Error())
			}
			item := parameters.GetConfigTemplateItem(&componentParameter.Spec, templateName)
			if item == nil {
				return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "not found config template item: %s", templateName)
			}
			if err := mergeWithOverrideForOps(item, parameters.DerefMapValues(paramsInFile)); err != nil {
				return err
			}
		}
	}
	for templateName, templateExtension := range reconfigure.UserConfigTemplates {
		item := parameters.GetConfigTemplateItem(&componentParameter.Spec, templateName)
		if item == nil {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "not found config template item: %s", templateName)
		}
		item.CustomTemplates = templateExtension.DeepCopy()
	}
	if err := cli.Patch(reqCtx.Ctx, componentParameter, patch); err != nil {
		return err
	}
	return nil
}

func resolveReconfigureTargetTemplates(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1.Cluster, componentName string, reconfigure opsv1alpha1.Reconfigure) ([]string, error) {
	templateSet := map[string]struct{}{}
	for templateName := range reconfigure.UserConfigTemplates {
		templateSet[templateName] = struct{}{}
	}
	if len(reconfigure.Parameters) != 0 {
		componentParameter, err := getRunningComponentParameter(reqCtx.Ctx, cli, cluster.Namespace, cluster.Name, componentName)
		if err != nil {
			return nil, err
		}
		componentDefObj := &appsv1.ComponentDefinition{}
		componentDefName := componentParameter.Labels[constant.AppComponentLabelKey]
		if componentDefName == "" {
			componentObj, err := getComponentByShortName(reqCtx.Ctx, cli, cluster.Namespace, cluster.Name, componentName)
			if err != nil {
				return nil, err
			}
			componentDefName = componentObj.Spec.CompDef
		}
		if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: componentDefName}, componentDefObj); err != nil {
			return nil, err
		}
		configDescs, paramsDefs, err := parameters.ResolveCmpdParametersDefs(reqCtx.Ctx, cli, componentDefObj)
		if err != nil {
			return nil, err
		}
		configmaps, err := resolveComponentRefConfigMapForOps(reqCtx.Ctx, cli, cluster.Namespace, cluster.Name, componentName)
		if err != nil {
			return nil, err
		}
		classifiedParameters, err := parameters.ClassifyComponentParameters(
			transformComponentParameters(reconfigure.Parameters),
			paramsDefs,
			componentDefObj.Spec.Configs,
			configmaps,
			configDescs,
		)
		if err != nil {
			return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "%s", err.Error())
		}
		for templateName := range classifiedParameters {
			templateSet[templateName] = struct{}{}
		}
	}
	templates := make([]string, 0, len(templateSet))
	for templateName := range templateSet {
		templates = append(templates, templateName)
	}
	return templates, nil
}

func resolveReconfigureComponents(ctx context.Context, reader client.Reader, cluster *appsv1.Cluster, componentName string) ([]string, error) {
	if compSpec := cluster.Spec.GetComponentByName(componentName); compSpec != nil {
		return []string{compSpec.Name}, nil
	}
	shardingComp := cluster.Spec.GetShardingByName(componentName)
	if shardingComp == nil {
		return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "component not found: %s", componentName)
	}
	components, err := sharding.ListShardingComponents(ctx, reader, cluster, componentName)
	if err != nil {
		return nil, err
	}
	componentNames := make([]string, 0, len(components))
	for _, comp := range components {
		shortName, err := component.ShortName(cluster.Name, comp.Name)
		if err != nil {
			return nil, err
		}
		componentNames = append(componentNames, shortName)
	}
	return componentNames, nil
}

func getRunningComponentParameter(ctx context.Context, cli client.Client, namespace, clusterName, componentName string) (*parametersv1alpha1.ComponentParameter, error) {
	componentParameter := &parametersv1alpha1.ComponentParameter{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      configcore.GenerateComponentConfigurationName(clusterName, componentName),
	}
	if err := cli.Get(ctx, key, componentParameter); err != nil {
		return nil, err
	}
	return componentParameter, nil
}

func getComponentByShortName(ctx context.Context, cli client.Client, namespace, clusterName, shortName string) (*appsv1.Component, error) {
	componentList := &appsv1.ComponentList{}
	if err := cli.List(ctx, componentList, client.InNamespace(namespace), client.MatchingLabels{constant.AppInstanceLabelKey: clusterName}); err != nil {
		return nil, err
	}
	for i := range componentList.Items {
		componentObj := &componentList.Items[i]
		name, err := component.ShortName(clusterName, componentObj.Name)
		if err != nil {
			return nil, err
		}
		if name == shortName {
			return componentObj, nil
		}
	}
	return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "component not found: %s", shortName)
}

func resolveComponentRefConfigMapForOps(ctx context.Context, cli client.Client, namespace, clusterName, componentName string) (map[string]*corev1.ConfigMap, error) {
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

func validateCustomTemplates(ctx context.Context, cli client.Reader, customTemplates map[string]parametersv1alpha1.ConfigTemplateExtension) error {
	for _, tpl := range customTemplates {
		configMap := &corev1.ConfigMap{}
		namespace := tpl.Namespace
		if namespace == "" {
			namespace = "default"
		}
		if err := cli.Get(ctx, client.ObjectKey{Name: tpl.TemplateRef, Namespace: namespace}, configMap); err != nil {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "not found configmap[%s/%s] for custom template", namespace, tpl.TemplateRef)
		}
	}
	return nil
}

func mergeWithOverrideForOps(item *parametersv1alpha1.ConfigTemplateItemDetail, updatedParameters map[string]parametersv1alpha1.ParametersInFile) error {
	if item.ConfigFileParams == nil {
		item.ConfigFileParams = updatedParameters
		return nil
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
	return nil
}

func resolveBaseDataForOps(updatedParameters map[string]*parametersv1alpha1.ParametersInFile) map[string]string {
	baseData := make(map[string]string, len(updatedParameters))
	for key := range updatedParameters {
		baseData[key] = ""
	}
	return baseData
}

func itemStatusMessage(status *parametersv1alpha1.ConfigTemplateItemDetailStatus) string {
	if status == nil || status.Message == nil {
		return ""
	}
	return *status.Message
}
