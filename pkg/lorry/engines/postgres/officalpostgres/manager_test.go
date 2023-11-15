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
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/pashagolub/pgxmock/v2"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/postgres"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
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
	viper.Set(postgres.PGMAJOR, 14)
	mock, err := pgxmock.NewPool(pgxmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}

	dbManager, err := NewManager(engines.Properties(properties))
	if err != nil {
		t.Fatal(err)
	}
	manager := dbManager.(*Manager)
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
		assert.Nil(t, err)
		assert.Equal(t, true, isLeader)
	})

	t.Run("get member role secondary", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"pg_is_in_recovery"}).AddRow(true))

		isLeader, err := manager.IsLeader(ctx, nil)
		assert.Nil(t, err)
		assert.Equal(t, false, isLeader)
	})

	t.Run("query failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		isLeader, err := manager.IsLeader(ctx, nil)
		assert.NotNil(t, err)
		assert.Equal(t, false, isLeader)
	})

	t.Run("parse query failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"pg_is_in_recovery"}))
		isLeader, err := manager.IsLeader(ctx, nil)
		assert.NotNil(t, err)
		assert.Equal(t, false, isLeader)
	})

	t.Run("has set isLeader", func(t *testing.T) {
		manager.SetIsLeader(true)
		isLeader, err := manager.IsLeader(ctx, nil)
		assert.Nil(t, err)
		assert.Equal(t, true, isLeader)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
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
			t.Errorf("expect check is cluster initialized success but failed")
		}

		assert.True(t, isInitialized)
		manager.DBStartupReady = false
	})

	t.Run("DBStartup is not set ready and ping success", func(t *testing.T) {
		mock.ExpectPing()
		isInitialized, err := manager.IsClusterInitialized(ctx, cluster)
		if err != nil {
			t.Errorf("expect check is cluster initialized success but failed")
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
			t.Errorf("expect check is cluster initialized success but failed")
		}

		assert.False(t, isInitialized)
		manager.DBStartupReady = false
	})
}

func TestGetMemberAddrs(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{Namespace: "default"}

	t.Run("get empty addrs", func(t *testing.T) {
		addrs := manager.GetMemberAddrs(ctx, cluster)

		assert.Equal(t, []string{}, addrs)
	})

	t.Run("get addrs", func(t *testing.T) {
		cluster.ClusterCompName = "pg"
		cluster.Members = append(cluster.Members, dcs.Member{
			Name:   "test",
			DBPort: "5432",
		})
		addrs := manager.GetMemberAddrs(ctx, cluster)

		assert.Equal(t, 1, len(addrs))
		assert.Equal(t, "test.pg-headless.default.svc.cluster.local:5432", addrs[0])
	})
}

