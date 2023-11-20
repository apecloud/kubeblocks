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
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
)

func TestGetDBConnWithMember(t *testing.T) {
	manager, _, _ := mockDatabase(t)
	cluster := &dcs.Cluster{
		ClusterCompName: fakeClusterCompName,
		Namespace:       fakeNamespace,
	}

	t.Run("new db connection failed", func(t *testing.T) {
		_, _ = NewConfig(fakePropertiesWithWrongURL)
		db, err := manager.GetDBConnWithMember(cluster, &dcs.Member{})

		assert.Nil(t, db)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "new db connection failed")
	})

	t.Run("return current member connection", func(t *testing.T) {
		db, err := manager.GetDBConnWithMember(cluster, nil)

		assert.NotNil(t, db)
		assert.Nil(t, err)
		assert.Equal(t, db, manager.DB)
	})
}

func TestGetLeaderMember(t *testing.T) {
	manager, mock, _ := mockDatabase(t)
	cluster := &dcs.Cluster{
		Members: []dcs.Member{
			{
				Name: fakePodName,
			},
		},
	}

	t.Run("Get cluster local info failed", func(t *testing.T) {
		mock.ExpectQuery("select *").
			WillReturnError(fmt.Errorf("some error"))

		leaderMember := manager.GetLeaderMember(cluster)
		assert.Nil(t, leaderMember)
	})

	t.Run("leader addr is empty", func(t *testing.T) {
		mock.ExpectQuery("select *").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER"}).AddRow(""))

		leaderMember := manager.GetLeaderMember(cluster)
		assert.Nil(t, leaderMember)
	})

	t.Run("get leader member success", func(t *testing.T) {
		mock.ExpectQuery("select *").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER"}).AddRow(fakePodName + ".test-wesql.headless"))

		leaderMember := manager.GetLeaderMember(cluster)
		assert.NotNil(t, leaderMember)
		assert.Equal(t, fakePodName, leaderMember.Name)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestGetLeaderConn(t *testing.T) {
	manager, mock, _ := mockDatabase(t)
	cluster := &dcs.Cluster{
		ClusterCompName: fakeClusterCompName,
		Namespace:       fakeNamespace,
	}

	t.Run("the cluster has no leader", func(t *testing.T) {
		mock.ExpectQuery("select *").
			WillReturnError(fmt.Errorf("some error"))

		db, err := manager.GetLeaderConn(cluster)
		assert.Nil(t, db)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "the cluster has no leader")
	})

	t.Run("get leader conn successfully", func(t *testing.T) {
		_, _ = NewConfig(fakeProperties)
		mock.ExpectQuery("select *").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER"}).AddRow(fakePodName + ".test-wesql.headless"))
		cluster.Members = []dcs.Member{
			{
				Name: fakePodName,
			},
		}

		db, err := manager.GetLeaderConn(cluster)
		assert.NotNil(t, db)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}
