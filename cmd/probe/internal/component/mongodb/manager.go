package mongodb

import (
	"context"
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

func NewManager(logger logger.Logger) (*Manager, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.Client().
		SetHosts([]string{config.host}).
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

	mgr := &Manager{
		DBManagerBase: component.DBManagerBase{
			CurrentMemberName: viper.GetString("KB_POD_FQDN"),
			ClusterCompName:   viper.GetString("KB_CLUSTER_COMP_NAME"),
			Logger:            logger,
		},
		Client:   client,
		Database: client.Database(config.databaseName),
	}
	return mgr, nil

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
	status := &ReplSetStatus{}

	resp := mgr.Client.Database("admin").RunCommand(ctx, bson.D{{Key: "replSetGetStatus", Value: 1}})
	if resp.Err() != nil {
		return status, errors.Wrap(resp.Err(), "replSetGetStatus")
	}

	if err := resp.Decode(status); err != nil {
		return status, errors.Wrap(err, "failed to decode rs status")
	}

	if status.OK != 1 {
		return status, errors.Errorf("mongo says: %s", status.Errmsg)
	}

	return status, nil
}

func (mgr *Manager) IsLeader(ctx context.Context) (bool, error) {
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

func (mgr *Manager) InitiateCluster(cluster *dcs.Cluster) error {
	return nil
}

// InitiateMongoClusterRS is a method to create MongoDB cluster
func (mgr *Manager) InitiateReplSet(cluster *dcs.Cluster) error {
	configMembers := make([]ConfigMember, len(cluster.Members))

	for i, member := range cluster.Members {
		configMembers[i].ID = i
		configMembers[i].Host = member.Name + ":" + member.DBPort
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

	response := mgr.Client.Database("admin").RunCommand(context.Background(), bson.M{"replSetInitiate": config})
	if response.Err() != nil {
		return response.Err()
	}
	return nil
}

// CheckMongoClusterInitialized is a method to check if cluster is initailized or not
func (mgr *Manager) IsClusterInitialized() (bool, error) {
	status, err := mgr.GetReplSetStatus(context.Background())
	if err != nil {
		return false, err
	}
	if status.OK != 0 {
		return true, nil
	}
	return false, nil
}

func (mgr *Manager) Initialize()             {}
func (mgr *Manager) IsRunning()              {}
func (mgr *Manager) IsHealthy()              {}
func (mgr *Manager) Recover()                {}
func (mgr *Manager) AddToCluster()           {}
func (mgr *Manager) Premote()                {}
func (mgr *Manager) Demote()                 {}
func (mgr *Manager) GetHealthiestMember()    {}
func (mgr *Manager) HasOtherHealthtyLeader() {}
