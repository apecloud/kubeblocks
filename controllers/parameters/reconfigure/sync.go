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

package reconfigure

import (
	"encoding/json"
	"slices"
	"strconv"

	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

func init() {
	registerPolicy(parametersv1alpha1.SyncDynamicReloadPolicy, syncPolicy)
	registerPolicy(parametersv1alpha1.DynamicReloadAndRestartPolicy, syncNRestartPolicy)
}

var (
	syncPolicy         = createSyncPolicy(false)
	syncNRestartPolicy = createSyncPolicy(true)
)

func createSyncPolicy(restart bool) func(Context) (Status, error) {
	return func(ctx Context) (Status, error) {
		var (
			paramDef               = ctx.ParametersDef
			dynamicAction          = parameters.NeedDynamicReloadAction(paramDef)
			needReloadStaticParams = parameters.ReloadStaticParameters(paramDef)
			visualizedParams       = core.GenerateVisualizedParamsList(ctx.Patch,
				[]parametersv1alpha1.ComponentConfigDescription{*ctx.ConfigDescription})
		)
		params := make(map[string]string)
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
		if len(params) == 0 {
			// Legacy reload generation in release-1.1 only happened when there were
			// reloadable params left after filtering. Keep the same gate here for
			// compatibility, even though "should invoke reload action" is not purely
			// equivalent to "has reloadable params" as an abstract rule.
			//
			// No reloadable params, but a restart can still be required (static params
			// change or merge-reload-and-restart). Keep the same release-1.1 fallback:
			// when no reload task is generated, we still submit the restart.
			if restart {
				return submit(ctx, nil, true)
			}
			return makeStatus(StatusNone, withReason("has NO updated parameters")), nil
		}
		if shouldBuildLegacyReconfigureAction(ctx, params, restart) {
			if err := ValidateLegacyConfigManagerRuntime(ctx.ITS); err != nil {
				return makeStatus(StatusFailed, withReason(err.Error())), nil
			}
		}
		return submit(ctx, params, restart)
	}
}

func submit(ctx Context, parameters map[string]string, restart bool) (Status, error) {
	var config *appsv1.ClusterComponentConfig
	for i, cfg := range ctx.ClusterComponent.Configs {
		if ptr.Deref(cfg.Name, "") == ctx.ConfigTemplate.Name {
			config = &ctx.ClusterComponent.Configs[i]
			break
		}
	}
	if config == nil {
		// TODO: remove me after the ConfigMap source is set to the Cluster object
		ctx.ClusterComponent.Configs = append(ctx.ClusterComponent.Configs, appsv1.ClusterComponentConfig{
			Name: ptr.To(ctx.ConfigTemplate.Name),
			// do not set the ConfigMap source here, it will be merged in copyAndMergeComponent on the Component object
		})
		config = &ctx.ClusterComponent.Configs[len(ctx.ClusterComponent.Configs)-1]
	}
	if !ptr.Equal(config.ConfigHash, ctx.getTargetConfigHash()) {
		return applyChangesToCluster(ctx, config, parameters, restart), nil
	}
	return syncReconfigureStatus(ctx), nil
}

func applyChangesToCluster(ctx Context, config *appsv1.ClusterComponentConfig, params map[string]string, restart bool) Status {
	config.Variables = params
	config.ConfigHash = ctx.getTargetConfigHash()
	if restart {
		config.Restart = ptr.To(true)
	} else {
		config.Restart = nil
	}
	if shouldBuildLegacyReconfigureAction(ctx, params, restart) {
		config.Reconfigure = reloadActionToReconfigureAction(ctx, params)
	} else {
		config.Reconfigure = nil
	}
	return makeStatus(StatusRetry, withReason("apply changes to cluster API"), withExpected(int32(ctx.getTargetReplicas())), withSucceed(0))
}

// shouldBuildLegacyReconfigureAction returns true only when this change should be translated
// into a config-manager gRPC proxy call. This excludes auto-triggered legacy reloads and the
// merged-restart case where release-1.1 would restart without issuing a separate sync reload.
func shouldBuildLegacyReconfigureAction(ctx Context, params map[string]string, restart bool) bool {
	if len(params) == 0 {
		// This compatibility path intentionally follows the legacy task-generation
		// rule from release-1.1: without reloadable params, no standalone reload task
		// was produced. Keep that behavior here so the new reconfigure action matches
		// the old reload-action trigger conditions.
		return false
	}
	if ctx.ParametersDef == nil || ctx.ParametersDef.ReloadAction == nil {
		return false
	}
	if ctx.ParametersDef.ReloadAction.AutoTrigger != nil {
		return false
	}
	if ctx.ParametersDef.ReloadAction.ShellTrigger == nil {
		return false
	}
	if restart && !parameters.NeedDynamicReloadAction(ctx.ParametersDef) {
		return false
	}
	return true
}

func reloadActionToReconfigureAction(ctx Context, params map[string]string) *appsv1.Action {
	pd := ctx.ParametersDef
	if pd == nil || pd.ReloadAction == nil || pd.ReloadAction.ShellTrigger == nil {
		return nil
	}
	request, err := legacyConfigManagerRequestParams(ctx, params)
	if err != nil {
		ctx.Log.Error(err, "failed to build config-manager proxy request")
		return nil
	}
	return &appsv1.Action{
		GRPC: &appsv1.GRPCAction{
			Port:    strconv.Itoa(resolveLegacyConfigManagerPort(ctx.ITS)),
			Service: legacyConfigManagerGRPCService,
			Method:  legacyConfigManagerGRPCMethod,
			Request: request,
			Response: appsv1.GRPCResponse{
				Status: "errMessage",
			},
		},
	}
}

func legacyConfigManagerRequestParams(ctx Context, params map[string]string) (appsv1.GRPCRequest, error) {
	paramsData, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return appsv1.GRPCRequest{
		"configSpec": ctx.ConfigTemplate.Name,
		"configFile": resolveConfigFile(ctx.ConfigDescription),
		"params":     string(paramsData),
	}, nil
}

func resolveConfigFile(desc *parametersv1alpha1.ComponentConfigDescription) string {
	if desc == nil {
		return ""
	}
	return desc.Name
}

func syncReconfigureStatus(ctx Context) Status {
	var (
		replicas   = int32(ctx.getTargetReplicas())
		configHash = ctx.getTargetConfigHash()
	)
	updated := int32(0)
	if ctx.ITS != nil {
		for _, inst := range ctx.ITS.Status.InstanceStatus {
			idx := slices.IndexFunc(inst.Configs, func(cfg workloads.InstanceConfigStatus) bool {
				return cfg.Name == ctx.ConfigTemplate.Name
			})
			if idx >= 0 && ptr.Equal(inst.Configs[idx].ConfigHash, configHash) {
				updated++
			}
		}
	}
	if updated == replicas {
		return makeStatus(StatusNone, withReason("reconfigure completed"), withExpected(replicas), withSucceed(updated))
	}
	return makeStatus(StatusRetry, withReason("reconfiguring"), withExpected(replicas), withSucceed(updated))
}
