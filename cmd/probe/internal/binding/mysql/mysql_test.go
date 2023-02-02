/*
Copyright ApeCloud, Inc.
Copyright 2021 The Dapr Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/components-contrib/metadata"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/dapr/kit/logger"
	"github.com/stretchr/testify/assert"
)

func TestGetRunningPort(t *testing.T) {
	m := &Mysql{
		metadata: bindings.Metadata{
			Base: metadata.Base{
				Properties: map[string]string{
					"url": "root:@tcp(127.0.0.1:3307)/mysql?multiStatements=true",
				},
			},
		},
	}

	port := m.GetRunningPort()
	assert.Equal(t, 3307, port)

	m.metadata.Properties["url"] = "root:@tcp(127.0.0.1)/mysql?multiStatements=true"
	port = m.GetRunningPort()
	assert.Equal(t, defaultDbPort, port)
}

func TestGetRole(t *testing.T) {
	m, mock, _ := mockDatabase(t)

	t.Run("GetRole succeed", func(t *testing.T) {
		col1 := sqlmock.NewColumn("CURRENT_LEADER").OfType("VARCHAR", "")
		col2 := sqlmock.NewColumn("ROLE").OfType("VARCHAR", "")
		col3 := sqlmock.NewColumn("SERVER_ID").OfType("INT", 0)
		rows := sqlmock.NewRowsWithColumnDefinition(col1, col2, col3).AddRow("wesql-main-1.wesql-main-headless:13306", "Follower", 1)
		mock.ExpectQuery("select .* from information_schema.wesql_cluster_local").WillReturnRows(rows)

		role, err := m.GetRole(context.Background(), "")
		assert.Nil(t, err)
		assert.Equal(t, "Follower", role)
	})

	t.Run("GetRole fails", func(t *testing.T) {
		mock.ExpectQuery("select .* from information_schema.wesql_cluster_local").WillReturnError(errors.New("no record"))

		role, err := m.GetRole(context.Background(), "")
		assert.Equal(t, "", role)
		assert.NotNil(t, err)
	})
}

func mockDatabase(t *testing.T) (*Mysql, sqlmock.Sqlmock, error) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	m := NewMysql(logger.NewLogger("test")).(*Mysql)
	m.db = db

	return m, mock, err
}
