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
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

// DynamicParamUpdater is designed to adapt to the dapper implementation.
type DynamicParamUpdater interface {
	ExecCommand(ctx context.Context, command string, args ...string) (string, error)
	Close()
}

type mysqlCommandChannel struct {
	db *sql.DB
}

const (
	dbType = "DB_TYPE"

	connectTimeout = 5 * time.Second

	mysql             = "mysql"
	patroni           = "patroni"
	mysqlDsnEnv       = "DATA_SOURCE_NAME"
	patroniRestAPIURL = "PATRONI_REST_API_URL"
)

func NewCommandChannel(ctx context.Context, dataType, dsn string) (DynamicParamUpdater, error) {
	// TODO using dapper command channel

	if dataType == "" {
		dataType = viper.GetString(dbType)
	}

	logger.V(1).Info(fmt.Sprintf("new command channel. [%s]", dataType))
	switch strings.ToLower(dataType) {
	case mysql:
		return NewMysqlConnection(ctx, dsn)
	case patroni:
		return NewPGPatroniConnection(dsn)
	default:
		// TODO mock db begin support dapper
	}
	return nil, cfgcore.MakeError("not support type[%s]", dataType)
}

func NewMysqlConnection(ctx context.Context, dsn string) (DynamicParamUpdater, error) {
	logger.V(1).Info("connecting mysql.")
	if dsn == "" {
		dsn = os.Getenv(mysqlDsnEnv)
	}
	if dsn == "" {
		return nil, cfgcore.MakeError("require DATA_SOURCE_NAME env.")
	}
	db, err := sql.Open(mysql, dsn)
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to opening connection to mysql.")
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	// Set max lifetime for a connection.
	db.SetConnMaxLifetime(1 * time.Minute)

	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Error(err, "failed to  pinging mysqld.")
		return nil, err
	}
	logger.V(1).Info("succeed to connect mysql.")
	return &mysqlCommandChannel{db: db}, nil
}

func (m *mysqlCommandChannel) ExecCommand(ctx context.Context, sql string, _ ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
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

type pgPatroniCommandChannel struct {
	restapiURL string
}

func (p *pgPatroniCommandChannel) ExecCommand(ctx context.Context, command string, args ...string) (string, error) {
	const (
		Config  = "config"
		Reload  = "reload"
		Restart = "restart"
	)

	if len(args) == 0 {
		return "", cfgcore.MakeError("require patroni functional.")
	}

	functional := args[0]
	restPath := strings.Join([]string{p.restapiURL, functional}, "/")
	logger.V(1).Info(fmt.Sprintf("exec patroni command: %s", functional))
	switch functional {
	case Config:
		return sendRestRequest(ctx, command, restPath, "PATCH")
	case Restart, Reload:
		return sendRestRequest(ctx, command, restPath, "POST")
	}
	return "", cfgcore.MakeError("not support patroni functional[%s]", args[0])
}

func (p *pgPatroniCommandChannel) Close() {
}

func NewPGPatroniConnection(hostURL string) (DynamicParamUpdater, error) {
	logger.V(1).Info("connecting patroni.")
	if hostURL == "" {
		hostURL = os.Getenv(patroniRestAPIURL)
	}
	if hostURL == "" {
		return nil, cfgcore.MakeError("require PATRONI_REST_API_URL env.")
	}

	return &pgPatroniCommandChannel{restapiURL: formatRestAPIPath(hostURL)}, nil
}

func sendRestRequest(ctx context.Context, body string, url string, method string) (string, error) {
	client := &http.Client{}
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	// create new HTTP PATCH request with JSON payload
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	if err != nil {
		return "", err
	}

	// set content-type header to JSON
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	response, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(response), nil
}

func formatRestAPIPath(path string) string {
	const restAPIPrefix = "http://"
	if strings.HasPrefix(path, restAPIPrefix) {
		return path
	}
	return fmt.Sprintf("%s%s", restAPIPrefix, path)
}
