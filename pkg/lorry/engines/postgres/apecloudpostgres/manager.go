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

	"github.com/pkg/errors"
	"github.com/spf13/cast"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/postgres"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type Manager struct {
	postgres.Manager
	memberAddrs  []string
	healthStatus *postgres.ConsensusMemberHealthStatus
}

var _ engines.DBManager = &Manager{}

var Mgr *Manager

func NewManager(properties engines.Properties) (engines.DBManager, error) {
	Mgr = &Manager{}

	baseManager, err := postgres.NewManager(properties)
	if err != nil {
		return nil, errors.Errorf("new base manager failed, err: %v", err)
	}

	Mgr.Manager = *baseManager.(*postgres.Manager)
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
		mgr.Logger.Error(err, "check is leader failed")
		return nil
	}
	mgr.SetIsLeader(isLeader)

	memberAddrs := mgr.GetMemberAddrs(ctx, cluster)
	if memberAddrs == nil {
		mgr.Logger.Error(nil, "get member addrs failed")
		return nil
	}
	mgr.memberAddrs = memberAddrs

	healthStatus, err := mgr.getMemberHealthStatus(ctx, cluster, cluster.GetMemberWithName(mgr.CurrentMemberName))
	if err != nil {
		mgr.Logger.Error(err, "get member health status failed")
		return nil
	}
	mgr.healthStatus = healthStatus

	mgr.DBState = dbState
	return dbState
}

func (mgr *Manager) IsLeader(ctx context.Context, _ *dcs.Cluster) (bool, error) {
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

	return role == models.LEADER, nil
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
	mgr.Logger.Info("DB startup ready")
	return true
}

func (mgr *Manager) isConsensusReadyUp(ctx context.Context) bool {
	sql := `SELECT extname FROM pg_extension WHERE extname = 'consensus_monitor';`
	resp, err := mgr.Query(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, fmt.Sprintf("query sql:%s failed", sql))
		return false
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Error(err, fmt.Sprintf("parse query response:%s failed", string(resp)))
		return false
	}

	return resMap[0]["extname"] != nil
}

func (mgr *Manager) IsClusterInitialized(ctx context.Context, _ *dcs.Cluster) (bool, error) {
	if !mgr.IsFirstMember() {
		mgr.Logger.Info("I am not the first member, just skip and wait for the first member to initialize the cluster.")
		return true, nil
	}

	if !mgr.IsDBStartupReady() {
		return false, nil
	}

	sql := `SELECT usename FROM pg_user WHERE usename = 'replicator';`
	resp, err := mgr.Query(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, fmt.Sprintf("query sql:%s failed", sql))
		return false, err
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Error(err, fmt.Sprintf("parse query response:%s failed", string(resp)))
		return false, err
	}

	return resMap[0]["usename"] != nil, nil
}

func (mgr *Manager) InitializeCluster(ctx context.Context, _ *dcs.Cluster) error {
	sql := "create role replicator with superuser login password 'replicator';" +
		"create extension if not exists consensus_monitor;"

	_, err := mgr.Exec(ctx, sql)
	return err
}

func (mgr *Manager) GetMemberRoleWithHost(ctx context.Context, host string) (string, error) {
	sql := `select paxos_role from consensus_member_status;`

	resp, err := mgr.QueryWithHost(ctx, sql, host)
	if err != nil {
		mgr.Logger.Error(err, fmt.Sprintf("query sql:%s failed", sql))
		return "", err
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Error(err, fmt.Sprintf("parse query response:%s failed", string(resp)))
		return "", err
	}

	// TODO:paxos roles are currently represented by numbers, will change to string in the future
	var role string
	switch cast.ToInt(resMap[0]["paxos_role"]) {
	case 0:
		role = models.FOLLOWER
	case 1:
		role = models.CANDIDATE
	case 2:
		role = models.LEADER
	case 3:
		role = models.LEARNER
	default:
		mgr.Logger.Info(fmt.Sprintf("get invalid role number:%d", cast.ToInt(resMap[0]["paxos_role"])))
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
		mgr.Logger.Error(err, fmt.Sprintf("query %s with leader failed", sql))
		return nil
	}

	result, err := postgres.ParseQuery(string(resp))
	if err != nil {
		mgr.Logger.Error(err, fmt.Sprintf("parse query response:%s failed", string(resp)))
		return nil
	}

	var addrs []string
	for _, m := range result {
		addrs = append(addrs, strings.Split(cast.ToString(m["ip_port"]), ":")[0])
	}

	return addrs
}

func (mgr *Manager) GetMemberAddrWithName(ctx context.Context, cluster *dcs.Cluster, memberName string) string {
	addrs := mgr.GetMemberAddrs(ctx, cluster)
	for _, addr := range addrs {
		if strings.HasPrefix(addr, memberName) {
			return addr
		}
	}
	return ""
}

func (mgr *Manager) IsCurrentMemberInCluster(ctx context.Context, cluster *dcs.Cluster) bool {
	return mgr.GetMemberAddrWithName(ctx, cluster, mgr.CurrentMemberName) != ""
}

func (mgr *Manager) IsCurrentMemberHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	return mgr.IsMemberHealthy(ctx, cluster, cluster.GetMemberWithName(mgr.CurrentMemberName))
}

