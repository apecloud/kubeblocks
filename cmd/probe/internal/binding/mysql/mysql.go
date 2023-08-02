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

package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/pkg/errors"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal"
	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component/mysql"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
	. "github.com/apecloud/kubeblocks/internal/sqlchannel/util"
)

// MysqlOperations represents MySQL output bindings.
type MysqlOperations struct {
	manager *mysql.Manager
	BaseOperations
}

type QueryRes []map[string]interface{}

var _ RMDBInternalOps = &MysqlOperations{}

const (
	superUserPriv = "ALL PRIVILEGES"
	readWritePriv = "SELECT, INSERT, UPDATE, DELETE"
	readOnlyRPriv = "SELECT"
	noPriv        = "USAGE"

	listUserTpl  = "SELECT user AS userName, CASE password_expired WHEN 'N' THEN 'F' ELSE 'T' END as expired FROM mysql.user WHERE host = '%' and user <> 'root' and user not like 'kb%';"
	showGrantTpl = "SHOW GRANTS FOR '%s'@'%%';"
	getUserTpl   = `
	SELECT user AS userName, CASE password_expired WHEN 'N' THEN 'F' ELSE 'T' END as expired
	FROM mysql.user
	WHERE host = '%%' and user <> 'root' and user not like 'kb%%' and user ='%s';"
	`
	createUserTpl         = "CREATE USER '%s'@'%%' IDENTIFIED BY '%s';"
	deleteUserTpl         = "DROP USER IF EXISTS '%s'@'%%';"
	grantTpl              = "GRANT %s TO '%s'@'%%';"
	revokeTpl             = "REVOKE %s FROM '%s'@'%%';"
	listSystemAccountsTpl = "SELECT user AS userName FROM mysql.user WHERE host = '%' and user like 'kb%';"
)

// NewMysql returns a new MySQL output binding.
func NewMysql(logger logger.Logger) bindings.OutputBinding {
	return &MysqlOperations{BaseOperations: BaseOperations{Logger: logger}}
}

// Init initializes the MySQL binding.
func (mysqlOps *MysqlOperations) Init(metadata bindings.Metadata) error {
	mysqlOps.Logger.Debug("Initializing MySQL binding")
	mysqlOps.BaseOperations.Init(metadata)
	config, err := mysql.NewConfig(metadata.Properties)
	if err != nil {
		mysqlOps.Logger.Errorf("MySQL config initialize failed: %v", err)
		return err
	}
	manager, err := mysql.NewManager(mysqlOps.Logger)
	if err != nil {
		mysqlOps.Logger.Errorf("MySQL DB Manager initialize failed: %v", err)
		return err
	}
	mysqlOps.manager = manager
	mysqlOps.DBType = "mysql"
	// mysqlOps.InitIfNeed = mysqlOps.initIfNeed
	mysqlOps.BaseOperations.GetRole = mysqlOps.GetRole
	mysqlOps.DBPort = config.GetDBPort()

	mysqlOps.RegisterOperationOnDBReady(GetRoleOperation, mysqlOps.GetRoleOps, manager)
	mysqlOps.RegisterOperationOnDBReady(CheckRoleOperation, mysqlOps.CheckRoleOps, manager)
	mysqlOps.RegisterOperationOnDBReady(GetLagOperation, mysqlOps.GetLagOps, manager)
	mysqlOps.RegisterOperationOnDBReady(CheckStatusOperation, mysqlOps.CheckStatusOps, manager)
	mysqlOps.RegisterOperationOnDBReady(ExecOperation, mysqlOps.ExecOps, manager)
	mysqlOps.RegisterOperationOnDBReady(QueryOperation, mysqlOps.QueryOps, manager)

	// following are ops for account management
	mysqlOps.RegisterOperationOnDBReady(ListUsersOp, mysqlOps.listUsersOps, manager)
	mysqlOps.RegisterOperationOnDBReady(CreateUserOp, mysqlOps.createUserOps, manager)
	mysqlOps.RegisterOperationOnDBReady(DeleteUserOp, mysqlOps.deleteUserOps, manager)
	mysqlOps.RegisterOperationOnDBReady(DescribeUserOp, mysqlOps.describeUserOps, manager)
	mysqlOps.RegisterOperationOnDBReady(GrantUserRoleOp, mysqlOps.grantUserRoleOps, manager)
	mysqlOps.RegisterOperationOnDBReady(RevokeUserRoleOp, mysqlOps.revokeUserRoleOps, manager)
	mysqlOps.RegisterOperationOnDBReady(ListSystemAccountsOp, mysqlOps.listSystemAccountsOps, manager)
	return nil
}

