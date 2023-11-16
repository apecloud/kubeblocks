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
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
)

const (
	fakePodName         = "fake-mysql-0"
	fakeClusterCompName = "test-mysql"
	fakeNamespace       = "fake-namespace"
	fakeDBPort          = "fake-port"
)

func TestNewManager(t *testing.T) {
	defer viper.Reset()

	t.Run("new config failed", func(t *testing.T) {
		manager, err := NewManager(fakePropertiesWithPem)

		assert.Nil(t, manager)
		assert.NotNil(t, err)
	})

	t.Run("new db manager base failed", func(t *testing.T) {
		manager, err := NewManager(fakeProperties)

		assert.Nil(t, manager)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "KB_POD_NAME is not set")
	})

	viper.Set(constant.KBEnvPodName, "fake")
	viper.Set(constant.KBEnvClusterCompName, fakeClusterCompName)
	viper.Set(constant.KBEnvNamespace, fakeNamespace)
	t.Run("get server id failed", func(t *testing.T) {
		manager, err := NewManager(fakeProperties)

		assert.Nil(t, manager)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "the format of member name is wrong")
	})

	viper.Set(constant.KBEnvPodName, fakePodName)
	t.Run("get local connection failed", func(t *testing.T) {
		manager, err := NewManager(fakePropertiesWithWrongURL)

		assert.Nil(t, manager)
		assert.NotNil(t, err)
	})

	t.Run("new manager successfully", func(t *testing.T) {
		managerIFace, err := NewManager(fakeProperties)
		assert.Nil(t, err)

		manager, ok := managerIFace.(*Manager)
		assert.True(t, ok)
		assert.Equal(t, fakePodName, manager.CurrentMemberName)
		assert.Equal(t, fakeNamespace, manager.Namespace)
		assert.Equal(t, fakeClusterCompName, manager.ClusterCompName)
		assert.Equal(t, uint(1), manager.serverID)
	})
}

