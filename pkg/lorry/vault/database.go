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
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/hashicorp/vault/sdk/database/helper/credsutil"
)

const (
	lorryTypeName        = "lorry"
	defaultRedisUserRule = `["~*", "+@read"]`
	defaultTimeout       = 20000 * time.Millisecond
	maxKeyLength         = 64
)

var _ dbplugin.Database = &LorryDB{}

type LorryDB struct {
	credsutil.CredentialsProducer
	logger hclog.Logger
	sync.Mutex
}

// New implements builtinplugins.BuiltinFactory
func New(logger hclog.Logger) (interface{}, error) {
	db := new()
	db.logger = logger
	// Wrap the plugin with middleware to sanitize errors
	dbType := dbplugin.NewDatabaseErrorSanitizerMiddleware(db, nil)
	return dbType, nil
}

func new() *LorryDB {

	db := &LorryDB{}

	return db
}

func (c *LorryDB) Initialize(ctx context.Context, req dbplugin.InitializeRequest) (dbplugin.InitializeResponse, error) {
	resp := dbplugin.InitializeResponse{
		Config: req.Config,
	}
	// internal logger to os.Stderr
	logger := hclog.New(&hclog.LoggerOptions{})
	logger.Debug("lorry plugin", "config", req.Config)
	return resp, nil
}

func (c *LorryDB) NewUser(ctx context.Context, req dbplugin.NewUserRequest) (dbplugin.NewUserResponse, error) {
	// Grab the lock
	c.Lock()
	defer c.Unlock()

	username, err := credsutil.GenerateUsername(
		credsutil.DisplayName(req.UsernameConfig.DisplayName, maxKeyLength),
		credsutil.RoleName(req.UsernameConfig.RoleName, maxKeyLength))
	if err != nil {
		return dbplugin.NewUserResponse{}, fmt.Errorf("failed to generate username: %w", err)
	}
	username = strings.ToUpper(username)

	// db, err := c.getConnection(ctx)
	// if err != nil {
	// 	return dbplugin.NewUserResponse{}, fmt.Errorf("failed to get connection: %w", err)
	// }

	// err = newUser(ctx, db, username, req)
	// if err != nil {
	// 	return dbplugin.NewUserResponse{}, err
	// }

	// internal logger to os.Stderr
	logger := hclog.New(&hclog.LoggerOptions{})
	logger.Debug("lorry plugin", "username", username)
	resp := dbplugin.NewUserResponse{
		Username: username,
	}

	return resp, nil
}

func (c *LorryDB) UpdateUser(ctx context.Context, req dbplugin.UpdateUserRequest) (dbplugin.UpdateUserResponse, error) {
	// if req.Password != nil {
	// 	err := c.changeUserPassword(ctx, req.Username, req.Password.NewPassword)
	// 	return dbplugin.UpdateUserResponse{}, err
	// }
	return dbplugin.UpdateUserResponse{}, nil
}

func (c *LorryDB) DeleteUser(ctx context.Context, req dbplugin.DeleteUserRequest) (dbplugin.DeleteUserResponse, error) {
	c.Lock()
	defer c.Unlock()
	//
	//	db, err := c.getConnection(ctx)
	//	if err != nil {
	//		return dbplugin.DeleteUserResponse{}, fmt.Errorf("failed to make connection: %w", err)
	//	}
	//
	//	// Close the database connection to ensure no new connections come in
	//	defer func() {
	//		if err := c.close(); err != nil {
	//			logger := hclog.New(&hclog.LoggerOptions{})
	//			logger.Error("defer close failed", "error", err)
	//		}
	//	}()
	//
	//	var response string
	//
	//	err = db.Do(ctx, radix.Cmd(&response, "ACL", "DELUSER", req.Username))
	//
	//	if err != nil {
	//		return dbplugin.DeleteUserResponse{}, err
	//	}

	return dbplugin.DeleteUserResponse{}, nil
}

func (c *LorryDB) Type() (string, error) {
	return lorryTypeName, nil
}

func (c *LorryDB) Close() error {
	return nil
}

// func newUser(ctx context.Context, db radix.Client, username string, req dbplugin.NewUserRequest) error {
// 	statements := removeEmpty(req.Statements.Commands)
// 	if len(statements) == 0 {
// 		statements = append(statements, defaultRedisUserRule)
// 	}
//
// 	aclargs := []string{"SETUSER", username, "ON", ">" + req.Password}
//
// 	var args []string
// 	err := json.Unmarshal([]byte(statements[0]), &args)
// 	if err != nil {
// 		return errwrap.Wrapf("error unmarshalling REDIS rules in the creation statement JSON: {{err}}", err)
// 	}
//
// 	aclargs = append(aclargs, args...)
// 	var response string
//
// 	err = db.Do(ctx, radix.Cmd(&response, "ACL", aclargs...))
//
// 	if err != nil {
// 		return err
// 	}
//
// 	return nil
// }
//
// func (c *LorryDB) changeUserPassword(ctx context.Context, username, password string) error {
// 	c.Lock()
// 	defer c.Unlock()
//
// 	db, err := c.getConnection(ctx)
// 	if err != nil {
// 		return err
// 	}
//
// 	// Close the database connection to ensure no new connections come in
// 	defer func() {
// 		if err := c.close(); err != nil {
// 			logger := hclog.New(&hclog.LoggerOptions{})
// 			logger.Error("defer close failed", "error", err)
// 		}
// 	}()
//
// 	var response resp3.ArrayHeader
// 	mn := radix.Maybe{Rcv: &response}
// 	var redisErr resp3.SimpleError
// 	err = db.Do(ctx, radix.Cmd(&mn, "ACL", "GETUSER", username))
// 	if errors.As(err, &redisErr) {
// 		return fmt.Errorf("redis error returned: %s", redisErr.Error())
// 	}
//
// 	if err != nil {
// 		return fmt.Errorf("reset of passwords for user %s failed in changeUserPassword: %w", username, err)
// 	}
//
// 	if mn.Null {
// 		return fmt.Errorf("changeUserPassword for user %s failed, user not found!", username)
// 	}
//
// 	var sresponse string
// 	err = db.Do(ctx, radix.Cmd(&sresponse, "ACL", "SETUSER", username, "RESETPASS", ">"+password))
//
// 	if err != nil {
// 		return err
// 	}
//
// 	return nil
// }
//
// func removeEmpty(strs []string) []string {
// 	var newStrs []string
// 	for _, str := range strs {
// 		str = strings.TrimSpace(str)
// 		if str == "" {
// 			continue
// 		}
// 		newStrs = append(newStrs, str)
// 	}
//
// 	return newStrs
// }
//
// func computeTimeout(ctx context.Context) (timeout time.Duration) {
// 	deadline, ok := ctx.Deadline()
// 	if ok {
// 		return time.Until(deadline)
// 	}
// 	return defaultTimeout
// }
//
// func (c *LorryDB) getConnection(ctx context.Context) (radix.Client, error) {
// 	db, err := c.Connection(ctx)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return db.(radix.Client), nil
// }
//
// func (c *LorryDB) Type() (string, error) {
// 	return redisTypeName, nil
// }
//
