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

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"golang.org/x/exp/slices"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component/postgres"
	. "github.com/apecloud/kubeblocks/internal/sqlchannel/util"
)

// List of operations.
const (
	connectionURLKey = "url"
	commandSQLKey    = "sql"

	listUserTpl = `
	SELECT usename AS userName, valuntil <now() AS expired,  usesuper,
	ARRAY(SELECT
		case
			when b.rolname = 'pg_read_all_data' THEN 'readonly'
			when b.rolname = 'pg_write_all_data' THEN 'readwrite'
		else b.rolname
		end
	FROM pg_catalog.pg_auth_members m
	JOIN pg_catalog.pg_roles b ON (m.roleid = b.oid)
	WHERE m.member = usesysid ) as roles
	FROM pg_catalog.pg_user
	WHERE usename <> 'postgres' and usename  not like 'kb%'
	ORDER BY usename;
	`
	descUserTpl = `
	SELECT usename AS userName,  valuntil <now() AS expired, usesuper,
	ARRAY(SELECT
	 case
		 when b.rolname = 'pg_read_all_data' THEN 'readonly'
		 when b.rolname = 'pg_write_all_data' THEN 'readwrite'
	 else b.rolname
	 end
	FROM pg_catalog.pg_auth_members m
	JOIN pg_catalog.pg_roles b ON (m.roleid = b.oid)
	WHERE m.member = usesysid ) as roles
	FROM pg_user
	WHERE usename = '%s';
	`
	createUserTpl         = "CREATE USER %s WITH PASSWORD '%s';"
	dropUserTpl           = "DROP USER IF EXISTS %s;"
	grantTpl              = "GRANT %s TO %s;"
	revokeTpl             = "REVOKE %s FROM %s;"
	listSystemAccountsTpl = "SELECT rolname FROM pg_catalog.pg_roles WHERE pg_roles.rolname LIKE 'kb%'"
)

// PostgresOperations represents PostgreSQL output binding.
type PostgresOperations struct {
	manager *postgres.Manager
	BaseOperations
}

var _ BaseInternalOps = &PostgresOperations{}

// NewPostgres returns a new PostgreSQL output binding.
func NewPostgres(logger logger.Logger) bindings.OutputBinding {
	return &PostgresOperations{BaseOperations: BaseOperations{Logger: logger}}
}

// Init initializes the PostgreSql binding.
func (pgOps *PostgresOperations) Init(metadata bindings.Metadata) error {
	pgOps.Logger.Debug("Initializing Postgres binding")
	pgOps.BaseOperations.Init(metadata)
	config, err := postgres.NewConfig(metadata.Properties)
	if err != nil {
		pgOps.Logger.Errorf("new postgresql config failed, err:%v", err)
	}
	manager, err := postgres.NewManager(pgOps.Logger)
	if err != nil {
		pgOps.Logger.Errorf("new postgresql manager failed, err:%v", err)
	}

	pgOps.DBType = "postgresql"
	pgOps.manager = manager
	pgOps.DBPort = config.GetDBPort()
	pgOps.BaseOperations.GetRole = pgOps.GetRole
	pgOps.RegisterOperation(GetRoleOperation, pgOps.GetRoleOps)
	// pgOps.RegisterOperation(GetLagOperation, pgOps.GetLagOps)
	pgOps.RegisterOperationOnDBReady(CheckStatusOperation, pgOps.CheckStatusOps, manager)
	pgOps.RegisterOperationOnDBReady(ExecOperation, pgOps.ExecOps, manager)
	pgOps.RegisterOperationOnDBReady(QueryOperation, pgOps.QueryOps, manager)

	// following are ops for account management
	pgOps.RegisterOperationOnDBReady(ListUsersOp, pgOps.listUsersOps, manager)
	pgOps.RegisterOperationOnDBReady(CreateUserOp, pgOps.createUserOps, manager)
	pgOps.RegisterOperationOnDBReady(DeleteUserOp, pgOps.deleteUserOps, manager)
	pgOps.RegisterOperationOnDBReady(DescribeUserOp, pgOps.describeUserOps, manager)
	pgOps.RegisterOperationOnDBReady(GrantUserRoleOp, pgOps.grantUserRoleOps, manager)
	pgOps.RegisterOperationOnDBReady(RevokeUserRoleOp, pgOps.revokeUserRoleOps, manager)
	pgOps.RegisterOperationOnDBReady(ListSystemAccountsOp, pgOps.listSystemAccountsOps, manager)
	return nil
}

func (pgOps *PostgresOperations) GetRunningPort() int {
	return 0
}

func (pgOps *PostgresOperations) GetRole(ctx context.Context, request *bindings.InvokeRequest, response *bindings.InvokeResponse) (string, error) {
	return pgOps.manager.GetMemberStateWithPool(ctx, nil)
}

