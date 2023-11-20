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

package wesql

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/mysql"
)

const (
	fakePodName         = "test-wesql-0"
	fakeClusterCompName = "test-wesql"
	fakeNamespace       = "fake-namespace"
)

func mockDatabase(t *testing.T) (*Manager, sqlmock.Sqlmock, error) {
	manager := &Manager{
		mysql.Manager{
			DBManagerBase: engines.DBManagerBase{
				CurrentMemberName: fakePodName,
				ClusterCompName:   fakeClusterCompName,
				Namespace:         fakeNamespace,
				Logger:            ctrl.Log.WithName("WeSQL-TEST"),
			},
		},
	}

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	manager.DB = db

	return manager, mock, err
}

func TestNewManager(t *testing.T) {
	t.Run("new config failed", func(t *testing.T) {
		manager, err := NewManager(fakePropertiesWithWrongPem)

		assert.Nil(t, manager)
		assert.NotNil(t, err)
	})

	t.Run("new mysql manager failed", func(t *testing.T) {
		manager, err := NewManager(fakeProperties)

		assert.Nil(t, manager)
		assert.NotNil(t, err)
	})

	viper.Set(constant.KBEnvPodName, fakePodName)
	defer viper.Reset()
	t.Run("new manger successfully", func(t *testing.T) {
		manager, err := NewManager(fakeProperties)

		assert.Nil(t, err)
		assert.NotNil(t, manager)
	})
}

