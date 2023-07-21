package component

import (
	"context"
	"strings"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
	"github.com/dapr/kit/logger"
)

type DBManager interface {
	IsRunning() bool
	IsCurrentMemberInCluster(*dcs.Cluster) bool
	IsCurrentMemberHealthy() bool
	IsMemberHealthy(*dcs.Cluster, *dcs.Member) bool
	IsClusterHealthy(context.Context, *dcs.Cluster) bool
	IsClusterInitialized(context.Context, *dcs.Cluster) (bool, error)
	InitializeCluster(context.Context, *dcs.Cluster) error
	IsLeader(context.Context, *dcs.Cluster) (bool, error)
	IsLeaderMember(context.Context, *dcs.Cluster, *dcs.Member) (bool, error)
	IsFirstMember() bool
	IsDBStartupReady() bool
	Recover()
	AddCurrentMemberToCluster(*dcs.Cluster) error
	DeleteMemberFromCluster(*dcs.Cluster, string) error
	Promote() error
	Demote() error
	Follow(*dcs.Cluster) error
	GetHealthiestMember(*dcs.Cluster, string) *dcs.Member
	// IsHealthiestMember(*dcs.Cluster) bool
	HasOtherHealthyLeader(*dcs.Cluster) *dcs.Member
	HasOtherHealthyMembers(*dcs.Cluster, string) []*dcs.Member
	GetCurrentMemberName() string
	GetMemberAddrs(*dcs.Cluster) []string

	IsRootCreated(context.Context) (bool, error)
	CreateRoot(context.Context) error
	GetLogger() logger.Logger
}

var managers = make(map[string]DBManager)

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

func (mgr *DBManagerBase) IsFirstMember() bool {
	return strings.HasSuffix(mgr.CurrentMemberName, "-0")
}

func RegisterManager(characterType string, manager DBManager) {
	managers[characterType] = manager
}

func GetManager(characterType string) DBManager {
	characterType = strings.ToLower(characterType)
	return managers[characterType]
}
