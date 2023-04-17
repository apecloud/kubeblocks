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
