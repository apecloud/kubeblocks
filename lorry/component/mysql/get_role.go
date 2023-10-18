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

	"github.com/pkg/errors"

	. "github.com/apecloud/kubeblocks/lorry/binding"
	. "github.com/apecloud/kubeblocks/lorry/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func (mgr *Manager) GetRole(ctx context.Context) (string, error) {
	workloadType := viper.GetString(constant.KBEnvWorkloadType)
	if strings.EqualFold(workloadType, Replication) {
		return mgr.GetRoleForReplication(ctx)
	}
	return mgr.GetRoleForConsensus(ctx)
}

func (mgr *Manager) GetRoleForReplication(ctx context.Context) (string, error) {
	slaveRunning, err := mgr.isSlaveRunning(ctx)
	if err != nil {
		return "", err
	}
	if slaveRunning {
		return SECONDARY, nil
	}

	hasSlave, err := mgr.hasSlaveHosts(ctx)
	if err != nil {
		return "", err
	}
	if hasSlave {
		return PRIMARY, nil
	}

	isReadonly, err := mgr.IsReadonly(ctx, nil, nil)
	if err != nil {
		return "", err
	}
	if isReadonly {
		// TODO: in case of diskfull lock, dababase will be set readonly,
		// how to deal with this situation
		return SECONDARY, nil
	}

	return PRIMARY, nil
}

func (mgr *Manager) GetRoleForConsensus(ctx context.Context) (string, error) {
	sql := "select CURRENT_LEADER, ROLE, SERVER_ID  from information_schema.wesql_cluster_local"

	rows, err := mgr.DB.QueryContext(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, fmt.Sprintf("error executing %s", sql))
		return "", errors.Wrapf(err, "error executing %s", sql)
	}

	defer func() {
		_ = rows.Close()
		_ = rows.Err()
	}()

	var curLeader string
	var role string
	var serverID string
	var isReady bool
	for rows.Next() {
		if err = rows.Scan(&curLeader, &role, &serverID); err != nil {
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
