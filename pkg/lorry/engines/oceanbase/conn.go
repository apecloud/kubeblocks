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

package oceanbase

import (
	"database/sql"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	mysqlengine "github.com/apecloud/kubeblocks/pkg/lorry/engines/mysql"
)

// GetDBConnWithMember retrieves a database connection for a specific member of a cluster.
func (mgr *Manager) GetDBConnWithMember(cluster *dcs.Cluster, member *dcs.Member) (db *sql.DB, err error) {
	if member != nil && member.Name != mgr.CurrentMemberName {
		addr := cluster.GetMemberAddrWithPort(*member)
		db, err = config.GetDBConnWithAddr(addr)
		if err != nil {
			return nil, errors.Wrap(err, "new db connection failed")
		}
	} else {
		db = mgr.DB
	}
	return db, nil
}

func (mgr *Manager) GetMySQLDBConn() (*sql.DB, error) {
	mysqlConfig, err := mysql.ParseDSN(config.URL)
	if err != nil {
		return nil, errors.Wrapf(err, "illegal Data Source Name (DNS) specified by %s", config.URL)
	}
	mysqlConfig.User = fmt.Sprintf("%s@%s", "root", mgr.ReplicaTenant)
	mysqlConfig.Passwd = config.Password
	db, err := mysqlengine.GetDBConnection(mysqlConfig.FormatDSN())
	if err != nil {
		return nil, errors.Wrap(err, "get DB connection failed")
	}

	return db, nil
}

func (mgr *Manager) GetMySQLDBConnWithAddr(addr string) (*sql.DB, error) {
	mysqlConfig, err := mysql.ParseDSN(config.URL)
	if err != nil {
		return nil, errors.Wrapf(err, "illegal Data Source Name (DNS) specified by %s", config.URL)
	}
	mysqlConfig.User = fmt.Sprintf("%s@%s", "root", mgr.ReplicaTenant)
	mysqlConfig.Passwd = config.Password
	mysqlConfig.Addr = addr
	db, err := mysqlengine.GetDBConnection(mysqlConfig.FormatDSN())
	if err != nil {
		return nil, errors.Wrap(err, "get DB connection failed")
	}

	return db, nil
}
