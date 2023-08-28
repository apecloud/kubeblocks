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
	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/component/postgres"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/dcs"
	"github.com/apecloud/kubeblocks/internal/constant"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
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

func TestIsLeader(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("get member role primary", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"pg_is_in_recovery"}).AddRow(false))

		isLeader, err := manager.IsLeader(ctx, nil)
		if err != nil {
			t.Errorf("expect get member role success, but failed, err:%v", err)
		}

		assert.Equal(t, true, isLeader)
	})

	t.Run("get member role secondary", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"pg_is_in_recovery"}).AddRow(true))

		isLeader, err := manager.IsLeader(ctx, nil)
		if err != nil {
			t.Errorf("expect get member role success, but failed, err:%v", err)
		}

		assert.Equal(t, false, isLeader)
	})

	t.Run("get member failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		isLeader, err := manager.IsLeader(ctx, nil)
		if err == nil {
			t.Errorf("expect get member role failed, but success")
		}

		assert.Equal(t, false, isLeader)
	})

	t.Run("has set isLeader", func(t *testing.T) {
		manager.SetIsLeader(true)
		isLeader, err := manager.IsLeader(ctx, nil)
		if err != nil {
			t.Errorf("expect get member role success, but failed")
		}

		assert.Equal(t, true, isLeader)
	})
}

func TestIsClusterInitialized(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{}

	t.Run("DBStartup is set Ready", func(t *testing.T) {
		manager.DBStartupReady = true

		isInitialized, err := manager.IsClusterInitialized(ctx, cluster)
		if err != nil {
			t.Errorf("exepect check is cluster initialized success but failed")
		}

		assert.True(t, isInitialized)
		manager.DBStartupReady = false
	})

	t.Run("DBStartup is not set ready and ping success", func(t *testing.T) {
		mock.ExpectPing()
		isInitialized, err := manager.IsClusterInitialized(ctx, cluster)
		if err != nil {
			t.Errorf("exepect check is cluster initialized success but failed")
		}

		if err = mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}

		assert.True(t, isInitialized)
		manager.DBStartupReady = false
	})

	t.Run("DBStartup is not set ready but ping failed", func(t *testing.T) {
		isInitialized, err := manager.IsClusterInitialized(ctx, cluster)
		if err != nil {
			t.Errorf("exepect check is cluster initialized success but failed")
		}

		assert.False(t, isInitialized)
		manager.DBStartupReady = false
	})
}

func TestGetMemberAddrs(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{}

	t.Run("get empty addrs", func(t *testing.T) {
		addrs := manager.GetMemberAddrs(ctx, cluster)

		assert.Equal(t, []string{}, addrs)
	})

	t.Run("get addrs", func(t *testing.T) {
		cluster.ClusterCompName = "test"
		cluster.Members = append(cluster.Members, dcs.Member{
			Name:   "test",
			DBPort: "5432",
		})
		addrs := manager.GetMemberAddrs(ctx, cluster)

		assert.Equal(t, 1, len(addrs))
		assert.Equal(t, "test.test-headless:5432", addrs[0])
	})
}

func TestIsCurrentMemberHealthy(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{}

	t.Run("current member is healthy", func(t *testing.T) {
		cluster.Leader = &dcs.Leader{
			Name: "test-pod-0",
		}
		cluster.Members = append(cluster.Members, dcs.Member{
			Name: manager.CurrentMemberName,
		})
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("off"))
		mock.ExpectExec(`create table if not exists`).
			WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
		row := pgxmock.NewRows([]string{"check_ts"}).AddRow(1)
		mock.ExpectQuery("select").WillReturnRows(row)

		isCurrentMemberHealthy := manager.IsCurrentMemberHealthy(ctx, cluster)

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}

		assert.True(t, isCurrentMemberHealthy)
	})

	t.Run("get replication mode failed", func(t *testing.T) {
		cluster.Leader = &dcs.Leader{}
		cluster.Members = append(cluster.Members, dcs.Member{
			Name: manager.CurrentMemberName,
		})
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("on"))

		isCurrentMemberHealthy := manager.IsCurrentMemberHealthy(ctx, cluster)
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}

		assert.False(t, isCurrentMemberHealthy)
	})

	t.Run("get replication mode failed", func(t *testing.T) {
		cluster.Leader = &dcs.Leader{}
		cluster.Members = append(cluster.Members, dcs.Member{
			Name: manager.CurrentMemberName,
		})
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some err"))

		isCurrentMemberHealthy := manager.IsCurrentMemberHealthy(ctx, cluster)
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}

		assert.False(t, isCurrentMemberHealthy)
	})

	t.Run("write check failed", func(t *testing.T) {
		cluster.Leader = &dcs.Leader{
			Name: "test-pod-0",
		}
		cluster.Members = append(cluster.Members, dcs.Member{
			Name: manager.CurrentMemberName,
		})
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("off"))
		mock.ExpectExec(`create table if not exists`).
			WillReturnError(fmt.Errorf("some err"))

		isCurrentMemberHealthy := manager.IsCurrentMemberHealthy(ctx, cluster)
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}

		assert.False(t, isCurrentMemberHealthy)
	})

	t.Run("read check failed", func(t *testing.T) {
		cluster.Leader = &dcs.Leader{
			Name: "test-pod-1",
		}
		cluster.Members = append(cluster.Members, dcs.Member{
			Name: manager.CurrentMemberName,
		})
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("off"))
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some err"))

		isCurrentMemberHealthy := manager.IsCurrentMemberHealthy(ctx, cluster)
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}

		assert.False(t, isCurrentMemberHealthy)
	})
}

