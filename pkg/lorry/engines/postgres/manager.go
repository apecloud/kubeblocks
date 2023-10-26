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
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

type Manager struct {
	engines.DBManagerBase
	MajorVersion int
	Pool         PgxPoolIFace
	Proc         *process.Process
	Config       *Config
	isLeader     int
}

func NewManager(properties map[string]string) (engines.DBManager, error) {
	logger := ctrl.Log.WithName("PostgreSQL")
	config, err := NewConfig(properties)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config.pgxConfig)
	if err != nil {
		return nil, errors.Errorf("unable to ping the DB: %v", err)
	}

	managerBase, err := engines.NewDBManagerBase(logger)
	if err != nil {
		return nil, err
	}
	managerBase.DataDir = viper.GetString(PGDATA)

	mgr := &Manager{
		DBManagerBase: *managerBase,
		Pool:          pool,
		Config:        config,
		MajorVersion:  viper.GetInt(PGMAJOR),
	}

	return mgr, nil
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

func (mgr *Manager) newProcessFromPidFile() error {
	pidFile, err := readPidFile(mgr.DataDir)
	if err != nil {
		mgr.Logger.Error(err, "read pid file failed, err")
		return err
	}

	proc, err := process.NewProcess(pidFile.pid)
	if err != nil {
		mgr.Logger.Error(err, "new process failed, err")
		return err
	}

	mgr.Proc = proc
	return nil
}

func (mgr *Manager) Recover(context.Context) error {
	return nil
}

func (mgr *Manager) GetHealthiestMember(*dcs.Cluster, string) *dcs.Member {
	return nil
}

func (mgr *Manager) SetIsLeader(isLeader bool) {
	if isLeader {
		mgr.isLeader = 1
	} else {
		mgr.isLeader = -1
	}
}

func (mgr *Manager) UnsetIsLeader() {
	mgr.isLeader = 0
}

// GetIsLeader returns whether the "isLeader" is set or not and whether current member is leader or not
func (mgr *Manager) GetIsLeader() (bool, bool) {
	return mgr.isLeader != 0, mgr.isLeader == 1
}

func (mgr *Manager) IsLeaderMember(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) (bool, error) {
	if member == nil {
		return false, errors.Errorf("member is nil, can't check is leader member or not")
	}

	leaderMember := cluster.GetLeaderMember()
	if leaderMember == nil {
		return false, errors.Errorf("leader member is nil, can't check is leader member or not")
	}

	if leaderMember.Name != member.Name {
		return false, nil
	}

	return true, nil
}

func (mgr *Manager) ReadCheck(ctx context.Context, host string) bool {
	readSQL := fmt.Sprintf(`select check_ts from kb_health_check where type=%d limit 1;`, engines.CheckStatusType)
	_, err := mgr.QueryWithHost(ctx, readSQL, host)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			// no healthy check records, return true
			return true
		}
		mgr.Logger.Error(err, "read check failed")
		return false
	}
	return true
}

func (mgr *Manager) WriteCheck(ctx context.Context, host string) bool {
	writeSQL := fmt.Sprintf(`
		create table if not exists kb_health_check(type int, check_ts timestamp, primary key(type));
		insert into kb_health_check values(%d, CURRENT_TIMESTAMP) on conflict(type) do update set check_ts = CURRENT_TIMESTAMP;
		`, engines.CheckStatusType)
	_, err := mgr.ExecWithHost(ctx, writeSQL, host)
	if err != nil {
		mgr.Logger.Error(err, "write check failed")
		return false
	}
	return true
}

func (mgr *Manager) PgReload(ctx context.Context) error {
	reload := "select pg_reload_conf();"

	_, err := mgr.Exec(ctx, reload)

	return err
}

func (mgr *Manager) IsPgReady(ctx context.Context) bool {
	err := mgr.Pool.Ping(ctx)
	if err != nil {
		mgr.Logger.Error(err, "DB is not ready, ping failed")
		return false
	}

	return true
}

func (mgr *Manager) Lock(ctx context.Context, reason string) error {
	sql := "alter system set default_transaction_read_only=on;"

	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, fmt.Sprintf("exec sql:%s failed", sql))
		return err
	}

	if err = mgr.PgReload(ctx); err != nil {
		mgr.Logger.Error(err, "reload conf failed")
		return err
	}

	mgr.Logger.Info(fmt.Sprintf("Lock db success: %s", reason))
	return nil
}

func (mgr *Manager) Unlock(ctx context.Context) error {
	sql := "alter system set default_transaction_read_only=off;"

	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		mgr.Logger.Error(err, fmt.Sprintf("exec sql:%s failed", sql))
		return err
	}

	if err = mgr.PgReload(ctx); err != nil {
		mgr.Logger.Error(err, "reload conf failed")
		return err
	}

	mgr.Logger.Info("UnLock db success")
	return nil
}

func (mgr *Manager) ShutDownWithWait() {
	mgr.Pool.Close()
}
