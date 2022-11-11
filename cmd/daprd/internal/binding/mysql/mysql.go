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
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
)

const (
	// list of operations.
	execOperation        bindings.OperationKind = "exec"
	statusCheckOperation bindings.OperationKind = "statusCheck"
	roleCheckOperation   bindings.OperationKind = "roleCheck"
	queryOperation       bindings.OperationKind = "query"
	closeOperation       bindings.OperationKind = "close"

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
)

var oriRole = ""
var bootTime = time.Now()

// Mysql represents MySQL output bindings.
type Mysql struct {
	db       *sql.DB
	logger   logger.Logger
	metadata bindings.Metadata
}

// NewMysql returns a new MySQL output binding.
func NewMysql(logger logger.Logger) bindings.OutputBinding {
	return &Mysql{logger: logger}
}

// Init initializes the MySQL binding.
func (m *Mysql) Init(metadata bindings.Metadata) error {
	m.logger.Debug("Initializing MySql binding")
	m.metadata = metadata
	return nil
}

// InitDelay TODO add mutex lock to resolve concurrency problem
func (m *Mysql) InitDelay() error {
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
	startTime := time.Now()
	resp := &bindings.InvokeResponse{
		Metadata: map[string]string{
			respOpKey:        string(req.Operation),
			respSQLKey:       "test",
			respStartTimeKey: startTime.Format(time.RFC3339Nano),
		},
	}

	if req == nil {
		return nil, errors.Errorf("invoke request required")
	}

	if m.db == nil {
		go m.InitDelay()
		resp.Data = []byte("db not ready")
		return resp, nil
	}

	if req.Operation == closeOperation {
		return nil, m.db.Close()
	}

	if req.Metadata == nil {
		return nil, errors.Errorf("metadata required")
	}
	m.logger.Debugf("operation: %v", req.Operation)

	s, ok := req.Metadata[commandSQLKey]
	if !ok {
		return nil, errors.Errorf("required metadata not set: %s", commandSQLKey)
	}

	switch req.Operation { //nolint:exhaustive
	case execOperation:
		r, err := m.exec(ctx, s)
		if err != nil {
			return nil, err
		}
		resp.Metadata[respRowsAffectedKey] = strconv.FormatInt(r, 10)

	case queryOperation:
		d, err := m.query(ctx, s)
		if err != nil {
			return nil, err
		}
		resp.Data = d

	case statusCheckOperation:
		d, err := m.statusCheck(ctx, s)
		if err != nil {
			return nil, err
		}
		resp.Data = d

	case roleCheckOperation:
		d, err := m.roleCheck(ctx, s)
		if err != nil {
			return nil, err
		}
		resp.Data = d

	default:
		return nil, errors.Errorf("invalid operation type: %s. Expected %s, %s, or %s",
			req.Operation, execOperation, queryOperation, closeOperation)
	}

	endTime := time.Now()
	resp.Metadata[respEndTimeKey] = endTime.Format(time.RFC3339Nano)
	resp.Metadata[respDurationKey] = endTime.Sub(startTime).String()

	return resp, nil
}

// Operations returns list of operations supported by Mysql binding.
func (m *Mysql) Operations() []bindings.OperationKind {
	return []bindings.OperationKind{
		execOperation,
		queryOperation,
		closeOperation,
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

func (m *Mysql) statusCheck(ctx context.Context, sql string) ([]byte, error) {
	m.logger.Debugf("status Check exec: %s", sql)
	if sql == "" {
		sql = "select CURRENT_LEADER, ROLE, SERVER_ID  from information_schema.wesql_cluster_local"
	}

	rows, err := m.db.QueryContext(ctx, sql)
	if err != nil {
		return nil, errors.Wrapf(err, "error executing %s", sql)
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
	return []byte(role), nil
}

func (m *Mysql) roleCheck(ctx context.Context, sql string) ([]byte, error) {
	m.logger.Debugf("query: %s", sql)
	if sql == "" {
		sql = "select CURRENT_LEADER, ROLE, SERVER_ID  from information_schema.wesql_cluster_local"
	}

	ctx1, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	rows, err := m.db.QueryContext(ctx1, sql)
	if err != nil {
		return nil, errors.Wrapf(err, "error executing %s", sql)
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
	if oriRole != role {
		result := map[string]string{}
		result["event"] = "roleChanged"
		result["originalRole"] = oriRole
		result["role"] = role
		msg, _ := json.Marshal(result)
		m.logger.Infof(string(msg))
		oriRole = role
		return nil, errors.Errorf(string(msg))
	}
	return []byte(role), nil
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
