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

type GetLag struct {
	actions.Base
	dcsStore dcs.DCS
}

var getlag actions.Action = &GetLag{}

func init() {
	err := actions.Register("getlag", getlag)
	if err != nil {
		panic(err.Error())
	}
}

func (s *GetLag) Init(ctx context.Context) error {
	s.dcsStore = dcs.GetStore()
	if s.dcsStore == nil {
		return errors.New("dcs store init failed")
	}

	s.Logger = ctrl.Log.WithName("GetLag")
	s.Action = constant.GetLagAction
	return s.Base.Init(ctx)
}

func (s *GetLag) IsReadonly(context.Context) bool {
	return true
}

func (s *GetLag) Do(ctx context.Context, req *actions.OpsRequest) (*actions.OpsResponse, error) {
	sql := req.GetString("sql")
	if sql == "" {
		return nil, errors.New("no sql provided")
	}

	resp := &actions.OpsResponse{
		Data: map[string]any{},
	}
	resp.Data["operation"] = util.ExecOperation
	k8sStore := s.dcsStore.(*dcs.KubernetesStore)
	cluster := k8sStore.GetClusterFromCache()

	lag, err := s.Handler.GetLag(ctx, cluster)
	if err != nil {
		s.Logger.Info("executing getlag error", "error", err)
		return resp, err
	}

	resp.Data["lag"] = lag
	return resp, err
}
