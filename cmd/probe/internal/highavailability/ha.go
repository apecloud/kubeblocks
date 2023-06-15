package highavailability

import (
	"context"
	"time"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/component"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
	"github.com/dubbogo/gost/log/logger"
)

type Ha struct {
	ctx       context.Context
	dbManager component.DBManager
	dcs       dcs.DCS
}

func (ha *Ha) runCycle() {
	cluster := ha.dcs.GetCluster()
	switch {
	case !dbManger.IsRunning():
		logger.Infof("DB Service is not running,  wait for sqlctl to start it")
		if dcs.HasLock() {
			dcs.ReleaseLock()
		}

	case !dbManger.IsHealthy():
		logger.Infof("DB Service is not healthy,  do some recover")
		if dcs.HasLock() {
			dcs.ReleaseLock()
		}
		dbManager.Recover()

	case !cluster.HasThisMember():
		logger.Infof("Current node is not in cluster, add it to cluster")
		dbManager.AddToCluser(cluster)
		cluster.AddThisMember()

	case cluster.IsUnlocked():
		logger.Infof("Cluster has no leader, attemp to take the leader")
		candidate := ""
		if cluster.SwitchOver != nil && cluster.SwitchOver.Candinate != "" {
			candiate = cluster.SwitchOver.Candinate
		}
		healthiestMember := ha.dbManager.GetHealthiesMember(cluster, candinate)
		if healthiestMember != nil && healthiestMember.Name == dbManager.MemberName {
			if dcs.attempAcquireLeader() {
				dbManager.Premote()
			}
		}

	case cluster.GetLeader() == dbManager.MemberName:
		logger.Infof("This member is Cluster's leader")
		if dbManager.isLeader() {
			logger.Infof("Refresh leader ttl")
			dcs.UpdateLeader()
		} else if dbManger.HasOtherHealthyLeader() {
			logger.Infof("Release leader")
			dcs.ReleaseLeader()
		}

	case cluster.SwitchOver.Leader != dbManager.Name && dbManager.IsLeader():
		logger.Infof("Cluster has no leader, attemp to take the leader")
		dbManager.Demote()

	case cluster.SwitchOver.Leader == dbManager.Name && !dbManager.IsLeader():
		logger.Infof("Cluster has no leader, attemp to take the leader")
		dbManager.Premote()
	}
}

func (ha *Ha) Start() {
	for true {
		ha.runCycle()
		time.sleep(1 * time.Second)
	}
}

func (ha *Ha) ShutdownAndWait() {
}
