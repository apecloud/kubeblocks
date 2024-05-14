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

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type GrantRole struct {
	operations.Base
}

var grantRole operations.Operation = &GrantRole{}

func init() {
	err := operations.Register(strings.ToLower(string(util.GrantUserRoleOp)), grantRole)
	if err != nil {
		panic(err.Error())
	}
}

func (s *GrantRole) Init(ctx context.Context) error {
	s.Logger = ctrl.Log.WithName("grantRole")
	return nil
}

func (s *GrantRole) IsReadonly(ctx context.Context) bool {
	return false
}

func (s *GrantRole) PreCheck(ctx context.Context, req *operations.OpsRequest) error {
	userInfo, err := UserInfoParser(req)
	if err != nil {
		return err
	}

	return userInfo.UserNameValidator()
}

func (s *GrantRole) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	userInfo, _ := UserInfoParser(req)
	resp := operations.NewOpsResponse(util.GrantUserRoleOp)

	dbManager, err := s.GetDBManager()
	if err != nil {
		return resp, errors.Wrap(err, "get manager failed")
	}

	err = dbManager.GrantUserRole(ctx, userInfo.UserName, userInfo.RoleName)
	if err != nil {
		s.Logger.Info("executing grantRole error", "error", err)
		return resp, err
	}

	return resp.WithSuccess("")
}
