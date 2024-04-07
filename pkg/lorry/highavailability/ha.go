/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	dcs3 "github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type Ha struct {
	ctx               context.Context
	dbManager         engines.DBManager
	dcs               dcs3.DCS
	logger            logr.Logger
	deleteLock        sync.Mutex
	disableDNSChecker bool
}

var ha *Ha

func NewHa(disableDNSChecker bool) *Ha {
	logger := ctrl.Log.WithName("HA")

	dcs := dcs3.GetStore()
	manager, err := register.GetDBManager(nil)
	if err != nil {
		logger.Error(err, "No DB Manager")
		return nil
	}

	ha = &Ha{
		ctx:               context.Background(),
		dcs:               dcs,
		logger:            logger,
		dbManager:         manager,
		disableDNSChecker: disableDNSChecker,
	}
	return ha
}

func GetHa() *Ha {
	return ha
}

func (ha *Ha) RunCycle() {
	cluster, err := ha.dcs.GetCluster()
	if err != nil {
		ha.logger.Error(err, "Get Cluster failed")
		return
	}

	if !cluster.HaConfig.IsEnable() {
		return
	}

	currentMember := cluster.GetMemberWithName(ha.dbManager.GetCurrentMemberName())

	if cluster.HaConfig.IsDeleting(currentMember) {
		ha.logger.Info("Current Member is deleted!")
		_ = ha.DeleteCurrentMember(ha.ctx, cluster)
		return
	}

	if !ha.dbManager.IsRunning() {
		ha.logger.Info("DB Service is not running,  wait for hypervisor to start it")
		if ha.dcs.HasLease() {
			_ = ha.dcs.ReleaseLease()
		}
		_ = ha.dbManager.Start(ha.ctx, cluster)
		return
	}

	DBState := ha.dbManager.GetDBState(ha.ctx, cluster)
	// store leader's db state in dcs
	if cluster.Leader != nil && cluster.Leader.Name == ha.dbManager.GetCurrentMemberName() {
		cluster.Leader.DBState = DBState
	}

	switch {
	// IsClusterHealthy is just for consensus cluster healthy check.
	// For Replication cluster IsClusterHealthy will always return true,
	// and its cluster's healthy is equal to leader member's healthy.
	case !ha.dbManager.IsClusterHealthy(ha.ctx, cluster):
		ha.logger.Error(nil, "The cluster is not healthy, wait...")

	case !ha.dbManager.IsCurrentMemberInCluster(ha.ctx, cluster) && int(cluster.Replicas) > len(ha.dbManager.GetMemberAddrs(ha.ctx, cluster)):
		ha.logger.Info("Current member is not in cluster, add it to cluster")
		_ = ha.dbManager.JoinCurrentMemberToCluster(ha.ctx, cluster)

	case !ha.dbManager.IsCurrentMemberHealthy(ha.ctx, cluster):
		ha.logger.Info("DB Service is not healthy,  do some recover")
		if ha.dcs.HasLease() {
			_ = ha.dcs.ReleaseLease()
		}
		err = ha.dbManager.Recover(ha.ctx, cluster)
		if err != nil {
			ha.logger.Info("recover member failed", "error", err.Error())
		}

	case !cluster.IsLocked():
		ha.logger.Info("Cluster has no leader, attempt to take the leader")
		if !ha.IsHealthiestMember(ha.ctx, cluster) {
			break
		}

		cluster.Leader.DBState = DBState
		if ha.dcs.AttemptAcquireLease() != nil {
			break
		}

		err := ha.dbManager.Promote(ha.ctx, cluster)
		if err != nil {
			ha.logger.Error(err, "Take the leader failed")
			_ = ha.dcs.ReleaseLease()
			break
		}
		cluster.Leader.Name = ha.dbManager.GetCurrentMemberName()

		ha.logger.Info("Take the leader success!")
		fallthrough

	case ha.dcs.HasLease():
		ha.logger.Info("This member is Cluster's leader")
		if cluster.Switchover != nil {
			if cluster.Switchover.Leader == ha.dbManager.GetCurrentMemberName() ||
				(cluster.Switchover.Candidate != "" && cluster.Switchover.Candidate != ha.dbManager.GetCurrentMemberName()) {
				if ha.HasOtherHealthyMember(cluster) {
					_ = ha.dbManager.Demote(ha.ctx)
					_ = ha.dcs.ReleaseLease()
					break
				}
				ha.logger.Info("The cluster has no other helathy members!")

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
			// for replicationSet cluster,  HasOtherHealthyLeader will always be false.
			ha.logger.Info("Release leader")
			_ = ha.dcs.ReleaseLease()
			break
		}
		err = ha.dbManager.Promote(ha.ctx, cluster)
		if err != nil {
			ha.logger.Error(err, "promote failed")
			break
		}

		ha.logger.Info("Refresh leader ttl")
		_ = ha.dcs.UpdateLease()

		if int(cluster.Replicas) < len(ha.dbManager.GetMemberAddrs(ha.ctx, cluster)) && cluster.Replicas != 0 {
			ha.DecreaseClusterReplicas(cluster)
		}

	case !ha.dcs.HasLease():
		if cluster.Switchover != nil {
			break
		}
		// TODO: In the event that the database service and SQL channel both go down concurrently, eg. Pod deleted,
		// there is no healthy leader node and the lock remains unreleased, attempt to acquire the leader lock.

		// leaderMember := cluster.GetLeaderMember()
		// lockOwnerIsLeader, _ := ha.dbManager.IsLeaderMember(ha.ctx, cluster, leaderMember)
		// currentMemberIsLeader, _ := ha.dbManager.IsLeader(context.TODO(), cluster)
		// if lockOwnerIsLeader && currentMemberIsLeader {
		// ha.logger.Info("Lock owner is real Leader, demote myself and follow the real leader")
		err = ha.dbManager.Demote(ha.ctx)
		if err != nil {
			ha.logger.Info("promote failed", "error", err)
		}

		err = ha.dbManager.Follow(ha.ctx, cluster)
		if err != nil {
			ha.logger.Info("follow failed", "error", err)
		}
	}
}

func (ha *Ha) Start() {
	ha.logger.Info("HA starting")
	cluster, err := ha.dcs.GetCluster()
	for cluster == nil {
		ha.logger.Error(err, "Get Cluster failed.", "cluster-name", ha.dcs.GetClusterName())
		time.Sleep(10 * time.Second)
		cluster, err = ha.dcs.GetCluster()
	}

	if !ha.disableDNSChecker {
		util.WaitForPodReady(false)
	}

	ha.logger.Info(fmt.Sprintf("cluster: %v", cluster))
	isInitialized, err := ha.dbManager.IsClusterInitialized(context.TODO(), cluster)
	for err != nil || !isInitialized {
		ha.logger.Info("Waiting for the database cluster to be initialized.")
		// TODO: implement DBManager initialize to replace pod's entrypoint scripts
		// if I am the node of index 0, then do initialization
		if err == nil && !isInitialized && ha.dbManager.IsFirstMember() {
			ha.logger.Info("Initialize cluster.")
			err := ha.dbManager.InitializeCluster(ha.ctx, cluster)
			if err != nil {
				ha.logger.Error(err, "Cluster initialize failed")
			}
		} else if err != nil {
			ha.logger.Info("Initialize the database cluster Failed.", "error", err.Error())
		}
		time.Sleep(5 * time.Second)
		isInitialized, err = ha.dbManager.IsClusterInitialized(context.TODO(), cluster)
	}
	ha.logger.Info("The database cluster is initialized.")

	isRootCreated, err := ha.dbManager.IsRootCreated(ha.ctx)
	for err != nil || !isRootCreated {
		if err == nil && !isRootCreated && ha.dbManager.IsFirstMember() {
			ha.logger.Info("Create Root.")
			err := ha.dbManager.CreateRoot(ha.ctx)
			if err != nil {
				ha.logger.Error(err, "Cluster initialize failed")
			}
		}
		time.Sleep(5 * time.Second)
		isRootCreated, err = ha.dbManager.IsRootCreated(ha.ctx)
	}

	isExist, _ := ha.dcs.IsLeaseExist()
	for !isExist {
		if ok, _ := ha.dbManager.IsLeader(context.Background(), cluster); ok {
			err := ha.dcs.Initialize()
			if err != nil {
				ha.logger.Error(err, "DCS initialize failed")
				time.Sleep(5 * time.Second)
				continue
			}
			break
		}

		ha.logger.Info("Waiting for the database Leader to be ready.")
		time.Sleep(5 * time.Second)
		isExist, _ = ha.dcs.IsLeaseExist()
	}

	for {
		startAt := time.Now()
		ha.RunCycle()
		duration := time.Since(startAt)
		if duration < 10*time.Second {
			time.Sleep(10*time.Second - duration)
		}
	}
}

func (ha *Ha) DecreaseClusterReplicas(cluster *dcs3.Cluster) {
	hosts := ha.dbManager.GetMemberAddrs(ha.ctx, cluster)
	sort.Strings(hosts)
	deleteHost := hosts[len(hosts)-1]
	ha.logger.Info("Delete member", "name", deleteHost)
	// The pods in the cluster are managed by a StatefulSet. If the replica count is decreased,
	// then the last pod will be removed first.
	//
	if strings.HasPrefix(deleteHost, ha.dbManager.GetCurrentMemberName()) {
		ha.logger.Info(fmt.Sprintf("The last pod %s is the primary member and cannot be deleted. waiting "+
			"for The controller to perform a switchover to a new primary member before this pod can be removed. ", deleteHost))
		_ = ha.dbManager.Demote(ha.ctx)
		_ = ha.dcs.ReleaseLease()
		return
	}
	memberName := strings.Split(deleteHost, ".")[0]
	member := cluster.GetMemberWithName(memberName)
	if member != nil {
		ha.logger.Info(fmt.Sprintf("member %s exists, do not delete", memberName))
		return
	}
	_ = ha.dbManager.LeaveMemberFromCluster(ha.ctx, cluster, memberName)
}

func (ha *Ha) IsHealthiestMember(ctx context.Context, cluster *dcs3.Cluster) bool {
	currentMemberName := ha.dbManager.GetCurrentMemberName()
	currentMember := cluster.GetMemberWithName(currentMemberName)
	if cluster.Switchover != nil {
		switchover := cluster.Switchover
		leader := switchover.Leader
		candidate := switchover.Candidate

		if leader == currentMemberName {
			ha.logger.Info("manual switchover to other member")
			return false
		}

		if candidate == currentMemberName {
			ha.logger.Info("manual switchover to current member", "member", candidate)
			isCurrentLagging, _ := ha.dbManager.IsMemberLagging(ctx, cluster, currentMember)
			return !isCurrentLagging
		}

		if candidate != "" {
			ha.logger.Info("manual switchover to new leader", "new leader", candidate)
			return false
		}
		return ha.isMinimumLag(ctx, cluster, currentMember)
	}

	if member := ha.dbManager.HasOtherHealthyLeader(ctx, cluster); member != nil {
		ha.logger.Info("there is a healthy leader exists", "leader", member.Name)
		return false
	}

	return ha.isMinimumLag(ctx, cluster, currentMember)
}

func (ha *Ha) HasOtherHealthyMember(cluster *dcs3.Cluster) bool {
	var otherMembers = make([]*dcs3.Member, 0, 1)
	if cluster.Switchover != nil && cluster.Switchover.Candidate != "" {
		candidate := cluster.Switchover.Candidate
		if candidate != ha.dbManager.GetCurrentMemberName() {
			otherMembers = append(otherMembers, cluster.GetMemberWithName(candidate))
		}
	} else {
		for i, member := range cluster.Members {
			if member.Name == ha.dbManager.GetCurrentMemberName() {
				continue
			}
			otherMembers = append(otherMembers, &cluster.Members[i])
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

func (ha *Ha) DeleteCurrentMember(ctx context.Context, cluster *dcs3.Cluster) error {
	currentMember := cluster.GetMemberWithName(ha.dbManager.GetCurrentMemberName())
	if cluster.HaConfig.IsDeleted(currentMember) {
		return nil
	}

	ha.deleteLock.Lock()
	defer ha.deleteLock.Unlock()

	// if current member is leader, take a switchover first
	if ha.dcs.HasLease() {
		for cluster.Switchover != nil {
			ha.logger.Info("cluster is doing switchover, wait for it to finish")
			return nil
		}

		leaderMember := cluster.GetLeaderMember()
		if len(ha.dbManager.HasOtherHealthyMembers(ctx, cluster, leaderMember.Name)) == 0 {
			message := "cluster has no other healthy members"
			ha.logger.Info(message)
			return errors.New(message)
		}

		err := ha.dcs.CreateSwitchover(leaderMember.Name, "")
		if err != nil {
			ha.logger.Error(err, "switchover failed")
			return err
		}

		ha.logger.Info("cluster is doing switchover, wait for it to finish")
		return nil
	}

	// redistribute the data of the current member among other members if needed
	err := ha.dbManager.MoveData(ctx, cluster)
	if err != nil {
		ha.logger.Error(err, "Move data failed")
		return err
	}

	// remove current member from db cluster
	err = ha.dbManager.LeaveMemberFromCluster(ctx, cluster, ha.dbManager.GetCurrentMemberName())
	if err != nil {
		ha.logger.Error(err, "Delete member form cluster failed")
		return err
	}
	cluster.HaConfig.FinishDeleted(currentMember)
	return ha.dcs.UpdateHaConfig()
}

func (ha *Ha) isMinimumLag(ctx context.Context, cluster *dcs3.Cluster, member *dcs3.Member) bool {
	isCurrentLagging, currentLag := ha.dbManager.IsMemberLagging(ctx, cluster, member)
	if isCurrentLagging {
		return false
	}

	for _, m := range cluster.Members {
		if m.Name != member.Name {
			isLagging, lag := ha.dbManager.IsMemberLagging(ctx, cluster, &m)
			// There are other members with smaller lag
			if !isLagging && lag < currentLag {
				return false
			}
		}
	}

	return true
}

func (ha *Ha) ShutdownWithWait() {
	ha.dbManager.ShutDownWithWait()
}