func (mysqlOps *MysqlOperations) GetRole(ctx context.Context, request *bindings.InvokeRequest, response *bindings.InvokeResponse) (string, error) {
	workloadType := request.Metadata[WorkloadTypeKey]
	if strings.EqualFold(workloadType, Replication) {
		return mysqlOps.GetRoleForReplication(ctx, request, response)
	}
	return mysqlOps.GetRoleForConsensus(ctx, request, response)
}

func (mysqlOps *MysqlOperations) GetRunningPort() int {
	return 0
}

func (mysqlOps *MysqlOperations) GetRoleForReplication(ctx context.Context, request *bindings.InvokeRequest, response *bindings.InvokeResponse) (string, error) {
	dcsStore := dcs.GetStore()
	if dcsStore == nil {
		return "", nil
	}
	k8sStore := dcsStore.(*dcs.KubernetesStore)
	cluster := k8sStore.GetClusterFromCache()
	if cluster == nil || !cluster.IsLocked() {
		return "", nil
	} else if !dcsStore.HasLock() {
		return SECONDARY, nil
	}

	getReadOnlySQL := `show global variables like 'read_only';`
	data, err := mysqlOps.query(ctx, getReadOnlySQL)
	if err != nil {
		mysqlOps.Logger.Infof("error executing %s: %v", getReadOnlySQL, err)
		return "", errors.Wrapf(err, "error executing %s", getReadOnlySQL)
	}

	queryRes := &QueryRes{}
	err = json.Unmarshal(data, queryRes)
	if err != nil {
		return "", errors.Errorf("parse query failed, err:%v", err)
	}

	for _, mapVal := range *queryRes {
		if mapVal["Variable_name"] == "read_only" {
			if mapVal["Value"].(string) == "OFF" {
				return PRIMARY, nil
			} else if mapVal["Value"].(string) == "ON" {
				return SECONDARY, nil
			}
		}
	}
	return "", errors.Errorf("parse query failed, no records")
}

func (mysqlOps *MysqlOperations) GetRoleForConsensus(ctx context.Context, request *bindings.InvokeRequest, response *bindings.InvokeResponse) (string, error) {
	sql := "select CURRENT_LEADER, ROLE, SERVER_ID  from information_schema.wesql_cluster_local"

	rows, err := mysqlOps.manager.DB.QueryContext(ctx, sql)
	if err != nil {
		mysqlOps.Logger.Infof("error executing %s: %v", sql, err)
		return "", errors.Wrapf(err, "error executing %s", sql)
	}

	defer func() {
		_ = rows.Close()
		_ = rows.Err()
	}()

	var curLeader string
	var role string
	var serverID string
	var isReady bool
	for rows.Next() {
		if err = rows.Scan(&curLeader, &role, &serverID); err != nil {
			mysqlOps.Logger.Errorf("Role query error: %v", err)
			return role, err
		}
		isReady = true
	}
	if isReady {
		return role, nil
	}
	return "", errors.Errorf("exec sql %s failed: no data returned", sql)
}

