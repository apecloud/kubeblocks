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
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

func init() {
	RegisterPolicy(dbaasv1alpha1.RestartPolicy, &ParallelUpgradePolicy{})
}

type ParallelUpgradePolicy struct {
}

func (p *ParallelUpgradePolicy) Upgrade(params ReconfigureParams) (ExecStatus, error) {
	finished, err := p.restartPods(params)
	if err != nil {
		return ES_Failed, err
	}

	if finished {
		return ES_None, nil
	}
	return ES_Retry, nil
}

func (p *ParallelUpgradePolicy) GetPolicyName() string {
	return string(dbaasv1alpha1.RestartPolicy)
}

func (p *ParallelUpgradePolicy) restartPods(params ReconfigureParams) (bool, error) {
	// TODO(zt) kill program
	return false, cfgcore.MakeError("")
}
