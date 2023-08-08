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
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/dapr/kit/logger"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/component"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
)

type Manager struct {
	component.DBManagerBase
	Client   *mongo.Client
	Database *mongo.Database
}

var Mgr *Manager
var _ component.DBManager = &Manager{}

func NewManager(logger logger.Logger) (*Manager, error) {
	ctx := context.Background()

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
				logger.Errorf("failed to disconnect: %v", err)
			}
		}
	}()

	Mgr = &Manager{
		DBManagerBase: component.DBManagerBase{
			CurrentMemberName: viper.GetString("KB_POD_NAME"),
			ClusterCompName:   viper.GetString("KB_CLUSTER_COMP_NAME"),
			Namespace:         viper.GetString("KB_NAMESPACE"),
			Logger:            logger,
		},
		Client:   client,
		Database: client.Database(config.databaseName),
	}

	component.RegisterManager("mongodb", Mgr)
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
			configMembers[i].Priority = 2
		} else {
			configMembers[i].Priority = 1
		}
	}

	config := RSConfig{
		ID:      mgr.ClusterCompName,
		Members: configMembers,
	}
	client, err := NewLocalUnauthClient(ctx)
	if err != nil {
		mgr.Logger.Debugf("Get local unauth client failed: %v", err)
		return err
	}
	defer client.Disconnect(context.TODO()) //nolint:errcheck

	mgr.Logger.Infof("Initial Replset Config: %v", config)
	response := client.Database("admin").RunCommand(ctx, bson.M{"replSetInitiate": config})
	if response.Err() != nil {
		return response.Err()
	}
	return nil
}

