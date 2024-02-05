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

package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

type Manager struct {
	engines.DBManagerBase
	DB                           *sql.DB
	hostname                     string
	serverID                     uint
	version                      string
	binlogFormat                 string
	logbinEnabled                bool
	logReplicationUpdatesEnabled bool
	opTimestamp                  int64
	globalState                  map[string]string
	masterStatus                 RowMap
	slaveStatus                  RowMap
}

var _ engines.DBManager = &Manager{}

func NewManager(properties engines.Properties) (engines.DBManager, error) {
	logger := ctrl.Log.WithName("MySQL")
	config, err := NewConfig(properties)
	if err != nil {
		return nil, err
	}

	managerBase, err := engines.NewDBManagerBase(logger)
	if err != nil {
		return nil, err
	}

	serverID, err := engines.GetIndex(managerBase.CurrentMemberName)
	if err != nil {
		return nil, err
	}

	db, err := config.GetLocalDBConn()
	if err != nil {
		return nil, errors.Wrap(err, "connect to MySQL")
	}

	mgr := &Manager{
		DBManagerBase: *managerBase,
		serverID:      uint(serverID) + 1,
		DB:            db,
	}

	return mgr, nil
}

func (mgr *Manager) InitializeCluster(context.Context, *dcs.Cluster) error {
	return nil
}

func (mgr *Manager) IsRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// test if db is ready to connect or not
	err := mgr.DB.PingContext(ctx)
	if err != nil {
		var driverErr *mysql.MySQLError
		if errors.As(err, &driverErr) {
			// Now the error number is accessible directly
			if driverErr.Number == 1040 {
				mgr.Logger.Info("connect failed: Too many connections")
				return true
			}
		}
		mgr.Logger.Info("DB is not ready", "error", err)
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
		mgr.Logger.Info("DB is not ready", "error", err)
		return false
	}

	mgr.DBStartupReady = true
	mgr.Logger.Info("DB startup ready")
	return true
}

