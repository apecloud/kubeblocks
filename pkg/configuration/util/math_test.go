/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"math"
	"testing"
)

func TestSafe2Int32(t *testing.T) {
	tests := []struct {
		name string
		args int
		want int32
	}{{
		name: "test",
		args: 0,
		want: 0,
	}, {
		name: "test",
		args: 999,
		want: 999,
	}, {
		name: "test",
		args: math.MinInt32,
		want: math.MinInt32,
	}, {
		name: "test",
		args: math.MaxInt32,
		want: math.MaxInt32,
	}, {
		name: "test",
		args: math.MinInt32 - 2,
		want: math.MinInt32,
	}, {
		name: "test",
		args: math.MaxInt32 + 10,
		want: math.MaxInt32,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Safe2Int32(tt.args); got != tt.want {
				t.Errorf("Safe2Int32() = %v, want %v", got, tt.want)
			}
		})
	}
}
