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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/internal/constant"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
	. "github.com/apecloud/kubeblocks/lorry/binding"
	"github.com/apecloud/kubeblocks/lorry/component"
	"github.com/apecloud/kubeblocks/lorry/component/mysql"
	"github.com/apecloud/kubeblocks/lorry/dcs"
	. "github.com/apecloud/kubeblocks/lorry/util"
)

// MysqlOperations represents MySQL output bindings.
type MysqlOperations struct {
	BaseOperations
}

type QueryRes []map[string]interface{}

var _ BaseInternalOps = &MysqlOperations{}

const (
	superUserPriv = "SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, RELOAD, SHUTDOWN, PROCESS, FILE, REFERENCES, INDEX, ALTER, SHOW DATABASES, SUPER, CREATE TEMPORARY TABLES, LOCK TABLES, EXECUTE, REPLICATION SLAVE, REPLICATION CLIENT, CREATE VIEW, SHOW VIEW, CREATE ROUTINE, ALTER ROUTINE, CREATE USER, EVENT, TRIGGER, CREATE TABLESPACE, CREATE ROLE, DROP ROLE ON *.*"
	readWritePriv = "SELECT, INSERT, UPDATE, DELETE ON *.*"
	readOnlyRPriv = "SELECT ON *.*"
	noPriv        = "USAGE ON *.*"

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
func NewMysql() *MysqlOperations {
	logger := ctrl.Log.WithName("Mysql")
	return &MysqlOperations{BaseOperations: BaseOperations{Logger: logger}}
}

// Init initializes the MySQL binding.
func (mysqlOps *MysqlOperations) Init(metadata component.Properties) error {
	mysqlOps.Logger.Info("Initializing MySQL binding")
	mysqlOps.BaseOperations.Init(metadata)
	config, err := mysql.NewConfig(metadata)
	if err != nil {
		mysqlOps.Logger.Error(err, "MySQL config initialize failed")
		return err
	}

	var manager component.DBManager
	workloadType := viper.GetString(constant.KBEnvWorkloadType)
	if strings.EqualFold(workloadType, Consensus) {
		manager, err = mysql.NewWesqlManager(mysqlOps.Logger)
		if err != nil {
			mysqlOps.Logger.Error(err, "WeSQL DB Manager initialize failed")
			return err
		}
	} else {
		manager, err = mysql.NewManager(mysqlOps.Logger)
		if err != nil {
			mysqlOps.Logger.Error(err, "MySQL DB Manager initialize failed")
			return err
		}

	}

	mysqlOps.Manager = manager
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

func (mysqlOps *MysqlOperations) GetRole(ctx context.Context, request *ProbeRequest, response *ProbeResponse) (string, error) {
	workloadType := viper.GetString(constant.KBEnvWorkloadType)
	if strings.EqualFold(workloadType, Replication) {
		dcsStore := dcs.GetStore()
		if dcsStore == nil {
			return "", nil
		}
		k8sStore := dcsStore.(*dcs.KubernetesStore)
		cluster := k8sStore.GetClusterFromCache()
		if cluster == nil || !cluster.IsLocked() {
			return "", nil
		}

		manager := mysqlOps.Manager.(*mysql.Manager)
		return manager.GetRole(ctx)
	}
	manager := mysqlOps.Manager.(*mysql.WesqlManager)
	return manager.GetRole(ctx)
}

func (mysqlOps *MysqlOperations) GetRunningPort() int {
	return 0
}

func (mysqlOps *MysqlOperations) GetRoleForReplication(ctx context.Context, request *ProbeRequest, response *ProbeResponse) (string, error) {
	dcsStore := dcs.GetStore()
	if dcsStore == nil {
		return "", nil
	}
	k8sStore := dcsStore.(*dcs.KubernetesStore)
	cluster := k8sStore.GetClusterFromCache()
	if cluster == nil || !cluster.IsLocked() {
		return "", nil
	} else if !dcsStore.HasLease() {
		return SECONDARY, nil
	}

	return PRIMARY, nil
}

func (mysqlOps *MysqlOperations) GetRoleForConsensus(ctx context.Context, request *ProbeRequest, response *ProbeResponse) (string, error) {
	sql := "select CURRENT_LEADER, ROLE, SERVER_ID  from information_schema.wesql_cluster_local"
	manager := mysqlOps.Manager.(*mysql.WesqlManager)
	rows, err := manager.DB.QueryContext(ctx, sql)
	if err != nil {
		mysqlOps.Logger.Error(err, fmt.Sprintf("error executing %s", sql))
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
			mysqlOps.Logger.Error(err, "Role query error")
			return role, err
		}
		isReady = true
	}
	if isReady {
		return role, nil
	}
	return "", errors.Errorf("exec sql %s failed: no data returned", sql)
}

func (mysqlOps *MysqlOperations) ExecOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	result := OpsResult{}
	sql, ok := req.Metadata["sql"]
	if !ok || sql == "" {
		result["event"] = "ExecFailed"
		result["message"] = ErrNoSQL
		return result, nil
	}

	manager, ok := mysqlOps.Manager.(*mysql.Manager)
	if !ok {
		manager = &mysqlOps.Manager.(*mysql.WesqlManager).Manager
	}
	count, err := manager.Exec(ctx, sql)
	if err != nil {
		mysqlOps.Logger.Error(err, "sql exec error")
		result["event"] = OperationFailed
		result["message"] = err.Error()
	} else {
		result["event"] = OperationSuccess
		result["count"] = count
	}
	return result, nil
}

