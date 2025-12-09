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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

type ReloadAction interface {
	ExecReload() (returnedStatus, error)
	ReloadType() string
}

type reconfigureTask struct {
	parametersv1alpha1.ReloadPolicy
	taskCtx reconfigureContext
}

func (r reconfigureTask) ReloadType() string {
	return string(r.ReloadPolicy)
}

func (r reconfigureTask) ExecReload() (returnedStatus, error) {
	if executor, ok := upgradePolicyMap[r.ReloadPolicy]; ok {
		return executor.Upgrade(r.taskCtx)
	}
	return returnedStatus{}, fmt.Errorf("not support reload action[%s]", r.ReloadPolicy)
}

func resolveReloadActionPolicy(jsonPatch string,
	format *parametersv1alpha1.FileFormatConfig,
	pd *parametersv1alpha1.ParametersDefinitionSpec) (parametersv1alpha1.ReloadPolicy, error) {
	var policy = parametersv1alpha1.NonePolicy
	dynamicUpdate, err := core.CheckUpdateDynamicParameters(format, pd, jsonPatch)
	if err != nil {
		return policy, err
	}

	// make decision
	switch {
	case !dynamicUpdate && parameters.NeedDynamicReloadAction(pd): // static parameters update and need to do hot update
		policy = parametersv1alpha1.DynamicReloadAndRestartPolicy
	case !dynamicUpdate: // static parameters update and only need to restart
		policy = parametersv1alpha1.RestartPolicy
	case parameters.IsAutoReload(pd.ReloadAction): // if core support hot update, don't need to do anything
		policy = parametersv1alpha1.AsyncDynamicReloadPolicy
	case enableSyncTrigger(pd.ReloadAction): // sync config-manager exec hot update
		policy = parametersv1alpha1.SyncDynamicReloadPolicy
	default: // config-manager auto trigger to hot update
		policy = parametersv1alpha1.AsyncDynamicReloadPolicy
	}
	return policy, nil
}

// genReconfigureActionTasks generates a list of reconfiguration tasks based on the provided templateSpec,
// reconfiguration context, configuration patch, and a restart flag.
func genReconfigureActionTasks(templateSpec *appsv1.ComponentFileTemplate, rctx *ReconcileContext, patch *core.ConfigPatchInfo, restart bool) ([]ReloadAction, error) {
	var tasks []ReloadAction

	// If the patch or ConfigRender is nil, return a single restart task.
	if patch == nil || rctx.ConfigRender == nil {
		return []ReloadAction{buildRestartTask(templateSpec, rctx)}, nil
	}

	// needReloadAction determines if a reload action is needed based on the ParametersDefinition and ReloadPolicy.
	needReloadAction := func(pd *parametersv1alpha1.ParametersDefinition, policy parametersv1alpha1.ReloadPolicy) bool {
		return !restart || (policy == parametersv1alpha1.SyncDynamicReloadPolicy && parameters.NeedDynamicReloadAction(&pd.Spec))
	}

	for key, jsonPatch := range patch.UpdateConfig {
		pd, ok := rctx.ParametersDefs[key]
		// If the ParametersDefinition or its ReloadAction is nil, continue to the next iteration.
		if !ok || pd.Spec.ReloadAction == nil {
			continue
		}
		configFormat := parameters.GetComponentConfigDescription(&rctx.ConfigRender.Spec, key)
		if configFormat == nil || configFormat.FileFormatConfig == nil {
			continue
		}
		// Determine the appropriate ReloadPolicy.
		policy, err := resolveReloadActionPolicy(string(jsonPatch), configFormat.FileFormatConfig, &pd.Spec)
		if err != nil {
			return nil, err
		}
		// If a reload action is needed, append a new reload action task to the tasks slice.
		if needReloadAction(pd, policy) {
			tasks = append(tasks, buildReloadActionTask(policy, templateSpec, rctx, pd, configFormat, patch))
		}
	}

	// If no tasks were added, return a single restart task.
	if len(tasks) == 0 {
		return []ReloadAction{buildRestartTask(templateSpec, rctx)}, nil
	}

	return tasks, nil
}

func buildReloadActionTask(reloadPolicy parametersv1alpha1.ReloadPolicy, templateSpec *appsv1.ComponentFileTemplate, rctx *ReconcileContext, pd *parametersv1alpha1.ParametersDefinition, configDescription *parametersv1alpha1.ComponentConfigDescription, patch *core.ConfigPatchInfo) reconfigureTask {
	return reconfigureTask{
		ReloadPolicy: reloadPolicy,
		taskCtx: reconfigureContext{
			RequestCtx:           rctx.RequestCtx,
			Client:               rctx.Client,
			ConfigTemplate:       *templateSpec,
			VersionHash:          computeTargetVersionHash(rctx.RequestCtx, rctx.ConfigMap.Data),
			ParametersDef:        &pd.Spec,
			ConfigDescription:    configDescription,
			Cluster:              rctx.ClusterObj,
			ClusterComponent:     rctx.ClusterComObj,
			SynthesizedComponent: rctx.BuiltinComponent,
			its:                  rctx.its,
			Patch:                patch,
		},
	}
}

func buildRestartTask(configTemplate *appsv1.ComponentFileTemplate, rctx *ReconcileContext) reconfigureTask {
	return reconfigureTask{
		ReloadPolicy: parametersv1alpha1.RestartPolicy,
		taskCtx: reconfigureContext{
			RequestCtx:           rctx.RequestCtx,
			Client:               rctx.Client,
			ConfigTemplate:       *configTemplate,
			VersionHash:          computeTargetVersionHash(rctx.RequestCtx, rctx.ConfigMap.Data),
			ClusterComponent:     rctx.ClusterComObj,
			Cluster:              rctx.ClusterObj,
			SynthesizedComponent: rctx.BuiltinComponent,
			its:                  rctx.its,
		},
	}
}
