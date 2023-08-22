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
)

func TestReadWrite(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t, "")
	defer mock.Close()

	t.Run("write check success", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(`create table if not exists`).
			WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
		mock.ExpectCommit()

		if ok := manager.writeCheck(ctx, ""); !ok {
			t.Errorf("write check failed")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}
	})

	t.Run("write check failed", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(`create table if not exists`).
			WillReturnError(fmt.Errorf("some error"))
		mock.ExpectRollback()

		if ok := manager.writeCheck(ctx, ""); ok {
			t.Errorf("expect write check failed, but success")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}
	})

	t.Run("read check", func(t *testing.T) {
		row := pgxmock.NewRows([]string{"check_ts"}).AddRow(1)
		mock.ExpectQuery("select").WillReturnRows(row)

		if ok := manager.readCheck(ctx, ""); !ok {
			t.Errorf("read check failed")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}
	})
}

func TestPgIsReady(t *testing.T) {
	ctx := context.TODO()
	manager, mock, _ := mockDatabase(t, "")
	defer mock.Close()

	t.Run("pg is ready", func(t *testing.T) {
		mock.ExpectPing()

		if isReady := manager.IsPgReady(ctx); !isReady {
			t.Errorf("test pg is ready failed")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}
	})

	t.Run("pg is not ready", func(t *testing.T) {
		mock.ExpectPing().WillReturnError(fmt.Errorf("can't ping to db"))
		if isReady := manager.IsPgReady(ctx); isReady {
			t.Errorf("expect pg is not ready, but get ready")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %v", err)
		}
	})
}
