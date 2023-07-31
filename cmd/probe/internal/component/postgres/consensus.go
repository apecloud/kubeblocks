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

package postgres

import (
	"context"
	"fmt"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
)

func (mgr *Manager) IsClusterInitializedConsensus(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	if mgr.IsDBStartupReady() {
		return false, nil
	}

	sql := `SELECT usename FROM pg_user WHERE usename = 'replicator';`
	resp, err := mgr.Query(ctx, sql)
	if err != nil {
		mgr.Logger.Errorf("query sql:%s failed, err:%v", sql, err)
		return false, err
	}

	result, err := parseSingleQuery(string(resp))
	if err != nil {
		mgr.Logger.Errorf("parse query failed, err:%v", err)
		return false, err
	}

	return result["username"] == "replicator", nil
}

func (mgr *Manager) InitializeClusterConsensus(ctx context.Context, cluster *dcs.Cluster) error {
	sql := "create role replicator with superuser login password 'replicator';" +
		"create extension if not exists consensus_monitor;"

	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		mgr.Logger.Errorf("exec sql:%s failed, err:%v", sql, err)
		return err
	}

	return nil
}

func (mgr *Manager) GetMemberStateWithPoolConsensus(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	sql := `select paxos_role from consensus_member_status;`

	resp, err := mgr.QueryWithPool(ctx, sql, pool)
	if err != nil {
		mgr.Logger.Errorf("query sql:%s failed, err:%v", sql, err)
		return "", err
	}

	result, err := parseSingleQuery(string(resp))
	if err != nil {
		mgr.Logger.Errorf("parse query failed, err:%v", err)
		return "", err
	}

	// TODO:paxos roles are currently represented by numbers, will change to string in the future
	var role string
	switch result["paxos_role"].(string) {
	case "0":
		role = binding.FOLLOWER
	case "1":
		role = binding.CANDIDATE
	case "2":
		role = binding.LEADER
	case "3":
		role = binding.LEARNER
	default:
		mgr.Logger.Warnf("get invalid role number:%s", result["paxos_role"].(string))
		role = ""
	}

	return role, nil
}

func (mgr *Manager) GetMemberAddrsConsensus(cluster *dcs.Cluster) []string {
	ctx := context.TODO()
	sql := `select ip_port from consensus_cluster_status;`
	var addrs []string

	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		return nil
	}

	pools, err := mgr.GetOtherPoolsWithHosts(ctx, []string{cluster.GetMemberAddr(*leaderMember)})
	if err != nil || pools[0] == nil {
		mgr.Logger.Errorf("Get leader pools failed, err:%v", err)
		return nil
	}

	resp, err := mgr.QueryWithPool(ctx, sql, pools[0])
	if err != nil {
		mgr.Logger.Errorf("query sql:%s failed, err:%v", sql, err)
		return nil
	}

	result, err := parseQuery(string(resp))
	if err != nil || len(*result) == 0 {
		mgr.Logger.Errorf("parse query failed, err:%v", sql, err)
	}
	for _, m := range *result {
		addrs = append(addrs, strings.Split(m["ip_port"], ":")[0])
	}

	return addrs
}

func (mgr *Manager) IsCurrentMemberInClusterConsensus(cluster *dcs.Cluster) bool {
	memberAddrs := mgr.GetMemberAddrs(cluster)
	if memberAddrs == nil {
		mgr.Logger.Errorf("can't get addresses of members")
		return false
	}

	return slices.Contains(memberAddrs, cluster.GetMemberAddrWithName(mgr.CurrentMemberName))
}

func (mgr *Manager) IsMemberHealthyConsensus(cluster *dcs.Cluster, member *dcs.Member) bool {
	ctx := context.TODO()

	pools := []*pgxpool.Pool{nil}
	var err error
	if member != nil && cluster != nil {
		pools, err = mgr.GetOtherPoolsWithHosts(ctx, []string{cluster.GetMemberAddr(*member)})
		if err != nil || pools[0] == nil {
			mgr.Logger.Errorf("Get other pools failed, err:%v", err)
			return false
		}
	}

	sql := `select connected, log_delay_num from consensus_cluster_health where server_id = (select server_id from consensus_member_status);`
	resp, err := mgr.QueryWithPool(ctx, sql, pools[0])
	if err != nil {
		mgr.Logger.Errorf("query sql:%s failed, err:%v", sql, err)
		return false
	}

	result, err := parseSingleQuery(string(resp))
	if err != nil {
		mgr.Logger.Errorf("parse query failed, err:%v", err)
		return false
	}

	connected := result["connected"] == "true"
	logDelayNum, _ := strconv.ParseInt(result["log_delay_num"].(string), 10, 64)

	return connected && logDelayNum <= cluster.HaConfig.GetMaxLagOnSwitchover()
}

