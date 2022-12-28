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
	"strings"
	"sync"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/cmd/probe/internal"
)

// Mysql represents MySQL output bindings.
type Mysql struct {
	db       *sql.DB
	mu       sync.Mutex
	logger   logger.Logger
	metadata bindings.Metadata
	base     internal.ProbeBase
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

const (
	statusCheckType = iota
)

var (
	defaultDbPort = 3306
	dbUser        = "root"
	dbPasswd      = ""
	dbRoles       = map[string]internal.AccessMode{}
)

// NewMysql returns a new MySQL output binding.
func NewMysql(logger logger.Logger) bindings.OutputBinding {
	return &Mysql{logger: logger}
}

// Init initializes the MySQL binding.
func (m *Mysql) Init(metadata bindings.Metadata) error {
	if viper.IsSet("KB_SERVICE_USER") {
		dbUser = viper.GetString("KB_SERVICE_USER")
	}

	if viper.IsSet("KB_SERVICE_PASSWORD") {
		dbPasswd = viper.GetString("KB_SERVICE_PASSWORD")
	}

	if viper.IsSet("KB_SERVICE_ROLES") {
		val := viper.GetString("KB_SERVICE_ROLES")
		if err := json.Unmarshal([]byte(val), &dbRoles); err != nil {
			fmt.Println(errors.Wrap(err, "KB_DB_ROLES env format error").Error())
		}
	}
	m.logger.Debug("Initializing MySQL binding")
	m.metadata = metadata

	m.base = internal.ProbeBase{
		Logger:    m.logger,
		Operation: m,
	}
	m.base.Init()

	return nil
}

// Invoke handles all invoke operations.
func (m *Mysql) Invoke(ctx context.Context, req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	return m.base.Invoke(ctx, req)
}

// Operations returns list of operations supported by Mysql binding.
func (m *Mysql) Operations() []bindings.OperationKind {
	return m.base.Operations()
}

func (m *Mysql) InitIfNeed() error {
	if m.db == nil {
		go m.InitDelay()
		return fmt.Errorf("Init db connection asynchronously.")
	}
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

func (m *Mysql) GetRunningPort() int {
	p := m.metadata.Properties
	url, ok := p[connectionURLKey]
	if !ok || url == "" {
		return defaultDbPort
	}

	config, err := mysql.ParseDSN(url)
	if err != nil {
		return defaultDbPort
	}
	index := strings.LastIndex(config.Addr, ":")
	if index < 0 {
		return defaultDbPort
	}
	port, err := strconv.Atoi(config.Addr[index+1:])
	if err != nil {
		return defaultDbPort
	}

	return port
}

func (m *Mysql) GetRole(ctx context.Context, sql string) (string, error) {
	m.logger.Debugf("query: %s", sql)
	if sql == "" {
		sql = "select CURRENT_LEADER, ROLE, SERVER_ID  from information_schema.wesql_cluster_local"
	}

	// sql exec timeout need to be less than httpget's timeout which default is 1s.
	ctx1, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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

// design details: https://infracreate.feishu.cn/wiki/wikcndch7lMZJneMnRqaTvhQpwb#doxcnOUyQ4Mu0KiUo232dOr5aad
func (m *Mysql) StatusCheck(ctx context.Context, sql string, resp *bindings.InvokeResponse) ([]byte, error) {
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
