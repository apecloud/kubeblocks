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

	"github.com/apecloud/kubeblocks/lorry/util"
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

func (mgr *Manager) ListUsers(ctx context.Context) ([]util.UserInfo, error) {
	users := []util.UserInfo{}

	err := QueryRowsMap(mgr.DB, listUserSQL, func(rMap RowMap) error {
		user := util.UserInfo{
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
