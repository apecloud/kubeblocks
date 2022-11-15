/*
Copyright 2022.

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

package configuration

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDifference(t *testing.T) {
	type args struct {
		left  *Set
		right *Set
	}
	tests := []struct {
		name string
		args args
		want *Set
	}{
		{
			name: "test1",
			args: args{
				left: NewSetFromList([]string{
					"a", "b", "e", "g",
				}),
				right: NewSetFromList([]string{}),
			},
			want: NewSetFromList([]string{
				"b", "a", "e", "g",
			}),
		},
		{
			name: "empty_test",
			args: args{
				left:  NewSetFromList([]string{}),
				right: NewSetFromList([]string{}),
			},
			want: NewSetFromList([]string{}),
		},
		{
			name: "test2",
			args: args{
				left: NewSetFromList([]string{
					"a", "b", "e", "g",
				}),
				right: NewSetFromList([]string{
					"a", "g", "x", "z",
				}),
			},
			want: NewSetFromList([]string{
				"b", "e",
			}),
		},
		{
			name: "test_contained",
			args: args{
				left: NewSetFromList([]string{
					"a", "b", "e", "g",
				}),
				right: NewSetFromList([]string{
					"a", "g",
				}),
			},
			want: NewSetFromList([]string{
				"b", "e",
			}),
		},
		{
			name: "test_contained2",
			args: args{
				left: NewSetFromList([]string{"a"}),
				right: NewSetFromList([]string{
					"a", "g",
				}),
			},
			want: NewSetFromList([]string{}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Difference(tt.args.left, tt.args.right); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Difference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMapKeyDifference(t *testing.T) {
	type args struct {
		left  map[string]interface{}
		right map[string]interface{}
	}
	tests := []struct {
		name string
		args args
		want *Set
	}{
		// for map
		{
			name: "test_map",
			args: args{
				left: map[string]interface{}{
					"a": 2,
					"b": 3,
					"c": 5,
				},
				right: map[string]interface{}{
					"a": 2,
					"e": 3,
					"f": 5,
				},
			},
			want: NewSetFromList([]string{"b", "c"}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MapKeyDifference(tt.args.left, tt.args.right); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MapKeyDifference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSet_ForEach(t *testing.T) {
	s := NewSetFromList([]string{"a", "b", "c"})
	require.Equal(t, s.Size(), 3)
	require.True(t, s.Contains("b"))
	require.False(t, s.Contains("bb"))

	require.Equal(t, Union(s, NewSetFromList([]string{"a"})).Size(), 1)
	require.True(t, Union(s, NewSetFromList([]string{"bb"})).Empty())

	require.True(t, Union(NewSetFromList([]string{}), NewSetFromList([]string{})).Empty())

	count := 0
	s.ForEach(func(key string) {
		count++
	})

	require.Equal(t, count, s.Size())
}
