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
	"encoding/json"
	"reflect"
	"testing"

	"github.com/StudioSol/set"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	a int
	b string
}

func TestUnstructuredObjectWalk(t *testing.T) {
	type args struct {
		data     string
		isStruct bool
		expected []string
		sdata    interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{{
		name: "test",
		args: args{
			data:     `"a"`,
			expected: []string{},
			isStruct: false,
		},
	}, {
		name: "test",
		args: args{
			data:     `{"a": "b"}`,
			expected: []string{"a"},
			isStruct: false,
		},
	}, {
		name: "test",
		args: args{
			data: `{"a": 
{ "b" : { "e": {
				"c" : 10,
				"d" : "abcd"
			   },
          "f" : 12.6,
 		  "z" : [
					{"x1" : 1,
					 "x2" : 2
					},
					{"x3" : 1,
					 "x4" : 2
					}
			]
		},
  "g" : [ "e1","e2","e3"],
  "x" : [ 20,30] 
}}`,
			expected: []string{"a.b.z.x1", "a.b.e.c", "a.b.e.d", "a.b.f", "a.b.z.x2", "a.b.z.x4", "a.b.z.x3", "a.g", "a.x"},
			isStruct: false,
		},
	}, {
		name: "testStruct",
		args: args{
			data:     "",
			expected: []string{},
			isStruct: true,
			sdata: testStruct{
				a: 10,
				b: "for_test",
			},
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				obj any
				res = make([]string, 0)
			)

			if !tt.args.isStruct {
				err := json.Unmarshal([]byte(tt.args.data), &obj)
				require.Nil(t, err)
			} else {
				obj = tt.args.sdata
			}
			if err := UnstructuredObjectWalk(obj, func(parent, cur string, v reflect.Value, fn UpdateFn) error {
				if cur == "" && parent != "" {
					res = append(res, parent)
				} else if cur != "" {
					if parent != "" {
						cur = parent + "." + cur
					}
					res = append(res, cur)
				}
				return nil
			}, false); (err != nil) != tt.wantErr {
				t.Errorf("UnstructuredObjectWalk() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				require.True(t, Contains(res, tt.args.expected), "res: %v, expected: %v", res, tt.args.expected)
			}
		})
	}
}

func Contains(left, right []string) bool {
	if len(left) < len(right) {
		return false
	}

	sets := set.NewLinkedHashSetString(left...)
	for _, k := range right {
		if !sets.InArray(k) {
			return false
		}
	}

	return true
}
