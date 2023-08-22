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

	"github.com/dapr/kit/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/spf13/viper"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type Manager struct {
	component.DBManagerBase
	Pool         PgxPoolIFace
	Proc         *process.Process
	syncStandbys *PGStandby
	isLeader     bool
	workLoadType string
}

var Mgr *Manager

func NewManager(logger logger.Logger) (*Manager, error) {
	pool, err := pgxpool.NewWithConfig(context.Background(), config.pool)
	if err != nil {
		return nil, errors.Errorf("unable to ping the DB: %v", err)
	}

	Mgr = &Manager{
		DBManagerBase: component.DBManagerBase{
			CurrentMemberName: viper.GetString(constant.KBEnvPodName),
			ClusterCompName:   viper.GetString(constant.KBEnvClusterCompName),
			Namespace:         viper.GetString(constant.KBEnvNamespace),
			Logger:            logger,
			DataDir:           viper.GetString(PGDATA),
		},
		Pool:         pool,
		workLoadType: viper.GetString(constant.KBEnvWorkloadType),
	}

	component.RegisterManager("postgresql", Mgr)
	return Mgr, nil
}

// GetMemberRoleWithHost get specified member's role with its connection
func (mgr *Manager) GetMemberRoleWithHost(ctx context.Context, host string) (string, error) {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.GetMemberRoleWithHostConsensus(ctx, host)
	case Replication:
		return mgr.GetMemberRoleWithHostReplication(ctx, host)
	default:
		return "", InvalidWorkLoadType
	}
}

func (mgr *Manager) IsDBStartupReady() bool {
	ctx := context.TODO()
	if mgr.DBStartupReady {
		return true
	}

	if !mgr.IsPgReady(ctx) {
		return false
	}

	// For Consensus, probe relies on the consensus_monitor view.
	if mgr.workLoadType == Consensus && !mgr.IsConsensusReadyUp(ctx) {
		return false
	}

	mgr.DBStartupReady = true
	mgr.Logger.Infof("DB startup ready")
	return true
}

func (mgr *Manager) GetDBState(ctx context.Context, cluster *dcs.Cluster) *dcs.DBState {
	switch mgr.workLoadType {
	case Consensus:
		return nil
	case Replication:
		return mgr.GetDBStateReplication(ctx, cluster)
	default:
		mgr.Logger.Errorf("get DB State failed, err:%v", InvalidWorkLoadType)
		return nil
	}
}

func (mgr *Manager) IsLeader(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	if mgr.DBState != nil {
		return mgr.isLeader, nil
	}

	return mgr.CheckMemberIsLeader(ctx, "")
}

// CheckMemberIsLeader determines whether a specific member is the leader, using its connection
func (mgr *Manager) CheckMemberIsLeader(ctx context.Context, host string) (bool, error) {
	role, err := mgr.GetMemberRoleWithHost(ctx, host)
	if err != nil {
		return false, errors.Wrap(err, "check is leader")
	}

	return role == binding.PRIMARY || role == binding.LEADER, nil
}

func (mgr *Manager) GetMemberAddrs(cluster *dcs.Cluster) []string {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.GetMemberAddrsConsensus(cluster)
	case Replication:
		return cluster.GetMemberAddrs()
	default:
		mgr.Logger.Errorf("get member addrs failed, err:%v", InvalidWorkLoadType)
		return nil
	}
}

func (mgr *Manager) InitializeCluster(ctx context.Context, cluster *dcs.Cluster) error {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.InitializeClusterConsensus(ctx, cluster)
	case Replication:
		return nil
	default:
		return InvalidWorkLoadType
	}
}

func (mgr *Manager) IsCurrentMemberInCluster(ctx context.Context, cluster *dcs.Cluster) bool {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.IsCurrentMemberInClusterConsensus(ctx, cluster)
	case Replication:
		return true
	default:
		mgr.Logger.Errorf("check current member in cluster failed, err:%v", InvalidWorkLoadType)
		return false
	}
}

