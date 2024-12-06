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

package generics

func CountFunc[S ~[]E, E any](s S, f func(E) bool) int {
	cnt := 0
	for i := range s {
		if f(s[i]) {
			cnt += 1
		}
	}
	return cnt
}

func FindFunc[S ~[]E, E any](s S, f func(E) bool) S {
	var arr S
	for _, e := range s {
		if f(e) {
			arr = append(arr, e)
		}
	}
	return arr
}

func FindFirstFunc[S ~[]E, E any](s S, f func(E) bool) int {
	for i, e := range s {
		if f(e) {
			return i
		}
	}
	return -1
}

func Map[E any, F any](s []E, f func(E) F) []F {
	var arr []F
	for _, e := range s {
		arr = append(arr, f(e))
	}
	return arr
}
