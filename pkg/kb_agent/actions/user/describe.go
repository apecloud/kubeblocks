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
	"encoding/json"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/actions"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers/models"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

type DescribeUser struct {
	actions.Base
}

var describeUser actions.Action = &DescribeUser{}

func init() {
	err := actions.Register("describeuser", describeUser)
	if err != nil {
		panic(err.Error())
	}
}

func (s *DescribeUser) Init(ctx context.Context) error {
	s.Logger = ctrl.Log.WithName("describeUser")
	s.Action = constant.DescribeUserAction
	return s.Base.Init(ctx)
}

func (s *DescribeUser) IsReadonly(ctx context.Context) bool {
	return true
}

func (s *DescribeUser) PreCheck(ctx context.Context, req *actions.ActionRequest) error {
	userInfo, err := UserInfoParser(req)
	if err != nil {
		return err
	}

	return userInfo.UserNameValidator()
}

func (s *DescribeUser) Do(ctx context.Context, req *actions.ActionRequest) (*actions.ActionResponse, error) {
	userInfo, _ := UserInfoParser(req)
	resp := actions.NewOpsResponse(util.DescribeUserOp)

	result, err := s.Handler.DescribeUser(ctx, userInfo.UserName)
	if err != nil {
		s.Logger.Info("executing describeUser error", "error", err)
		return resp, err
	}

	resp.Data["user"] = result
	return resp.WithSuccess("")
}

func UserInfoParser(req *actions.ActionRequest) (*models.UserInfo, error) {
	user := &models.UserInfo{}
	if req == nil || req.Parameters == nil {
		return nil, fmt.Errorf("no Parameters provided")
	} else if jsonData, err := json.Marshal(req.Parameters); err != nil {
		return nil, err
	} else if err = json.Unmarshal(jsonData, user); err != nil {
		return nil, err
	}
	return user, nil
}
