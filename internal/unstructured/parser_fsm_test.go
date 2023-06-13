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

package unstructured

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
			if err := f.parse(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("parse() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.expected != nil && !reflect.DeepEqual(f.param.Values, tt.expected) {
				t.Errorf("parse() param = %v, expected %v", f.param.Values, tt.expected)
			}
		})
	}
}
