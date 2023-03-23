/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mysql

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	. "github.com/apecloud/kubeblocks/cmd/probe/util"
)

// MysqlOperations represents MySQL output bindings.
type MysqlOperations struct {
	db *sql.DB
	mu sync.Mutex
	BaseOperations
}

const (
	// configurations to connect to Mysql, either a data source name represent by URL.
	connectionURLKey = "url"

	// To connect to MySQL running in Azure over SSL you have to download a
	// SSL certificate. If this is provided the driver will connect using
	// SSL. If you have disable SSL you can leave this empty.
	// When the user provides a pem path their connection string must end with
	// &tls=custom
	// The connection string should be in the following format
	// "%s:%s@tcp(%s:3306)/%s?allowNativePasswords=true&tls=custom",'myadmin@mydemoserver', 'yourpassword', 'mydemoserver.mysql.database.azure.com', 'targetdb'.
	pemPathKey = "pemPath"

	// other general settings for DB connections.
	maxIdleConnsKey    = "maxIdleConns"
	maxOpenConnsKey    = "maxOpenConns"
	connMaxLifetimeKey = "connMaxLifetime"
	connMaxIdleTimeKey = "connMaxIdleTime"
)

var (
	defaultDBPort = 3306
	dbUser        = "root"
	dbPasswd      = ""
)

// NewMysql returns a new MySQL output binding.
func NewMysql(logger logger.Logger) bindings.OutputBinding {
	return &MysqlOperations{BaseOperations: BaseOperations{Logger: logger}}
}

// Init initializes the MySQL binding.
func (mysqlOps *MysqlOperations) Init(metadata bindings.Metadata) error {
	mysqlOps.BaseOperations.Init(metadata)
	if viper.IsSet("KB_SERVICE_USER") {
		dbUser = viper.GetString("KB_SERVICE_USER")
	}

	if viper.IsSet("KB_SERVICE_PASSWORD") {
		dbPasswd = viper.GetString("KB_SERVICE_PASSWORD")
	}

	mysqlOps.Logger.Debug("Initializing MySQL binding")
	mysqlOps.DBType = "mysql"
	mysqlOps.InitIfNeed = mysqlOps.initIfNeed
	mysqlOps.BaseOperations.GetRole = mysqlOps.GetRole
	mysqlOps.DBPort = mysqlOps.GetRunningPort()
	mysqlOps.RegisterOperation(GetRoleOperation, mysqlOps.GetRoleOps)
	mysqlOps.RegisterOperation(GetLagOperation, mysqlOps.GetLagOps)
	mysqlOps.RegisterOperation(CheckStatusOperation, mysqlOps.CheckStatusOps)
	mysqlOps.RegisterOperation(ExecOperation, mysqlOps.ExecOps)
	mysqlOps.RegisterOperation(QueryOperation, mysqlOps.QueryOps)

	// following are ops for account management
	mysqlOps.RegisterOperation(ListUsersOp, mysqlOps.ListUsersOps)
	mysqlOps.RegisterOperation(CreateUserOp, mysqlOps.CreateUserOps)
	mysqlOps.RegisterOperation(DeleteUserOp, mysqlOps.DeleteUserOps)
	mysqlOps.RegisterOperation(DescribeUserOp, mysqlOps.DescribeUserOps)
	mysqlOps.RegisterOperation(GrantUserRoleOp, mysqlOps.GrantUserRoleOps)
	mysqlOps.RegisterOperation(RevokeUserRoleOp, mysqlOps.RevokeUserRoleOps)
	return nil
}

func (mysqlOps *MysqlOperations) initIfNeed() bool {
	if mysqlOps.db == nil {
		go func() {
			err := mysqlOps.InitDelay()
			mysqlOps.Logger.Errorf("MySQl connection init failed: %v", err)
		}()
		return true
	}
	return false
}

