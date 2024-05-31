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

package models

import "testing"

func TestRoleType_EqualTo(t *testing.T) {
	r := RoleType("superuser")
	role := "superuser"
	if !r.EqualTo(role) {
		t.Errorf("Expected EqualTo to return true, but got false")
	}

	r = RoleType("readwrite")
	role = "readonly"
	if r.EqualTo(role) {
		t.Errorf("Expected EqualTo to return false, but got true")
	}

	// Add more test cases here if needed
}

func TestRoleType_GetWeight(t *testing.T) {
	tests := []struct {
		name string
		role RoleType
		want int32
	}{
		{
			name: "SuperUserRole",
			role: SuperUserRole,
			want: 8,
		},
		{
			name: "ReadWriteRole",
			role: ReadWriteRole,
			want: 4,
		},
		{
			name: "ReadOnlyRole",
			role: ReadOnlyRole,
			want: 2,
		},
		{
			name: "CustomizedRole",
			role: CustomizedRole,
			want: 1,
		},
		{
			name: "InvalidRole",
			role: InvalidRole,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.role.GetWeight(); got != tt.want {
				t.Errorf("RoleType.GetWeight() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortRoleByWeight(t *testing.T) {
	tests := []struct {
		name   string
		role1  RoleType
		role2  RoleType
		result bool
	}{
		{
			name:   "SuperUserRole vs ReadWriteRole",
			role1:  SuperUserRole,
			role2:  ReadWriteRole,
			result: true,
		},
		{
			name:   "ReadWriteRole vs ReadOnlyRole",
			role1:  ReadWriteRole,
			role2:  ReadOnlyRole,
			result: true,
		},
		{
			name:   "ReadOnlyRole vs CustomizedRole",
			role1:  ReadOnlyRole,
			role2:  CustomizedRole,
			result: true,
		},
		{
			name:   "CustomizedRole vs SuperUserRole",
			role1:  CustomizedRole,
			role2:  SuperUserRole,
			result: false,
		},
		{
			name:   "SuperUserRole vs SuperUserRole",
			role1:  SuperUserRole,
			role2:  SuperUserRole,
			result: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SortRoleByWeight(tt.role1, tt.role2); got != tt.result {
				t.Errorf("SortRoleByWeight(%v, %v) = %v, want %v", tt.role1, tt.role2, got, tt.result)
			}
		})
	}
}

func TestString2RoleType(t *testing.T) {
	tests := []struct {
		name     string
		roleName string
		want     RoleType
	}{
		{
			name:     "SuperUserRole",
			roleName: "superuser",
			want:     SuperUserRole,
		},
		{
			name:     "ReadWriteRole",
			roleName: "readwrite",
			want:     ReadWriteRole,
		},
		{
			name:     "ReadOnlyRole",
			roleName: "readonly",
			want:     ReadOnlyRole,
		},
		{
			name:     "NoPrivileges",
			roleName: "",
			want:     NoPrivileges,
		},
		{
			name:     "CustomizedRole",
			roleName: "customized",
			want:     CustomizedRole,
		},
		{
			name:     "InvalidRole",
			roleName: "invalid",
			want:     CustomizedRole,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := String2RoleType(tt.roleName); got != tt.want {
				t.Errorf("String2RoleType(%v) = %v, want %v", tt.roleName, got, tt.want)
			}
		})
	}
}
