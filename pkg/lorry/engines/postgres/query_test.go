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
	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
)

const (
	execTest  = "create database test"
	queryTest = "select 1"
)

func TestQuery(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("query success", func(t *testing.T) {
		sql := queryTest
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"1"}))

		_, err := manager.Query(ctx, sql)
		assert.Nil(t, err)
	})

	t.Run("query failed", func(t *testing.T) {
		sql := queryTest
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		_, err := manager.Query(ctx, sql)
		assert.NotNil(t, err)
	})

	t.Run("parse rows failed", func(t *testing.T) {
		sql := queryTest
		var val chan string
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"1"}).AddRow(val))
		_, err := manager.Query(ctx, sql)
		assert.NotNil(t, err)
	})

	t.Run("can't connect db", func(t *testing.T) {
		sql := queryTest
		resp, err := manager.QueryWithHost(ctx, sql, "localhost")
		assert.NotNil(t, err)
		assert.Nil(t, resp)
	})

	t.Run("query leader success", func(t *testing.T) {
		sql := queryTest
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"1"}).AddRow("1"))
		cluster := &dcs.Cluster{
			Leader: &dcs.Leader{
				Name: manager.CurrentMemberName,
			},
		}
		cluster.Members = append(cluster.Members, dcs.Member{
			Name: manager.CurrentMemberName,
		})

		resp, err := manager.QueryLeader(ctx, sql, cluster)
		if err != nil {
			t.Errorf("expect query leader success but failed")
		}

		assert.Equal(t, []byte(`[{"1":"1"}]`), resp)
	})

	t.Run("query leader failed, cluster has no leader", func(t *testing.T) {
		sql := queryTest
		cluster := &dcs.Cluster{}

		_, err := manager.QueryLeader(ctx, sql, cluster)
		if err == nil {
			t.Errorf("expect query leader success but failed")
		}

		assert.ErrorIs(t, ClusterHasNoLeader, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestParseQuery(t *testing.T) {
	t.Run("parse query success", func(t *testing.T) {
		data := []byte(`[{"current_setting":"off"}]`)
		resMap, err := ParseQuery(string(data))
		assert.NotNil(t, resMap)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(resMap))
		assert.Equal(t, "off", resMap[0]["current_setting"].(string))
	})

	t.Run("parse query failed", func(t *testing.T) {
		data := []byte(`{"current_setting":"off"}`)
		resMap, err := ParseQuery(string(data))
		assert.NotNil(t, err)
		assert.Nil(t, resMap)
	})
}

func TestExec(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("exec success", func(t *testing.T) {
		sql := execTest

		mock.ExpectExec("create database").
			WillReturnResult(pgxmock.NewResult("CREATE DATABASE", 1))

		_, err := manager.Exec(ctx, sql)
		assert.Nil(t, err)
	})

	t.Run("exec failed", func(t *testing.T) {
		sql := execTest

		mock.ExpectExec("create database").
			WillReturnError(fmt.Errorf("some error"))

		_, err := manager.Exec(ctx, sql)
		assert.NotNil(t, err)
	})

	t.Run("can't connect db", func(t *testing.T) {
		sql := execTest
		resp, err := manager.ExecWithHost(ctx, sql, "test")
		if err == nil {
			t.Errorf("expect query failed, but success")
		}
		assert.Equal(t, int64(0), resp)
	})

	t.Run("exec leader success", func(t *testing.T) {
		sql := execTest
		mock.ExpectExec("create").
			WillReturnResult(pgxmock.NewResult("CREATE", 1))
		cluster := &dcs.Cluster{
			Leader: &dcs.Leader{
				Name: manager.CurrentMemberName,
			},
		}
		cluster.Members = append(cluster.Members, dcs.Member{
			Name: manager.CurrentMemberName,
		})

		resp, err := manager.ExecLeader(ctx, sql, cluster)
		if err != nil {
			t.Errorf("expect exec leader success but failed")
		}
		assert.Equal(t, int64(1), resp)
	})

	t.Run("exec leader failed, cluster has no leader", func(t *testing.T) {
		sql := execTest
		cluster := &dcs.Cluster{}

		_, err := manager.ExecLeader(ctx, sql, cluster)
		if err == nil {
			t.Errorf("expect exec leader success but failed")
		}

		assert.ErrorIs(t, ClusterHasNoLeader, err)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

func TestGetPgCurrentSetting(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := MockDatabase(t)
	defer mock.Close()

	t.Run("query failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnError(fmt.Errorf("some error"))

		res, err := manager.GetPgCurrentSetting(ctx, "test")
		assert.NotNil(t, err)
		assert.Equal(t, "", res)
	})

	t.Run("parse query failed", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}))

		res, err := manager.GetPgCurrentSetting(ctx, "test")
		assert.NotNil(t, err)
		assert.Equal(t, "", res)
	})

	t.Run("query success", func(t *testing.T) {
		mock.ExpectQuery("select").
			WillReturnRows(pgxmock.NewRows([]string{"current_setting"}).AddRow("test"))

		res, err := manager.GetPgCurrentSetting(ctx, "test")
		assert.Nil(t, err)
		assert.Equal(t, "test", res)
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}
