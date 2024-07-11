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
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type CreateUser struct {
	operations.Base
	dbManager engines.DBManager
	logger    logr.Logger
}

var createUser operations.Operation = &CreateUser{}

func init() {
	err := operations.Register(strings.ToLower(string(util.CreateUserOp)), createUser)
	if err != nil {
		panic(err.Error())
	}
}

func (s *CreateUser) Init(ctx context.Context) error {
	s.logger = ctrl.Log.WithName("CreateUser")

	actionJSON := viper.GetString(constant.KBEnvActionCommands)
	if actionJSON != "" {
		actionCommands := map[string][]string{}
		err := json.Unmarshal([]byte(actionJSON), &actionCommands)
		if err != nil {
			s.logger.Info("get action commands failed", "error", err.Error())
			return err
		}
		accoutProvisionCmd, ok := actionCommands[constant.AccountProvisionAction]
		if ok && len(accoutProvisionCmd) > 0 {
			s.Command = accoutProvisionCmd
		}
	}
	dbManager, err := register.GetDBManager(s.Command)
	if err != nil {
		return errors.Wrap(err, "get manager failed")
	}
	s.dbManager = dbManager
	return nil
}

func (s *CreateUser) IsReadonly(ctx context.Context) bool {
	return false
}

func (s *CreateUser) PreCheck(ctx context.Context, req *operations.OpsRequest) error {
	userInfo, err := UserInfoParser(req)
	if err != nil {
		return err
	}

	return userInfo.UserNameAndPasswdValidator()
}

func (s *CreateUser) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	userInfo, _ := UserInfoParser(req)
	resp := operations.NewOpsResponse(util.CreateUserOp)

	user, err := s.dbManager.DescribeUser(ctx, userInfo.UserName)
	if err == nil && user != nil {
		return resp.WithSuccess("account already exists")
	}

	// for compatibility with old addons that specify accoutprovision action but not work actually.
	err = s.dbManager.CreateUser(ctx, userInfo.UserName, userInfo.Password, userInfo.Statement)
	if err != nil {
		err = errors.Cause(err)
		s.logger.Info("executing CreateUser error", "error", err.Error())
		return resp, err
	}

	if userInfo.RoleName != "" {
		err := s.dbManager.GrantUserRole(ctx, userInfo.UserName, userInfo.RoleName)
		if err != nil && err != models.ErrNotImplemented {
			s.logger.Info("executing grantRole error", "error", err.Error())
			return resp, err
		}
	}

	return resp.WithSuccess("")
}
