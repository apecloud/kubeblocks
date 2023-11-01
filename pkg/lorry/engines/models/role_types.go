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

package models

import "strings"

const (
	SuperUserRole  RoleType = "superuser"
	ReadWriteRole  RoleType = "readwrite"
	ReadOnlyRole   RoleType = "readonly"
	NoPrivileges   RoleType = ""
	CustomizedRole RoleType = "customized"
	InvalidRole    RoleType = "invalid"
)

type RoleType string

func (r RoleType) EqualTo(role string) bool {
	return strings.EqualFold(string(r), role)
}

func (r RoleType) GetWeight() int32 {
	switch r {
	case SuperUserRole:
		return 1 << 3
	case ReadWriteRole:
		return 1 << 2
	case ReadOnlyRole:
		return 1 << 1
	case CustomizedRole:
		return 1
	default:
		return 0
	}
}

func SortRoleByWeight(r1, r2 RoleType) bool {
	return int(r1.GetWeight()) > int(r2.GetWeight())
}

func String2RoleType(roleName string) RoleType {
	if SuperUserRole.EqualTo(roleName) {
		return SuperUserRole
	}
	if ReadWriteRole.EqualTo(roleName) {
		return ReadWriteRole
	}
	if ReadOnlyRole.EqualTo(roleName) {
		return ReadOnlyRole
	}
	if NoPrivileges.EqualTo(roleName) {
		return NoPrivileges
	}
	return CustomizedRole
}
