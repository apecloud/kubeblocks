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

package bench

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

const (
	unknownDB   = "Unknown database"
	createDBDDL = "CREATE DATABASE IF NOT EXISTS "
	mysqlDriver = "mysql"
)

func openDB() error {
	var (
		err error
		ds  = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", user, password, host, port, dbName)
	)

	// allow multiple statements in one query to allow Q15 on the TPC-H
	fullDsn := fmt.Sprintf("%s?multiStatements=true", ds)
	globalDB, err = sql.Open(mysqlDriver, fullDsn)
	if err != nil {
		return err
	}

	return ping()
}

func ping() error {
	if globalDB == nil {
		return nil
	}
	if err := globalDB.Ping(); err != nil {
		errString := err.Error()
		if strings.Contains(errString, unknownDB) {
			return createDB()
		} else {
			globalDB = nil
		}
		return err
	}
	return nil
}

func createDB() error {
	tmpDs := fmt.Sprintf("%s:%s@tcp(%s:%d)/", user, password, host, port)
	tmpDB, _ := sql.Open(mysqlDriver, tmpDs)
	defer tmpDB.Close()
	if _, err := tmpDB.Exec(createDBDDL + dbName); err != nil {
		return fmt.Errorf("failed to create database, err %v", err)
	}
	return nil
}

func closeDB() error {
	if globalDB == nil {
		return nil
	}
	return globalDB.Close()
}
