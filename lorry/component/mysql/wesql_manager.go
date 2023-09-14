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
	"database/sql"
	"fmt"
	"strings"

	"github.com/dapr/kit/logger"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/internal/constant"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
	"github.com/apecloud/kubeblocks/lorry/component"
	"github.com/apecloud/kubeblocks/lorry/dcs"
	"github.com/apecloud/kubeblocks/lorry/util"
)

const (
	Role        = "ROLE"
	CurrentRole = "CURRENT_ROLE"
	Leader      = "Leader"
)

type WesqlManager struct {
	Manager
}

var _ component.DBManager = &WesqlManager{}

func NewWesqlManager(logger logger.Logger) (*WesqlManager, error) {
	db, err := config.GetLocalDBConn()
	if err != nil {
		return nil, errors.Wrap(err, "connect to MySQL")
	}

	defer func() {
		if err != nil {
			derr := db.Close()
			if derr != nil {
				logger.Errorf("failed to close: %v", err)
			}
		}
	}()

	currentMemberName := viper.GetString(constant.KBEnvPodName)
	if currentMemberName == "" {
		return nil, fmt.Errorf("KB_POD_NAME is not set")
	}

	serverID, err := component.GetIndex(currentMemberName)
	if err != nil {
		return nil, err
	}

	mgr := &WesqlManager{
		Manager: Manager{
			DBManagerBase: component.DBManagerBase{
				CurrentMemberName: currentMemberName,
				ClusterCompName:   viper.GetString(constant.KBEnvClusterCompName),
				Namespace:         viper.GetString(constant.KBEnvNamespace),
				Logger:            logger,
			},
			DB:       db,
			serverID: uint(serverID) + 1,
		},
	}

	component.RegisterManager("mysql", util.Consensus, mgr)
	return mgr, nil
}

func (mgr *WesqlManager) InitializeCluster(ctx context.Context, cluster *dcs.Cluster) error {
	return nil
}

func (mgr *WesqlManager) IsLeader(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	role, err := mgr.GetRole(ctx)

	if err != nil {
		return false, err
	}

	if strings.EqualFold(role, Leader) {
		return true, nil
	}

	return false, nil
}

func (mgr *WesqlManager) IsLeaderMember(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (bool, error) {
	if member == nil {
		return false, nil
	}

	leaderMember := mgr.GetLeaderMember(ctx, cluster)
	if leaderMember == nil {
		return false, nil
	}

	if leaderMember.Name != member.Name {
		return false, nil
	}

	return true, nil
}

func (mgr *WesqlManager) InitiateCluster(cluster *dcs.Cluster) error {
	return nil
}

func (mgr *WesqlManager) GetMemberAddrs(ctx context.Context, cluster *dcs.Cluster) []string {
	addrs := make([]string, 0, 3)
	clusterInfo := mgr.GetClusterInfo(ctx, cluster)
	clusterInfo = strings.Split(clusterInfo, "@")[0]
	for _, addr := range strings.Split(clusterInfo, ";") {
		if !strings.Contains(addr, ":") {
			continue
		}
		addrs = append(addrs, strings.Split(addr, "#")[0])
	}

	return addrs
}

func (mgr *WesqlManager) GetAddrWithMemberName(ctx context.Context, cluster *dcs.Cluster, memberName string) string {
	addrs := mgr.GetMemberAddrs(ctx, cluster)
	for _, addr := range addrs {
		if strings.HasPrefix(addr, memberName) {
			return addr
		}
	}
	return ""
}

func (mgr *WesqlManager) IsCurrentMemberInCluster(ctx context.Context, cluster *dcs.Cluster) bool {
	clusterInfo := mgr.GetClusterInfo(ctx, cluster)
	return strings.Contains(clusterInfo, mgr.CurrentMemberName)
}

func (mgr *WesqlManager) IsMemberLagging(context.Context, *dcs.Cluster, *dcs.Member) (bool, int64) {
	return false, 0
}

func (mgr *WesqlManager) Recover(context.Context) error {
	return nil
}

func (mgr *WesqlManager) JoinCurrentMemberToCluster(context.Context, *dcs.Cluster) error {
	return nil
}

func (mgr *WesqlManager) LeaveMemberFromCluster(ctx context.Context, cluster *dcs.Cluster, memberName string) error {
	db, err := mgr.GetLeaderConn(ctx, cluster)
	if err != nil {
		mgr.Logger.Infof("Get leader conn failed: %v", err)
		return err
	}
	addr := mgr.GetAddrWithMemberName(ctx, cluster, memberName)
	if addr == "" {
		mgr.Logger.Infof("member %s already deleted", memberName)
		return nil
	}

	sql := fmt.Sprintf("call dbms_consensus.downgrade_follower('%s');"+
		"call dbms_consensus.drop_learner('%s');", addr, addr)
	_, err = db.ExecContext(ctx, sql)
	if err != nil {
		mgr.Logger.Warnf("delete member from db cluster failed: %v", err)
		return errors.Wrapf(err, "error executing %s", sql)
	}
	return nil
}

func (mgr *WesqlManager) IsClusterHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	db, err := mgr.GetLeaderConn(ctx, cluster)
	if err != nil {
		mgr.Logger.Infof("Get leader conn failed: %v", err)
		return false
	}
	if db == nil {
		return false
	}

	defer db.Close()
	var leaderRecord RowMap
	sql := "select * from information_schema.wesql_cluster_global;"
	err = QueryRowsMap(db, sql, func(rMap RowMap) error {
		if rMap.GetString(Role) == Leader {
			leaderRecord = rMap
		}
		return nil
	})
	if err != nil {
		mgr.Logger.Errorf("error executing %s: %v", sql, err)
		return false
	}

	if len(leaderRecord) > 0 {
		return true
	}
	return false
}

