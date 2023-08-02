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

package postgres

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unicode"

	"github.com/dapr/kit/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/spf13/viper"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
)

type Manager struct {
	component.DBManagerBase
	Pool         *pgxpool.Pool
	Proc         *process.Process
	workLoadType string
}

var Mgr *Manager

func NewManager(logger logger.Logger) (*Manager, error) {
	pool, err := pgxpool.NewWithConfig(context.Background(), config.pool)
	if err != nil {
		return nil, errors.Errorf("unable to ping the DB: %v", err)
	}

	Mgr = &Manager{
		DBManagerBase: component.DBManagerBase{
			CurrentMemberName: viper.GetString("KB_POD_NAME"),
			ClusterCompName:   viper.GetString("KB_CLUSTER_COMP_NAME"),
			Namespace:         viper.GetString("KB_NAMESPACE"),
			Logger:            logger,
			DataDir:           viper.GetString("PGDATA"),
		},
		Pool:         pool,
		workLoadType: viper.GetString("KB_WORKLOAD_TYPE"),
	}

	component.RegisterManager("postgresql", Mgr)
	return Mgr, nil
}

func (mgr *Manager) newProcessFromPidFile() error {
	pidFile, err := readPidFile(mgr.DataDir)
	if err != nil {
		mgr.Logger.Errorf("read pid file failed, err:%v", err)
		return errors.Wrap(err, "read pid file")
	}

	proc, err := process.NewProcess(pidFile.pid)
	if err != nil {
		mgr.Logger.Errorf("new process failed, err:%v", err)
		return err
	}

	mgr.Proc = proc
	return nil
}

func (mgr *Manager) Query(ctx context.Context, sql string) (result []byte, err error) {
	return mgr.QueryWithPool(ctx, sql, nil)
}

func (mgr *Manager) QueryWithPool(ctx context.Context, sql string, pool *pgxpool.Pool) (result []byte, err error) {
	mgr.Logger.Debugf("query: %s", sql)

	var rows pgx.Rows
	if pool != nil {
		rows, err = pool.Query(ctx, sql)
		defer pool.Close()
	} else {
		rows, err = mgr.Pool.Query(ctx, sql)
	}
	if err != nil {
		return nil, fmt.Errorf("error executing query: %w", err)
	}
	defer func() {
		rows.Close()
		_ = rows.Err()
	}()

	rs := make([]interface{}, 0)
	columnTypes := rows.FieldDescriptions()
	for rows.Next() {
		values := make([]interface{}, len(columnTypes))
		for i := range values {
			values[i] = new(interface{})
		}

		if err = rows.Scan(values...); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		r := map[string]interface{}{}
		for i, ct := range columnTypes {
			r[ct.Name] = values[i]
		}
		rs = append(rs, r)
	}

	if result, err = json.Marshal(rs); err != nil {
		err = fmt.Errorf("error serializing results: %w", err)
	}
	return result, err
}

func (mgr *Manager) Exec(ctx context.Context, sql string) (result int64, err error) {
	return mgr.ExecWithPool(ctx, sql, nil)
}

func (mgr *Manager) ExecWithPool(ctx context.Context, sql string, pool *pgxpool.Pool) (result int64, err error) {
	mgr.Logger.Debugf("exec: %s", sql)

	var res pgconn.CommandTag
	if pool != nil {
		res, err = pool.Exec(ctx, sql)
		defer pool.Close()
	} else {
		res, err = mgr.Pool.Exec(ctx, sql)
	}
	if err != nil {
		return 0, fmt.Errorf("error executing query: %w", err)
	}

	result = res.RowsAffected()

	return
}

func (mgr *Manager) QueryOthers(sql string, member *dcs.Member) {

}

func (mgr *Manager) ExecOthers(sql string, member *dcs.Member) {

}

func (mgr *Manager) IsPgReady(host string) bool {
	cmd := exec.Command("pg_isready")
	cmd.Args = append(cmd.Args, "-h", host)

	if config.username != "" {
		cmd.Args = append(cmd.Args, "-U", config.username)
	}
	if config.port != 0 {
		cmd.Args = append(cmd.Args, "-p", strconv.FormatUint(uint64(config.port), 10))
	}
	err := cmd.Run()
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

	if !mgr.IsPgReady(config.host) {
		return false
	}

	if mgr.workLoadType == Consensus && !mgr.IsConsensusReadyUp() {
		return false
	}

	mgr.DBStartupReady = true
	mgr.Logger.Infof("DB startup ready")
	return true
}

