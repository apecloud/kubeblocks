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

import "fmt"

const (
	errMsgNoSQL           = "no sql provided"
	errMsgNoUserName      = "no username provided"
	errMsgNoPassword      = "no password provided"
	errMsgNoRoleName      = "no rolename provided"
	errMsgInvalidRoleName = "invalid rolename, should be one of [superuser, readwrite, readonly]"
	errMsgNoSuchUser      = "no such user"
	errMsgNotImplemented  = "not implemented"
)

var (
	ErrNoSQL           = fmt.Errorf(errMsgNoSQL)
	ErrNoUserName      = fmt.Errorf(errMsgNoUserName)
	ErrNoPassword      = fmt.Errorf(errMsgNoPassword)
	ErrNoRoleName      = fmt.Errorf(errMsgNoRoleName)
	ErrInvalidRoleName = fmt.Errorf(errMsgInvalidRoleName)
	ErrNoSuchUser      = fmt.Errorf(errMsgNoSuchUser)
	ErrNotImplemented  = fmt.Errorf(errMsgNotImplemented)
)
