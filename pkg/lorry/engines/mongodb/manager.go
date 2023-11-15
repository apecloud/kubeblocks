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

package mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

const (
	PrimaryPriority   = 2
	SecondaryPriority = 1

	ServiceType = "mongodb"
)

type Manager struct {
	engines.DBManagerBase
	Client   *mongo.Client
	Database *mongo.Database
}

var Mgr *Manager
var _ engines.DBManager = &Manager{}

func NewManager(properties engines.Properties) (engines.DBManager, error) {
	ctx := context.Background()
	logger := ctrl.Log.WithName("MongoDB")
	config, err := NewConfig(properties)
	if err != nil {
		return nil, err
	}

	opts := options.Client().
		SetHosts(config.hosts).
		SetReplicaSet(config.replSetName).
		SetAuth(options.Credential{
			Password: config.password,
			Username: config.username,
		}).
		SetWriteConcern(writeconcern.New(writeconcern.WMajority(), writeconcern.J(true))).
		SetReadPreference(readpref.Primary()).
		SetDirect(config.direct)

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "connect to mongodb")
	}

	defer func() {
		if err != nil {
			derr := client.Disconnect(ctx)
			if derr != nil {
				logger.Error(err, "failed to disconnect")
			}
		}
	}()

	managerBase, err := engines.NewDBManagerBase(logger)
	if err != nil {
		return nil, err
	}

	Mgr = &Manager{
		DBManagerBase: *managerBase,
		Client:        client,
		Database:      client.Database(config.databaseName),
	}

	return Mgr, nil
}

func (mgr *Manager) InitializeCluster(ctx context.Context, cluster *dcs.Cluster) error {
	return mgr.InitiateReplSet(ctx, cluster)
}

// InitiateReplSet is a method to create MongoDB cluster
func (mgr *Manager) InitiateReplSet(ctx context.Context, cluster *dcs.Cluster) error {
	configMembers := make([]ConfigMember, len(cluster.Members))

	for i, member := range cluster.Members {
		configMembers[i].ID = i
		configMembers[i].Host = cluster.GetMemberAddrWithPort(member)
		if strings.HasPrefix(member.Name, mgr.CurrentMemberName) {
			configMembers[i].Priority = PrimaryPriority
		} else {
			configMembers[i].Priority = SecondaryPriority
		}
	}

	config := RSConfig{
		ID:      mgr.ClusterCompName,
		Members: configMembers,
	}
	client, err := NewLocalUnauthClient(ctx)
	if err != nil {
		mgr.Logger.Error(err, "Get local unauth client failed")
		return err
	}
	defer client.Disconnect(context.TODO()) //nolint:errcheck

	configJSON, _ := json.Marshal(config)
	mgr.Logger.Info(fmt.Sprintf("Initial Replset Config: %s", string(configJSON)))
	response := client.Database("admin").RunCommand(ctx, bson.M{"replSetInitiate": config})
	if response.Err() != nil {
		return response.Err()
	}
	return nil
}

// IsClusterInitialized is a method to check if cluster is initialized or not
func (mgr *Manager) IsClusterInitialized(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	client, err := mgr.GetReplSetClient(ctx, cluster)
	if err != nil {
		mgr.Logger.Info("Get leader client failed", "error", err)
		return false, err
	}
	defer client.Disconnect(ctx) //nolint:errcheck

	ctx1, cancel := context.WithTimeout(ctx, 1000*time.Millisecond)
	defer cancel()
	rsStatus, err := GetReplSetStatus(ctx1, client)
	if rsStatus != nil {
		return rsStatus.Set != "", nil
	}
	mgr.Logger.Info("Get replSet status failed", "error", err)

	if !mgr.IsFirstMember() {
		return false, nil
	}

	client, err = NewLocalUnauthClient(ctx)
	if err != nil {
		mgr.Logger.Info("Get local unauth client failed", "error", err)
		return false, err
	}
	defer client.Disconnect(ctx) //nolint:errcheck

	rsStatus, err = GetReplSetStatus(ctx, client)
	if rsStatus != nil {
		return rsStatus.Set != "", nil
	}

	err = errors.Cause(err)
	if cmdErr, ok := err.(mongo.CommandError); ok && cmdErr.Name == "NotYetInitialized" {
		return false, nil
	}
	mgr.Logger.Info("Get replSet status with local unauth client failed", "error", err)

	rsStatus, err = mgr.GetReplSetStatus(ctx)
	if rsStatus != nil {
		return rsStatus.Set != "", nil
	}
	if err != nil {
		mgr.Logger.Info("Get replSet status with local auth client failed", "error", err)
		return false, err
	}

	mgr.Logger.Info("Get replSet status failed", "error", err)
	return false, err
}

