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
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestGetRole(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t)

	t.Run("error executing sql", func(t *testing.T) {
		mock.ExpectQuery("select CURRENT_LEADER, ROLE, SERVER_ID from information_schema.wesql_cluster_local").
			WillReturnError(fmt.Errorf("some error"))

		role, err := manager.GetReplicaRole(ctx, nil)
		assert.Equal(t, "", role)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("scan rows failed", func(t *testing.T) {
		mock.ExpectQuery("select CURRENT_LEADER, ROLE, SERVER_ID from information_schema.wesql_cluster_local").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER", "ROLE"}).AddRow("test-wesql-0", "leader"))

		role, err := manager.GetReplicaRole(ctx, nil)
		assert.Equal(t, "", role)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "sql: expected 2 destination arguments in Scan, not 3")
	})

	t.Run("no data returned", func(t *testing.T) {
		mock.ExpectQuery("select CURRENT_LEADER, ROLE, SERVER_ID from information_schema.wesql_cluster_local").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER", "ROLE"}))

		role, err := manager.GetReplicaRole(ctx, nil)
		assert.Equal(t, "", role)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "no data returned")
	})

	t.Run("get role successfully", func(t *testing.T) {
		mock.ExpectQuery("select CURRENT_LEADER, ROLE, SERVER_ID from information_schema.wesql_cluster_local").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER", "ROLE", "SERVER_ID"}).AddRow("test-wesql-0", "leader", "1"))

		role, err := manager.GetReplicaRole(ctx, nil)
		assert.Equal(t, "leader", role)
		assert.Nil(t, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestGetClusterLocalInfo(t *testing.T) {
	manager, mock, _ := mockDatabase(t)

	t.Run("error executing sql", func(t *testing.T) {
		mock.ExpectQuery("select *").
			WillReturnError(fmt.Errorf("some error"))

		clusterLocalInfo, err := manager.GetClusterLocalInfo()
		assert.Nil(t, clusterLocalInfo)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("get cluster local info successfully", func(t *testing.T) {
		mock.ExpectQuery("select *").
			WillReturnRows(sqlmock.NewRows([]string{"CURRENT_LEADER", "ROLE", "SERVER_ID"}).AddRow("test-wesql-0", "leader", "1"))

		clusterLocalInfo, err := manager.GetClusterLocalInfo()
		assert.NotNil(t, clusterLocalInfo)
		assert.Nil(t, err)
		assert.Equal(t, "test-wesql-0", clusterLocalInfo.GetString("CURRENT_LEADER"))
		assert.Equal(t, "leader", clusterLocalInfo.GetString("ROLE"))
		assert.Equal(t, "1", clusterLocalInfo.GetString("SERVER_ID"))
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}