func (mysqlOps *MysqlOperations) InitDelay() error {
	mysqlOps.mu.Lock()
	defer mysqlOps.mu.Unlock()
	if mysqlOps.db != nil {
		return nil
	}

	p := mysqlOps.Metadata.Properties
	url, ok := p[connectionURLKey]
	if !ok || url == "" {
		return fmt.Errorf("missing MySQL connection string")
	}

	db, err := initDB(url, mysqlOps.Metadata.Properties[pemPathKey])
	if err != nil {
		return err
	}

	err = propertyToInt(p, maxIdleConnsKey, db.SetMaxIdleConns)
	if err != nil {
		return err
	}

	err = propertyToInt(p, maxOpenConnsKey, db.SetMaxOpenConns)
	if err != nil {
		return err
	}

	err = propertyToDuration(p, connMaxIdleTimeKey, db.SetConnMaxIdleTime)
	if err != nil {
		return err
	}

	err = propertyToDuration(p, connMaxLifetimeKey, db.SetConnMaxLifetime)
	if err != nil {
		return err
	}

	// test if db is ready to connect or not
	err = db.Ping()
	if err != nil {
		mysqlOps.Logger.Infof("unable to ping the DB")
		return errors.Wrap(err, "unable to ping the DB")
	}
	mysqlOps.db = db

	return nil
}

func (mysqlOps *MysqlOperations) GetRunningPort() int {
	p := mysqlOps.Metadata.Properties
	url, ok := p[connectionURLKey]
	if !ok || url == "" {
		return defaultDBPort
	}

	config, err := mysql.ParseDSN(url)
	if err != nil {
		return defaultDBPort
	}
	index := strings.LastIndex(config.Addr, ":")
	if index < 0 {
		return defaultDBPort
	}
	port, err := strconv.Atoi(config.Addr[index+1:])
	if err != nil {
		return defaultDBPort
	}

	return port
}

