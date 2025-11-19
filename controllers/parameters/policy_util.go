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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	cfgcm "github.com/apecloud/kubeblocks/pkg/parameters/configmanager"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

// GetComponentPods gets all pods of the component.
func GetComponentPods(params reconfigureContext) ([]corev1.Pod, error) {
	componentPods := make([]corev1.Pod, 0)
	for i := range params.InstanceSetUnits {
		pods, err := intctrlutil.GetPodListByInstanceSet(params.Ctx, params.Client, &params.InstanceSetUnits[i])
		if err != nil {
			return nil, err
		}
		componentPods = append(componentPods, pods...)
	}
	return componentPods, nil
}

// CheckReconfigureUpdateProgress checks pods of the component is ready.
func CheckReconfigureUpdateProgress(pods []corev1.Pod, configKey, version string) int32 {
	var (
		readyPods        int32 = 0
		cfgAnnotationKey       = core.GenerateUniqKeyWithConfig(constant.UpgradeRestartAnnotationKey, configKey)
	)

	for _, pod := range pods {
		annotations := pod.Annotations
		if len(annotations) != 0 && annotations[cfgAnnotationKey] == version && intctrlutil.IsPodReady(&pod) {
			readyPods++
		}
	}
	return readyPods
}

func getPodsForOnlineUpdate(params reconfigureContext) ([]corev1.Pod, error) {
	if len(params.InstanceSetUnits) > 1 {
		return nil, core.MakeError("component require only one InstanceSet, actual %d components", len(params.InstanceSetUnits))
	}

	if len(params.InstanceSetUnits) == 0 {
		return nil, nil
	}

	pods, err := GetComponentPods(params)
	if err != nil {
		return nil, err
	}

	if params.SynthesizedComponent != nil {
		instanceset.SortPods(
			pods,
			instanceset.ComposeRolePriorityMap(params.SynthesizedComponent.Roles),
			true,
		)
	}
	return pods, nil
}

func commonOnlineUpdateWithPod(pod *corev1.Pod, ctx context.Context, configSpec string, configFile string, updatedParams map[string]string) error {
	// TODO: update cluster spec to call the reconfigure action
	return fmt.Errorf("not yet implemented")
}

func getComponentSpecPtrByName(cli client.Client, ctx intctrlutil.RequestCtx, cluster *appsv1.Cluster, compName string) (*appsv1.ClusterComponentSpec, error) {
	for i := range cluster.Spec.ComponentSpecs {
		componentSpec := &cluster.Spec.ComponentSpecs[i]
		if componentSpec.Name == compName {
			return componentSpec, nil
		}
	}
	// check if the component is a sharding component
	compObjList := &appsv1.ComponentList{}
	if err := cli.List(ctx.Ctx, compObjList, client.MatchingLabels{
		constant.AppInstanceLabelKey:    cluster.Name,
		constant.KBAppComponentLabelKey: compName,
	}); err != nil {
		return nil, err
	}
	if len(compObjList.Items) > 0 {
		shardingName := compObjList.Items[0].Labels[constant.KBAppShardingNameLabelKey]
		if shardingName != "" {
			for i := range cluster.Spec.Shardings {
				shardSpec := &cluster.Spec.Shardings[i]
				if shardSpec.Name == shardingName {
					return &shardSpec.Template, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("component %s not found", compName)
}

func restartComponent(cli client.Client, ctx intctrlutil.RequestCtx, configKey string, newVersion string, cluster *appsv1.Cluster, compName string) error {
	cfgAnnotationKey := core.GenerateUniqKeyWithConfig(constant.UpgradeRestartAnnotationKey, configKey)

	compSpec, err := getComponentSpecPtrByName(cli, ctx, cluster, compName)
	if err != nil {
		return err
	}

	if compSpec.Annotations == nil {
		compSpec.Annotations = map[string]string{}
	}

	if compSpec.Annotations[cfgAnnotationKey] == newVersion {
		return nil
	}

	compSpec.Annotations[cfgAnnotationKey] = newVersion

	return cli.Update(ctx.Ctx, cluster)
}

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
	case cfgcm.IsAutoReload(pd.ReloadAction): // if core support hot update, don't need to do anything
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
	reCtx := reconfigureContext{
		RequestCtx:           rctx.RequestCtx,
		Client:               rctx.Client,
		ConfigTemplate:       *templateSpec,
		ConfigMap:            rctx.ConfigMap,
		ParametersDef:        &pd.Spec,
		ConfigDescription:    configDescription,
		Cluster:              rctx.ClusterObj,
		ContainerNames:       rctx.Containers,
		InstanceSetUnits:     rctx.InstanceSetList,
		ClusterComponent:     rctx.ClusterComObj,
		SynthesizedComponent: rctx.BuiltinComponent,
		Patch:                patch,
	}

	return reconfigureTask{ReloadPolicy: reloadPolicy, taskCtx: reCtx}
}

func buildRestartTask(configTemplate *appsv1.ComponentFileTemplate, rctx *ReconcileContext) reconfigureTask {
	return reconfigureTask{
		ReloadPolicy: parametersv1alpha1.RestartPolicy,
		taskCtx: reconfigureContext{
			RequestCtx:           rctx.RequestCtx,
			Client:               rctx.Client,
			ConfigTemplate:       *configTemplate,
			ClusterComponent:     rctx.ClusterComObj,
			Cluster:              rctx.ClusterObj,
			SynthesizedComponent: rctx.BuiltinComponent,
			InstanceSetUnits:     rctx.InstanceSetList,
			ConfigMap:            rctx.ConfigMap,
		},
	}
}

func generateOnlineUpdateParams(configPatch *core.ConfigPatchInfo, paramDef *parametersv1alpha1.ParametersDefinitionSpec, description parametersv1alpha1.ComponentConfigDescription) map[string]string {
	params := make(map[string]string)
	dynamicAction := parameters.NeedDynamicReloadAction(paramDef)
	needReloadStaticParams := parameters.ReloadStaticParameters(paramDef)
	visualizedParams := core.GenerateVisualizedParamsList(configPatch, []parametersv1alpha1.ComponentConfigDescription{description})

	for _, key := range visualizedParams {
		if key.UpdateType != core.UpdatedType {
			continue
		}
		for _, p := range key.Parameters {
			if dynamicAction && !needReloadStaticParams && !core.IsDynamicParameter(p.Key, paramDef) {
				continue
			}
			if p.Value != nil {
				params[p.Key] = *p.Value
			}
		}
	}
	return params
}