func (mgr *Manager) IsReadonly(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (bool, error) {
	db, err := mgr.GetMemberConnection(cluster, member)
	if err != nil {
		mgr.Logger.Info("Get Member conn failed", "error", err.Error())
		return false, err
	}

	var readonly bool
	err = db.QueryRowContext(ctx, "select @@global.hostname, @@global.version, "+
		"@@global.read_only, @@global.binlog_format, @@global.log_bin, @@global.log_slave_updates").
		Scan(&mgr.hostname, &mgr.version, &readonly, &mgr.binlogFormat,
			&mgr.logbinEnabled, &mgr.logReplicationUpdatesEnabled)
	if err != nil {
		mgr.Logger.Info("Get global readonly failed", "error", err.Error())
		return false, err
	}
	return readonly, nil
}

func (mgr *Manager) IsLeader(ctx context.Context, _ *dcs.Cluster) (bool, error) {
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

func (mgr *Manager) InitiateCluster(*dcs.Cluster) error {
	return nil
}

func (mgr *Manager) GetMemberAddrs(_ context.Context, cluster *dcs.Cluster) []string {
	return cluster.GetMemberAddrs()
}

func (mgr *Manager) IsCurrentMemberInCluster(context.Context, *dcs.Cluster) bool {
	return true
}

func (mgr *Manager) IsCurrentMemberHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	// _, _ = mgr.EnsureServerID(ctx)
	member := cluster.GetMemberWithName(mgr.CurrentMemberName)

	return mgr.IsMemberHealthy(ctx, cluster, member)
}

func (mgr *Manager) IsMemberLagging(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (bool, int64) {
	var leaderDBState *dcs.DBState
	if cluster.Leader == nil || cluster.Leader.DBState == nil {
		mgr.Logger.Info("No leader DBState info")
		return true, 0
	}
	leaderDBState = cluster.Leader.DBState

	db, err := mgr.GetMemberConnection(cluster, member)
	if err != nil {
		mgr.Logger.Info("Get Member conn failed", "error", err)
		return true, 0
	}

	opTimestamp, err := mgr.GetOpTimestamp(ctx, db)
	if err != nil {
		mgr.Logger.Info("get op timestamp failed", "error", err)
		return true, 0
	}
	lag := leaderDBState.OpTimestamp - opTimestamp
	if lag <= cluster.HaConfig.GetMaxLagOnSwitchover() {
		return false, lag
	}
	mgr.Logger.Info(fmt.Sprintf("The member %s has lag: %d", member.Name, lag))
	return true, lag
}

func (mgr *Manager) IsMemberHealthy(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) bool {
	db, err := mgr.GetMemberConnection(cluster, member)
	if err != nil {
		mgr.Logger.Info("Get Member conn failed", "error", err)
		return false
	}

	if cluster.Leader != nil && cluster.Leader.Name == member.Name {
		if !mgr.WriteCheck(ctx, db) {
			return false
		}
	}
	if !mgr.ReadCheck(ctx, db) {
		return false
	}

	return true
}

func (mgr *Manager) GetDBState(ctx context.Context, cluster *dcs.Cluster) *dcs.DBState {
	mgr.DBState = nil

	globalState, err := mgr.GetGlobalState(ctx, mgr.DB)
	if err != nil {
		mgr.Logger.Info("select global failed", "error", err)
		return nil
	}

	masterStatus, err := mgr.GetMasterStatus(ctx, mgr.DB)
	if err != nil {
		mgr.Logger.Info("show master status failed", "error", err)
		return nil
	}

	slaveStatus, err := mgr.GetSlaveStatus(ctx, mgr.DB)
	if err != nil {
		mgr.Logger.Info("show slave status failed", "error", err)
		return nil
	}

	opTimestamp, err := mgr.GetOpTimestamp(ctx, mgr.DB)
	if err != nil {
		mgr.Logger.Info("get op timestamp failed", "error", err)
		return nil
	}

	dbState := &dcs.DBState{
		OpTimestamp: opTimestamp,
		Extra:       map[string]string{},
	}
	for k, v := range globalState {
		dbState.Extra[k] = v
	}

	if cluster.Leader != nil && cluster.Leader.Name == mgr.CurrentMemberName {
		dbState.Extra["Binlog_File"] = masterStatus.GetString("File")
		dbState.Extra["Binlog_Pos"] = masterStatus.GetString("Pos")
	} else {
		dbState.Extra["Master_Host"] = slaveStatus.GetString("Master_Host")
		dbState.Extra["Master_UUID"] = slaveStatus.GetString("Master_UUID")
		dbState.Extra["Slave_IO_Running"] = slaveStatus.GetString("Slave_IO_Running")
		dbState.Extra["Slave_SQL_Running"] = slaveStatus.GetString("Slave_SQL_Running")
		dbState.Extra["Last_IO_Error"] = slaveStatus.GetString("Last_IO_Error")
		dbState.Extra["Last_SQL_Error"] = slaveStatus.GetString("Last_SQL_Error")
		dbState.Extra["Master_Log_File"] = slaveStatus.GetString("Master_Log_File")
		dbState.Extra["Read_Master_Log_Pos"] = slaveStatus.GetString("Read_Master_Log_Pos")
		dbState.Extra["Relay_Master_Log_File"] = slaveStatus.GetString("Relay_Master_Log_File")
		dbState.Extra["Exec_Master_Log_Pos"] = slaveStatus.GetString("Exec_Master_Log_Pos")
	}

	mgr.globalState = globalState
	mgr.masterStatus = masterStatus
	mgr.slaveStatus = slaveStatus
	mgr.opTimestamp = opTimestamp
	mgr.DBState = dbState

	return dbState
}

func (mgr *Manager) GetSecondsBehindMaster(ctx context.Context) (int, error) {
	slaveStatus, err := mgr.GetSlaveStatus(ctx, mgr.DB)
	if err != nil {
		mgr.Logger.Info("show slave status failed", "error", err)
		return 0, err
	}
	if len(slaveStatus) == 0 {
		return 0, nil
	}
	secondsBehindMaster := slaveStatus.GetString("Seconds_Behind_Master")
	if secondsBehindMaster == "NULL" || secondsBehindMaster == "" {
		return 0, nil
	}
	return strconv.Atoi(secondsBehindMaster)
}

func (mgr *Manager) WriteCheck(ctx context.Context, db *sql.DB) bool {
	writeSQL := fmt.Sprintf(`BEGIN;
CREATE DATABASE IF NOT EXISTS kubeblocks;
CREATE TABLE IF NOT EXISTS kubeblocks.kb_health_check(type INT, check_ts BIGINT, PRIMARY KEY(type));
INSERT INTO kubeblocks.kb_health_check VALUES(%d, UNIX_TIMESTAMP()) ON DUPLICATE KEY UPDATE check_ts = UNIX_TIMESTAMP();
COMMIT;`, engines.CheckStatusType)
	_, err := db.ExecContext(ctx, writeSQL)
	if err != nil {
		mgr.Logger.Info(writeSQL+" executing failed", "error", err.Error())
		return false
	}
	return true
}

func (mgr *Manager) ReadCheck(ctx context.Context, db *sql.DB) bool {
	_, err := mgr.GetOpTimestamp(ctx, db)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// no healthy check records, return true
			return true
		}
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && (mysqlErr.Number == 1049 || mysqlErr.Number == 1146) {
			// error 1049: database does not exists
			// error 1146: table does not exists
			// no healthy database, return true
			return true
		}
		mgr.Logger.Info("Read check failed", "error", err)
		return false
	}

	return true
}