func TestIsLeader(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	t.Run("get role failed", func(t *testing.T) {
		mock.ExpectQuery("select *").
			WillReturnError(fmt.Errorf("some error"))

		isLeader, err := manager.IsLeader(ctx, nil)
		assert.False(t, isLeader)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("get role leader", func(t *testing.T) {
		mock.ExpectQuery("select CURRENT_LEADER, ROLE, SERVER_ID from information_schema.wesql_cluster_local").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER", "ROLE", "SERVER_ID"}).AddRow("test-wesql-0", "leader", "1"))

		isLeader, err := manager.IsLeader(ctx, nil)
		assert.True(t, isLeader)
		assert.Nil(t, err)
	})

	t.Run("get role follower", func(t *testing.T) {
		mock.ExpectQuery("select CURRENT_LEADER, ROLE, SERVER_ID from information_schema.wesql_cluster_local").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER", "ROLE", "SERVER_ID"}).AddRow("test-wesql-1", "follower", "2"))

		isLeader, err := manager.IsLeader(ctx, nil)
		assert.False(t, isLeader)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestIsLeaderMember(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)
	cluster := &dcs.Cluster{
		Members: []dcs.Member{
			{
				Name: "test-wesql-1",
			},
		},
	}

	t.Run("member is nil", func(t *testing.T) {
		isLeaderMember, err := manager.IsLeaderMember(ctx, cluster, nil)
		assert.False(t, isLeaderMember)
		assert.Nil(t, err)
	})

	member := &dcs.Member{
		Name: fakePodName,
	}
	t.Run("leader member is nil", func(t *testing.T) {
		mock.ExpectQuery("select *").
			WillReturnError(fmt.Errorf("some error"))

		isLeaderMember, err := manager.IsLeaderMember(ctx, cluster, member)
		assert.False(t, isLeaderMember)
		assert.Nil(t, err)
	})

	t.Run("member is not Leader member", func(t *testing.T) {
		mock.ExpectQuery("select *").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER"}).AddRow("test-wesql-1.test-wesql.headless"))

		isLeaderMember, err := manager.IsLeaderMember(ctx, cluster, member)
		assert.False(t, isLeaderMember)
		assert.Nil(t, err)
	})

	cluster.Members = append(cluster.Members, *member)
	t.Run("member is Leader member", func(t *testing.T) {
		mock.ExpectQuery("select *").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER"}).AddRow(fakePodName + ".test-wesql.headless"))

		isLeaderMember, err := manager.IsLeaderMember(ctx, cluster, member)
		assert.True(t, isLeaderMember)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestGetClusterInfo(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	t.Run("Get leader conn failed", func(t *testing.T) {
		clusterInfo := manager.GetClusterInfo(ctx, &dcs.Cluster{})
		assert.Empty(t, clusterInfo)
	})

	t.Run("get cluster info failed", func(t *testing.T) {
		mock.ExpectQuery("select cluster_id, cluster_info from mysql.consensus_info").
			WillReturnError(fmt.Errorf("some error"))

		clusterInfo := manager.GetClusterInfo(ctx, nil)
		assert.Empty(t, clusterInfo)
	})

	t.Run("get cluster info success", func(t *testing.T) {
		mock.ExpectQuery("select cluster_id, cluster_info from mysql.consensus_info").
			WillReturnRows(sqlmock.NewRows([]string{"cluster_id", "cluster_info"}).
				AddRow("1", "test-wesql-0.test-wesql-headless:13306;test-wesql-1.test-wesql-headless:13306@1"))

		clusterInfo := manager.GetClusterInfo(ctx, nil)
		assert.Equal(t, "test-wesql-0.test-wesql-headless:13306;test-wesql-1.test-wesql-headless:13306@1", clusterInfo)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestGetMemberAddrs(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	mock.ExpectQuery("select cluster_id, cluster_info from mysql.consensus_info").
		WillReturnRows(sqlmock.NewRows([]string{"cluster_id", "cluster_info"}).
			AddRow("1", "test-wesql-0.test-wesql-headless:13306;test-wesql-1.test-wesql-headless:13306;test-wesql-2.test-wesql-headless@1"))

	addrs := manager.GetMemberAddrs(ctx, nil)
	assert.Equal(t, 2, len(addrs))
	assert.Equal(t, "test-wesql-0.test-wesql-headless:13306", addrs[0])
	assert.Equal(t, "test-wesql-1.test-wesql-headless:13306", addrs[1])

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestGetAddrWithMemberName(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	memberNames := []string{"test-wesql-0", "test-wesql-2"}
	expectAddrs := []string{"test-wesql-0.test-wesql-headless:13306", ""}
	for i, name := range memberNames {
		mock.ExpectQuery("select cluster_id, cluster_info from mysql.consensus_info").
			WillReturnRows(sqlmock.NewRows([]string{"cluster_id", "cluster_info"}).
				AddRow("1", "test-wesql-0.test-wesql-headless:13306;test-wesql-1.test-wesql-headless:13306;@1"))

		addr := manager.GetAddrWithMemberName(ctx, nil, name)
		assert.Equal(t, expectAddrs[i], addr)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestIsCurrentMemberInCluster(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	mock.ExpectQuery("select cluster_id, cluster_info from mysql.consensus_info").
		WillReturnRows(sqlmock.NewRows([]string{"cluster_id", "cluster_info"}).
			AddRow("1", "test-wesql-0.test-wesql-headless:13306;test-wesql-1.test-wesql-headless:13306@1"))

	assert.True(t, manager.IsCurrentMemberInCluster(ctx, nil))

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_LeaveMemberFromCluster(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)
	cluster := &dcs.Cluster{
		ClusterCompName: fakeClusterCompName,
		Namespace:       fakeNamespace,
	}
	memberName := fakePodName

	t.Run("Get leader conn failed", func(t *testing.T) {
		err := manager.LeaveMemberFromCluster(ctx, cluster, memberName)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "the cluster has no leader")
	})

	cluster.Leader = &dcs.Leader{Name: fakePodName}
	cluster.Members = []dcs.Member{{Name: fakePodName}}
	t.Run("member already deleted", func(t *testing.T) {
		err := manager.LeaveMemberFromCluster(ctx, cluster, memberName)
		assert.Nil(t, err)
	})

	t.Run("delete member from db cluster failed", func(t *testing.T) {
		mock.ExpectQuery("select cluster_id, cluster_info from mysql.consensus_info").
			WillReturnRows(sqlmock.NewRows([]string{"cluster_id", "cluster_info"}).
				AddRow("1", "test-wesql-0.test-wesql-headless:13306;test-wesql-1.test-wesql-headless:13306@1"))
		mock.ExpectExec("call dbms_consensus").
			WillReturnError(fmt.Errorf("some error"))

		err := manager.LeaveMemberFromCluster(ctx, cluster, memberName)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("delete member successfully", func(t *testing.T) {
		mock.ExpectQuery("select cluster_id, cluster_info from mysql.consensus_info").
			WillReturnRows(sqlmock.NewRows([]string{"cluster_id", "cluster_info"}).
				AddRow("1", "test-wesql-0.test-wesql-headless:13306;test-wesql-1.test-wesql-headless:13306@1"))
		mock.ExpectExec("call dbms_consensus").
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := manager.LeaveMemberFromCluster(ctx, cluster, memberName)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestManager_IsClusterHealthy(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)
	cluster := &dcs.Cluster{}

	t.Run("Get leader conn failed", func(t *testing.T) {
		isHealthy := manager.IsClusterHealthy(ctx, cluster)
		assert.False(t, isHealthy)
	})

	cluster.Leader = &dcs.Leader{Name: fakePodName}
	cluster.Members = []dcs.Member{{Name: fakePodName}}
	t.Run("get wesql cluster information failed", func(t *testing.T) {
		mock.ExpectQuery("select *").
			WillReturnError(fmt.Errorf("some error"))

		isHealthy := manager.IsClusterHealthy(ctx, cluster)
		assert.False(t, isHealthy)
	})

	t.Run("check cluster healthy status successfully", func(t *testing.T) {
		roles := []string{Leader, "Follow"}
		expectedRes := []bool{true, false}

		for i, role := range roles {
			mock.ExpectQuery("select *").
				WillReturnRows(sqlmock.NewRows([]string{Role}).AddRow(role))

			isHealthy := manager.IsClusterHealthy(ctx, cluster)
			assert.Equal(t, expectedRes[i], isHealthy)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestIsClusterInitialized(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	t.Run("cluster is initialized", func(t *testing.T) {
		mock.ExpectQuery("select cluster_id, cluster_info from mysql.consensus_info").
			WillReturnRows(sqlmock.NewRows([]string{"cluster_id", "cluster_info"}).
				AddRow("1", "test-wesql-0.test-wesql-headless:13306;test-wesql-1.test-wesql-headless:13306@1"))

		isInitialized, err := manager.IsClusterInitialized(ctx, nil)
		assert.True(t, isInitialized)
		assert.Nil(t, err)
	})

	t.Run("cluster is not initialized", func(t *testing.T) {
		mock.ExpectQuery("select cluster_id, cluster_info from mysql.consensus_info").
			WillReturnRows(sqlmock.NewRows([]string{"cluster_id", "cluster_info"}))

		isInitialized, err := manager.IsClusterInitialized(ctx, nil)
		assert.False(t, isInitialized)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestIsPromoted(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	mock.ExpectQuery("select CURRENT_LEADER, ROLE, SERVER_ID from information_schema.wesql_cluster_local").
		WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER", "ROLE", "SERVER_ID"}).AddRow("test-wesql-0", "leader", "1"))

	assert.True(t, manager.IsPromoted(ctx))

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestHasOtherHealthyLeader(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)
	cluster := &dcs.Cluster{
		Members: []dcs.Member{},
	}

	t.Run("Get cluster local info failed", func(t *testing.T) {
		mock.ExpectQuery("select *").
			WillReturnError(fmt.Errorf("some error"))

		member := manager.HasOtherHealthyLeader(ctx, cluster)
		assert.Nil(t, member)
	})

	t.Run("current member is leader", func(t *testing.T) {
		mock.ExpectQuery("select *").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER", "ROLE"}).AddRow(fakePodName, Leader))

		member := manager.HasOtherHealthyLeader(ctx, cluster)
		assert.Nil(t, member)
	})

	t.Run("leader addr is empty", func(t *testing.T) {
		mock.ExpectQuery("select *").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER", "ROLE"}).AddRow("", "follow"))

		member := manager.HasOtherHealthyLeader(ctx, cluster)
		assert.Nil(t, member)
	})

	t.Run("member is not in the cluster", func(t *testing.T) {
		mock.ExpectQuery("select *").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER", "ROLE"}).AddRow(fakePodName, "follow"))

		member := manager.HasOtherHealthyLeader(ctx, cluster)
		assert.Nil(t, member)
	})

	cluster.Members = append(cluster.Members, dcs.Member{
		Name: fakePodName,
	})
	t.Run("get other healthy leader", func(t *testing.T) {
		mock.ExpectQuery("select *").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER", "ROLE"}).AddRow(fakePodName+".test-wesql-headless", "follow"))

		member := manager.HasOtherHealthyLeader(ctx, cluster)
		assert.NotNil(t, member)
		assert.Equal(t, fakePodName, member.Name)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestHasOtherHealthyMembers(t *testing.T) {
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

func TestManager_Promote(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)
	cluster := &dcs.Cluster{}

	t.Run("current member is leader", func(t *testing.T) {
		mock.ExpectQuery("select CURRENT_LEADER, ROLE, SERVER_ID from information_schema.wesql_cluster_local").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER", "ROLE", "SERVER_ID"}).AddRow("test-wesql-0", "leader", "1"))

		err := manager.Promote(ctx, cluster)
		assert.Nil(t, err)
	})

	t.Run("Get leader conn failed", func(t *testing.T) {
		mock.ExpectQuery("select CURRENT_LEADER, ROLE, SERVER_ID from information_schema.wesql_cluster_local").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER", "ROLE", "SERVER_ID"}).AddRow("test-wesql-0", "follower", "1"))

		err := manager.Promote(ctx, cluster)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "Get leader conn failed")
	})

	cluster.Leader = &dcs.Leader{Name: fakePodName}
	cluster.Members = []dcs.Member{{Name: fakePodName}}
	t.Run("get addr failed", func(t *testing.T) {
		mock.ExpectQuery("select CURRENT_LEADER, ROLE, SERVER_ID from information_schema.wesql_cluster_local").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER", "ROLE", "SERVER_ID"}).AddRow("test-wesql-0", "follower", "1"))
		mock.ExpectQuery("select cluster_id, cluster_info from mysql.consensus_info").
			WillReturnRows(sqlmock.NewRows([]string{"cluster_id", "cluster_info"}).AddRow("1", "test-wesql-1.test-wesql-headless:13306;"))

		err := manager.Promote(ctx, cluster)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "get current member's addr failed")
	})

	t.Run("promote failed", func(t *testing.T) {
		mock.ExpectQuery("select CURRENT_LEADER, ROLE, SERVER_ID from information_schema.wesql_cluster_local").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER", "ROLE", "SERVER_ID"}).AddRow("test-wesql-0", "follower", "1"))
		mock.ExpectQuery("select cluster_id, cluster_info from mysql.consensus_info").
			WillReturnRows(sqlmock.NewRows([]string{"cluster_id", "cluster_info"}).AddRow("1", "test-wesql-0.test-wesql-headless:13306;"))
		mock.ExpectExec("call dbms_consensus").
			WillReturnError(fmt.Errorf("some error"))

		err := manager.Promote(ctx, cluster)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("promote successfully", func(t *testing.T) {
		mock.ExpectQuery("select CURRENT_LEADER, ROLE, SERVER_ID from information_schema.wesql_cluster_local").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER", "ROLE", "SERVER_ID"}).AddRow("test-wesql-0", "follower", "1"))
		mock.ExpectQuery("select cluster_id, cluster_info from mysql.consensus_info").
			WillReturnRows(sqlmock.NewRows([]string{"cluster_id", "cluster_info"}).AddRow("1", "test-wesql-0.test-wesql-headless:13306;"))
		mock.ExpectExec("call dbms_consensus").
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := manager.Promote(ctx, cluster)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}
