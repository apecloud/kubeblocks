/*
Copyright 2021 The Dapr Authors
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

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
)

// List of operations.
const (
	connectionURLKey = "url"
	commandSQLKey    = "sql"
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

// NewPostgres returns a new PostgreSQL output binding.
func NewPostgres(logger logger.Logger) bindings.OutputBinding {
	return &PostgresOperations{BaseOperations: BaseOperations{Logger: logger}}
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
	// pgOps.RegisterOperation(CheckStatusOperation, pgOps.CheckStatusOps)
	pgOps.RegisterOperation(ExecOperation, pgOps.ExecOps)
	pgOps.RegisterOperation(QueryOperation, pgOps.QueryOps)
	return nil
}

func (pgOps *PostgresOperations) initIfNeed() bool {
	if pgOps.db == nil {
		go func() {
			err := pgOps.InitDelay()
			pgOps.Logger.Errorf("MySQl connection init failed: %v", err)
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

	var role string
	var isRecovery bool
	for rows.Next() {
		if err = rows.Scan(&isRecovery); err != nil {
			pgOps.Logger.Errorf("Role query error: %v", err)
			return role, err
		}
	}
	role = "primary"
	if isRecovery {
		role = "secondary"
	}
	return role, nil
}

func (pgOps *PostgresOperations) ExecOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	result := OpsResult{}
	sql, ok := req.Metadata["sql"]
	if !ok || sql == "" {
		result["event"] = "ExecFailed"
		result["message"] = "no sql provided"
		return result, nil
	}
	count, err := pgOps.exec(ctx, sql)
	if err != nil {
		pgOps.Logger.Infof("exec error: %v", err)
		result["event"] = "ExecFailed"
		result["message"] = err.Error()
	} else {
		result["event"] = "ExecSuccess"
		result["count"] = count
	}
	return result, nil
}

func (pgOps *PostgresOperations) QueryOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	result := OpsResult{}
	sql, ok := req.Metadata["sql"]
	if !ok || sql == "" {
		result["event"] = "QueryFailed"
		result["message"] = "no sql provided"
		return result, nil
	}
	data, err := pgOps.query(ctx, sql)
	if err != nil {
		pgOps.Logger.Infof("Query error: %v", err)
		result["event"] = "QueryFailed"
		result["message"] = err.Error()
	} else {
		result["event"] = "QuerySuccess"
		result["message"] = string(data)
	}
	return result, nil
}

func (pgOps *PostgresOperations) query(ctx context.Context, sql string) (result []byte, err error) {
	pgOps.Logger.Debugf("query: %s", sql)

	rows, err := pgOps.db.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("error executing query: %w", err)
	}

	rs := make([]any, 0)
	for rows.Next() {
		val, rowErr := rows.Values()
		if rowErr != nil {
			return nil, fmt.Errorf("error parsing result '%v': %w", rows.Err(), rowErr)
		}
		rs = append(rs, val) //nolint:asasalint
	}

	if result, err = json.Marshal(rs); err != nil {
		err = fmt.Errorf("error serializing results: %w", err)
	}

	return
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