func (mysqlOps *MysqlOperations) GetRole(ctx context.Context, request *bindings.InvokeRequest, response *bindings.InvokeResponse) (string, error) {
	sql := "select CURRENT_LEADER, ROLE, SERVER_ID  from information_schema.wesql_cluster_local"

	// sql exec timeout need to be less than httpget's timeout which default is 1s.
	ctx1, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	rows, err := mysqlOps.db.QueryContext(ctx1, sql)
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
	for rows.Next() {
		if err = rows.Scan(&curLeader, &role, &serverID); err != nil {
			mysqlOps.Logger.Errorf("Role query error: %v", err)
			return role, err
		}
	}
	return role, nil
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
	sql := "show slave status"
	_, err := mysqlOps.query(ctx, sql)
	if err != nil {
		mysqlOps.Logger.Infof("GetLagOps error: %v", err)
		result["event"] = OperationFailed
		result["message"] = err.Error()
	} else {
		result["event"] = OperationSuccess
		result["lag"] = 0
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

// CheckStatusOps design details: https://infracreate.feishu.cn/wiki/wikcndch7lMZJneMnRqaTvhQpwb#doxcnOUyQ4Mu0KiUo232dOr5aad
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
			mysqlOps.Logger.Infof("status checks failed %v times continuously", mysqlOps.CheckStatusFailedCount)
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

func propertyToInt(props map[string]string, key string, setter func(int)) error {
	if v, ok := props[key]; ok {
		if i, err := strconv.Atoi(v); err == nil {
			setter(i)
		} else {
			return errors.Wrapf(err, "error converitng %s:%s to int", key, v)
		}
	}

	return nil
}

func propertyToDuration(props map[string]string, key string, setter func(time.Duration)) error {
	if v, ok := props[key]; ok {
		if d, err := time.ParseDuration(v); err == nil {
			setter(d)
		} else {
			return errors.Wrapf(err, "error converitng %s:%s to time duration", key, v)
		}
	}

	return nil
}

func initDB(url, pemPath string) (*sql.DB, error) {
	config, err := mysql.ParseDSN(url)
	if err != nil {
		return nil, errors.Wrapf(err, "illegal Data Source Name (DNS) specified by %s", connectionURLKey)
	}
	config.User = dbUser
	config.Passwd = dbPasswd

	if pemPath != "" {
		rootCertPool := x509.NewCertPool()
		pem, err := os.ReadFile(pemPath)
		if err != nil {
			return nil, errors.Wrapf(err, "Error reading PEM file from %s", pemPath)
		}

		ok := rootCertPool.AppendCertsFromPEM(pem)
		if !ok {
			return nil, fmt.Errorf("failed to append PEM")
		}

		err = mysql.RegisterTLSConfig("custom", &tls.Config{RootCAs: rootCertPool, MinVersion: tls.VersionTLS12})
		if err != nil {
			return nil, errors.Wrap(err, "Error register TLS config")
		}
	}

	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		return nil, errors.Wrap(err, "error opening DB connection")
	}

	return db, nil
}

func (mysqlOps *MysqlOperations) query(ctx context.Context, sql string) ([]byte, error) {
	mysqlOps.Logger.Debugf("query: %s", sql)
	rows, err := mysqlOps.db.QueryContext(ctx, sql)
	if err != nil {
		return nil, errors.Wrapf(err, "error executing %s", sql)
	}
	defer func() {
		_ = rows.Close()
		_ = rows.Err()
	}()
	result, err := mysqlOps.jsonify(rows)
	if err != nil {
		return nil, errors.Wrapf(err, "error marshalling query result for %s", sql)
	}
	return result, nil
}

func (mysqlOps *MysqlOperations) exec(ctx context.Context, sql string) (int64, error) {
	mysqlOps.Logger.Debugf("exec: %s", sql)
	res, err := mysqlOps.db.ExecContext(ctx, sql)
	if err != nil {
		return 0, errors.Wrapf(err, "error executing %s", sql)
	}
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
		values[i] = reflect.New(types[i]).Interface()
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

func (mysqlOps *MysqlOperations) ListUsersOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	const (
		opsKind     = ListUsersOp
		listUserTpl = "SELECT user AS userName, user_attributes->>\"$.metadata.kbroles\" AS roleName FROM mysql.user;"
	)
	sqlTplRend := func(user UserInfo) string {
		return listUserTpl
	}
	dataProcessor := func(data []byte) (interface{}, error) {
		users := make([]*UserInfo, 0)
		err := json.Unmarshal(data, &users)
		if err != nil {
			return nil, err
		}
		for _, user := range users {
			if user.Expired == "N" {
				user.Expired = "F"
			} else {
				user.Expired = "T"
			}
			roles := []string{}
			if user.RoleName != "" {
				if err = json.Unmarshal([]byte(user.RoleName), &roles); err != nil {
					return nil, err
				}
				user.RoleName = strings.Join(roles, ",")
			}
		}
		if jsonData, err := json.Marshal(users); err != nil {
			return nil, err
		} else {
			return string(jsonData), nil
		}
	}
	return mysqlOps.queryUser(ctx, req, opsKind, nil, sqlTplRend, dataProcessor)
}

func (mysqlOps *MysqlOperations) DescribeUserOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	const (
		opsKind     = DescribeUserOp
		descUserTpl = "SELECT user AS userName, password_expired as expired, user_attributes->>\"$.metadata.kbroles\" AS roleName FROM mysql.user WHERE USER ='%s';"
	)
	validFn := func(user UserInfo) error {
		if len(user.UserName) == 0 {
			return ErrNoUserName
		}
		return nil
	}

	dataProcessor := func(data []byte) (interface{}, error) {
		users := make([]*UserInfo, 0)
		err := json.Unmarshal(data, &users)
		if err != nil {
			return nil, err
		}
		if len(users) == 0 {
			return nil, ErrNoUserFound
		}
		for _, user := range users {
			if user.Expired == "N" {
				user.Expired = "F"
			} else {
				user.Expired = "T"
			}
			roles := []string{}
			if user.RoleName != "" {
				if err = json.Unmarshal([]byte(user.RoleName), &roles); err != nil {
					return nil, err
				}
				user.RoleName = strings.Join(roles, ",")
			}
		}
		if jsonData, err := json.Marshal(users); err != nil {
			return nil, err
		} else {
			return string(jsonData), nil
		}
	}

	sqlTplRend := func(user UserInfo) string {
		return fmt.Sprintf(descUserTpl, user.UserName)
	}
	return mysqlOps.queryUser(ctx, req, opsKind, validFn, sqlTplRend, dataProcessor)
}

