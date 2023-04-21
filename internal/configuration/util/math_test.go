/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package util

import (
	"reflect"
	"testing"

	"golang.org/x/exp/constraints"
)

type sData[T constraints.Ordered] [2]T

func makeTestData[T constraints.Ordered](l, r T) sData[T] {
	var data sData[T]
	data[0] = l
	data[1] = r
	return data
}

func testGenericType[T constraints.Ordered](t *testing.T, data []sData[T], expected []int) {
	for i := 0; i < len(expected); i++ {
		t.Run("test", func(t *testing.T) {
			if got := Min[T](data[i][0], data[i][1]); !reflect.DeepEqual(got, data[i][expected[i]]) {
				t.Errorf("Min() = %v, want %v", got, expected[i])
			}
			if got := Max[T](data[i][0], data[i][1]); !reflect.DeepEqual(got, data[i][1-expected[i]]) {
				t.Errorf("Min() = %v, want %v", got, expected[i])
			}
		})
	}
}

func TestMin(t *testing.T) {
	testGenericType(t, []sData[int]{
		makeTestData(1, 2),
		makeTestData(2, 1),
		makeTestData(1, 1),
	}, []int{0, 1, 0})

	testGenericType(t, []sData[float64]{
		makeTestData(1.1, 2.2),
		makeTestData(2.0, 1.2),
		makeTestData(1.0, 1.0),
	}, []int{0, 1, 0})

	testGenericType(t, []sData[string]{
		makeTestData("abc", "ab"),
		makeTestData("efg", "efge"),
		makeTestData("a", "b"),
		makeTestData("b", "a"),
		makeTestData("", "a"),
		makeTestData("ab", ""),
		makeTestData("", ""),
	}, []int{1, 0, 0, 1, 0, 1, 0})
}
