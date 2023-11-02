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

package mysql

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
)

const (
	superUserPriv = "SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, RELOAD, SHUTDOWN, PROCESS, FILE, REFERENCES, INDEX, ALTER, SHOW DATABASES, SUPER, CREATE TEMPORARY TABLES, LOCK TABLES, EXECUTE, REPLICATION SLAVE, REPLICATION CLIENT, CREATE VIEW, SHOW VIEW, CREATE ROUTINE, ALTER ROUTINE, CREATE USER, EVENT, TRIGGER, CREATE TABLESPACE, CREATE ROLE, DROP ROLE ON *.*"
	readWritePriv = "SELECT, INSERT, UPDATE, DELETE ON *.*"
	readOnlyRPriv = "SELECT ON *.*"
	noPriv        = "USAGE ON *.*"

	listUserSQL  = "SELECT user AS userName, CASE password_expired WHEN 'N' THEN 'F' ELSE 'T' END as expired FROM mysql.user WHERE host = '%' and user <> 'root' and user not like 'kb%';"
	showGrantSQL = "SHOW GRANTS FOR '%s'@'%%';"
	getUserSQL   = `
	SELECT user AS userName, CASE password_expired WHEN 'N' THEN 'F' ELSE 'T' END as expired
	FROM mysql.user
	WHERE host = '%%' and user <> 'root' and user not like 'kb%%' and user ='%s';"
	`
	createUserSQL         = "CREATE USER '%s'@'%%' IDENTIFIED BY '%s';"
	deleteUserSQL         = "DROP USER IF EXISTS '%s'@'%%';"
	grantSQL              = "GRANT %s TO '%s'@'%%';"
	revokeSQL             = "REVOKE %s FROM '%s'@'%%';"
	listSystemAccountsSQL = "SELECT user AS userName FROM mysql.user WHERE host = '%' and user like 'kb%';"
)

func (mgr *Manager) ListUsers(ctx context.Context) ([]models.UserInfo, error) {
	users := []models.UserInfo{}

	err := QueryRowsMap(mgr.DB, listUserSQL, func(rMap RowMap) error {
		user := models.UserInfo{
			UserName: rMap.GetString("userName"),
			Expired:  rMap.GetString("expired"),
		}
		users = append(users, user)
		return nil
	})
	if err != nil {
		mgr.Logger.Error(err, "error executing %s")
		return nil, err
	}
	return users, nil
}

func (mgr *Manager) ListSystemAccounts(ctx context.Context) ([]models.UserInfo, error) {
	users := []models.UserInfo{}

	err := QueryRowsMap(mgr.DB, listSystemAccountsSQL, func(rMap RowMap) error {
		user := models.UserInfo{
			UserName: rMap.GetString("userName"),
		}
		users = append(users, user)
		return nil
	})
	if err != nil {
		mgr.Logger.Error(err, "error executing %s")
		return nil, err
	}
	return users, nil
}

func (mgr *Manager) DescribeUser(ctx context.Context, userName string) (*models.UserInfo, error) {
	user := &models.UserInfo{}
	// only keep one role name of the highest privilege
	userRoles := make([]models.RoleType, 0)

	sql := fmt.Sprintf(showGrantSQL, userName)

	err := QueryRowsMap(mgr.DB, sql, func(rMap RowMap) error {
		for k, v := range rMap {
			if user.UserName == "" {
				user.UserName = strings.TrimPrefix(strings.TrimSuffix(k, "@%"), "Grants for ")
			}
			mysqlRoleType := priv2Role(strings.TrimPrefix(v.String, "GRANT "))
			userRoles = append(userRoles, mysqlRoleType)
		}

		return nil
	})
	if err != nil {
		mgr.Logger.Error(err, "execute sql failed", "sql", sql)
		return nil, err
	}

	slices.SortFunc(userRoles, models.SortRoleByWeight)
	if len(userRoles) > 0 {
		user.RoleName = (string)(userRoles[0])
	}
	return user, nil
}

func (mgr *Manager) CreateUser(ctx context.Context, userName, password string) error {
	sql := fmt.Sprintf(createUserSQL, userName, password)

	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, "execute sql failed", "sql", sql)
		return err
	}

	return nil
}

func (mgr *Manager) DeleteUser(ctx context.Context, userName string) error {
	sql := fmt.Sprintf(deleteUserSQL, userName)

	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, "execute sql failed", "sql", sql)
		return err
	}

	return nil
}

func (mgr *Manager) GrantUserRole(ctx context.Context, userName, roleName string) error {
	// render sql stmts
	roleDesc, _ := role2Priv(roleName)
	// update privilege
	sql := fmt.Sprintf(grantSQL, roleDesc, userName)
	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, "execute sql failed", "sql", sql)
		return err
	}

	return nil
}

func (mgr *Manager) RevokeUserRole(ctx context.Context, userName, roleName string) error {
	// render sql stmts
	roleDesc, _ := role2Priv(roleName)
	// update privilege
	sql := fmt.Sprintf(revokeSQL, roleDesc, userName)
	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, "execute sql failed", "sql", sql)
		return err
	}

	return nil
}

func role2Priv(roleName string) (string, error) {
	roleType := models.String2RoleType(roleName)
	switch roleType {
	case models.SuperUserRole:
		return superUserPriv, nil
	case models.ReadWriteRole:
		return readWritePriv, nil
	case models.ReadOnlyRole:
		return readOnlyRPriv, nil
	}
	return "", fmt.Errorf("role name: %s is not supported", roleName)
}

func priv2Role(priv string) models.RoleType {
	if strings.HasPrefix(priv, readOnlyRPriv) {
		return models.ReadOnlyRole
	}
	if strings.HasPrefix(priv, readWritePriv) {
		return models.ReadWriteRole
	}
	if strings.HasPrefix(priv, superUserPriv) {
		return models.SuperUserRole
	}
	if strings.HasPrefix(priv, noPriv) {
		return models.NoPrivileges
	}
	return models.CustomizedRole
}
