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

package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
	"golang.org/x/exp/slices"

	bindings "github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"

	// import this json-iterator package to replace the default
	// to avoid the error: 'json: unsupported type: map[interface {}]interface {}'
	json "github.com/json-iterator/go"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	rediscomponent "github.com/apecloud/kubeblocks/cmd/probe/internal/component/redis"
	. "github.com/apecloud/kubeblocks/cmd/probe/util"
)

var (
	redisPreDefinedUsers = []string{
		"default",
		"kbadmin",
		"kbdataprotection",
		"kbmonitoring",
		"kbprobe",
		"kbreplicator",
	}
)

// Redis is a redis output binding.
type Redis struct {
	client         redis.UniversalClient
	clientSettings *rediscomponent.Settings

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc

	BaseOperations
}

var _ BaseInternalOps = &Redis{}

// NewRedis returns a new redis bindings instance.
func NewRedis(logger logger.Logger) bindings.OutputBinding {
	return &Redis{BaseOperations: BaseOperations{Logger: logger}}
}

// Init performs metadata parsing and connection creation.
func (r *Redis) Init(meta bindings.Metadata) (err error) {
	r.BaseOperations.Init(meta)

	r.Logger.Debug("Initializing Redis binding")
	r.DBType = "redis"
	r.InitIfNeed = r.initIfNeed
	r.BaseOperations.GetRole = r.GetRole

	// register redis operations
	r.RegisterOperation(bindings.CreateOperation, r.createOps)
	r.RegisterOperation(bindings.DeleteOperation, r.deleteOps)
	r.RegisterOperation(bindings.GetOperation, r.getOps)

	// following are ops for account management
	r.RegisterOperation(ListUsersOp, r.listUsersOps)
	r.RegisterOperation(CreateUserOp, r.createUserOps)
	r.RegisterOperation(DeleteUserOp, r.deleteUserOps)
	r.RegisterOperation(DescribeUserOp, r.describeUserOps)
	r.RegisterOperation(GrantUserRoleOp, r.grantUserRoleOps)
	r.RegisterOperation(RevokeUserRoleOp, r.revokeUserRoleOps)

	return nil
}

func (r *Redis) GetRunningPort() int {
	// parse port from host
	if r.clientSettings != nil {
		host := r.clientSettings.Host
		if strings.Contains(host, ":") {
			parts := strings.Split(host, ":")
			if len(parts) == 2 {
				port, _ := strconv.Atoi(parts[1])
				return port
			}
		}
	}
	return 0
}

func (r *Redis) initIfNeed() bool {
	if r.client == nil {
		go func() {
			if err := r.initDelay(); err != nil {
				r.Logger.Errorf("redis connection init failed: %v", err)
			} else {
				r.Logger.Info("redis connection init succeed.")
			}
		}()
		return true
	}
	return false
}

func (r *Redis) initDelay() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client != nil {
		return nil
	}
	var err error
	r.client, r.clientSettings, err = rediscomponent.ParseClientFromProperties(r.Metadata.Properties, nil)
	if err != nil {
		return err
	}

	r.ctx, r.cancel = context.WithCancel(context.Background())
	err = r.Ping()
	if err != nil {
		return fmt.Errorf("redis binding: error connecting to redis at %s: %s", r.clientSettings.Host, err)
	}
	r.DBPort = r.GetRunningPort()
	return nil
}

func (r *Redis) Ping() error {
	if _, err := r.client.Ping(r.ctx).Result(); err != nil {
		return fmt.Errorf("redis binding: error connecting to redis at %s: %s", r.clientSettings.Host, err)
	}

	return nil
}

// GetLogger returns the logger, implements BaseInternalOps interface.
func (r *Redis) GetLogger() logger.Logger {
	return r.Logger
}