func (mysqlOps *MysqlOperations) CreateUserOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	const (
		opsKind       = CreateUserOp
		createUserTpl = "CREATE USER '%s'@'%%' IDENTIFIED BY '%s';"
	)

	validFn := func(user UserInfo) error {
		if len(user.UserName) == 0 {
			return ErrNoUserName
		}
		if len(user.Password) == 0 {
			return ErrNoPassword
		}
		return nil
	}

	sqlTplRend := func(user UserInfo) string {
		return fmt.Sprintf(createUserTpl, user.UserName, user.Password)
	}

	msgTplRend := func(user UserInfo) string {
		return fmt.Sprintf("created user: %s, with password: %s", user.UserName, user.Password)
	}

	return mysqlOps.execUser(ctx, req, opsKind, validFn, sqlTplRend, msgTplRend, nil)
}

func (mysqlOps *MysqlOperations) DeleteUserOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	const (
		opsKind       = DeleteUserOp
		deleteUserTpl = "DROP USER IF EXISTS '%s'@'%%';"
	)

	validFn := func(user UserInfo) error {
		if len(user.UserName) == 0 {
			return ErrNoUserName
		}
		return nil
	}
	sqlTplRend := func(user UserInfo) string {
		return fmt.Sprintf(deleteUserTpl, user.UserName)
	}
	msgTplRend := func(user UserInfo) string {
		return fmt.Sprintf("deleted user: %s", user.UserName)
	}
	return mysqlOps.execUser(ctx, req, opsKind, validFn, sqlTplRend, msgTplRend, nil)
}

func (mysqlOps *MysqlOperations) GrantUserRoleOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		grantTpl   = "GRANT %s TO '%s'@'%%';"
		succMsgTpl = "role %s granted to user: %s"
	)
	return mysqlOps.managePrivillege(ctx, req, GrantUserRoleOp, grantTpl, succMsgTpl)
}

func (mysqlOps *MysqlOperations) RevokeUserRoleOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		revokeTpl  = "REVOKE %s FROM '%s'@'%%';"
		succMsgTpl = "role %s revoked from user: %s"
	)
	return mysqlOps.managePrivillege(ctx, req, RevokeUserRoleOp, revokeTpl, succMsgTpl)
}

func (mysqlOps *MysqlOperations) managePrivillege(ctx context.Context, req *bindings.InvokeRequest, op bindings.OperationKind, sqlTpl string, succMsgTpl string) (OpsResult, error) {
	validFn := func(user UserInfo) error {
		if len(user.UserName) == 0 {
			return ErrNoUserName
		}
		if len(user.RoleName) == 0 {
			return ErrNoRoleName
		}
		roles := []string{ReadOnlyRole, ReadWriteRole, SuperUserRole}
		if !slices.Contains(roles, strings.ToLower(user.RoleName)) {
			return ErrInvalidRoleName
		}
		return nil
	}
	sqlTplRend := func(user UserInfo) string {
		// render sql stmts
		roleDesc, _ := mysqlOps.renderRoleByName(user.RoleName)
		// update privilege
		sql := fmt.Sprintf(sqlTpl, roleDesc, user.UserName)
		return sql
	}
	msgTplRend := func(user UserInfo) string {
		return fmt.Sprintf(succMsgTpl, user.RoleName, user.UserName)
	}

	postProcessor := func(user UserInfo) error {
		return mysqlOps.updateUserAttr(ctx, user, op)
	}

	return mysqlOps.execUser(ctx, req, op, validFn, sqlTplRend, msgTplRend, postProcessor)
}

func (mysqlOps *MysqlOperations) updateUserAttr(ctx context.Context, user UserInfo, op bindings.OperationKind) error {
	var (
		getUserRolesTpl = `
		SELECT user AS userName, JSON_UNQUOTE(ATTRIBUTE->>"$.kbroles") AS roleName FROM INFORMATION_SCHEMA.USER_ATTRIBUTES WHERE USER = '%s';
		`
		alterUserRolesTpl = "ALTER USER '%s'@'%%' ATTRIBUTE '%s';"
		roles             = make([]string, 0)
	)

	// get user roles
	if jsonData, err := mysqlOps.query(ctx, fmt.Sprintf(getUserRolesTpl, user.UserName)); err != nil {
		return err
	} else {
		users := []UserInfo{}
		err = json.Unmarshal(jsonData, &users)
		if err != nil {
			return err
		}
		if len(users) == 0 {
			return fmt.Errorf("no such user: %s", user.UserName)
		}
		// user roles is an aarray of strings
		if len(users[0].RoleName) > 0 {
			if err = json.Unmarshal([]byte(users[0].RoleName), &roles); err != nil {
				return err
			}
		}
	}

	// check if role exists
	idx := slices.Index(roles, user.RoleName)
	if idx == -1 {
		if op == RevokeUserRoleOp {
			// op does not exist, do nothing
			return nil
		} else {
			// update roles
			roles = append(roles, user.RoleName)
		}
	} else {
		if op == GrantUserRoleOp {
			// op already exists, do nothing
			return nil
		} else {
			roles = slices.Delete(roles, idx, idx+1)
		}
	}

	// update user attributes
	jsonData, _ := json.Marshal(map[string][]string{"kbroles": roles})
	sql := fmt.Sprintf(alterUserRolesTpl, user.UserName, string(jsonData))
	mysqlOps.Logger.Debugf("MysqlOperations.updateUserAttr() with sql: %s", sql)
	_, err := mysqlOps.exec(ctx, sql)
	return err
}

