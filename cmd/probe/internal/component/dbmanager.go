package component

import (
	"context"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
	"github.com/dapr/kit/logger"
)

type DBManager interface {
	Initialize()
	IsRunning() bool
	IsCurrentMemberInCluster(*dcs.Cluster) bool
	IsCurrentMemberHealthy() bool
	IsMemberHealthy(*dcs.Cluster, *dcs.Member) bool
	IsClusterHealthy(context.Context, *dcs.Cluster) bool
	IsClusterInitialized(context.Context, *dcs.Cluster) (bool, error)
	IsLeader(context.Context) (bool, error)
	IsDBStartupReady() bool
	Recover()
	AddCurrentMemberToCluster(*dcs.Cluster) error
	DeleteMemberFromCluster(*dcs.Cluster, string) error
	Premote() error
	Demote() error
	GetHealthiestMember(*dcs.Cluster, string) *dcs.Member
	// IsHealthiestMember(*dcs.Cluster) bool
	HasOtherHealthyLeader(*dcs.Cluster) *dcs.Member
	HasOtherHealthyMembers(*dcs.Cluster) []*dcs.Member
	GetCurrentMemberName() string
	GetMemberAddrs(*dcs.Cluster) []string
	GetLogger() logger.Logger
}

type DBManagerBase struct {
	CurrentMemberName string
	ClusterCompName   string
	Namespace         string
	DataDir           string
	Logger            logger.Logger
	DBStartupReady    bool
}

func (mgr *DBManagerBase) IsDBStartupReady() bool {
	return mgr.DBStartupReady
}

func (mgr *DBManagerBase) GetLogger() logger.Logger {
	return mgr.Logger
}

func (mgr *DBManagerBase) GetCurrentMemberName() string {
	return mgr.CurrentMemberName
}
