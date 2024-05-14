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

package user

import (
	"context"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type ListSystemAccounts struct {
	operations.Base
}

var listSystemAccounts operations.Operation = &ListSystemAccounts{}

func init() {
	err := operations.Register("listsystemaccounts", listSystemAccounts)
	if err != nil {
		panic(err.Error())
	}
}

func (s *ListSystemAccounts) Init(ctx context.Context) error {
	s.Logger = ctrl.Log.WithName("listSystemAccounts")
	return nil
}

func (s *ListSystemAccounts) IsReadonly(ctx context.Context) bool {
	return true
}

func (s *ListSystemAccounts) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	resp := operations.NewOpsResponse(util.ListSystemAccountsOp)

	dbManager, err := s.GetDBManager()
	if err != nil {
		return resp, errors.Wrap(err, "get manager failed")
	}

	result, err := dbManager.ListSystemAccounts(ctx)
	if err != nil {
		s.Logger.Info("executing ListSystemAccounts error", "error", err)
		return resp, err
	}

	resp.Data["systemAccounts"] = result
	return resp.WithSuccess("")
}
