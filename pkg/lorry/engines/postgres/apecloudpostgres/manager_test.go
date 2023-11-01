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

package apecloudpostgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/pashagolub/pgxmock/v2"
	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
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

func TestIsConsensusReadyUp(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("consensus has been ready up", func(t *testing.T) {
		mock.ExpectQuery("SELECT extname FROM pg_extension").
			WillReturnRows(pgxmock.NewRows([]string{"extname"}).AddRow("consensus_monitor"))

		isReadyUp := manager.isConsensusReadyUp(ctx)
		assert.True(t, isReadyUp)
	})

	t.Run("consensus has not been ready up", func(t *testing.T) {
		mock.ExpectQuery("SELECT extname FROM pg_extension").
			WillReturnRows(pgxmock.NewRows([]string{"extname"}))

		isReadyUp := manager.isConsensusReadyUp(ctx)
		assert.False(t, isReadyUp)
	})

	t.Run("query pg_extension error", func(t *testing.T) {
		mock.ExpectQuery("SELECT extname FROM pg_extension").
			WillReturnError(fmt.Errorf("some errors"))

		isReadyUp := manager.isConsensusReadyUp(ctx)
		assert.False(t, isReadyUp)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestIsDBStartupReady(t *testing.T) {
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("db start up has been set", func(t *testing.T) {
		manager.DBStartupReady = true

		isReady := manager.IsDBStartupReady()
		assert.True(t, isReady)
	})

	t.Run("ping db failed", func(t *testing.T) {
		manager.DBStartupReady = false
		mock.ExpectPing().
			WillReturnError(fmt.Errorf("some error"))

		isReady := manager.IsDBStartupReady()
		assert.False(t, isReady)
	})

	t.Run("ping db success but consensus not ready up", func(t *testing.T) {
		manager.DBStartupReady = false
		mock.ExpectPing()
		mock.ExpectQuery("SELECT extname FROM pg_extension").
			WillReturnRows(pgxmock.NewRows([]string{"extname"}))

		isReady := manager.IsDBStartupReady()
		assert.False(t, isReady)
	})

	t.Run("db is startup ready", func(t *testing.T) {
		manager.DBStartupReady = false
		mock.ExpectPing()
		mock.ExpectQuery("SELECT extname FROM pg_extension").
			WillReturnRows(pgxmock.NewRows([]string{"extname"}).AddRow("consensus_monitor"))

		isReady := manager.IsDBStartupReady()
		assert.True(t, isReady)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestIsClusterInitialized(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("is not first member", func(t *testing.T) {
		manager.CurrentMemberName = "test-pod-1"

		isClusterInitialized, err := manager.IsClusterInitialized(ctx, nil)
		assert.True(t, isClusterInitialized)
		assert.Nil(t, err)
		manager.CurrentMemberName = "test-pod-0"
	})

	t.Run("db is not startup ready", func(t *testing.T) {
		manager.DBStartupReady = false
		mock.ExpectPing().
			WillReturnError(fmt.Errorf("some error"))

		isClusterInitialized, err := manager.IsClusterInitialized(ctx, nil)
		assert.False(t, isClusterInitialized)
		assert.Nil(t, err)
	})

	t.Run("query db user error", func(t *testing.T) {
		manager.DBStartupReady = false
		mock.ExpectPing()
		mock.ExpectQuery("SELECT extname FROM pg_extension").
			WillReturnRows(pgxmock.NewRows([]string{"extname"}).AddRow("consensus_monitor"))
		mock.ExpectQuery("SELECT usename FROM pg_user").
			WillReturnError(fmt.Errorf("some error"))

		isClusterInitialized, err := manager.IsClusterInitialized(ctx, nil)
		assert.False(t, isClusterInitialized)
		assert.NotNil(t, err)
	})

	t.Run("parse query error", func(t *testing.T) {
		manager.DBStartupReady = false
		mock.ExpectPing()
		mock.ExpectQuery("SELECT extname FROM pg_extension").
			WillReturnRows(pgxmock.NewRows([]string{"extname"}).AddRow("consensus_monitor"))
		mock.ExpectQuery("SELECT usename FROM pg_user").
			WillReturnRows(pgxmock.NewRows([]string{"usename"}))

		isClusterInitialized, err := manager.IsClusterInitialized(ctx, nil)
		assert.False(t, isClusterInitialized)
		assert.NotNil(t, err)
	})

	t.Run("cluster is initialized", func(t *testing.T) {
		manager.DBStartupReady = false
		mock.ExpectPing()
		mock.ExpectQuery("SELECT extname FROM pg_extension").
			WillReturnRows(pgxmock.NewRows([]string{"extname"}).AddRow("consensus_monitor"))
		mock.ExpectQuery("SELECT usename FROM pg_user").
			WillReturnRows(pgxmock.NewRows([]string{"usename"}).AddRow("replicator"))

		isClusterInitialized, err := manager.IsClusterInitialized(ctx, nil)
		assert.True(t, isClusterInitialized)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestInitializeCluster(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("exec create role and extension failed", func(t *testing.T) {
		mock.ExpectExec("create role replicator").
			WillReturnError(fmt.Errorf("some error"))

		err := manager.InitializeCluster(ctx, nil)
		assert.NotNil(t, err)
	})

	t.Run("exec create role and extension failed", func(t *testing.T) {
		mock.ExpectExec("create role replicator").
			WillReturnResult(pgxmock.NewResult("create", 1))

		err := manager.InitializeCluster(ctx, nil)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestGetMemberRoleWithHost(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	roles := []string{models.FOLLOWER, models.CANDIDATE, models.LEADER, models.LEARNER, ""}

	t.Run("query paxos role failed", func(t *testing.T) {
		mock.ExpectQuery("select paxos_role from consensus_member_status;").
			WillReturnError(fmt.Errorf("some error"))

		role, err := manager.GetMemberRoleWithHost(ctx, "")
		assert.Equal(t, "", role)
		assert.NotNil(t, err)
	})

	t.Run("parse query failed", func(t *testing.T) {
		mock.ExpectQuery("select paxos_role from consensus_member_status;").
			WillReturnRows(pgxmock.NewRows([]string{"paxos_role"}))

		role, err := manager.GetMemberRoleWithHost(ctx, "")
		assert.Equal(t, "", role)
		assert.NotNil(t, err)
	})

	t.Run("get member role with host success", func(t *testing.T) {
		for i, r := range roles {
			mock.ExpectQuery("select paxos_role from consensus_member_status;").
				WillReturnRows(pgxmock.NewRows([]string{"paxos_role"}).AddRow(i))

			role, err := manager.GetMemberRoleWithHost(ctx, "")
			assert.Equal(t, r, role)
			assert.Nil(t, err)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestIsLeaderWithHost(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("get member role with host failed", func(t *testing.T) {
		mock.ExpectQuery("select paxos_role from consensus_member_status;").
			WillReturnError(fmt.Errorf("some error"))

		isLeader, err := manager.IsLeaderWithHost(ctx, "")
		assert.False(t, isLeader)
		assert.NotNil(t, err)
	})

	t.Run("check is leader success", func(t *testing.T) {
		mock.ExpectQuery("select paxos_role from consensus_member_status;").
			WillReturnRows(pgxmock.NewRows([]string{"paxos_role"}).AddRow(2))

		isLeader, err := manager.IsLeaderWithHost(ctx, "")
		assert.True(t, isLeader)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestIsLeader(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("is leader has been set", func(t *testing.T) {
		manager.SetIsLeader(true)

		isLeader, err := manager.IsLeader(ctx, nil)
		assert.True(t, isLeader)
		assert.Nil(t, err)
	})

	t.Run("is leader has not been set", func(t *testing.T) {
		manager.UnsetIsLeader()
		mock.ExpectQuery("select paxos_role from consensus_member_status;").
			WillReturnRows(pgxmock.NewRows([]string{"paxos_role"}).AddRow(2))

		isLeader, err := manager.IsLeader(ctx, nil)
		assert.True(t, isLeader)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestGetMemberAddrs(t *testing.T) {
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

	t.Run("query ip port failed", func(t *testing.T) {
		mock.ExpectQuery("select ip_port from consensus_cluster_status;").
			WillReturnError(fmt.Errorf("some errors"))

		addrs := manager.GetMemberAddrs(ctx, cluster)
		assert.Nil(t, addrs)
	})

	t.Run("parse query failed", func(t *testing.T) {
		mock.ExpectQuery("select ip_port from consensus_cluster_status;").
			WillReturnRows(pgxmock.NewRows([]string{"ip_port"}))

		addrs := manager.GetMemberAddrs(ctx, cluster)
		assert.Nil(t, addrs)
	})

	t.Run("get member addrs success", func(t *testing.T) {
		mock.ExpectQuery("select ip_port from consensus_cluster_status;").
			WillReturnRows(pgxmock.NewRows([]string{"ip_port"}).AddRows([][]any{{"a"}, {"b"}, {"c"}}...))

		addrs := manager.GetMemberAddrs(ctx, cluster)
		assert.Equal(t, []string{"a", "b", "c"}, addrs)
	})

	t.Run("has set addrs", func(t *testing.T) {
		manager.DBState = &dcs.DBState{}
		manager.memberAddrs = []string{"a", "b", "c"}

		addrs := manager.GetMemberAddrs(ctx, cluster)
		assert.Equal(t, []string{"a", "b", "c"}, addrs)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestIsCurrentMemberInCluster(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	manager.DBState = &dcs.DBState{}
	cluster := &dcs.Cluster{
		Namespace:       manager.Namespace,
		ClusterCompName: manager.ClusterCompName,
	}

	t.Run("currentMember is in cluster", func(t *testing.T) {
		manager.memberAddrs = []string{cluster.GetMemberAddrWithName(manager.CurrentMemberName)}

		inCluster := manager.IsCurrentMemberInCluster(ctx, cluster)
		assert.True(t, inCluster)
	})

	t.Run("currentMember is in cluster", func(t *testing.T) {
		manager.memberAddrs[0] = cluster.GetMemberAddrWithName("test-pod-1")

		inCluster := manager.IsCurrentMemberInCluster(ctx, cluster)
		assert.False(t, inCluster)
	})
}

func TestIsCurrentMemberHealthy(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{}
	cluster.Members = append(cluster.Members, dcs.Member{
		Name: manager.CurrentMemberName,
	})

	t.Run("cluster has no leader", func(t *testing.T) {
		isHealthy := manager.IsCurrentMemberHealthy(ctx, cluster)
		assert.True(t, isHealthy)
	})

	cluster.Leader = &dcs.Leader{
		Name: manager.CurrentMemberName,
	}

	t.Run("get member health status failed", func(t *testing.T) {
		mock.ExpectQuery("select connected, log_delay_num from consensus_cluster_health").
			WillReturnError(fmt.Errorf("some error"))

		isHealthy := manager.IsCurrentMemberHealthy(ctx, cluster)
		assert.False(t, isHealthy)
	})

	t.Run("member is healthy", func(t *testing.T) {
		mock.ExpectQuery("select connected, log_delay_num from consensus_cluster_health").
			WillReturnRows(pgxmock.NewRows([]string{"connected", "log_delay_num"}).AddRow(true, 0))

		isHealthy := manager.IsCurrentMemberHealthy(ctx, cluster)
		assert.True(t, isHealthy)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestGetMemberHealthyStatus(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{}
	cluster.Members = append(cluster.Members, dcs.Member{
		Name: manager.CurrentMemberName,
	})
	cluster.Leader = &dcs.Leader{
		Name: manager.CurrentMemberName,
	}

	t.Run("query failed", func(t *testing.T) {
		mock.ExpectQuery("select connected, log_delay_num from consensus_cluster_health").
			WillReturnError(fmt.Errorf("some error"))

		healthStatus, err := manager.getMemberHealthStatus(ctx, cluster, cluster.GetMemberWithName(manager.CurrentMemberName))
		assert.NotNil(t, err)
		assert.Nil(t, healthStatus)
	})

	t.Run("parse query failed", func(t *testing.T) {
		mock.ExpectQuery("select connected, log_delay_num from consensus_cluster_health").
			WillReturnRows(pgxmock.NewRows([]string{"connected, log_delay_num"}))

		healthStatus, err := manager.getMemberHealthStatus(ctx, cluster, cluster.GetMemberWithName(manager.CurrentMemberName))
		assert.NotNil(t, err)
		assert.Nil(t, healthStatus)
	})

	t.Run("get member health status success", func(t *testing.T) {
		mock.ExpectQuery("select connected, log_delay_num from consensus_cluster_health").
			WillReturnRows(pgxmock.NewRows([]string{"connected", "log_delay_num"}).AddRow(true, 0))

		healthStatus, err := manager.getMemberHealthStatus(ctx, cluster, cluster.GetMemberWithName(manager.CurrentMemberName))
		assert.Nil(t, err)
		assert.True(t, healthStatus.Connected)
		assert.Equal(t, int64(0), healthStatus.LogDelayNum)
	})

	t.Run("health status has been set", func(t *testing.T) {
		manager.healthStatus = &postgres.ConsensusMemberHealthStatus{
			Connected:   false,
			LogDelayNum: 200,
		}
		manager.DBState = &dcs.DBState{}

		healthStatus, err := manager.getMemberHealthStatus(ctx, cluster, cluster.GetMemberWithName(manager.CurrentMemberName))
		assert.Nil(t, err)
		assert.False(t, healthStatus.Connected)
		assert.Equal(t, int64(200), healthStatus.LogDelayNum)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
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

	t.Run("cluster has no leader", func(t *testing.T) {
		isLagging, lag := manager.IsMemberLagging(ctx, cluster, currentMember)
		assert.False(t, isLagging)
		assert.Equal(t, int64(0), lag)
	})

	cluster.Leader = &dcs.Leader{
		Name: manager.CurrentMemberName,
	}

	t.Run("get member health status failed", func(t *testing.T) {
		mock.ExpectQuery("select connected, log_delay_num from consensus_cluster_health").
			WillReturnError(fmt.Errorf("some error"))

		isLagging, lag := manager.IsMemberLagging(ctx, cluster, currentMember)
		assert.True(t, isLagging)
		assert.Equal(t, int64(1), lag)
	})

	t.Run("member is not lagging", func(t *testing.T) {
		mock.ExpectQuery("select connected, log_delay_num from consensus_cluster_health").
			WillReturnRows(pgxmock.NewRows([]string{"connected", "log_delay_num"}).AddRow(true, 0))

		isLagging, lag := manager.IsMemberLagging(ctx, cluster, currentMember)
		assert.False(t, isLagging)
		assert.Equal(t, int64(0), lag)
	})

	cluster.Leader = &dcs.Leader{
		Name: manager.CurrentMemberName,
	}
}

func TestJoinCurrentMemberToCluster(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{}
	cluster.Leader = &dcs.Leader{
		Name: manager.CurrentMemberName,
	}
	cluster.Members = append(cluster.Members, dcs.Member{
		Name: manager.CurrentMemberName,
	})

	t.Run("exec alter system failed", func(t *testing.T) {
		mock.ExpectExec("alter system").
			WillReturnError(fmt.Errorf("some error"))

		err := manager.JoinCurrentMemberToCluster(ctx, cluster)
		assert.NotNil(t, err)
	})

	t.Run("exec alter system success", func(t *testing.T) {
		mock.ExpectExec("alter system").
			WillReturnResult(pgxmock.NewResult("alter system", 1))

		err := manager.JoinCurrentMemberToCluster(ctx, cluster)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestLeaveMemberFromCluster(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{}

	t.Run("exec alter system failed", func(t *testing.T) {
		mock.ExpectExec("alter system").
			WillReturnError(fmt.Errorf("some error"))

		err := manager.LeaveMemberFromCluster(ctx, cluster, "")
		assert.NotNil(t, err)
	})

	t.Run("exec alter system success", func(t *testing.T) {
		mock.ExpectExec("alter system").
			WillReturnResult(pgxmock.NewResult("alter system", 1))

		err := manager.LeaveMemberFromCluster(ctx, cluster, "")
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestIsClusterHealthy(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{}
	cluster.Members = append(cluster.Members, dcs.Member{
		Name: manager.CurrentMemberName,
	})

	t.Run("cluster has no leader", func(t *testing.T) {
		isClusterHealthy := manager.IsClusterHealthy(ctx, cluster)
		assert.True(t, isClusterHealthy)
	})

	cluster.Leader = &dcs.Leader{}

	t.Run("current member is leader", func(t *testing.T) {
		cluster.Leader.Name = manager.CurrentMemberName
		isClusterHealthy := manager.IsClusterHealthy(ctx, cluster)
		assert.True(t, isClusterHealthy)
	})

	t.Run("cluster is healthy", func(t *testing.T) {
		cluster.Leader.Name = "test"
		cluster.Members[0].Name = "test"
		manager.DBState = &dcs.DBState{}
		manager.healthStatus = &postgres.ConsensusMemberHealthStatus{
			Connected: true,
		}

		isClusterHealthy := manager.IsClusterHealthy(ctx, cluster)
		assert.True(t, isClusterHealthy)
	})
}

func TestPromote(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{
		Namespace:       manager.Namespace,
		ClusterCompName: manager.ClusterCompName,
	}

	t.Run("query leader ip port failed", func(t *testing.T) {
		mock.ExpectQuery("select ip_port from consensus_cluster_status").
			WillReturnError(fmt.Errorf("some error"))

		err := manager.Promote(ctx, cluster)
		assert.NotNil(t, err)
	})

	t.Run("parse query failed", func(t *testing.T) {
		mock.ExpectQuery("select ip_port from consensus_cluster_status").
			WillReturnRows(pgxmock.NewRows([]string{"ip_port"}))

		err := manager.Promote(ctx, cluster)
		assert.NotNil(t, err)
	})

	t.Run("exec promote failed", func(t *testing.T) {
		mock.ExpectQuery("select ip_port from consensus_cluster_status").
			WillReturnRows(pgxmock.NewRows([]string{"ip_port"}).AddRow(":"))
		mock.ExpectExec("alter system").
			WillReturnError(fmt.Errorf("some error"))

		err := manager.Promote(ctx, cluster)
		assert.NotNil(t, err)
	})

	t.Run("exec promote success", func(t *testing.T) {
		mock.ExpectQuery("select ip_port from consensus_cluster_status").
			WillReturnRows(pgxmock.NewRows([]string{"ip_port"}).AddRow(":"))
		mock.ExpectExec("alter system").
			WillReturnResult(pgxmock.NewResult("alter system", 1))

		err := manager.Promote(ctx, cluster)
		assert.Nil(t, err)
	})

	t.Run("current member is already the leader", func(t *testing.T) {
		manager.SetIsLeader(true)

		err := manager.Promote(ctx, cluster)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestIsPromoted(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("is promoted", func(t *testing.T) {
		manager.SetIsLeader(true)
		isPromoted := manager.IsPromoted(ctx)

		assert.True(t, isPromoted)
	})
}

func TestHasOtherHealthyLeader(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{}

	t.Run("query failed", func(t *testing.T) {
		mock.ExpectQuery("select ip_port from consensus_cluster_status").
			WillReturnError(fmt.Errorf("some error"))

		member := manager.HasOtherHealthyLeader(ctx, cluster)
		assert.Nil(t, member)
	})

	t.Run("parse query failed", func(t *testing.T) {
		mock.ExpectQuery("select ip_port from consensus_cluster_status").
			WillReturnRows(pgxmock.NewRows([]string{"ip_port"}))

		member := manager.HasOtherHealthyLeader(ctx, cluster)
		assert.Nil(t, member)
	})

	t.Run("has other healthy leader", func(t *testing.T) {
		cluster.Members = append(cluster.Members, dcs.Member{
			Name: "test",
		})
		mock.ExpectQuery("select ip_port from consensus_cluster_status").
			WillReturnRows(pgxmock.NewRows([]string{"ip_port"}).AddRow("test:5432"))

		member := manager.HasOtherHealthyLeader(ctx, cluster)
		assert.NotNil(t, member)
	})

	t.Run("current member is leader", func(t *testing.T) {
		manager.SetIsLeader(true)

		member := manager.HasOtherHealthyLeader(ctx, cluster)
		assert.Nil(t, member)
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

func TestGetDBState(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()
	cluster := &dcs.Cluster{}
	cluster.Members = append(cluster.Members, dcs.Member{
		Name: manager.CurrentMemberName,
	})
	cluster.Leader = &dcs.Leader{
		Name: manager.CurrentMemberName,
	}

	t.Run("check is leader failed", func(t *testing.T) {
		mock.ExpectQuery("select paxos_role").
			WillReturnError(fmt.Errorf("some error"))

		dbState := manager.GetDBState(ctx, cluster)
		assert.Nil(t, dbState)
	})

	t.Run("get member addrs failed", func(t *testing.T) {
		mock.ExpectQuery("select paxos_role").
			WillReturnRows(pgxmock.NewRows([]string{"paxos_role"}).AddRow(2))
		mock.ExpectQuery("select ip_port").
			WillReturnError(fmt.Errorf("some error"))

		dbState := manager.GetDBState(ctx, cluster)
		assert.Nil(t, dbState)
	})

	t.Run("get member health status failed", func(t *testing.T) {
		mock.ExpectQuery("select paxos_role").
			WillReturnRows(pgxmock.NewRows([]string{"paxos_role"}).AddRow(2))
		mock.ExpectQuery("select ip_port").
			WillReturnRows(pgxmock.NewRows([]string{"ip_port"}).AddRows([][]any{{"a"}, {"b"}, {"c"}}...))
		mock.ExpectQuery("select connected, log_delay_num").
			WillReturnError(fmt.Errorf("some error"))

		dbState := manager.GetDBState(ctx, cluster)
		assert.Nil(t, dbState)
	})

	t.Run("get db state success", func(t *testing.T) {
		mock.ExpectQuery("select paxos_role").
			WillReturnRows(pgxmock.NewRows([]string{"paxos_role"}).AddRow(2))
		mock.ExpectQuery("select ip_port").
			WillReturnRows(pgxmock.NewRows([]string{"ip_port"}).AddRows([][]any{{"a"}, {"b"}, {"c"}}...))
		mock.ExpectQuery("select connected, log_delay_num").
			WillReturnRows(pgxmock.NewRows([]string{"connected", "log_delay_num"}).AddRow(true, 20))

		dbState := manager.GetDBState(ctx, cluster)
		assert.NotNil(t, dbState)
		assert.Equal(t, []string{"a", "b", "c"}, manager.memberAddrs)
		assert.Equal(t, int64(20), manager.healthStatus.LogDelayNum)
		assert.True(t, manager.healthStatus.Connected)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}
