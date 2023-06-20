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
	StartupReady() bool
	Recover()
	AddToCluster()
	Premote()
	Demote()
	GetHealthiestMember()
	HasOtherHealthtyLeader()
}

type DBManagerBase struct {
	CurrentMemberName string
	DataDir           string
	Logger            logger.Logger
	DBStartupReady    bool
}

func (mgr *DBManagerBase) StartupReady() bool {
	return mgr.DBStartupReady
}