func (mgr *Manager) GetOpTimestamp(ctx context.Context, db *sql.DB) (int64, error) {
	readSQL := fmt.Sprintf(`select check_ts from kubeblocks.kb_health_check where type=%d limit 1;`, engines.CheckStatusType)
	var opTimestamp int64
	err := db.QueryRowContext(ctx, readSQL).Scan(&opTimestamp)
	return opTimestamp, err
}

func (mgr *Manager) GetGlobalState(ctx context.Context, db *sql.DB) (map[string]string, error) {
	var hostname, serverUUID, gtidExecuted, gtidPurged, isReadonly, superReadonly string
	err := db.QueryRowContext(ctx, "select  @@global.hostname, @@global.server_uuid, @@global.gtid_executed, @@global.gtid_purged, @@global.read_only, @@global.super_read_only").
		Scan(&hostname, &serverUUID, &gtidExecuted, &gtidPurged, &isReadonly, &superReadonly)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"hostname":        hostname,
		"server_uuid":     serverUUID,
		"gtid_executed":   gtidExecuted,
		"gtid_purged":     gtidPurged,
		"read_only":       isReadonly,
		"super_read_only": superReadonly,
	}, nil
}

func (mgr *Manager) GetSlaveStatus(context.Context, *sql.DB) (RowMap, error) {
	sql := "show slave status"
	var rowMap RowMap

	err := QueryRowsMap(mgr.DB, sql, func(rMap RowMap) error {
		rowMap = rMap
		return nil
	})
	if err != nil {
		mgr.Logger.Info("executing "+sql+" failed", "error", err.Error())
		return nil, err
	}
	return rowMap, nil
}

func (mgr *Manager) GetMasterStatus(context.Context, *sql.DB) (RowMap, error) {
	sql := "show master status"
	var rowMap RowMap

	err := QueryRowsMap(mgr.DB, sql, func(rMap RowMap) error {
		rowMap = rMap
		return nil
	})
	if err != nil {
		mgr.Logger.Info("executing "+sql+" failed", "error", err.Error())
		return nil, err
	}
	return rowMap, nil
}

func (mgr *Manager) Recover(context.Context) error {
	return nil
}

func (mgr *Manager) JoinCurrentMemberToCluster(context.Context, *dcs.Cluster) error {
	return nil
}

func (mgr *Manager) LeaveMemberFromCluster(context.Context, *dcs.Cluster, string) error {
	return nil
}

// func (mgr *Manager) IsClusterHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
// 	leaderMember := cluster.GetLeaderMember()
// 	if leaderMember == nil {
// 		mgr.Logger.Info("IsClusterHealthy: has no leader.")
// 		return true
// 	}

// 	return mgr.IsMemberHealthy(ctx, cluster, leaderMember)
// }

// IsClusterInitialized is a method to check if cluster is initialized or not
func (mgr *Manager) IsClusterInitialized(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	_, err := mgr.GetVersion(ctx)
	if err != nil {
		return false, err
	}

	if cluster != nil {
		isValid, err := mgr.ValidateAddr(ctx, cluster)
		if err != nil || !isValid {
			return isValid, err
		}
	}

	// err = mgr.EnableSemiSyncIfNeed(ctx)
	// if err != nil {
	// 	return false, err
	// }
	return mgr.EnsureServerID(ctx)
}

func (mgr *Manager) GetVersion(ctx context.Context) (string, error) {
	if mgr.version != "" {
		return mgr.version, nil
	}
	err := mgr.DB.QueryRowContext(ctx, "select version()").Scan(&mgr.version)
	if err != nil {
		return "", errors.Wrap(err, "Get version failed")
	}
	return mgr.version, nil
}

