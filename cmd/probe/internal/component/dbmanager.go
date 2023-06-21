package component

import (
	"context"

	"github.com/dapr/kit/logger"
)

type DBManager interface {
	Initialize()
	IsInitialized()
	IsRunning()
	IsHealthy()
	IsLeader(context.Context) (bool, error)
	IsDBStartupReady() bool
	Recover()
	AddToCluster()
	Premote()
	Demote()
	GetHealthiestMember()
	HasOtherHealthtyLeader()
	GetLogger() logger.Logger
}

type DBManagerBase struct {
	CurrentMemberName string
	ClusterCompName   string
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
