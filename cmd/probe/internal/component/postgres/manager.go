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
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"

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

func (mgr *Manager) IsCurrentMemberInCluster(ctx context.Context, cluster *dcs.Cluster) bool {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.IsCurrentMemberInClusterConsensus(ctx, cluster)
	case Replication:
		return true
	default:
		mgr.Logger.Errorf("check current member in cluster failed, err:%v", InvalidWorkLoadType)
		return false
	}
}

func (mgr *Manager) IsCurrentMemberHealthy(ctx context.Context, cluster *dcs.Cluster) bool {
	return mgr.IsMemberHealthy(ctx, cluster, nil)
}

func (mgr *Manager) IsMemberHealthy(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) bool {
	switch mgr.workLoadType {
	case Consensus:
		return mgr.IsMemberHealthyConsensus(ctx, cluster, member)
	case Replication:
		return mgr.IsMemberHealthyReplication(ctx, cluster, member)
	default:
		mgr.Logger.Errorf("check current member healthy failed, err:%v", InvalidWorkLoadType)
		return false
	}
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

func (mgr *Manager) GetHealthiestMember(cluster *dcs.Cluster, candidate string) *dcs.Member {
	// TODO: check SynchronizedToLeader and compare the lags
	return nil
}

func (mgr *Manager) HasOtherHealthyLeader(ctx context.Context, cluster *dcs.Cluster) *dcs.Member {
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

func (mgr *Manager) HasOtherHealthyMembers(ctx context.Context, cluster *dcs.Cluster, leader string) []*dcs.Member {
	members := make([]*dcs.Member, 0)

	for i, m := range cluster.Members {
		if m.Name != leader && mgr.IsMemberHealthy(ctx, cluster, &m) {
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

func (mgr *Manager) Lock(ctx context.Context, reason string) error {
	mgr.Logger.Infof("Lock db: %s", reason)
	sql := "alter system set default_transaction_read_only=on;"

	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		return err
	}

	return mgr.pgReload(ctx)
}

func (mgr *Manager) Unlock(ctx context.Context) error {
	mgr.Logger.Infof("UnLock db")
	sql := "alter system set default_transaction_read_only=off;"

	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		return err
	}

	return mgr.pgReload(ctx)
}

func (mgr *Manager) pgReload(ctx context.Context) error {
	reload := "select pg_reload_conf();"

	_, err := mgr.Exec(ctx, reload)

	return err
}
