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
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/plugin"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type GetRole struct {
	operations.Base
	dcsStore dcs.DCS
}

var getrole operations.Operation = &GetRole{}

func init() {
	err := operations.Register("getrole", getrole)
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

func (s *GetRole) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	resp := &operations.OpsResponse{
		Data: map[string]any{},
	}
	resp.Data["operation"] = util.GetRoleOperation
	var role string
	var err error
	switch {
	case intctrlutil.IsNil(s.DBPluginClient):
		role, err = s.GetRoleThroughGRPC(ctx)
	default:
		dbManager, err1 := s.GetDBManager()
		if err1 != nil {
			return nil, errors.Wrap(err1, "get manager failed")
		}

		cluster := s.dcsStore.GetClusterFromCache()
		role, err = dbManager.GetReplicaRole(ctx, cluster)
	}

	if err != nil {
		s.Logger.Info("executing getrole error", "error", err.Error())
		return resp, err
	}

	resp.Data["role"] = role
	return resp, err
}

func (s *GetRole) GetRoleThroughGRPC(ctx context.Context) (string, error) {
	getRoleRequest := &plugin.GetRoleRequest{
		DbInfo: plugin.GetDBInfo(),
	}

	resp, err := s.DBPluginClient.GetRole(ctx, getRoleRequest)
	if err != nil {
		return "", err
	}

	return resp.Role, nil
}