func (mgr *Manager) GetMemberStateWithPool(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.GetMemberStateWithPoolConsensus(ctx, pool)
	case Replication:
		return mgr.GetMemberStateWithPoolReplication(ctx, pool)
	default:
		return "", InvalidWorkLoadType
	}
}

func (mgr *Manager) IsLeader(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	return mgr.IsLeaderWithPool(ctx, nil)
}

func (mgr *Manager) IsLeaderWithPool(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	role, err := mgr.GetMemberStateWithPool(ctx, pool)
	if err != nil {
		return false, errors.Wrap(err, "check is leader")
	}

	return role == binding.PRIMARY || role == binding.LEADER, nil
}

func (mgr *Manager) GetMemberAddrs(cluster *dcs.Cluster) []string {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.GetMemberAddrsConsensus(cluster)
	case Replication:
		return cluster.GetMemberAddrs()
	default:
		mgr.Logger.Errorf("get member addrs failed, err:%v", InvalidWorkLoadType)
		return nil
	}
}

func (mgr *Manager) InitializeCluster(ctx context.Context, cluster *dcs.Cluster) error {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.InitializeClusterConsensus(ctx, cluster)
	case Replication:
		return nil
	default:
		return InvalidWorkLoadType
	}
}

func (mgr *Manager) IsRunning() bool {
	if mgr.Proc != nil {
		if isRunning, err := mgr.Proc.IsRunning(); isRunning && err == nil {
			return true
		}
		mgr.Proc = nil
	}

	return mgr.newProcessFromPidFile() == nil
}

func (mgr *Manager) IsCurrentMemberInCluster(cluster *dcs.Cluster) bool {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.IsCurrentMemberInClusterConsensus(cluster)
	case Replication:
		return true
	default:
		mgr.Logger.Errorf("check current member in cluster failed, err:%v", InvalidWorkLoadType)
		return false
	}
}

func (mgr *Manager) IsCurrentMemberHealthy(cluster *dcs.Cluster) bool {
	return mgr.IsMemberHealthy(cluster, nil)
}

func (mgr *Manager) IsMemberHealthy(cluster *dcs.Cluster, member *dcs.Member) bool {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.IsMemberHealthyConsensus(cluster, member)
	case Replication:
		return mgr.IsMemberHealthyReplication(cluster, member)
	default:
		mgr.Logger.Errorf("check current member healthy failed, err:%v", InvalidWorkLoadType)
		return false
	}
}

func (mgr *Manager) getWalPositionWithPool(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	var (
		lsn      int64
		isLeader bool
		err      error
	)

	if isLeader, err = mgr.IsLeaderWithPool(ctx, pool); isLeader && err == nil {
		lsn, err = mgr.getLsnWithPool(ctx, "current", pool)
		if err != nil {
			return 0, err
		}
	} else {
		replayLsn, errReplay := mgr.getLsnWithPool(ctx, "replay", pool)
		receiveLsn, errReceive := mgr.getLsnWithPool(ctx, "receive", pool)
		if errReplay != nil && errReceive != nil {
			return 0, errors.Errorf("get replayLsn or receiveLsn failed, replayLsn err:%v, receiveLsn err:%v", errReplay, errReceive)
		}
		lsn = component.MaxInt64(replayLsn, receiveLsn)
	}

	return lsn, nil
}

func (mgr *Manager) getLsnWithPool(ctx context.Context, types string, pool *pgxpool.Pool) (int64, error) {
	var sql string
	switch types {
	case "current":
		sql = "select pg_catalog.pg_wal_lsn_diff(pg_catalog.pg_current_wal_lsn(), '0/0')::bigint;"
	case "replay":
		sql = "select pg_catalog.pg_wal_lsn_diff(pg_catalog.pg_last_wal_replay_lsn(), '0/0')::bigint;"
	case "receive":
		sql = "select pg_catalog.pg_wal_lsn_diff(COALESCE(pg_catalog.pg_last_wal_receive_lsn(), '0/0'), '0/0')::bigint;"
	}

	resp, err := mgr.QueryWithPool(ctx, sql, pool)
	if err != nil {
		mgr.Logger.Errorf("get wal position failed, err:%v", err)
		return 0, err
	}
	lsnStr := strings.TrimFunc(string(resp), func(r rune) bool {
		return !unicode.IsDigit(r)
	})

	lsn, err := strconv.ParseInt(lsnStr, 10, 64)
	if err != nil {
		mgr.Logger.Errorf("convert lsnStr to lsn failed, err:%v", err)
	}

	return lsn, nil
}