// IsClusterInitialized is a method to check if cluster is initailized or not
func (mgr *WesqlManager) IsClusterInitialized(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	clusterInfo := mgr.GetClusterInfo(ctx, nil)
	if clusterInfo != "" {
		return true, nil
	}

	return false, nil
}

func (mgr *WesqlManager) GetClusterInfo(ctx context.Context, cluster *dcs.Cluster) string {
	var db *sql.DB
	var err error
	if cluster != nil {
		db, err = mgr.GetLeaderConn(ctx, cluster)
		if err != nil {
			mgr.Logger.Infof("Get leader conn failed: %v", err)
			return ""
		}
		if db != nil {
			defer db.Close()
		}
	} else {
		db = mgr.DB

	}
	var clusterID, clusterInfo string
	err = db.QueryRowContext(ctx, "select cluster_id, cluster_info from mysql.consensus_info").
		Scan(&clusterID, &clusterInfo)
	if err != nil {
		mgr.Logger.Error("Cluster info query failed: %v", err)
	}
	return clusterInfo
}

func (mgr *WesqlManager) Promote(ctx context.Context, cluster *dcs.Cluster) error {
	isLeader, _ := mgr.IsLeader(ctx, nil)
	if isLeader {
		return nil
	}

	db, err := mgr.GetLeaderConn(ctx, cluster)
	if err != nil {
		return errors.Wrap(err, "Get leader conn failed")
	}
	if db != nil {
		defer db.Close()
	}

	currentMember := cluster.GetMemberWithName(mgr.GetCurrentMemberName())
	addr := cluster.GetMemberAddr(*currentMember)
	resp, err := db.Exec(fmt.Sprintf("call dbms_consensus.change_leader('%s:13306');", addr))
	if err != nil {
		mgr.Logger.Errorf("promote err: %v", err)
		return err
	}

	mgr.Logger.Infof("promote success, resp:%v", resp)
	return nil
}

func (mgr *WesqlManager) IsPromoted(ctx context.Context) bool {
	isLeader, _ := mgr.IsLeader(ctx, nil)
	return isLeader
}

func (mgr *WesqlManager) Demote(context.Context) error {
	return nil
}

func (mgr *WesqlManager) Follow(ctx context.Context, cluster *dcs.Cluster) error {
	return nil
}

func (mgr *WesqlManager) GetHealthiestMember(cluster *dcs.Cluster, candidate string) *dcs.Member {
	return nil
}

func (mgr *WesqlManager) HasOtherHealthyLeader(ctx context.Context, cluster *dcs.Cluster) *dcs.Member {
	clusterLocalInfo, err := mgr.GetClusterLocalInfo(ctx)
	if err != nil || clusterLocalInfo == nil {
		mgr.Logger.Errorf("Get cluster local info failed: %v", err)
		return nil
	}

	if clusterLocalInfo.GetString(Role) == Leader {
		// I am the leader, just return nil
		return nil
	}

	leaderAddr := clusterLocalInfo.GetString(CurrentRole)
	if leaderAddr == "" {
		return nil
	}
	leaderParts := strings.Split(leaderAddr, ".")
	if len(leaderParts) > 0 {
		return cluster.GetMemberWithName(leaderParts[0])
	}

	return nil
}

// HasOtherHealthyMembers checks if there are any healthy members, excluding the leader
func (mgr *WesqlManager) HasOtherHealthyMembers(ctx context.Context, cluster *dcs.Cluster, leader string) []*dcs.Member {
	members := make([]*dcs.Member, 0)
	for _, member := range cluster.Members {
		if member.Name == leader {
			continue
		}
		if !mgr.IsMemberHealthy(ctx, cluster, &member) {
			continue
		}
		members = append(members, &member)
	}

	return members
}
