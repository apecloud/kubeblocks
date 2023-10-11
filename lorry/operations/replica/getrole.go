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
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/lorry/engines/register"
	"github.com/apecloud/kubeblocks/lorry/operations"
	"github.com/apecloud/kubeblocks/lorry/util"
)

type GetRole struct {
	operations.Base
	logger  logr.Logger
	Timeout time.Duration
}

var getrole operations.Operation = &GetRole{}

func init() {
	err := operations.Register("getrole", getrole)
	if err != nil {
		panic(err.Error())
	}
}

func (s *GetRole) Init(ctx context.Context) error {
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

	role, err := manager.GetReplicaRole(ctx)
	if err != nil {
		s.logger.Error(err, "executing getrole error")
		return resp, err
	}

	resp.Data["role"] = role
	return resp, err
}
