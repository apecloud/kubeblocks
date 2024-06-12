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

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/actions"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

type ListSystemAccounts struct {
	actions.Base
}

var listSystemAccounts actions.Action = &ListSystemAccounts{}

func init() {
	err := actions.Register("listsystemaccounts", listSystemAccounts)
	if err != nil {
		panic(err.Error())
	}
}

func (s *ListSystemAccounts) Init(ctx context.Context) error {
	s.Logger = ctrl.Log.WithName("listSystemAccounts")
	s.Action = constant.ListSystemAccountsAction
	return s.Base.Init(ctx)
}

func (s *ListSystemAccounts) IsReadonly(ctx context.Context) bool {
	return true
}

func (s *ListSystemAccounts) Do(ctx context.Context, req *actions.ActionRequest) (*actions.ActionResponse, error) {
	resp := actions.NewOpsResponse(util.ListSystemAccountsOp)

	result, err := s.Handler.ListSystemAccounts(ctx)
	if err != nil {
		s.Logger.Info("executing ListSystemAccounts error", "error", err)
		return resp, err
	}

	resp.Data["systemAccounts"] = result
	return resp.WithSuccess("")
}
