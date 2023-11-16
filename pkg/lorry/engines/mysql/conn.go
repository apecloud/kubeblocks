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
	"database/sql"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
)

var connectionPoolCache = make(map[string]*sql.DB)

// GetDBConnection returns a DB Connection based on dsn.
func GetDBConnection(dsn string) (*sql.DB, error) {
	if db, ok := connectionPoolCache[dsn]; ok {
		return db, nil
	}

	db, err := sql.Open(adminDatabase, dsn)
	if err != nil {
		return nil, err
	}

	connectionPoolCache[dsn] = db
	return db, nil
}

func (mgr *Manager) GetMemberConnection(cluster *dcs.Cluster, member *dcs.Member) (db *sql.DB, err error) {
	if member != nil && member.Name != mgr.CurrentMemberName {
		addr := cluster.GetMemberAddrWithPort(*member)
		db, err = config.GetDBConnWithAddr(addr)
		if err != nil {
			return nil, err
		}
	} else {
		db = mgr.DB
	}

	return db, nil
}
