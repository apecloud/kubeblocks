/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
