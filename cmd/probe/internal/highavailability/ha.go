/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package highavailability

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/dapr/kit/logger"
	probing "github.com/prometheus-community/pro-bing"
	"github.com/spf13/viper"

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
	characterType := viper.GetString("KB_SERVICE_CHARACTER_TYPE")
	if characterType == "" {
		logger.Errorf("KB_SERVICE_CHARACTER_TYPE not set")
		return nil
	}

	manager := component.GetManager(characterType)
	if manager == nil {
		logger.Errorf("No DB Manager for character type %s", characterType)
		return nil
	}

	ha := &Ha{
		ctx:       context.Background(),
		dcs:       dcs,
		logger:    logger,
		dbManager: manager,
	}
	return ha
}

func (ha *Ha) RunCycle() {
	cluster, err := ha.dcs.GetCluster()
	if err != nil {
		ha.logger.Infof("Get Cluster err: %v.", err)
		return
	}

	if !ha.dbManager.IsRunning() {
		ha.logger.Infof("DB Service is not running,  wait for sqlctl to start it")
		if ha.dcs.HasLock() {
			_ = ha.dcs.ReleaseLock()
		}
		_ = ha.dbManager.Start(cluster)
		return
	}

	DBState := ha.dbManager.GetDBState(ha.ctx, cluster)
	// TODO: Is Reasonable?
	if DBState == nil {
		return
	}
	// store leader's db state in dcs
	if cluster.Leader != nil && cluster.Leader.Name == ha.dbManager.GetCurrentMemberName() {
		cluster.Leader.DBState = DBState
	}

	switch {
	// IsClusterHealthy is just for consensus cluster healthy check.
	// For Replication cluster IsClusterHealthy will always return true,
	// and its cluster's healthy is equal to leader member's healthy.
	case !ha.dbManager.IsClusterHealthy(ha.ctx, cluster):
		ha.logger.Errorf("The cluster is not healthy, wait...")

	case !ha.dbManager.IsCurrentMemberInCluster(ha.ctx, cluster) && int(cluster.Replicas) > len(ha.dbManager.GetMemberAddrs(cluster)):
		ha.logger.Infof("Current member is not in cluster, add it to cluster")
		_ = ha.dbManager.AddCurrentMemberToCluster(cluster)

	case !ha.dbManager.IsCurrentMemberHealthy(ha.ctx, cluster):
		ha.logger.Infof("DB Service is not healthy,  do some recover")
		if ha.dcs.HasLock() {
			_ = ha.dcs.ReleaseLock()
		}
	//	dbManager.Recover()

	case !cluster.IsLocked():
		ha.logger.Infof("Cluster has no leader, attempt to take the leader")
		if ha.IsHealthiestMember(ha.ctx, cluster) {
			cluster.Leader.DBState = DBState
			if ha.dcs.AttempAcquireLock() == nil {
				if ha.dbManager.Promote(ha.ctx) != nil {
					ha.logger.Infof("Take the leader failed: %v", err)
					_ = ha.dcs.ReleaseLock()
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
				if ha.HasOtherHealthyMember(cluster) {
					_ = ha.dbManager.Demote(ha.ctx)
					_ = ha.dcs.ReleaseLock()
					break
				}

			} else if cluster.Switchover.Candidate == "" || cluster.Switchover.Candidate == ha.dbManager.GetCurrentMemberName() {
				if !ha.dbManager.IsPromoted(ha.ctx) {
					// wait and retry
					break
				}
				_ = ha.dcs.DeleteSwitchover()
			}
		}

		if ha.dbManager.HasOtherHealthyLeader(ha.ctx, cluster) != nil {
			// this case is applicable only to consensus cluster, where the db's internal
			// role services as the source of truth.
			// for replicationset cluster,  HasOtherHealthyLeader will always be false.
			ha.logger.Infof("Release leader")
			_ = ha.dcs.ReleaseLock()
			break
		}
		_ = ha.dbManager.Promote(ha.ctx)
		ha.logger.Infof("Refresh leader ttl")
		_ = ha.dcs.UpdateLock()

		if int(cluster.Replicas) < len(ha.dbManager.GetMemberAddrs(cluster)) {
			ha.DecreaseClusterReplicas(cluster)
		}

	case !ha.dcs.HasLock():
		if cluster.Switchover != nil {
			break
		}
		// TODO: In the event that the database service and SQL channel both go down concurrently, eg. Pod deleted,
		// there is no healthy leader node and the lock remains unreleased, attempt to acquire the leader lock.

		// leaderMember := cluster.GetLeaderMember()
		// lockOwnerIsLeader, _ := ha.dbManager.IsLeaderMember(ha.ctx, cluster, leaderMember)
		// currentMemberIsLeader, _ := ha.dbManager.IsLeader(context.TODO(), cluster)
		// if lockOwnerIsLeader && currentMemberIsLeader {
		// ha.logger.Infof("Lock owner is real Leader, demote myself and follow the real leader")
		_ = ha.dbManager.Demote(ha.ctx)
		_ = ha.dbManager.Follow(ha.ctx, cluster)
	}
}

func (ha *Ha) Start() {
	ha.logger.Info("HA starting")
	cluster, err := ha.dcs.GetCluster()
	if cluster == nil {
		ha.logger.Errorf("Get Cluster %s error: %v, so HA exists.", ha.dcs.GetClusterName(), err)
		return
	}

	isPodReady, err := ha.IsPodReady()
	for err != nil || !isPodReady {
		ha.logger.Infof("Waiting for dns resolution to be ready")
		time.Sleep(3 * time.Second)
		isPodReady, err = ha.IsPodReady()
	}
	ha.logger.Infof("dns resolution is ready")

	ha.logger.Debugf("cluster: %v", cluster)
	isInitialized, err := ha.dbManager.IsClusterInitialized(context.TODO(), cluster)
	for err != nil || !isInitialized {
		ha.logger.Infof("Waiting for the database cluster to be initialized.")
		// TODO: implement dbmanager initialize to replace pod's entrypoint scripts
		// if I am the node of index 0, then do initialization
		if err == nil && !isInitialized && ha.dbManager.IsFirstMember() {
			ha.logger.Infof("Initialize cluster.")
			err := ha.dbManager.InitializeCluster(ha.ctx, cluster)
			if err != nil {
				ha.logger.Warnf("Cluster initialize failed: %v", err)
			}
		}
		time.Sleep(5 * time.Second)
		isInitialized, err = ha.dbManager.IsClusterInitialized(context.TODO(), cluster)
	}
	ha.logger.Infof("The database cluster is initialized.")

	isRootCreated, err := ha.dbManager.IsRootCreated(ha.ctx)
	for err != nil || !isRootCreated {
		if err == nil && !isRootCreated && ha.dbManager.IsFirstMember() {
			ha.logger.Infof("Create Root.")
			err := ha.dbManager.CreateRoot(ha.ctx)
			if err != nil {
				ha.logger.Warnf("Cluster initialize failed: %v", err)
			}
		}
		time.Sleep(5 * time.Second)
		isRootCreated, err = ha.dbManager.IsRootCreated(ha.ctx)
	}

	isExist, _ := ha.dcs.IsLockExist()
	for !isExist {
		if ok, _ := ha.dbManager.IsLeader(context.Background(), cluster); ok {
			_ = ha.dcs.Initialize()
			break
		}
		ha.logger.Infof("Waiting for the database Leader to be ready.")
		time.Sleep(5 * time.Second)
		isExist, _ = ha.dcs.IsLockExist()
	}

	for {
		ha.RunCycle()
		time.Sleep(1 * time.Second)
	}
}

func (ha *Ha) DecreaseClusterReplicas(cluster *dcs.Cluster) {
	hosts := ha.dbManager.GetMemberAddrs(cluster)
	sort.Strings(hosts)
	deleteHost := hosts[len(hosts)-1]
	ha.logger.Infof("Delete member: %s", deleteHost)
	// The pods in the cluster are managed by a StatefulSet. If the replica count is decreased,
	// then the last pod will be removed first.
	//
	if strings.HasPrefix(deleteHost, ha.dbManager.GetCurrentMemberName()) {
		ha.logger.Infof("The last pod %s is the primary member and cannot be deleted. waiting "+
			"for The controller to perform a switchover to a new primary member before this pod can be removed. ", deleteHost)
		_ = ha.dbManager.Demote(ha.ctx)
		_ = ha.dcs.ReleaseLock()
		return
	}
	_ = ha.dbManager.DeleteMemberFromCluster(cluster, deleteHost)
}

func (ha *Ha) IsHealthiestMember(ctx context.Context, cluster *dcs.Cluster) bool {
	currentMemberName := ha.dbManager.GetCurrentMemberName()
	currentMember := cluster.GetMemberWithName(currentMemberName)
	if cluster.Switchover != nil {
		switchover := cluster.Switchover
		leader := switchover.Leader
		candidate := switchover.Candidate

		if leader == currentMemberName {
			ha.logger.Infof("manual switchover to other member")
			return false
		}

		if candidate == currentMemberName {
			ha.logger.Infof("manual switchover to current member: %s", candidate)
			return true
		}

		if candidate != "" {
			ha.logger.Infof("manual switchover to new leader: %s", candidate)
			return false
		}
	}

	if member := ha.dbManager.HasOtherHealthyLeader(ctx, cluster); member != nil {
		ha.logger.Infof("there is a healthy leader exists: %s", member.Name)
		return false
	}

	isCurrentLagging, currentLag := ha.dbManager.IsMemberLagging(ctx, cluster, currentMember)
	if !isCurrentLagging {
		return true
	}

	for _, member := range cluster.Members {
		if member.Name != currentMemberName {
			isLagging, lag := ha.dbManager.IsMemberLagging(ctx, cluster, &member)
			// There are other members with smaller lag
			if !isLagging || lag < currentLag {
				return false
			}
		}
	}

	return true
}

func (ha *Ha) HasOtherHealthyMember(cluster *dcs.Cluster) bool {
	var otherMembers = make([]*dcs.Member, 0, 1)
	if cluster.Switchover != nil && cluster.Switchover.Candidate != "" {
		candidate := cluster.Switchover.Candidate
		if candidate != ha.dbManager.GetCurrentMemberName() {
			otherMembers = append(otherMembers, cluster.GetMemberWithName(candidate))
		}
	} else {
		for _, member := range cluster.Members {
			if member.Name == ha.dbManager.GetCurrentMemberName() {
				continue
			}
			otherMembers = append(otherMembers, &member)
		}
	}

	for _, other := range otherMembers {
		if ha.dbManager.IsMemberHealthy(ha.ctx, cluster, other) {
			if isLagging, _ := ha.dbManager.IsMemberLagging(ha.ctx, cluster, other); !isLagging {
				return true
			}
		}
	}

	return false
}

// IsPodReady checks if pod is ready, it can successfully resolve domain
func (ha *Ha) IsPodReady() (bool, error) {
	domain := viper.GetString("KB_POD_FQDN")

	pinger, err := probing.NewPinger(domain)
	if err != nil {
		ha.logger.Errorf("new pinger failed, err:%v", err)
		return false, err
	}

	pinger.Count = 3
	err = pinger.Run()
	if err != nil {
		ha.logger.Errorf("ping domain:%s failed, err:%v", domain, err)
		return false, err
	}

	return true, nil
}

func (ha *Ha) ShutdownWithWait() {
	ha.dbManager.ShutDownWithWait()
}
