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

package apecloudpostgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dapr/kit/logger"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"golang.org/x/exp/slices"

	"github.com/apecloud/kubeblocks/cmd/probe/internal"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component/postgres"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
)

type Manager struct {
	postgres.Manager
	memberAddrs []string
}

var Mgr *Manager

func NewManager(logger logger.Logger) (*Manager, error) {
	Mgr = &Manager{}

	baseManager, err := postgres.NewManager(logger)
	if err != nil {
		return nil, errors.Errorf("new base manager failed, err: %v", err)
	}

	Mgr.Manager = *baseManager
	component.RegisterManager("postgresql", internal.Consensus, Mgr)

	return Mgr, nil
}

func (mgr *Manager) GetDBState(ctx context.Context, cluster *dcs.Cluster) *dcs.DBState {
	mgr.DBState = nil
	mgr.UnsetIsLeader()
	dbState := &dcs.DBState{
		Extra: map[string]string{},
	}

	isLeader, err := mgr.IsLeader(ctx, cluster)
	if err != nil {
		mgr.Logger.Errorf("check is leader failed, err:%v", err)
		return nil
	}
	mgr.SetIsLeader(isLeader)

	memberAddrs := mgr.GetMemberAddrs(ctx, cluster)
	if memberAddrs == nil {
		mgr.Logger.Errorf("get member addrs failed")
		return nil
	}
	mgr.memberAddrs = memberAddrs

	mgr.DBState = dbState
	return dbState
}

func (mgr *Manager) IsLeader(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	isSet, isLeader := mgr.GetIsLeader()
	if isSet {
		return isLeader, nil
	}

	return mgr.IsLeaderWithHost(ctx, "")
}

func (mgr *Manager) IsLeaderWithHost(ctx context.Context, host string) (bool, error) {
	role, err := mgr.GetMemberRoleWithHost(ctx, host)
	if err != nil {
		return false, errors.Errorf("check is leader with host:%s failed, err:%v", host, err)
	}

	mgr.Logger.Infof("get member:%s role:%s", host, role)
	return role == binding.LEADER, nil
}

func (mgr *Manager) IsDBStartupReady() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if mgr.DBStartupReady {
		return true
	}

	if !mgr.IsPgReady(ctx) {
		return false
	}

	if !mgr.isConsensusReadyUp(ctx) {
		return false
	}

	mgr.DBStartupReady = true
	mgr.Logger.Infof("DB startup ready")
	return true
}

func (mgr *Manager) isConsensusReadyUp(ctx context.Context) bool {
	sql := `SELECT extname FROM pg_extension WHERE extname = 'consensus_monitor';`
	resp, err := mgr.Query(ctx, sql)
	if err != nil {
		mgr.Logger.Errorf("query sql:%s failed, err:%v", sql, err)
		return false
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Errorf("parse query response:%s failed, err:%v", string(resp), err)
		return false
	}

	return resMap[0]["extname"] != nil
}

func (mgr *Manager) IsClusterInitialized(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	if !mgr.IsFirstMember() {
		mgr.Logger.Infof("I am not the first member, just skip and wait for the first member to initialize the cluster.")
		return true, nil
	}

	if !mgr.IsDBStartupReady() {
		return false, nil
	}

	sql := `SELECT usename FROM pg_user WHERE usename = 'replicator';`
	resp, err := mgr.Query(ctx, sql)
	if err != nil {
		mgr.Logger.Errorf("query sql:%s failed, err:%v", sql, err)
		return false, err
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Errorf("parse query response:%s failed, err:%v", string(resp), err)
		return false, err
	}

	return resMap[0]["usename"] != nil, nil
}

func (mgr *Manager) InitializeCluster(ctx context.Context, cluster *dcs.Cluster) error {
	sql := "create role replicator with superuser login password 'replicator';" +
		"create extension if not exists consensus_monitor;"

	_, err := mgr.Exec(ctx, sql)
	return err
}