// IsClusterInitialized is a method to check if cluster is initailized or not
func (mgr *Manager) IsClusterInitialized(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	client, err := mgr.GetReplSetClient(ctx, cluster)
	if err != nil {
		mgr.Logger.Debugf("Get leader client failed: %v", err)
		return false, err
	}
	defer client.Disconnect(ctx) //nolint:errcheck

	ctx1, cancel := context.WithTimeout(ctx, 1000*time.Millisecond)
	defer cancel()
	rsStatus, err := GetReplSetStatus(ctx1, client)
	if rsStatus != nil {
		return rsStatus.Set != "", nil
	}
	mgr.Logger.Infof("Get replSet status failed: %v", err)

	if !mgr.IsFirstMember() {
		return false, nil
	}

	client, err = NewLocalUnauthClient(ctx)
	if err != nil {
		mgr.Logger.Infof("Get local unauth client failed: %v", err)
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
	mgr.Logger.Infof("Get replSet status with local unauth client failed: %v", err)

	rsStatus, err = mgr.GetReplSetStatus(ctx)
	if rsStatus != nil {
		return rsStatus.Set != "", nil
	}
	if err != nil {
		mgr.Logger.Infof("Get replSet status with local auth client failed: %v", err)
		return false, err
	}

	mgr.Logger.Errorf("Get replSet status failed: %v", err)
	return false, err
}

func (mgr *Manager) IsRootCreated(ctx context.Context) (bool, error) {
	if !mgr.IsFirstMember() {
		return true, nil
	}

	client, err := NewLocalUnauthClient(ctx)
	if err != nil {
		mgr.Logger.Infof("Get local unauth client failed: %v", err)
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

	mgr.Logger.Infof("Get replSet status with local unauth client failed: %v", err)

	_, err = mgr.GetReplSetStatus(ctx)
	if err == nil {
		return true, nil
	}

	mgr.Logger.Infof("Get replSet status with local auth client failed: %v", err)
	return false, err

}

func (mgr *Manager) CreateRoot(ctx context.Context) error {
	if !mgr.IsFirstMember() {
		return nil
	}

	client, err := NewLocalUnauthClient(ctx)
	if err != nil {
		mgr.Logger.Infof("Get local unauth client failed: %v", err)
		return err
	}
	defer client.Disconnect(ctx) //nolint:errcheck

	role := map[string]interface{}{
		"role": "root",
		"db":   "admin",
	}

	mgr.Logger.Infof("Create user: %s, passwd: %s, roles: %v", config.username, config.password, role)
	err = CreateUser(ctx, client, config.username, config.password, role)
	if err != nil {
		mgr.Logger.Errorf("Create Root failed: %v", err)
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
		mgr.Logger.Infof("DB is not ready: %v", err)
		return false
	}
	mgr.DBStartupReady = true
	mgr.Logger.Infof("DB startup ready")
	return true
}

func (mgr *Manager) GetMemberState(ctx context.Context) (string, error) {
	status, err := mgr.GetReplSetStatus(ctx)
	if err != nil {
		mgr.Logger.Errorf("rs.status() error: %", err)
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
		mgr.Logger.Errorf("rs.status() error: %", err)
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

func (mgr *Manager) GetMemberAddrs(cluster *dcs.Cluster) []string {
	client, err := mgr.GetReplSetClient(context.TODO(), cluster)
	if err != nil {
		mgr.Logger.Errorf("Get replSet client failed: %v", err)
		return nil
	}
	defer client.Disconnect(context.TODO()) //nolint:errcheck

	rsConfig, err := GetReplSetConfig(context.TODO(), client)
	if rsConfig == nil {
		mgr.Logger.Errorf("Get replSet config failed: %v", err)
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
		mgr.Logger.Errorf("Get replset client whitout hosts")
		return nil, errors.New("Get replset client whitout hosts")
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
		mgr.Logger.Errorf("Get replSet client failed: %v", err)
		return true
	}
	defer client.Disconnect(ctx) //nolint:errcheck

	rsConfig, err := GetReplSetConfig(ctx, client)
	if rsConfig == nil {
		mgr.Logger.Errorf("Get replSet config failed: %v", err)
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

func (mgr *Manager) IsCurrentMemberHealthy(ctx context.Context) bool {
	return mgr.IsMemberHealthy(ctx, nil, nil)
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

func (mgr *Manager) Recover() {}

func (mgr *Manager) AddCurrentMemberToCluster(cluster *dcs.Cluster) error {
	client, err := mgr.GetReplSetClient(context.TODO(), cluster)
	if err != nil {
		return err
	}
	defer client.Disconnect(context.TODO()) //nolint:errcheck

	currentMember := cluster.GetMemberWithName(mgr.GetCurrentMemberName())
	currentHost := cluster.GetMemberAddrWithPort(*currentMember)
	rsConfig, err := GetReplSetConfig(context.TODO(), client)
	if rsConfig == nil {
		mgr.Logger.Errorf("Get replSet config failed: %v", err)
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
	configMember.Priority = 1
	rsConfig.Members = append(rsConfig.Members, configMember)

	rsConfig.Version++
	return SetReplSetConfig(context.TODO(), client, rsConfig)
}

func (mgr *Manager) DeleteMemberFromCluster(cluster *dcs.Cluster, host string) error {
	client, err := mgr.GetReplSetClient(context.TODO(), cluster)
	if err != nil {
		return err
	}
	defer client.Disconnect(context.TODO()) //nolint:errcheck

	rsConfig, err := GetReplSetConfig(context.TODO(), client)
	if rsConfig == nil {
		mgr.Logger.Errorf("Get replSet config failed: %v", err)
		return err
	}

	mgr.Logger.Infof("Delete member: %s", host)
	configMembers := make([]ConfigMember, 0, len(rsConfig.Members)-1)
	for _, configMember := range rsConfig.Members {
		if configMember.Host != host {
			configMembers = append(configMembers, configMember)
		}
	}

	rsConfig.Members = configMembers
	rsConfig.Version++
	return SetReplSetConfig(context.TODO(), client, rsConfig)
}

func (mgr *Manager) IsClusterHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	client, err := mgr.GetReplSetClient(ctx, cluster)
	if err != nil {
		mgr.Logger.Debugf("Get leader client failed: %v", err)
		return false
	}
	defer client.Disconnect(ctx) //nolint:errcheck

	status, err := GetReplSetStatus(ctx, client)
	if err != nil {
		return false
	}
	mgr.Logger.Debugf("cluster status: %v", status)
	return status.OK != 0
}

func (mgr *Manager) Promote() error {
	rsConfig, err := mgr.GetReplSetConfig(context.TODO())
	if rsConfig == nil {
		mgr.Logger.Errorf("Get replSet config failed: %v", err)
		return err
	}

	hosts := mgr.GetMemberAddrsFromRSConfig(rsConfig)
	client, err := NewReplSetClient(context.TODO(), hosts)
	if err != nil {
		return err
	}
	defer client.Disconnect(context.TODO()) //nolint:errcheck

	for i := range rsConfig.Members {
		if strings.HasPrefix(rsConfig.Members[i].Host, mgr.CurrentMemberName) {
			rsConfig.Members[i].Priority = 2
		} else if rsConfig.Members[i].Priority == 2 {
			rsConfig.Members[i].Priority = 1
		}
	}

	rsConfig.Version++
	return SetReplSetConfig(context.TODO(), client, rsConfig)
}

func (mgr *Manager) Demote() error {
	// mongodb do premote and demote in one action, here do nothing.
	return nil
}

func (mgr *Manager) Follow(cluster *dcs.Cluster) error {
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
		mgr.Logger.Infof("no health member for candidate: %s", candidate)
		return nil
	}

	if leader != "" {
		return cluster.GetMemberWithName(leader)
	}

	// TODO: use lag and other info to pick the healthiest member
	rand.Seed(time.Now().Unix())
	healthiestMember := healthyMembers[rand.Intn(len(healthyMembers))]
	return cluster.GetMemberWithName(healthiestMember)

}

func (mgr *Manager) HasOtherHealthyLeader(ctx context.Context, cluster *dcs.Cluster) *dcs.Member {
	rsStatus, _ := mgr.GetReplSetStatus(ctx)
	if rsStatus == nil {
		return nil
	}
	var otherLeader string
	for _, member := range rsStatus.Members {
		if member.State != 1 {
			continue
		}
		memberName := strings.Split(member.Name, ".")[0]
		if memberName != mgr.CurrentMemberName {
			otherLeader = memberName
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
		if member.State != 1 {
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
	mgr.Logger.Infof("Lock db: %s", reason)
	m := bson.D{
		{Key: "fsync", Value: 1},
		{Key: "lock", Value: true},
		{Key: "comment", Value: reason},
	}
	lockResp := LockResp{}

	response := mgr.Client.Database("admin").RunCommand(ctx, m)
	if response.Err() != nil {
		mgr.Logger.Infof("Lock db (%s) failed: %v", reason, response.Err())
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
	mgr.Logger.Infof("Lock db success times: %d", lockResp.LockCount)
	return nil
}

func (mgr *Manager) Unlock(ctx context.Context) error {
	mgr.Logger.Infof("Unlock db")
	m := bson.M{"fsyncUnlock": 1}
	unlockResp := LockResp{}
	response := mgr.Client.Database("admin").RunCommand(ctx, m)
	if response.Err() != nil {
		mgr.Logger.Infof("Unlock db failed: %v", response.Err())
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
			mgr.Logger.Infof("Unlock db failed: %v", response)
			return response.Err()
		}
		if err := response.Decode(&unlockResp); err != nil {
			err := errors.Wrap(err, "failed to decode unlock response")
			return err
		}
	}
	mgr.Logger.Infof("Unlock db success")
	return nil
}
