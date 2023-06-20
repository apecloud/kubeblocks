package highavailability

import (
	"context"
	"time"

	"github.com/dapr/kit/logger"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/component"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
)

type Ha struct {
	ctx       context.Context
	dbManager component.DBManager
	dcs       dcs.DCS
	logger    logger.Logger
}

func NewHa(logger logger.Logger) *Ha {

	dcs, _ := dcs.NewKubernetesStore(logger)
	ha := &Ha{
		ctx:    context.Background(),
		dcs:    dcs,
		logger: logger,
	}
	return ha
}

func (ha *Ha) RunCycle() {

	cluster, _ := ha.dcs.GetCluster()
	ha.logger.Debugf("cluster: %v", cluster)
	//switch {
	//case !dbManger.IsRunning():
	//	logger.Infof("DB Service is not running,  wait for sqlctl to start it")
	//	if dcs.HasLock() {
	//		dcs.ReleaseLock()
	//	}

	//case !dbManger.IsHealthy():
	//	logger.Infof("DB Service is not healthy,  do some recover")
	//	if dcs.HasLock() {
	//		dcs.ReleaseLock()
	//	}
	//	dbManager.Recover()

	//case !cluster.HasThisMember():
	//	logger.Infof("Current node is not in cluster, add it to cluster")
	//	dbManager.AddToCluser(cluster)
	//	cluster.AddThisMember()

	//case cluster.IsUnlocked():
	//	logger.Infof("Cluster has no leader, attemp to take the leader")
	//	candidate := ""
	//	if cluster.SwitchOver != nil && cluster.SwitchOver.Candinate != "" {
	//		candiate = cluster.SwitchOver.Candinate
	//	}
	//	healthiestMember := ha.dbManager.GetHealthiesMember(cluster, candinate)
	//	if healthiestMember != nil && healthiestMember.Name == dbManager.MemberName {
	//		if dcs.attempAcquireLeader() {
	//			dbManager.Premote()
	//		}
	//	}

	//case cluster.GetLeader() == dbManager.MemberName:
	//	logger.Infof("This member is Cluster's leader")
	//	if dbManager.isLeader() {
	//		logger.Infof("Refresh leader ttl")
	//		dcs.UpdateLeader()
	//	} else if dbManger.HasOtherHealthyLeader() {
	//		logger.Infof("Release leader")
	//		dcs.ReleaseLock()
	//	}

	//case cluster.SwitchOver.Leader != dbManager.Name && dbManager.IsLeader():
	//	logger.Infof("Cluster has no leader, attemp to take the leader")
	//	dbManager.Demote()

	//case cluster.SwitchOver.Leader == dbManager.Name && !dbManager.IsLeader():
	//	logger.Infof("Cluster has no leader, attemp to take the leader")
	//	dbManager.Premote()
	//}
}

func (ha *Ha) Start() {
	// for !ha.dbManager.IsInitialized() {
	// 	ha.logger.Infof("Waiting for the database cluster to be initialized.")
	// 	// TODO: implement dbmanager initialize to replace pod's entrypoint scripts
	// 	// if I am the node of index 0, then do initialization
	// 	// ha.dbManager.Initialize()
	// 	time.Sleep(1 * time.Second)
	// }

	isExist, _ := ha.dcs.IsLockExist()
	for !isExist {
		if ok, _ := ha.dbManager.IsLeader(context.Background()); ok {
			ha.dcs.Initialize()
			break
		}
		ha.logger.Infof("Waiting for the database Leader to be ready.")
		time.Sleep(1 * time.Second)
		isExist, _ = ha.dcs.IsLockExist()
	}

	for true {
		ha.RunCycle()
		time.Sleep(1 * time.Second)
	}
}

func (ha *Ha) ShutdownAndWait() {
}
