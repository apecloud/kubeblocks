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

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/process"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/component"
)

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

func (mgr *Manager) readCheck(ctx context.Context, host string) bool {
	readSQL := fmt.Sprintf(`select check_ts from kb_health_check where type=%d limit 1;`, component.CheckStatusType)
	_, err := mgr.QueryWithHost(ctx, readSQL, host)
	if err != nil {
		mgr.Logger.Errorf("read check failed, err:%v", err)
		return false
	}
	return true
}

func (mgr *Manager) writeCheck(ctx context.Context, host string) bool {
	writeSQL := fmt.Sprintf(`
		create table if not exists kb_health_check(type int, check_ts timestamp, primary key(type));
		insert into kb_health_check values(%d, CURRENT_TIMESTAMP) on conflict(type) do update set check_ts = CURRENT_TIMESTAMP;
		`, component.CheckStatusType)
	_, err := mgr.ExecWithHost(ctx, writeSQL, host)
	if err != nil {
		mgr.Logger.Errorf("write check failed, err:%v", err)
		return false
	}
	return true
}

func (mgr *Manager) pgReload(ctx context.Context) error {
	reload := "select pg_reload_conf();"

	_, err := mgr.Exec(ctx, reload)

	return err
}

func (mgr *Manager) IsPgReady(ctx context.Context) bool {
	err := mgr.Pool.Ping(ctx)
	if err != nil {
		mgr.Logger.Warnf("DB is not ready, ping failed, err:%v", err)
		return false
	}

	return true
}

func (mgr *Manager) Lock(ctx context.Context, reason string) error {
	sql := "alter system set default_transaction_read_only=on;"

	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		mgr.Logger.Errorf("exec sql:%s failed, err:%v", sql, err)
		return err
	}

	if err = mgr.pgReload(ctx); err != nil {
		mgr.Logger.Errorf("reload conf failed, err:%v", err)
		return err
	}

	mgr.Logger.Infof("Lock db success: %s", reason)
	return nil
}

func (mgr *Manager) Unlock(ctx context.Context) error {
	sql := "alter system set default_transaction_read_only=off;"

	_, err := mgr.Exec(ctx, sql)
	if err != nil {
		mgr.Logger.Errorf("exec sql:%s failed, err:%v", sql, err)
		return err
	}

	if err = mgr.pgReload(ctx); err != nil {
		mgr.Logger.Errorf("reload conf failed, err:%v", err)
		return err
	}

	mgr.Logger.Infof("UnLock db success")
	return nil
}

func (mgr *Manager) ShutDownWithWait() {
	mgr.Pool.Close()
}
