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

import cfgcore "github.com/apecloud/kubeblocks/internal/configuration"

type ParallelUpgradePolicy struct {
}

func (p *ParallelUpgradePolicy) Upgrade(params ReconfigureParams) (ExecStatus, error) {
	if err := p.restartPods(params); err != nil {
		return ES_Failed, err
	}

	return ES_None, nil
}

func (p *ParallelUpgradePolicy) restartPods(params ReconfigureParams) error {
	return cfgcore.MakeError("")
}
