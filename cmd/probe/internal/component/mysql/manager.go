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

package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/component"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
)

const (
	addFollowerFmt    = "call dbms_consensus.add_learner('%s');"
	demoteFollowerFmt = "call dbms_consensus.downgrade_follower('%s');"
	dropLearnerFmt    = "call dbms_consensus.drop_learner('%s');"
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

func NewManager(logger logr.Logger) (*Manager, error) {
	db, err := config.GetLocalDBConn()
	if err != nil {
		return nil, errors.Wrap(err, "connect to MySQL")
	}

	defer func() {
		if err != nil {
			derr := db.Close()
			if derr != nil {
				logger.Error(err, "failed to close")
			}
		}
	}()

	currentMemberName := viper.GetString("KB_POD_NAME")
	if currentMemberName == "" {
		return nil, fmt.Errorf("KB_POD_NAME is not set")
	}

	serverID, err := getIndex(currentMemberName)
	if err != nil {
		return nil, err
	}

	Mgr = &Manager{
		DBManagerBase: component.DBManagerBase{
			CurrentMemberName: currentMemberName,
			ClusterCompName:   viper.GetString("KB_CLUSTER_COMP_NAME"),
			Namespace:         viper.GetString("KB_NAMESPACE"),
			Logger:            logger,
		},
		DB:       db,
		serverID: uint(serverID) + 1,
	}

	component.RegisterManager("mysql", Mgr)
	return Mgr, nil
}

func getIndex(memberName string) (int, error) {
	i := strings.LastIndex(memberName, "-")
	if i < 0 {
		return 0, fmt.Errorf("the format of member name is wrong: %s", memberName)
	}
	return strconv.Atoi(memberName[i+1:])
}

func (mgr *Manager) InitializeCluster(ctx context.Context, cluster *dcs.Cluster) error {
	return nil
}

func (mgr *Manager) IsRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// test if db is ready to connect or not
	err := mgr.DB.PingContext(ctx)
	if err != nil {
		if driverErr, ok := err.(*mysql.MySQLError); ok {
			// Now the error number is accessible directly
			if driverErr.Number == 1040 {
				mgr.Logger.Error(err, "Too many connections")
				return true
			}
		}
		mgr.Logger.Error(err, "DB is not ready")
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
		mgr.Logger.Error(err, "DB is not ready")
		return false
	}

	mgr.DBStartupReady = true
	mgr.Logger.Info("DB startup ready")
	return true
}