func (mgr *Manager) IsRootCreated(ctx context.Context) (bool, error) {
	if !mgr.IsFirstMember() {
		return true, nil
	}

	client, err := NewLocalUnauthClient(ctx)
	if err != nil {
		mgr.Logger.Info("Get local unauth client failed", "error", err)
		return false, err
	}
	defer client.Disconnect(ctx) //nolint:errcheck

	_, err = GetReplSetStatus(ctx, client)
	if err == nil {
		return false, nil
	}
	err = errors.Cause(err)
	if cmdErr, ok := err.(mongo.CommandError); ok && cmdErr.Name == "Unauthorized" {
		return true, nil
	}

	mgr.Logger.Info("Get replSet status with local unauth client failed", "error", err)

	_, err = mgr.GetReplSetStatus(ctx)
	if err == nil {
		return true, nil
	}

	mgr.Logger.Info("Get replSet status with local auth client failed", "error", err)
	return false, err

}

func (mgr *Manager) CreateRoot(ctx context.Context) error {
	if !mgr.IsFirstMember() {
		return nil
	}

	client, err := NewLocalUnauthClient(ctx)
	if err != nil {
		mgr.Logger.Info("Get local unauth client failed", "error", err)
		return err
	}
	defer client.Disconnect(ctx) //nolint:errcheck

	role := map[string]interface{}{
		"role": "root",
		"db":   "admin",
	}

	mgr.Logger.Info(fmt.Sprintf("Create user: %s, passwd: %s, roles: %v", config.username, config.password, role))
	err = CreateUser(ctx, client, config.username, config.password, role)
	if err != nil {
		mgr.Logger.Info("Create Root failed", "error", err)
		return err
	}

	return nil
}

func (mgr *Manager) IsRunning() bool {
	// ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	// defer cancel()

	// err := mgr.Client.Ping(ctx, readpref.Nearest())
	// if err != nil {
	// 	mgr.Logger.Infof("DB is not ready: %v", err)
	// 	return false
	// }
	return true
}

