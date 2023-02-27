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

package configmanager

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

// DynamicParamUpdater is designed to adapt to the dapper implementation.
type DynamicParamUpdater interface {
	ExecCommand(command string) (string, error)
	Close()
}

type mysqlCommandChannel struct {
	db *sql.DB
}

const (
	DBType = "DB_TYPE"

	MYSQL       = "mysql"
	MYSQLDsnEnv = "DATA_SOURCE_NAME"
)

func NewCommandChannel(dbType string) (DynamicParamUpdater, error) {
	// TODO using dapper command channel

	switch dbType {
	case MYSQL:
		return NewMysqlConnection()
	default:
		// TODO mock db begin support dapper
	}
	return nil, cfgcore.MakeError("not support type[%s]", dbType)
}

func NewMysqlConnection() (DynamicParamUpdater, error) {
	logger.V(1).Info("connecting mysql.")
	dsn := os.Getenv(MYSQLDsnEnv)
	if dsn == "" {
		return nil, cfgcore.MakeError("require DATA_SOURCE_NAME env.")
	}
	db, err := sql.Open(MYSQL, dsn)
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to opening connection to mysql.")
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	// Set max lifetime for a connection.
	db.SetConnMaxLifetime(1 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Error(err, "failed to  pinging mysqld.")
		return nil, err
	}
	logger.V(1).Info("succeed to connect mysql.")
	return &mysqlCommandChannel{db: db}, nil
}

func (m *mysqlCommandChannel) ExecCommand(sql string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, err := m.db.ExecContext(ctx, sql)
	if err != nil {
		return "", err
	}
	affected, err := r.RowsAffected()
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(affected, 10), nil
}

func (m *mysqlCommandChannel) Close() {
	m.db.Close()
	logger.V(1).Info("closed mysql connection.")
}
