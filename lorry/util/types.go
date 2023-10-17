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
)

type OperationKind string

const (
	RespFieldEvent = "event"
	RespTypMsg     = "message"
	RespTypMeta    = "metadata"
	RespEveSucc    = "Success"
	RespEveFail    = "Failed"

	GetOperation    OperationKind = "get"
	CreateOperation OperationKind = "create"
	DeleteOperation OperationKind = "delete"
	ListOperation   OperationKind = "list"

	CheckRunningOperation OperationKind = "checkRunning"
	CheckStatusOperation  OperationKind = "checkStatus"
	CheckRoleOperation    OperationKind = "checkRole"
	GetRoleOperation      OperationKind = "getRole"
	GetLagOperation       OperationKind = "getLag"
	SwitchoverOperation   OperationKind = "switchover"
	ExecOperation         OperationKind = "exec"
	QueryOperation        OperationKind = "query"
	CloseOperation        OperationKind = "close"

	LockOperation    OperationKind = "lockInstance"
	UnlockOperation  OperationKind = "unlockInstance"
	VolumeProtection OperationKind = "volumeProtection"

	// actions for cluster accounts management
	ListUsersOp          OperationKind = "listUsers"
	CreateUserOp         OperationKind = "createUser"
	DeleteUserOp         OperationKind = "deleteUser"
	DescribeUserOp       OperationKind = "describeUser"
	GrantUserRoleOp      OperationKind = "grantUserRole"
	RevokeUserRoleOp     OperationKind = "revokeUserRole"
	ListSystemAccountsOp OperationKind = "listSystemAccounts"

	JoinMemberOperation  OperationKind = "joinMember"
	LeaveMemberOperation OperationKind = "leaveMember"

	OperationNotImplemented    = "NotImplemented"
	OperationInvalid           = "Invalid"
	OperationSuccess           = "Success"
	OperationFailed            = "Failed"
	DefaultProbeTimeoutSeconds = 2

	HTTPRequestPrefx string = "curl -X POST -H 'Content-Type: application/json' http://localhost:%d/v1.0/bindings/%s"

	// this is a general script template, which can be used for all kinds of exec request to databases.
	DataScriptRequestTpl string = `
		response=$(curl -s -X POST -H 'Content-Type: application/json' http://%s:3501/v1.0/bindings/%s -d '%s')
		result=$(echo $response | jq -r '.event')
		message=$(echo $response | jq -r '.message')
		if [ "$result" == "Failed" ]; then
			echo $message
			exit 1
		else
			echo "$result"
			exit 0
		fi
			`
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

type errorReason string

const (
	UnsupportedOps errorReason = "unsupported operation"
)

// SQLChannelError is the error for sqlchannel, it implements error interface
type SQLChannelError struct {
	Reason errorReason
}

var _ error = SQLChannelError{}

func (e SQLChannelError) Error() string {
	return string(e.Reason)
}

// IsUnSupportedError checks if the error is unsupported operation error
func IsUnSupportedError(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(SQLChannelError); ok {
		return e.Reason == UnsupportedOps
	}
	return false
}

// ProbeError is the error for Lorry probe api, it implements error interface
type ProbeError struct {
	message string
}

var _ error = ProbeError{}

func (e ProbeError) Error() string {
	return e.message
}

func NewProbeError(msg string) error {
	return ProbeError{
		message: msg,
	}
}
