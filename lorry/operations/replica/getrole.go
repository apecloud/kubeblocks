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

package replica

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/lorry/engines/register"
	"github.com/apecloud/kubeblocks/lorry/operations"
	"github.com/apecloud/kubeblocks/lorry/util"
)

type GetRole struct {
	operations.Base
	logger       logr.Logger
	ProbeTimeout time.Duration
}

var getrole operations.Operation = &GetRole{}

func init() {
	err := operations.Register("getrole", getrole)
	if err != nil {
		panic(err.Error())
	}
}

func (s *GetRole) Init(ctx context.Context) error {
	timeoutSeconds := defaultRoleProbeTimeoutSeconds
	if viper.IsSet(constant.KBEnvRoleProbeTimeout) {
		timeoutSeconds = viper.GetInt(constant.KBEnvRoleProbeTimeout)
	}
	// lorry utilizes the pod readiness probe to trigger role probe and 'timeoutSeconds' is directly copied from the 'probe.timeoutSeconds' field of pod.
	// here we give 80% of the total time to role probe job and leave the remaining 20% to kubelet to handle the readiness probe related tasks.
	s.ProbeTimeout = time.Duration(timeoutSeconds) * (800 * time.Millisecond)
	s.logger = ctrl.Log.WithName("getrole")
	return nil
}

func (s *GetRole) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	manager, err := register.GetOrCreateManager()
	if err != nil {
		return nil, errors.Wrap(err, "get manager failed")
	}

	resp := &operations.OpsResponse{}
	resp.Data["operation"] = util.GetRoleOperation

	ctx1, cancel := context.WithTimeout(ctx, s.ProbeTimeout)
	defer cancel()
	role, err := manager.GetReplicaRole(ctx1)
	if err != nil {
		s.logger.Error(err, "executing getrole error")
		return resp, err
	}

	resp.Data["role"] = role
	return resp, err
}
