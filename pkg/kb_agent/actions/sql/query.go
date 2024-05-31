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

package sql

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type Query struct {
	operations.Base
	dbManager engines.DBManager
	logger    logr.Logger
}

var query operations.Operation = &Query{}

func init() {
	err := operations.Register("query", query)
	if err != nil {
		panic(err.Error())
	}
}

func (s *Query) Init(context.Context) error {
	dbManager, err := register.GetDBManager(nil)
	if err != nil {
		return errors.Wrap(err, "get manager failed")
	}
	s.dbManager = dbManager
	s.logger = ctrl.Log.WithName("query")
	return nil
}

func (s *Query) IsReadonly(context.Context) bool {
	return true
}

func (s *Query) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	sql := req.GetString("sql")
	if sql == "" {
		return nil, errors.New("no sql provided")
	}

	resp := operations.NewOpsResponse(util.QueryOperation)

	result, err := s.dbManager.Query(ctx, sql)
	if err != nil {
		s.logger.Info("executing query error", "error", err)
		return resp, err
	}

	resp.Data["result"] = string(result)
	return resp.WithSuccess("")
}
