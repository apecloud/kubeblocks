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

package vault

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/hashicorp/vault/sdk/database/helper/credsutil"

	"github.com/apecloud/kubeblocks/pkg/lorry/client"
)

const (
	lorryTypeName        = "lorry"
	defaultLorryUserRule = "readonly"
	defaultTimeout       = 20000 * time.Millisecond
	maxKeyLength         = 32
)

var _ dbplugin.Database = &LorryDB{}

type LorryDB struct {
	credsutil.CredentialsProducer
	lorryClient client.Client
	logger      hclog.Logger
	config      map[string]any
	sync.Mutex
}

// New implements builtinplugins.BuiltinFactory
func New() (interface{}, error) {
	db := new()
	// Wrap the plugin with middleware to sanitize errors
	dbType := dbplugin.NewDatabaseErrorSanitizerMiddleware(db, nil)
	return dbType, nil
}

func new() *LorryDB {
	db := &LorryDB{
		logger: hclog.New(&hclog.LoggerOptions{}),
	}

	return db
}

func (db *LorryDB) Initialize(ctx context.Context, req dbplugin.InitializeRequest) (dbplugin.InitializeResponse, error) {
	db.logger.Info("initialize", "config", req.Config)
	resp := dbplugin.InitializeResponse{
		Config: req.Config,
	}
	lorryURL, ok := req.Config["url"]
	if !ok {
		msg := "lorry url is not set"
		db.logger.Info(msg)
		return resp, errors.New(msg)
	}

	lorryClient, err := client.NewHTTPClientWithURL(lorryURL.(string))
	if err != nil {
		db.logger.Info("new lorry http client failed", "error", err)
		return resp, err
	}
	db.lorryClient = lorryClient
	db.config = req.Config

	return resp, nil
}

func (db *LorryDB) NewUser(ctx context.Context, req dbplugin.NewUserRequest) (dbplugin.NewUserResponse, error) {
	db.logger.Info("new user", "req", req)
	// Grab the lock
	db.Lock()
	defer db.Unlock()

	username, err := credsutil.GenerateUsername(
		credsutil.DisplayName(req.UsernameConfig.DisplayName, maxKeyLength),
		credsutil.RoleName(req.UsernameConfig.RoleName, maxKeyLength),
		credsutil.MaxLength(maxKeyLength))
	if err != nil {
		return dbplugin.NewUserResponse{}, fmt.Errorf("failed to generate username: %w", err)
	}
	username = strings.ToUpper(username)
	password := req.Password

	statements := removeEmpty(req.Statements.Commands)
	accessMode := defaultLorryUserRule
	if len(statements) > 0 {
		accessMode = statements[0]
	}

	err = db.lorryClient.CreateUser(ctx, username, password, accessMode)
	if err != nil {
		db.logger.Info("create user failed", "error", err)
		return dbplugin.NewUserResponse{}, err
	}

	resp := dbplugin.NewUserResponse{
		Username: username,
	}

	return resp, nil
}

func (db *LorryDB) UpdateUser(ctx context.Context, req dbplugin.UpdateUserRequest) (dbplugin.UpdateUserResponse, error) {
	if req.Password != nil {
		err := errors.New("not support yet")
		return dbplugin.UpdateUserResponse{}, err
	}
	return dbplugin.UpdateUserResponse{}, nil
}

func (db *LorryDB) DeleteUser(ctx context.Context, req dbplugin.DeleteUserRequest) (dbplugin.DeleteUserResponse, error) {
	db.Lock()
	defer db.Unlock()

	err := db.lorryClient.DeleteUser(ctx, req.Username)

	if err != nil {
		return dbplugin.DeleteUserResponse{}, err
	}

	return dbplugin.DeleteUserResponse{}, nil
}

func (db *LorryDB) Type() (string, error) {
	return lorryTypeName, nil
}

func (db *LorryDB) Close() error {
	return nil
}

func removeEmpty(strs []string) []string {
	var newStrs []string
	for _, str := range strs {
		str = strings.TrimSpace(str)
		if str == "" {
			continue
		}
		newStrs = append(newStrs, str)
	}

	return newStrs
}
