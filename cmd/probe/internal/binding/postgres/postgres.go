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
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component/configuration_store"
	. "github.com/apecloud/kubeblocks/cmd/probe/util"
	"github.com/apecloud/kubeblocks/internal/sqlchannel"
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
	BaseOperations
}

var _ BaseInternalOps = &PostgresOperations{}

// NewPostgres returns a new PostgreSQL output binding.
func NewPostgres(logger logger.Logger) bindings.OutputBinding {
	return &PostgresOperations{
		BaseOperations: BaseOperations{
			Logger: logger,
			Cs:     configuration_store.NewConfigurationStore(),
		},
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
	pgOps.DBType = "postgresql" // TODO: check postgres or postgresql
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

	err := pgOps.Cs.GetClusterFromKubernetes()
	if err != nil {
		pgOps.Logger.Errorf("get cluster err:%v", err)
		result["event"] = OperationFailed
		result["message"] = err.Error()
		return result, nil
	}

	if can, err := pgOps.isSwitchoverPossible(ctx, primary, candidate, operation); !can || err != nil {
		result["event"] = OperationFailed
		result["message"] = err.Error()
		return result, nil
	}

	if err := pgOps.manualSwitchover(primary, candidate); err != nil {
		result["event"] = OperationFailed
		result["message"] = err.Error()
		return result, nil
	}

	if ok, err := pgOps.getSwitchoverResult(candidate, primary); err != nil {
		result["event"] = OperationFailed
		result["message"] = err.Error()
		return result, nil
	} else if !ok {
		result["event"] = OperationFailed
		result["message"] = fmt.Sprintf("%s to candidate: %s fail", string(req.Operation), candidate)
		return result, nil
	}
	result["event"] = OperationSuccess
	result["message"] = fmt.Sprintf("Successfully %s to: %s", string(req.Operation), candidate)

	return result, nil
}

// Checks whether there are nodes that could take it over after demoting the primary
func (pgOps *PostgresOperations) isSwitchoverPossible(ctx context.Context, primary string, candidate string, operation bindings.OperationKind) (bool, error) {
	if operation == FailoverOperation && candidate == "" {
		return false, errors.Errorf("failover need a candidate")
	} else if operation == SwitchoverOperation && primary == "" {
		return false, errors.Errorf("switchover need a primary")
	}

	replicationMode, err := pgOps.getReplicationMode(ctx)
	if err != nil {
		return false, errors.Errorf("get replication mode failed, err:%v", err)
	}
	if candidate != "" {
		if pgOps.Cs.GetCluster().Leader != nil && pgOps.Cs.GetCluster().Leader.GetMember().GetName() != primary {
			return false, errors.Errorf("leader name does not match ,leader name: %s, primary: %s", pgOps.Cs.GetCluster().Leader.GetMember().GetName(), primary)
		}
		// candidate存在时，即使candidate并未同步也可以进行failover
		if operation == SwitchoverOperation && replicationMode == SynchronousMode && !pgOps.checkStandbySynchronizedToLeader(ctx, candidate, false) {
			return false, errors.Errorf("candidate name does not match with sync_standby")
		}

		if !pgOps.Cs.GetCluster().HasMember(candidate) {
			return false, errors.Errorf("candidate does not exist")
		}
	} else if replicationMode == SynchronousMode {
		syncToLeader := 0
		for _, member := range pgOps.Cs.GetCluster().GetMemberName() {
			if pgOps.checkStandbySynchronizedToLeader(ctx, member, false) {
				syncToLeader++
			}
		}
		if syncToLeader == 0 {
			return false, errors.Errorf("can not find sync standby")
		}
	} else {
		hasMemberExceptLeader := false
		for _, member := range pgOps.Cs.GetCluster().Members {
			if member.GetName() != pgOps.Cs.GetCluster().Leader.GetMember().GetName() {
				hasMemberExceptLeader = true
			}
		}
		if !hasMemberExceptLeader {
			return false, errors.Errorf("cluster does not have member except leader")
		}
	}

	runningMembers := 0
	pods, err := pgOps.Cs.ListPods()
	for _, pod := range pods.Items {
		client, err := sqlchannel.NewClientWithPod(&pod, pgOps.DBType)
		if err != nil {
			return false, errors.Errorf("new client with pod err:%v", err)
		}
		resp, err := client.CheckStatus()
		if err != nil {
			return false, errors.Errorf("client check status err:%v", err)
		}
		if resp == OperationSuccess {
			runningMembers++
		}
	}
	if runningMembers == 0 {
		return false, errors.Errorf("no running candidates have been found")
	}

	return true, nil
}

func (pgOps *PostgresOperations) manualSwitchover(primary, candidate string) error {
	annotations := map[string]string{
		LEADER:    primary,
		CANDIDATE: candidate,
	}
	_, err := pgOps.Cs.CreateConfigMap(pgOps.Cs.GetClusterCompName()+configuration_store.SwitchoverSuffix, annotations)
	if err != nil {
		return err
	}

	return nil
}

func (pgOps *PostgresOperations) getSwitchoverResult(oldPrimary, candidate string) (bool, error) {
	wait := int(pgOps.Cs.GetCluster().Config.GetData().GetTtl() / 2)
	for i := 0; i < wait; i++ {
		time.Sleep(time.Second)
		_ = pgOps.Cs.GetClusterFromKubernetes()
		newPrimary := pgOps.Cs.GetCluster().Leader.GetMember().GetName()
		if newPrimary == candidate || (newPrimary != oldPrimary && candidate == "") {
			return true, nil
		}
	}

	return false, errors.New("switchover fail")
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

func (pgOps *PostgresOperations) getReplicationMode(ctx context.Context) (string, error) {
	sql := "select pg_catalog.current_setting('synchronous_commit');"
	mode, err := pgOps.query(ctx, sql)
	if err != nil {
		return "", err
	}

	switch string(mode) {
	case "off":
		return AsynchronousMode, nil
	case "local":
		return AsynchronousMode, nil
	case "remote_write":
		return AsynchronousMode, nil
	case "on":
		return SynchronousMode, nil
	case "remote_apply":
		return SynchronousMode, nil
	default: // default "on"
		return SynchronousMode, nil
	}
}

func (pgOps *PostgresOperations) checkStandbySynchronizedToLeader(ctx context.Context, member string, checkLeader bool) bool {
	sql := "select pg_catalog.current_setting('synchronous_standby_names');"
	resp, err := pgOps.query(ctx, sql)
	if err != nil {
		pgOps.Logger.Errorf("query sql:%s, err:%v", sql, err)
		return false
	}
	syncStandbys, err := ParsePGSyncStandby(string(resp))
	if err != nil {
		pgOps.Logger.Errorf("parse pg sync standby failed, err:%v", err)
		return false
	}

	return (checkLeader && member == pgOps.Cs.LeaderObservedRecord.GetLeader()) || syncStandbys.Members.Contains(member)
}

func (pgOps *PostgresOperations) getWalPosition(ctx context.Context) (int64, error) {
	var lsn int64
	var err error
	if pgOps.IsLeader(ctx) {
		lsn, err = pgOps.getLsn(ctx, "current")
		if err != nil {
			return 0, err
		}
	} else {
		replayLsn, errReplay := pgOps.getLsn(ctx, "replay")
		receiveLsn, errReceive := pgOps.getLsn(ctx, "receive")
		if errReplay != nil && errReceive != nil {
			return 0, errors.Errorf("get replayLsn or receiveLsn failed, replayLsn err:%v, receiveLsn err:%v", errReplay, errReceive)
		}
		lsn = MaxInt64(replayLsn, receiveLsn)
	}

	return lsn, nil
}

func (pgOps *PostgresOperations) getLsn(ctx context.Context, types string) (int64, error) {
	var sql string
	switch types {
	case "current":
		sql = "select pg_catalog.pg_wal_lsn_diff(pg_catalog.pg_current_wal_lsn(), '0/0')::bigint;"
	case "replay":
		sql = "select pg_catalog.pg_wal_lsn_diff(pg_catalog.pg_last_wal_replay_lsn(), '0/0')::bigint;"
	case "receive":
		sql = "select pg_catalog.pg_wal_lsn_diff(COALESCE(pg_catalog.pg_last_wal_receive_lsn(), '0/0'), '0/0')::bigint;"
	}

	resp, err := pgOps.query(ctx, sql)
	if err != nil {
		pgOps.Logger.Errorf("get wal position err:%v", err)
		return 0, err
	}

	return int64(binary.BigEndian.Uint64(resp)), nil
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

func (pgOps *PostgresOperations) GetExtra(ctx context.Context) (map[string]string, error) {
	timeLineID, err := pgOps.getTimeline(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"timeline_id": timeLineID,
	}, nil
}

func (pgOps *PostgresOperations) getTimeline(ctx context.Context) (string, error) {
	sql := "SELECT CASE WHEN pg_catalog.pg_is_in_recovery() THEN 0 " +
		"ELSE ('x' || pg_catalog.substr(pg_catalog.pg_walfile_name(pg_catalog.pg_current_wal_lsn()), 1, 8))::bit(32)::int END"

	res, err := pgOps.query(ctx, sql)
	if err != nil {
		return "", err
	}

	return string(res), nil
}

// Promote 考虑异步
func (pgOps *PostgresOperations) Promote(ctx context.Context, podName string) error {
	err := pgOps.prePromote(ctx)
	if err != nil {
		return err
	}

	cmd := "su -c 'pg_ctl promote' postgres"
	resp, err := pgOps.Cs.ExecCmdWithPod(ctx, podName, cmd, pgOps.DBType)
	if err != nil {
		pgOps.Logger.Errorf("promote err: %v", err)
		return err
	}
	pgOps.Logger.Infof("response: ", resp)

	err = pgOps.waitPromote(ctx)
	return nil
}

// 执行一个脚本
func (pgOps *PostgresOperations) prePromote(ctx context.Context) error {
	return nil
}

func (pgOps *PostgresOperations) waitPromote(ctx context.Context) error {
	return nil
}

func (pgOps *PostgresOperations) Demote(ctx context.Context, podName string) error {
	stopCmd := "su -c 'pg_ctl stop -m fast' postgres"
	_, err := pgOps.Cs.ExecCmdWithPod(ctx, podName, stopCmd, pgOps.DBType)
	if err != nil {
		pgOps.Logger.Errorf("stop err: %v", err)
		return err
	}

	opTime, _ := pgOps.getWalPosition(ctx)
	err = pgOps.Cs.DeleteLeader(opTime)

	// Give a time to somebody to take the leader lock
	time.Sleep(time.Second * 2)
	_ = pgOps.Cs.GetClusterFromKubernetes()
	leader := pgOps.Cs.GetCluster().Leader

	return pgOps.HandleFollow(ctx, leader, podName, true)
}

// GetStatus TODO：GetStatus后期考虑用postmaster替代
func (pgOps *PostgresOperations) GetStatus(ctx context.Context) (string, error) {
	resp, err := pgOps.CheckStatusOps(ctx, &bindings.InvokeRequest{}, &bindings.InvokeResponse{
		Metadata: map[string]string{},
	})
	if err != nil || resp["event"] == OperationFailed {
		return "", errors.Errorf("get status failed:%s", resp["message"])
	}

	return "running", nil
}

func (pgOps *PostgresOperations) GetOpTime(ctx context.Context) (int64, error) {
	return pgOps.getWalPosition(ctx)
}

func (pgOps *PostgresOperations) IsLeader(ctx context.Context) bool {
	role, err := pgOps.GetRole(ctx, &bindings.InvokeRequest{}, &bindings.InvokeResponse{})
	if err != nil {
		pgOps.Logger.Errorf("get role failed, err:%v", err)
	}

	return role == PRIMARY
}

func (pgOps *PostgresOperations) IsHealthiest(ctx context.Context, podName string) bool {
	err := pgOps.Cs.GetClusterFromKubernetes()
	if err != nil {
		pgOps.Logger.Errorf("get cluster from k8s failed, err:%v", err)
	}

	replicationMode, err := pgOps.getReplicationMode(ctx)
	if err != nil {
		pgOps.Logger.Errorf("get db replication mode err:%v", err)
		return false
	}

	var members []string
	if replicationMode == SynchronousMode {
		if !pgOps.checkStandbySynchronizedToLeader(ctx, podName, true) {
			return false
		}
		for _, m := range pgOps.Cs.GetCluster().Members {
			if pgOps.checkStandbySynchronizedToLeader(ctx, m.GetName(), true) && m.GetName() != podName {
				members = append(members, m.GetName())
			}
		}
	} else {
		for _, m := range pgOps.Cs.GetCluster().Members {
			if m.GetName() != podName {
				members = append(members, m.GetName())
			}
		}
	}

	walPosition, _ := pgOps.getWalPosition(ctx)
	if pgOps.isLagging(walPosition) {
		pgOps.Logger.Infof("my wal position exceeds max lag")
		return false
	}

	timeline, err := pgOps.getTimeline(ctx)
	if err != nil {
		pgOps.Logger.Errorf("get timelineID  err:%v", err)
		return false
	}
	clusterTimeLine := pgOps.Cs.LeaderObservedRecord.GetExtra()["timeline_id"]
	timelineID, _ := strconv.Atoi(timeline)
	clusterTimeLineID, _ := strconv.Atoi(clusterTimeLine)
	if timelineID < clusterTimeLineID {
		pgOps.Logger.Infof("My timeline %s is behind last known cluster timeline %s", timeline, clusterTimeLine)
		return false
	}

	pods, err := pgOps.Cs.ListPods()
	for _, pod := range pods.Items {
		client, err := sqlchannel.NewClientWithPod(&pod, pgOps.DBType)
		if err != nil {
			pgOps.Logger.Errorf("new client with pod err:%v", err)
		}
		role, err := client.GetRole()
		if err != nil {
			pgOps.Logger.Errorf("client check status err:%v", err)
			return false
		}
		if role == PRIMARY {
			pgOps.Logger.Errorf("Primary %s is still alive", pod.Name)
			return false
		}
		// TODO: getLag
	}

	return true
}

func (pgOps *PostgresOperations) IsRunning(ctx context.Context) bool {
	return true
}

func (pgOps *PostgresOperations) isLagging(walPosition int64) bool {
	lag := pgOps.Cs.GetCluster().GetOpTime() - walPosition
	return lag > pgOps.Cs.GetCluster().Config.GetData().GetMaxLagOnSwitchover()
}

func (pgOps *PostgresOperations) HandleFollow(ctx context.Context, leader *configuration_store.Leader, podName string, restart bool) error {
	need, _ := pgOps.isRewindOrReinitializePossible(ctx, leader, podName)
	if !need {
		return nil
	}
	pgOps.executeRewind()

	if restart {
		return pgOps.start(ctx, podName)
	}

	return nil
}

func (pgOps *PostgresOperations) start(ctx context.Context, podName string) error {
	startCmd := "su -c 'postgres -D /postgresql/data --config-file=/opt/bitnami/postgresql/conf/postgresql.conf --external_pid_file=/opt/bitnami/postgresql/tmp/postgresql.pid --hba_file=/opt/bitnami/postgresql/conf/pg_hba.conf' postgres &"
	_, err := pgOps.Cs.ExecCmdWithPod(ctx, podName, startCmd, pgOps.DBType)
	if err != nil {
		pgOps.Logger.Errorf("start err: %v", err)
		return err
	}

	return nil
}

func (pgOps *PostgresOperations) executeRewind() {
}

func (pgOps *PostgresOperations) isRewindOrReinitializePossible(ctx context.Context, leader *configuration_store.Leader, podName string) (bool, error) {
	return pgOps.checkTimelineAndLsn(ctx, leader, podName), nil
}

func (pgOps *PostgresOperations) checkTimelineAndLsn(ctx context.Context, leader *configuration_store.Leader, podName string) bool {
	var needRewind bool
	var historys []*history

	isRecovery, localTimeLine, localLsn := pgOps.getLocalTimeLineAndLsn(ctx, podName)
	if localTimeLine == 0 || localLsn == 0 {
		return false
	}

	if leader.GetMember().GetName() != MASTER {
		return false
	}

	leaderPod, err := pgOps.Cs.GetPod(leader.GetMember().GetName())
	if err != nil {
		pgOps.Logger.Errorf("get leader pod failed, err:%v", err)
	}
	client, err := sqlchannel.NewClientWithPod(leaderPod, pgOps.DBType)
	// check leader is in recovery
	role, err := client.GetRole()
	if role != PRIMARY {
		pgOps.Logger.Infof("Leader is still in_recovery and therefore can't be used for rewind")
		return false
	}

	primaryTimeLine, err := pgOps.getPrimaryTimeLine(ctx, podName)
	if err != nil {
		pgOps.Logger.Errorf("get primary timeLine failed, err:%v", err)
		return false
	}

	if localTimeLine > primaryTimeLine {
		needRewind = true
	} else if localTimeLine == primaryTimeLine {
		needRewind = false
	} else if primaryTimeLine > 1 {
		historys = pgOps.getHistory()
	}

	if historys != nil {
		for _, h := range historys {
			if h.parentTimeline == localTimeLine {
				if isRecovery {
					needRewind = localLsn > h.switchPoint
				} else if localLsn >= h.switchPoint {
					needRewind = true
				} else {
					// TODO:get checkpoint end
				}
				break
			} else if h.parentTimeline > localTimeLine {
				needRewind = true
				break
			}
		}
	}

	return needRewind
}

type history struct {
	parentTimeline int64
	switchPoint    int64
}

// TODO
func (pgOps *PostgresOperations) getHistory() []*history {
	return nil
}

func (pgOps *PostgresOperations) getPrimaryTimeLine(ctx context.Context, podName string) (int64, error) {
	cmd := `psql "replication=database" -c "IDENTIFY_SYSTEM";`
	resp, err := pgOps.ExecCmd(ctx, podName, cmd)
	if err != nil {
		return 0, err
	}

	stdout := resp["stdout"]
	stdoutList := strings.Split(stdout, "\n")
	value := stdoutList[2]
	values := strings.Split(value, "|")

	primaryTimeLine := strings.TrimSpace(values[1])

	return strconv.ParseInt(primaryTimeLine, 10, 64)
}

func (pgOps *PostgresOperations) getLocalTimeLineAndLsn(ctx context.Context, podName string) (bool, int64, int64) {
	var isRecovery bool

	status, err := pgOps.GetStatus(ctx)
	if err != nil || status != "running" {
		return pgOps.getLocalTimeLineAndLsnFromControlData(ctx, podName)
	}

	isRecovery = true
	timeLine := pgOps.getReceivedTimeLine(ctx)
	lsn, _ := pgOps.getLsn(ctx, "replay")

	return isRecovery, timeLine, lsn
}

func (pgOps *PostgresOperations) getLocalTimeLineAndLsnFromControlData(ctx context.Context, podName string) (bool, int64, int64) {
	var isRecovery bool
	var timeLineStr, lsnStr string
	var timeLine, lsn int64

	pgControlData := pgOps.getPgControlData(ctx, podName)
	if slices.Contains([]string{"shut down in recovery", "in archive recovery"}, (*pgControlData)["Database cluster state"]) {
		isRecovery = true
		lsnStr = (*pgControlData)["Minimum recovery ending location"]
		timeLineStr = (*pgControlData)["Min recovery ending loc's timeline"]
	} else if (*pgControlData)["Database cluster state"] == "shut down" {
		isRecovery = false
		lsnStr = (*pgControlData)["Latest checkpoint location"]
		timeLineStr = (*pgControlData)["Latest checkpoint's TimeLineID"]
	}

	if lsnStr != "" {
		lsn = ParsePgLsn(lsnStr)
	}
	if timeLineStr != "" {
		timeLine, _ = strconv.ParseInt(timeLineStr, 10, 64)
	}

	return isRecovery, timeLine, lsn
}

func (pgOps *PostgresOperations) getReceivedTimeLine(ctx context.Context) int64 {
	sql := "select case when latest_end_lsn is null then null " +
		"else received_tli end as received_tli from pg_catalog.pg_stat_get_wal_receiver();"

	resp, err := pgOps.query(ctx, sql)
	if err != nil || resp == nil {
		pgOps.Logger.Errorf("get received time line failed, err%v", err)
		return 0
	}

	return int64(binary.BigEndian.Uint64(resp))
}

func (pgOps *PostgresOperations) getPgControlData(ctx context.Context, podName string) *map[string]string {
	cmd := "pg_controldata"
	result := map[string]string{}

	// 后期直接用脚本替代
	resp, err := pgOps.ExecCmd(ctx, podName, cmd)
	if err != nil {
		pgOps.Logger.Errorf("get pg control data failed, err:%v", err)
		return &result
	}

	stdoutList := strings.Split(resp["stdout"], "\n")
	for _, s := range stdoutList {
		stdout := strings.Split(s, ":")
		if len(stdout) == 2 {
			result[stdout[0]] = strings.TrimSpace(stdout[1])
		}
	}

	return &result
}

func (pgOps *PostgresOperations) EnforcePrimaryRole(ctx context.Context, podName string) error {
	if pgOps.IsLeader(ctx) {
		err := pgOps.processSyncReplication()
		return err
	} else {
		replicationMode, err := pgOps.getReplicationMode(ctx)
		if err != nil {
			return err
		}

		if replicationMode == SynchronousMode {
			err = pgOps.setSynchronousStandbyNames()
			if err != nil {
				return err
			}
		}

		err = pgOps.Promote(ctx, podName)
	}

	return nil
}

func (pgOps *PostgresOperations) processSyncReplication() error {
	return nil
}

// set synchronous_standby_names and reload
func (pgOps *PostgresOperations) setSynchronousStandbyNames() error {
	return nil
}

func (pgOps *PostgresOperations) ProcessManualSwitchoverFromLeader(ctx context.Context, podName string) error {
	err := pgOps.Cs.GetClusterFromKubernetes()
	if err != nil {
		pgOps.Logger.Errorf("get cluster from k8s failed, err:%v", err)
		return err
	}

	switchover := pgOps.Cs.GetCluster().Switchover
	if switchover == nil {
		return nil
	}

	leader := switchover.GetLeader()
	candidate := switchover.GetCandidate()
	if leader == "" || leader == podName {
		if candidate == "" || candidate != podName {
			replicationMode, err := pgOps.getReplicationMode(ctx)
			if err != nil {
				return err
			}

			var members []string
			if replicationMode == SynchronousMode {
				if candidate != "" && pgOps.checkStandbySynchronizedToLeader(ctx, candidate, false) {
					pgOps.Logger.Warnf("candidate=%s does not match", candidate)
				} else {
					for _, m := range pgOps.Cs.GetCluster().Members {
						if pgOps.checkStandbySynchronizedToLeader(ctx, m.GetName(), false) {
							members = append(members, m.GetName())
						}
					}
				}
			} else {
				for _, m := range pgOps.Cs.GetCluster().Members {
					if switchover.GetCandidate() != "" || m.GetName() == candidate {
						members = append(members, m.GetName())
					}
				}
			}

			if pgOps.isFailoverPossible() {
				return pgOps.Demote(ctx, podName)
			}
		} else {
			pgOps.Logger.Warnf("manual failover: I am already the leader, no need to failover")
		}
	} else {
		pgOps.Logger.Warnf("manual switchover, leader name does not match, %s != %s", switchover.GetLeader(), podName)
	}

	return pgOps.Cs.DeleteConfigMap(pgOps.Cs.GetClusterCompName() + configuration_store.SwitchoverSuffix)
}

func (pgOps *PostgresOperations) isFailoverPossible() bool {
	return true
}
