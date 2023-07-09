package mysql

import (
	"context"
	"database/sql"
	"fmt"
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
	serverID                     uint
	version                      string
	binlogFormat                 string
	logbinEnabled                bool
	logReplicationUpdatesEnabled bool
}

var Mgr *Manager
var _ component.DBManager = &Manager{}

func NewManager(logger logger.Logger) (*Manager, error) {
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
	component.RegisterManager("mysql", Mgr)
	return Mgr, nil
}

func (mgr *Manager) Initialize() {}

func (mgr *Manager) IsRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// test if db is ready to connect or not
	err := mgr.DB.PingContext(ctx)
	if err != nil {
		mgr.Logger.Infof("DB is not ready: %v", err)
		return false
	}

	return true
}

func (mgr *Manager) IsDBStartupReady() bool {
	if mgr.DBStartupReady {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// test if db is ready to connect or not
	err := mgr.DB.PingContext(ctx)
	if err != nil {
		mgr.Logger.Infof("DB is not ready: %v", err)
		return false
	}

	mgr.DBStartupReady = true
	mgr.Logger.Infof("DB startup ready")
	return true
}

func (mgr *Manager) IsReadonly(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (bool, error) {
	var db *sql.DB
	if member != nil {
		addr := cluster.GetMemberAddr(*member)
		db, err := config.GetDBConnWithAddr(addr)
		if err != nil {
			mgr.Logger.Infof("Get Member conn failed: %v", err)
			return false, err
		}
		if db != nil {
			defer db.Close()
		}
	} else {
		db = mgr.DB
	}

	var readonly bool
	err := db.QueryRowContext(ctx, "select @@global.hostname, @@global.server_id, @@global.version, "+
		"@@global.read_only, @@global.binlog_format, @@global.log_bin, @@global.log_slave_updates").
		Scan(&mgr.hostname, &mgr.serverID, &mgr.version, &readonly, &mgr.binlogFormat,
			&mgr.logbinEnabled, &mgr.logReplicationUpdatesEnabled)
	if err != nil {
		mgr.Logger.Infof("Get global readonly failed: %v", err)
		return false, err
	}
	return readonly, nil
}

func (mgr *Manager) IsLeader(ctx context.Context) (bool, error) {
	readonly, err := mgr.IsReadonly(ctx, nil, nil)
	return !readonly, err
}

func (mgr *Manager) IsLeaderMember(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (bool, error) {
	readonly, err := mgr.IsReadonly(ctx, cluster, member)
	return !readonly, err
}

func (mgr *Manager) InitiateCluster(cluster *dcs.Cluster) error {
	return nil
}

func (mgr *Manager) GetMemberAddrs(cluster *dcs.Cluster) []string {
	return cluster.GetMemberAddrs()
}

func (mgr *Manager) GetLeaderClient(ctx context.Context, cluster *dcs.Cluster) (*sql.DB, error) {
	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		return nil, fmt.Errorf("cluster has no leader")
	}

	addr := cluster.GetMemberAddr(*leaderMember)
	return config.GetDBConnWithAddr(addr)
}

func (mgr *Manager) IsCurrentMemberInCluster(cluster *dcs.Cluster) bool {
	return true
}

func (mgr *Manager) IsCurrentMemberHealthy() bool {
	return mgr.IsMemberHealthy(nil, nil)
}

func (mgr *Manager) IsMemberHealthy(cluster *dcs.Cluster, member *dcs.Member) bool {
	var db *sql.DB
	var err error
	if member != nil {
		addr := cluster.GetMemberAddr(*member)
		db, err = config.GetDBConnWithAddr(addr)
		if err != nil {
			mgr.Logger.Infof("Get Member conn failed: %v", err)
			return false
		}
		if db != nil {
			defer db.Close()
		}
	} else {
		db = mgr.DB
	}

	roSQL := `select 1`
	_, err = db.Query(roSQL)
	if err != nil {
		mgr.Logger.Infof("Check Member failed: %v", err)
		return false
	}
	return true
}

func (mgr *Manager) Recover() {}

func (mgr *Manager) AddCurrentMemberToCluster(cluster *dcs.Cluster) error {
	return nil
}

func (mgr *Manager) DeleteMemberFromCluster(cluster *dcs.Cluster, host string) error {
	return nil
}

func (mgr *Manager) IsClusterHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		return fmt.Errorf("cluster has no leader")
	}

	return mgr.IsMemberHealthy(cluster, leaderMember)
}

