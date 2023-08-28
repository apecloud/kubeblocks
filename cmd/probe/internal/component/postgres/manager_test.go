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

	"github.com/dapr/kit/logger"
	"github.com/pashagolub/pgxmock/v2"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/internal/constant"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
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

	manager, err := NewManager(logger.NewLogger("test"))
	if err != nil {
		t.Fatal(err)
	}
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

		if ok := manager.WriteCheck(ctx, ""); !ok {
			t.Errorf("write check failed")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}
	})

	t.Run("write check failed", func(t *testing.T) {
		mock.ExpectExec(`create table if not exists`).
			WillReturnError(fmt.Errorf("some error"))

		if ok := manager.WriteCheck(ctx, ""); ok {
			t.Errorf("expect write check failed, but success")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}
	})

	t.Run("read check", func(t *testing.T) {
		row := pgxmock.NewRows([]string{"check_ts"}).AddRow(1)
		mock.ExpectQuery("select").WillReturnRows(row)

		if ok := manager.ReadCheck(ctx, ""); !ok {
			t.Errorf("read check failed")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}
	})
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

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}
	})

	t.Run("pg is not ready", func(t *testing.T) {
		mock.ExpectPing().WillReturnError(fmt.Errorf("can't ping to db"))
		if isReady := manager.IsPgReady(ctx); isReady {
			t.Errorf("expect pg is not ready, but get ready")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}
	})
}
