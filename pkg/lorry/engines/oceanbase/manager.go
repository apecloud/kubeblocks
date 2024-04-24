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

package oceanbase

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/mysql"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

const (
	Role          = "ROLE"
	CurrentLeader = "CURRENT_LEADER"
	PRIMARY       = "PRIMARY"
	STANDBY       = "STANDBY"

	repUser      = "rep_user"
	repPassword  = "rep_user"
	normalStatus = "NORMAL"
	MYSQL        = "MYSQL"
	ORACLE       = "ORACLE"
)

type Manager struct {
	mysql.Manager
	ReplicaTenant     string
	CompatibilityMode string
	Members           []dcs.Member
	MaxLag            int64
}

var _ engines.DBManager = &Manager{}

func NewManager(properties engines.Properties) (engines.DBManager, error) {
	logger := ctrl.Log.WithName("Oceanbase")
	config, err := NewConfig(properties)
	if err != nil {
		return nil, err
	}

	managerBase, err := engines.NewDBManagerBase(logger)
	if err != nil {
		return nil, err
	}

	db, err := config.GetLocalDBConn()
	if err != nil {
		return nil, errors.Wrap(err, "connect to Oceanbase failed")
	}

	mgr := &Manager{
		Manager: mysql.Manager{
			DBManagerBase: *managerBase,
			DB:            db,
		},
	}
	mgr.ReplicaTenant = viper.GetString("TENANT_NAME")
	if mgr.ReplicaTenant == "" {
		return nil, errors.New("replica tenant is not set")
	}
	return mgr, nil
}

func (mgr *Manager) IsClusterInitialized(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	time.Sleep(120 * time.Second)
	return true, nil
}

func (mgr *Manager) InitializeCluster(context.Context, *dcs.Cluster) error {
	return nil
}

func (mgr *Manager) IsLeader(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	return mgr.IsLeaderMember(ctx, cluster, nil)
}

