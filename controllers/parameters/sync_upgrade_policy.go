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

	apisappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

var syncPolicyInstance = &syncPolicy{}

type syncPolicy struct{}

func init() {
	registerPolicy(parametersv1alpha1.SyncDynamicReloadPolicy, syncPolicyInstance)
}

func (o *syncPolicy) Upgrade(rctx reconfigureContext) (returnedStatus, error) {
	updateParams := o.updateParameters(rctx)
	if len(updateParams) == 0 {
		return makeReturnedStatus(ESNone), nil
	}
	return o.sync(rctx, updateParams)
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

func (o *syncPolicy) sync(rctx reconfigureContext, parameters map[string]string) (returnedStatus, error) {
	var config *apisappsv1.ClusterComponentConfig
	for i, cfg := range rctx.ClusterComponent.Configs {
		if ptr.Deref(cfg.Name, "") == rctx.ConfigTemplate.Name {
			config = &rctx.ClusterComponent.Configs[i]
			break
		}
	}
	if config == nil {
		return makeReturnedStatus(ESFailedAndRetry), fmt.Errorf("config %s not found", rctx.ConfigTemplate.Name)
	}
	if config.VersionHash != rctx.getTargetVersionHash() {
		return o.update(rctx, config, parameters), nil
	}
	return syncLatestConfigStatus(rctx), nil
}

func (o *syncPolicy) update(rctx reconfigureContext, config *apisappsv1.ClusterComponentConfig, parameters map[string]string) returnedStatus {
	var (
		replicas = rctx.getTargetReplicas()
		// fileName string
	)
	// if rctx.ConfigDescription != nil {
	//	fileName = rctx.ConfigDescription.Name
	// }

	// TODO: config file?
	config.Variables = parameters // TODO: variables vs parameters?
	config.VersionHash = rctx.getTargetVersionHash()

	return makeReturnedStatus(ESRetry, withExpected(replicas), withSucceed(0))
}

func syncLatestConfigStatus(rctx reconfigureContext) returnedStatus {
	var (
		replicas    = rctx.getTargetReplicas()
		versionHash = rctx.getTargetVersionHash()
	)
	updated := int32(0)
	for _, inst := range rctx.its.Status.InstanceStatus {
		idx := slices.IndexFunc(inst.Configs, func(cfg workloads.InstanceConfigStatus) bool {
			return cfg.Name == rctx.ConfigTemplate.Name
		})
		if idx >= 0 && inst.Configs[idx].VersionHash == versionHash {
			updated++
		}
	}
	if updated == replicas {
		return makeReturnedStatus(ESNone, withExpected(replicas), withSucceed(updated))
	}
	return makeReturnedStatus(ESRetry, withExpected(replicas), withSucceed(updated))
}
