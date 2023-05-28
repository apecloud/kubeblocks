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
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component/configuration_store"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/ha"
	"strconv"
	"sync"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	. "github.com/apecloud/kubeblocks/cmd/probe/util"
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

var (
	defaultDBPort = 3306
	dbUser        = ""
	dbPasswd      = ""
)

// PostgresOperations represents PostgreSQL output binding.
type PostgresOperations struct {
	mu sync.Mutex
	db *pgxpool.Pool
	cs *configuration_store.ConfigurationStore
	BaseOperations
}

var _ ha.DB = &PostgresOperations{}
var _ BaseInternalOps = &PostgresOperations{}

// NewPostgres returns a new PostgreSQL output binding.
func NewPostgres(logger logger.Logger) bindings.OutputBinding {
	return &PostgresOperations{
		BaseOperations: BaseOperations{Logger: logger},
		cs:             configuration_store.NewConfigurationStore(),
	}
}

// Init initializes the PostgreSql binding.
func (pgOps *PostgresOperations) Init(metadata bindings.Metadata) error {
	pgOps.BaseOperations.Init(metadata)
	if viper.IsSet("KB_SERVICE_USER") {
		dbUser = viper.GetString("KB_SERVICE_USER")
	}

	if viper.IsSet("KB_SERVICE_PASSWORD") {
		dbPasswd = viper.GetString("KB_SERVICE_PASSWORD")
	}

	pgOps.Logger.Debug("Initializing Postgres binding")
	pgOps.DBType = "postgres"
	pgOps.InitIfNeed = pgOps.initIfNeed
	pgOps.BaseOperations.GetRole = pgOps.GetRole
	pgOps.DBPort = pgOps.GetRunningPort()
	pgOps.RegisterOperation(GetRoleOperation, pgOps.GetRoleOps)
	// pgOps.RegisterOperation(GetLagOperation, pgOps.GetLagOps)
	pgOps.RegisterOperation(CheckStatusOperation, pgOps.CheckStatusOps)
	pgOps.RegisterOperation(ExecOperation, pgOps.ExecOps)
	pgOps.RegisterOperation(QueryOperation, pgOps.QueryOps)
	pgOps.RegisterOperation(SwitchoverOperation, pgOps.SwitchoverOps)
	pgOps.RegisterOperation(FailoverOperation, pgOps.FailoverOps)

	// following are ops for account management
	pgOps.RegisterOperation(ListUsersOp, pgOps.listUsersOps)
	pgOps.RegisterOperation(CreateUserOp, pgOps.createUserOps)
	pgOps.RegisterOperation(DeleteUserOp, pgOps.deleteUserOps)
	pgOps.RegisterOperation(DescribeUserOp, pgOps.describeUserOps)
	pgOps.RegisterOperation(GrantUserRoleOp, pgOps.grantUserRoleOps)
	pgOps.RegisterOperation(RevokeUserRoleOp, pgOps.revokeUserRoleOps)
	pgOps.RegisterOperation(ListSystemAccountsOp, pgOps.listSystemAccountsOps)
	return nil
}

func (pgOps *PostgresOperations) initIfNeed() bool {
	if pgOps.db == nil {
		go func() {
			err := pgOps.InitDelay()
			if err != nil {
				pgOps.Logger.Errorf("Postgres connection init failed: %v", err)
			} else {
				pgOps.Logger.Info("Postgres connection init success: %s", pgOps.db.Config().ConnConfig)
			}
		}()
		return true
	}
	return false
}

func (pgOps *PostgresOperations) InitDelay() error {
	pgOps.mu.Lock()
	defer pgOps.mu.Unlock()
	if pgOps.db != nil {
		return nil
	}

	p := pgOps.Metadata.Properties
	url, ok := p[connectionURLKey]
	if !ok || url == "" {
		return fmt.Errorf("required metadata not set: %s", connectionURLKey)
	}

	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		return fmt.Errorf("error opening DB connection: %w", err)
	}
	if dbUser != "" {
		poolConfig.ConnConfig.User = dbUser
	}
	if dbPasswd != "" {
		poolConfig.ConnConfig.Password = dbPasswd
	}

	// This context doesn't control the lifetime of the connection pool, and is
	// only scoped to postgres creating resources at init.
	pgOps.db, err = pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return fmt.Errorf("unable to ping the DB: %w", err)
	}

	return nil
}

