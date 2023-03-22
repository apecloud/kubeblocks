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

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	. "github.com/apecloud/kubeblocks/cmd/probe/util"
)

// List of operations.
const (
	connectionURLKey = "url"
	commandSQLKey    = "sql"
	PRIMARY          = "primary"
	SECONDARY        = "secondary"
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
	pgOps.RegisterOperation(CheckStatusOperation, pgOps.CheckStatusOps)
	pgOps.RegisterOperation(ExecOperation, pgOps.ExecOps)
	pgOps.RegisterOperation(QueryOperation, pgOps.QueryOps)
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
	pgOps.OriRole = PRIMARY
	if isRecovery {
		pgOps.OriRole = SECONDARY
	}
	return pgOps.OriRole, nil
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
