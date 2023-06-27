package highavailability

import (
	"context"
	"time"

	"github.com/dapr/kit/logger"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/component"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component/mongodb"
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
		ctx:       context.Background(),
		dcs:       dcs,
		logger:    logger,
		dbManager: mongodb.Mgr,
	}
	return ha
}

func (ha *Ha) RunCycle() {
	cluster, err := ha.dcs.GetCluster()
	if err != nil {
		ha.logger.Infof("Get Cluster err: %v.", err)
		return
	}

	switch {
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

	case !cluster.IsLocked():
		ha.logger.Infof("Cluster has no leader, attemp to take the leader")
		candidate := ""
		if cluster.Switchover != nil && cluster.Switchover.Candidate != "" {
			candidate = cluster.Switchover.Candidate
		}
		healthiestMember := ha.dbManager.GetHealthiestMember(cluster, candidate)
		if healthiestMember != nil && healthiestMember.Name == ha.dbManager.GetCurrentMemberName() {
			if ha.dcs.AttempAcquireLock() == nil {
				ha.dbManager.Premote()
			}
		}

	case ha.dcs.HasLock():
		ha.logger.Infof("This member is Cluster's leader")
		if cluster.Switchover != nil {
			if cluster.Switchover.Leader == ha.dbManager.GetCurrentMemberName() {
				ha.dbManager.Demote()
				ha.dcs.ReleaseLock()
				break
			} else if cluster.Switchover.Candidate == "" || cluster.Switchover.Candidate == ha.dbManager.GetCurrentMemberName() {
				ha.dcs.DeleteSwitchover()
			}
		}

		if ok, _ := ha.dbManager.IsLeader(context.TODO()); ok {
			ha.logger.Infof("Refresh leader ttl")
			ha.dcs.UpdateLock()
		} else if ha.dbManager.HasOtherHealthyLeader(cluster) != nil {
			ha.logger.Infof("Release leader")
			ha.dcs.ReleaseLock()
		} else {
			ha.dbManager.Premote()
			ha.dcs.UpdateLock()
		}

	case !ha.dcs.HasLock():
		if cluster.Switchover != nil {
			break
		}
		if ok, _ := ha.dbManager.IsLeader(context.TODO()); ok {
			ha.logger.Infof("try to acquire lock")
			if ha.dcs.AttempAcquireLock() == nil {
				ha.dbManager.Premote()
			}
		}

		// case cluster.SwitchOver != nil && cluster.SwitchOver.Leader == ha.dbManager.GetCurrentMemberName():
		// 	logger.Infof("Cluster has no leader, attemp to take the leader")
		// 	ha.dbManager.Demote()

		// case cluster.SwitchOver != nil && cluster.SwitchOver.Candidate == ha.dbManager.GetCurrentMemberName():
		// 	logger.Infof("Cluster has no leader, attemp to take the leader")
		// 	ha.dbManager.Premote()
	}
}

func (ha *Ha) Start() {
	ha.logger.Info("HA starting")
	cluster, err := ha.dcs.GetCluster()
	if errors.IsNotFound(err) {
		ha.logger.Infof("Cluster %s is not found, so HA exists.", ha.dcs.GetClusterName())
		return
	}

	ha.logger.Debugf("cluster: %v", cluster)
	isInitialized, _ := ha.dbManager.IsClusterInitialized()
	for !isInitialized {
		ha.logger.Infof("Waiting for the database cluster to be initialized.")
		// TODO: implement dbmanager initialize to replace pod's entrypoint scripts
		// if I am the node of index 0, then do initialization
		// ha.dbManager.Initialize()
		time.Sleep(1 * time.Second)
		isInitialized, _ = ha.dbManager.IsClusterInitialized()
	}
	ha.logger.Infof("The database cluster is initialized.")

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
		return
	}
}

func (ha *Ha) ShutdownWithWait() {
}