func (mgr *Manager) IsDBStartupReady() bool {
	if mgr.DBStartupReady {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := mgr.Client.Ping(ctx, readpref.Primary())
	if err != nil {
		mgr.Logger.Info("DB is not ready", "error", err)
		return false
	}
	mgr.DBStartupReady = true
	mgr.Logger.Info("DB startup ready")
	return true
}

func (mgr *Manager) GetMemberState(ctx context.Context) (string, error) {
	status, err := mgr.GetReplSetStatus(ctx)
	if err != nil {
		mgr.Logger.Error(err, "rs.status() error")
		return "", err
	}

	self := status.GetSelf()
	if self == nil {
		return "", nil
	}
	return strings.ToLower(self.StateStr), nil
}

func (mgr *Manager) GetReplSetStatus(ctx context.Context) (*ReplSetStatus, error) {
	return GetReplSetStatus(ctx, mgr.Client)
}

func (mgr *Manager) IsLeaderMember(ctx context.Context, cluster *dcs.Cluster, dcsMember *dcs.Member) (bool, error) {
	status, err := mgr.GetReplSetStatus(ctx)
	if err != nil {
		mgr.Logger.Error(err, "rs.status() error")
		return false, err
	}
	for _, member := range status.Members {
		if strings.HasPrefix(member.Name, dcsMember.Name) {
			if member.StateStr == "PRIMARY" {
				return true, nil
			}
			break
		}
	}
	return false, nil
}

func (mgr *Manager) IsLeader(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	cur := mgr.Client.Database("admin").RunCommand(ctx, bson.D{{Key: "isMaster", Value: 1}})
	if cur.Err() != nil {
		return false, errors.Wrap(cur.Err(), "run isMaster")
	}

	resp := IsMasterResp{}
	if err := cur.Decode(&resp); err != nil {
		return false, errors.Wrap(err, "decode isMaster response")
	}

	if resp.OK != 1 {
		return false, errors.Errorf("mongo says: %s", resp.Errmsg)
	}

	return resp.IsMaster, nil
}

func (mgr *Manager) GetReplSetConfig(ctx context.Context) (*RSConfig, error) {
	return GetReplSetConfig(ctx, mgr.Client)
}

func (mgr *Manager) GetMemberAddrs(ctx context.Context, cluster *dcs.Cluster) []string {
	client, err := mgr.GetReplSetClient(ctx, cluster)
	if err != nil {
		mgr.Logger.Error(err, "Get replSet client failed")
		return nil
	}
	defer client.Disconnect(ctx) //nolint:errcheck

	rsConfig, err := GetReplSetConfig(ctx, client)
	if rsConfig == nil {
		mgr.Logger.Error(err, "Get replSet config failed")
		return nil
	}

	return mgr.GetMemberAddrsFromRSConfig(rsConfig)
}

func (mgr *Manager) GetMemberAddrsFromRSConfig(rsConfig *RSConfig) []string {
	if rsConfig == nil {
		return []string{}
	}

	hosts := make([]string, len(rsConfig.Members))
	for i, member := range rsConfig.Members {
		hosts[i] = member.Host
	}
	return hosts
}

func (mgr *Manager) GetReplSetClient(ctx context.Context, cluster *dcs.Cluster) (*mongo.Client, error) {
	hosts := cluster.GetMemberAddrs()
	return NewReplSetClient(ctx, hosts)
}

func (mgr *Manager) GetLeaderClient(ctx context.Context, cluster *dcs.Cluster) (*mongo.Client, error) {
	if cluster.Leader == nil || cluster.Leader.Name == "" {
		return nil, fmt.Errorf("cluster has no leader")
	}

	leaderMember := cluster.GetMemberWithName(cluster.Leader.Name)
	host := cluster.GetMemberAddrWithPort(*leaderMember)
	return NewReplSetClient(context.TODO(), []string{host})
}

func (mgr *Manager) GetReplSetClientWithHosts(ctx context.Context, hosts []string) (*mongo.Client, error) {
	if len(hosts) == 0 {
		err := errors.New("Get replset client without hosts")
		mgr.Logger.Error(err, "Get replset client without hosts")
		return nil, err
	}

	opts := options.Client().
		SetHosts(hosts).
		SetReplicaSet(config.replSetName).
		SetAuth(options.Credential{
			Password: config.password,
			Username: config.username,
		}).
		SetWriteConcern(writeconcern.New(writeconcern.WMajority(), writeconcern.J(true))).
		SetReadPreference(readpref.Primary()).
		SetDirect(false)

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "connect to mongodb")
	}
	return client, err
}

func (mgr *Manager) IsCurrentMemberInCluster(ctx context.Context, cluster *dcs.Cluster) bool {
	client, err := mgr.GetReplSetClient(ctx, cluster)
	if err != nil {
		mgr.Logger.Error(err, "Get replSet client failed")
		return true
	}
	defer client.Disconnect(ctx) //nolint:errcheck

	rsConfig, err := GetReplSetConfig(ctx, client)
	if rsConfig == nil {
		mgr.Logger.Error(err, "Get replSet config failed")
		//
		return true
	}

	for _, member := range rsConfig.Members {
		if strings.HasPrefix(member.Host, mgr.GetCurrentMemberName()) {
			return true
		}
	}

	return false
}

func (mgr *Manager) IsCurrentMemberHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	return mgr.IsMemberHealthy(ctx, cluster, nil)
}

func (mgr *Manager) IsMemberHealthy(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) bool {
	var memberName string
	if member != nil {
		memberName = member.Name
	} else {
		memberName = mgr.CurrentMemberName
	}

	rsStatus, _ := mgr.GetReplSetStatus(ctx)
	if rsStatus == nil {
		return false
	}

	for _, member := range rsStatus.Members {
		if strings.HasPrefix(member.Name, memberName) && member.Health == 1 {
			return true
		}
	}
	return false
}

func (mgr *Manager) Recover(context.Context) error {
	return nil
}

func (mgr *Manager) JoinCurrentMemberToCluster(ctx context.Context, cluster *dcs.Cluster) error {
	client, err := mgr.GetReplSetClient(ctx, cluster)
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx) //nolint:errcheck

	currentMember := cluster.GetMemberWithName(mgr.GetCurrentMemberName())
	currentHost := cluster.GetMemberAddrWithPort(*currentMember)
	rsConfig, err := GetReplSetConfig(ctx, client)
	if rsConfig == nil {
		mgr.Logger.Error(err, "Get replSet config failed")
		return err
	}

	var lastID int
	var configMember ConfigMember
	for _, configMember = range rsConfig.Members {
		if configMember.ID > lastID {
			lastID = configMember.ID
		}
	}
	configMember.ID = lastID + 1
	configMember.Host = currentHost
	configMember.Priority = SecondaryPriority
	rsConfig.Members = append(rsConfig.Members, configMember)

	rsConfig.Version++
	return SetReplSetConfig(ctx, client, rsConfig)
}