func (mysqlOps *MysqlOperations) ExecOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	result := OpsResult{}
	sql, ok := req.Metadata["sql"]
	if !ok || sql == "" {
		result["event"] = "ExecFailed"
		result["message"] = ErrNoSQL
		return result, nil
	}
	count, err := mysqlOps.exec(ctx, sql)
	if err != nil {
		mysqlOps.Logger.Infof("exec error: %v", err)
		result["event"] = OperationFailed
		result["message"] = err.Error()
	} else {
		result["event"] = OperationSuccess
		result["count"] = count
	}
	return result, nil
}

func (mysqlOps *MysqlOperations) GetLagOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	result := OpsResult{}
	slaveStatus := make([]SlaveStatus, 0)
	var err error

	if mysqlOps.OriRole == "" {
		mysqlOps.OriRole, err = mysqlOps.GetRole(ctx, req, resp)
		if err != nil {
			result["event"] = OperationFailed
			result["message"] = err.Error()
			return result, nil
		}
	}
	if mysqlOps.OriRole == LEADER {
		result["event"] = OperationSuccess
		result["lag"] = 0
		result["message"] = "This is leader instance, leader has no lag"
		return result, nil
	}

	sql := "show slave status"
	data, err := mysqlOps.query(ctx, sql)
	if err != nil {
		mysqlOps.Logger.Infof("GetLagOps error: %v", err)
		result["event"] = OperationFailed
		result["message"] = err.Error()
	} else {
		err = json.Unmarshal(data, &slaveStatus)
		if err != nil {
			result["event"] = OperationFailed
			result["message"] = err.Error()
		} else {
			result["event"] = OperationSuccess
			result["lag"] = slaveStatus[0].SecondsBehindMaster
		}
	}
	return result, nil
}

func (mysqlOps *MysqlOperations) QueryOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	result := OpsResult{}
	sql, ok := req.Metadata["sql"]
	if !ok || sql == "" {
		result["event"] = OperationFailed
		result["message"] = "no sql provided"
		return result, nil
	}
	data, err := mysqlOps.query(ctx, sql)
	if err != nil {
		mysqlOps.Logger.Infof("Query error: %v", err)
		result["event"] = OperationFailed
		result["message"] = err.Error()
	} else {
		result["event"] = OperationSuccess
		result["message"] = string(data)
	}
	return result, nil
}

func (mysqlOps *MysqlOperations) CheckStatusOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	rwSQL := fmt.Sprintf(`begin;
	create table if not exists kb_health_check(type int, check_ts bigint, primary key(type));
	insert into kb_health_check values(%d, now()) on duplicate key update check_ts = now();
	commit;
	select check_ts from kb_health_check where type=%d limit 1;`, CheckStatusType, CheckStatusType)
	roSQL := fmt.Sprintf(`select check_ts from kb_health_check where type=%d limit 1;`, CheckStatusType)
	var err error
	var data []byte
	switch mysqlOps.DBRoles[strings.ToLower(mysqlOps.OriRole)] {
	case ReadWrite:
		var count int64
		count, err = mysqlOps.exec(ctx, rwSQL)
		data = []byte(strconv.FormatInt(count, 10))
	case Readonly:
		data, err = mysqlOps.query(ctx, roSQL)
	default:
		msg := fmt.Sprintf("unknown access mode for role %s: %v", mysqlOps.OriRole, mysqlOps.DBRoles)
		mysqlOps.Logger.Info(msg)
		data = []byte(msg)
	}

	result := OpsResult{}
	if err != nil {
		mysqlOps.Logger.Infof("CheckStatus error: %v", err)
		result["event"] = OperationFailed
		result["message"] = err.Error()
		if mysqlOps.CheckStatusFailedCount%mysqlOps.FailedEventReportFrequency == 0 {
			mysqlOps.Logger.Infof("status check failed %v times continuously", mysqlOps.CheckStatusFailedCount)
			resp.Metadata[StatusCode] = OperationFailedHTTPCode
		}
		mysqlOps.CheckStatusFailedCount++
	} else {
		result["event"] = OperationSuccess
		result["message"] = string(data)
		mysqlOps.CheckStatusFailedCount = 0
	}
	return result, nil
}