func (pgOps *PostgresOperations) ExecOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	result := OpsResult{}
	sql, ok := req.Metadata["sql"]
	if !ok || sql == "" {
		result["event"] = OperationFailed
		result["message"] = "no sql provided"
		return result, nil
	}
	count, err := pgOps.manager.Exec(ctx, sql)
	if err != nil {
		pgOps.Logger.Infof("exec error: %v", err)
		result["event"] = OperationFailed
		result["message"] = err.Error()
	} else {
		result["event"] = OperationSuccess
		result["count"] = count
	}
	return result, nil
}

func (pgOps *PostgresOperations) CheckStatusOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	rwSQL := fmt.Sprintf(`begin;
create table if not exists kb_health_check(type int, check_ts timestamp, primary key(type));
insert into kb_health_check values(%d, CURRENT_TIMESTAMP) on conflict(type) do update set check_ts = CURRENT_TIMESTAMP;
	commit;
	select check_ts from kb_health_check where type=%d limit 1;`, CheckStatusType, CheckStatusType)
	roSQL := fmt.Sprintf(`select check_ts from kb_health_check where type=%d limit 1;`, CheckStatusType)
	var err error
	var data []byte
	switch pgOps.OriRole {
	case PRIMARY:
		var count int64
		count, err = pgOps.manager.Exec(ctx, rwSQL)
		data = []byte(strconv.FormatInt(count, 10))
	case SECONDARY:
		data, err = pgOps.manager.Query(ctx, roSQL)
	default:
		msg := fmt.Sprintf("unknown role %s: %v", pgOps.OriRole, pgOps.DBRoles)
		pgOps.Logger.Info(msg)
		data = []byte(msg)
	}

	result := OpsResult{}
	if err != nil {
		pgOps.Logger.Infof("CheckStatus error: %v", err)
		result["event"] = OperationFailed
		result["message"] = err.Error()
		if pgOps.CheckStatusFailedCount%pgOps.FailedEventReportFrequency == 0 {
			pgOps.Logger.Infof("status checks failed %v times continuously", pgOps.CheckStatusFailedCount)
			resp.Metadata[StatusCode] = OperationFailedHTTPCode
		}
		pgOps.CheckStatusFailedCount++
	} else {
		result["event"] = OperationSuccess
		result["message"] = string(data)
		pgOps.CheckStatusFailedCount = 0
	}
	return result, nil
}

func (pgOps *PostgresOperations) QueryOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	result := OpsResult{}
	sql, ok := req.Metadata["sql"]
	if !ok || sql == "" {
		result["event"] = OperationFailed
		result["message"] = "no sql provided"
		return result, nil
	}
	data, err := pgOps.manager.Query(ctx, sql)
	if err != nil {
		pgOps.Logger.Infof("Query error: %v", err)
		result["event"] = OperationFailed
		result["message"] = err.Error()
	} else {
		result["event"] = OperationSuccess
		result["message"] = string(data)
	}
	return result, nil
}

// InternalQuery is used for internal query, implements BaseInternalOps interface
func (pgOps *PostgresOperations) InternalQuery(ctx context.Context, sql string) (result []byte, err error) {
	return pgOps.manager.Query(ctx, sql)
}

// InternalExec is used for internal execution, implements BaseInternalOps interface
func (pgOps *PostgresOperations) InternalExec(ctx context.Context, sql string) (result int64, err error) {
	return pgOps.manager.Exec(ctx, sql)
}

// GetLogger is used for getting logger, implements BaseInternalOps interface
func (pgOps *PostgresOperations) GetLogger() logger.Logger {
	return pgOps.Logger
}

func (pgOps *PostgresOperations) createUserOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		object  = UserInfo{}
		opsKind = CreateUserOp

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

	return ExecuteObject(ctx, pgOps, req, opsKind, sqlTplRend, msgTplRend, object)
}

func (pgOps *PostgresOperations) deleteUserOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		object     = UserInfo{}
		opsKind    = CreateUserOp
		sqlTplRend = func(user UserInfo) string {
			return fmt.Sprintf(dropUserTpl, user.UserName)
		}
		msgTplRend = func(user UserInfo) string {
			return fmt.Sprintf("deleted user: %s", user.UserName)
		}
	)

	if err := ParseObjFromRequest(req, DefaultUserInfoParser, UserNameValidator, &object); err != nil {
		result := OpsResult{}
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}
	return ExecuteObject(ctx, pgOps, req, opsKind, sqlTplRend, msgTplRend, object)
}

func (pgOps *PostgresOperations) grantUserRoleOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		succMsgTpl = "role %s granted to user: %s"
	)
	return pgOps.managePrivillege(ctx, req, GrantUserRoleOp, grantTpl, succMsgTpl)
}