func (mgr *Manager) LeaveMemberFromCluster(ctx context.Context, cluster *dcs.Cluster, memberName string) error {
	client, err := mgr.GetReplSetClient(ctx, cluster)
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx) //nolint:errcheck

	rsConfig, err := GetReplSetConfig(ctx, client)
	if rsConfig == nil {
		mgr.Logger.Error(err, "Get replSet config failed")
		return err
	}

	mgr.Logger.Info(fmt.Sprintf("Delete member: %s", memberName))
	configMembers := make([]ConfigMember, 0, len(rsConfig.Members)-1)
	for _, configMember := range rsConfig.Members {
		if strings.HasPrefix(configMember.Host, memberName) {
			configMembers = append(configMembers, configMember)
		}
	}

	rsConfig.Members = configMembers
	rsConfig.Version++
	return SetReplSetConfig(ctx, client, rsConfig)
}

func (mgr *Manager) IsClusterHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	client, err := mgr.GetReplSetClient(ctx, cluster)
	if err != nil {
		mgr.Logger.Error(err, "Get leader client failed")
		return false
	}
	defer client.Disconnect(ctx) //nolint:errcheck

	status, err := GetReplSetStatus(ctx, client)
	if err != nil {
		return false
	}
	mgr.Logger.Info(fmt.Sprintf("cluster status: %v", status))
	return status.OK != 0
}

func (mgr *Manager) IsPromoted(ctx context.Context) bool {
	isLeader, err := mgr.IsLeader(ctx, nil)
	if err != nil || !isLeader {
		mgr.Logger.Error(err, "Is leader check failed")
		return false
	}

	rsConfig, err := mgr.GetReplSetConfig(ctx)
	if rsConfig == nil {
		mgr.Logger.Error(err, "Get replSet config failed")
		return false
	}
	for i := range rsConfig.Members {
		if strings.HasPrefix(rsConfig.Members[i].Host, mgr.CurrentMemberName) {
			if rsConfig.Members[i].Priority == PrimaryPriority {
				return true
			}
		}
	}
	return false
}

func (mgr *Manager) Promote(ctx context.Context, cluster *dcs.Cluster) error {
	rsConfig, err := mgr.GetReplSetConfig(ctx)
	if rsConfig == nil {
		mgr.Logger.Error(err, "Get replSet config failed")
		return err
	}

	for i := range rsConfig.Members {
		if strings.HasPrefix(rsConfig.Members[i].Host, mgr.CurrentMemberName) {
			if rsConfig.Members[i].Priority == PrimaryPriority {
				mgr.Logger.Info("Current member already has the highest priority!")
				return nil
			}

			rsConfig.Members[i].Priority = PrimaryPriority
		} else if rsConfig.Members[i].Priority == PrimaryPriority {
			rsConfig.Members[i].Priority = SecondaryPriority
		}
	}

	rsConfig.Version++

	hosts := mgr.GetMemberAddrsFromRSConfig(rsConfig)
	client, err := NewReplSetClient(ctx, hosts)
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx) //nolint:errcheck
	mgr.Logger.Info("reconfig replset", "config", rsConfig)
	return SetReplSetConfig(ctx, client, rsConfig)
}

func (mgr *Manager) Demote(context.Context) error {
	// mongodb do premote and demote in one action, here do nothing.
	return nil
}

func (mgr *Manager) Follow(ctx context.Context, cluster *dcs.Cluster) error {
	return nil
}

func (mgr *Manager) GetHealthiestMember(cluster *dcs.Cluster, candidate string) *dcs.Member {
	rsStatus, _ := mgr.GetReplSetStatus(context.TODO())
	if rsStatus == nil {
		return nil
	}
	healthyMembers := make([]string, 0, len(rsStatus.Members))
	var leader string
	for _, member := range rsStatus.Members {
		if member.Health == 1 {
			memberName := strings.Split(member.Name, ".")[0]
			if memberName == candidate {
				return cluster.GetMemberWithName(candidate)
			}
			healthyMembers = append(healthyMembers, memberName)
			if member.State == 1 {
				leader = memberName
			}
		}
	}

	if candidate != "" {
		mgr.Logger.Info("no health member for candidate", "candidate", candidate)
		return nil
	}

	if leader != "" {
		return cluster.GetMemberWithName(leader)
	}

	// TODO: use lag and other info to pick the healthiest member
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	healthiestMember := healthyMembers[r.Intn(len(healthyMembers))]
	return cluster.GetMemberWithName(healthiestMember)

}