func (mgr *Manager) isLagging(walPosition int64, cluster *dcs.Cluster) bool {
	lag := cluster.GetOpTime() - walPosition
	return lag > cluster.HaConfig.GetMaxLagOnSwitchover()
}

func (mgr *Manager) Recover() {}

func (mgr *Manager) AddCurrentMemberToCluster(cluster *dcs.Cluster) error {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.AddCurrentMemberToClusterConsensus(cluster)
	case Replication:
		// replication postgresql don't need to add member
		return nil
	default:
		return InvalidWorkLoadType
	}
}

// DeleteMemberFromCluster postgresql don't need to delete member
func (mgr *Manager) DeleteMemberFromCluster(cluster *dcs.Cluster, host string) error {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.DeleteMemberFromClusterConsensus(cluster, host)
	case Replication:
		// replication postgresql don't need to add member
		return nil
	default:
		return InvalidWorkLoadType
	}
}

func (mgr *Manager) IsClusterHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.IsClusterHealthyConsensus(ctx, cluster)
	case Replication:
		// replication postgresql don't need to check cluster
		return true
	default:
		mgr.Logger.Errorf("check cluster healthy failed, err:%v", InvalidWorkLoadType)
		return false
	}
}

func (mgr *Manager) IsClusterInitialized(ctx context.Context, cluster *dcs.Cluster) (bool, error) {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.IsClusterInitializedConsensus(ctx, cluster)
	case Replication:
		// for replication, the setup script imposes a constraint where the successful startup of the primary database (db0)
		// is a prerequisite for the successful launch of the remaining databases.
		return mgr.IsDBStartupReady(), nil
	default:
		return false, InvalidWorkLoadType
	}
}

func (mgr *Manager) Promote() error {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.PromoteConsensus()
	case Replication:
		return mgr.PromoteReplication()
	default:
		return InvalidWorkLoadType
	}
}

func (mgr *Manager) prePromote() error {
	return nil
}

func (mgr *Manager) postPromote() error {
	return nil
}

func (mgr *Manager) Demote() error {
	mgr.Logger.Infof("current member demoting: %s", mgr.CurrentMemberName)

	switch mgr.workLoadType {
	case Consensus:
		return mgr.DemoteConsensus()
	case Replication:
		return mgr.DemoteReplication()
	default:
		return InvalidWorkLoadType
	}
}

func (mgr *Manager) Stop() error {
	mgr.Logger.Infof("wait for send signal 1 to deactivate sql channel")
	sqlChannelProc, err := component.GetSQLChannelProc()
	if err != nil {
		mgr.Logger.Errorf("can't find sql channel process, err:%v", err)
		return errors.Errorf("can't find sql channel process, err:%v", err)
	}

	// deactivate sql channel restart db
	err = sqlChannelProc.Signal(syscall.SIGUSR1)
	if err != nil {
		return errors.Errorf("send signal1 to sql channel failed, err:%v", err)
	}

	var stdout, stderr bytes.Buffer
	stopCmd := exec.Command("su", "-c", "pg_ctl stop -m fast", "postgres")
	stopCmd.Stdout = &stdout
	stopCmd.Stderr = &stderr

	err = stopCmd.Run()
	if err != nil || stderr.String() != "" {
		mgr.Logger.Errorf("stop failed, err:%v, stderr:%s", err, stderr.String())
		return err
	}

	return nil
}

func (mgr *Manager) Follow(cluster *dcs.Cluster) error {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.FollowConsensus(cluster)
	case Replication:
		return mgr.FollowReplication(cluster)
	default:
		return InvalidWorkLoadType
	}
}

