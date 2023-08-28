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

	"github.com/dapr/kit/logger"
	"github.com/pashagolub/pgxmock/v2"
	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
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
	roles := []string{binding.FOLLOWER, binding.CANDIDATE, binding.LEADER, binding.LEARNER, ""}

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

	t.Run("")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}
