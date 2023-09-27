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

package foxlake

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/apecloud/kubeblocks/lorry/binding"
	"github.com/apecloud/kubeblocks/lorry/component"
	"github.com/apecloud/kubeblocks/lorry/component/foxlake"
	. "github.com/apecloud/kubeblocks/lorry/util"
	"github.com/go-logr/logr"
	"golang.org/x/exp/slices"
	ctrl "sigs.k8s.io/controller-runtime"
)

type FoxLakeOperations struct {
	BaseOperations
}

var _ BaseInternalOps = &FoxLakeOperations{}

const (
	superUserPriv = "SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, RELOAD, SHUTDOWN, PROCESS, FILE, REFERENCES, INDEX, ALTER, SHOW DATABASES, SUPER, CREATE TEMPORARY TABLES, LOCK TABLES, EXECUTE, REPLICATION SLAVE, REPLICATION CLIENT, CREATE VIEW, SHOW VIEW, CREATE ROUTINE, ALTER ROUTINE, CREATE USER, EVENT, TRIGGER, CREATE TABLESPACE, CREATE ROLE, DROP ROLE ON *.*"
	readWritePriv = "SELECT, INSERT, UPDATE, DELETE ON *.*"
	readOnlyRPriv = "SELECT ON *.*"
	createUserTpl = "CREATE USER '%s'@'%%' IDENTIFIED BY '%s';"
	deleteUserTpl = "DROP USER IF EXISTS '%s'@'%%';"
	grantTpl      = "GRANT %s TO '%s'@'%%';"
	revokeTpl     = "REVOKE %s FROM '%s'@'%%';"
	listUserTpl   = "SELECT user AS userName, CASE password_expired WHEN 'N' THEN 'F' ELSE 'T' END as expired FROM mysql.user WHERE host = '%' and user <> 'root' and user not like 'kb%';"
	showGrantTpl  = "SHOW GRANTS FOR '%s'@'%%';"
	noPriv        = "USAGE ON *.*"
)

func NewFoxLake() *FoxLakeOperations {
	logger := ctrl.Log.WithName("FoxLake")
	return &FoxLakeOperations{BaseOperations: BaseOperations{Logger: logger}}
}

func (foxlakeOps *FoxLakeOperations) Init(properties component.Properties) error {
	foxlakeOps.Logger.Info("Initializing foxlake binding")
	foxlakeOps.BaseOperations.Init(properties)
	config, err := foxlake.NewConfig(properties)
	if err != nil {
		foxlakeOps.Logger.Error(err, "foxlake config initialize failed")
		return err
	}
	manager, err := foxlake.NewManager(foxlakeOps.Logger)
	if err != nil {
		foxlakeOps.Logger.Error(err, "foxlake manager initialize failed")
		return err
	}

	foxlakeOps.Manager = manager
	foxlakeOps.DBType = "foxlake"
	foxlakeOps.DBPort = config.GetDBPort()

	foxlakeOps.RegisterOperationOnDBReady(ListUsersOp, foxlakeOps.listUsersOps, manager)
	foxlakeOps.RegisterOperationOnDBReady(CreateUserOp, foxlakeOps.createUserOps, manager)
	foxlakeOps.RegisterOperationOnDBReady(DeleteUserOp, foxlakeOps.deleteUserOps, manager)
	foxlakeOps.RegisterOperationOnDBReady(DescribeUserOp, foxlakeOps.describeUserOps, manager)
	foxlakeOps.RegisterOperationOnDBReady(GrantUserRoleOp, foxlakeOps.grantUserRoleOps, manager)
	foxlakeOps.RegisterOperationOnDBReady(RevokeUserRoleOp, foxlakeOps.revokeUserRoleOps, manager)

	return nil
}

// InternalQuery is used for internal query, implements BaseInternalOps interface
func (foxlakeOps *FoxLakeOperations) InternalQuery(ctx context.Context, sql string) ([]byte, error) {
	manager := foxlakeOps.Manager.(*foxlake.Manager)
	return manager.Query(ctx, sql)
}

// InternalExec is used for internal execution, implements BaseInternalOps interface
func (foxlakeOps *FoxLakeOperations) InternalExec(ctx context.Context, sql string) (int64, error) {
	manager := foxlakeOps.Manager.(*foxlake.Manager)
	return manager.Exec(ctx, sql)
}

// GetLogger is used for getting logger, implements BaseInternalOps interface
func (foxlakeOps *FoxLakeOperations) GetLogger() logr.Logger {
	return foxlakeOps.Logger
}

// GetRunningPort implements BaseInternalOps interface
func (foxlakeOps *FoxLakeOperations) GetRunningPort() int {
	return 0
}

func (foxlakeOps *FoxLakeOperations) listUsersOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	sqlTplRend := func(user UserInfo) string {
		return listUserTpl
	}

	return QueryObject(ctx, foxlakeOps, req, ListUsersOp, sqlTplRend, nil, UserInfo{})
}

