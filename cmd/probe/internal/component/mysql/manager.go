package mongodb

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/dapr/kit/logger"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/component"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
)

type Manager struct {
	component.DBManagerBase
	DB                           *sql.DB
	hostname                     string
	serverId                     uint
	version                      string
	binlogFormat                 string
	logbinEnabled                bool
	logReplicationUpdatesEnabled bool
}

var Mgr *Manager

func NewManager(logger logger.Logger) (*Manager, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := config.GetLocalDBConn()
	if err != nil {
		return nil, errors.Wrap(err, "connect to MySQL")
	}

	defer func() {
		if err != nil {
			derr := db.Close()
			if derr != nil {
				logger.Errorf("failed to close: %v", err)
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
		DB: db,
	}
	return Mgr, nil

}

func (mgr *Manager) IsDBStartupReady() bool {
	if mgr.DBStartupReady {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// test if db is ready to connect or not
	err = mgr.db.Ping()
	if err != nil {
		mgr.Logger.Infof("DB is not ready: %v", err)
		return false
	}

	mgr.DBStartupReady = true
	mgr.Logger.Infof("DB startup ready")
	return true
}

func (mgr *Manager) IsReadonly(ctx context.Context) (bool, error) {
	var readonly bool
	err = mgr.DB.QueryRowContext(ctx, "select @@global.hostname, @@global.server_id, @@global.version, "+
		"@@global.read_only, @@global.binlog_format, @@global.log_bin, @@global.log_slave_updates").Scan(
		&mgr.hostname, &mgr.serverID, &mgr.version, &readonly, &mgr.binlogFormat,
		&mgr.logbinEnabled, &mgr.logReplicationUpdatesEnabled)
	if err != nil {
		return false, err
	}
	return readonly, nil
}

func (mgr *Manager) IsLeader(ctx context.Context) (bool, error) {
	readonly, err := mgr.IsReadonly(ctx)
	return !readonly, err
}

func (mgr *Manager) InitiateCluster(cluster *dcs.Cluster) error {
	return nil
}

func (mgr *Manager) GetMemberAddrs(cluster *dcs.Cluster) []string {
	return cluster.GetMemberAddrs()
}

func (mgr *Manager) GetLeaderClient(ctx context.Context, cluster *dcs.Cluster) (*mysql.DB, error) {
	if cluster.Leader == nil || cluster.Leader.Name == "" {
		return nil, fmt.Errorf("cluster has no leader")
	}

	leaderMember := cluster.GetMemberWithName(cluster.Leader.Name)
	addr := cluster.GetMemberAddr(*leaderMember)
	return config.GetDBConnWithAddr(addr)
}

func (mgr *Manager) Initialize() {}
func (mgr *Manager) IsRunning()  {}

func (mgr *Manager) IsCurrentMemberInCluster(cluster *dcs.Cluster) bool {
	return true
}

func (mgr *Manager) IsCurrentMemberHealthy() bool {
	return mgr.IsMemberHealthy(mgr.CurrentMemberName)
}

func (mgr *Manager) IsMemberHealthy(memberName string) bool {
	rsStatus, _ := mgr.GetReplSetStatus(context.TODO())
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
	return nil
}

func (mgr *Manager) DeleteMemberFromCluster(cluster *dcs.Cluster, host string) error {
	return nil
}

func (mgr *Manager) IsClusterHealthy(ctx context.Context, cluster *dcs.Cluster) bool {

	client, err := mgr.GetReplSetClient(ctx, cluster)
	if err != nil {
		mgr.Logger.Debugf("Get leader client failed: %v", err)
		return false
	}
	defer client.Disconnect(ctx)
	status, err := mgr.GetReplSetStatusWithClient(ctx, client)
	if err != nil {
		return false
	}
	mgr.Logger.Debugf("cluster status: %v", status)
	if status.OK != 0 {
		return true
	}
	return false
}

// IsClusterInitialized is a method to check if cluster is initailized or not
func (mgr *Manager) IsClusterInitialized(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	client, err := mgr.GetLeaderClient(ctx, cluster)
	if err != nil {
		return true, err
	}

	defer client.Disconnect(ctx)
	rsConfig, err := mgr.GetReplSetConfigWithClient(ctx, client)
	if rsConfig == nil {
		mgr.Logger.Errorf("Get replSet config failed: %v", err)
		return false, err
	}
	return rsConfig.ID != "", nil
}

func (mgr *Manager) Premote() error {
	rsConfig, err := mgr.GetReplSetConfig(context.TODO())
	if rsConfig == nil {
		mgr.Logger.Errorf("Get replSet config failed: %v", err)
		return err
	}

	hosts := mgr.GetMemberAddrsFromRSConfig(rsConfig)
	client, _ := mgr.GetReplSetClientWithHosts(context.TODO(), hosts)
	defer client.Disconnect(context.TODO())
	for i := range rsConfig.Members {
		if strings.HasPrefix(rsConfig.Members[i].Host, mgr.CurrentMemberName) {
			rsConfig.Members[i].Priority = 2
		} else if rsConfig.Members[i].Priority == 2 {
			rsConfig.Members[i].Priority = 1
		}
	}

	rsConfig.Version++
	return mgr.SetReplSetConfig(context.TODO(), client, rsConfig)
}

func (mgr *Manager) Demote() error {
	// mongodb do premote and demote in one action, here do nothing.
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

func (mgr *Manager) HasOtherHealthyLeader(cluster *dcs.Cluster) *dcs.Member {
	rsStatus, _ := mgr.GetReplSetStatus(context.TODO())
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

func (mgr *Manager) HasOtherHealthyMembers(cluster *dcs.Cluster) []*dcs.Member {
	members := make([]*dcs.Member, 0)
	rsStatus, _ := mgr.GetReplSetStatus(context.TODO())
	if rsStatus == nil {
		return members
	}

	for _, member := range rsStatus.Members {
		if member.State != 1 {
			continue
		}
		memberName := strings.Split(member.Name, ".")[0]
		if memberName == mgr.CurrentMemberName {
			continue
		}
		member := cluster.GetMemberWithName(memberName)
		if member != nil {
			members = append(members, member)
		}
	}

	return members
}