func (mysqlOps *MysqlOperations) execUser(ctx context.Context, req *bindings.InvokeRequest, opsKind bindings.OperationKind,
	validFn UserDefinedObjectValidator[UserInfo], sqlTplRend SQLRender[UserInfo], msgTplRend SQLRender[UserInfo], postProcessor SQLPostProcessor[UserInfo]) (OpsResult, error) {
	var (
		result   = OpsResult{}
		userInfo = UserInfo{}
		metadata = OpsMetadata{StartTime: time.Now(), Operation: opsKind}
	)

	result[RespTypMeta] = &metadata
	// parser userinfo from metadata
	if err := ParseObjectFromMetadata(req.Metadata, &userInfo, validFn); err != nil {
		metadata.EndTime = time.Now()
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}

	sql := sqlTplRend(userInfo)
	mysqlOps.Logger.Debugf("MysqlOperations.execUser() with sql: %s", sql)
	_, err := mysqlOps.exec(ctx, sql)
	metadata.EndTime = time.Now()
	metadata.Extra = sql

	if err != nil {
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}

	if postProcessor != nil {
		err = postProcessor(userInfo)
		if err != nil {
			result[RespTypEve] = RespEveFail
			result[RespTypMsg] = err.Error()
			return result, nil
		}
	}

	result[RespTypEve] = RespEveSucc
	result[RespTypMsg] = msgTplRend(userInfo)
	return result, nil
}

func (mysqlOps *MysqlOperations) queryUser(ctx context.Context, req *bindings.InvokeRequest, opsKind bindings.OperationKind,
	validFn UserDefinedObjectValidator[UserInfo], sqlTplRend SQLRender[UserInfo], dataProcessor DataRender) (OpsResult, error) {
	var (
		result   = OpsResult{}
		userInfo = UserInfo{}
		metadata = OpsMetadata{StartTime: time.Now(), Operation: opsKind}
	)

	result[RespTypMeta] = &metadata
	// parser userinfo from metadata
	if err := ParseObjectFromMetadata(req.Metadata, &userInfo, validFn); err != nil {
		metadata.EndTime = time.Now()
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}

	sql := sqlTplRend(userInfo)
	mysqlOps.Logger.Debugf("MysqlOperations.queryUser() with sql: %s", sql)
	jsonData, err := mysqlOps.query(ctx, sql)
	metadata.EndTime = time.Now()
	metadata.Extra = sql

	if err != nil {
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}
	var ret interface{}
	if dataProcessor == nil {
		ret = string(jsonData)
	} else {
		if ret, err = dataProcessor(jsonData); err != nil {
			result[RespTypEve] = RespEveFail
			result[RespTypMsg] = err.Error()
			return result, nil
		}
	}

	result[RespTypEve] = RespEveSucc
	result[RespTypData] = ret
	return result, nil
}

func (mysqlOps *MysqlOperations) renderRoleByName(roleName string) (string, error) {
	switch strings.ToLower(roleName) {
	case SuperUserRole:
		return "ALL PRIVILEGES ON *.*", nil
	case ReadWriteRole:
		return "SELECT, INSERT, UPDATE, DELETE ON *.*", nil
	case ReadOnlyRole:
		return "SELECT ON *.*", nil
	default:
		return "", fmt.Errorf("role name: %s is not supported", roleName)
	}
}
