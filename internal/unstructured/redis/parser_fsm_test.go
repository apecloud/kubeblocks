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

package redis

import (
	"reflect"
	"testing"
)

func TestFsmParse(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		wantErr  bool
		expected []string
	}{{
		name:     "test",
		args:     "sds",
		wantErr:  false,
		expected: []string{"sds"},
	}, {
		name:     "test",
		args:     "100",
		wantErr:  false,
		expected: []string{"100"},
	}, {
		name:     "test",
		args:     `"efg"`,
		wantErr:  false,
		expected: []string{"efg"},
	}, {
		name:     "test",
		args:     `'efg'`,
		wantErr:  false,
		expected: []string{"efg"},
	}, {
		name:     "test",
		args:     `'efg""'`,
		wantErr:  false,
		expected: []string{"efg\"\""},
	}, {
		name:     "test",
		args:     `'efg\' test'`,
		wantErr:  false,
		expected: []string{"efg' test"},
	}, { // for error
		name:    "test",
		args:    `efg\' test`,
		wantErr: true,
	}, {
		name:     "test",
		args:     `' test'`,
		expected: []string{" test"},
		wantErr:  false,
	}, {
		name:     "test",
		args:     `bind 192.168.1.100 10.0.0.1`,
		expected: []string{"bind", "192.168.1.100", "10.0.0.1"},
		wantErr:  false,
	}, {
		name:     "test",
		args:     `bind 127.0.0.1 ::1 `,
		expected: []string{"bind", "127.0.0.1", "::1"},
		wantErr:  false,
	}, {
		name:     "test",
		args:     `bind * -::* `,
		expected: []string{"bind", "*", "-::*"},
		wantErr:  false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fsm{
				param:           &Item{},
				splitCharacters: trimChars,
			}
			if err := f.Parse(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.expected != nil && !reflect.DeepEqual(f.param.Values, tt.expected) {
				t.Errorf("Parse() param = %v, expected %v", f.param.Values, tt.expected)
			}
		})
	}
}