// InternalQuery is used for internal query, implement BaseInternalOps interface.
func (r *Redis) InternalQuery(ctx context.Context, cmd string) ([]byte, error) {
	redisArgs := tokenizeCmd2Args(cmd)
	// Be aware of the result type.
	// type of result could be string, []string, []interface{}, map[interface]interface{}
	// it is not solely determined by the command, but also the redis version.
	result, err := r.query(ctx, redisArgs...)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

// InternalExec is used for internal execution, implement BaseInternalOps interface.
func (r *Redis) InternalExec(ctx context.Context, cmd string) (int64, error) {
	// split command into array of args
	redisArgs := tokenizeCmd2Args(cmd)
	return 0, r.exec(ctx, redisArgs...)
}

func (r *Redis) exec(ctx context.Context, args ...interface{}) error {
	return r.client.Do(ctx, args...).Err()
}

func (r *Redis) query(ctx context.Context, args ...interface{}) (interface{}, error) {
	// parse result into an slice of string
	return r.client.Do(ctx, args...).Result()
}

func (r *Redis) createOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		object = RedisEntry{}

		cmdRender = func(redis RedisEntry) string {
			return fmt.Sprintf("SET %s %s", redis.Key, redis.Data)
		}
		msgRender = func(redis RedisEntry) string {
			return fmt.Sprintf("SET key : %s", redis.Key)
		}
	)

	if err := ParseObjFromRequest(req, defaultRedisEntryParser, defaultRedisEntryValidator, &object); err != nil {
		result := OpsResult{}
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}
	return ExecuteObject(ctx, r, req, bindings.CreateOperation, cmdRender, msgRender, object)
}

func (r *Redis) deleteOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		object    = RedisEntry{}
		cmdRender = func(redis RedisEntry) string {
			return fmt.Sprintf("DEL %s", redis.Key)
		}
		msgRender = func(redis RedisEntry) string {
			return fmt.Sprintf("deleted key: %s", redis.Key)
		}
	)
	if err := ParseObjFromRequest(req, defaultRedisEntryParser, defaultRedisEntryValidator, &object); err != nil {
		result := OpsResult{}
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}

	return ExecuteObject(ctx, r, req, bindings.DeleteOperation, cmdRender, msgRender, object)
}

func (r *Redis) getOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		object    = RedisEntry{}
		cmdRender = func(redis RedisEntry) string {
			return fmt.Sprintf("GET %s", redis.Key)
		}
	)
	if err := ParseObjFromRequest(req, defaultRedisEntryParser, defaultRedisEntryValidator, &object); err != nil {
		result := OpsResult{}
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}
	return QueryObject(ctx, r, req, bindings.GetOperation, cmdRender, nil, object)
}

func (r *Redis) listUsersOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	dataProcessor := func(data interface{}) (interface{}, error) {
		// data is an array of interface{} of string
		// parse redis user name and roles
		results := make([]string, 0)
		err := json.Unmarshal(data.([]byte), &results)
		if err != nil {
			return nil, err
		}
		users := make([]UserInfo, 0)
		for _, userInfo := range results {
			userName := strings.TrimSpace(userInfo)
			if slices.Contains(redisPreDefinedUsers, userName) {
				continue
			}
			user := UserInfo{UserName: userName}
			users = append(users, user)
		}
		if jsonData, err := json.Marshal(users); err != nil {
			return nil, err
		} else {
			return string(jsonData), nil
		}
	}

	cmdRender := func(user UserInfo) string {
		return "ACL USERS"
	}

	return QueryObject(ctx, r, req, ListUsersOp, cmdRender, dataProcessor, UserInfo{})
}

func (r *Redis) describeUserOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		object        = UserInfo{}
		profile       map[string]string
		err           error
		dataProcessor = func(data interface{}) (interface{}, error) {
			// parse it to a map or an []interface
			// try map first
			profile, err = parseCommandAndKeyFromMap(data)
			if err != nil {
				// try list
				profile, err = parseCommandAndKeyFromList(data)
				if err != nil {
					return nil, err
				}
			}

			users := make([]UserInfo, 0)
			user := UserInfo{
				UserName: object.UserName,
				RoleName: (string)(r.priv2Role(profile["commands"] + " " + profile["keys"])),
			}
			users = append(users, user)
			if jsonData, err := json.Marshal(users); err != nil {
				return nil, err
			} else {
				return string(jsonData), nil
			}
		}
		cmdRender = func(user UserInfo) string {
			return fmt.Sprintf("ACL GETUSER %s", user.UserName)
		}
	)

	if err := ParseObjFromRequest(req, DefaultUserInfoParser, UserNameValidator, &object); err != nil {
		result := OpsResult{}
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}

	return QueryObject(ctx, r, req, DescribeUserOp, cmdRender, dataProcessor, object)
}