func (mysqlOps *MysqlOperations) GetLagOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
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

	manager, ok := mysqlOps.Manager.(*mysql.Manager)
	if !ok {
		manager = &mysqlOps.Manager.(*mysql.WesqlManager).Manager
	}
	data, err := manager.Query(ctx, sql)
	if err != nil {
		mysqlOps.Logger.Error(err, "GetLagOps error")
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

func (mysqlOps *MysqlOperations) QueryOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	result := OpsResult{}
	sql, ok := req.Metadata["sql"]
	if !ok || sql == "" {
		result["event"] = OperationFailed
		result["message"] = "no sql provided"
		return result, nil
	}
	manager, ok := mysqlOps.Manager.(*mysql.Manager)
	if !ok {
		manager = &mysqlOps.Manager.(*mysql.WesqlManager).Manager
	}
	data, err := manager.Query(ctx, sql)
	if err != nil {
		mysqlOps.Logger.Error(err, "Query error")
		result["event"] = OperationFailed
		result["message"] = err.Error()
	} else {
		result["event"] = OperationSuccess
		result["message"] = string(data)
	}
	return result, nil
}

func (mysqlOps *MysqlOperations) CheckStatusOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	rwSQL := fmt.Sprintf(`begin;
	create table if not exists kb_health_check(type int, check_ts bigint, primary key(type));
	insert into kb_health_check values(%d, now()) on duplicate key update check_ts = now();
	commit;
	select check_ts from kb_health_check where type=%d limit 1;`, component.CheckStatusType, component.CheckStatusType)
	roSQL := fmt.Sprintf(`select check_ts from kb_health_check where type=%d limit 1;`, component.CheckStatusType)
	var err error
	var data []byte

	manager, ok := mysqlOps.Manager.(*mysql.Manager)
	if !ok {
		manager = &mysqlOps.Manager.(*mysql.WesqlManager).Manager
	}
	switch mysqlOps.DBRoles[strings.ToLower(mysqlOps.OriRole)] {
	case ReadWrite:
		var count int64
		count, err = manager.Exec(ctx, rwSQL)
		data = []byte(strconv.FormatInt(count, 10))
	case Readonly:
		data, err = manager.Query(ctx, roSQL)
	default:
		msg := fmt.Sprintf("unknown access mode for role %s: %v", mysqlOps.OriRole, mysqlOps.DBRoles)
		mysqlOps.Logger.Info(msg)
		data = []byte(msg)
	}

	result := OpsResult{}
	if err != nil {
		mysqlOps.Logger.Error(err, "CheckStatus error")
		result["event"] = OperationFailed
		result["message"] = err.Error()
		if mysqlOps.CheckStatusFailedCount%mysqlOps.FailedEventReportFrequency == 0 {
			mysqlOps.Logger.Info("status check failed continuously", "times", mysqlOps.CheckStatusFailedCount)
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

// InternalQuery is used for internal query, implements BaseInternalOps interface
func (mysqlOps *MysqlOperations) InternalQuery(ctx context.Context, sql string) ([]byte, error) {
	manager, ok := mysqlOps.Manager.(*mysql.Manager)
	if !ok {
		manager = &mysqlOps.Manager.(*mysql.WesqlManager).Manager
	}
	return manager.Query(ctx, sql)
}

// InternalExec is used for internal execution, implements BaseInternalOps interface
func (mysqlOps *MysqlOperations) InternalExec(ctx context.Context, sql string) (int64, error) {
	manager, ok := mysqlOps.Manager.(*mysql.Manager)
	if !ok {
		manager = &mysqlOps.Manager.(*mysql.WesqlManager).Manager
	}
	return manager.Exec(ctx, sql)
}

// GetLogger is used for getting logger, implements BaseInternalOps interface
func (mysqlOps *MysqlOperations) GetLogger() logr.Logger {
	return mysqlOps.Logger
}

func (mysqlOps *MysqlOperations) listUsersOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	sqlTplRend := func(user UserInfo) string {
		return listUserTpl
	}

	return QueryObject(ctx, mysqlOps, req, ListUsersOp, sqlTplRend, nil, UserInfo{})
}

func (mysqlOps *MysqlOperations) listSystemAccountsOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
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

func (mysqlOps *MysqlOperations) describeUserOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
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
					mysqlRoleType := mysqlOps.priv2Role(strings.TrimPrefix(v, "GRANT "))
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

	return QueryObject(ctx, mysqlOps, req, DescribeUserOp, sqlTplRend, dataProcessor, object)
}

func (mysqlOps *MysqlOperations) createUserOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
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

func (mysqlOps *MysqlOperations) deleteUserOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
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

func (mysqlOps *MysqlOperations) grantUserRoleOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	var (
		succMsgTpl = "role %s granted to user: %s"
	)
	return mysqlOps.managePrivillege(ctx, req, GrantUserRoleOp, grantTpl, succMsgTpl)
}

func (mysqlOps *MysqlOperations) revokeUserRoleOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	var (
		succMsgTpl = "role %s revoked from user: %s"
	)
	return mysqlOps.managePrivillege(ctx, req, RevokeUserRoleOp, revokeTpl, succMsgTpl)
}

func (mysqlOps *MysqlOperations) managePrivillege(ctx context.Context, req *ProbeRequest, op OperationKind, sqlTpl string, succMsgTpl string) (OpsResult, error) {
	var (
		object     = UserInfo{}
		sqlTplRend = func(user UserInfo) string {
			// render sql stmts
			roleDesc, _ := mysqlOps.role2Priv(user.RoleName)
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
	}
	return "", fmt.Errorf("role name: %s is not supported", roleName)
}

func (mysqlOps *MysqlOperations) priv2Role(priv string) RoleType {
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
