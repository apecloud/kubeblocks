/*
Copyright ApeCloud, Inc.

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
