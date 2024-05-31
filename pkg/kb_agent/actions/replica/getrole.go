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

package replica

import (
	"context"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/actions"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/dcs"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

type GetRole struct {
	actions.Base
	dcsStore dcs.DCS
}

var getrole actions.Action = &GetRole{}

func init() {
	err := actions.Register("getrole", getrole)
	if err != nil {
		panic(err.Error())
	}
}

func (s *GetRole) Init(ctx context.Context) error {
	s.dcsStore = dcs.GetStore()
	if s.dcsStore == nil {
		return errors.New("dcs store init failed")
	}

	s.Logger = ctrl.Log.WithName("GetRole")
	s.Action = constant.RoleProbeAction
	return s.Base.Init(ctx)
}

func (s *GetRole) IsReadonly(ctx context.Context) bool {
	return true
}

func (s *GetRole) Do(ctx context.Context, req *actions.OpsRequest) (*actions.OpsResponse, error) {
	resp := &actions.OpsResponse{
		Data: map[string]any{},
	}
	resp.Data["operation"] = util.GetRoleOperation
	cluster := s.dcsStore.GetClusterFromCache()
	role, err := s.Handler.GetReplicaRole(ctx, cluster)

	if err != nil {
		s.Logger.Info("executing getrole error", "error", err.Error())
		return resp, err
	}

	resp.Data["role"] = role
	return resp, err
}
