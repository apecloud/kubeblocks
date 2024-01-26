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
	"fmt"

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
)

var semiSyncMaxTimeout int = 4294967295
var semiSyncSourceVersion string = "8.0.26"

func (mgr *Manager) GetSemiSyncSourcePlugin() string {
	plugin := "rpl_semi_sync_source"
	if IsBeforeVersion(mgr.version, semiSyncSourceVersion) {
		plugin = "rpl_semi_sync_master"
	}
	return plugin
}

func (mgr *Manager) GetSemiSyncReplicaPlugin() string {
	plugin := "rpl_semi_sync_replica"
	if IsBeforeVersion(mgr.version, semiSyncSourceVersion) {
		plugin = "rpl_semi_sync_slave"
	}
	return plugin
}

func (mgr *Manager) EnableSemiSyncSource(ctx context.Context) error {
	plugin := mgr.GetSemiSyncSourcePlugin()
	var status string
	sql := fmt.Sprintf("SELECT PLUGIN_STATUS FROM INFORMATION_SCHEMA.PLUGINS WHERE PLUGIN_NAME ='%s';", plugin)
	err := mgr.DB.QueryRowContext(ctx, sql).Scan(&status)
	if err != nil {
		errors.Wrapf(err, "Get %s plugin status failed", plugin)
	}

	// In MySQL 8.0, semi-sync configuration options should not be specified in my.cnf,
	// as this may cause the database initialization process to fail:
	//    [Warning] [MY-013501] [Server] Ignoring --plugin-load[_add] list as the server is running with --initialize(-insecure).
	//    [ERROR] [MY-000067] [Server] unknown variable 'rpl_semi_sync_master_enabled=1'.
	if status != "ACTIVE" {
		return errors.Errorf("plugin %s is not active: %s", plugin, status)
	}

	isSemiSyncSourceEnabled, err := mgr.IsSemiSyncSourceEnabled(ctx)
	if err != nil {
		return err
	}
	if isSemiSyncSourceEnabled {
		return nil
	}
	setSourceEnable := fmt.Sprintf("SET GLOBAL %s_enabled = 1;", plugin)
	setSourceTimeout := fmt.Sprintf("SET GLOBAL %s_timeout = 10000;", plugin)
	_, err = mgr.DB.Exec(setSourceEnable + setSourceTimeout)
	if err != nil {
		return errors.Wrap(err, setSourceEnable+setSourceTimeout+" execute failed")
	}
	return nil
}

func (mgr *Manager) DisableSemiSyncSource(ctx context.Context) error {
	isSemiSyncSourceEnabled, err := mgr.IsSemiSyncSourceEnabled(ctx)
	if err != nil {
		return err
	}
	if !isSemiSyncSourceEnabled {
		return nil
	}
	plugin := mgr.GetSemiSyncSourcePlugin()
	setSourceDisable := fmt.Sprintf("SET GLOBAL %s_enabled = 0;", plugin)
	_, err = mgr.DB.Exec(setSourceDisable)
	if err != nil {
		return errors.Wrap(err, setSourceDisable+" execute failed")
	}
	return nil
}

func (mgr *Manager) IsSemiSyncSourceEnabled(ctx context.Context) (bool, error) {
	plugin := mgr.GetSemiSyncSourcePlugin()
	var value int
	sql := fmt.Sprintf("select @@global.%s_enabled", plugin)
	err := mgr.DB.QueryRowContext(ctx, sql).Scan(&value)
	if err != nil {
		return false, errors.Wrapf(err, "exec %s failed", sql)
	}
	return value == 1, nil
}

func (mgr *Manager) GetSemiSyncSourceTimeout(ctx context.Context) (int, error) {
	plugin := mgr.GetSemiSyncSourcePlugin()
	var value int
	sql := fmt.Sprintf("select @@global.%s_timeout", plugin)
	err := mgr.DB.QueryRowContext(ctx, sql).Scan(&value)
	if err != nil {
		return 0, errors.Wrapf(err, "exec %s failed", sql)
	}
	return value, nil
}

func (mgr *Manager) SetSemiSyncSourceTimeout(ctx context.Context, cluster *dcs.Cluster, leader *dcs.Member) error {
	db, err := mgr.GetMemberConnection(cluster, leader)
	if err != nil {
		mgr.Logger.Info("Get Member conn failed", "error", err.Error())
		return err
	}

	plugin := mgr.GetSemiSyncSourcePlugin()
	setSourceTimeout := fmt.Sprintf("SET GLOBAL %s_timeout = %d;", plugin, semiSyncMaxTimeout)
	_, err = db.Exec(setSourceTimeout)
	if err != nil {
		return errors.Wrap(err, setSourceTimeout+" execute failed")
	}
	return nil
}

func (mgr *Manager) EnableSemiSyncReplica(ctx context.Context) error {
	plugin := mgr.GetSemiSyncReplicaPlugin()
	var status string
	sql := fmt.Sprintf("SELECT PLUGIN_STATUS FROM INFORMATION_SCHEMA.PLUGINS WHERE PLUGIN_NAME ='%s';", plugin)
	err := mgr.DB.QueryRowContext(ctx, sql).Scan(&status)
	if err != nil {
		return errors.Wrap(err, "get "+plugin+" status failed")
	}
	if status == "ACTIVE" {
		return errors.Errorf("plugin %s is not active: %s", plugin, status)
	}

	isSemiSyncReplicaEnabled, err := mgr.IsSemiSyncReplicaEnabled(ctx)
	if err != nil {
		return err
	}
	if isSemiSyncReplicaEnabled {
		return nil
	}

	setReplicaEnable := fmt.Sprintf("SET GLOBAL %s_enabled = 1;", plugin)
	_, err = mgr.DB.Exec(setReplicaEnable)
	if err != nil {
		return errors.Wrap(err, setReplicaEnable+" execute failed")
	}
	return nil
}

func (mgr *Manager) IsSemiSyncReplicaEnabled(ctx context.Context) (bool, error) {
	plugin := mgr.GetSemiSyncReplicaPlugin()
	var value int
	sql := fmt.Sprintf("select @@global.%s_enabled", plugin)
	err := mgr.DB.QueryRowContext(ctx, sql).Scan(&value)
	if err != nil {
		return false, errors.Wrapf(err, "exec %s failed", sql)
	}
	return value == 1, nil
}

func (mgr *Manager) DisableSemiSyncReplica(ctx context.Context) error {
	isSemiSyncReplicaEnabled, err := mgr.IsSemiSyncReplicaEnabled(ctx)
	if err != nil {
		return err
	}
	if !isSemiSyncReplicaEnabled {
		return nil
	}
	plugin := mgr.GetSemiSyncReplicaPlugin()
	setReplicaDisable := fmt.Sprintf("SET GLOBAL %s_enabled = 0;", plugin)
	_, err = mgr.DB.Exec(setReplicaDisable)
	if err != nil {
		return errors.Wrap(err, setReplicaDisable+" execute failed")
	}
	return nil
}
