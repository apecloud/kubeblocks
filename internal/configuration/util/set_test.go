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
