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

import appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"

type combineUpgradePolicy struct {
	policyExecutors []reconfigurePolicy
}

func init() {
	RegisterPolicy(appsv1alpha1.DynamicReloadAndRestartPolicy, &combineUpgradePolicy{
		policyExecutors: []reconfigurePolicy{&syncPolicy{}, &simplePolicy{}},
	})
}

func (h *combineUpgradePolicy) GetPolicyName() string {
	return string(appsv1alpha1.DynamicReloadAndRestartPolicy)
}

func (h *combineUpgradePolicy) Upgrade(params reconfigureParams) (ReturnedStatus, error) {
	var ret ReturnedStatus
	for _, executor := range h.policyExecutors {
		retStatus, err := executor.Upgrade(params)
		if err != nil {
			return retStatus, err
		}
		ret = retStatus
	}
	return ret, nil
}
