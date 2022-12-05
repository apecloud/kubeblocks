/*
Copyright ApeCloud Inc.

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
	"net"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/cmd/probe/internal"
)

// Mysql represents MySQL output bindings.
type Mysql struct {
	db       *sql.DB
	mu       sync.Mutex
	logger   logger.Logger
	metadata bindings.Metadata
}

const (
	// list of operations.
	execOperation         bindings.OperationKind = "exec"
	runningCheckOperation bindings.OperationKind = "runningCheck"
	statusCheckOperation  bindings.OperationKind = "statusCheck"
	roleCheckOperation    bindings.OperationKind = "roleCheck"
	queryOperation        bindings.OperationKind = "query"
	closeOperation        bindings.OperationKind = "close"

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

	// keys from request's metadata.
	commandSQLKey = "sql"

	// keys from response's metadata.
	respOpKey           = "operation"
	respSQLKey          = "sql"
	respStartTimeKey    = "start-time"
	respRowsAffectedKey = "rows-affected"
	respEndTimeKey      = "end-time"
	respDurationKey     = "duration"
	statusCode          = "status-code"
	//451 Unavailable For Legal Reasons, used to indicate check failed and trigger kubelet events
	checkFailedHTTPCode = "451"
)

const (
	runningCheckType = iota
	statusCheckType
	roleChangedCheckType
)

var oriRole = ""
var bootTime = time.Now()
var runningCheckFailedCount = 0
var statusCheckFailedCount = 0
var roleCheckFailedCount = 0
var roleCheckCount = 0
var eventAggregationNum = 10
var eventIntervalNum = 60
var dbPort = 3306
var dbRoles = map[string]internal.AccessMode{}

func init() {
	val, ok := os.LookupEnv("KB_AGGREGATION_NUMBER")
	if ok {
		num, err := strconv.Atoi(val)
		if err == nil {
			eventAggregationNum = num
		}
	}

	val, ok = os.LookupEnv("KB_DB_PORT")
	if ok {
		num, err := strconv.Atoi(val)
		if err == nil {
			dbPort = num
		}
	}

	val, ok = os.LookupEnv("KB_DB_ROLES")
	if ok {
		if err := json.Unmarshal([]byte(val), &dbRoles); err != nil {
			fmt.Println(errors.Wrap(err, "KB_DB_ROLES env format error").Error())
		}
	}
}

// NewMysql returns a new MySQL output binding.
func NewMysql(logger logger.Logger) bindings.OutputBinding {
	return &Mysql{logger: logger}
}

// Init initializes the MySQL binding.
func (m *Mysql) Init(metadata bindings.Metadata) error {
	m.logger.Debug("Initializing MySQL binding")
	m.metadata = metadata
	return nil
}

func (m *Mysql) InitDelay() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db != nil {
		return nil
	}

	p := m.metadata.Properties
	url, ok := p[connectionURLKey]
	if !ok || url == "" {
		return fmt.Errorf("missing MySql connection string")
	}

	db, err := initDB(url, m.metadata.Properties[pemPathKey])
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
		m.logger.Infof("unable to ping the DB")
		return errors.Wrap(err, "unable to ping the DB")
	}
	m.db = db

	return nil
}

// Invoke handles all invoke operations.
func (m *Mysql) Invoke(ctx context.Context, req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	if req == nil {
		return nil, errors.Errorf("invoke request required")
	}

	var sql string
	var ok bool
	startTime := time.Now()
	resp := &bindings.InvokeResponse{
		Metadata: map[string]string{
			respOpKey:        string(req.Operation),
			respSQLKey:       "test",
			respStartTimeKey: startTime.Format(time.RFC3339Nano),
		},
	}

	updateRespMetadata := func() (*bindings.InvokeResponse, error) {
		endTime := time.Now()
		resp.Metadata[respEndTimeKey] = endTime.Format(time.RFC3339Nano)
		resp.Metadata[respDurationKey] = endTime.Sub(startTime).String()
		return resp, nil
	}

	if req.Operation == runningCheckOperation {
		d, err := m.runningCheck(ctx, resp)
		if err != nil {
			return nil, err
		}
		resp.Data = d
		return updateRespMetadata()
	}

	if m.db == nil {
		go m.InitDelay()
		resp.Data = []byte("db not ready")
		return updateRespMetadata()
	}

	if req.Operation == closeOperation {
		return nil, m.db.Close()
	}

	if req.Metadata == nil {
		return nil, errors.Errorf("metadata required")
	}
	m.logger.Debugf("operation: %v", req.Operation)

	sql, ok = req.Metadata[commandSQLKey]
	if !ok {
		return nil, errors.Errorf("required metadata not set: %s", commandSQLKey)
	}

	switch req.Operation { //nolint:exhaustive
	case execOperation:
		r, err := m.exec(ctx, sql)
		if err != nil {
			return nil, err
		}
		resp.Metadata[respRowsAffectedKey] = strconv.FormatInt(r, 10)

	case queryOperation:
		d, err := m.query(ctx, sql)
		if err != nil {
			return nil, err
		}
		resp.Data = d

	case statusCheckOperation:
		d, err := m.statusCheck(ctx, sql, resp)
		if err != nil {
			return nil, err
		}
		resp.Data = d

	case roleCheckOperation:
		d, err := m.roleCheck(ctx, sql, resp)
		if err != nil {
			return nil, err
		}
		resp.Data = d

	default:
		return nil, errors.Errorf("invalid operation type: %s. Expected %s, %s, or %s",
			req.Operation, execOperation, queryOperation, closeOperation)
	}

	return updateRespMetadata()
}

// Operations returns list of operations supported by Mysql binding.
func (m *Mysql) Operations() []bindings.OperationKind {
	return []bindings.OperationKind{
		execOperation,
		queryOperation,
		closeOperation,
		runningCheckOperation,
		statusCheckOperation,
		roleCheckOperation,
	}
}

// Close will close the DB.
func (m *Mysql) Close() error {
	if m.db != nil {
		return m.db.Close()
	}

	return nil
}

func (m *Mysql) query(ctx context.Context, sql string) ([]byte, error) {
	m.logger.Debugf("query: %s", sql)

	rows, err := m.db.QueryContext(ctx, sql)
	if err != nil {
		return nil, errors.Wrapf(err, "error executing %s", sql)
	}

	defer func() {
		_ = rows.Close()
		_ = rows.Err()
	}()

	result, err := m.jsonify(rows)
	if err != nil {
		return nil, errors.Wrapf(err, "error marshalling query result for %s", sql)
	}

	return result, nil
}

func (m *Mysql) exec(ctx context.Context, sql string) (int64, error) {
	m.logger.Debugf("exec: %s", sql)

	res, err := m.db.ExecContext(ctx, sql)
	if err != nil {
		return 0, errors.Wrapf(err, "error executing %s", sql)
	}

	return res.RowsAffected()
}

func (m *Mysql) runningCheck(ctx context.Context, resp *bindings.InvokeResponse) ([]byte, error) {
	host := fmt.Sprintf("127.0.0.1:%d", dbPort)
	conn, err := net.DialTimeout("tcp", host, 900*time.Millisecond)
	message := ""
	result := internal.ProbeMessage{}
	if err != nil {
		message = fmt.Sprintf("running check %s error: %v", host, err)
		result.Event = "runningCheckFailed"
		m.logger.Errorf(message)
		if runningCheckFailedCount++; runningCheckFailedCount%eventAggregationNum == 1 {
			m.logger.Infof("running checks failed %v times continuously", runningCheckFailedCount)
			resp.Metadata[statusCode] = checkFailedHTTPCode
		}
	} else {
		runningCheckFailedCount = 0
		message = "TCP Connection Established Successfully!"
		if tcpCon, ok := conn.(*net.TCPConn); ok {
			tcpCon.SetLinger(0)
		}
		defer conn.Close()
	}
	result.Message = message
	msg, _ := json.Marshal(result)
	return msg, nil
}

// design details: https://infracreate.feishu.cn/wiki/wikcndch7lMZJneMnRqaTvhQpwb#doxcnOUyQ4Mu0KiUo232dOr5aad
func (m *Mysql) statusCheck(ctx context.Context, sql string, resp *bindings.InvokeResponse) ([]byte, error) {
	// rwSql := fmt.Sprintf(`begin;
	// create table if not exists kb_health_check(type int, check_ts bigint, primary key(type));
	// insert into kb_health_check values(%d, now()) on duplicate key update check_ts = now();
	// commit;
	// select check_ts from kb_health_check where type=%d limit 1;`, statusCheckType, statusCheckType)
	// roSql := fmt.Sprintf(`select check_ts from kb_health_check where type=%d limit 1;`, statusCheckType)
	// var err error
	// var data []byte
	// switch dbRoles[strings.ToLower(oriRole)] {
	// case internal.ReadWrite:
	// 	var count int64
	// 	count, err = m.exec(ctx, rwSql)
	// 	data = []byte(strconv.FormatInt(count, 10))
	// case internal.Readonly:
	// 	data, err = m.query(ctx, roSql)
	// default:
	// 	msg := fmt.Sprintf("unknown access mode for role %s: %v", oriRole, dbRoles)
	// 	m.logger.Info(msg)
	// 	data = []byte(msg)
	// }

	// result := internal.ProbeMessage{}
	// if err != nil {
	// 	m.logger.Infof("statusCheck error: %v", err)
	// 	result.Event = "statusCheckFailed"
	// 	result.Message = err.Error()
	// 	if statusCheckFailedCount++; statusCheckFailedCount%eventAggregationNum == 1 {
	// 		m.logger.Infof("status checks failed %v times continuously", statusCheckFailedCount)
	// 		resp.Metadata[statusCode] = checkFailedHTTPCode
	// 	}
	// } else {
	// 	result.Message = string(data)
	// 	statusCheckFailedCount = 0
	// }
	// msg, _ := json.Marshal(result)
	// return msg, nil
	return []byte("Not supported yet"), nil

}

func (m *Mysql) getRole(ctx context.Context, sql string) (string, error) {
	m.logger.Debugf("query: %s", sql)
	if sql == "" {
		sql = "select CURRENT_LEADER, ROLE, SERVER_ID  from information_schema.wesql_cluster_local"
	}

	// sql exec timeout need to be less than httpget's timeout which default is 1s.
	ctx1, cancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
	defer cancel()
	rows, err := m.db.QueryContext(ctx1, sql)
	if err != nil {
		m.logger.Infof("error executing %s: %v", sql, err)
		return "", errors.Wrapf(err, "error executing %s", sql)
	}

	defer func() {
		_ = rows.Close()
		_ = rows.Err()
	}()

	var curLeader string
	var role string
	var serverId string
	for rows.Next() {
		if err := rows.Scan(&curLeader, &role, &serverId); err != nil {
			m.logger.Errorf("checkRole error: %", err)
		}
	}
	return role, nil
}

func (m *Mysql) roleCheck(ctx context.Context, sql string, resp *bindings.InvokeResponse) ([]byte, error) {
	result := internal.ProbeMessage{}
	result.OriginalRole = oriRole
	role, err := m.getRole(ctx, sql)
	if err != nil {
		m.logger.Infof("error executing roleCheck: %v", err)
		result.Event = "roleCheckFailed"
		result.Message = err.Error()
		if roleCheckFailedCount++; roleCheckFailedCount%eventAggregationNum == 1 {
			m.logger.Infof("role checks failed %v times continuously", roleCheckFailedCount)
			resp.Metadata[statusCode] = checkFailedHTTPCode
		}
		msg, _ := json.Marshal(result)
		return msg, nil
	}

	result.Role = role
	if oriRole != role {
		result.Event = "roleChanged"
		oriRole = role
		roleCheckCount = 0
	} else {
		result.Event = "roleUnchanged"
	}

	// reporting role event periodly to get pod's role lable updating accurately
	// in case of event losing.
	if roleCheckCount++; roleCheckCount%eventIntervalNum == 1 {
		resp.Metadata[statusCode] = checkFailedHTTPCode
	}
	msg, _ := json.Marshal(result)
	m.logger.Infof(string(msg))
	return msg, nil
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
	if _, err := mysql.ParseDSN(url); err != nil {
		return nil, errors.Wrapf(err, "illegal Data Source Name (DNS) specified by %s", connectionURLKey)
	}

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

	db, err := sql.Open("mysql", url)
	if err != nil {
		return nil, errors.Wrap(err, "error opening DB connection")
	}

	return db, nil
}

func (m *Mysql) jsonify(rows *sql.Rows) ([]byte, error) {
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

		r := m.convert(columnTypes, values)
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

func (m *Mysql) convert(columnTypes []*sql.ColumnType, values []interface{}) map[string]interface{} {
	r := map[string]interface{}{}

	for i, ct := range columnTypes {
		value := values[i]

		switch v := values[i].(type) {
		case driver.Valuer:
			if vv, err := v.Value(); err == nil {
				value = interface{}(vv)
			} else {
				m.logger.Warnf("error to convert value: %v", err)
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
