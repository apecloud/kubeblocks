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
	"database/sql"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
)

func (mgr *Manager) Failover(ctx context.Context, cluster *dcs.Cluster, candidate string) error {
	if mgr.ReplicaTenant == "" {
		return errors.New("the cluster has no replica tenant set")
	}

	candidateComponentName, err := getCompnentName(candidate)
	if err != nil {
		return err
	}
	candidateStore, _ := dcs.NewKubernetesStore()
	candidateStore.SetCompName(candidateComponentName)
	candidateCluster, err := candidateStore.GetCluster()
	if err != nil {
		return err
	}

	if len(candidateCluster.Members) != 1 {
		return errors.Errorf("candidate component has %d replicas, "+
			"the replicas count need to be 1", len(candidateCluster.Members))
	}

	candidateMember := candidateCluster.Members[0]

	candidateAddr := fmt.Sprintf("%s:%s", candidateMember.PodIP, candidateMember.DBPort)
	candidateDB, err := config.GetDBConnWithAddr(candidateAddr)
	if err != nil {
		mgr.Logger.Info("new candidatedb connection failed", "error", err)
		return err
	}

	err = mgr.activeTenant(ctx, candidateDB)
	if err != nil {
		return err
	}

	return nil
}

func (mgr *Manager) activeTenant(ctx context.Context, db *sql.DB) error {
	primaryTenant := "ALTER SYSTEM ACTIVATE STANDBY TENANT = " + mgr.ReplicaTenant
	_, err := db.Exec(primaryTenant)
	if err != nil {
		mgr.Logger.Info("activate standby tenant failed", "error", err)
		return err
	}

	var tenantRole, roleStatus string
	queryTenant := fmt.Sprintf("SELECT TENANT_ROLE, SWITCHOVER_STATUS FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME='%s'", mgr.ReplicaTenant)
	for {
		err := db.QueryRowContext(ctx, queryTenant).Scan(&tenantRole, &roleStatus)
		if err != nil {
			mgr.Logger.Info("query zone info failed", "error", err)
			return err
		}

		if tenantRole == PRIMARY && roleStatus == normalStatus {
			break
		}
		time.Sleep(time.Second)
	}

	return nil
}