func TestIsMemberLagging(t *testing.T) {

}

func TestGetReplicationMode(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	values := []string{"off", "local", "remote_write", "remote_apply", "on", ""}
	expects := []string{postgres.Asynchronous, postgres.Asynchronous, postgres.Asynchronous, postgres.Synchronous, postgres.Synchronous, postgres.Synchronous}
	manager.DBState = &dcs.DBState{
		Extra: map[string]string{},
	}

	t.Run("synchronous_commit has not been set", func(t *testing.T) {
		for i, v := range values {
			mock.ExpectQuery("select").
				WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow(v))

			res, err := manager.getReplicationMode(ctx)
			if err != nil {
				t.Errorf("expect get replication mode success but failed, err:%v", err)
			}

			assert.Equal(t, expects[i], res)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}
	})

	t.Run("synchronous_commit has been set", func(t *testing.T) {
		for i, v := range expects {
			manager.DBState.Extra[postgres.ReplicationMode] = v
			res, err := manager.getReplicationMode(ctx)
			if err != nil {
				t.Errorf("expect get replication mode success but failed, err:%v", err)
			}

			assert.Equal(t, expects[i], res)
		}
	})
}

func TestGetWalPositionWithHost(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("get primary wal position", func(t *testing.T) {
		manager.SetIsLeader(true)
		manager.DBState = &dcs.DBState{}
		mock.ExpectQuery("pg_catalog.pg_current_wal_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}).AddRow(23454272))

		res, err := manager.getWalPositionWithHost(ctx, "")
		if err != nil {
			t.Errorf("expect get wal postition success but failed, err:%v", err)
		}

		assert.Equal(t, int64(23454272), res)
	})

	t.Run("get secondary wal position", func(t *testing.T) {
		manager.SetIsLeader(false)
		manager.DBState = &dcs.DBState{}
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
		manager.SetIsLeader(true)
		manager.DBState = &dcs.DBState{}
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
			WillReturnRows(pgxmock.NewRows([]string{"name", "setting"}).AddRow("primary_conninfo", "host=maple72-postgresql-0.maple72-postgresql-headless port=5432 application_name=my-application"))

		leaderName := "maple72-postgresql-0"
		primaryInfo := manager.readRecoveryParams(ctx)
		assert.True(t, strings.HasPrefix(primaryInfo, leaderName))
	})

	t.Run("host not match", func(t *testing.T) {
		mock.ExpectQuery("pg_catalog.pg_settings").
			WillReturnRows(pgxmock.NewRows([]string{"name", "setting"}).AddRow("primary_conninfo", "host=test port=5432 user=postgres application_name=my-application"))

		leaderName := "a"
		primaryInfo := manager.readRecoveryParams(ctx)
		assert.False(t, strings.HasPrefix(primaryInfo, leaderName))
	})

	t.Run("read recovery params failed", func(t *testing.T) {
		mock.ExpectQuery("pg_catalog.pg_settings").
			WillReturnError(fmt.Errorf("some error"))

		primaryInfo := manager.readRecoveryParams(ctx)
		assert.Equal(t, "", primaryInfo)
	})
}