func parseCommandAndKeyFromList(data interface{}) (map[string]string, error) {
	var (
		redisUserPrivContxt  = []string{"commands", "keys", "channels", "selectors"}
		redisUserInfoContext = []string{"flags", "passwords"}
	)

	profile := make(map[string]string, 0)
	results := make([]interface{}, 0)

	err := json.Unmarshal(data.([]byte), &results)
	if err != nil {
		return nil, err
	}
	// parse line by line
	var context string
	for i := 0; i < len(results); i++ {
		result := results[i]
		switch result := result.(type) {
		case string:
			strVal := strings.TrimSpace(result)
			if len(strVal) == 0 {
				continue
			}
			if slices.Contains(redisUserInfoContext, strVal) {
				i++
				continue
			}
			if slices.Contains(redisUserPrivContxt, strVal) {
				context = strVal
			} else {
				profile[context] = strVal
			}
		case []interface{}:
			selectors := make([]string, 0)
			for _, sel := range result {
				selectors = append(selectors, sel.(string))
			}
			profile[context] = strings.Join(selectors, " ")
		}
	}
	return profile, nil
}

func parseCommandAndKeyFromMap(data interface{}) (map[string]string, error) {
	var (
		redisUserPrivContxt = []string{"commands", "keys", "channels", "selectors"}
	)

	profile := make(map[string]string, 0)
	results := make(map[string]interface{}, 0)

	err := json.Unmarshal(data.([]byte), &results)
	if err != nil {
		return nil, err
	}
	for k, v := range results {
		// each key is string, and each v is eigher a string or a list of string
		if !slices.Contains(redisUserPrivContxt, k) {
			continue
		}

		switch v := v.(type) {
		case string:
			profile[k] = v
		case []interface{}:
			selectors := make([]string, 0)
			for _, sel := range v {
				selectors = append(selectors, sel.(string))
			}
			profile[k] = strings.Join(selectors, " ")
		default:
			return nil, fmt.Errorf("unknown data type: %v", v)
		}
	}
	return profile, nil
}

func (r *Redis) createUserOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		object = UserInfo{}

		cmdRender = func(user UserInfo) string {
			return fmt.Sprintf("ACL SETUSER %s >%s", user.UserName, user.Password)
		}

		msgTplRend = func(user UserInfo) string {
			return fmt.Sprintf("created user: %s, with password: %s", user.UserName, user.Password)
		}
	)

	if err := ParseObjFromRequest(req, DefaultUserInfoParser, UserNameAndPasswdValidator, &object); err != nil {
		result := OpsResult{}
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}

	return ExecuteObject(ctx, r, req, CreateUserOp, cmdRender, msgTplRend, object)
}

func (r *Redis) deleteUserOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		object    = UserInfo{}
		cmdRender = func(user UserInfo) string {
			return fmt.Sprintf("ACL DELUSER %s", user.UserName)
		}

		msgTplRend = func(user UserInfo) string {
			return fmt.Sprintf("deleted user: %s", user.UserName)
		}
	)
	if err := ParseObjFromRequest(req, DefaultUserInfoParser, UserNameValidator, &object); err != nil {
		result := OpsResult{}
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}

	return ExecuteObject(ctx, r, req, DeleteUserOp, cmdRender, msgTplRend, object)
}

func (r *Redis) grantUserRoleOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		succMsgTpl = "role %s granted to user: %s"
	)
	return r.managePrivillege(ctx, req, GrantUserRoleOp, succMsgTpl)
}

