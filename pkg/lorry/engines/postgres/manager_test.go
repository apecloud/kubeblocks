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
	"testing"

	"github.com/pashagolub/pgxmock/v2"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func MockDatabase(t *testing.T) (*Manager, pgxmock.PgxPoolIface, error) {
	properties := map[string]string{
		ConnectionURLKey: "user=test password=test host=localhost port=5432 dbname=postgres",
	}
	testConfig, err := NewConfig(properties)
	assert.NotNil(t, testConfig)
	assert.Nil(t, err)

	viper.Set(constant.KBEnvPodName, "test-pod-0")
	viper.Set(constant.KBEnvClusterCompName, "test")
	viper.Set(constant.KBEnvNamespace, "default")
	viper.Set(PGDATA, "test")
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

func TestIsRunning(t *testing.T) {
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("proc is nil, can't read file", func(t *testing.T) {
		isRunning := manager.IsRunning()
		assert.False(t, isRunning)
	})

	t.Run("proc is not nil ,process is not exist", func(t *testing.T) {
		manager.Proc = &process.Process{
			Pid: 100000,
		}

		isRunning := manager.IsRunning()
		assert.False(t, isRunning)
	})
}

func TestNewProcessFromPidFile(t *testing.T) {
	fs = afero.NewMemMapFs()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("file is not exist", func(t *testing.T) {
		err := manager.newProcessFromPidFile()
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "file does not exist")
	})

	t.Run("process is not exist", func(t *testing.T) {
		data := "100000\n/postgresql/data\n1692770488\n5432\n/var/run/postgresql\n*\n  2388960         4\nready"
		err := afero.WriteFile(fs, manager.DataDir+"/postmaster.pid", []byte(data), 0644)
		if err != nil {
			t.Fatal(err)
		}

		err = manager.newProcessFromPidFile()
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "process does not exist")
	})
}

func TestReadWrite(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("write check success", func(t *testing.T) {
		mock.ExpectExec(`create table if not exists`).
			WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))

		ok := manager.WriteCheck(ctx, "")
		assert.True(t, ok)
	})

	t.Run("write check failed", func(t *testing.T) {
		mock.ExpectExec(`create table if not exists`).
			WillReturnError(fmt.Errorf("some error"))

		ok := manager.WriteCheck(ctx, "")
		assert.False(t, ok)
	})

	t.Run("read check success", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"check_ts"}).AddRow(1))

		ok := manager.ReadCheck(ctx, "")
		assert.True(t, ok)
	})

	t.Run("read check failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		ok := manager.ReadCheck(ctx, "")
		assert.False(t, ok)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestPgIsReady(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("pg is ready", func(t *testing.T) {
		mock.ExpectPing()

		if isReady := manager.IsPgReady(ctx); !isReady {
			t.Errorf("test pg is ready failed")
		}
	})

	t.Run("pg is not ready", func(t *testing.T) {
		mock.ExpectPing().WillReturnError(fmt.Errorf("can't ping to db"))
		if isReady := manager.IsPgReady(ctx); isReady {
			t.Errorf("expect pg is not ready, but get ready")
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestSetAndUnsetIsLeader(t *testing.T) {
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("set is leader", func(t *testing.T) {
		manager.SetIsLeader(true)
		isSet, isLeader := manager.GetIsLeader()
		assert.True(t, isSet)
		assert.True(t, isLeader)
	})

	t.Run("set is not leader", func(t *testing.T) {
		manager.SetIsLeader(false)
		isSet, isLeader := manager.GetIsLeader()
		assert.True(t, isSet)
		assert.False(t, isLeader)
	})

	t.Run("unset is leader", func(t *testing.T) {
		manager.UnsetIsLeader()
		isSet, isLeader := manager.GetIsLeader()
		assert.False(t, isSet)
		assert.False(t, isLeader)
	})
}

func TestIsLeaderMember(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{}
	currentMember := dcs.Member{
		Name: manager.CurrentMemberName,
	}

	t.Run("member is nil", func(t *testing.T) {
		isLeaderMember, err := manager.IsLeaderMember(ctx, cluster, nil)
		assert.False(t, isLeaderMember)
		assert.NotNil(t, err)
	})

	t.Run("leader member is nil", func(t *testing.T) {
		isLeaderMember, err := manager.IsLeaderMember(ctx, cluster, &currentMember)
		assert.False(t, isLeaderMember)
		assert.NotNil(t, err)
	})

	cluster.Leader = &dcs.Leader{
		Name: manager.CurrentMemberName,
	}
	cluster.Members = append(cluster.Members, currentMember)
	t.Run("is leader member", func(t *testing.T) {
		isLeaderMember, err := manager.IsLeaderMember(ctx, cluster, &currentMember)
		assert.True(t, isLeaderMember)
		assert.Nil(t, err)
	})

	member := &dcs.Member{
		Name: "test",
	}
	t.Run("is not leader member", func(t *testing.T) {
		isLeaderMember, err := manager.IsLeaderMember(ctx, cluster, member)
		assert.False(t, isLeaderMember)
		assert.Nil(t, err)
	})
}

func TestPgReload(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("pg reload success", func(t *testing.T) {
		mock.ExpectExec("select pg_reload_conf()").
			WillReturnResult(pgxmock.NewResult("select", 1))

		err := manager.PgReload(ctx)
		assert.Nil(t, err)
	})

	t.Run("pg reload failed", func(t *testing.T) {
		mock.ExpectExec("select pg_reload_conf()").
			WillReturnError(fmt.Errorf("some error"))

		err := manager.PgReload(ctx)
		assert.NotNil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestLock(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("alter system failed", func(t *testing.T) {
		mock.ExpectExec("alter system").
			WillReturnError(fmt.Errorf("alter system failed"))

		err := manager.Lock(ctx, "test")
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "alter system failed")
	})

	t.Run("pg reload failed", func(t *testing.T) {
		mock.ExpectExec("alter system").
			WillReturnResult(pgxmock.NewResult("alter", 1))
		mock.ExpectExec("select pg_reload_conf()").
			WillReturnError(fmt.Errorf("pg reload failed"))
		err := manager.Lock(ctx, "test")
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "pg reload failed")
	})

	t.Run("lock success", func(t *testing.T) {
		mock.ExpectExec("alter system").
			WillReturnResult(pgxmock.NewResult("alter", 1))
		mock.ExpectExec("select pg_reload_conf()").
			WillReturnResult(pgxmock.NewResult("select", 1))
		err := manager.Lock(ctx, "test")
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestUnlock(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("alter system failed", func(t *testing.T) {
		mock.ExpectExec("alter system").
			WillReturnError(fmt.Errorf("alter system failed"))

		err := manager.Unlock(ctx)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "alter system failed")
	})

	t.Run("pg reload failed", func(t *testing.T) {
		mock.ExpectExec("alter system").
			WillReturnResult(pgxmock.NewResult("alter", 1))
		mock.ExpectExec("select pg_reload_conf()").
			WillReturnError(fmt.Errorf("pg reload failed"))
		err := manager.Unlock(ctx)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "pg reload failed")
	})

	t.Run("unlock success", func(t *testing.T) {
		mock.ExpectExec("alter system").
			WillReturnResult(pgxmock.NewResult("alter", 1))
		mock.ExpectExec("select pg_reload_conf()").
			WillReturnResult(pgxmock.NewResult("select", 1))
		err := manager.Unlock(ctx)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}
