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

package validate

import "testing"

func TestIsQuotesString(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want bool
	}{{
		name: "quotes_test",
		arg:  ``,
		want: false,
	}, {
		name: "quotes_test",
		arg:  `''`,
		want: true,
	}, {
		name: "quotes_test",
		arg:  `""`,
		want: true,
	}, {
		name: "quotes_test",
		arg:  `'`,
		want: false,
	}, {
		name: "quotes_test",
		arg:  `"`,
		want: false,
	}, {
		name: "quotes_test",
		arg:  `for test`,
		want: false,
	}, {
		name: "quotes_test",
		arg:  `'test'`,
		want: true,
	}, {
		name: "quotes_test",
		arg:  `'test`,
		want: false,
	}, {
		name: "quotes_test",
		arg:  `test'`,
		want: false,
	}, {
		name: "quotes_test",
		arg:  `"test"`,
		want: true,
	}, {
		name: "quotes_test",
		arg:  `test"`,
		want: false,
	}, {
		name: "quotes_test",
		arg:  `"test`,
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isQuotesString(tt.arg); got != tt.want {
				t.Errorf("isQuotesString() = %v, want %v", got, tt.want)
			}
		})
	}
}
