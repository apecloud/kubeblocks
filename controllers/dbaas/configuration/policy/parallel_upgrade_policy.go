/*
Copyright ApeCloud Inc.

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

package policy

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

func init() {
	RegisterPolicy(dbaasv1alpha1.RestartPolicy, &ParallelUpgradePolicy{})
}

type ParallelUpgradePolicy struct {
}

func (p *ParallelUpgradePolicy) Upgrade(params ReconfigureParams) (ExecStatus, error) {
	if finished, err := p.restartPods(params); err != nil {
		return ESAndRetryFailed, err
	} else if !finished {
		return ESRetry, nil
	}

	return ESNone, nil
}

func (p *ParallelUpgradePolicy) GetPolicyName() string {
	return string(dbaasv1alpha1.RestartPolicy)
}

func (p *ParallelUpgradePolicy) restartPods(params ReconfigureParams) (bool, error) {
	var (
		funcs         RollingUpgradeFuncs
		cType         = params.ComponentType()
		configKey     = params.GetConfigKey()
		configVersion = params.GetModifyVersion()
	)

	updatePodLabelsVersion := func(pod *corev1.Pod, labelKey, labelValue string) error {
		patch := client.MergeFrom(pod.DeepCopy())
		if pod.Labels == nil {
			pod.Labels = make(map[string]string, 1)
		}
		pod.Labels[labelKey] = labelValue
		return params.Client.Patch(params.Ctx.Ctx, pod, patch)
	}

	switch cType {
	case dbaasv1alpha1.Consensus:
		funcs = GetConsensusRollingUpgradeFuncs()
	case dbaasv1alpha1.Stateful:
		funcs = GetStatefulSetRollingUpgradeFuncs()
	default:
		return false, cfgcore.MakeError("not support component type[%s]", cType)
	}

	pods, err := funcs.GetPodsFunc(params)
	if err != nil {
		return false, err
	}

	for _, pod := range pods {
		if err := funcs.RestartContainerFunc(&pod, params.ContainerNames, params.ReconfigureClientFactory); err != nil {
			return false, err
		}
		if err := updatePodLabelsVersion(&pod, configKey, configVersion); err != nil {
			return false, err
		}
	}
	return true, nil
}