func (mgr *Manager) IsCurrentMemberHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	return mgr.IsMemberHealthy(ctx, cluster, cluster.GetMemberWithName(mgr.CurrentMemberName))
}

func (mgr *Manager) IsMemberHealthy(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) bool {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.IsMemberHealthyConsensus(ctx, cluster, member)
	case Replication:
		return mgr.IsMemberHealthyReplication(ctx, cluster, member)
	default:
		mgr.Logger.Errorf("check current member healthy failed, err:%v", InvalidWorkLoadType)
		return false
	}
}

func (mgr *Manager) Recover(context.Context) error {
	return nil
}

func (mgr *Manager) AddCurrentMemberToCluster(cluster *dcs.Cluster) error {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.AddCurrentMemberToClusterConsensus(cluster)
	case Replication:
		return nil
	default:
		return InvalidWorkLoadType
	}
}

func (mgr *Manager) DeleteMemberFromCluster(cluster *dcs.Cluster, host string) error {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.DeleteMemberFromClusterConsensus(cluster, host)
	case Replication:
		return nil
	default:
		return InvalidWorkLoadType
	}
}

func (mgr *Manager) IsClusterHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.IsClusterHealthyConsensus(ctx, cluster)
	case Replication:
		return true
	default:
		mgr.Logger.Errorf("check cluster healthy failed, err:%v", InvalidWorkLoadType)
		return false
	}
}

func (mgr *Manager) IsClusterInitialized(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.IsClusterInitializedConsensus(ctx, cluster)
	case Replication:
		// for replication, the setup script imposes a constraint where the successful startup of the primary database (db0)
		// is a prerequisite for the successful launch of the remaining databases.
		return mgr.IsDBStartupReady(), nil
	default:
		return false, InvalidWorkLoadType
	}
}

func (mgr *Manager) Promote(ctx context.Context) error {
	if isLeader, err := mgr.IsLeader(ctx, nil); isLeader && err == nil {
		mgr.Logger.Infof("i am already the leader, don't need to promote")
		return nil
	}

	switch mgr.workLoadType {
	case Consensus:
		return mgr.PromoteConsensus(ctx)
	case Replication:
		return mgr.PromoteReplication()
	default:
		return InvalidWorkLoadType
	}
}

func (mgr *Manager) Demote(ctx context.Context) error {
	mgr.Logger.Infof("current member demoting: %s", mgr.CurrentMemberName)

	switch mgr.workLoadType {
	case Consensus:
		return nil
	case Replication:
		return mgr.DemoteReplication(ctx)
	default:
		return InvalidWorkLoadType
	}
}

func (mgr *Manager) Follow(ctx context.Context, cluster *dcs.Cluster) error {
	switch mgr.workLoadType {
	case Consensus:
		return nil
	case Replication:
		return mgr.FollowReplication(ctx, cluster)
	default:
		return InvalidWorkLoadType
	}
}

func (mgr *Manager) GetHealthiestMember(cluster *dcs.Cluster, candidate string) *dcs.Member {
	// TODO: check SynchronizedToLeader and compare the lags
	return nil
}

func (mgr *Manager) HasOtherHealthyLeader(ctx context.Context, cluster *dcs.Cluster) *dcs.Member {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.HasOtherHealthyLeaderConsensus(ctx, cluster)
	case Replication:
		return nil
	default:
		mgr.Logger.Errorf("check other healthy leader failed, err:%v", InvalidWorkLoadType)
		return nil
	}
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

func (mgr *Manager) IsLeaderMember(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (bool, error) {
	return mgr.CheckMemberIsLeader(ctx, cluster.GetMemberAddr(*member))
}

func (mgr *Manager) IsRootCreated(ctx context.Context) (bool, error) {
	return true, nil
}

func (mgr *Manager) CreateRoot(ctx context.Context) error {
	return nil
}
