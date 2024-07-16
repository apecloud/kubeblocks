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
	"strings"
)

func EnvM2L(m map[string]string) []string {
	l := make([]string, 0)
	for k, v := range m {
		l = append(l, strings.ToUpper(k)+"="+v)
	}
	return l
}

func EnvL2M(l []string) map[string]string {
	m := make(map[string]string, 0)
	for _, p := range l {
		kv := strings.Split(p, "=")
		if len(kv) == 2 {
			m[strings.ToLower(kv[0])] = kv[1]
		}
		if len(kv) == 1 {
			m[strings.ToLower(kv[0])] = ""
		}
	}
	return m
}