func (mgr *Manager) HasOtherHealthyLeader(ctx context.Context, cluster *dcs.Cluster) *dcs.Member {
	rsStatus, _ := mgr.GetReplSetStatus(ctx)
	if rsStatus == nil {
		return nil
	}
	healthMembers := map[string]struct{}{}
	var otherLeader string
	for _, member := range rsStatus.Members {
		memberName := strings.Split(member.Name, ".")[0]
		if member.State == 1 || member.State == 2 {
			healthMembers[memberName] = struct{}{}
		}

		if member.State != 1 {
			continue
		}
		if memberName != mgr.CurrentMemberName {
			otherLeader = memberName
		}
	}
	if otherLeader != "" {
		return cluster.GetMemberWithName(otherLeader)
	}

	rsConfig, err := mgr.GetReplSetConfig(ctx)
	if rsConfig == nil {
		mgr.Logger.Error(err, "Get replSet config failed")
		return nil
	}

	for _, mb := range rsConfig.Members {
		memberName := strings.Split(mb.Host, ".")[0]
		if mb.Priority == PrimaryPriority && memberName != mgr.CurrentMemberName {
			if _, ok := healthMembers[memberName]; ok {
				otherLeader = memberName
			}
		}
	}

	if otherLeader != "" {
		return cluster.GetMemberWithName(otherLeader)
	}

	return nil
}

// HasOtherHealthyMembers Are there any healthy members other than the leader?
func (mgr *Manager) HasOtherHealthyMembers(ctx context.Context, cluster *dcs.Cluster, leader string) []*dcs.Member {
	members := make([]*dcs.Member, 0)
	rsStatus, _ := mgr.GetReplSetStatus(ctx)
	if rsStatus == nil {
		return members
	}

	for _, member := range rsStatus.Members {
		if member.Health != 1 {
			continue
		}
		memberName := strings.Split(member.Name, ".")[0]
		if memberName == leader {
			continue
		}
		member := cluster.GetMemberWithName(memberName)
		if member != nil {
			members = append(members, member)
		}
	}

	return members
}

func (mgr *Manager) Lock(ctx context.Context, reason string) error {
	mgr.Logger.Info(fmt.Sprintf("Lock db: %s", reason))
	m := bson.D{
		{Key: "fsync", Value: 1},
		{Key: "lock", Value: true},
		{Key: "comment", Value: reason},
	}
	lockResp := LockResp{}

	response := mgr.Client.Database("admin").RunCommand(ctx, m)
	if response.Err() != nil {
		mgr.Logger.Error(response.Err(), fmt.Sprintf("Lock db (%s) failed", reason))
		return response.Err()
	}
	if err := response.Decode(&lockResp); err != nil {
		err := errors.Wrap(err, "failed to decode lock response")
		return err
	}

	if lockResp.OK != 1 {
		err := errors.Errorf("mongo says: %s", lockResp.Errmsg)
		return err
	}
	mgr.IsLocked = true
	mgr.Logger.Info(fmt.Sprintf("Lock db success times: %d", lockResp.LockCount))
	return nil
}

func (mgr *Manager) Unlock(ctx context.Context) error {
	mgr.Logger.Info("Unlock db")
	m := bson.M{"fsyncUnlock": 1}
	unlockResp := LockResp{}
	response := mgr.Client.Database("admin").RunCommand(ctx, m)
	if response.Err() != nil {
		mgr.Logger.Error(response.Err(), "Unlock db failed")
		return response.Err()
	}
	if err := response.Decode(&unlockResp); err != nil {
		err := errors.Wrap(err, "failed to decode unlock response")
		return err
	}

	if unlockResp.OK != 1 {
		err := errors.Errorf("mongo says: %s", unlockResp.Errmsg)
		return err
	}
	for unlockResp.LockCount > 0 {
		response = mgr.Client.Database("admin").RunCommand(ctx, m)
		if response.Err() != nil {
			mgr.Logger.Error(response.Err(), "Unlock db failed")
			return response.Err()
		}
		if err := response.Decode(&unlockResp); err != nil {
			err := errors.Wrap(err, "failed to decode unlock response")
			return err
		}
	}
	mgr.IsLocked = false
	mgr.Logger.Info("Unlock db success")
	return nil
}
