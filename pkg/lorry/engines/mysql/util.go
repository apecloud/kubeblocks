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
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const DateTimeFormat = "2006-01-02 15:04:05.999999"

// RowMap represents one row in a result set. Its objective is to allow
// for easy, typed getters by column name.
type RowMap map[string]CellData

// CellData is the result of a single (atomic) column in a single row
type CellData sql.NullString

func (cd *CellData) MarshalJSON() ([]byte, error) {
	if cd.Valid {
		return json.Marshal(cd.String)
	} else {
		return json.Marshal(nil)
	}
}

// UnmarshalJSON reds this object from JSON
func (cd *CellData) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	cd.String = s
	cd.Valid = true

	return nil
}

func (cd *CellData) NullString() *sql.NullString {
	return (*sql.NullString)(cd)
}

// RowData is the result of a single row, in positioned array format
type RowData []CellData

// MarshalJSON will marshal this map as JSON
func (rd *RowData) MarshalJSON() ([]byte, error) {
	cells := make([]*CellData, len(*rd))
	for i, val := range *rd {
		d := val
		cells[i] = &d
	}
	return json.Marshal(cells)
}

func (rd *RowData) Args() []interface{} {
	result := make([]interface{}, len(*rd))
	for i := range *rd {
		result[i] = *(*rd)[i].NullString()
	}
	return result
}

// ResultData is an ordered row set of RowData
type ResultData []RowData
type NamedResultData struct {
	Columns []string
	Data    ResultData
}

var EmptyResultData = ResultData{}

func (rm *RowMap) GetString(key string) string {
	return (*rm)[key].String
}

// GetStringD returns a string from the map, or a default value if the key does not exist
func (rm *RowMap) GetStringD(key string, def string) string {
	if cell, ok := (*rm)[key]; ok {
		return cell.String
	}
	return def
}

func (rm *RowMap) GetInt64(key string) int64 {
	res, _ := strconv.ParseInt(rm.GetString(key), 10, 0)
	return res
}

func (rm *RowMap) GetNullInt64(key string) sql.NullInt64 {
	i, err := strconv.ParseInt(rm.GetString(key), 10, 0)
	if err == nil {
		return sql.NullInt64{Int64: i, Valid: true}
	} else {
		return sql.NullInt64{Valid: false}
	}
}

func (rm *RowMap) GetInt(key string) int {
	res, _ := strconv.Atoi(rm.GetString(key))
	return res
}

func (rm *RowMap) GetIntD(key string, def int) int {
	res, err := strconv.Atoi(rm.GetString(key))
	if err != nil {
		return def
	}
	return res
}

func (rm *RowMap) GetUint(key string) uint {
	res, _ := strconv.ParseUint(rm.GetString(key), 10, 0)
	return uint(res)
}

func (rm *RowMap) GetUintD(key string, def uint) uint {
	res, err := strconv.Atoi(rm.GetString(key))
	if err != nil {
		return def
	}
	return uint(res)
}

func (rm *RowMap) GetUint64(key string) uint64 {
	res, _ := strconv.ParseUint(rm.GetString(key), 10, 0)
	return res
}

func (rm *RowMap) GetUint64D(key string, def uint64) uint64 {
	res, err := strconv.ParseUint(rm.GetString(key), 10, 0)
	if err != nil {
		return def
	}
	return res
}

func (rm *RowMap) GetBool(key string) bool {
	return rm.GetInt(key) != 0
}

func (rm *RowMap) GetTime(key string) time.Time {
	if t, err := time.Parse(DateTimeFormat, rm.GetString(key)); err == nil {
		return t
	}
	return time.Time{}
}

func RowToArray(rows *sql.Rows, columns []string) []CellData {
	buff := make([]interface{}, len(columns))
	data := make([]CellData, len(columns))
	for i := range buff {
		buff[i] = data[i].NullString()
	}
	_ = rows.Scan(buff...)
	return data
}

// ScanRowsToArrays is a convenience function, typically not called directly, which maps rows
// already read from the database into arrays of NullString
func ScanRowsToArrays(rows *sql.Rows, onRow func([]CellData) error) error {
	columns, _ := rows.Columns()
	for rows.Next() {
		arr := RowToArray(rows, columns)
		err := onRow(arr)
		if err != nil {
			return err
		}
	}
	return nil
}

func rowToMap(row []CellData, columns []string) map[string]CellData {
	m := make(map[string]CellData)
	for k, dataCol := range row {
		m[columns[k]] = dataCol
	}
	return m
}

// ScanRowsToMaps is a convenience function, typically not called directly, which maps rows
// already read from the database into RowMap entries.
func ScanRowsToMaps(rows *sql.Rows, onRow func(RowMap) error) error {
	columns, _ := rows.Columns()
	err := ScanRowsToArrays(rows, func(arr []CellData) error {
		m := rowToMap(arr, columns)
		err := onRow(m)
		if err != nil {
			return err
		}
		return nil
	})
	return err
}

// QueryRowsMap is a convenience function allowing querying a result set while providing a callback
// function activated per read row.
func QueryRowsMap(db *sql.DB, query string, onRow func(RowMap) error, args ...interface{}) (err error) {
	var rows *sql.Rows
	rows, err = db.Query(query, args...)
	if rows != nil {
		defer rows.Close()
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	err = ScanRowsToMaps(rows, onRow)
	return
}

func VersionParts(version string) []string {
	return strings.Split(version, ".")
}

func IsBeforeVersion(version string, otherVersion string) bool {
	thisVersions := VersionParts(version)
	otherVersions := VersionParts(otherVersion)
	if len(thisVersions) < len(otherVersions) {
		return false
	}

	for i := 0; i < len(thisVersions); i++ {
		thisToken, _ := strconv.Atoi(thisVersions[i])
		otherToken, _ := strconv.Atoi(otherVersions[i])
		if thisToken < otherToken {
			return true
		}
		if thisToken > otherToken {
			return false
		}
	}
	return false
}
