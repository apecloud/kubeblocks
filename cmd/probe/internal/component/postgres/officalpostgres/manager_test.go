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

package officalpostgres

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/dapr/kit/logger"
	"github.com/pashagolub/pgxmock/v2"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/component/postgres"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
	"github.com/apecloud/kubeblocks/internal/constant"
)

func MockDatabase(t *testing.T) (*Manager, pgxmock.PgxPoolIface, error) {
	properties := map[string]string{
		postgres.ConnectionURLKey: "user=test password=test host=localhost port=5432 dbname=postgres",
	}
	testConfig, err := postgres.NewConfig(properties)
	assert.NotNil(t, testConfig)
	assert.Nil(t, err)

	viper.Set(constant.KBEnvPodName, "test-pod-0")
	viper.Set(constant.KBEnvClusterCompName, "test")
	viper.Set(constant.KBEnvNamespace, "default")
	viper.Set(postgres.PGDATA, "test")
	mock, err := pgxmock.NewPool(pgxmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}

	manager, err := NewManager(logger.NewLogger("test"))
	if err != nil {
		t.Fatal(err)
	}
	manager.Pool = mock

	return manager, mock, err
}

func TestGetMemberRoleWithHost(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("get member role primary", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"pg_is_in_recovery"}).AddRow(false))

		role, err := manager.GetMemberRoleWithHost(ctx, "")
		if err != nil {
			t.Errorf("expect get member role success, but failed, err:%v", err)
		}

		assert.Equal(t, role, binding.PRIMARY)
	})

	t.Run("get member role secondary", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"pg_is_in_recovery"}).AddRow(true))

		role, err := manager.GetMemberRoleWithHost(ctx, "")
		if err != nil {
			t.Errorf("expect get member role success, but failed, err:%v", err)
		}

		assert.Equal(t, role, binding.SECONDARY)
	})

	t.Run("get member failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		role, err := manager.GetMemberRoleWithHost(ctx, "")
		if err == nil {
			t.Errorf("expect get member role failed, but success")
		}

		assert.Equal(t, role, "")
	})
}

func TestGetReplicationMode(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("synchronous_commit off", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("off"))

		res, err := manager.getReplicationMode(ctx)
		if err != nil {
			t.Errorf("expect get replication mode success but failed, err:%v", err)
		}

		assert.Equal(t, res, postgres.Asynchronous)
	})

	t.Run("synchronous_commit on", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("on"))

		res, err := manager.getReplicationMode(ctx)
		if err != nil {
			t.Errorf("expect get replication mode success but failed, err:%v", err)
		}

		assert.Equal(t, res, postgres.Synchronous)
	})
}

func TestGetWalPositionWithHost(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("get primary wal position", func(t *testing.T) {
		manager.isLeader = true
		mock.ExpectQuery("pg_catalog.pg_current_wal_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}).AddRow(23454272))

		res, err := manager.getWalPositionWithHost(ctx, "")
		if err != nil {
			t.Errorf("expect get wal postition success but failed, err:%v", err)
		}

		assert.Equal(t, res, int64(23454272))
	})

	t.Run("get secondary wal position", func(t *testing.T) {
		manager.isLeader = false
		mock.ExpectQuery("pg_last_wal_replay_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}).AddRow(23454272))
		mock.ExpectQuery("pg_catalog.pg_last_wal_receive_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}).AddRow(23454273))

		res, err := manager.getWalPositionWithHost(ctx, "")
		if err != nil {
			t.Errorf("expect get wal postition success but failed, err:%v", err)
		}

		assert.Equal(t, res, int64(23454273))
	})

	t.Run("get wal position failed", func(t *testing.T) {
		manager.isLeader = true
		mock.ExpectQuery("pg_catalog.pg_current_wal_lsn()").
			WillReturnError(fmt.Errorf("some error"))

		res, err := manager.getWalPositionWithHost(ctx, "")
		if err == nil {
			t.Errorf("expect get wal postition failed but success")
		}

		assert.Equal(t, res, int64(0))
	})
}

func TestGetSyncStandbys(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("get sync standbys success", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow(`ANY 4("a",*,b)`))

		standbys := manager.getSyncStandbys(ctx)
		assert.NotNil(t, standbys)
		assert.True(t, standbys.HasStar)
		assert.True(t, standbys.Members.Contains("a"))
		assert.Equal(t, 4, standbys.Amount)
	})

	t.Run("get sync standbys failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		standbys := manager.getSyncStandbys(ctx)
		assert.Nil(t, standbys)
	})
}

func TestCheckStandbySynchronizedToLeader(t *testing.T) {
	cluster := &dcs.Cluster{
		Leader: &dcs.Leader{
			DBState: &dcs.DBState{
				Extra: map[string]string{},
			},
		},
	}

	t.Run("synchronized to leader", func(t *testing.T) {
		manager, _, _ := MockDatabase(t)
		manager.CurrentMemberName = "a"
		cluster.Leader.DBState.Extra[postgres.SyncStandBys] = "a,b,c"

		ok := manager.checkStandbySynchronizedToLeader(true, cluster)
		assert.True(t, ok)
	})

	t.Run("is leader", func(t *testing.T) {
		manager, _, _ := MockDatabase(t)
		manager.CurrentMemberName = "a"
		cluster.Leader.Name = "a"
		cluster.Leader.DBState.Extra[postgres.SyncStandBys] = "b,c"

		ok := manager.checkStandbySynchronizedToLeader(true, cluster)
		assert.True(t, ok)
	})

	t.Run("not synchronized to leader", func(t *testing.T) {
		manager, _, _ := MockDatabase(t)
		manager.CurrentMemberName = "d"
		cluster.Leader.DBState.Extra[postgres.SyncStandBys] = "a,b,c"

		ok := manager.checkStandbySynchronizedToLeader(true, cluster)
		assert.False(t, ok)
	})
}

func TestGetReceivedTimeLine(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("get received timeline success", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"received_tli"}).AddRow(1))

		timeLine := manager.getReceivedTimeLine(ctx)
		assert.Equal(t, timeLine, int64(1))
	})

	t.Run("get received timeline failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		timeLine := manager.getReceivedTimeLine(ctx)
		assert.Equal(t, timeLine, int64(0))
	})
}

func TestReadRecoveryParams(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("host match", func(t *testing.T) {
		mock.ExpectQuery("pg_catalog.pg_settings").
			WillReturnRows(pgxmock.NewRows([]string{"name", "setting"}).AddRow("primary_conninfo", "host=test port=5432 user=postgres application_name=my-application"))

		leaderName := "test"
		primaryInfo := manager.readRecoveryParams(ctx)
		assert.True(t, strings.HasPrefix(primaryInfo["host"], leaderName))
	})

	t.Run("host not match", func(t *testing.T) {
		mock.ExpectQuery("pg_catalog.pg_settings").
			WillReturnRows(pgxmock.NewRows([]string{"name", "setting"}).AddRow("primary_conninfo", "host=test port=5432 user=postgres application_name=my-application"))

		leaderName := "a"
		primaryInfo := manager.readRecoveryParams(ctx)
		assert.False(t, strings.HasPrefix(primaryInfo["host"], leaderName))
	})

	t.Run("read recovery params failed", func(t *testing.T) {
		mock.ExpectQuery("pg_catalog.pg_settings").
			WillReturnError(fmt.Errorf("some error"))

		primaryInfo := manager.readRecoveryParams(ctx)
		assert.Nil(t, primaryInfo)
	})
}