func (mgr *Manager) GetMemberRoleWithHost(ctx context.Context, host string) (string, error) {
	sql := `select paxos_role from consensus_member_status;`

	resp, err := mgr.QueryWithHost(ctx, sql, host)
	if err != nil {
		mgr.Logger.Errorf("query sql:%s failed, err:%v", sql, err)
		return "", err
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Errorf("parse query response:%s failed, err:%v", string(resp), err)
		return "", err
	}

	// TODO:paxos roles are currently represented by numbers, will change to string in the future
	var role string
	switch cast.ToInt(resMap[0]["paxos_role"]) {
	case 0:
		role = binding.FOLLOWER
	case 1:
		role = binding.CANDIDATE
	case 2:
		role = binding.LEADER
	case 3:
		role = binding.LEARNER
	default:
		mgr.Logger.Warnf("get invalid role number:%d", cast.ToInt(resMap[0]["paxos_role"]))
		role = ""
	}

	return role, nil
}

func (mgr *Manager) GetMemberAddrs(ctx context.Context, cluster *dcs.Cluster) []string {
	if mgr.DBState != nil && mgr.memberAddrs != nil {
		return mgr.memberAddrs
	}

	sql := `select ip_port from consensus_cluster_status;`
	resp, err := mgr.QueryLeader(ctx, sql, cluster)
	if err != nil {
		mgr.Logger.Errorf("query %s with leader failed, err:%v", sql, err)
		return nil
	}

	result, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Errorf("parse query response:%s failed, err:%v", string(resp), err)
		return nil
	}

	var addrs []string
	for _, m := range result {
		addrs = append(addrs, strings.Split(cast.ToString(m["ip_port"]), ":")[0])
	}

	return addrs
}

func (mgr *Manager) IsCurrentMemberInCluster(ctx context.Context, cluster *dcs.Cluster) bool {
	memberAddrs := mgr.GetMemberAddrs(ctx, cluster)
	// AddCurrentMemberToCluster is executed only when memberAddrs are successfully obtained and memberAddrs not Contains CurrentMember
	if memberAddrs != nil && !slices.Contains(memberAddrs, cluster.GetMemberAddrWithName(mgr.CurrentMemberName)) {
		return false
	}
	return true
}

func (mgr *Manager) IsCurrentMemberHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	return mgr.IsMemberHealthy(ctx, cluster, cluster.GetMemberWithName(mgr.CurrentMemberName))
}

// IsMemberHealthy firstly get the leader's connection pool,
// because only leader can get the cluster healthy view
func (mgr *Manager) IsMemberHealthy(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) bool {
	IPPort := mgr.Config.GetConsensusIPPort(cluster, member.Name)

	sql := fmt.Sprintf(`select connected, log_delay_num from consensus_cluster_health where ip_port = '%s';`, IPPort)
	resp, err := mgr.QueryLeader(ctx, sql, cluster)
	if errors.Is(err, postgres.ClusterHasNoLeader) {
		mgr.Logger.Infof("cluster has no leader, will compete the leader lock")
		return true
	} else if err != nil {
		mgr.Logger.Errorf("query %s with leader failed, err:%v", sql, err)
		return false
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Errorf("parse query response:%s failed, err:%v", string(resp), err)
		return false
	}

	var connected bool
	var logDelayNum int64
	if resMap[0]["connected"] != nil {
		connected = cast.ToBool(resMap[0]["connected"])
	}
	if resMap[0]["log_delay_num"] != nil {
		logDelayNum = cast.ToInt64(resMap[0]["log_delay_num"])
	}

	return connected && logDelayNum <= cluster.HaConfig.GetMaxLagOnSwitchover()
}

func (mgr *Manager) JoinCurrentMemberToCluster(ctx context.Context, cluster *dcs.Cluster) error {
	sql := fmt.Sprintf(`alter system consensus add follower '%s:%d';`,
		cluster.GetMemberAddrWithName(mgr.CurrentMemberName), mgr.Config.GetDBPort())

	_, err := mgr.ExecLeader(ctx, sql, cluster)
	if err != nil {
		mgr.Logger.Errorf("exec sql:%s failed, err:%v", sql, err)
		return err
	}

	return nil
}

