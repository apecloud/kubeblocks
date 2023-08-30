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

package core

import "testing"

type testType struct {
	name string
}

func TestTypeMatch(t *testing.T) {
	type args struct {
		expected interface{}
		values   []interface{}
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		"string_type_test",
		args{
			expected: "",
			values:   []interface{}{"", "xxxx"},
		},
		true,
	}, {
		"byte_type_test_failed",
		args{
			expected: []byte{},
			values:   []interface{}{[]byte("abcd")},
		},
		true,
	}, {
		"byte_type_test_failed_without_match",
		args{
			expected: []byte{},
			values:   []interface{}{"abcd"},
		},
		false,
	}, {
		"byte_type_test_failed_with_null",
		args{
			expected: []byte{},
			values:   []interface{}{nil},
		},
		false,
	}, {
		"byte_type_test_failed_with_null",
		args{
			expected: nil,
			values:   []interface{}{nil},
		},
		false,
	}, {
		"byte_type_test_failed_with_null2",
		args{
			expected: nil,
			values:   []interface{}{[]byte("abcd")},
		},
		false,
	}, {
		"custom_type",
		args{
			expected: testType{},
			values:   []interface{}{testType{name: "abcd"}},
		},
		true,
	}, {
		"custom_type2",
		args{
			expected: &testType{},
			values:   []interface{}{testType{name: "abcd"}},
		},
		false,
	}, {
		"custom_type_with_pointer",
		args{
			expected: &testType{},
			values:   []interface{}{&testType{name: "abcd"}},
		},
		true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := typeMatch(tt.args.expected, tt.args.values...); got != tt.want {
				t.Errorf("typeMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompareWithConfig(t *testing.T) {
	type args struct {
		left   interface{}
		right  interface{}
		option CfgOption
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{{
		name: "raw_type_test",
		args: args{
			left:   []byte("byte"),
			right:  "string",
			option: CfgOption{Type: CfgRawType},
		},
		wantErr: true,
	}, {
		name: "raw_type_test",
		args: args{
			left:   []byte("byte"),
			right:  []byte("test"),
			option: CfgOption{Type: CfgRawType},
		},
		want: false,
	}, {
		name: "raw_type_test",
		args: args{
			left:   []byte("byte"),
			right:  []byte("byte"),
			option: CfgOption{Type: CfgRawType},
		},
		want: true,
	}, {
		name: "localfile_type_test",
		args: args{
			left:   []byte("byte"),
			right:  "string",
			option: CfgOption{Type: CfgLocalType},
		},
		wantErr: true,
	}, {
		name: "localfile_type_test",
		args: args{
			left:   "byte",
			right:  "string",
			option: CfgOption{Type: CfgLocalType},
		},
		want: false,
	}, {
		name: "tpl_type_test",
		args: args{
			left:   &ConfigResource{},
			right:  "string",
			option: CfgOption{Type: CfgCmType},
		},
		wantErr: true,
	}, {
		name: "tpl_type_test",
		args: args{
			option: CfgOption{Type: "not_support"},
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compareWithConfig(tt.args.left, tt.args.right, tt.args.option)
			if (err != nil) != tt.wantErr {
				t.Errorf("compareWithConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("compareWithConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}