func (mysqlOps *MysqlOperations) query(ctx context.Context, query string) ([]byte, error) {
	var (
		rows *sql.Rows
		err  error
	)
	mysqlOps.Logger.Debugf("query: %s", query)

	rows, err = mysqlOps.manager.DB.QueryContext(ctx, query)

	if err != nil {
		return nil, errors.Wrapf(err, "error executing %s", query)
	}

	defer func() {
		_ = rows.Close()
		_ = rows.Err()
	}()

	result, err := mysqlOps.jsonify(rows)
	if err != nil {
		return nil, errors.Wrapf(err, "error marshalling query result for %s", query)
	}
	return result, nil
}

func (mysqlOps *MysqlOperations) queryWithDB(ctx context.Context, query string, db string) ([]byte, error) {
	var (
		rows *sql.Rows
		err  error
	)
	mysqlOps.Logger.Debugf("query: %s", query)
	if len(db) == 0 {
		return nil, errors.New("empty db name")
	}
	conn, err := mysqlOps.manager.ConnectDB(ctx, db)
	if err != nil {
		return nil, err
	}
	rows, err = conn.QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrapf(err, "error executing %s", query)
	}

	defer func() {
		_ = rows.Close()
		_ = rows.Err()
		conn.Close()
	}()

	result, err := mysqlOps.jsonify(rows)
	if err != nil {
		return nil, errors.Wrapf(err, "error marshalling query result for %s", query)
	}
	return result, nil
}

func (mysqlOps *MysqlOperations) exec(ctx context.Context, query string) (int64, error) {
	var (
		res sql.Result
		err error
	)
	mysqlOps.Logger.Debugf("exec: %s", query)

	res, err = mysqlOps.manager.DB.ExecContext(ctx, query)

	if err != nil {
		return 0, errors.Wrapf(err, "error executing %s", query)
	}
	return res.RowsAffected()
}

func (mysqlOps *MysqlOperations) execWithDB(ctx context.Context, query string, db string) (int64, error) {
	var (
		res sql.Result
		err error
	)
	mysqlOps.Logger.Debugf("exec: %s", query)

	conn, err := mysqlOps.manager.ConnectDB(ctx, db)
	if err != nil {
		return 0, err
	}

	res, err = conn.ExecContext(ctx, query)

	if err != nil {
		return 0, errors.Wrapf(err, "error executing %s", query)
	}
	defer func() {
		conn.Close()
	}()

	return res.RowsAffected()
}

func (mysqlOps *MysqlOperations) jsonify(rows *sql.Rows) ([]byte, error) {
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	var ret []interface{}
	for rows.Next() {
		values := prepareValues(columnTypes)
		err := rows.Scan(values...)
		if err != nil {
			return nil, err
		}
		r := mysqlOps.convert(columnTypes, values)
		ret = append(ret, r)
	}
	return json.Marshal(ret)
}

func prepareValues(columnTypes []*sql.ColumnType) []interface{} {
	types := make([]reflect.Type, len(columnTypes))
	for i, tp := range columnTypes {
		types[i] = tp.ScanType()
	}
	values := make([]interface{}, len(columnTypes))
	for i := range values {
		switch types[i].Kind() {
		case reflect.String, reflect.Interface:
			values[i] = &sql.NullString{}
		case reflect.Bool:
			values[i] = &sql.NullBool{}
		case reflect.Float64:
			values[i] = &sql.NullFloat64{}
		case reflect.Int16, reflect.Uint16:
			values[i] = &sql.NullInt16{}
		case reflect.Int32, reflect.Uint32:
			values[i] = &sql.NullInt32{}
		case reflect.Int64, reflect.Uint64:
			values[i] = &sql.NullInt64{}
		default:
			values[i] = reflect.New(types[i]).Interface()
		}
	}
	return values
}

