package component

import (
	"context"
	"fmt"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
	"github.com/dapr/kit/logger"
)

type DBManager interface {
	Initialize()
	IsClusterInitialized() (bool, error)
	IsRunning()
	IsHealthy() bool
	IsLeader(context.Context) (bool, error)
	IsDBStartupReady() bool
	Recover()
	AddToCluster()
	Premote() error
	Demote() error
	GetHealthiestMember(*dcs.Cluster, string) *dcs.Member
	HasOtherHealthyLeader(*dcs.Cluster) *dcs.Member
	GetCurrentMemberName() string
	GetMemberAddr(string) string
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

func (mgr *DBManagerBase) GetMemberAddr(podName string) string {
	return fmt.Sprintf("%s.%s-headless.%s.svc", podName, mgr.ClusterCompName, mgr.Namespace)
}
