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
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/zapr"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestQuery(t *testing.T) {
	manager, mock, _ := mockDatabase(t)

	t.Run("no dbType provided", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "value", "timestamp"}).
			AddRow(1, "value-1", time.Now()).
			AddRow(2, "value-2", time.Now().Add(1000)).
			AddRow(3, "value-3", time.Now().Add(2000))

		mock.ExpectQuery("SELECT \\* FROM foo WHERE id < 4").WillReturnRows(rows)
		ret, err := manager.Query(context.Background(), `SELECT * FROM foo WHERE id < 4`)
		assert.Nil(t, err)
		t.Logf("query result: %s", ret)
		assert.Contains(t, string(ret), "\"id\":\"1")
		var result []interface{}
		err = json.Unmarshal(ret, &result)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(result))
	})

	t.Run("dbType provided", func(t *testing.T) {
		col1 := sqlmock.NewColumn("id").OfType("BIGINT", 1)
		col2 := sqlmock.NewColumn("value").OfType("FLOAT", 1.0)
		col3 := sqlmock.NewColumn("timestamp").OfType("TIME", time.Now())
		rows := sqlmock.NewRowsWithColumnDefinition(col1, col2, col3).
			AddRow(1, 1.1, time.Now()).
			AddRow(2, 2.2, time.Now().Add(1000)).
			AddRow(3, 3.3, time.Now().Add(2000))
		mock.ExpectQuery("SELECT \\* FROM foo WHERE id < 4").WillReturnRows(rows)
		ret, err := manager.Query(context.Background(), "SELECT * FROM foo WHERE id < 4")
		assert.Nil(t, err)
		t.Logf("query result: %s", ret)

		// verify number
		assert.Contains(t, string(ret), "\"id\":1")
		assert.Contains(t, string(ret), "\"value\":2.2")

		var result []interface{}
		err = json.Unmarshal(ret, &result)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(result))

		// verify timestamp
		ts, ok := result[0].(map[string]interface{})["timestamp"].(string)
		assert.True(t, ok)
		var tt time.Time
		tt, err = time.Parse(time.RFC3339, ts)
		assert.Nil(t, err)
		t.Logf("time stamp is: %v", tt)
	})
}

func TestExec(t *testing.T) {
	manager, mock, _ := mockDatabase(t)
	mock.ExpectExec("INSERT INTO foo \\(id, v1, ts\\) VALUES \\(.*\\)").WillReturnResult(sqlmock.NewResult(1, 1))
	i, err := manager.Exec(context.Background(), "INSERT INTO foo (id, v1, ts) VALUES (1, 'test-1', '2021-01-22')")
	assert.Equal(t, int64(1), i)
	assert.Nil(t, err)
}

func mockDatabase(t *testing.T) (*Manager, sqlmock.Sqlmock, error) {
	viper.SetDefault("KB_SERVICE_ROLES", "{\"follower\":\"Readonly\",\"leader\":\"ReadWrite\"}")
	viper.Set("KB_POD_NAME", "test-pod-0")
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	manager := &Manager{}
	development, _ := zap.NewDevelopment()
	manager.Logger = zapr.NewLogger(development)
	manager.DB = db

	return manager, mock, err
}
