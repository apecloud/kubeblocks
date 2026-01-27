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
	"slices"

	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

func init() {
	registerPolicy(parametersv1alpha1.SyncDynamicReloadPolicy, syncPolicyInst)
}

var syncPolicyInst = &syncPolicy{}

type syncPolicy struct{}

func (o *syncPolicy) Upgrade(rctx reconfigureContext) (reconfigureStatus, error) {
	updateParams := o.updateParameters(rctx)
	if len(updateParams) == 0 {
		return makeReconfigureStatus(reconfigureStatusNone), nil
	}
	return submitUpdatedConfig(rctx, updateParams, false)
}

func (o *syncPolicy) updateParameters(rctx reconfigureContext) map[string]string {
	var (
		paramDef               = rctx.ParametersDef
		dynamicAction          = parameters.NeedDynamicReloadAction(paramDef)
		needReloadStaticParams = parameters.ReloadStaticParameters(paramDef)
		visualizedParams       = core.GenerateVisualizedParamsList(rctx.Patch,
			[]parametersv1alpha1.ComponentConfigDescription{*rctx.ConfigDescription})
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
	return params
}

func submitUpdatedConfig(rctx reconfigureContext, parameters map[string]string, restart bool) (reconfigureStatus, error) {
	var config *appsv1.ClusterComponentConfig
	for i, cfg := range rctx.ClusterComponent.Configs {
		if ptr.Deref(cfg.Name, "") == rctx.ConfigTemplate.Name {
			config = &rctx.ClusterComponent.Configs[i]
			break
		}
	}
	if config == nil {
		// TODO: fix me
		return makeReconfigureStatus(reconfigureStatusFailedAndRetry), fmt.Errorf("config %s not found", rctx.ConfigTemplate.Name)
	}
	if !ptr.Equal(config.ConfigHash, rctx.getTargetConfigHash()) {
		return applyConfigChangesToCluster(rctx, config, parameters, restart), nil
	}
	return syncConfigStatus(rctx), nil
}

func applyConfigChangesToCluster(rctx reconfigureContext, config *appsv1.ClusterComponentConfig, parameters map[string]string, restart bool) reconfigureStatus {
	config.Variables = parameters
	config.ConfigHash = rctx.getTargetConfigHash()
	if restart {
		config.RestartOnConfigChange = ptr.To(true)
	} else {
		config.RestartOnConfigChange = nil
	}
	return makeReconfigureStatus(reconfigureStatusRetry, withExpected(int32(rctx.getTargetReplicas())), withSucceed(0))
}

func syncConfigStatus(rctx reconfigureContext) reconfigureStatus {
	var (
		replicas   = int32(rctx.getTargetReplicas())
		configHash = rctx.getTargetConfigHash()
	)
	updated := int32(0)
	if rctx.its != nil {
		for _, inst := range rctx.its.Status.InstanceStatus {
			idx := slices.IndexFunc(inst.Configs, func(cfg workloads.InstanceConfigStatus) bool {
				return cfg.Name == rctx.ConfigTemplate.Name
			})
			if idx >= 0 && ptr.Equal(inst.Configs[idx].ConfigHash, configHash) {
				updated++
			}
		}
	}
	if updated == replicas {
		return makeReconfigureStatus(reconfigureStatusNone, withExpected(replicas), withSucceed(updated))
	}
	return makeReconfigureStatus(reconfigureStatusRetry, withExpected(replicas), withSucceed(updated))
}