// IsMemberHealthy firstly get the leader's connection pool,
// because only leader can get the cluster healthy view
func (mgr *Manager) IsMemberHealthy(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) bool {
	healthStatus, err := mgr.getMemberHealthStatus(ctx, cluster, member)
	if errors.Is(err, postgres.ClusterHasNoLeader) {
		mgr.Logger.Info("cluster has no leader, will compete the leader lock")
		return true
	} else if err != nil {
		mgr.Logger.Error(err, "check member healthy failed")
		return false
	}

	return healthStatus.Connected
}

func (mgr *Manager) getMemberHealthStatus(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (*postgres.ConsensusMemberHealthStatus, error) {
	if mgr.DBState != nil && mgr.healthStatus != nil {
		return mgr.healthStatus, nil
	}
	res := &postgres.ConsensusMemberHealthStatus{}

	IPPort := mgr.Config.GetConsensusIPPort(cluster, member.Name)
	sql := fmt.Sprintf(`select connected, log_delay_num from consensus_cluster_health where ip_port = '%s';`, IPPort)
	resp, err := mgr.QueryLeader(ctx, sql, cluster)
	if err != nil {
		return nil, err
	}

	resMap, err := postgres.ParseQuery(string(resp))
	if err != nil {
		return nil, err
	}

	if resMap[0]["connected"] != nil {
		res.Connected = cast.ToBool(resMap[0]["connected"])
	}
	if resMap[0]["log_delay_num"] != nil {
		res.LogDelayNum = cast.ToInt64(resMap[0]["log_delay_num"])
	}

	return res, nil
}

func (mgr *Manager) IsMemberLagging(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (bool, int64) {
	healthStatus, err := mgr.getMemberHealthStatus(ctx, cluster, member)
	if errors.Is(err, postgres.ClusterHasNoLeader) {
		mgr.Logger.Info("cluster has no leader, so member has no lag")
		return false, 0
	} else if err != nil {
		mgr.Logger.Error(err, "check member lag failed")
		return true, cluster.HaConfig.GetMaxLagOnSwitchover() + 1
	}

	return healthStatus.LogDelayNum > cluster.HaConfig.GetMaxLagOnSwitchover(), healthStatus.LogDelayNum
}

func (mgr *Manager) JoinCurrentMemberToCluster(ctx context.Context, cluster *dcs.Cluster) error {
	// use the env KB_POD_FQDN consistently with the startup script
	sql := fmt.Sprintf(`alter system consensus add follower '%s:%d';`,
		viper.GetString("KB_POD_FQDN"), mgr.Config.GetDBPort())

	_, err := mgr.ExecLeader(ctx, sql, cluster)
	if err != nil {
		mgr.Logger.Error(err, fmt.Sprintf("exec sql:%s failed", sql))
		return err
	}

	return nil
}

func (mgr *Manager) LeaveMemberFromCluster(ctx context.Context, _ *dcs.Cluster, host string) error {
	sql := fmt.Sprintf(`alter system consensus drop follower '%s:%d';`,
		host, mgr.Config.GetDBPort())

	// only leader can delete member, so don't need to get pool
	_, err := mgr.ExecWithHost(ctx, sql, "")
	if err != nil {
		mgr.Logger.Error(err, fmt.Sprintf("exec sql:%s failed", sql))
		return err
	}

	return nil
}

// IsClusterHealthy considers the health status of the cluster equivalent to the health status of the leader
func (mgr *Manager) IsClusterHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		mgr.Logger.Info("cluster has no leader, wait for leader to take the lock")
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
		mgr.Logger.Info("i am already the leader, don't need to promote")
		return nil
	}

	currentLeaderAddr, err := mgr.GetLeaderAddr(ctx)
	if err != nil {
		return err
	}

	currentMemberAddr := mgr.GetMemberAddrWithName(ctx, cluster, mgr.CurrentMemberName)
	if currentMemberAddr == "" {
		return errors.New("get current member addr failed")
	}

	promoteSQL := fmt.Sprintf(`alter system consensus CHANGE LEADER TO '%s:%d';`, currentMemberAddr, mgr.Config.GetDBPort())
	_, err = mgr.ExecWithHost(ctx, promoteSQL, currentLeaderAddr)
	if err != nil {
		mgr.Logger.Error(err, fmt.Sprintf("exec sql:%s failed", promoteSQL))
		return err
	}

	return nil
}

func (mgr *Manager) IsPromoted(ctx context.Context) bool {
	isLeader, _ := mgr.IsLeader(ctx, nil)
	return isLeader
}

func (mgr *Manager) Follow(_ context.Context, cluster *dcs.Cluster) error {
	mgr.Logger.Info("current member still follow the leader", "leader name", cluster.Leader.Name)
	return nil
}

func (mgr *Manager) HasOtherHealthyLeader(ctx context.Context, cluster *dcs.Cluster) *dcs.Member {
	if isLeader, err := mgr.IsLeader(ctx, cluster); isLeader && err == nil {
		// I am the leader, just return nil
		return nil
	}

	host, err := mgr.GetLeaderAddr(ctx)
	if err != nil {
		mgr.Logger.Error(err, "get leader addr failed")
		return nil
	}

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
