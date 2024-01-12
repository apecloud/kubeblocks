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
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
)

func (mgr *Manager) GetReplicaRole(ctx context.Context, _ *dcs.Cluster) (string, error) {
	if mgr.ReplicaTenant == "" {
		mgr.Logger.V(1).Info("the cluster has no replica tenant set")
		return "", nil
	}

	var zoneCount int
	zoneSQL := `select count(distinct(zone)) as count from oceanbase.__all_zone where zone!=''`
	err := mgr.DB.QueryRowContext(ctx, zoneSQL).Scan(&zoneCount)
	if err != nil {
		mgr.Logger.Info("query zone info failed", "error", err)
		return "", err
	}

	if zoneCount > 1 {
		mgr.Logger.Info("zone count is more than 1, return no role", "zone count", zoneCount)
		return "", nil
	}

	sql := fmt.Sprintf("SELECT TENANT_ROLE FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME='%s'", mgr.ReplicaTenant)

	rows, err := mgr.DB.QueryContext(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, fmt.Sprintf("error executing %s", sql))
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
