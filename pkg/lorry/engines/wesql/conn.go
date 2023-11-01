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

package wesql

import (
	"context"
	"database/sql"
	"strings"

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
)

// GetDBConnWithMember retrieves a database connection for a specific member of a cluster.
func (mgr *Manager) GetDBConnWithMember(cluster *dcs.Cluster, member *dcs.Member) (*sql.DB, error) {
	var db *sql.DB
	var err error
	if member != nil {
		addr := cluster.GetMemberAddrWithPort(*member)
		db, err = config.GetDBConnWithAddr(addr)
		if err != nil {
			return nil, errors.Wrap(err, "new db connection failed")
		}
	}
	return db, nil
}

// GetLeaderConn retrieves a database connection to the leader member of a cluster.
func (mgr *Manager) GetLeaderConn(ctx context.Context, cluster *dcs.Cluster) (*sql.DB, error) {
	mgr.Logger.Info("Get leaader from dcs cluster")
	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		mgr.Logger.Info("Get leaader from db cluster local")
		leaderMember = mgr.GetLeaderMember(ctx, cluster)
	}
	if leaderMember == nil {
		return nil, errors.New("the cluster has no leader")
	}
	return mgr.GetDBConnWithMember(cluster, leaderMember)
}

// GetLeaderMember retrieves the leader member of a cluster
func (mgr *Manager) GetLeaderMember(ctx context.Context, cluster *dcs.Cluster) *dcs.Member {
	clusterLocalInfo, err := mgr.GetClusterLocalInfo(ctx)
	if err != nil || clusterLocalInfo == nil {
		mgr.Logger.Error(err, "Get cluster local info failed")
		return nil
	}

	leaderAddr := clusterLocalInfo.GetString("CURRENT_LEADER")
	if leaderAddr == "" {
		return nil
	}
	leaderParts := strings.Split(leaderAddr, ".")
	if len(leaderParts) > 0 {
		return cluster.GetMemberWithName(leaderParts[0])
	}

	return nil
}