func (r *Redis) revokeUserRoleOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	var (
		succMsgTpl = "role %s revoked from user: %s"
	)
	return r.managePrivillege(ctx, req, RevokeUserRoleOp, succMsgTpl)
}

func (r *Redis) managePrivillege(ctx context.Context, req *bindings.InvokeRequest, op bindings.OperationKind, succMsgTpl string) (OpsResult, error) {
	var (
		object = UserInfo{}

		cmdRend = func(user UserInfo) string {
			command := r.role2Priv(op, user.RoleName)
			return fmt.Sprintf("ACL SETUSER %s %s", user.UserName, command)
		}

		msgTplRend = func(user UserInfo) string {
			return fmt.Sprintf(succMsgTpl, user.RoleName, user.UserName)
		}
	)

	if err := ParseObjFromRequest(req, DefaultUserInfoParser, UserNameAndRoleValidator, &object); err != nil {
		result := OpsResult{}
		result[RespTypEve] = RespEveFail
		result[RespTypMsg] = err.Error()
		return result, nil
	}

	return ExecuteObject(ctx, r, req, op, cmdRend, msgTplRend, object)
}

func (r *Redis) role2Priv(op bindings.OperationKind, roleName string) string {
	const (
		grantPrefix  = "+"
		revokePrefix = "-"
	)
	var prefix string
	if op == GrantUserRoleOp {
		prefix = grantPrefix
	} else {
		prefix = revokePrefix
	}
	var command string

	roleType := String2RoleType(roleName)
	switch roleType {
	case SuperUserRole:
		command = fmt.Sprintf("%s@all allkeys", prefix)
	case ReadWriteRole:
		command = fmt.Sprintf("-@all %s@write %s@read allkeys", prefix, prefix)
	case ReadOnlyRole:
		command = fmt.Sprintf("-@all %s@read allkeys", prefix)
	}
	return command
}

func (r *Redis) priv2Role(commands string) RoleType {
	if commands == "-@all" {
		return NoPrivileges
	}
	switch commands {
	case "-@all +@read ~*":
		return ReadOnlyRole
	case "-@all +@write +@read ~*":
		return ReadWriteRole
	case "+@all ~*":
		return SuperUserRole
	default:
		return CustomizedRole
	}
}

func (r *Redis) Close() error {
	if r.cancel == nil {
		return nil
	}
	r.cancel()
	return r.client.Close()
}

func (r *Redis) GetRole(ctx context.Context, request *bindings.InvokeRequest, response *bindings.InvokeResponse) (string, error) {
	// sql exec timeout need to be less than httpget's timeout which default is 1s.
	// ctx1, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	// defer cancel()
	ctx1 := ctx
	section := "Replication"

	var role string
	result, err := r.client.Info(ctx1, section).Result()
	if err != nil {
		r.Logger.Errorf("Role query error: %v", err)
		return role, err
	} else {
		// split the result into lines
		lines := strings.Split(result, "\r\n")
		// find the line with role
		for _, line := range lines {
			if strings.HasPrefix(line, "role:") {
				role = strings.Split(line, ":")[1]
				break
			}
		}
	}
	if role == MASTER {
		return PRIMARY, nil
	}
	if role == SLAVE {
		return SECONDARY, nil
	}
	return role, nil
}

func defaultRedisEntryParser(req *bindings.InvokeRequest, object *RedisEntry) error {
	if req == nil || req.Metadata == nil {
		return fmt.Errorf("no metadata provided")
	}
	object.Key = req.Metadata["key"]
	object.Data = req.Data
	return nil
}

func defaultRedisEntryValidator(redis RedisEntry) error {
	if len(redis.Key) == 0 {
		return fmt.Errorf("redis binding: missing key in request metadata")
	}
	return nil
}

func tokenizeCmd2Args(cmd string) []interface{} {
	args := strings.Split(cmd, " ")
	redisArgs := make([]interface{}, 0, len(args))
	for _, arg := range args {
		redisArgs = append(redisArgs, arg)
	}
	return redisArgs
}
