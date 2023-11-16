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
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	fakeGTIDString = "ee194423-3040-11ee-9393-eab5dfc9b22a:1-5:7:9-10"
	fakeGTIDSet    = "  ee194423-3040-11ee-9393-eab5dfc9b22a:1-5:7:9-10,b3512340-fc03-11ec-920f-000c29f6e7cf:1-4 "
	fakeServerUUID = "ee194423-3040-11ee-9393-eab5dfc9b22a"
)

func TestNewGTIDItem(t *testing.T) {
	fakeGTIDStrings := []string{
		fakeServerUUID,
		":1-5",
		"ee194423-3040-11ee-9393-eab5dfc9b22a:",
		fakeGTIDString,
	}
	expectResults := []string{
		"GTID wrong format:",
		"GTID no server UUID:",
		"GTID no range:",
		"",
	}

	for i, s := range fakeGTIDStrings {
		gtidItem, err := NewGTIDItem(s)
		assert.Equal(t, expectResults[i] == "", err == nil)
		if err != nil {
			assert.ErrorContains(t, err, expectResults[i])
		}
		assert.Equal(t, expectResults[i] != "", gtidItem == nil)
	}
}

func TestGTIDItem_Explode(t *testing.T) {
	gtidItem, err := NewGTIDItem(fakeGTIDString)
	assert.Nil(t, err)

	items := gtidItem.Explode()
	assert.NotNil(t, items)
	assert.Len(t, items, 8)
}

func TestNewOracleGtidSet(t *testing.T) {
	testCases := []struct {
		gtidSets       string
		expectItemsLen int
		expectErrorMsg string
	}{
		{"", 0, ""},
		{" , :1-5, ", 0, "GTID no server UUID:"},
		{fakeGTIDSet, 2, ""},
	}

	for _, testCase := range testCases {
		gtidSets, err := NewOracleGtidSet(testCase.gtidSets)
		assert.Len(t, gtidSets.Items, testCase.expectItemsLen)
		assert.Equal(t, err == nil, testCase.expectErrorMsg == "")
		if err != nil {
			assert.ErrorContains(t, err, testCase.expectErrorMsg)
		}
	}
}

func TestGTIDSet_RemoveUUID(t *testing.T) {
	gtidSets, err := NewOracleGtidSet(fakeGTIDSet)
	assert.Nil(t, err)

	testCases := []struct {
		uuid          string
		expectRemoved bool
	}{
		{fakeServerUUID, true},
		{"", false},
	}

	for _, testCase := range testCases {
		assert.Equal(t, testCase.expectRemoved, gtidSets.RemoveUUID(testCase.uuid))
	}
}

func TestGTIDSet_RetainUUID(t *testing.T) {
	gtidSets, err := NewOracleGtidSet(fakeGTIDSet)
	assert.Nil(t, err)

	assert.True(t, gtidSets.RetainUUID(fakeServerUUID))
}

func TestGTIDSet_RetainUUIDs(t *testing.T) {
	gtidSets, err := NewOracleGtidSet(fakeGTIDSet)
	assert.Nil(t, err)

	testCases := []struct {
		uuids                 []string
		expectAnythingRemoved bool
	}{
		{[]string{fakeServerUUID}, true},
		{[]string{fakeServerUUID, "b3512340-fc03-11ec-920f-000c29f6e7cf"}, false},
	}

	for _, testCase := range testCases {
		assert.Equal(t, testCase.expectAnythingRemoved, gtidSets.RetainUUIDs(testCase.uuids))
	}
}

func TestGTIDSet_SharedUUIDs(t *testing.T) {
	gtidSets, err := NewOracleGtidSet(fakeGTIDSet)
	assert.Nil(t, err)
	assert.False(t, gtidSets.IsEmpty())

	testCases := []struct {
		other                *GTIDSet
		expectSharedItemsLen int
	}{
		{&GTIDSet{
			Items: []*GTIDItem{
				{
					ServerUUID: fakeServerUUID,
				},
			},
		}, 1},
		{&GTIDSet{Items: make([]*GTIDItem, 0)}, 0},
	}

	for _, testCase := range testCases {
		assert.Len(t, gtidSets.SharedUUIDs(testCase.other), testCase.expectSharedItemsLen)
	}
}

func TestGTIDSet_Explode(t *testing.T) {
	gtidSets, err := NewOracleGtidSet(fakeGTIDSet)
	assert.Nil(t, err)

	items := gtidSets.Explode()
	assert.NotNil(t, items)
	assert.Len(t, items, 12)
}

func TestGTIDSet_String(t *testing.T) {
	gtidSets, err := NewOracleGtidSet(fakeGTIDSet)
	assert.Nil(t, err)

	assert.Equal(t, "ee194423-3040-11ee-9393-eab5dfc9b22a:1-5:7:9-10,b3512340-fc03-11ec-920f-000c29f6e7cf:1-4", gtidSets.String())
}
