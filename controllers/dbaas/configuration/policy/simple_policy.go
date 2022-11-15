/*
Copyright 2022.

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
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	dbaascfg "github.com/apecloud/kubeblocks/controllers/dbaas/configuration"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

func init() {
	RegisterPolicy(dbaasv1alpha1.NormalPolicy, &SimplePolicy{})
}

type SimplePolicy struct {
}

func (s *SimplePolicy) Upgrade(params ReconfigureParams) (ExecStatus, error) {
	params.Ctx.Log.V(1).Info("simple policy begin....")

	switch params.ComponentType() {
	case dbaasv1alpha1.Stateful, dbaasv1alpha1.Consensus:
		return rollingStatefulSets(params)
		// process consensus
	default:
		return ES_NotSpport, cfgcore.MakeError("not support component type:[%s]", params.ComponentType())
	}
}

func (s *SimplePolicy) GetPolicyName() string {
	return string(dbaasv1alpha1.NormalPolicy)
}

func rollingStatefulSets(param ReconfigureParams) (ExecStatus, error) {
	var (
		units      = param.ComponentUnits
		client     = param.Client
		newVersion = param.GetModifyVersion()
		configKey  = param.GetConfigKey()
	)

	if configKey == "" {
		return ES_Failed, cfgcore.MakeError("failed to found config meta. configmap : %s", param.TplName)
	}

	for _, sts := range units {
		if err := dbaascfg.RestartStsWithRolling(client, param.Ctx, sts, configKey, newVersion); err != nil {
			param.Ctx.Log.Error(err, "failed to restart statefulSet.", "stsName", sts.GetName())
			return ES_Retry, nil
		}
	}
	return ES_None, nil
}
