package component

import (
	"context"
	"strings"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"

	"github.com/go-logr/logr"
)

type DBManager interface {
	Initialize()
	IsRunning() bool
	IsCurrentMemberInCluster(*dcs.Cluster) bool
	IsCurrentMemberHealthy() bool
	IsMemberHealthy(*dcs.Cluster, *dcs.Member) bool
	IsClusterHealthy(context.Context, *dcs.Cluster) bool
	IsClusterInitialized(context.Context, *dcs.Cluster) (bool, error)
	IsLeader(context.Context, *dcs.Cluster) (bool, error)
	IsLeaderMember(context.Context, *dcs.Cluster, *dcs.Member) (bool, error)
	IsDBStartupReady() bool
	Recover()
	AddCurrentMemberToCluster(*dcs.Cluster) error
	DeleteMemberFromCluster(*dcs.Cluster, string) error
	Premote() error
	Demote() error
	Follow(*dcs.Cluster) error
	GetHealthiestMember(*dcs.Cluster, string) *dcs.Member
	// IsHealthiestMember(*dcs.Cluster) bool
	HasOtherHealthyLeader(*dcs.Cluster) *dcs.Member
	HasOtherHealthyMembers(*dcs.Cluster, string) []*dcs.Member
	GetCurrentMemberName() string
	GetMemberAddrs(*dcs.Cluster) []string
	GetLogger() logr.Logger
}

var managers = make(map[string]DBManager)

type DBManagerBase struct {
	CurrentMemberName string
	ClusterCompName   string
	Namespace         string
	DataDir           string
	Logger            logr.Logger
	DBStartupReady    bool
}

func (mgr *DBManagerBase) IsDBStartupReady() bool {
	return mgr.DBStartupReady
}

func (mgr *DBManagerBase) GetLogger() logr.Logger {
	return mgr.Logger
}

func (mgr *DBManagerBase) GetCurrentMemberName() string {
	return mgr.CurrentMemberName
}

func RegisterManager(characterType string, manager DBManager) {
	managers[characterType] = manager
}

func GetManager(characterType string) DBManager {
	characterType = strings.ToLower(characterType)
	return managers[characterType]
}