func (foxlakeOps *FoxLakeOperations) createUserOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	var (
		object = UserInfo{}

		sqlTplRend = func(user UserInfo) string {
			return fmt.Sprintf(createUserTpl, user.UserName, user.Password)
		}

		msgTplRend = func(user UserInfo) string {
			return fmt.Sprintf("created user: %s, with password: %s", user.UserName, user.Password)
		}
	)

	if err := ParseObjFromRequest(req, DefaultUserInfoParser, UserNameAndPasswdValidator, &object); err != nil {
		result := OpsResult{}
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}

	return ExecuteObject(ctx, foxlakeOps, req, CreateUserOp, sqlTplRend, msgTplRend, object)
}

func (foxlakeOps *FoxLakeOperations) deleteUserOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	var (
		object  = UserInfo{}
		validFn = func(user UserInfo) error {
			if len(user.UserName) == 0 {
				return ErrNoUserName
			}
			return nil
		}
		sqlTplRend = func(user UserInfo) string {
			return fmt.Sprintf(deleteUserTpl, user.UserName)
		}
		msgTplRend = func(user UserInfo) string {
			return fmt.Sprintf("deleted user: %s", user.UserName)
		}
	)
	if err := ParseObjFromRequest(req, DefaultUserInfoParser, validFn, &object); err != nil {
		result := OpsResult{}
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}

	return ExecuteObject(ctx, foxlakeOps, req, DeleteUserOp, sqlTplRend, msgTplRend, object)
}

func (foxlakeOps *FoxLakeOperations) grantUserRoleOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	var (
		succMsgTpl = "role %s granted to user: %s"
	)
	return foxlakeOps.managePrivillege(ctx, req, GrantUserRoleOp, grantTpl, succMsgTpl)
}

func (foxlakeOps *FoxLakeOperations) revokeUserRoleOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	var (
		succMsgTpl = "role %s revoked from user: %s"
	)
	return foxlakeOps.managePrivillege(ctx, req, RevokeUserRoleOp, revokeTpl, succMsgTpl)
}

func (foxlakeOps *FoxLakeOperations) managePrivillege(ctx context.Context, req *ProbeRequest, op OperationKind, sqlTpl string, succMsgTpl string) (OpsResult, error) {
	var (
		object     = UserInfo{}
		sqlTplRend = func(user UserInfo) string {
			// render sql stmts
			roleDesc, _ := foxlakeOps.role2Priv(user.RoleName)
			// update privilege
			sql := fmt.Sprintf(sqlTpl, roleDesc, user.UserName)
			return sql
		}
		msgTplRend = func(user UserInfo) string {
			return fmt.Sprintf(succMsgTpl, user.RoleName, user.UserName)
		}
	)
	if err := ParseObjFromRequest(req, DefaultUserInfoParser, UserNameAndRoleValidator, &object); err != nil {
		result := OpsResult{}
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}
	return ExecuteObject(ctx, foxlakeOps, req, op, sqlTplRend, msgTplRend, object)
}

func (foxlakeOps *FoxLakeOperations) describeUserOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	var (
		object = UserInfo{}

		// get user grants
		sqlTplRend = func(user UserInfo) string {
			return fmt.Sprintf(showGrantTpl, user.UserName)
		}

		dataProcessor = func(data interface{}) (interface{}, error) {
			roles := make([]map[string]string, 0)
			err := json.Unmarshal(data.([]byte), &roles)
			if err != nil {
				return nil, err
			}
			user := UserInfo{}
			// only keep one role name of the highest privilege
			userRoles := make([]RoleType, 0)
			for _, roleMap := range roles {
				for k, v := range roleMap {
					if len(user.UserName) == 0 {
						user.UserName = strings.TrimPrefix(strings.TrimSuffix(k, "@%"), "Grants for ")
					}
					mysqlRoleType := foxlakeOps.priv2Role(strings.TrimPrefix(v, "GRANT "))
					userRoles = append(userRoles, mysqlRoleType)
				}
			}
			// sort roles by weight
			slices.SortFunc(userRoles, SortRoleByWeight)
			if len(userRoles) > 0 {
				user.RoleName = (string)(userRoles[0])
			}
			if jsonData, err := json.Marshal([]UserInfo{user}); err != nil {
				return nil, err
			} else {
				return string(jsonData), nil
			}
		}
	)

	if err := ParseObjFromRequest(req, DefaultUserInfoParser, UserNameValidator, &object); err != nil {
		result := OpsResult{}
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}

	return QueryObject(ctx, foxlakeOps, req, DescribeUserOp, sqlTplRend, dataProcessor, object)
}

func (foxlakeOps *FoxLakeOperations) role2Priv(roleName string) (string, error) {
	roleType := String2RoleType(roleName)
	switch roleType {
	case SuperUserRole:
		return superUserPriv, nil
	case ReadWriteRole:
		return readWritePriv, nil
	case ReadOnlyRole:
		return readOnlyRPriv, nil
	}
	return "", fmt.Errorf("role name: %s is not supported", roleName)
}
func (foxlakeOps *FoxLakeOperations) priv2Role(priv string) RoleType {
	if strings.HasPrefix(priv, readOnlyRPriv) {
		return ReadOnlyRole
	}
	if strings.HasPrefix(priv, readWritePriv) {
		return ReadWriteRole
	}
	if strings.HasPrefix(priv, superUserPriv) {
		return SuperUserRole
	}
	if strings.HasPrefix(priv, noPriv) {
		return NoPrivileges
	}
	return CustomizedRole
}