// IsClusterInitialized is a method to check if cluster is initailized or not
func (mgr *Manager) IsClusterInitialized(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	return true, nil
}

func (mgr *Manager) Premote() error {
	stopReadOnly := `set global read_only=off;set global super_read_only=off;`
	stopSlave := `stop slave;`
	resp, err := mgr.DB.Exec(stopReadOnly + stopSlave)
	if err != nil {
		mgr.Logger.Errorf("promote err: %v", err)
		return err
	}

	mgr.Logger.Infof("promote success, resp:%v", resp)
	return nil
}

func (mgr *Manager) Demote() error {
	setReadOnly := `set global read_only=on;set global super_read_only=on;`

	_, err := mgr.DB.Exec(setReadOnly)
	if err != nil {
		mgr.Logger.Errorf("demote err: %v", err)
		return err
	}
	return nil
}

func (mgr *Manager) Follow(cluster *dcs.Cluster) error {
	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		return fmt.Errorf("cluster has no leader")
	}

	if mgr.CurrentMemberName == cluster.Leader.Name {
		mgr.Logger.Infof("i get the leader key, don't need to follow")
		return nil
	}

	stopSlave := `stop slave;`
	changeMaster := fmt.Sprintf(`change master to master_host='%s',master_user='%s',master_password='%s',master_port=%s,master_auto_position=1;`,
		cluster.GetMemberAddr(*leaderMember), config.username, config.password, leaderMember.DBPort)
	startSlave := `start slave;`

	_, err := mgr.DB.Exec(stopSlave + changeMaster + startSlave)
	if err != nil {
		mgr.Logger.Errorf("sql query failed, err:%v", err)
	}

	mgr.Logger.Infof("successfully follow new leader:%s", leaderMember.Name)
	return nil
}

func (mgr *Manager) checkRecoveryConf(ctx context.Context, leader string) bool {
	// sql := "show slave status"
	// data, err := mysqlOps.query(ctx, sql)
	// if err != nil {
	// 	mysqlOps.Logger.Errorf("error executing %s: %v", sql, err)
	// 	return true
	// }

	// result, err := ParseSingleQuery(string(data))
	// if err != nil {
	// 	mysqlOps.Logger.Errorf("parse query err:%v", err)
	// 	return true
	// }

	// if result == nil || strings.Split(result["Master_Host"].(string), ".")[0] != leader {
	// 	return true
	// }

	return false
}

func (mgr *Manager) GetHealthiestMember(cluster *dcs.Cluster, candidate string) *dcs.Member {
	return nil
}

func (mgr *Manager) HasOtherHealthyLeader(cluster *dcs.Cluster) *dcs.Member {
	for _, member := range cluster.Members {
		if member.Name == mgr.CurrentMemberName {
			continue
		}

		isLeader, err := mgr.IsLeaderMember(context.TODO(), cluster, &member)
		if err == nil && isLeader {
			return &member
		}
	}

	return nil
}

func (mgr *Manager) HasOtherHealthyMembers(cluster *dcs.Cluster) []*dcs.Member {
	// members := make([]*dcs.Member, 0)
	// rsStatus, _ := mgr.GetReplSetStatus(context.TODO())
	// if rsStatus == nil {
	// 	return members
	// }

	// for _, member := range rsStatus.Members {
	// 	if member.State != 1 {
	// 		continue
	// 	}
	// 	memberName := strings.Split(member.Name, ".")[0]
	// 	if memberName == mgr.CurrentMemberName {
	// 		continue
	// 	}
	// 	member := cluster.GetMemberWithName(memberName)
	// 	if member != nil {
	// 		members = append(members, member)
	// 	}
	// }

	// return members
	return nil
}
