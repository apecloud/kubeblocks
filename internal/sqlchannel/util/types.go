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
	"strings"
	"time"

	"github.com/dapr/components-contrib/bindings"
)

const (
	RespTypEve  = "event"
	RespTypMsg  = "message"
	RespTypMeta = "metadata"
	RespEveSucc = "Success"
	RespEveFail = "Failed"

	CheckRunningOperation bindings.OperationKind = "checkRunning"
	CheckStatusOperation  bindings.OperationKind = "checkStatus"
	CheckRoleOperation    bindings.OperationKind = "checkRole"
	GetRoleOperation      bindings.OperationKind = "getRole"
	GetLagOperation       bindings.OperationKind = "getLag"
	ExecOperation         bindings.OperationKind = "exec"
	QueryOperation        bindings.OperationKind = "query"
	CloseOperation        bindings.OperationKind = "close"

	// actions for cluster accounts management
	ListUsersOp      bindings.OperationKind = "listUsers"
	CreateUserOp     bindings.OperationKind = "createUser"
	DeleteUserOp     bindings.OperationKind = "deleteUser"
	DescribeUserOp   bindings.OperationKind = "describeUser"
	GrantUserRoleOp  bindings.OperationKind = "grantUserRole"
	RevokeUserRoleOp bindings.OperationKind = "revokeUserRole"

	OperationNotImplemented = "NotImplemented"
	OperationInvalid        = "Invalid"
	OperationSuccess        = "Success"
	OperationFailed         = "Failed"

	HTTPRequestPrefx string = "curl -X POST -H 'Content-Type: application/json' http://localhost:%d/v1.0/bindings/%s"
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

const (
	SuperUserRole  RoleType = "superuser"
	ReadWriteRole  RoleType = "readwrite"
	ReadOnlyRole   RoleType = "readonly"
	NoPrivileges   RoleType = ""
	CustomizedRole RoleType = "customized"
	InvalidRole    RoleType = "invalid"
)

// UserInfo is the user information for account management
type UserInfo struct {
	UserName string        `json:"userName"`
	Password string        `json:"password,omitempty"`
	Expired  string        `json:"expired,omitempty"`
	ExpireAt time.Duration `json:"expireAt,omitempty"`
	RoleName string        `json:"roleName,omitempty"`
}

// SQLChannelRequest is the request for sqlchannel
type SQLChannelRequest struct {
	Operation string                 `json:"operation"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SQLChannelResponse is the response for sqlchannel
type SQLChannelResponse struct {
	Event    string         `json:"event,omitempty"`
	Message  string         `json:"message,omitempty"`
	Metadata SQLChannelMeta `json:"metadata,omitempty"`
}

// SQLChannelMeta is the metadata for sqlchannel
type SQLChannelMeta struct {
	Operation string    `json:"operation,omitempty"`
	StartTime time.Time `json:"startTime,omitempty"`
	EndTime   time.Time `json:"endTime,omitempty"`
	Extra     string    `json:"extra,omitempty"`
}