func (mgr *Manager) follow(needRestart bool, cluster *dcs.Cluster) error {
	leaderMember := cluster.GetLeaderMember()
	if mgr.CurrentMemberName == leaderMember.Name {
		mgr.Logger.Infof("i get the leader key, don't need to follow")
		return nil
	}

	primaryInfo := fmt.Sprintf("\nprimary_conninfo = 'host=%s port=%s user=%s password=%s application_name=my-application'",
		cluster.GetMemberAddr(*leaderMember), leaderMember.DBPort, config.username, viper.GetString("POSTGRES_PASSWORD"))

	pgConf, err := os.OpenFile("/kubeblocks/conf/postgresql.conf", os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		mgr.Logger.Errorf("open postgresql.conf failed, err:%v", err)
		return err
	}
	defer pgConf.Close()

	writer := bufio.NewWriter(pgConf)
	_, err = writer.WriteString(primaryInfo)
	if err != nil {
		mgr.Logger.Errorf("write into postgresql.conf failed, err:%v", err)
		return err
	}

	err = writer.Flush()
	if err != nil {
		mgr.Logger.Errorf("writer flush failed, err:%v", err)
		return err
	}

	if !needRestart {
		var stdout, stderr bytes.Buffer
		cmd := exec.Command("su", "-c", "pg_ctl reload", "postgres")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		if err != nil || stderr.String() != "" {
			mgr.Logger.Errorf("postgresql reload failed, err:%v, stderr:%s", err, stderr.String())
			return err
		}

		mgr.Logger.Infof("successfully follow new leader:%s", leaderMember.Name)
		return nil
	}

	return mgr.Start()
}

func (mgr *Manager) Start() error {
	mgr.Logger.Infof("wait for send signal 2 to activate sql channel")
	sqlChannelProc, err := component.GetSQLChannelProc()
	if err != nil {
		mgr.Logger.Errorf("can't find sql channel process, err:%v", err)
		return errors.Errorf("can't find sql channel process, err:%v", err)
	}

	// activate sql channel restart db
	err = sqlChannelProc.Signal(syscall.SIGUSR2)
	if err != nil {
		return errors.Errorf("send signal2 to sql channel failed, err:%v", err)
	}
	return nil
}

func (mgr *Manager) GetHealthiestMember(cluster *dcs.Cluster, candidate string) *dcs.Member {
	// TODO: check SynchronizedToLeader and compare the lags
	return nil
}

func (mgr *Manager) HasOtherHealthyLeader(cluster *dcs.Cluster) *dcs.Member {
	ctx := context.TODO()

	isLeader, err := mgr.IsLeader(ctx, cluster)
	if err == nil && isLeader {
		// if current member is leader, just return
		return nil
	}

	var hosts []string
	for _, m := range cluster.Members {
		if m.Name != mgr.CurrentMemberName {
			hosts = append(hosts, cluster.GetMemberAddr(m))
		}
	}
	pools, err := mgr.GetOtherPoolsWithHosts(ctx, hosts)
	if err != nil {
		mgr.Logger.Errorf("Get other pools failed, err:%v", err)
		return nil
	}

	for i, pool := range pools {
		if pool != nil {
			if isLeader, err = mgr.IsLeaderWithPool(ctx, pool); isLeader && err == nil {
				return cluster.GetMemberWithHost(hosts[i])
			}
		}
	}

	return nil
}

func (mgr *Manager) HasOtherHealthyMembers(cluster *dcs.Cluster, leader string) []*dcs.Member {
	members := make([]*dcs.Member, 0)

	for i, m := range cluster.Members {
		if m.Name != leader && mgr.IsMemberHealthy(cluster, &m) {
			members = append(members, &cluster.Members[i])
		}
	}

	return members
}

func (mgr *Manager) GetOtherPoolsWithHosts(ctx context.Context, hosts []string) ([]*pgxpool.Pool, error) {
	if len(hosts) == 0 {
		return nil, errors.New("Get other pool without hosts")
	}

	resp := make([]*pgxpool.Pool, len(hosts))
	for i, host := range hosts {
		tempConfig, err := pgxpool.ParseConfig(config.GetConnectURLWithHost(host))
		if err != nil {
			return nil, errors.Wrap(err, "new temp config")
		}

		resp[i], err = pgxpool.NewWithConfig(ctx, tempConfig)
		if err != nil {
			mgr.Logger.Errorf("unable to ping the DB: %v, host:%s", err, host)
			continue
		}
	}

	return resp, nil
}

func (mgr *Manager) IsLeaderMember(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (bool, error) {
	pools, err := mgr.GetOtherPoolsWithHosts(ctx, []string{cluster.GetMemberAddr(*member)})
	if err != nil || pools[0] == nil {
		mgr.Logger.Errorf("Get leader pools failed, err:%v", err)
		return false, err
	}

	return mgr.IsLeaderWithPool(ctx, pools[0])
}

func (mgr *Manager) IsRootCreated(ctx context.Context) (bool, error) {
	return true, nil
}

func (mgr *Manager) CreateRoot(ctx context.Context) error {
	return nil
}
