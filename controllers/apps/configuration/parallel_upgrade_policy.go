/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package configuration

import (
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	podutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type parallelUpgradePolicy struct {
}

func init() {
	RegisterPolicy(appsv1alpha1.RestartPolicy, &parallelUpgradePolicy{})
}

func (p *parallelUpgradePolicy) Upgrade(params reconfigureParams) (ReturnedStatus, error) {
	var funcs RollingUpgradeFuncs

	switch params.WorkloadType() {
	default:
		return makeReturnedStatus(ESNotSupport), cfgcore.MakeError("not supported component workload type[%s]", params.WorkloadType())
	case appsv1alpha1.Consensus:
		funcs = GetConsensusRollingUpgradeFuncs()
	case appsv1alpha1.Stateful:
		funcs = GetStatefulSetRollingUpgradeFuncs()
	case appsv1alpha1.Replication:
		funcs = GetReplicationRollingUpgradeFuncs()
	}

	pods, err := funcs.GetPodsFunc(params)
	if err != nil {
		return makeReturnedStatus(ESFailedAndRetry), err
	}

	return p.restartPods(params, pods, funcs)
}

func (p *parallelUpgradePolicy) GetPolicyName() string {
	return string(appsv1alpha1.RestartPolicy)
}

func (p *parallelUpgradePolicy) restartPods(params reconfigureParams, pods []corev1.Pod, funcs RollingUpgradeFuncs) (ReturnedStatus, error) {
	var configKey = params.getConfigKey()
	var configVersion = params.getTargetVersionHash()

	for _, pod := range pods {
		if podutil.IsMatchConfigVersion(&pod, configKey, configVersion) {
			continue
		}
		if err := funcs.RestartContainerFunc(&pod, params.Ctx.Ctx, params.ContainerNames, params.ReconfigureClientFactory); err != nil {
			return makeReturnedStatus(ESFailedAndRetry), err
		}
		if err := updatePodLabelsWithConfigVersion(&pod, configKey, configVersion, params.Client, params.Ctx.Ctx); err != nil {
			return makeReturnedStatus(ESFailedAndRetry), err
		}
	}
	return makeReturnedStatus(ESNone), nil
}
