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

package redis

import (
	"testing"
)

func TestContainerEscapeString(t *testing.T) {
	tests := []struct {
		args string
		want bool
	}{{
		args: "",
		want: false,
	}, {
		args: "abcd\"",
		want: true,
	}, {
		args: "ab cd",
		want: true,
	}, {
		args: "abcd",
		want: false,
	}, {
		args: "\xff",
		want: false,
	}, {
		args: "\075",
		want: false,
	}}
	for _, tt := range tests {
		t.Run("escapeStringTest", func(t *testing.T) {
			if got := ContainerEscapeString(tt.args); got != tt.want {
				t.Errorf("ContainerEscapeString() = %v, want %v", got, tt.want)
			}
		})
	}
}
