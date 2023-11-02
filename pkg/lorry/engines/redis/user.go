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
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
)

const (
	listUserTpl   = "ACL USERS"
	descUserTpl   = "ACL GETUSER %s"
	createUserTpl = "ACL SETUSER %s >%s"
	dropUserTpl   = "ACL DELUSER %s"
	grantTpl      = "ACL SETUSER %s %s"
	revokeTpl     = "ACL SETUSER %s %s"
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

func (mgr *Manager) ListUsers(ctx context.Context) ([]models.UserInfo, error) {
	data, err := mgr.Query(ctx, listUserTpl)
	if err != nil {
		mgr.Logger.Error(err, "error executing %s")
		return nil, err
	}

	results := make([]string, 0)
	err = json.Unmarshal(data, &results)
	if err != nil {
		return nil, err
	}
	users := make([]models.UserInfo, 0)
	for _, userInfo := range results {
		userName := strings.TrimSpace(userInfo)
		if slices.Contains(redisPreDefinedUsers, userName) {
			continue
		}
		user := models.UserInfo{UserName: userName}
		users = append(users, user)
	}
	return users, nil
}

func (mgr *Manager) ListSystemAccounts(ctx context.Context) ([]models.UserInfo, error) {
	data, err := mgr.Query(ctx, listUserTpl)
	if err != nil {
		mgr.Logger.Error(err, "error executing %s")
		return nil, err
	}

	results := make([]string, 0)
	err = json.Unmarshal(data, &results)
	if err != nil {
		return nil, err
	}
	users := make([]models.UserInfo, 0)
	for _, userInfo := range results {
		userName := strings.TrimSpace(userInfo)
		if !slices.Contains(redisPreDefinedUsers, userName) {
			continue
		}
		user := models.UserInfo{UserName: userName}
		users = append(users, user)
	}
	return users, nil
}

func (mgr *Manager) DescribeUser(ctx context.Context, userName string) (*models.UserInfo, error) {
	sql := fmt.Sprintf(descUserTpl, userName)

	data, err := mgr.Query(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, "execute sql failed", "sql", sql)
		return nil, err
	}

	// parse it to a map or an []interface
	// try map first
	var profile map[string]string
	profile, err = parseCommandAndKeyFromMap(data)
	if err != nil {
		// try list
		profile, err = parseCommandAndKeyFromList(data)
		if err != nil {
			return nil, err
		}
	}

	user := &models.UserInfo{
		UserName: userName,
		RoleName: (string)(priv2Role(profile["commands"] + " " + profile["keys"])),
	}
	return user, nil
}

func (mgr *Manager) CreateUser(ctx context.Context, userName, password string) error {
	sql := fmt.Sprintf(createUserTpl, userName, password)

	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, "execute sql failed", "sql", sql)
		return err
	}

	return nil
}

func (mgr *Manager) DeleteUser(ctx context.Context, userName string) error {
	sql := fmt.Sprintf(dropUserTpl, userName)

	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, "execute sql failed", "sql", sql)
		return err
	}

	return nil
}

func (mgr *Manager) GrantUserRole(ctx context.Context, userName, roleName string) error {
	var sql string
	command := role2Priv("+", roleName)
	sql = fmt.Sprintf(grantTpl, userName, command)
	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, "execute sql failed", "sql", sql)
		return err
	}

	return nil
}

func (mgr *Manager) RevokeUserRole(ctx context.Context, userName, roleName string) error {
	var sql string
	command := role2Priv("-", roleName)
	sql = fmt.Sprintf(revokeTpl, userName, command)
	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, "execute sql failed", "sql", sql)
		return err
	}

	return nil
}

func role2Priv(prefix, roleName string) string {
	var command string

	roleType := models.String2RoleType(roleName)
	switch roleType {
	case models.SuperUserRole:
		command = fmt.Sprintf("%s@all allkeys", prefix)
	case models.ReadWriteRole:
		command = fmt.Sprintf("-@all %s@write %s@read allkeys", prefix, prefix)
	case models.ReadOnlyRole:
		command = fmt.Sprintf("-@all %s@read allkeys", prefix)
	}
	return command
}

func priv2Role(commands string) models.RoleType {
	if commands == "-@all" {
		return models.NoPrivileges
	}
	switch commands {
	case "-@all +@read ~*":
		return models.ReadOnlyRole
	case "-@all +@write +@read ~*":
		return models.ReadWriteRole
	case "+@all ~*":
		return models.SuperUserRole
	default:
		return models.CustomizedRole
	}
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
		// each key is string, and each v is string or list of string
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