func (mgr *Manager) IsReadonly(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (bool, error) {
	var db *sql.DB
	var err error
	if member != nil {
		addr := cluster.GetMemberAddrWithPort(*member)
		db, err = config.GetDBConnWithAddr(addr)
		if err != nil {
			mgr.Logger.Error(err, "Get Member conn failed")
			return false, err
		}
		if db != nil {
			defer db.Close()
		}
	} else {
		db = mgr.DB
	}

	var readonly bool
	err = db.QueryRowContext(ctx, "select @@global.hostname, @@global.version, "+
		"@@global.read_only, @@global.binlog_format, @@global.log_bin, @@global.log_slave_updates").
		Scan(&mgr.hostname, &mgr.version, &readonly, &mgr.binlogFormat,
			&mgr.logbinEnabled, &mgr.logReplicationUpdatesEnabled)
	if err != nil {
		mgr.Logger.Error(err, "Get global readonly failed")
		return false, err
	}
	return readonly, nil
}

func (mgr *Manager) IsLeader(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	readonly, err := mgr.IsReadonly(ctx, nil, nil)

	if err != nil || readonly {
		return false, err
	}

	// if cluster.Leader != nil && cluster.Leader.Name != "" {
	// 	if cluster.Leader.Name == mgr.CurrentMemberName {
	// 		return true, nil
	// 	} else {
	// 		return false, nil
	// 	}
	// }

	// // During the initialization of cluster, there would be more than one leader,
	// // in this case, the first member is chosen as the leader
	// if mgr.CurrentMemberName == cluster.Members[0].Name {
	// 	return true, nil
	// }
	// isFirstMemberLeader, err := mgr.IsLeaderMember(ctx, cluster, &cluster.Members[0])
	// if err == nil && isFirstMemberLeader {
	// 	return false, nil
	// }

	return true, err
}

func (mgr *Manager) IsLeaderMember(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (bool, error) {
	readonly, err := mgr.IsReadonly(ctx, cluster, member)
	if err != nil || readonly {
		return false, err
	}

	return true, err
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

	addr := cluster.GetMemberAddrWithPort(*leaderMember)
	return config.GetDBConnWithAddr(addr)
}

func (mgr *Manager) IsCurrentMemberInCluster(ctx context.Context, cluster *dcs.Cluster) bool {
	return true
}

func (mgr *Manager) IsCurrentMemberHealthy(ctx context.Context) bool {
	_, _ = mgr.EnsureServerID(ctx)
	return mgr.IsMemberHealthy(ctx, nil, nil)
}

func (mgr *Manager) IsMemberHealthy(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) bool {
	var db *sql.DB
	var err error
	if member != nil {
		addr := cluster.GetMemberAddrWithPort(*member)
		db, err = config.GetDBConnWithAddr(addr)
		if err != nil {
			mgr.Logger.Error(err, "Get Member conn failed")
			return false
		}
		if db != nil {
			defer db.Close()
		}
	} else {
		db = mgr.DB
	}

	roSQL := `select 1`
	rows, err := db.Query(roSQL)
	if rows != nil {
		defer rows.Close()
	}
	if err != nil {
		mgr.Logger.Error(err, "Check Member failed")
		return false
	}
	return true
}

func (mgr *Manager) Recover() {}

func (mgr *Manager) AddCurrentMemberToCluster(cluster *dcs.Cluster) error {
	// fixme what if there is no leader
	client, err := mgr.GetLeaderClient(context.Background(), cluster)
	if err != nil {
		return err
	}
	defer client.Close() //nolint:errcheck

	currentMember := cluster.GetMemberWithName(mgr.GetCurrentMemberName())
	currentHost := cluster.GetMemberAddrWithPort(*currentMember)
	sql := fmt.Sprintf(addFollowerFmt, currentHost)
	if _, err = client.Exec(sql); err != nil {
		return err
	}
	// todo implement learner

	return nil
}

func (mgr *Manager) DeleteMemberFromCluster(cluster *dcs.Cluster, host string) error {
	client, err := mgr.GetLeaderClient(context.Background(), cluster)
	if err != nil {
		return err
	}
	defer client.Close() //nolint:errcheck

	currentMember := cluster.GetMemberWithName(mgr.GetCurrentMemberName())
	currentHost := cluster.GetMemberAddrWithPort(*currentMember)
	sql := fmt.Sprintf(demoteFollowerFmt, currentHost)
	if _, err = client.Exec(sql); err != nil {
		return err
	}
	sql = fmt.Sprintf(dropLearnerFmt, currentHost)
	if _, err = client.Exec(sql); err != nil {
		return err
	}

	return nil
}

func (mgr *Manager) IsClusterHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		mgr.Logger.Info("cluster has no leader, wait for leader to take the lock")
		return true
	}

	return mgr.IsMemberHealthy(ctx, cluster, leaderMember)
}

// IsClusterInitialized is a method to check if cluster is initailized or not
func (mgr *Manager) IsClusterInitialized(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	return mgr.EnsureServerID(ctx)
}

func (mgr *Manager) EnsureServerID(ctx context.Context) (bool, error) {
	var serverID uint
	err := mgr.DB.QueryRowContext(ctx, "select @@global.server_id").Scan(&serverID)
	if err != nil {
		mgr.Logger.Error(err, "Get global server id failed: %v")
		return false, err
	}
	if serverID == mgr.serverID {
		return true, nil
	}
	mgr.Logger.Info("Set global server id : ")

	setServerID := fmt.Sprintf(`set global server_id = %d`, mgr.serverID)
	mgr.Logger.Info("Set global server id", "server-id", setServerID)
	_, err = mgr.DB.Exec(setServerID)
	if err != nil {
		mgr.Logger.Error(err, "set server id err")
		return false, err
	}

	return true, nil
}

func (mgr *Manager) Promote() error {
	stopReadOnly := `set global read_only=off;set global super_read_only=off;`
	stopSlave := `stop slave;`
	resp, err := mgr.DB.Exec(stopReadOnly + stopSlave)
	if err != nil {
		mgr.Logger.Error(err, "promote err")
		return err
	}

	mgr.Logger.Info(fmt.Sprintf("promote success, resp:%v", resp))
	return nil
}

