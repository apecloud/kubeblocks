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
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

func init() {
	registerPolicy(parametersv1alpha1.RestartPolicy, restartPolicyInstance)
}

var restartPolicyInstance = &restartPolicy{}

type restartPolicy struct{}

func (s *restartPolicy) Upgrade(rctx reconfigureContext) (returnedStatus, error) {
	rctx.Log.V(1).Info("simple policy begin....")

	s.restart(rctx)

	return syncLatestConfigStatus(rctx), nil
}

func (s *restartPolicy) restart(rctx reconfigureContext) {
	var (
		configKey        = rctx.generateConfigIdentifier()
		newVersion       = rctx.getTargetVersionHash()
		cfgAnnotationKey = core.GenerateUniqKeyWithConfig(constant.UpgradeRestartAnnotationKey, configKey)
	)
	if rctx.ClusterComponent.Annotations == nil {
		rctx.ClusterComponent.Annotations = map[string]string{}
	}
	if rctx.ClusterComponent.Annotations[cfgAnnotationKey] != newVersion {
		rctx.ClusterComponent.Annotations[cfgAnnotationKey] = newVersion
	}
}
