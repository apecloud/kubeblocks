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

package util

import (
	"reflect"
	"testing"
)

func TestDifference(t *testing.T) {
	type args struct {
		left  *Sets
		right *Sets
	}
	tests := []struct {
		name string
		args args
		want *Sets
	}{{
		name: "test1",
		args: args{
			left:  NewSet("a", "b", "e", "g"),
			right: NewSet(),
		},
		want: NewSet("b", "a", "e", "g"),
	}, {
		name: "empty_test",
		args: args{
			left:  NewSet(),
			right: NewSet(),
		},
		want: NewSet(),
	}, {
		name: "test2",
		args: args{
			left:  NewSet("a", "b", "e", "g"),
			right: NewSet("a", "g", "x", "z"),
		},
		want: NewSet("b", "e"),
	}, {
		name: "test_contained",
		args: args{
			left:  NewSet("a", "b", "e", "g"),
			right: NewSet("a", "g"),
		},
		want: NewSet("b", "e"),
	}, {
		name: "test_contained2",
		args: args{
			left:  NewSet("a"),
			right: NewSet("a", "g"),
		},
		want: NewSet(),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Difference(tt.args.left, tt.args.right); !EqSet(got, tt.want) {
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
		want *Sets
	}{{
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
		want: NewSet("b", "c"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MapKeyDifference(tt.args.left, tt.args.right); !EqSet(got, tt.want) {
				t.Errorf("MapKeyDifference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnion(t *testing.T) {
	type args struct {
		left  *Sets
		right *Sets
	}
	tests := []struct {
		name string
		args args
		want *Sets
	}{{
		name: "test1",
		args: args{
			left:  NewSet("a", "b", "e", "g"),
			right: NewSet("a", "g", "x", "z"),
		},
		want: NewSet("a", "g"),
	}, {
		name: "test2",
		args: args{
			left:  NewSet("i", "b", "e", "g"),
			right: NewSet("a", "f", "x", "z"),
		},
		want: NewSet(),
	}, {
		name: "test3",
		args: args{
			left:  NewSet(),
			right: NewSet("a", "f", "x", "z"),
		},
		want: NewSet(),
	}, {
		name: "test4",
		args: args{
			left:  NewSet(),
			right: NewSet(),
		},
		want: NewSet(),
	}, {
		name: "test5",
		args: args{
			left:  NewSet("a", "b", "e", "g"),
			right: NewSet("a", "b", "e", "g"),
		},
		want: NewSet("a", "b", "e", "g"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Union(tt.args.left, tt.args.right); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Union() = %v, want %v", got.AsSlice(), tt.want.AsSlice())
			}
		})
	}
}
