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
	"testing"

	"github.com/StudioSol/set"
)

func TestDifference(t *testing.T) {
	type args struct {
		left  *set.LinkedHashSetString
		right *set.LinkedHashSetString
	}
	tests := []struct {
		name string
		args args
		want *set.LinkedHashSetString
	}{{
		name: "test1",
		args: args{
			left:  set.NewLinkedHashSetString("a", "b", "e", "g"),
			right: set.NewLinkedHashSetString(),
		},
		want: set.NewLinkedHashSetString("b", "a", "e", "g"),
	}, {
		name: "empty_test",
		args: args{
			left:  set.NewLinkedHashSetString(),
			right: set.NewLinkedHashSetString(),
		},
		want: set.NewLinkedHashSetString(),
	}, {
		name: "test2",
		args: args{
			left:  set.NewLinkedHashSetString("a", "b", "e", "g"),
			right: set.NewLinkedHashSetString("a", "g", "x", "z"),
		},
		want: set.NewLinkedHashSetString("b", "e"),
	}, {
		name: "test_contained",
		args: args{
			left:  set.NewLinkedHashSetString("a", "b", "e", "g"),
			right: set.NewLinkedHashSetString("a", "g"),
		},
		want: set.NewLinkedHashSetString("b", "e"),
	}, {
		name: "test_contained2",
		args: args{
			left:  set.NewLinkedHashSetString("a"),
			right: set.NewLinkedHashSetString("a", "g"),
		},
		want: set.NewLinkedHashSetString(),
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
		want *set.LinkedHashSetString
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
		want: set.NewLinkedHashSetString("b", "c"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MapKeyDifference(tt.args.left, tt.args.right); !EqSet(got, tt.want) {
				t.Errorf("MapKeyDifference() = %v, want %v", got, tt.want)
			}
		})
	}
}
