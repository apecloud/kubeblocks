/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

# This file is part of KubeBlocks project

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
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestManager_SemiSync(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)
	_, _ = NewConfig(fakeProperties)

	t.Run("semi sync plugin", func(t *testing.T) {
		t.Run("version before 8.0.26", func(t *testing.T) {
			manager.version = "5.7.42"
			semiSyncSourcePlugin := manager.GetSemiSyncSourcePlugin()
			semiSyncReplicaPlugin := manager.GetSemiSyncReplicaPlugin()
			assert.Equal(t, semiSyncSourcePlugin, "rpl_semi_sync_master")
			assert.Equal(t, semiSyncReplicaPlugin, "rpl_semi_sync_slave")
		})

		t.Run("version after 8.0.26", func(t *testing.T) {
			manager.version = "8.0.30"
			semiSyncSourcePlugin := manager.GetSemiSyncSourcePlugin()
			semiSyncReplicaPlugin := manager.GetSemiSyncReplicaPlugin()
			assert.Equal(t, semiSyncSourcePlugin, "rpl_semi_sync_source")
			assert.Equal(t, semiSyncReplicaPlugin, "rpl_semi_sync_replica")
		})
	})

	t.Run("enable semi sync source", func(t *testing.T) {
		t.Run("failed if plugin not load", func(t *testing.T) {
			mock.ExpectQuery("SELECT PLUGIN_STATUS FROM INFORMATION_SCHEMA.PLUGINS " +
				"WHERE PLUGIN_NAME ='rpl_semi_sync_source';").WillReturnRows(sqlmock.NewRows([]string{"PLUGIN_STATUS"}))
			err := manager.EnableSemiSyncSource(ctx)
			assert.NotNil(t, err)
			assert.ErrorContains(t, err, "plugin status failed")
		})
		t.Run("failed if plugin not active", func(t *testing.T) {
			mock.ExpectQuery("SELECT PLUGIN_STATUS FROM INFORMATION_SCHEMA.PLUGINS " +
				"WHERE PLUGIN_NAME ='rpl_semi_sync_source';").WillReturnRows(sqlmock.NewRows([]string{"PLUGIN_STATUS"}).AddRow("NotActive"))
			err := manager.EnableSemiSyncSource(ctx)
			assert.NotNil(t, err)
			assert.ErrorContains(t, err, "is not active")
		})

		t.Run("already enabled", func(t *testing.T) {
			mock.ExpectQuery("SELECT PLUGIN_STATUS FROM INFORMATION_SCHEMA.PLUGINS " +
				"WHERE PLUGIN_NAME ='rpl_semi_sync_source';").WillReturnRows(sqlmock.NewRows([]string{"PLUGIN_STATUS"}).AddRow("ACTIVE"))
			mock.ExpectQuery("select @@global.rpl_semi_sync_source_enabled").WillReturnRows(sqlmock.NewRows([]string{"STATUS"}).AddRow(1))
			err := manager.EnableSemiSyncSource(ctx)
			assert.Nil(t, err)
		})

		t.Run("enable", func(t *testing.T) {
			mock.ExpectQuery("SELECT PLUGIN_STATUS FROM INFORMATION_SCHEMA.PLUGINS " +
				"WHERE PLUGIN_NAME ='rpl_semi_sync_source';").WillReturnRows(sqlmock.NewRows([]string{"PLUGIN_STATUS"}).AddRow("ACTIVE"))
			mock.ExpectQuery("select @@global.rpl_semi_sync_source_enabled").WillReturnRows(sqlmock.NewRows([]string{"STATUS"}).AddRow(0))
			mock.ExpectExec("SET GLOBAL rpl_semi_sync_source_enabled = 1;" +
				"SET GLOBAL rpl_semi_sync_source_timeout = 0;").
				WillReturnResult(sqlmock.NewResult(1, 1))
			err := manager.EnableSemiSyncSource(ctx)
			assert.Nil(t, err)
		})
	})

	t.Run("disable semi sync source", func(t *testing.T) {
		t.Run("already disabled", func(t *testing.T) {
			mock.ExpectQuery("select @@global.rpl_semi_sync_source_enabled").WillReturnRows(sqlmock.NewRows([]string{"STATUS"}).AddRow(0))
			err := manager.DisableSemiSyncSource(ctx)
			assert.Nil(t, err)
		})

		t.Run("disable", func(t *testing.T) {
			mock.ExpectQuery("select @@global.rpl_semi_sync_source_enabled").WillReturnRows(sqlmock.NewRows([]string{"STATUS"}).AddRow(1))
			mock.ExpectExec("SET GLOBAL rpl_semi_sync_source_enabled = 0;").
				WillReturnResult(sqlmock.NewResult(1, 1))
			err := manager.DisableSemiSyncSource(ctx)
			assert.Nil(t, err)
		})
	})

	t.Run("enable semi sync replica", func(t *testing.T) {
		t.Run("failed if plugin not load", func(t *testing.T) {
			mock.ExpectQuery("SELECT PLUGIN_STATUS FROM INFORMATION_SCHEMA.PLUGINS " +
				"WHERE PLUGIN_NAME ='rpl_semi_sync_replica';").WillReturnRows(sqlmock.NewRows([]string{"PLUGIN_STATUS"}))
			err := manager.EnableSemiSyncReplica(ctx)
			assert.NotNil(t, err)
			assert.ErrorContains(t, err, "status failed")
		})
		t.Run("failed if plugin not active", func(t *testing.T) {
			mock.ExpectQuery("SELECT PLUGIN_STATUS FROM INFORMATION_SCHEMA.PLUGINS " +
				"WHERE PLUGIN_NAME ='rpl_semi_sync_replica';").WillReturnRows(sqlmock.NewRows([]string{"PLUGIN_STATUS"}).AddRow("NotActive"))
			err := manager.EnableSemiSyncReplica(ctx)
			assert.NotNil(t, err)
			assert.ErrorContains(t, err, "is not active")
		})

		t.Run("already enabled", func(t *testing.T) {
			mock.ExpectQuery("SELECT PLUGIN_STATUS FROM INFORMATION_SCHEMA.PLUGINS " +
				"WHERE PLUGIN_NAME ='rpl_semi_sync_replica';").WillReturnRows(sqlmock.NewRows([]string{"PLUGIN_STATUS"}).AddRow("ACTIVE"))
			mock.ExpectQuery("select @@global.rpl_semi_sync_replica_enabled").WillReturnRows(sqlmock.NewRows([]string{"STATUS"}).AddRow(1))
			err := manager.EnableSemiSyncReplica(ctx)
			assert.Nil(t, err)
		})

		t.Run("enable", func(t *testing.T) {
			mock.ExpectQuery("SELECT PLUGIN_STATUS FROM INFORMATION_SCHEMA.PLUGINS " +
				"WHERE PLUGIN_NAME ='rpl_semi_sync_replica';").WillReturnRows(sqlmock.NewRows([]string{"PLUGIN_STATUS"}).AddRow("ACTIVE"))
			mock.ExpectQuery("select @@global.rpl_semi_sync_replica_enabled").WillReturnRows(sqlmock.NewRows([]string{"STATUS"}).AddRow(0))
			mock.ExpectExec("SET GLOBAL rpl_semi_sync_replica_enabled = 1;").
				WillReturnResult(sqlmock.NewResult(1, 1))
			err := manager.EnableSemiSyncReplica(ctx)
			assert.Nil(t, err)
		})
	})

	t.Run("disable semi sync replica", func(t *testing.T) {
		t.Run("already disabled", func(t *testing.T) {
			mock.ExpectQuery("select @@global.rpl_semi_sync_replica_enabled").WillReturnRows(sqlmock.NewRows([]string{"STATUS"}).AddRow(0))
			err := manager.DisableSemiSyncReplica(ctx)
			assert.Nil(t, err)
		})

		t.Run("disable", func(t *testing.T) {
			mock.ExpectQuery("select @@global.rpl_semi_sync_replica_enabled").WillReturnRows(sqlmock.NewRows([]string{"STATUS"}).AddRow(1))
			mock.ExpectExec("SET GLOBAL rpl_semi_sync_replica_enabled = 0;").
				WillReturnResult(sqlmock.NewResult(1, 1))
			err := manager.DisableSemiSyncReplica(ctx)
			assert.Nil(t, err)
		})
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}