func (pgOps *PostgresOperations) revokeUserRoleOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		succMsgTpl = "role %s revoked from user: %s"
	)
	return pgOps.managePrivillege(ctx, req, RevokeUserRoleOp, revokeTpl, succMsgTpl)
}

func (pgOps *PostgresOperations) managePrivillege(ctx context.Context, req *bindings.InvokeRequest, op bindings.OperationKind, sqlTpl string, succMsgTpl string) (OpsResult, error) {
	var (
		object = UserInfo{}

		sqlTplRend = func(user UserInfo) string {
			if SuperUserRole.EqualTo(user.RoleName) {
				if op == GrantUserRoleOp {
					return "ALTER USER " + user.UserName + " WITH SUPERUSER;"
				} else {
					return "ALTER USER " + user.UserName + " WITH NOSUPERUSER;"
				}
			}
			roleDesc, _ := pgOps.role2PGRole(user.RoleName)
			return fmt.Sprintf(sqlTpl, roleDesc, user.UserName)
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

	return ExecuteObject(ctx, pgOps, req, op, sqlTplRend, msgTplRend, object)
}

func (pgOps *PostgresOperations) listUsersOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		opsKind    = ListUsersOp
		sqlTplRend = func(user UserInfo) string {
			return listUserTpl
		}
	)
	return QueryObject(ctx, pgOps, req, opsKind, sqlTplRend, pgUserRolesProcessor, UserInfo{})
}

func (pgOps *PostgresOperations) listSystemAccountsOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		opsKind    = ListUsersOp
		sqlTplRend = func(user UserInfo) string {
			return listSystemAccountsTpl
		}
	)
	dataProcessor := func(data interface{}) (interface{}, error) {
		type roleInfo struct {
			Rolname string `json:"rolname"`
		}
		var roles []roleInfo
		if err := json.Unmarshal(data.([]byte), &roles); err != nil {
			return nil, err
		}

		roleNames := make([]string, 0)
		for _, role := range roles {
			roleNames = append(roleNames, role.Rolname)
		}
		if jsonData, err := json.Marshal(roleNames); err != nil {
			return nil, err
		} else {
			return string(jsonData), nil
		}
	}

	return QueryObject(ctx, pgOps, req, opsKind, sqlTplRend, dataProcessor, UserInfo{})
}

func (pgOps *PostgresOperations) describeUserOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		object  = UserInfo{}
		opsKind = DescribeUserOp

		sqlTplRend = func(user UserInfo) string {
			return fmt.Sprintf(descUserTpl, user.UserName)
		}
	)

	if err := ParseObjFromRequest(req, DefaultUserInfoParser, UserNameValidator, &object); err != nil {
		result := OpsResult{}
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}

	return QueryObject(ctx, pgOps, req, opsKind, sqlTplRend, pgUserRolesProcessor, object)
}

// post-processing
func pgUserRolesProcessor(data interface{}) (interface{}, error) {
	type pgUserInfo struct {
		UserName string   `json:"username"`
		Expired  bool     `json:"expired"`
		Super    bool     `json:"usesuper"`
		Roles    []string `json:"roles"`
	}
	// parse data to struct
	var pgUsers []pgUserInfo
	err := json.Unmarshal(data.([]byte), &pgUsers)
	if err != nil {
		return nil, err
	}
	// parse roles
	users := make([]UserInfo, len(pgUsers))
	for i := range pgUsers {
		users[i] = UserInfo{
			UserName: pgUsers[i].UserName,
		}

		if pgUsers[i].Expired {
			users[i].Expired = "T"
		} else {
			users[i].Expired = "F"
		}

		// parse Super attribute
		if pgUsers[i].Super {
			pgUsers[i].Roles = append(pgUsers[i].Roles, string(SuperUserRole))
		}

		// convert to RoleType and sort by weight
		roleTypes := make([]RoleType, 0)
		for _, role := range pgUsers[i].Roles {
			roleTypes = append(roleTypes, String2RoleType(role))
		}
		slices.SortFunc(roleTypes, SortRoleByWeight)
		if len(roleTypes) > 0 {
			users[i].RoleName = string(roleTypes[0])
		}
	}
	if jsonData, err := json.Marshal(users); err != nil {
		return nil, err
	} else {
		return string(jsonData), nil
	}
}

func (pgOps *PostgresOperations) role2PGRole(roleName string) (string, error) {
	roleType := String2RoleType(roleName)
	switch roleType {
	case ReadWriteRole:
		return "pg_write_all_data", nil
	case ReadOnlyRole:
		return "pg_read_all_data", nil
	}
	return "", fmt.Errorf("role name: %s is not supported", roleName)
}
