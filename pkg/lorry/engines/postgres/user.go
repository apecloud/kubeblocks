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

package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/exp/slices"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
)

const (
	listUserTpl = `
	SELECT usename AS userName, valuntil <now() AS expired,  usesuper,
	ARRAY(SELECT
		case
			when b.rolname = 'pg_read_all_data' THEN 'readonly'
			when b.rolname = 'pg_write_all_data' THEN 'readwrite'
		else b.rolname
		end
	FROM pg_catalog.pg_auth_members m
	JOIN pg_catalog.pg_roles b ON (m.roleid = b.oid)
	WHERE m.member = usesysid ) as roles
	FROM pg_catalog.pg_user
	WHERE usename <> 'postgres' and usename  not like 'kb%'
	ORDER BY usename;
	`
	descUserTpl = `
	SELECT usename AS userName,  valuntil <now() AS expired, usesuper,
	ARRAY(SELECT
	 case
		 when b.rolname = 'pg_read_all_data' THEN 'readonly'
		 when b.rolname = 'pg_write_all_data' THEN 'readwrite'
	 else b.rolname
	 end
	FROM pg_catalog.pg_auth_members m
	JOIN pg_catalog.pg_roles b ON (m.roleid = b.oid)
	WHERE m.member = usesysid ) as roles
	FROM pg_user
	WHERE usename = '%s';
	`
	createUserTpl         = "CREATE USER %s WITH PASSWORD '%s';"
	dropUserTpl           = "DROP USER IF EXISTS %s;"
	grantTpl              = "GRANT %s TO %s;"
	revokeTpl             = "REVOKE %s FROM %s;"
	listSystemAccountsTpl = "SELECT rolname FROM pg_catalog.pg_roles WHERE pg_roles.rolname LIKE 'kb%'"
)

func (mgr *Manager) ListUsers(ctx context.Context) ([]models.UserInfo, error) {
	data, err := mgr.Query(ctx, listUserTpl)
	if err != nil {
		mgr.Logger.Error(err, "error executing %s")
		return nil, err
	}

	return pgUserRolesProcessor(data)
}

func (mgr *Manager) ListSystemAccounts(ctx context.Context) ([]models.UserInfo, error) {
	data, err := mgr.Query(ctx, listSystemAccountsTpl)
	if err != nil {
		mgr.Logger.Error(err, "error executing %s")
		return nil, err
	}
	type roleInfo struct {
		Rolname string `json:"rolname"`
	}
	var roles []roleInfo
	if err := json.Unmarshal(data, &roles); err != nil {
		return nil, err
	}

	users := []models.UserInfo{}
	for _, role := range roles {
		user := models.UserInfo{
			RoleName: role.Rolname,
		}
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

	users, err := pgUserRolesProcessor(data)
	if err != nil {
		mgr.Logger.Error(err, "parse data failed", "data", string(data))
		return nil, err
	}

	if len(users) > 0 {
		return &users[0], nil
	}
	return nil, nil
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
	if models.SuperUserRole.EqualTo(roleName) {
		sql = "ALTER USER " + userName + " WITH SUPERUSER;"
	} else {
		roleDesc, _ := role2PGRole(roleName)
		sql = fmt.Sprintf(grantTpl, roleDesc, userName)
	}
	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, "execute sql failed", "sql", sql)
		return err
	}

	return nil
}

func (mgr *Manager) RevokeUserRole(ctx context.Context, userName, roleName string) error {
	var sql string
	if models.SuperUserRole.EqualTo(roleName) {
		sql = "ALTER USER " + userName + " WITH NOSUPERUSER;"
	} else {
		roleDesc, _ := role2PGRole(roleName)
		sql = fmt.Sprintf(revokeTpl, roleDesc, userName)
	}

	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, "execute sql failed", "sql", sql)
		return err
	}

	return nil
}

// post-processing
func pgUserRolesProcessor(data interface{}) ([]models.UserInfo, error) {
	type pgUserInfo struct {
		UserName string   `json:"username"`
		Expired  bool     `json:"expired"`
		Super    bool     `json:"usesuper"`
		Roles    []string `json:"roles"`
	}
	// parse data to struct
	var pgUsers []pgUserInfo
	err := json.Unmarshal(data.([]byte), &pgUsers)
	if err != nil {
		return nil, err
	}
	// parse roles
	users := make([]models.UserInfo, len(pgUsers))
	for i := range pgUsers {
		users[i] = models.UserInfo{
			UserName: pgUsers[i].UserName,
		}

		if pgUsers[i].Expired {
			users[i].Expired = "T"
		} else {
			users[i].Expired = "F"
		}

		// parse Super attribute
		if pgUsers[i].Super {
			pgUsers[i].Roles = append(pgUsers[i].Roles, string(models.SuperUserRole))
		}

		// convert to RoleType and sort by weight
		roleTypes := make([]models.RoleType, 0)
		for _, role := range pgUsers[i].Roles {
			roleTypes = append(roleTypes, models.String2RoleType(role))
		}
		slices.SortFunc(roleTypes, models.SortRoleByWeight)
		if len(roleTypes) > 0 {
			users[i].RoleName = string(roleTypes[0])
		}
	}
	return users, nil
}

func role2PGRole(roleName string) (string, error) {
	roleType := models.String2RoleType(roleName)
	switch roleType {
	case models.ReadWriteRole:
		return "pg_write_all_data", nil
	case models.ReadOnlyRole:
		return "pg_read_all_data", nil
	}
	return "", fmt.Errorf("role name: %s is not supported", roleName)
}
