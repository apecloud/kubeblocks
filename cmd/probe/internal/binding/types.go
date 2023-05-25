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

package binding

import (
	"fmt"
	"strings"
)

const (
	ReadWrite AccessMode = "ReadWrite"
	Readonly  AccessMode = "Readonly"
	None      AccessMode = "None"

	// keys from response's metadata.
	RespOpKey           = "operation"
	RespStartTimeKey    = "start-time"
	RespRowsAffectedKey = "rows-affected"
	RespEndTimeKey      = "end-time"
	RespDurationKey     = "duration"
	StatusCode          = "status-code"
	// 451 Unavailable For Legal Reasons, used to indicate operation failed.
	OperationFailedHTTPCode = "451"
	// 404 Not Found, used to indicate operation not found.
	OperationNotFoundHTTPCode = "404"

	// CommandSQLKey keys from request's metadata.
	CommandSQLKey = "sql"

	roleEventRecordQPS                = 1. / 60.
	roleEventReportFrequency          = int(1 / roleEventRecordQPS)
	defaultFailedEventReportFrequency = 1800
	defaultRoleDetectionThreshold     = 300
)

const (
	// types for probe
	CheckRunningType int = iota
	CheckStatusType
	CheckRoleChangedType
)

const (
	RespTypEve  = "event"
	RespTypMsg  = "message"
	RespTypMeta = "metadata"

	RespEveSucc = "Success"
	RespEveFail = "Failed"

	PRIMARY   = "primary"
	SECONDARY = "secondary"
	MASTER    = "master"
	SLAVE     = "slave"
	LEADER    = "Leader"
	FOLLOWER  = "Follower"
	LEARNER   = "Learner"
	CANDIDATE = "Candidate"
)

type RoleType string

const (
	SuperUserRole  RoleType = "superuser"
	ReadWriteRole  RoleType = "readwrite"
	ReadOnlyRole   RoleType = "readonly"
	NoPrivileges   RoleType = ""
	CustomizedRole RoleType = "customized"
	InvalidRole    RoleType = "invalid"
)

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

const (
	errMsgNoSQL           = "no sql provided"
	errMsgNoUserName      = "no username provided"
	errMsgNoPassword      = "no password provided"
	errMsgNoRoleName      = "no rolename provided"
	errMsgInvalidRoleName = "invalid rolename, should be one of [superuser, readwrite, readonly]"
	errMsgNoSuchUser      = "no such user"
)

var (
	ErrNoSQL           = fmt.Errorf(errMsgNoSQL)
	ErrNoUserName      = fmt.Errorf(errMsgNoUserName)
	ErrNoPassword      = fmt.Errorf(errMsgNoPassword)
	ErrNoRoleName      = fmt.Errorf(errMsgNoRoleName)
	ErrInvalidRoleName = fmt.Errorf(errMsgInvalidRoleName)
	ErrNoSuchUser      = fmt.Errorf(errMsgNoSuchUser)
)

type SlaveStatus struct {
	SecondsBehindMaster int64 `json:"Seconds_Behind_Master"`
}

const (
	Postgres   = "postgres"
	Postgresql = "postgresql"
	Mysql      = "mysql"
	Mongodb    = "mongodb"
	Redis      = "redis"
	Etcd       = "etcd"
)

var (
	dbTypeToContainer = map[string]string{
		Postgres: Postgresql,
	}
)