func (pgOps *PostgresOperations) GetRunningPort() int {
	p := pgOps.Metadata.Properties
	url, ok := p[connectionURLKey]
	if !ok || url == "" {
		return defaultDBPort
	}

	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		return defaultDBPort
	}
	if poolConfig.ConnConfig.Port == 0 {
		return defaultDBPort
	}
	return int(poolConfig.ConnConfig.Port)
}

func (pgOps *PostgresOperations) GetRole(ctx context.Context, request *bindings.InvokeRequest, response *bindings.InvokeResponse) (string, error) {
	sql := "select pg_is_in_recovery();"

	// sql exec timeout need to be less than httpget's timeout which default is 1s.
	ctx1, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	rows, err := pgOps.db.Query(ctx1, sql)
	if err != nil {
		pgOps.Logger.Infof("error executing %s: %v", sql, err)
		return "", errors.Wrapf(err, "error executing %s", sql)
	}

	var isRecovery bool
	for rows.Next() {
		if err = rows.Scan(&isRecovery); err != nil {
			pgOps.Logger.Errorf("Role query error: %v", err)
			return "", err
		}
	}
	if isRecovery {
		return SECONDARY, nil
	}
	return PRIMARY, nil
}

func (pgOps *PostgresOperations) ExecOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	result := OpsResult{}
	sql, ok := req.Metadata["sql"]
	if !ok || sql == "" {
		result["event"] = OperationFailed
		result["message"] = "no sql provided"
		return result, nil
	}
	count, err := pgOps.exec(ctx, sql)
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

// CheckStatusOps design details: https://infracreate.feishu.cn/wiki/wikcndch7lMZJneMnRqaTvhQpwb#doxcnOUyQ4Mu0KiUo232dOr5aad
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
		count, err = pgOps.exec(ctx, rwSQL)
		data = []byte(strconv.FormatInt(count, 10))
	case SECONDARY:
		data, err = pgOps.query(ctx, roSQL)
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
	data, err := pgOps.query(ctx, sql)
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

// SwitchoverOps switchover and failover share this
func (pgOps *PostgresOperations) SwitchoverOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	result := OpsResult{}
	primary, _ := req.Metadata[PRIMARY]
	candidate, _ := req.Metadata[CANDIDATE]
	operation := req.Operation

	err := pgOps.cs.GetCluster()
	if err != nil {
		pgOps.Logger.Errorf("get cluster err:%v", err)
		result["event"] = OperationFailed
		result["message"] = err.Error()
		return result, nil
	}

	if can, err := pgOps.isSwitchOverPossible(primary, candidate, operation); !can || err != nil {
		result["event"] = OperationFailed
		result["message"] = err.Error()
		return result, nil
	}

	if err := pgOps.manualSwitchOver(ctx, primary, candidate); err != nil {
		result["event"] = OperationFailed
		result["message"] = err.Error()
		return result, nil
	}
	if ok, err := pgOps.getSwitchOverResult(candidate); err != nil {
		result["event"] = OperationFailed
		result["message"] = err.Error()
		return result, nil
	} else if !ok {
		result["event"] = OperationFailed
		result["message"] = fmt.Sprintf("switchover to candidate: %s fail", candidate)
		return result, nil
	}

	result["event"] = OperationSuccess
	result["message"] = fmt.Sprintf("Successfully switch over to: %s", candidate)

	return result, nil
}

