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

package oceanbase

import (
	"context"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/mysql"
	"github.com/apecloud/kubeblocks/pkg/viperx"
	"github.com/stretchr/testify/assert"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	fakePodName         = "test-ob-0"
	fakeClusterCompName = "test-ob"
	fakeNamespace       = "fake-namespace"
)

func mockDatabase(t *testing.T) (*Manager, sqlmock.Sqlmock, error) {
	manager := &Manager{
		Manager: mysql.Manager{
			DBManagerBase: engines.DBManagerBase{
				CurrentMemberName: fakePodName,
				ClusterCompName:   fakeClusterCompName,
				Namespace:         fakeNamespace,
				Logger:            ctrl.Log.WithName("ob-TEST"),
			},
		},
	}

	manager.ReplicaTenant = viperx.GetString("TENANT_NAME")
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	manager.DB = db

	return manager, mock, err
}

func TestGetRole(t *testing.T) {
	ctx := context.TODO()
	viperx.SetDefault("TENANT_NAME", "alice")
	manager, mock, _ := mockDatabase(t)

	t.Run("error executing sql", func(t *testing.T) {
		mock.ExpectQuery(`select count\(distinct\(zone\)\) as count from oceanbase.__all_zone where zone!=''`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT TENANT_ROLE FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME='alice'").
			WillReturnError(fmt.Errorf("some error"))

		role, err := manager.GetReplicaRole(ctx, nil)
		assert.Equal(t, "", role)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "some error")
	})

	t.Run("scan rows failed", func(t *testing.T) {
		mock.ExpectQuery(`select count\(distinct\(zone\)\) as count from oceanbase.__all_zone where zone!=''`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT TENANT_ROLE FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME='alice'").
			WillReturnRows(sqlmock.NewRows([]string{"TENANT_ROLE"}).AddRow(PRIMARY))

		role, err := manager.GetReplicaRole(ctx, nil)
		assert.Equal(t, PRIMARY, role)
		assert.Nil(t, err)
	})

	t.Run("no data returned", func(t *testing.T) {
		mock.ExpectQuery(`select count\(distinct\(zone\)\) as count from oceanbase.__all_zone where zone!=''`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT TENANT_ROLE FROM oceanbase.DBA_OB_TENANTS where TENANT_NAME='alice'").
			WillReturnRows(sqlmock.NewRows([]string{"ROLE"}))

		role, err := manager.GetReplicaRole(ctx, nil)
		assert.Equal(t, "", role)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "no data returned")
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}