func (mgr *Manager) AddCurrentMemberToClusterConsensus(cluster *dcs.Cluster) error {
	ctx := context.TODO()
	sql := fmt.Sprintf(`alter system consensus add follower '%s:%d';`,
		cluster.GetMemberAddrWithName(mgr.CurrentMemberName), config.port)

	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		return errors.New("get leader member failed")
	}

	pools, err := mgr.GetOtherPoolsWithHosts(ctx, []string{cluster.GetMemberAddr(*leaderMember)})
	if err != nil || pools[0] == nil {
		mgr.Logger.Errorf("Get leader pools failed, err:%v", err)
		return err
	}

	_, err = mgr.ExecWithPool(ctx, sql, pools[0])
	if err != nil {
		mgr.Logger.Errorf("exec sql:%s failed, err:%v", sql, err)
		return err
	}

	return nil
}

func (mgr *Manager) DeleteMemberFromClusterConsensus(cluster *dcs.Cluster, host string) error {
	ctx := context.TODO()
	sql := fmt.Sprintf(`alter system consensus drop follower '%s:%d';`,
		cluster.GetMemberAddrWithName(mgr.CurrentMemberName), config.port)

	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		return errors.New("get leader member failed")
	}

	pools, err := mgr.GetOtherPoolsWithHosts(ctx, []string{cluster.GetMemberAddr(*leaderMember)})
	if err != nil || pools[0] == nil {
		mgr.Logger.Errorf("Get leader pools failed, err:%v", err)
		return err
	}

	_, err = mgr.ExecWithPool(ctx, sql, pools[0])
	if err != nil {
		mgr.Logger.Errorf("exec sql:%s failed, err:%v", sql, err)
		return err
	}

	return nil
}

func (mgr *Manager) IsClusterHealthyConsensus(ctx context.Context, cluster *dcs.Cluster) bool {
	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		mgr.Logger.Infof("cluster has no leader, wait for leader to take the lock")
		return false
	}

	return mgr.IsMemberHealthy(cluster, leaderMember)
}

func (mgr *Manager) PromoteConsensus() error {
	ctx := context.TODO()
	if isLeader, err := mgr.IsLeader(context.TODO(), nil); isLeader && err == nil {
		mgr.Logger.Infof("i am already the leader, don't need to promote")
		return nil
	}

	sql := `select ip_port from consensus_cluster_status where server_id = (select current_leader from consensus_member_status);`
	resp, err := mgr.Query(ctx, sql)
	if err != nil {
		mgr.Logger.Errorf("query sql:%s failed, err:%v", sql, err)
		return err
	}

	result, err := parseSingleQuery(string(resp))
	if err != nil {
		return err
	}

	currentLeaderAddr := strings.Split(result["ip_port"].(string), ":")[0]
	pools, err := mgr.GetOtherPoolsWithHosts(ctx, []string{currentLeaderAddr})
	if err != nil || pools[0] == nil {
		mgr.Logger.Errorf("get current leader pool failed, err%v", err)
		return err
	}

	promoteSQL := fmt.Sprintf(`alter system consensus CHANGE LEADER TO '%s:%d';`, viper.GetString("$KB_POD_FQDN"), config.port)
	_, err = mgr.ExecWithPool(ctx, promoteSQL, pools[0])
	if err != nil {
		mgr.Logger.Errorf("exec sql:%s failed, err:%v", sql, err)
		return err
	}

	return nil
}

func (mgr *Manager) DemoteConsensus() error {
	return nil
}

func (mgr *Manager) FollowConsensus(cluster *dcs.Cluster) error {
	return nil
}
