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

type OperationKind string

const (
	RespFieldEvent   = "event"
	RespFieldMessage = "message"
	RespTypMeta      = "metadata"
	RespEveSucc      = "Success"
	RespEveFail      = "Failed"

	CheckRunningOperation OperationKind = "checkRunning"
	HealthyCheckOperation OperationKind = "healthyCheck"
	CheckRoleOperation    OperationKind = "checkRole"
	GetRoleOperation      OperationKind = "getRole"
	GetLagOperation       OperationKind = "getLag"
	SwitchoverOperation   OperationKind = "switchover"

	LockOperation    OperationKind = "lockInstance"
	UnlockOperation  OperationKind = "unlockInstance"
	VolumeProtection OperationKind = "volumeProtection"

	// for component
	PostProvisionOperation OperationKind = "postProvision"
	PreTerminateOperation  OperationKind = "preTerminate"

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

	DefaultProbeTimeoutSeconds = 2

	DataDumpOperation OperationKind = "dataDump"
	DataLoadOperation OperationKind = "dataLoad"

	KBAgentEventFieldPath = "spec.containers{kb-agent}"
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

type CronJob struct {
	TimeoutSeconds   int `json:"timeoutSeconds,omitempty"`
	PeriodSeconds    int `json:"periodSeconds,omitempty"`
	SuccessThreshold int `json:"successThreshold,omitempty"`
	FailureThreshold int `json:"failureThreshold,omitempty"`
}

type Handlers struct {
	Command []string          `json:"command,omitempty"`
	GPRC    map[string]string `json:"grpc,omitempty"`
	CronJob *CronJob          `json:"cronJob,omitempty"`
}