func (mgr *Manager) IsLeaderMember(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (bool, error) {
	role, err := mgr.GetReplicaRoleForMember(ctx, cluster, member)

	if err != nil {
		return false, err
	}

	if strings.EqualFold(role, PRIMARY) {
		return true, nil
	}

	return false, nil
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

func (mgr *Manager) GetCompatibilityMode(ctx context.Context) (string, error) {
	if mgr.CompatibilityMode != "" {
		return mgr.CompatibilityMode, nil
	}
	sql := fmt.Sprintf("SELECT COMPATIBILITY_MODE FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME='%s'", mgr.ReplicaTenant)
	err := mgr.DB.QueryRowContext(ctx, sql).Scan(&mgr.CompatibilityMode)
	if err != nil {
		return "", errors.Wrap(err, "query compatibility mode failed")
	}
	return mgr.CompatibilityMode, nil
}

func (mgr *Manager) MemberHealthyCheck(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) error {
	compatibilityMode, err := mgr.GetCompatibilityMode(ctx)
	if err != nil {
		return errors.Wrap(err, "compatibility mode unknown")
	}
	switch compatibilityMode {
	case MYSQL:
		return mgr.HealthyCheckForMySQLMode(ctx, cluster, member)
	case ORACLE:
		return mgr.HealthyCheckForOracleMode(ctx, cluster, member)
	default:
		return errors.Errorf("compatibility mode not supported: [%s]", compatibilityMode)
	}
}

func (mgr *Manager) IsCurrentMemberHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	err := mgr.CurrentMemberHealthyCheck(ctx, cluster)
	if err != nil {
		mgr.Logger.Info("current member is unhealthy", "error", err.Error())
		return false
	}
	return true
}

func (mgr *Manager) CurrentMemberHealthyCheck(ctx context.Context, cluster *dcs.Cluster) error {
	member := cluster.GetMemberWithName(mgr.CurrentMemberName)
	return mgr.MemberHealthyCheck(ctx, cluster, member)
}

func (mgr *Manager) LeaderHealthyCheck(ctx context.Context, cluster *dcs.Cluster) error {
	members := cluster.Members
	for _, member := range members {
		if isLeader, _ := mgr.IsLeaderMember(ctx, cluster, &member); isLeader {
			return mgr.MemberHealthyCheck(ctx, cluster, &member)
		}
	}

	return errors.New("no leader found")
}

func (mgr *Manager) HealthyCheckForMySQLMode(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) error {
	isLeader, err := mgr.IsLeaderMember(ctx, cluster, member)
	if err != nil {
		return err
	}
	addr := cluster.GetMemberAddrWithPort(*member)
	db, err := mgr.GetMySQLDBConnWithAddr(addr)
	if err != nil {
		return err
	}
	if isLeader {
		err = mgr.WriteCheck(ctx, db)
		if err != nil {
			return err
		}
	}
	err = mgr.ReadCheck(ctx, db)
	if err != nil {
		return err
	}

	return nil
}

func (mgr *Manager) WriteCheck(ctx context.Context, db *sql.DB) error {
	writeSQL := fmt.Sprintf(`BEGIN;
CREATE DATABASE IF NOT EXISTS kubeblocks;
CREATE TABLE IF NOT EXISTS kubeblocks.kb_health_check(type INT, check_ts BIGINT, PRIMARY KEY(type));
INSERT INTO kubeblocks.kb_health_check VALUES(%d, UNIX_TIMESTAMP()) ON DUPLICATE KEY UPDATE check_ts = UNIX_TIMESTAMP();
COMMIT;`, engines.CheckStatusType)
	opTimestamp, _ := mgr.GetOpTimestamp(ctx, db)
	if opTimestamp != 0 {
		// if op timestamp is not 0, it means the table is ready created
		writeSQL = fmt.Sprintf(`
		INSERT INTO kubeblocks.kb_health_check VALUES(%d, UNIX_TIMESTAMP()) ON DUPLICATE KEY UPDATE check_ts = UNIX_TIMESTAMP();
		`, engines.CheckStatusType)
	}
	_, err := db.ExecContext(ctx, writeSQL)
	if err != nil {
		return errors.Wrap(err, "Write check failed")
	}
	return nil
}

func (mgr *Manager) HealthyCheckForOracleMode(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) error {
	// there is no golang driver for oceanbase oracle mode, so use mysql client to check
	isLeader, err := mgr.IsLeaderMember(ctx, cluster, member)
	if err != nil {
		return err
	}
	if isLeader {
		cmd := []string{"mysql", "-h", member.PodIP, "-P", member.DBPort, "-u", "SYS@" + mgr.ReplicaTenant, "-e", "SELECT t.table_name tablename FROM user_tables t WHERE table_name = 'KB_HEALTH_CHECK'"}
		output, err := util.ExecCommand(ctx, cmd, os.Environ())
		if err != nil {
			return errors.Wrap(err, "check table failed")
		}
		if !strings.Contains(output, "KB_HEALTH_CHECK") {
			sql := "create table kb_health_check (type int primary key, check_ts NUMBER);"
			sql += fmt.Sprintf("INSERT INTO kb_health_check (type, check_ts) VALUES (1, %d);", time.Now().Unix())
			sql += "commit;"
			cmd = []string{"mysql", "-h", member.PodIP, "-P", member.DBPort, "-u", "SYS@" + mgr.ReplicaTenant, "-e", sql}
			_, err = util.ExecCommand(ctx, cmd, os.Environ())
			if err != nil {
				return errors.Wrap(err, "create table failed")
			}
		}
		sql := fmt.Sprintf("UPDATE kb_health_check SET check_ts = %d WHERE type=1;", time.Now().Unix())
		sql += "commit;"
		cmd = []string{"mysql", "-h", member.PodIP, "-P", member.DBPort, "-u", "SYS@" + mgr.ReplicaTenant, "-e", sql}
		_, err = util.ExecCommand(ctx, cmd, os.Environ())
		if err != nil {
			return errors.Wrap(err, "create table failed")
		}
	}

	sql := "SELECT check_ts from kb_health_check WHERE type=1;"
	cmd := []string{"mysql", "-h", member.PodIP, "-P", member.DBPort, "-u", "SYS@" + mgr.ReplicaTenant, "-e", sql}
	_, err = util.ExecCommand(ctx, cmd, os.Environ())
	if err != nil {
		return errors.Wrap(err, "create table failed")
	}
	return nil
}

func (mgr *Manager) IsMemberHealthy(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) bool {
	err := mgr.MemberHealthyCheck(ctx, cluster, member)
	if err != nil {
		mgr.Logger.Info("member is unhealthy", "error", err.Error())
		return false
	}
	return true
}

func (mgr *Manager) IsMemberLagging(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (bool, int64) {
	var leaderOpTimestamp int64
	if cluster.Leader == nil || cluster.Leader.DBState == nil {
		mgr.Logger.Info("leader's db state is nil, maybe leader is not ready yet")
		return false, 0
	}
	leaderOpTimestamp = cluster.Leader.DBState.OpTimestamp
	if leaderOpTimestamp == 0 {
		mgr.Logger.Info("leader's op timestamp is 0")
		return true, 0
	}

	opTimestamp, err := mgr.GetMemberOpTimestamp(ctx, cluster, member)
	if err != nil {
		mgr.Logger.Info("get op timestamp failed", "error", err.Error())
		return true, 0
	}
	lag := leaderOpTimestamp - opTimestamp
	if lag > mgr.MaxLag {
		mgr.Logger.Info("member is lagging", "opTimestamp", opTimestamp, "leaderOpTimestamp", leaderOpTimestamp)
		return true, lag
	}
	return false, lag
}

func (mgr *Manager) GetDBState(ctx context.Context, cluster *dcs.Cluster) *dcs.DBState {
	mgr.DBState = nil
	member := cluster.GetMemberWithName(mgr.CurrentMemberName)
	opTimestamp, err := mgr.GetMemberOpTimestamp(ctx, cluster, member)
	if err != nil {
		mgr.Logger.Info("get op timestamp failed", "error", err)
		return nil
	}
	mgr.DBState = &dcs.DBState{
		OpTimestamp: opTimestamp,
	}
	return mgr.DBState
}

func (mgr *Manager) GetMemberOpTimestamp(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (int64, error) {
	compatibilityMode, err := mgr.GetCompatibilityMode(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "compatibility mode unknown")
	}
	if compatibilityMode == ORACLE {
		sql := "SELECT check_ts from kb_health_check WHERE type=1;"
		cmd := []string{"mysql", "-h", member.PodIP, "-P", member.DBPort, "-u", "SYS@" + mgr.ReplicaTenant, "-e", sql}
		output, err := util.ExecCommand(ctx, cmd, os.Environ())
		if err != nil {
			return 0, errors.Wrap(err, "get timestamp failed")
		}
		stimeStamp := strings.Split(output, "\n")
		if len(stimeStamp) < 2 {
			return 0, nil
		}
		return strconv.ParseInt(stimeStamp[1], 10, 64)
	}
	addr := cluster.GetMemberAddrWithPort(*member)
	db, err := mgr.GetMySQLDBConnWithAddr(addr)
	if err != nil {
		mgr.Logger.Info("get db connection failed", "error", err.Error())
		return 0, err
	}
	return mgr.GetOpTimestamp(ctx, db)
}

func (mgr *Manager) Promote(ctx context.Context, cluster *dcs.Cluster) error {
	db := mgr.DB
	isLeader, err := mgr.IsLeader(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "leader check failed")
	}
	if isLeader {
		return nil
	}
	// if there is no switchover, it's a failover: old leader is down, we need to promote a new leader, and the old leader can't be used anymore.
	primaryTenant := "ALTER SYSTEM ACTIVATE STANDBY TENANT = " + mgr.ReplicaTenant
	if cluster.Switchover != nil {
		// it's a manual switchover
		mgr.Logger.Info("manual switchover")
		primaryTenant = "ALTER SYSTEM SWITCHOVER TO PRIMARY TENANT = " + mgr.ReplicaTenant
	} else {
		mgr.Logger.Info("unexpected switchover, promote to primary directly")
	}

	_, err = db.Exec(primaryTenant)
	if err != nil {
		mgr.Logger.Info("activate standby tenant failed", "error", err)
		return err
	}

	var tenantRole, roleStatus string
	queryTenant := fmt.Sprintf("SELECT TENANT_ROLE, SWITCHOVER_STATUS FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME='%s'", mgr.ReplicaTenant)
	for {
		err := db.QueryRowContext(ctx, queryTenant).Scan(&tenantRole, &roleStatus)
		if err != nil {
			return errors.Wrap(err, "query tenant role failed")
		}

		if tenantRole == PRIMARY && roleStatus == normalStatus {
			break
		}
		time.Sleep(time.Second)
	}

	return nil
}

func (mgr *Manager) Demote(ctx context.Context) error {
	db := mgr.DB
	standbyTenant := "ALTER SYSTEM SWITCHOVER TO STANDBY TENANT = " + mgr.ReplicaTenant
	_, err := db.Exec(standbyTenant)
	if err != nil {
		return errors.Wrap(err, "standby primary tenant failed")
	}

	var tenantRole, roleStatus string
	queryTenant := fmt.Sprintf("SELECT TENANT_ROLE, SWITCHOVER_STATUS FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME='%s'", mgr.ReplicaTenant)
	for {
		err := db.QueryRowContext(ctx, queryTenant).Scan(&tenantRole, &roleStatus)
		if err != nil {
			return errors.Wrap(err, "query tenant role failed")
		}

		if tenantRole == STANDBY && roleStatus == normalStatus {
			break
		}
		time.Sleep(time.Second)
	}

	return nil
}

func (mgr *Manager) Follow(ctx context.Context, cluster *dcs.Cluster) error {
	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		return errors.New("no leader found")
	}
	sourceAddr := leaderMember.PodIP + ":" + leaderMember.DBPort
	db := mgr.DB

	sql := fmt.Sprintf("ALTER SYSTEM SET LOG_RESTORE_SOURCE = 'SERVICE=%s USER=%s@%s PASSWORD=%s' TENANT = %s",
		sourceAddr, repUser, mgr.ReplicaTenant, repPassword, mgr.ReplicaTenant)
	_, err := db.Exec(sql)
	if err != nil {
		mgr.Logger.Info(sql+" failed", "error", err) //nolint:goconst
		return err
	}

	time.Sleep(time.Second)
	var scn int64
	queryTenant := fmt.Sprintf("SELECT RECOVERY_UNTIL_SCN FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME='%s'", mgr.ReplicaTenant)
	for {
		err := db.QueryRowContext(ctx, queryTenant).Scan(&scn)
		if err != nil {
			mgr.Logger.Info("query zone info failed", "error", err)
			return err
		}

		if scn == 4611686018427387903 {
			break
		}
		time.Sleep(time.Second)
	}
	return nil
}
