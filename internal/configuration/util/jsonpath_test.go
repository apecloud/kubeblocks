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

func TestJSONPatch(t *testing.T) {
	type args struct {
		original interface{}
		modified interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{{
		name: "modify_test",
		args: args{
			original: map[string]string{
				"a": "b",
			},
			modified: map[string]string{
				"a": "c",
			},
		},
		want: []byte(`{"a":"c"}`),
	}, {
		name: "add_test",
		args: args{
			original: map[string]string{},
			modified: map[string]string{
				"a": "b",
			},
		},
		want: []byte(`{"a":"b"}`),
	}, {
		name: "delete_test",
		args: args{
			original: map[string]string{
				"a": "b",
			},
			modified: map[string]string{},
		},
		want: []byte(`{"a":null}`),
	}, {
		name: "not_modify_test",
		args: args{
			original: map[string]string{
				"a": "b",
			},
			modified: map[string]string{
				"a": "b",
			},
		},
		want: []byte(`{}`),
	}, {
		name: "null_test1",
		args: args{
			original: nil,
			modified: map[string]string{
				"a": "c",
			},
		},
		want: []byte(`{"a":"c"}`),
	}, {
		name: "null_test2",
		args: args{
			original: map[string]string{
				"a": "b",
			},
			modified: nil,
		},
		want: []byte(`{"a":null}`),
	}, {
		name: "test1",
		args: args{
			original: map[string]interface{}{
				"a": map[string]string{
					"b": "c",
					"e": "f",
				},
			},
			modified: map[string]interface{}{
				"a": map[string]string{
					"b": "g",
				},
				"b": "e",
			},
		},
		want: []byte(`{"a":{"b":"g","e":null},"b":"e"}`),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := JSONPatch(tt.args.original, tt.args.modified)
			if (err != nil) != tt.wantErr {
				t.Errorf("JSONPatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("JSONPatch() got = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}

func TestRetrievalWithJSONPath(t *testing.T) {
	testData := map[string]interface{}{
		"a": map[string]interface{}{
			"b": "c",
			"d": "f",
		},
		"b": map[string]interface{}{
			"c": map[string]interface{}{
				"d": "e",
			},
		},
		"test": "test",
	}
	type args struct {
		jsonObj  interface{}
		jsonpath string
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{{
		name: "test1",
		args: args{
			jsonObj:  testData,
			jsonpath: "$.a.b",
		},
		want: []byte(`c`),
	}, {
		name: "test2",
		args: args{
			jsonObj:  testData,
			jsonpath: "$.test",
		},
		want: []byte(`test`),
	}, {
		name: "test3",
		args: args{
			jsonObj:  testData,
			jsonpath: "$..d",
		},
		want: []byte(`["f","e"]`),
	}, {
		name: "failed_test",
		args: args{
			jsonObj:  testData,
			jsonpath: "a.b",
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RetrievalWithJSONPath(tt.args.jsonObj, tt.args.jsonpath)
			if (err != nil) != tt.wantErr {
				t.Errorf("RetrievalWithJSONPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RetrievalWithJSONPath() got = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}