func (mysqlOps *MysqlOperations) convert(columnTypes []*sql.ColumnType, values []interface{}) map[string]interface{} {
	r := map[string]interface{}{}
	for i, ct := range columnTypes {
		value := values[i]
		switch v := values[i].(type) {
		case driver.Valuer:
			if vv, err := v.Value(); err == nil {
				value = interface{}(vv)
			} else {
				mysqlOps.Logger.Warnf("error to convert value: %v", err)
			}
		case *sql.RawBytes:
			// special case for sql.RawBytes, see https://github.com/go-sql-driver/mysql/blob/master/fields.go#L178
			switch ct.DatabaseTypeName() {
			case "VARCHAR", "CHAR", "TEXT", "LONGTEXT":
				value = string(*v)
			}
		}
		if value != nil {
			r[ct.Name()] = value
		}
	}
	return r
}

// InternalQuery is used for internal query, implements BaseInternalOps interface
func (mysqlOps *MysqlOperations) InternalQuery(ctx context.Context, sql string) ([]byte, error) {
	return mysqlOps.query(ctx, sql)
}

// InternalExec is used for internal execution, implements BaseInternalOps interface
func (mysqlOps *MysqlOperations) InternalExec(ctx context.Context, sql string) (int64, error) {
	return mysqlOps.exec(ctx, sql)
}

// InternalQueryWithDB is used for internal query, implements BaseInternalOps interface
func (mysqlOps *MysqlOperations) InternalQueryWithDB(ctx context.Context, sql string, db string) ([]byte, error) {
	return mysqlOps.queryWithDB(ctx, sql, db)
}

// InternalExecWithDB is used for internal execution, implements BaseInternalOps interface
func (mysqlOps *MysqlOperations) InternalExecWithDB(ctx context.Context, sql string, db string) (int64, error) {
	return mysqlOps.execWithDB(ctx, sql, db)
}

// GetLogger is used for getting logger, implements BaseInternalOps interface
func (mysqlOps *MysqlOperations) GetLogger() logger.Logger {
	return mysqlOps.Logger
}

func (mysqlOps *MysqlOperations) listUsersOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	sqlTplRend := func(user UserInfo) string {
		return listUserTpl
	}

	return QueryObject(ctx, mysqlOps, req, ListUsersOp, sqlTplRend, nil, UserInfo{})
}

func (mysqlOps *MysqlOperations) listSystemAccountsOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	sqlTplRend := func(user UserInfo) string {
		return listSystemAccountsTpl
	}
	dataProcessor := func(data interface{}) (interface{}, error) {
		var users []UserInfo
		if err := json.Unmarshal(data.([]byte), &users); err != nil {
			return nil, err
		}
		userNames := make([]string, 0)
		for _, user := range users {
			userNames = append(userNames, user.UserName)
		}
		if jsonData, err := json.Marshal(userNames); err != nil {
			return nil, err
		} else {
			return string(jsonData), nil
		}
	}
	return QueryObject(ctx, mysqlOps, req, ListSystemAccountsOp, sqlTplRend, dataProcessor, UserInfo{})
}

func (mysqlOps *MysqlOperations) describeUserOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
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

			if len(roles) == 0 {
				return nil, fmt.Errorf("no such user %s", object.UserName)
			}

			users := make([]UserInfo, 0)
			var userName string
			for _, roleMap := range roles {
				for k, v := range roleMap {
					if len(userName) == 0 {
						userName = strings.TrimPrefix(strings.TrimSuffix(k, "@%"), "Grants for ")
					}
					mysqlRoleType, database := mysqlOps.priv2Role(v)
					if mysqlRoleType == NoPrivileges || mysqlRoleType == InvalidRole {
						continue
					}
					users = append(users, UserInfo{UserName: userName, RoleName: string(mysqlRoleType), Database: database})
				}
			}
			// if no privileges, return empty
			if len(users) == 0 {
				users = append(users, UserInfo{UserName: object.UserName, RoleName: string(NoPrivileges), Database: ""})
			}

			if jsonData, err := json.Marshal(users); err != nil {
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

	return QueryObject(ctx, mysqlOps, req, DescribeUserOp, sqlTplRend, dataProcessor, object)
}

