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
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
)

func init() {
	registerPolicy(parametersv1alpha1.DynamicReloadAndRestartPolicy, combinedPolicyInst)
}

var combinedPolicyInst = &combinedPolicy{
	policies: []reconfigurePolicy{
		syncPolicyInst,
		restartPolicyInst,
	},
}

type combinedPolicy struct {
	policies []reconfigurePolicy
}

func (h *combinedPolicy) Upgrade(rctx reconfigureContext) (returnedStatus, error) {
	var (
		status returnedStatus
		err    error
	)
	for _, policy := range h.policies {
		status, err = policy.Upgrade(rctx)
		if err != nil {
			return status, err
		}
	}
	// TODO: how to merge the status?
	return status, nil
}
