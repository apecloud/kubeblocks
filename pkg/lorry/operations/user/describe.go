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

package user

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type DescribeUser struct {
	operations.Base
	dbManager engines.DBManager
	logger    logr.Logger
}

var describeUser operations.Operation = &DescribeUser{}

func init() {
	err := operations.Register("describeuser", describeUser)
	if err != nil {
		panic(err.Error())
	}
}

func (s *DescribeUser) Init(ctx context.Context) error {
	dbManager, err := register.GetDBManager()
	if err != nil {
		return errors.Wrap(err, "get manager failed")
	}
	s.dbManager = dbManager
	s.logger = ctrl.Log.WithName("describeUser")
	return nil
}

func (s *DescribeUser) IsReadonly(ctx context.Context) bool {
	return true
}

func (s *DescribeUser) PreCheck(ctx context.Context, req *operations.OpsRequest) error {
	userInfo, err := UserInfoParser(req)
	if err != nil {
		return err
	}

	return userInfo.UserNameValidator()
}

func (s *DescribeUser) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	userInfo, _ := UserInfoParser(req)
	resp := operations.NewOpsResponse(util.DescribeUserOp)

	result, err := s.dbManager.DescribeUser(ctx, userInfo.UserName)
	if err != nil {
		s.logger.Info("executing describeUser error", "error", err)
		return resp, err
	}

	resp.Data["user"] = result
	return resp.WithSuccess("")
}

func UserInfoParser(req *operations.OpsRequest) (*models.UserInfo, error) {
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