func (mysqlOps *MysqlOperations) createUserOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
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

	return ExecuteObject(ctx, mysqlOps, req, CreateUserOp, sqlTplRend, msgTplRend, object)
}

func (mysqlOps *MysqlOperations) deleteUserOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
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

	return ExecuteObject(ctx, mysqlOps, req, DeleteUserOp, sqlTplRend, msgTplRend, object)
}

func (mysqlOps *MysqlOperations) grantUserRoleOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		succMsgTpl = "role %s granted to user: %s"
	)
	return mysqlOps.managePrivillege(ctx, req, GrantUserRoleOp, grantTpl, succMsgTpl)
}

func (mysqlOps *MysqlOperations) revokeUserRoleOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		succMsgTpl = "role %s revoked from user: %s"
	)
	return mysqlOps.managePrivillege(ctx, req, RevokeUserRoleOp, revokeTpl, succMsgTpl)
}

func (mysqlOps *MysqlOperations) managePrivillege(ctx context.Context, req *bindings.InvokeRequest, op bindings.OperationKind, sqlTpl string, succMsgTpl string) (OpsResult, error) {
	var (
		object     = UserInfo{}
		sqlTplRend = func(user UserInfo) string {
			// render sql stmts
			priv, _ := mysqlOps.role2Priv(user.RoleName)
			role := fmt.Sprintf("%s on %s.*", priv, user.Database)
			// update privilege
			sql := fmt.Sprintf(sqlTpl, role, user.UserName)
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
	return ExecuteObject(ctx, mysqlOps, req, op, sqlTplRend, msgTplRend, object)
}

func (mysqlOps *MysqlOperations) role2Priv(roleName string) (string, error) {
	roleType := String2RoleType(roleName)
	switch roleType {
	case SuperUserRole:
		return superUserPriv, nil
	case ReadWriteRole:
		return readWritePriv, nil
	case ReadOnlyRole:
		return readOnlyRPriv, nil
	default:
		return "", fmt.Errorf("role name: %s is not supported", roleName)
	}
}

func (mysqlOps *MysqlOperations) priv2Role(privileges string) (RoleType, string) {

	tokenize := func(priv string) (string, string) {
		// tokenize privilege with GRANT and ON
		// e.g. GRANT SELECT, INSERT, UPDATE, DELETE ON *.* TO 'user'@'%'
		// => [GRANT SELECT, INSERT, UPDATE, DELETE, ON *.* TO 'user'@'%']
		// e.g. GRANT ALL PRIVILEGES ON *.* TO 'user'@'%'
		// => [GRANT ALL PRIVILEGES, ON *.* TO 'user'@'%']
		// split by ON
		// resutls = ["GRANT SELECT, INSERT, UPDATE, DELETE", "*.* TO 'user'@'%'"]
		results := strings.Split(priv, " ON ")
		if len(results) != 2 {
			return "", ""
		}
		// split by GRANT
		// privs := ["", "SELECT, INSERT, UPDATE, DELETE"]
		privs := strings.Split(results[0], "GRANT ")
		if len(privs) != 2 {
			return "", ""
		}

		// split by TO
		// targets := ["*.*", "'user'@'%'"]
		targets := strings.Split(results[1], " TO ")
		database := targets[0]
		if database == "*.*" {
			database = "ALL DATABASES"
		} else {
			database = strings.Split(database, ".")[0]
		}
		return privs[1], database
	}

	priv, db := tokenize(privileges)
	if len(priv) == 0 {
		return InvalidRole, ""
	}
	switch priv {
	case noPriv:
		return NoPrivileges, db
	case readOnlyRPriv:
		return ReadOnlyRole, db
	case readWritePriv:
		return ReadWriteRole, db
	case superUserPriv:
		return SuperUserRole, db
	default:
		return CustomizedRole, db
	}
}
