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
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestJsonify(t *testing.T) {
	manager, mock, _ := mockDatabase(t)

	t.Run("jsonify successfully with testCases", func(t *testing.T) {
		fakeRows := sqlmock.NewRowsWithColumnDefinition([]*sqlmock.Column{
			sqlmock.NewColumn("name").OfType("VARCHAR", ""),
			sqlmock.NewColumn("age").OfType("INT", 0),
			sqlmock.NewColumn("sex").OfType("BOOL", false),
			sqlmock.NewColumn("grade").OfType("FLOAT", 92.8),
			sqlmock.NewColumn("height").OfType("INT", int16(142)),
			sqlmock.NewColumn("weight").OfType("INT", int32(80)),
			sqlmock.NewColumn("timestamp").OfType("TIMESTAMP", int64(1000000)),
			sqlmock.NewColumn("raw").OfType("VARCHAR", sql.RawBytes{}),
		}...).AddRow("bob", 8, true, 92.8, int16(142), int32(80), int64(1000000), sql.RawBytes("123"))

		mock.ExpectQuery("select").WillReturnRows(fakeRows)
		rows, err := manager.DB.Query("select")
		assert.Nil(t, err)

		ret, err := jsonify(rows)
		assert.Equal(t, `[{"age":8,"grade":92.8,"height":142,"name":"bob","raw":"123","sex":true,"timestamp":1000000,"weight":80}]`, string(ret))
		assert.Nil(t, err)
	})

	t.Run("rows closed", func(t *testing.T) {
		fakeRows := sqlmock.NewRows([]string{"fake"}).AddRow("1")
		mock.ExpectQuery("select").WillReturnRows(fakeRows)
		rows, err := manager.DB.Query("select")
		assert.Nil(t, err)

		_ = rows.Close()
		ret, err := jsonify(rows)
		assert.Nil(t, ret)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "Rows are closed")
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}
