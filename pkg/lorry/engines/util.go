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

package engines

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// types for probe
	CheckRunningType int = iota
	CheckStatusType
	CheckRoleChangedType
)

func MaxInt64(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}

func GetIndex(memberName string) (int, error) {
	i := strings.LastIndex(memberName, "-")
	if i < 0 {
		return 0, fmt.Errorf("the format of member name is wrong: %s", memberName)
	}
	return strconv.Atoi(memberName[i+1:])
}

func AddSingleQuote(str string) string {
	return "'" + str + "'"
}

type Properties map[string]string