func TestIsCurrentMemberHealthy(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{
		Leader: &dcs.Leader{
			Name: manager.CurrentMemberName,
		},
	}
	cluster.Members = append(cluster.Members, dcs.Member{
		Name: manager.CurrentMemberName,
	})

	t.Run("current member is healthy", func(t *testing.T) {
		mock.ExpectExec(`create table if not exists`).
			WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"check_ts"}).AddRow(1))

		isCurrentMemberHealthy := manager.IsCurrentMemberHealthy(ctx, cluster)
		assert.True(t, isCurrentMemberHealthy)
	})

	t.Run("write check failed", func(t *testing.T) {
		mock.ExpectExec(`create table if not exists`).
			WillReturnError(fmt.Errorf("some error"))

		isCurrentMemberHealthy := manager.IsCurrentMemberHealthy(ctx, cluster)
		assert.False(t, isCurrentMemberHealthy)
	})

	t.Run("read check failed", func(t *testing.T) {
		cluster.Leader.Name = "test"
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		isCurrentMemberHealthy := manager.IsCurrentMemberHealthy(ctx, cluster)
		assert.False(t, isCurrentMemberHealthy)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
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

	t.Run("parse query failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}))

		res, err := manager.getReplicationMode(ctx)
		assert.NotNil(t, err)
		assert.Equal(t, "", res)
	})

	t.Run("synchronous_commit has not been set", func(t *testing.T) {
		for i, v := range values {
			mock.ExpectQuery("select").
				WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow(v))

			res, err := manager.getReplicationMode(ctx)
			assert.Nil(t, err)
			assert.Equal(t, expects[i], res)
		}
	})

	t.Run("synchronous_commit has been set", func(t *testing.T) {
		for i, v := range expects {
			manager.DBState.Extra[postgres.ReplicationMode] = v
			res, err := manager.getReplicationMode(ctx)
			assert.Nil(t, err)
			assert.Equal(t, expects[i], res)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestGetWalPositionWithHost(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("check is leader failed", func(t *testing.T) {
		res, err := manager.getWalPositionWithHost(ctx, "test")
		assert.NotNil(t, err)
		assert.Zero(t, res)
	})

	t.Run("get primary wal position success", func(t *testing.T) {
		manager.SetIsLeader(true)
		mock.ExpectQuery("pg_catalog.pg_current_wal_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}).AddRow(23454272))

		res, err := manager.getWalPositionWithHost(ctx, "")
		assert.Nil(t, err)
		assert.Equal(t, int64(23454272), res)
	})

	t.Run("get secondary wal position success", func(t *testing.T) {
		manager.SetIsLeader(false)
		mock.ExpectQuery("pg_last_wal_replay_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}).AddRow(23454272))
		mock.ExpectQuery("pg_catalog.pg_last_wal_receive_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}).AddRow(23454273))

		res, err := manager.getWalPositionWithHost(ctx, "")
		assert.Nil(t, err)
		assert.Equal(t, int64(23454273), res)
	})

	t.Run("get primary wal position failed", func(t *testing.T) {
		manager.SetIsLeader(true)
		manager.DBState = &dcs.DBState{}
		mock.ExpectQuery("pg_catalog.pg_current_wal_lsn()").
			WillReturnError(fmt.Errorf("some error"))

		res, err := manager.getWalPositionWithHost(ctx, "")
		assert.NotNil(t, err)
		assert.Zero(t, res)
	})

	t.Run("get secondary wal position failed", func(t *testing.T) {
		manager.SetIsLeader(false)
		mock.ExpectQuery("pg_last_wal_replay_lsn()").
			WillReturnError(fmt.Errorf("some error"))
		mock.ExpectQuery("pg_catalog.pg_last_wal_receive_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}))

		res, err := manager.getWalPositionWithHost(ctx, "")
		assert.NotNil(t, err)
		assert.Zero(t, res)
	})

	t.Run("op time has been set", func(t *testing.T) {
		manager.DBState = &dcs.DBState{
			OpTimestamp: 100,
		}

		res, err := manager.getWalPositionWithHost(ctx, "")
		assert.Nil(t, err)
		assert.Equal(t, int64(100), res)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestGetSyncStandbys(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("query failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		standbys := manager.getSyncStandbys(ctx)
		assert.Nil(t, standbys)
	})

	t.Run("parse pg sync standby failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow(`ANY 4("a" b,"c c")`))

		standbys := manager.getSyncStandbys(ctx)
		assert.Nil(t, standbys)
	})

	t.Run("get sync standbys success", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow(`ANY 4("a",*,b)`))

		standbys := manager.getSyncStandbys(ctx)
		assert.NotNil(t, standbys)
		assert.True(t, standbys.HasStar)
		assert.True(t, standbys.Members.Contains("a"))
		assert.Equal(t, 4, standbys.Amount)
	})

	t.Run("pg sync standbys has been set", func(t *testing.T) {
		manager.DBState = &dcs.DBState{}
		manager.syncStandbys = &postgres.PGStandby{
			HasStar: true,
			Amount:  3,
		}

		standbys := manager.getSyncStandbys(ctx)
		assert.True(t, standbys.HasStar)
		assert.Equal(t, 3, standbys.Amount)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
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

		timeLine := manager.getReceivedTimeLine(ctx, "")
		assert.Equal(t, int64(1), timeLine)
	})

	t.Run("query failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		timeLine := manager.getReceivedTimeLine(ctx, "")
		assert.Equal(t, int64(0), timeLine)
	})

	t.Run("parse query failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"received_tli"}))

		timeLine := manager.getReceivedTimeLine(ctx, "")
		assert.Equal(t, int64(0), timeLine)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestReadRecoveryParams(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("host match", func(t *testing.T) {
		mock.ExpectQuery("pg_catalog.pg_settings").
			WillReturnRows(pgxmock.NewRows([]string{"name", "setting", "context"}).
				AddRow("primary_conninfo", "host=maple72-postgresql-0.maple72-postgresql-headless port=5432 application_name=my-application", "signup"))

		leaderName := "maple72-postgresql-0"
		recoveryParams, err := manager.readRecoveryParams(ctx)
		assert.Nil(t, err)
		assert.True(t, strings.HasPrefix(recoveryParams[postgres.PrimaryConnInfo]["host"], leaderName))
	})

	t.Run("host not match", func(t *testing.T) {
		mock.ExpectQuery("pg_catalog.pg_settings").
			WillReturnRows(pgxmock.NewRows([]string{"name", "setting", "context"}).
				AddRow("primary_conninfo", "host=test port=5432 user=postgres application_name=my-application", "signup"))

		leaderName := "a"
		recoveryParams, err := manager.readRecoveryParams(ctx)
		assert.Nil(t, err)
		assert.False(t, strings.HasPrefix(recoveryParams[postgres.PrimaryConnInfo]["host"], leaderName))
	})

	t.Run("query failed", func(t *testing.T) {
		mock.ExpectQuery("pg_catalog.pg_settings").
			WillReturnError(fmt.Errorf("some error"))

		recoveryParams, err := manager.readRecoveryParams(ctx)
		assert.NotNil(t, err)
		assert.Equal(t, "", recoveryParams[postgres.PrimaryConnInfo]["host"])
	})

	t.Run("parse query failed", func(t *testing.T) {
		mock.ExpectQuery("pg_catalog.pg_settings").
			WillReturnRows(pgxmock.NewRows([]string{"name", "setting", "context"}))

		recoveryParams, err := manager.readRecoveryParams(ctx)
		assert.NotNil(t, err)
		assert.Equal(t, "", recoveryParams[postgres.PrimaryConnInfo]["host"])
	})

	t.Run("primary info has been set", func(t *testing.T) {
		manager.recoveryParams = map[string]map[string]string{
			postgres.PrimaryConnInfo: {
				"host": "test",
			},
		}

		recoveryParams, err := manager.readRecoveryParams(ctx)
		assert.Nil(t, err)
		assert.Equal(t, "test", recoveryParams[postgres.PrimaryConnInfo]["host"])
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestCheckRecoveryConf(t *testing.T) {
	fs = afero.NewMemMapFs()
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("standby.signal not exist", func(t *testing.T) {
		needChange, needRestart := manager.checkRecoveryConf(ctx, manager.CurrentMemberName)
		assert.True(t, needChange)
		assert.True(t, needRestart)
	})

	_, err := fs.Create(manager.DataDir + "/standby.signal")
	assert.Nil(t, err)

	t.Run("query primaryInfo failed", func(t *testing.T) {
		mock.ExpectQuery("pg_catalog.pg_settings").
			WillReturnError(fmt.Errorf("some error"))

		needChange, needRestart := manager.checkRecoveryConf(ctx, manager.CurrentMemberName)
		assert.True(t, needChange)
		assert.True(t, needRestart)
	})

	t.Run("host not match and restart", func(t *testing.T) {
		mock.ExpectQuery("pg_catalog.pg_settings").
			WillReturnRows(pgxmock.NewRows([]string{"name", "setting", "context"}).
				AddRow("primary_conninfo", "host=maple72-postgresql-0.maple72-postgresql-headless port=5432 application_name=my-application", "postmaster"))

		needChange, needRestart := manager.checkRecoveryConf(ctx, manager.CurrentMemberName)
		assert.True(t, needChange)
		assert.True(t, needRestart)
	})

	t.Run("host not match and reload", func(t *testing.T) {
		mock.ExpectQuery("pg_catalog.pg_settings").
			WillReturnRows(pgxmock.NewRows([]string{"name", "setting", "context"}).
				AddRow("primary_conninfo", "host=maple72-postgresql-0.maple72-postgresql-headless port=5432 application_name=my-application", "signup"))

		needChange, needRestart := manager.checkRecoveryConf(ctx, manager.CurrentMemberName)
		assert.True(t, needChange)
		assert.False(t, needRestart)
	})

	t.Run("host match", func(t *testing.T) {
		mock.ExpectQuery("pg_catalog.pg_settings").
			WillReturnRows(pgxmock.NewRows([]string{"name", "setting", "context"}).
				AddRow("primary_conninfo", "host=test-pod-0.maple72-postgresql-headless port=5432 application_name=my-application", "signup"))

		needChange, needRestart := manager.checkRecoveryConf(ctx, manager.CurrentMemberName)
		assert.False(t, needChange)
		assert.False(t, needRestart)
	})

	if err = mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestIsMemberLagging(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{
		HaConfig: &dcs.HaConfig{},
	}
	cluster.Members = append(cluster.Members, dcs.Member{
		Name: manager.CurrentMemberName,
	})
	currentMember := cluster.GetMemberWithName(manager.CurrentMemberName)

	t.Run("db state is nil", func(t *testing.T) {
		isLagging, lag := manager.IsMemberLagging(ctx, cluster, currentMember)
		assert.False(t, isLagging)
		assert.Equal(t, int64(0), lag)
	})

	cluster.Leader = &dcs.Leader{
		DBState: &dcs.DBState{
			OpTimestamp: 100,
			Extra: map[string]string{
				postgres.TimeLine: "1",
			},
		},
	}

	t.Run("get replication mode failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		isLagging, lag := manager.IsMemberLagging(ctx, cluster, currentMember)
		assert.True(t, isLagging)
		assert.Equal(t, int64(1), lag)
	})

	t.Run("not sync to leader", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("on"))

		isLagging, lag := manager.IsMemberLagging(ctx, cluster, currentMember)
		assert.True(t, isLagging)
		assert.Equal(t, int64(1), lag)
	})

	t.Run("get timeline failed", func(t *testing.T) {
		manager.SetIsLeader(true)
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("off"))
		mock.ExpectQuery("SELECT timeline_id").
			WillReturnError(fmt.Errorf("some error"))

		isLagging, lag := manager.IsMemberLagging(ctx, cluster, currentMember)
		assert.True(t, isLagging)
		assert.Equal(t, int64(1), lag)
	})

	t.Run("timeline not match", func(t *testing.T) {
		manager.SetIsLeader(true)
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("off"))
		mock.ExpectQuery("SELECT timeline_id").
			WillReturnRows(pgxmock.NewRows([]string{"timeline_id"}).AddRow(2))
		isLagging, lag := manager.IsMemberLagging(ctx, cluster, currentMember)
		assert.True(t, isLagging)
		assert.Equal(t, int64(1), lag)
	})

	t.Run("get wal position failed", func(t *testing.T) {
		manager.SetIsLeader(true)
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("off"))
		mock.ExpectQuery("SELECT timeline_id").
			WillReturnRows(pgxmock.NewRows([]string{"timeline_id"}).AddRow(1))
		mock.ExpectQuery("pg_catalog.pg_current_wal_lsn()").
			WillReturnError(fmt.Errorf("some error"))

		isLagging, lag := manager.IsMemberLagging(ctx, cluster, currentMember)
		assert.True(t, isLagging)
		assert.Equal(t, int64(1), lag)
	})

	t.Run("current member is not lagging", func(t *testing.T) {
		manager.SetIsLeader(true)
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("off"))
		mock.ExpectQuery("SELECT timeline_id").
			WillReturnRows(pgxmock.NewRows([]string{"timeline_id"}).AddRow(1))
		mock.ExpectQuery("pg_catalog.pg_current_wal_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}).AddRow(100))

		isLagging, lag := manager.IsMemberLagging(ctx, cluster, currentMember)
		assert.False(t, isLagging)
		assert.Equal(t, int64(0), lag)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestGetCurrentTimeLine(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("query failed", func(t *testing.T) {
		mock.ExpectQuery("SELECT timeline_id").
			WillReturnError(fmt.Errorf("some error"))

		timeline := manager.getCurrentTimeLine(ctx, "")
		assert.Equal(t, int64(0), timeline)
	})

	t.Run("parse query failed", func(t *testing.T) {
		mock.ExpectQuery("SELECT timeline_id").
			WillReturnRows(pgxmock.NewRows([]string{"timeline_id"}))

		timeline := manager.getCurrentTimeLine(ctx, "")
		assert.Equal(t, int64(0), timeline)
	})

	t.Run("get current timeline success", func(t *testing.T) {
		mock.ExpectQuery("SELECT timeline_id").
			WillReturnRows(pgxmock.NewRows([]string{"timeline_id"}).AddRow(1))

		timeline := manager.getCurrentTimeLine(ctx, "")
		assert.Equal(t, int64(1), timeline)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestGetTimeLineWithHost(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("check is leader failed", func(t *testing.T) {
		timeLine := manager.getTimeLineWithHost(ctx, "test")
		assert.Zero(t, timeLine)
	})

	t.Run("timeLine has been set", func(t *testing.T) {
		manager.DBState = &dcs.DBState{
			Extra: map[string]string{
				postgres.TimeLine: "1",
			},
		}

		timeLine := manager.getTimeLineWithHost(ctx, "")
		assert.Equal(t, int64(1), timeLine)
	})
}

func TestGetLocalTimeLineAndLsn(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("db is not running", func(t *testing.T) {
		isRecovery, localTimeLine, localLsn := manager.getLocalTimeLineAndLsn(ctx)
		assert.False(t, isRecovery)
		assert.Equal(t, int64(0), localTimeLine)
		assert.Equal(t, int64(0), localLsn)
	})

	manager.Proc = &process.Process{
		// Process 1 is always in a running state.
		Pid: 1,
	}

	t.Run("get local timeline and lsn success", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"received_tli"}).AddRow(1))
		mock.ExpectQuery("pg_last_wal_replay_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}).AddRow(23454272))

		isRecovery, localTimeLine, localLsn := manager.getLocalTimeLineAndLsn(ctx)
		assert.True(t, isRecovery)
		assert.Equal(t, int64(1), localTimeLine)
		assert.Equal(t, int64(23454272), localLsn)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestCleanDBState(t *testing.T) {
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("clean db state", func(t *testing.T) {
		manager.cleanDBState()
		isSet, isLeader := manager.GetIsLeader()
		assert.False(t, isSet)
		assert.False(t, isLeader)
		assert.Nil(t, manager.recoveryParams)
		assert.Nil(t, manager.syncStandbys)
		assert.Equal(t, &dcs.DBState{
			Extra: map[string]string{},
		}, manager.DBState)
	})
}

func TestGetDBState(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	defer func() {
		postgres.LocalCommander = postgres.NewExecCommander
	}()
	cluster := &dcs.Cluster{}

	t.Run("check is leader failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		dbState := manager.GetDBState(ctx, cluster)
		assert.Nil(t, dbState)
	})

	t.Run("get replication mode failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"pg_is_in_recovery"}).AddRow(false))
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		dbState := manager.GetDBState(ctx, cluster)
		assert.Nil(t, dbState)
	})

	t.Run("synchronous mode but get wal position failed", func(t *testing.T) {
		cluster.Leader = &dcs.Leader{
			Name: manager.CurrentMemberName,
		}
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"pg_is_in_recovery"}).AddRow(false))
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("on"))
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow(`ANY 4("a",*,b)`))
		mock.ExpectQuery("pg_catalog.pg_current_wal_lsn()").
			WillReturnError(fmt.Errorf("some error"))

		dbState := manager.GetDBState(ctx, cluster)
		assert.Nil(t, dbState)
	})

	t.Run("get timeline failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"pg_is_in_recovery"}).AddRow(false))
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("off"))
		mock.ExpectQuery("pg_catalog.pg_current_wal_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}).AddRow(23454272))
		mock.ExpectQuery("SELECT timeline_id").
			WillReturnError(fmt.Errorf("some error"))

		dbState := manager.GetDBState(ctx, cluster)
		assert.Nil(t, dbState)
	})

	t.Run("read recovery params failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"pg_is_in_recovery"}).AddRow(true))
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("off"))
		mock.ExpectQuery("pg_last_wal_replay_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}).AddRow(23454272))
		mock.ExpectQuery("pg_catalog.pg_last_wal_receive_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}).AddRow(23454273))
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"received_tli"}).AddRow(1))
		mock.ExpectQuery("pg_catalog.pg_settings").
			WillReturnError(fmt.Errorf("some error"))

		dbState := manager.GetDBState(ctx, cluster)
		assert.Nil(t, dbState)
	})

	t.Run("get pg control data failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"pg_is_in_recovery"}).AddRow(true))
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("off"))
		mock.ExpectQuery("pg_last_wal_replay_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}).AddRow(23454272))
		mock.ExpectQuery("pg_catalog.pg_last_wal_receive_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}).AddRow(23454273))
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"received_tli"}).AddRow(1))
		mock.ExpectQuery("pg_catalog.pg_settings").
			WillReturnRows(pgxmock.NewRows([]string{"name", "setting", "context"}).
				AddRow("primary_conninfo", "host=maple72-postgresql-0.maple72-postgresql-headless port=5432 application_name=my-application", "postmaster"))
		postgres.LocalCommander = postgres.NewFakeCommander(func() error {
			return fmt.Errorf("some error")
		}, nil, nil)

		dbState := manager.GetDBState(ctx, cluster)
		assert.Nil(t, dbState)
	})

	t.Run("get db state success", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"pg_is_in_recovery"}).AddRow(true))
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("off"))
		mock.ExpectQuery("pg_last_wal_replay_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}).AddRow(23454272))
		mock.ExpectQuery("pg_catalog.pg_last_wal_receive_lsn()").
			WillReturnRows(pgxmock.NewRows([]string{"pg_wal_lsn_diff"}).AddRow(23454273))
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"received_tli"}).AddRow(1))
		mock.ExpectQuery("pg_catalog.pg_settings").
			WillReturnRows(pgxmock.NewRows([]string{"name", "setting", "context"}).
				AddRow("primary_conninfo", "host=maple72-postgresql-0.maple72-postgresql-headless port=5432 application_name=my-application", "postmaster"))
		fakeControlData := "WAL block size:                       8192\n" +
			"Database cluster state:               shut down"

		var stdout = bytes.NewBuffer([]byte(fakeControlData))
		postgres.LocalCommander = postgres.NewFakeCommander(func() error {
			return nil
		}, stdout, nil)

		dbState := manager.GetDBState(ctx, cluster)
		isSet, isLeader := manager.GetIsLeader()
		assert.NotNil(t, dbState)
		assert.True(t, isSet)
		assert.False(t, isLeader)
		assert.Equal(t, postgres.Asynchronous, dbState.Extra[postgres.ReplicationMode])
		assert.Equal(t, int64(23454273), dbState.OpTimestamp)
		assert.Equal(t, "1", dbState.Extra[postgres.TimeLine])
		assert.Equal(t, "maple72-postgresql-0.maple72-postgresql-headless", manager.recoveryParams[postgres.PrimaryConnInfo]["host"])
		assert.Equal(t, "postmaster", manager.recoveryParams[postgres.PrimaryConnInfo]["context"])
		assert.Equal(t, "shut down", manager.pgControlData["Database cluster state"])
		assert.Equal(t, "8192", manager.pgControlData["WAL block size"])
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestFollow(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{
		Leader: &dcs.Leader{
			Name: manager.CurrentMemberName,
		},
	}
	fs = afero.NewMemMapFs()

	t.Run("cluster has no leader now", func(t *testing.T) {
		err := manager.follow(ctx, false, cluster)
		assert.Nil(t, err)
	})

	cluster.Members = append(cluster.Members, dcs.Member{
		Name: manager.CurrentMemberName,
	})

	t.Run("current member is leader", func(t *testing.T) {
		err := manager.follow(ctx, false, cluster)
		assert.Nil(t, err)
	})

	manager.CurrentMemberName = "test"

	t.Run("open postgresql conf failed", func(t *testing.T) {
		err := manager.follow(ctx, true, cluster)
		assert.NotNil(t, err)
	})

	t.Run("open postgresql conf failed", func(t *testing.T) {
		err := manager.follow(ctx, true, cluster)
		assert.NotNil(t, err)
	})

	t.Run("follow without restart", func(t *testing.T) {
		_, _ = fs.Create("/kubeblocks/conf/postgresql.conf")
		mock.ExpectExec("select pg_reload_conf()").
			WillReturnResult(pgxmock.NewResult("select", 1))

		err := manager.follow(ctx, false, cluster)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestHasOtherHealthyMembers(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{}
	cluster.Members = append(cluster.Members, dcs.Member{
		Name: manager.CurrentMemberName,
	})

	t.Run("", func(t *testing.T) {
		members := manager.HasOtherHealthyMembers(ctx, cluster, manager.CurrentMemberName)
		assert.Equal(t, 0, len(members))
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestGetPgControlData(t *testing.T) {
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	defer func() {
		postgres.LocalCommander = postgres.NewExecCommander
	}()

	t.Run("get pg control data failed", func(t *testing.T) {
		postgres.LocalCommander = postgres.NewFakeCommander(func() error {
			return fmt.Errorf("some error")
		}, nil, nil)

		data := manager.getPgControlData()
		assert.Nil(t, data)
	})

	t.Run("get pg control data success", func(t *testing.T) {
		fakeControlData := "pg_control version number:            1002\n" +
			"Data page checksum version:           0"

		var stdout = bytes.NewBuffer([]byte(fakeControlData))
		postgres.LocalCommander = postgres.NewFakeCommander(func() error {
			return nil
		}, stdout, nil)

		data := manager.getPgControlData()
		assert.NotNil(t, data)
		assert.Equal(t, "1002", data["pg_control version number"])
		assert.Equal(t, "0", data["Data page checksum version"])
	})

	t.Run("pg control data has been set", func(t *testing.T) {
		manager.pgControlData = map[string]string{
			"Data page checksum version": "1",
		}

		data := manager.getPgControlData()
		assert.NotNil(t, data)
		assert.Equal(t, "1", data["Data page checksum version"])
	})
}