func (mgr *Manager) ValidateAddr(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	// The maximum length of the server addr is 255 characters. Before MySQL 8.0.17 it was 60 characters.
	currentMemberName := mgr.GetCurrentMemberName()
	member := cluster.GetMemberWithName(currentMemberName)
	addr := cluster.GetMemberShortAddr(*member)
	maxLength := 255
	if IsBeforeVersion(mgr.version, "8.0.17") {
		maxLength = 60
	}

	if len(addr) > maxLength {
		return false, errors.Errorf("The length of the member address must be less than or equal to %d", maxLength)
	}
	return true, nil
}

func (mgr *Manager) EnsureServerID(ctx context.Context) (bool, error) {
	var serverID uint
	err := mgr.DB.QueryRowContext(ctx, "select @@global.server_id").Scan(&serverID)
	if err != nil {
		mgr.Logger.Info("Get global server id failed", "error", err)
		return false, err
	}
	if serverID == mgr.serverID {
		return true, nil
	}
	mgr.Logger.Info("Set global server id", "server_id", serverID)

	setServerID := fmt.Sprintf(`set global server_id = %d`, mgr.serverID)
	mgr.Logger.Info("Set global server id", "server-id", setServerID)
	_, err = mgr.DB.Exec(setServerID)
	if err != nil {
		mgr.Logger.Info("set server id failed", "error", err)
		return false, err
	}

	return true, nil
}

func (mgr *Manager) EnableSemiSyncIfNeed(ctx context.Context) error {
	var status string
	err := mgr.DB.QueryRowContext(ctx, "SELECT PLUGIN_STATUS FROM INFORMATION_SCHEMA.PLUGINS "+ //nolint:goconst
		"WHERE PLUGIN_NAME ='rpl_semi_sync_source';").Scan(&status) //nolint:goconst
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		mgr.Logger.Info("Get rpl_semi_sync_source plugin status failed", "error", err.Error())
		return err
	}

	// In MySQL 8.0, semi-sync configuration options should not be specified in my.cnf,
	// as this may cause the database initialization process to fail:
	//    [Warning] [MY-013501] [Server] Ignoring --plugin-load[_add] list as the server is running with --initialize(-insecure).
	//    [ERROR] [MY-000067] [Server] unknown variable 'rpl_semi_sync_master_enabled=1'.
	if status == "ACTIVE" {
		setSourceEnable := "SET GLOBAL rpl_semi_sync_source_enabled = 1;" +
			"SET GLOBAL rpl_semi_sync_source_timeout = 100000;"
		_, err = mgr.DB.Exec(setSourceEnable)
		if err != nil {
			mgr.Logger.Info(setSourceEnable+" execute failed", "error", err.Error())
			return err
		}
	}

	err = mgr.DB.QueryRowContext(ctx, "SELECT PLUGIN_STATUS FROM INFORMATION_SCHEMA.PLUGINS "+
		"WHERE PLUGIN_NAME ='rpl_semi_sync_replica';").Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		mgr.Logger.Info("Get rpl_semi_sync_replica plugin status failed", "error", err.Error())
		return err
	}

	if status == "ACTIVE" {
		setSourceEnable := "SET GLOBAL rpl_semi_sync_replica_enabled = 1;"
		_, err = mgr.DB.Exec(setSourceEnable)
		if err != nil {
			mgr.Logger.Info(setSourceEnable+" execute failed", "error", err.Error())
			return err
		}
	}
	return nil
}

func (mgr *Manager) Promote(ctx context.Context, cluster *dcs.Cluster) error {
	err := mgr.EnableSemiSyncSource(ctx)
	if err != nil {
		return err
	}

	err = mgr.DisableSemiSyncReplica(ctx)
	if err != nil {
		return err
	}

	if (mgr.globalState["super_read_only"] == "0" && mgr.globalState["read_only"] == "0") &&
		(len(mgr.slaveStatus) == 0 || (mgr.slaveStatus.GetString("Slave_IO_Running") == "No" &&
			mgr.slaveStatus.GetString("Slave_SQL_Running") == "No")) {
		return nil
	}

	// wait relay log to play
	secondsBehindMaster, err := mgr.GetSecondsBehindMaster(ctx)
	for err != nil || secondsBehindMaster != 0 {
		time.Sleep(time.Second)
		secondsBehindMaster, err = mgr.GetSecondsBehindMaster(ctx)
	}

	stopReadOnly := `set global read_only=off;set global super_read_only=off;`
	stopSlave := `stop slave;`
	resp, err := mgr.DB.Exec(stopReadOnly + stopSlave)
	if err != nil {
		mgr.Logger.Info("promote failed", "error", err.Error())
		return err
	}

	// fresh db state
	mgr.GetDBState(ctx, cluster)
	mgr.Logger.Info(fmt.Sprintf("promote success, resp:%v", resp))
	return nil
}

