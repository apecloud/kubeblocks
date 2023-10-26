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

import (
	"time"
)

// UserInfo is the user information for account management
type UserInfo struct {
	UserName string        `json:"userName"`
	Password string        `json:"password,omitempty"`
	Expired  string        `json:"expired,omitempty"`
	ExpireAt time.Duration `json:"expireAt,omitempty"`
	RoleName string        `json:"roleName,omitempty"`
}

func (user *UserInfo) UserNameValidator() error {
	if user.UserName == "" {
		return ErrNoUserName
	}
	return nil
}

func (user *UserInfo) PasswdValidator() error {
	if user.Password == "" {
		return ErrNoPassword
	}
	return nil
}

func (user *UserInfo) RoleValidator() error {
	if user.RoleName == "" {
		return ErrNoRoleName
	}

	roles := []RoleType{ReadOnlyRole, ReadWriteRole, SuperUserRole}
	for _, role := range roles {
		if role.EqualTo(user.RoleName) {
			return nil
		}
	}
	return ErrInvalidRoleName
}

func (user *UserInfo) UserNameAndPasswdValidator() error {
	if err := user.UserNameValidator(); err != nil {
		return err
	}

	if err := user.PasswdValidator(); err != nil {
		return err
	}
	return nil
}

func (user *UserInfo) UserNameAndRoleValidator() error {
	if err := user.UserNameValidator(); err != nil {
		return err
	}

	if err := user.RoleValidator(); err != nil {
		return err
	}
	return nil
}
