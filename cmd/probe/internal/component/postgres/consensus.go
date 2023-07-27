package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
)

func (mgr *Manager) GetMemberStateWithPoolConsensus(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	return "", nil
}

func (mgr *Manager) GetMemberAddrsConsensus(cluster *dcs.Cluster) []string {
	return nil
}

func (mgr *Manager) InitializeClusterConsensus(context.Context, *dcs.Cluster) error {
	return nil
}

func (mgr *Manager) IsCurrentMemberInClusterConsensus(cluster *dcs.Cluster) bool {
	return false
}

func (mgr *Manager) IsMemberHealthyConsensus(cluster *dcs.Cluster, member *dcs.Member) bool {
	return false
}

func (mgr *Manager) AddCurrentMemberToClusterConsensus(cluster *dcs.Cluster) error {
	return nil
}

func (mgr *Manager) DeleteMemberFromClusterConsensus(cluster *dcs.Cluster, host string) error {
	return nil
}

func (mgr *Manager) IsClusterHealthyConsensus(ctx context.Context, cluster *dcs.Cluster) bool {
	return false
}

func (mgr *Manager) PromoteConsensus() error {
	return nil
}

func (mgr *Manager) DemoteConsensus() error {
	return nil
}
func (mgr *Manager) FollowConsensus(cluster *dcs.Cluster) error {
	return nil
}