func (mgr *Manager) Demote() error {
	setReadOnly := `set global read_only=on;set global super_read_only=on;`

	_, err := mgr.DB.Exec(setReadOnly)
	if err != nil {
		mgr.Logger.Error(err, "demote err")
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
		mgr.Logger.Info("i get the leader key, don't need to follow")
		return nil
	}

	if !mgr.isRecoveryConfOutdate(context.TODO(), cluster.Leader.Name) {
		return nil
	}

	stopSlave := `stop slave;`
	changeMaster := fmt.Sprintf(`change master to master_host='%s',master_user='%s',master_password='%s',master_port=%s,master_auto_position=1;`,
		cluster.GetMemberAddr(*leaderMember), config.username, config.password, leaderMember.DBPort)
	startSlave := `start slave;`

	_, err := mgr.DB.Exec(stopSlave + changeMaster + startSlave)
	if err != nil {
		mgr.Logger.Error(err, "sql query failed, err")
	}

	mgr.Logger.Info("successfully follow new leader", "leader-name", leaderMember.Name)
	return nil
}

func (mgr *Manager) isRecoveryConfOutdate(ctx context.Context, leader string) bool {
	sql := "show slave status"
	var rowMap RowMap

	err := QueryRowsMap(mgr.DB, sql, func(rMap RowMap) error {
		rowMap = rMap
		return nil
	})
	if err != nil {
		mgr.Logger.Error(err, fmt.Sprintf("error executing %s", sql))
		return true
	}

	if len(rowMap) == 0 {
		return true
	}

	ioError := rowMap.GetString("Last_IO_Error")
	sqlError := rowMap.GetString("Last_SQL_Error")
	if ioError != "" || sqlError != "" {
		mgr.Logger.Infof("slave status error, sqlError: %s, ioError: %s", sqlError, ioError)
		return true
	}

	masterHost := rowMap.GetString("Master_Host")
	return !strings.HasPrefix(masterHost, leader)
}

func (mgr *Manager) GetHealthiestMember(cluster *dcs.Cluster, candidate string) *dcs.Member {
	// 要是想拿到其他mysql的lag, 有三种方法:
	// 1. k8s有没性能监控 ?
	// 2. mysqlOps 获取
	// 3. 另外有一个地方存着所有实例的lag
	return nil
}

func (mgr *Manager) HasOtherHealthyLeader(ctx context.Context, cluster *dcs.Cluster) *dcs.Member {
	isLeader, err := mgr.IsLeader(ctx, cluster)
	if err == nil && isLeader {
		// if current member is leader, just return
		return nil
	}

	for _, member := range cluster.Members {
		if member.Name == mgr.CurrentMemberName {
			continue
		}

		isLeader, err := mgr.IsLeaderMember(ctx, cluster, &member)
		if err == nil && isLeader {
			return &member
		}
	}

	return nil
}

// HasOtherHealthyMembers checks if there are any healthy members, excluding the leader
func (mgr *Manager) HasOtherHealthyMembers(ctx context.Context, cluster *dcs.Cluster, leader string) []*dcs.Member {
	members := make([]*dcs.Member, 0)
	for _, member := range cluster.Members {
		if member.Name == leader {
			continue
		}
		if !mgr.IsMemberHealthy(ctx, cluster, &member) {
			continue
		}
		members = append(members, &member)
	}

	return members
}

func (mgr *Manager) IsRootCreated(ctx context.Context) (bool, error) {
	return true, nil
}

func (mgr *Manager) CreateRoot(ctx context.Context) error {
	return nil
}

func (mgr *Manager) Lock(ctx context.Context, reason string) error {
	setReadOnly := `set global read_only=on;`

	_, err := mgr.DB.Exec(setReadOnly)
	if err != nil {
		mgr.Logger.Errorf("Lock err: %v", err)
		return err
	}
	return nil
}

func (mgr *Manager) Unlock(ctx context.Context) error {
	setReadOnlyOff := `set global read_only=off;`

	_, err := mgr.DB.Exec(setReadOnlyOff)
	if err != nil {
		mgr.Logger.Errorf("Unlock err: %v", err)
		return err
	}
	return nil
}
