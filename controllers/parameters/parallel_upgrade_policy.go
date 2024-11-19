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
	corev1 "k8s.io/api/core/v1"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	podutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

var parallelUpgradePolicyInstance = &parallelUpgradePolicy{}

type parallelUpgradePolicy struct{}

func init() {
	RegisterPolicy(parametersv1alpha1.RestartPolicy, parallelUpgradePolicyInstance)
}

func (p *parallelUpgradePolicy) Upgrade(rctx reconfigureContext) (ReturnedStatus, error) {
	funcs := GetInstanceSetRollingUpgradeFuncs()
	pods, err := funcs.GetPodsFunc(rctx)
	if err != nil {
		return makeReturnedStatus(ESFailedAndRetry), err
	}

	return p.restartPods(rctx, pods, funcs)
}

func (p *parallelUpgradePolicy) GetPolicyName() string {
	return string(parametersv1alpha1.RestartPolicy)
}

func (p *parallelUpgradePolicy) restartPods(rctx reconfigureContext, pods []corev1.Pod, funcs RollingUpgradeFuncs) (ReturnedStatus, error) {
	var configKey = rctx.getConfigKey()
	var configVersion = rctx.getTargetVersionHash()

	for _, pod := range pods {
		if podutil.IsMatchConfigVersion(&pod, configKey, configVersion) {
			continue
		}
		if err := funcs.RestartContainerFunc(&pod, rctx.Ctx, rctx.ContainerNames, rctx.ReconfigureClientFactory); err != nil {
			return makeReturnedStatus(ESFailedAndRetry), err
		}
		if err := updatePodLabelsWithConfigVersion(&pod, configKey, configVersion, rctx.Client, rctx.Ctx); err != nil {
			return makeReturnedStatus(ESFailedAndRetry), err
		}
	}
	return makeReturnedStatus(ESNone), nil
}
