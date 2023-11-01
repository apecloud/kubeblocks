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

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
)

func (mgr *Manager) GetReplicaRole(ctx context.Context, cluster *dcs.Cluster) (string, error) {
	if cluster == nil {
		return "", errors.New("cluster not found")
	}

	if !cluster.IsLocked() {
		return "", errors.New("cluster has no leader lease")
	}

	if cluster.Leader.Name != mgr.CurrentMemberName {
		return models.SECONDARY, nil
	}

	return models.PRIMARY, nil
	// slaveRunning, err := mgr.isSlaveRunning(ctx)
	// if err != nil {
	// 	return "", err
	// }
	// if slaveRunning {
	// 	return SECONDARY, nil
	// }

	// hasSlave, err := mgr.hasSlaveHosts(ctx)
	// if err != nil {
	// 	return "", err
	// }
	// if hasSlave {
	// 	return PRIMARY, nil
	// }

	// isReadonly, err := mgr.IsReadonly(ctx, nil, nil)
	// if err != nil {
	// 	return "", err
	// }
	// if isReadonly {
	// 	// TODO: in case of diskfull lock, dababase will be set readonly,
	// 	// how to deal with this situation
	// 	return SECONDARY, nil
	// }

	// return PRIMARY, nil
}

// func (mgr *Manager) isSlaveRunning(ctx context.Context) (bool, error) {
// 	var rowMap = mgr.slaveStatus
//
// 	if len(rowMap) == 0 {
// 		return false, nil
// 	}
// 	ioRunning := rowMap.GetString("Slave_IO_Running")
// 	sqlRunning := rowMap.GetString("Slave_SQL_Running")
// 	if ioRunning == "Yes" || sqlRunning == "Yes" {
// 		return true, nil
// 	}
// 	return false, nil
// }
//
// func (mgr *Manager) hasSlaveHosts(ctx context.Context) (bool, error) {
// 	sql := "show slave hosts"
// 	var rowMap RowMap
//
// 	err := QueryRowsMap(mgr.DB, sql, func(rMap RowMap) error {
// 		rowMap = rMap
// 		return nil
// 	})
// 	if err != nil {
// 		mgr.Logger.Error(err, fmt.Sprintf("error executing %s", sql))
// 		return false, err
// 	}
//
// 	if len(rowMap) == 0 {
// 		return false, nil
// 	}
//
// 	return true, nil
// }