func (mgr *Manager) LeaveMemberFromCluster(ctx context.Context, cluster *dcs.Cluster, host string) error {
	sql := fmt.Sprintf(`alter system consensus drop follower '%s:%d';`,
		host, mgr.Config.GetDBPort())

	// only leader can delete member, so don't need to get pool
	_, err := mgr.ExecWithHost(ctx, sql, "")
	if err != nil {
		mgr.Logger.Errorf("exec sql:%s failed, err:%v", sql, err)
		return err
	}

	return nil
}

// IsClusterHealthy considers the health status of the cluster equivalent to the health status of the leader
func (mgr *Manager) IsClusterHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		mgr.Logger.Infof("cluster has no leader, wait for leader to take the lock")
		// when cluster has no leader, the health status of the cluster is assumed to be true by default,
		// in order to proceed with the logic of competing for the leader lock
		return true
	}

	if leaderMember.Name == mgr.CurrentMemberName {
		// if the member is leader, then its health status will check in IsMemberHealthy later
		return true
	}

	return mgr.IsMemberHealthy(ctx, cluster, leaderMember)
}

func (mgr *Manager) Promote(ctx context.Context, cluster *dcs.Cluster) error {
	if isLeader, err := mgr.IsLeader(ctx, nil); isLeader && err == nil {
		mgr.Logger.Infof("i am already the leader, don't need to promote")
		return nil
	}

	// TODO:will get leader ip_port from consensus_member_status directly in the future
	sql := `select ip_port from consensus_cluster_status where server_id = (select current_leader from consensus_member_status);`
	resp, err := mgr.Query(ctx, sql)
	if err != nil {
		mgr.Logger.Errorf("query sql:%s failed, err:%v", sql, err)
		return err
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		return errors.Errorf("parse query response:%s failed, err:%v", string(resp), err)
	}

	currentLeaderAddr := strings.Split(cast.ToString(resMap[0]["ip_port"]), ":")[0]
	promoteSQL := fmt.Sprintf(`alter system consensus CHANGE LEADER TO '%s:%d';`, cluster.GetMemberAddrWithName(mgr.CurrentMemberName), mgr.Config.GetDBPort())
	_, err = mgr.ExecOthers(ctx, promoteSQL, currentLeaderAddr)
	if err != nil {
		mgr.Logger.Errorf("exec sql:%s failed, err:%v", sql, err)
		return err
	}

	return nil
}

func (mgr *Manager) IsPromoted(ctx context.Context) bool {
	isLeader, _ := mgr.IsLeader(ctx, nil)
	return isLeader
}

func (mgr *Manager) HasOtherHealthyLeader(ctx context.Context, cluster *dcs.Cluster) *dcs.Member {
	if isLeader, err := mgr.IsLeader(ctx, cluster); isLeader && err == nil {
		// I am the leader, just return nil
		return nil
	}

	// TODO:will get leader ip_port from consensus_member_status directly in the future
	sql := `select ip_port from consensus_cluster_status where server_id = (select current_leader from consensus_member_status);`
	resp, err := mgr.Query(ctx, sql)
	if err != nil {
		mgr.Logger.Errorf("query sql:%s failed, err:%v", sql, err)
		return nil
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Errorf("parse query response:%s failed, err:%v", err)
		return nil
	}

	host := strings.Split(cast.ToString(resMap[0]["ip_port"]), ":")[0]
	leaderName := strings.Split(host, ".")[0]
	if len(leaderName) > 0 {
		return cluster.GetMemberWithName(leaderName)
	}

	return nil
}

func (mgr *Manager) HasOtherHealthyMembers(ctx context.Context, cluster *dcs.Cluster, leader string) []*dcs.Member {
	members := make([]*dcs.Member, 0)

	for i, m := range cluster.Members {
		if m.Name != leader && mgr.IsMemberHealthy(ctx, cluster, &m) {
			members = append(members, &cluster.Members[i])
		}
	}

	return members
}
