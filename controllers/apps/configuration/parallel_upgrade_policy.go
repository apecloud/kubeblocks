/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package configuration

import (
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	podutil "github.com/apecloud/kubeblocks/internal/controllerutil"
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
		return makeReturnedStatus(ESNotSupport), cfgcore.MakeError("not support component workload type[%s]", params.WorkloadType())
	case appsv1alpha1.Consensus:
		funcs = GetConsensusRollingUpgradeFuncs()
	case appsv1alpha1.Stateful:
		funcs = GetStatefulSetRollingUpgradeFuncs()
	case appsv1alpha1.Replication:
		funcs = GetReplicationRollingUpgradeFuncs()
	}

	pods, err := funcs.GetPodsFunc(params)
	if err != nil {
		return makeReturnedStatus(ESAndRetryFailed), err
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
			return makeReturnedStatus(ESAndRetryFailed), err
		}
		if err := updatePodLabelsWithConfigVersion(&pod, configKey, configVersion, params.Client, params.Ctx.Ctx); err != nil {
			return makeReturnedStatus(ESAndRetryFailed), err
		}
	}
	return makeReturnedStatus(ESNone), nil
}
