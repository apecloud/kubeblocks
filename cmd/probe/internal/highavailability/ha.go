package highavailability

import (
	"context"
	"sort"
	"strings"
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

	case !ha.dbManager.IsCurrentMemberInCluster(cluster) && int(cluster.Replicas) == len(ha.dbManager.GetMemberAddrs()):
		ha.logger.Infof("Current member is not in cluster, add it to cluster")
		ha.dbManager.AddCurrentMemberToCluster(cluster)

	case !ha.dbManager.IsCurrentMemberHealthy():
		ha.logger.Infof("DB Service is not healthy,  do some recover")
		if ha.dcs.HasLock() {
			ha.dcs.ReleaseLock()
		}
	//	dbManager.Recover()

	case !cluster.IsLocked():
		ha.logger.Infof("Cluster has no leader, attemp to take the leader")
		if ha.IsHealthiestMember(cluster) {
			if ha.dcs.AttempAcquireLock() == nil {
				err := ha.dbManager.Premote()
				if err != nil {
					ha.logger.Infof("Take the leader failed: %v", err)
					ha.dcs.ReleaseLock()
				} else {
					ha.logger.Infof("Take the leader success!")
				}
			}
		}

	case ha.dcs.HasLock():
		ha.logger.Infof("This member is Cluster's leader")
		if cluster.Switchover != nil {
			if cluster.Switchover.Leader == ha.dbManager.GetCurrentMemberName() ||
				(cluster.Switchover.Candidate != "" && cluster.Switchover.Candidate != ha.dbManager.GetCurrentMemberName()) {
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
			if int(cluster.Replicas) < len(ha.dbManager.GetMemberAddrs()) {
				ha.DecreaseClusterReplicas(cluster)
			}

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
		// TODO: In the event that the database service and SQL channel both go down concurrently, eg. Pod deleted,
		// there is no healthy leader node and the lock remains unreleased, attempt to acquire the leader lock.

		if ok, _ := ha.dbManager.IsLeader(context.TODO()); ok {
			ha.logger.Infof("I am the real leader, wait for lock released")
			// if ha.dcs.AttempAcquireLock() == nil {
			// 	ha.dbManager.Premote()
			// }
		} else {
			// make sure sync source is leader when role changed
			ha.dbManager.Demote()
		}
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
	}
}

func (ha *Ha) DecreaseClusterReplicas(cluster *dcs.Cluster) {
	hosts := ha.dbManager.GetMemberAddrs()
	sort.Strings(hosts)
	deleteHost := hosts[len(hosts)-1]
	ha.logger.Infof("Delete member: %s", deleteHost)
	// The pods in the cluster are managed by a StatefulSet. If the replica count is decreased,
	// then the last pod will be removed first.
	//
	if strings.HasPrefix(deleteHost, ha.dbManager.GetCurrentMemberName()) {
		ha.logger.Infof("The last pod %s is the primary member and cannot be deleted. waiting "+
			"for The controller to perform a switchover to a new primary member before this pod can be removed. ", deleteHost)
		ha.dbManager.Demote()
		ha.dcs.ReleaseLock()
		return
	}
	ha.dbManager.DeleteMemberFromCluster(cluster, deleteHost)
}

func (ha *Ha) IsHealthiestMember(cluster *dcs.Cluster) bool {
	if cluster.Switchover != nil {
		switchover := cluster.Switchover
		candidate := switchover.Candidate
		if candidate == ha.dbManager.GetCurrentMemberName() {
			return true
		}
		if candidate != "" && ha.dbManager.IsMemberHealthy(candidate) {
			ha.logger.Infof("manual switchover to new leader: %s", candidate)
			return false
		}

		if switchover.Leader == ha.dbManager.GetCurrentMemberName() && len(ha.dbManager.HasOtherHealthyMembers(cluster)) > 0 {
			ha.logger.Infof("manual switchover to other member")
			return false
		}
	}

	if member := ha.dbManager.HasOtherHealthyLeader(cluster); member != nil {
		ha.logger.Infof("there is a healthy leader exists: %s", member.Name)
		return false
	}

	return true
}

func (ha *Ha) ShutdownWithWait() {
}
