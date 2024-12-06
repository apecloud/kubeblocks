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
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// type ValidateConfigMap func(configTpl, ns string) (*corev1.ConfigMap, error)
// type ValidateConfigSchema func(tpl *appsv1beta1.ParametersSchema) (bool, error)

func checkConfigLabels(object client.Object, requiredLabs []string) bool {
	labels := object.GetLabels()
	if len(labels) == 0 {
		return false
	}

	for _, label := range requiredLabs {
		if _, ok := labels[label]; !ok {
			return false
		}
	}

	// reconfigure ConfigMap for db instance
	if ins, ok := labels[constant.CMConfigurationTypeLabelKey]; !ok || ins != constant.ConfigInstanceType {
		return false
	}

	return checkEnableCfgUpgrade(object)
}

func createConfigPatch(cfg *corev1.ConfigMap, configRender *parametersv1alpha1.ParameterDrivenConfigRender, paramsDefs map[string]*parametersv1alpha1.ParametersDefinition) (*core.ConfigPatchInfo, bool, error) {
	if configRender == nil || len(configRender.Spec.Configs) == 0 {
		return nil, true, nil
	}
	lastConfig, err := getLastVersionConfig(cfg)
	if err != nil {
		return nil, false, core.WrapError(err, "failed to get last version data. config[%v]", client.ObjectKeyFromObject(cfg))
	}

	patch, restart, err := core.CreateConfigPatch(lastConfig, cfg.Data, configRender.Spec, true)
	if err != nil {
		return nil, false, err
	}
	if !restart {
		restart = cfgcm.NeedRestart(paramsDefs, patch)
	}
	return patch, restart, nil
}

func generateReconcileTasks(reqCtx intctrlutil.RequestCtx, componentParameter *parametersv1alpha1.ComponentParameter) []Task {
	tasks := make([]Task, 0, len(componentParameter.Spec.ConfigItemDetails))
	for _, item := range componentParameter.Spec.ConfigItemDetails {
		if status := fromItemStatus(reqCtx, &componentParameter.Status, item, componentParameter.GetGeneration()); status != nil {
			tasks = append(tasks, NewTask(item, status))
		}
	}
	return tasks
}

func fromItemStatus(ctx intctrlutil.RequestCtx, status *parametersv1alpha1.ComponentParameterStatus, item parametersv1alpha1.ConfigTemplateItemDetail, generation int64) *parametersv1alpha1.ConfigTemplateItemDetailStatus {
	if item.ConfigSpec == nil {
		ctx.Log.V(1).WithName(item.Name).Info(fmt.Sprintf("configuration is creating and pass: %s", item.Name))
		return nil
	}
	itemStatus := intctrlutil.GetItemStatus(status, item.Name)
	if itemStatus == nil || itemStatus.Phase == "" {
		ctx.Log.WithName(item.Name).Info(fmt.Sprintf("ComponentParameters cr is creating: %v", item))
		status.ConfigurationItemStatus = append(status.ConfigurationItemStatus, parametersv1alpha1.ConfigTemplateItemDetailStatus{
			Name:           item.Name,
			Phase:          parametersv1alpha1.CInitPhase,
			UpdateRevision: strconv.FormatInt(generation, 10),
		})
		itemStatus = intctrlutil.GetItemStatus(status, item.Name)
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
