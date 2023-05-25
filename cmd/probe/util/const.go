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

import "github.com/dapr/components-contrib/bindings"

const (
	CheckRunningOperation bindings.OperationKind = "checkRunning"
	CheckStatusOperation  bindings.OperationKind = "checkStatus"
	CheckRoleOperation    bindings.OperationKind = "checkRole"
	GetRoleOperation      bindings.OperationKind = "getRole"
	GetLagOperation       bindings.OperationKind = "getLag"
	ExecOperation         bindings.OperationKind = "exec"
	QueryOperation        bindings.OperationKind = "query"
	CloseOperation        bindings.OperationKind = "close"
	SwitchoverOperation   bindings.OperationKind = "switchover"
	FailoverOperation     bindings.OperationKind = "failover"

	// actions for cluster accounts management
	ListUsersOp          bindings.OperationKind = "listUsers"
	CreateUserOp         bindings.OperationKind = "createUser"
	DeleteUserOp         bindings.OperationKind = "deleteUser"
	DescribeUserOp       bindings.OperationKind = "describeUser"
	GrantUserRoleOp      bindings.OperationKind = "grantUserRole"
	RevokeUserRoleOp     bindings.OperationKind = "revokeUserRole"
	ListSystemAccountsOp bindings.OperationKind = "listSystemAccounts"
	// actions for cluster roles management

	OperationNotImplemented = "NotImplemented"
	OperationInvalid        = "Invalid"
	OperationSuccess        = "Success"
	OperationFailed         = "Failed"
)

const (
	HostName               = "HOSTNAME"
	KbClusterName          = "KB_CLUSTER_NAME"
	KbClusterCompName      = "KB_CLUSTER_COMP_NAME"
	KbServiceCharacterType = "KB_SERVICE_CHARACTER_TYPE"
	KbNamespace            = "KB_NAMESPACE"
)
