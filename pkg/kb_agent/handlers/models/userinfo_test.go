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

func TestPasswdValidator(t *testing.T) {
	user := UserInfo{
		Password: "password123",
	}

	err := user.PasswdValidator()
	if err != nil {
		t.Errorf("PasswdValidator() returned an unexpected error: %v", err)
	}

	user.Password = ""
	err = user.PasswdValidator()
	if err != ErrNoPassword {
		t.Errorf("PasswdValidator() did not return the expected error. Got: %v, Want: %v", err, ErrNoPassword)
	}
}
func TestRoleValidator(t *testing.T) {
	user := UserInfo{
		RoleName: "readonly",
	}

	err := user.RoleValidator()
	if err != nil {
		t.Errorf("RoleValidator() returned an unexpected error: %v", err)
	}

	user.RoleName = "InvalidRole"
	err = user.RoleValidator()
	if err != ErrInvalidRoleName {
		t.Errorf("RoleValidator() did not return the expected error. Got: %v, Want: %v", err, ErrInvalidRoleName)
	}
}

func TestUserNameAndPasswdValidator(t *testing.T) {
	user := UserInfo{
		UserName: "john.doe",
		Password: "password123",
	}

	err := user.UserNameAndPasswdValidator()
	if err != nil {
		t.Errorf("UserNameAndPasswdValidator() returned an unexpected error: %v", err)
	}

	user.UserName = ""
	err = user.UserNameAndPasswdValidator()
	if err != ErrNoUserName {
		t.Errorf("UserNameAndPasswdValidator() did not return the expected error. Got: %v, Want: %v", err, ErrNoUserName)
	}

	user.UserName = "john.doe"
	user.Password = ""
	err = user.UserNameAndPasswdValidator()
	if err != ErrNoPassword {
		t.Errorf("UserNameAndPasswdValidator() did not return the expected error. Got: %v, Want: %v", err, ErrNoPassword)
	}
}

func TestUserNameAndRoleValidator(t *testing.T) {
	user := UserInfo{
		UserName: "john.doe",
		RoleName: "readOnly",
	}

	err := user.UserNameAndRoleValidator()
	if err != nil {
		t.Errorf("UserNameAndRoleValidator() returned an unexpected error: %v", err)
	}

	user.UserName = ""
	err = user.UserNameAndRoleValidator()
	if err != ErrNoUserName {
		t.Errorf("UserNameAndRoleValidator() did not return the expected error. Got: %v, Want: %v", err, ErrNoUserName)
	}

	user.UserName = "john.doe"
	user.RoleName = ""
	err = user.UserNameAndRoleValidator()
	if err != ErrNoRoleName {
		t.Errorf("UserNameAndRoleValidator() did not return the expected error. Got: %v, Want: %v", err, ErrNoRoleName)
	}
}

func TestUserNameValidator(t *testing.T) {
	user := UserInfo{
		UserName: "john.doe",
	}

	err := user.UserNameValidator()
	if err != nil {
		t.Errorf("UserNameValidator() returned an unexpected error: %v", err)
	}

	user.UserName = ""
	err = user.UserNameValidator()
	if err != ErrNoUserName {
		t.Errorf("UserNameValidator() did not return the expected error. Got: %v, Want: %v", err, ErrNoUserName)
	}
}
