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
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/actions"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

type CreateUser struct {
	actions.Base
}

var createUser actions.Action = &CreateUser{}

func init() {
	err := actions.Register(strings.ToLower(string(util.CreateUserOp)), createUser)
	if err != nil {
		panic(err.Error())
	}
}

func (s *CreateUser) Init(ctx context.Context) error {
	s.Logger = ctrl.Log.WithName("CreateUser")
	s.Action = constant.CreateUserAction
	return s.Base.Init(ctx)
}

func (s *CreateUser) PreCheck(ctx context.Context, req *actions.OpsRequest) error {
	userInfo, err := UserInfoParser(req)
	if err != nil {
		return err
	}

	return userInfo.UserNameAndPasswdValidator()
}

func (s *CreateUser) Do(ctx context.Context, req *actions.OpsRequest) (*actions.OpsResponse, error) {
	userInfo, _ := UserInfoParser(req)
	resp := actions.NewOpsResponse(util.CreateUserOp)

	err := s.Handler.CreateUser(ctx, userInfo.UserName, userInfo.Password)
	if err != nil {
		s.Logger.Info("executing CreateUser error", "error", err)
		return resp, err
	}

	if userInfo.RoleName != "" {
		err := s.Handler.GrantUserRole(ctx, userInfo.UserName, userInfo.RoleName)
		if err != nil {
			s.Logger.Info("executing grantRole error", "error", err)
			return resp, err
		}
	}

	return resp.WithSuccess("")
}
