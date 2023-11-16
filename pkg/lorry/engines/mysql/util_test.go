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
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	fakeCellDataString = "fake-cell-data"
)

func TestCellData_MarshalJSON(t *testing.T) {
	testCases := []struct {
		fakeString  string
		valid       bool
		expectRet   string
		expectError error
	}{
		{fakeCellDataString, true, `"fake-cell-data"`, nil},
		{fakeCellDataString, false, "null", nil},
	}

	for _, testCase := range testCases {
		fakeCellData := &CellData{
			String: testCase.fakeString,
			Valid:  testCase.valid,
		}

		ret, err := fakeCellData.MarshalJSON()
		assert.ErrorIs(t, err, testCase.expectError)
		assert.Equal(t, testCase.expectRet, string(ret))
	}
}

func TestCellData_UnmarshalJSON(t *testing.T) {
	testCases := []struct {
		fakeJSON             string
		expectErrorMsg       string
		expectCellDataString string
	}{
		{`"fake"`, "", `fake`},
		{`{"data": "test"}`, "json: cannot unmarshal object into Go value of type string", ""},
	}

	for _, testCase := range testCases {
		fakeCellData := &CellData{}

		err := fakeCellData.UnmarshalJSON([]byte(testCase.fakeJSON))
		assert.Equal(t, testCase.expectErrorMsg == "", err == nil)
		if err != nil {
			assert.ErrorContains(t, err, testCase.expectErrorMsg)
		}
		assert.Equal(t, testCase.expectCellDataString, fakeCellData.String)
	}
}

func TestRowData(t *testing.T) {
	fakeRowData := &RowData{
		{
			String: fakeCellDataString,
			Valid:  true,
		},
	}

	ret, err := fakeRowData.MarshalJSON()
	assert.Nil(t, err)
	assert.Equal(t, `["fake-cell-data"]`, string(ret))

	args := fakeRowData.Args()
	cellData, ok := args[0].(sql.NullString)
	assert.True(t, ok)
	assert.Equal(t, sql.NullString{
		String: fakeCellDataString,
		Valid:  true,
	}, cellData)
}

func TestRowMap(t *testing.T) {
	fakeRowMap := &RowMap{
		"fake": CellData{
			String: DateTimeFormat,
			Valid:  true,
		},
		"num": CellData{
			String: "20",
		},
	}

	getStringDTestCases := []struct {
		key     string
		ret     string
		timeRet time.Time
	}{
		{"fake", DateTimeFormat, time.Date(2006, time.January, 2, 15, 4, 5, 999999000, time.UTC)},
		{"test", "test", time.Time{}},
	}
	for _, testCase := range getStringDTestCases {
		assert.Equal(t, testCase.ret, fakeRowMap.GetStringD(testCase.key, "test"))
		assert.Equal(t, testCase.timeRet, fakeRowMap.GetTime(testCase.key))
	}

	assert.Equal(t, int64(20), fakeRowMap.GetInt64("num"))
	assert.Equal(t, 20, fakeRowMap.GetInt("num"))
	assert.False(t, fakeRowMap.GetBool("fake"))
	assert.Equal(t, uint(20), fakeRowMap.GetUint("num"))
	assert.Equal(t, uint64(20), fakeRowMap.GetUint64("num"))

	getNullInt64TestCases := []struct {
		key        string
		nullRet    sql.NullInt64
		intDRet    int
		uintDRet   uint
		uint64DRet uint64
	}{
		{"num", sql.NullInt64{Int64: 20, Valid: true}, 20, uint(20), uint64(20)},
		{"test", sql.NullInt64{Valid: false}, 30, uint(30), uint64(30)},
	}
	for _, testCase := range getNullInt64TestCases {
		assert.Equal(t, testCase.nullRet, fakeRowMap.GetNullInt64(testCase.key))
		assert.Equal(t, testCase.intDRet, fakeRowMap.GetIntD(testCase.key, 30))
		assert.Equal(t, testCase.uintDRet, fakeRowMap.GetUintD(testCase.key, 30))
		assert.Equal(t, testCase.uint64DRet, fakeRowMap.GetUint64D(testCase.key, 30))
	}

}