func (mgr *Manager) Demote(context.Context) error {
	setReadOnly := `set global read_only=on;set global super_read_only=on;`

	_, err := mgr.DB.Exec(setReadOnly)
	if err != nil {
		mgr.Logger.Info("demote failed", "error", err.Error())
		return err
	}
	return nil
}

func (mgr *Manager) Follow(ctx context.Context, cluster *dcs.Cluster) error {
	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		return fmt.Errorf("cluster has no leader")
	}

	if mgr.CurrentMemberName == cluster.Leader.Name {
		mgr.Logger.Info("i get the leader key, don't need to follow")
		return nil
	}

	if !mgr.isRecoveryConfOutdated(cluster.Leader.Name) {
		return nil
	}
	err := mgr.EnableSemiSyncReplica(ctx)
	if err != nil {
		return err
	}
	err = mgr.DisableSemiSyncSource(ctx)
	if err != nil {
		return err
	}

	stopSlave := `stop slave;`
	// MySQL 5.7 has a limitation where the length of the master_host cannot exceed 60 characters.
	masterHost := cluster.GetMemberShortAddr(*leaderMember)
	changeMaster := fmt.Sprintf(`change master to master_host='%s',master_user='%s',master_password='%s',master_port=%s,master_auto_position=1;`,
		masterHost, config.Username, config.password, leaderMember.DBPort)
	mgr.Logger.Info("follow new leader", "changemaster", changeMaster)
	startSlave := `start slave;`

	_, err = mgr.DB.Exec(stopSlave + changeMaster + startSlave)
	if err != nil {
		mgr.Logger.Info("Follow master failed", "error", err.Error())
		return err
	}

	err = mgr.SetSemiSyncSourceTimeout(ctx, cluster, leaderMember)
	if err != nil {
		return err
	}

	// fresh db state
	mgr.GetDBState(ctx, cluster)
	mgr.Logger.Info("successfully follow new leader", "leader-name", leaderMember.Name)
	return nil
}

func (mgr *Manager) isRecoveryConfOutdated(leader string) bool {
	var rowMap = mgr.slaveStatus

	if len(rowMap) == 0 {
		return true
	}

	ioRunning := rowMap.GetString("Slave_IO_Running")
	sqlRunning := rowMap.GetString("Slave_SQL_Running")
	if ioRunning == "No" || sqlRunning == "No" {
		mgr.Logger.Info("slave status error", "status", rowMap)
		return true
	}

	masterHost := rowMap.GetString("Master_Host")
	return !strings.HasPrefix(masterHost, leader)
}

func (mgr *Manager) GetHealthiestMember(*dcs.Cluster, string) *dcs.Member {
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

func (mgr *Manager) IsRootCreated(context.Context) (bool, error) {
	return true, nil
}

func (mgr *Manager) CreateRoot(context.Context) error {
	return nil
}

func (mgr *Manager) Lock(context.Context, string) error {
	setReadOnly := `set global read_only=on;`

	_, err := mgr.DB.Exec(setReadOnly)
	if err != nil {
		mgr.Logger.Info("Lock failed", "error", err.Error())
		return err
	}
	mgr.IsLocked = true
	return nil
}

func (mgr *Manager) Unlock(context.Context) error {
	setReadOnlyOff := `set global read_only=off;`

	_, err := mgr.DB.Exec(setReadOnlyOff)
	if err != nil {
		mgr.Logger.Info("Unlock failed", "error", err.Error())
		return err
	}
	mgr.IsLocked = false
	return nil
}

func (mgr *Manager) ShutDownWithWait() {
	for _, db := range connectionPoolCache {
		_ = db.Close()
	}
	connectionPoolCache = make(map[string]*sql.DB)
}
