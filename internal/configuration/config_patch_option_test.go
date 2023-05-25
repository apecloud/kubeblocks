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

package configuration

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