// Checks whether there are nodes that could take it over after demoting the primary
func (pgOps *PostgresOperations) isSwitchOverPossible(primary string, candidate string, operation bindings.OperationKind) (bool, error) {
	if operation == FailoverOperation && candidate == "" {
		return false, errors.Errorf("failover need a candidate")
	} else if operation == SwitchoverOperation && primary == "" {
		return false, errors.Errorf("switchover need a primary")
	}

	clusterConfig := pgOps.cs.Cluster.Config.GetData()
	if candidate != "" {
		if pgOps.cs.Cluster.Leader != nil && pgOps.cs.Cluster.Leader.GetMember().GetName() != primary {
			return false, errors.Errorf("leader name does not match ,leader name: %s, primary: %s", pgOps.cs.Cluster.Leader.GetMember().GetName(), primary)
		}
		// candidate存在时，即使candidate并未同步也可以进行failover
		if operation == SwitchoverOperation && clusterConfig.GetReplicationMode() == SynchronousMode && !pgOps.cs.Cluster.Sync.SynchronizedToLeader(candidate) {
			return false, errors.Errorf("candidate name does not match with sync_standby")
		}

		if !pgOps.cs.Cluster.HasMember(candidate) {
			return false, errors.Errorf("candidate does not exist")
		}
	} else if clusterConfig.GetReplicationMode() == SynchronousMode {

	}

	// TODO: isFailOverPossible还依赖很多其他状态，先不做
	return true, nil
}

// 更改配置
func (pgOps *PostgresOperations) manualSwitchOver(ctx context.Context, primary string, candidate string) error {
	configMap, err := pgOps.cs.GetConfigMap("default", "test")
	if err != nil {
		return err
	}
	configMap.Data["primary"] = candidate

	_, err = pgOps.cs.UpdateConfigMap("default", configMap)
	return nil
}

func (pgOps *PostgresOperations) getSwitchOverResult(candidate string) (bool, error) {
	pgOps.cs.GetCluster()
	time.Sleep(30 * time.Second)
	if pgOps.cs.Cluster.Leader.GetMember().GetName() != candidate {
		return false, errors.New("switchover fail")
	}

	return true, nil
}

func (pgOps *PostgresOperations) FailoverOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	return pgOps.SwitchoverOps(ctx, req, resp)
}

func (pgOps *PostgresOperations) query(ctx context.Context, sql string) (result []byte, err error) {
	pgOps.Logger.Debugf("query: %s", sql)

	rows, err := pgOps.db.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("error executing query: %w", err)
	}
	defer func() {
		rows.Close()
		_ = rows.Err()
	}()

	rs := make([]interface{}, 0)
	columnTypes := rows.FieldDescriptions()
	for rows.Next() {
		values := make([]interface{}, len(columnTypes))
		for i := range values {
			values[i] = new(interface{})
		}

		if err = rows.Scan(values...); err != nil {
			return
		}

		r := map[string]interface{}{}
		for i, ct := range columnTypes {
			r[ct.Name] = values[i]
		}
		rs = append(rs, r)
	}

	if result, err = json.Marshal(rs); err != nil {
		err = fmt.Errorf("error serializing results: %w", err)
	}
	return result, err
}

func (pgOps *PostgresOperations) exec(ctx context.Context, sql string) (result int64, err error) {
	pgOps.Logger.Debugf("exec: %s", sql)

	res, err := pgOps.db.Exec(ctx, sql)
	if err != nil {
		return 0, fmt.Errorf("error executing query: %w", err)
	}

	result = res.RowsAffected()

	return
}

// InternalQuery is used for internal query, implement BaseInternalOps interface
func (pgOps *PostgresOperations) InternalQuery(ctx context.Context, sql string) (result []byte, err error) {
	return pgOps.query(ctx, sql)
}

// InternalExec is used for internal execution, implement BaseInternalOps interface
func (pgOps *PostgresOperations) InternalExec(ctx context.Context, sql string) (result int64, err error) {
	return pgOps.exec(ctx, sql)
}

// GetLogger is used for getting logger, implement BaseInternalOps interface
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

func (pgOps *PostgresOperations) Stop(ctx context.Context) error {
	return nil
}

func (pgOps *PostgresOperations) GetSysID(ctx context.Context) (string, error) {
	sql := "select system_identifier from pg_control_system();"
	res, err := pgOps.query(ctx, sql)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func (pgOps *PostgresOperations) GetState(ctx context.Context) (string, error) {
	return "running", nil
}

func (pgOps *PostgresOperations) GetExtra(ctx context.Context) map[string]string {
	return nil
}

func (pgOps *PostgresOperations) Promote() error {
	return nil
}

func (pgOps *PostgresOperations) Demote() error {
	return nil
}
