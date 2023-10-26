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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type DeleteUser struct {
	operations.Base
	dbManager engines.DBManager
	logger    logr.Logger
}

var deleteUser operations.Operation = &DeleteUser{}

func init() {
	err := operations.Register("deleteuser", deleteUser)
	if err != nil {
		panic(err.Error())
	}
}

func (s *DeleteUser) Init(ctx context.Context) error {
	dbManager, err := register.GetDBManager()
	if err != nil {
		return errors.Wrap(err, "get manager failed")
	}
	s.dbManager = dbManager
	s.logger = ctrl.Log.WithName("DeleteUser")
	return nil
}

func (s *DeleteUser) IsReadonly(ctx context.Context) bool {
	return false
}

func (s *DeleteUser) PreCheck(ctx context.Context, req *operations.OpsRequest) error {
	userInfo, err := UserInfoParser(req)
	if err != nil {
		return err
	}

	return userInfo.UserNameValidator()
}

func (s *DeleteUser) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	userInfo, _ := UserInfoParser(req)
	resp := operations.NewOpsResponse(util.DeleteUserOp)

	err := s.dbManager.DeleteUser(ctx, userInfo.UserName)
	if err != nil {
		s.logger.Info("executing DeleteUser error", "error", err)
		return resp, err
	}

	return resp.WithSuccess("")
}
