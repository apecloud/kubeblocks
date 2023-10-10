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

package polardbx

import (
	"context"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/lorry/engines"
	"github.com/apecloud/kubeblocks/lorry/engines/mysql"
)

const (
	POLARDBX_SERVICE_TYPE = "polardbx"
)

type Manager struct {
	mysql.Manager
}

var _ engines.DBManager = &Manager{}

func NewManager(properties engines.Properties) (engines.DBManager, error) {
	logger := ctrl.Log.WithName("MySQL")
	mysqlMgr, err := mysql.NewManager(properties)
	if err != nil {
		return nil, err
	}
	mgr := &Manager{
		Manager: *mysqlMgr.(*mysql.Manager),
	}
	mgr.Logger = logger

	return mgr, nil
}

func (mgr *Manager) GetRole(ctx context.Context) (string, error) {
	sql := "select role from information_schema.alisql_cluster_local"

	rows, err := mgr.DB.QueryContext(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, "error executing sql", "sql", sql)
		return "", errors.Wrapf(err, "error executing %s", sql)
	}

	defer func() {
		_ = rows.Close()
		_ = rows.Err()
	}()

	var role string
	var isReady bool
	for rows.Next() {
		if err = rows.Scan(&role); err != nil {
			mgr.Logger.Error(err, "Role query error")
			return role, err
		}
		isReady = true
	}
	if isReady {
		return role, nil
	}
	return "", errors.Errorf("exec sql %s failed: no data returned", sql)
}
