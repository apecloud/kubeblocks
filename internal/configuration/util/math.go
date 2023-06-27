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
	"math"

	"golang.org/x/exp/constraints"
)

func Min[T constraints.Ordered](l, r T) T {
	if l < r {
		return l
	}
	return r
}

func Max[T constraints.Ordered](l, r T) T {
	if l < r {
		return r
	}
	return l
}

func Safe2Int32(v int) int32 {
	switch {
	default:
		return int32(v)
	case v < math.MinInt32:
		return math.MinInt32
	case v > math.MaxInt32:
		return math.MaxInt32
	}
}