func TestManager_IsRunning(t *testing.T) {
	manager, mock, _ := mockDatabase(t)

	t.Run("Too many connections", func(t *testing.T) {
		mock.ExpectPing().
			WillReturnError(&mysql.MySQLError{Number: 1040})

		isRunning := manager.IsRunning()
		assert.True(t, isRunning)
	})

	t.Run("DB is not ready", func(t *testing.T) {
		mock.ExpectPing().
			WillReturnError(fmt.Errorf("some error"))

		isRunning := manager.IsRunning()
		assert.False(t, isRunning)
	})

	t.Run("ping db overtime", func(t *testing.T) {
		mock.ExpectPing().WillDelayFor(time.Second)

		isRunning := manager.IsRunning()
		assert.False(t, isRunning)
	})

	t.Run("db is running", func(t *testing.T) {
		mock.ExpectPing()

		isRunning := manager.IsRunning()
		assert.True(t, isRunning)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_IsDBStartupReady(t *testing.T) {
	manager, mock, _ := mockDatabase(t)

	t.Run("db has start up", func(t *testing.T) {
		manager.DBStartupReady = true
		defer func() {
			manager.DBStartupReady = false
		}()

		dbReady := manager.IsDBStartupReady()
		assert.True(t, dbReady)
	})

	t.Run("ping db failed", func(t *testing.T) {
		mock.ExpectPing().WillDelayFor(time.Second)

		dbReady := manager.IsDBStartupReady()
		assert.False(t, dbReady)
	})

	t.Run("check db start up successfully", func(t *testing.T) {
		mock.ExpectPing()

		dbReady := manager.IsDBStartupReady()
		assert.True(t, dbReady)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_IsReadonly(t *testing.T) {
	ctx := context.TODO()
	manager, _, _ := mockDatabase(t)
	cluster := &dcs.Cluster{}

	t.Run("Get Member conn failed", func(t *testing.T) {
		_, _ = NewConfig(fakePropertiesWithWrongURL)

		readonly, err := manager.IsReadonly(ctx, cluster, &dcs.Member{})
		assert.False(t, readonly)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "illegal Data Source Name (DNS) specified by")
	})
}

func TestManager_IsLeader(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	t.Run("check is read only failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		isLeader, err := manager.IsLeader(ctx, nil)
		assert.False(t, isLeader)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("current member is leader", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(sqlmock.NewRows([]string{"@@global.hostname", "@@global.version", "@@global.read_only",
				"@@global.binlog_format", "@@global.log_bin", "@@global.log_slave_updates"}).
				AddRow(fakePodName, "8.0.30", false, "MIXED", "1", "1"))

		isLeader, err := manager.IsLeader(ctx, nil)
		assert.True(t, isLeader)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_IsLeaderMember(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	t.Run("check is read only failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		isLeader, err := manager.IsLeaderMember(ctx, nil, nil)
		assert.False(t, isLeader)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("current member is leader", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(sqlmock.NewRows([]string{"@@global.hostname", "@@global.version", "@@global.read_only",
				"@@global.binlog_format", "@@global.log_bin", "@@global.log_slave_updates"}).
				AddRow(fakePodName, "8.0.30", false, "MIXED", "1", "1"))

		isLeader, err := manager.IsLeaderMember(ctx, nil, nil)
		assert.True(t, isLeader)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_GetMemberAddrs(t *testing.T) {
	ctx := context.TODO()
	manager, _, _ := mockDatabase(t)
	cluster := &dcs.Cluster{
		Members: []dcs.Member{
			{
				Name:   fakePodName,
				DBPort: fakeDBPort,
			},
		},
		Namespace: fakeNamespace,
	}

	viper.Set(constant.KubernetesClusterDomainEnv, "cluster.local")
	defer viper.Reset()
	addrs := manager.GetMemberAddrs(ctx, cluster)
	assert.Len(t, addrs, 1)
	assert.Equal(t, "fake-mysql-0.-headless.fake-namespace.svc.cluster.local:fake-port", addrs[0])
}

func TestManager_IsMemberLagging(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)
	cluster := &dcs.Cluster{Leader: &dcs.Leader{}, HaConfig: &dcs.HaConfig{}}

	t.Run("No leader DBState info", func(t *testing.T) {
		isMemberLagging, lags := manager.IsMemberLagging(ctx, cluster, nil)
		assert.False(t, isMemberLagging)
		assert.Zero(t, lags)
	})

	cluster.Leader.DBState = &dcs.DBState{}
	t.Run("Get Member conn failed", func(t *testing.T) {
		_, _ = NewConfig(fakePropertiesWithWrongURL)

		isMemberLagging, lags := manager.IsMemberLagging(ctx, cluster, &dcs.Member{})
		assert.False(t, isMemberLagging)
		assert.Zero(t, lags)
	})

	_, _ = NewConfig(fakeProperties)
	t.Run("get op timestamp failed", func(t *testing.T) {
		mock.ExpectQuery("select check_ts").
			WillReturnError(fmt.Errorf("some error"))

		isMemberLagging, lags := manager.IsMemberLagging(ctx, cluster, &dcs.Member{Name: fakePodName})
		assert.False(t, isMemberLagging)
		assert.Zero(t, lags)
	})

	cluster.Leader.DBState.OpTimestamp = 100
	t.Run("no lags", func(t *testing.T) {

		mock.ExpectQuery("select check_ts").
			WillReturnRows(sqlmock.NewRows([]string{"check_ts"}).AddRow(100))

		isMemberLagging, lags := manager.IsMemberLagging(ctx, cluster, &dcs.Member{Name: fakePodName})
		assert.False(t, isMemberLagging)
		assert.Zero(t, lags)
	})

	t.Run("member is lagging", func(t *testing.T) {
		mock.ExpectQuery("select check_ts").
			WillReturnRows(sqlmock.NewRows([]string{"check_ts"}).AddRow(0))

		isMemberLagging, lags := manager.IsMemberLagging(ctx, cluster, &dcs.Member{Name: fakePodName})
		assert.True(t, isMemberLagging)
		assert.Equal(t, int64(100), lags)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_IsMemberHealthy(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)
	member := dcs.Member{Name: fakePodName}
	cluster := &dcs.Cluster{
		Leader:  &dcs.Leader{Name: fakePodName},
		Members: []dcs.Member{member},
	}

	t.Run("Get Member conn failed", func(t *testing.T) {
		_, _ = NewConfig(fakePropertiesWithWrongURL)

		isHealthy := manager.IsMemberHealthy(ctx, cluster, &dcs.Member{})
		assert.False(t, isHealthy)
	})

	_, _ = NewConfig(fakeProperties)
	t.Run("write check failed", func(t *testing.T) {
		mock.ExpectExec("CREATE DATABASE IF NOT EXISTS kubeblocks").
			WillReturnError(fmt.Errorf("some error"))

		isHealthy := manager.IsCurrentMemberHealthy(ctx, cluster)
		assert.False(t, isHealthy)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_WriteCheck(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	t.Run("write check failed", func(t *testing.T) {
		mock.ExpectExec("CREATE DATABASE IF NOT EXISTS kubeblocks;").
			WillReturnError(fmt.Errorf("some error"))

		canWrite := manager.WriteCheck(ctx, manager.DB)
		assert.False(t, canWrite)
	})

	t.Run("write check successfully", func(t *testing.T) {
		mock.ExpectExec("CREATE DATABASE IF NOT EXISTS kubeblocks;").
			WillReturnResult(sqlmock.NewResult(1, 1))

		canWrite := manager.WriteCheck(ctx, manager.DB)
		assert.True(t, canWrite)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_ReadCheck(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	t.Run("no rows in result set", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(sql.ErrNoRows)

		canRead := manager.ReadCheck(ctx, manager.DB)
		assert.True(t, canRead)
	})

	t.Run("no healthy database", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(&mysql.MySQLError{Number: 1049})

		canRead := manager.ReadCheck(ctx, manager.DB)
		assert.True(t, canRead)
	})

	t.Run("Read check failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		canRead := manager.ReadCheck(ctx, manager.DB)
		assert.False(t, canRead)
	})

	t.Run("Read check successfully", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(sqlmock.NewRows([]string{"check_ts"}).AddRow(1))

		canRead := manager.ReadCheck(ctx, manager.DB)
		assert.True(t, canRead)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_GetGlobalState(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	t.Run("get global state successfully", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(sqlmock.NewRows([]string{"@@global.hostname", "@@global.server_uuid", "@@global.gtid_executed", "@@global.gtid_purged", "@@global.read_only", "@@global.super_read_only"}).
				AddRow(fakePodName, fakeServerUUID, fakeGTIDString, fakeGTIDSet, 1, 1))

		globalState, err := manager.GetGlobalState(ctx, manager.DB)
		assert.Nil(t, err)
		assert.NotNil(t, globalState)
		assert.Equal(t, fakePodName, globalState["hostname"])
		assert.Equal(t, fakeServerUUID, globalState["server_uuid"])
		assert.Equal(t, fakeGTIDString, globalState["gtid_executed"])
		assert.Equal(t, fakeGTIDSet, globalState["gtid_purged"])
		assert.Equal(t, "1", globalState["read_only"])
		assert.Equal(t, "1", globalState["super_read_only"])
	})

	t.Run("get global state failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		globalState, err := manager.GetGlobalState(ctx, manager.DB)
		assert.Nil(t, globalState)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_GetSlaveStatus(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	t.Run("query rows map failed", func(t *testing.T) {
		mock.ExpectQuery("show slave status").
			WillReturnError(fmt.Errorf("some error"))

		slaveStatus, err := manager.GetSlaveStatus(ctx, manager.DB)
		assert.Nil(t, slaveStatus)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("get slave status successfully", func(t *testing.T) {
		mock.ExpectQuery("show slave status").
			WillReturnRows(sqlmock.NewRows([]string{"Seconds_Behind_Master", "Slave_IO_Running"}).AddRow("249904", "Yes"))

		slaveStatus, err := manager.GetSlaveStatus(ctx, manager.DB)
		assert.Nil(t, err)
		assert.Equal(t, "249904", slaveStatus.GetString("Seconds_Behind_Master"))
		assert.Equal(t, "Yes", slaveStatus.GetString("Slave_IO_Running"))
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_GetMasterStatus(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	t.Run("query rows map failed", func(t *testing.T) {
		mock.ExpectQuery("show master status").
			WillReturnError(fmt.Errorf("some error"))

		slaveStatus, err := manager.GetMasterStatus(ctx, manager.DB)
		assert.Nil(t, slaveStatus)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("get slave status successfully", func(t *testing.T) {
		mock.ExpectQuery("show master status").
			WillReturnRows(sqlmock.NewRows([]string{"File", "Executed_Gtid_Set"}).AddRow("master-bin.000002", fakeGTIDSet))

		slaveStatus, err := manager.GetMasterStatus(ctx, manager.DB)
		assert.Nil(t, err)
		assert.Equal(t, "master-bin.000002", slaveStatus.GetString("File"))
		assert.Equal(t, fakeGTIDSet, slaveStatus.GetString("Executed_Gtid_Set"))
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_IsClusterInitialized(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)
	manager.serverID = 1

	t.Run("query server id failed", func(t *testing.T) {
		mock.ExpectQuery("select @@global.server_id").
			WillReturnError(fmt.Errorf("some error"))

		isInitialized, err := manager.IsClusterInitialized(ctx, nil)
		assert.False(t, isInitialized)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("server id equal to manager's server id", func(t *testing.T) {
		mock.ExpectQuery("select @@global.server_id").
			WillReturnRows(sqlmock.NewRows([]string{"@@global.server_id"}).AddRow(1))

		isInitialized, err := manager.IsClusterInitialized(ctx, nil)
		assert.True(t, isInitialized)
		assert.Nil(t, err)
	})

	t.Run("set server id failed", func(t *testing.T) {
		mock.ExpectQuery("select @@global.server_id").
			WillReturnRows(sqlmock.NewRows([]string{"@@global.server_id"}).AddRow(2))
		mock.ExpectExec("set global server_id").
			WillReturnError(fmt.Errorf("some error"))

		isInitialized, err := manager.IsClusterInitialized(ctx, nil)
		assert.False(t, isInitialized)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("set server id successfully", func(t *testing.T) {
		mock.ExpectQuery("select @@global.server_id").
			WillReturnRows(sqlmock.NewRows([]string{"@@global.server_id"}).AddRow(2))
		mock.ExpectExec("set global server_id").
			WillReturnResult(sqlmock.NewResult(1, 1))

		isInitialized, err := manager.IsClusterInitialized(ctx, nil)
		assert.True(t, isInitialized)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_Promote(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)
	manager.globalState = map[string]string{}
	manager.slaveStatus = RowMap{}

	t.Run("execute promote failed", func(t *testing.T) {
		mock.ExpectExec("set global read_only=off").
			WillReturnError(fmt.Errorf("some error"))

		err := manager.Promote(ctx, nil)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("execute promote successfully", func(t *testing.T) {
		mock.ExpectExec("set global read_only=off").
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := manager.Promote(ctx, nil)
		assert.Nil(t, err)
	})

	t.Run("current member has been promoted", func(t *testing.T) {
		manager.globalState["super_read_only"] = "0"
		manager.globalState["read_only"] = "0"

		err := manager.Promote(ctx, nil)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_Demote(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	t.Run("execute promote failed", func(t *testing.T) {
		mock.ExpectExec("set global read_only=on").
			WillReturnError(fmt.Errorf("some error"))

		err := manager.Demote(ctx)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("execute promote successfully", func(t *testing.T) {
		mock.ExpectExec("set global read_only=on").
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := manager.Demote(ctx)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_Follow(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)
	_, _ = NewConfig(fakeProperties)
	cluster := &dcs.Cluster{
		Members: []dcs.Member{
			{Name: fakePodName},
			{Name: "fake-pod-2"},
			{Name: "fake-pod-1"},
		},
	}

	t.Run("cluster has no leader", func(t *testing.T) {
		err := manager.Follow(ctx, cluster)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "cluster has no leader")
	})

	t.Run("i get the leader key, don't need to follow", func(t *testing.T) {
		cluster.Leader = &dcs.Leader{Name: manager.CurrentMemberName}

		err := manager.Follow(ctx, cluster)
		assert.Nil(t, err)
	})

	cluster.Leader = &dcs.Leader{Name: "fake-pod-1"}
	t.Run("recovery conf still right", func(t *testing.T) {
		manager.slaveStatus = RowMap{
			"Master_Host": CellData{
				String: "fake-pod-1",
			},
		}

		err := manager.Follow(ctx, cluster)
		assert.Nil(t, err)
	})

	manager.slaveStatus = RowMap{
		"Master_Host": CellData{
			String: "fake-pod-2",
		},
	}
	t.Run("execute follow failed", func(t *testing.T) {
		mock.ExpectExec("stop slave").
			WillReturnError(fmt.Errorf("some error"))

		err := manager.Follow(ctx, cluster)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("execute follow successfully", func(t *testing.T) {
		mock.ExpectExec("stop slave").
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := manager.Follow(ctx, cluster)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_isRecoveryConfOutdated(t *testing.T) {
	manager, _, _ := mockDatabase(t)
	manager.slaveStatus = RowMap{}

	t.Run("slaveStatus empty", func(t *testing.T) {
		outdated := manager.isRecoveryConfOutdated(fakePodName)
		assert.True(t, outdated)
	})

	t.Run("slave status error", func(t *testing.T) {
		manager.slaveStatus = RowMap{
			"Last_IO_Error": CellData{String: "some error"},
		}

		outdated := manager.isRecoveryConfOutdated(fakePodName)
		assert.True(t, outdated)
	})
}

func TestManager_HasOtherHealthyMembers(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	cluster := &dcs.Cluster{
		Members: []dcs.Member{
			{
				Name: "fake-pod-0",
			},
			{
				Name: "fake-pod-1",
			},
			{
				Name: fakePodName,
			},
		},
	}
	mock.ExpectQuery("select check_ts from kubeblocks.kb_health_check where type=1 limit 1").
		WillReturnError(sql.ErrNoRows)
	_, _ = NewConfig(fakeProperties)

	members := manager.HasOtherHealthyMembers(ctx, cluster, "fake-pod-0")
	assert.Len(t, members, 1)
	assert.Equal(t, fakePodName, members[0].Name)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_Lock(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	t.Run("lock failed", func(t *testing.T) {
		mock.ExpectExec("set global read_only=on").
			WillReturnError(fmt.Errorf("some error"))

		err := manager.Lock(ctx, "")
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
		assert.False(t, manager.IsLocked)
	})

	t.Run("lock successfully", func(t *testing.T) {
		mock.ExpectExec("set global read_only=on").
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := manager.Lock(ctx, "")
		assert.Nil(t, err)
		assert.True(t, manager.IsLocked)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_Unlock(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)
	manager.IsLocked = true

	t.Run("unlock failed", func(t *testing.T) {
		mock.ExpectExec("set global read_only=off").
			WillReturnError(fmt.Errorf("some error"))

		err := manager.Unlock(ctx)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
		assert.True(t, manager.IsLocked)
	})

	t.Run("lock successfully", func(t *testing.T) {
		mock.ExpectExec("set global read_only=off").
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := manager.Unlock(ctx)
		assert.Nil(t, err)
		assert.False(t, manager.IsLocked)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_GetDBState(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)
	cluster := &dcs.Cluster{
		Leader: &dcs.Leader{},
	}

	t.Run("select global failed", func(t *testing.T) {
		mock.ExpectQuery("select  @@global.hostname").
			WillReturnError(fmt.Errorf("some error"))

		dbState := manager.GetDBState(ctx, cluster)
		assert.Nil(t, dbState)
	})

	t.Run("show master status failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(sqlmock.NewRows([]string{"@@global.hostname", "@@global.server_uuid", "@@global.gtid_executed", "@@global.gtid_purged", "@@global.read_only", "@@global.super_read_only"}).
				AddRow(fakePodName, fakeServerUUID, fakeGTIDString, fakeGTIDSet, 1, 1))
		mock.ExpectQuery("show master status").
			WillReturnError(fmt.Errorf("some error"))

		dbState := manager.GetDBState(ctx, cluster)
		assert.Nil(t, dbState)
	})

	t.Run("show slave status failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(sqlmock.NewRows([]string{"@@global.hostname", "@@global.server_uuid", "@@global.gtid_executed", "@@global.gtid_purged", "@@global.read_only", "@@global.super_read_only"}).
				AddRow(fakePodName, fakeServerUUID, fakeGTIDString, fakeGTIDSet, 1, 1))
		mock.ExpectQuery("show master status").
			WillReturnRows(sqlmock.NewRows([]string{"Binlog_File", "Binlog_Pos"}).AddRow("master-bin.000002", 20))
		mock.ExpectQuery("show slave status").
			WillReturnError(fmt.Errorf("some error"))

		dbState := manager.GetDBState(ctx, cluster)
		assert.Nil(t, dbState)
	})

	t.Run("get op timestamp failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(sqlmock.NewRows([]string{"@@global.hostname", "@@global.server_uuid", "@@global.gtid_executed", "@@global.gtid_purged", "@@global.read_only", "@@global.super_read_only"}).
				AddRow(fakePodName, fakeServerUUID, fakeGTIDString, fakeGTIDSet, 1, 1))
		mock.ExpectQuery("show master status").
			WillReturnRows(sqlmock.NewRows([]string{"Binlog_File", "Binlog_Pos"}).AddRow("master-bin.000002", 20))
		mock.ExpectQuery("show slave status").
			WillReturnRows(sqlmock.NewRows([]string{"Master_UUID", "Slave_IO_Running"}).AddRow(fakeServerUUID, "Yes"))
		mock.ExpectQuery("select check_ts").
			WillReturnError(fmt.Errorf("some error"))

		dbState := manager.GetDBState(ctx, cluster)
		assert.Nil(t, dbState)
	})

	t.Run("current member is leader", func(t *testing.T) {
		cluster.Leader.Name = manager.CurrentMemberName
		mock.ExpectQuery("select").
			WillReturnRows(sqlmock.NewRows([]string{"@@global.hostname", "@@global.server_uuid", "@@global.gtid_executed", "@@global.gtid_purged", "@@global.read_only", "@@global.super_read_only"}).
				AddRow(fakePodName, fakeServerUUID, fakeGTIDString, fakeGTIDSet, 1, 1))
		mock.ExpectQuery("show master status").
			WillReturnRows(sqlmock.NewRows([]string{"File", "Pos"}).AddRow("master-bin.000002", 20))
		mock.ExpectQuery("show slave status").
			WillReturnRows(sqlmock.NewRows([]string{"Master_UUID", "Slave_IO_Running"}).AddRow(fakeServerUUID, "Yes"))
		mock.ExpectQuery("select").
			WillReturnRows(sqlmock.NewRows([]string{"check_ts"}).AddRow(1))

		dbState := manager.GetDBState(ctx, cluster)
		assert.NotNil(t, dbState)
		assert.Equal(t, fakePodName, dbState.Extra["hostname"])
		assert.Equal(t, "master-bin.000002", dbState.Extra["Binlog_File"])
	})

	t.Run("current member is not leader", func(t *testing.T) {
		cluster.Leader.Name = ""
		mock.ExpectQuery("select").
			WillReturnRows(sqlmock.NewRows([]string{"@@global.hostname", "@@global.server_uuid", "@@global.gtid_executed", "@@global.gtid_purged", "@@global.read_only", "@@global.super_read_only"}).
				AddRow(fakePodName, fakeServerUUID, fakeGTIDString, fakeGTIDSet, 1, 1))
		mock.ExpectQuery("show master status").
			WillReturnRows(sqlmock.NewRows([]string{"File", "Pos"}).AddRow("master-bin.000002", 20))
		mock.ExpectQuery("show slave status").
			WillReturnRows(sqlmock.NewRows([]string{"Master_UUID", "Slave_IO_Running"}).AddRow(fakeServerUUID, "Yes"))
		mock.ExpectQuery("select").
			WillReturnRows(sqlmock.NewRows([]string{"check_ts"}).AddRow(1))

		dbState := manager.GetDBState(ctx, cluster)
		assert.NotNil(t, dbState)
		assert.Equal(t, fakePodName, dbState.Extra["hostname"])
		assert.Equal(t, fakeServerUUID, dbState.Extra["Master_UUID"])
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}
