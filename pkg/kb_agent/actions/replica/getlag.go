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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type GetLag struct {
	operations.Base
	dcsStore  dcs.DCS
	dbManager engines.DBManager
	logger    logr.Logger
}

var getlag operations.Operation = &GetLag{}

func init() {
	err := operations.Register("getlag", getlag)
	if err != nil {
		panic(err.Error())
	}
}

func (s *GetLag) Init(context.Context) error {
	s.dcsStore = dcs.GetStore()
	if s.dcsStore == nil {
		return errors.New("dcs store init failed")
	}

	dbManager, err := register.GetDBManager(nil)
	if err != nil {
		return errors.Wrap(err, "get manager failed")
	}
	s.dbManager = dbManager
	s.logger = ctrl.Log.WithName("getlag")
	return nil
}

func (s *GetLag) IsReadonly(context.Context) bool {
	return false
}

func (s *GetLag) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	sql := req.GetString("sql")
	if sql == "" {
		return nil, errors.New("no sql provided")
	}

	resp := &operations.OpsResponse{
		Data: map[string]any{},
	}
	resp.Data["operation"] = util.ExecOperation
	k8sStore := s.dcsStore.(*dcs.KubernetesStore)
	cluster := k8sStore.GetClusterFromCache()

	lag, err := s.dbManager.GetLag(ctx, cluster)
	if err != nil {
		s.logger.Info("executing getlag error", "error", err)
		return resp, err
	}

	resp.Data["lag"] = lag
	return resp, err
}
